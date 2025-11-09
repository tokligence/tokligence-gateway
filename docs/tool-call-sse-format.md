# Tool Call SSE Format for OpenAI Response API

## Overview

Tool calls in the OpenAI Response API use a specific SSE event sequence. This document details the **actual working format** as implemented and validated in tokligence-gateway.

## Complete Tool Call Event Sequence

```
1. response.created                           # Stream start
2. response.output_item.added                 # Tool call item begins (type: function_call)
3. response.function_call_arguments.delta     # Arguments streamed in chunks (multiple events)
4. response.function_call_arguments.done      # Arguments complete
5. response.output_item.done                  # Tool call item complete
6. response.required_action                   # Client must call submit_tool_outputs
7. response.completed                         # Stream complete (status=incomplete)
```

Plus `ping` events periodically during generation.

## Detailed Event Structures

### 1. response.created

**Identical to text responses**:

```json
event: response.created
data: {
  "type": "response.created",
  "sequence_number": 0,
  "response": {
    "id": "resp_1762401620194553092",
    "object": "response",
    "created_at": 1762401620,
    "status": "in_progress"
  }
}
```

### 2. response.output_item.added

**Critical differences for tool calls**:
- `type`: **"function_call"** (not "message")
- Includes `name`, `call_id`, `arguments` fields
- No `role` or `content` fields

```json
event: response.output_item.added
data: {
  "type": "response.output_item.added",
  "sequence_number": 1,
  "output_index": 0,
  "item": {
    "id": "fc_1762401621560360328",
    "type": "function_call",
    "status": "in_progress",
    "name": "shell",
    "call_id": "call_1762401621560363538",
    "arguments": ""
  }
}
```

**Key Fields**:
- `item.id`: Unique ID for this function call (used as `item_id` in subsequent events)
- `item.type`: **"function_call"** (not "message")
- `item.name`: Function name (e.g., "shell", "calculator", "get_weather")
- `item.call_id`: Unique call identifier (preserved in response)
- `item.arguments`: Empty string initially (filled via deltas)
- `item.status`: "in_progress"

### 3. response.function_call_arguments.delta

**Streamed argument chunks**. Multiple events as arguments are generated:

```json
event: response.function_call_arguments.delta
data: {
  "type": "response.function_call_arguments.delta",
  "sequence_number": 2,
  "item_id": "fc_1762401621560360328",
  "output_index": 0,
  "delta": "{\"command\": "
}
```

**Key Fields**:
- `delta`: Incremental JSON string chunk (accumulate to build full arguments)
- `item_id`: **REQUIRED** - must match the `id` from `output_item.added`
- `output_index`: 0 for single output

**Example Sequence**:
```json
delta: "{\"command\": "
delta: "[\"echo\","
delta: "\"hello\"]}"
```

Accumulated result: `{"command": ["echo","hello"]}`

### 4. response.function_call_arguments.done

**Complete arguments after all deltas**:

```json
event: response.function_call_arguments.done
data: {
  "type": "response.function_call_arguments.done",
  "sequence_number": 7,
  "item_id": "fc_1762401621560360328",
  "output_index": 0,
  "arguments": "{\"command\": [\"echo\",\"hello\"]}"
}
```

**Key Fields**:
- `arguments`: **Full accumulated JSON string** (not just last delta)
- `item_id`: **REQUIRED** - must match

### 5. response.output_item.done

**Complete tool call item**:

```json
event: response.output_item.done
data: {
  "type": "response.output_item.done",
  "sequence_number": 8,
  "output_index": 0,
  "item": {
    "id": "fc_1762401621560360328",
    "type": "function_call",
    "status": "completed",
    "name": "shell",
    "call_id": "call_1762401621560363538",
    "arguments": "{\"command\": [\"echo\",\"hello\"]}"
  }
}
```

**Key Changes from `added` event**:
- `status`: **"completed"** (was "in_progress")
- `arguments`: Now contains full JSON string

### 6. response.required_action

**Instruction for the client to run the tool and POST `/v1/responses/{id}/submit_tool_outputs`.**

```json
event: response.required_action
data: {
  "type": "response.required_action",
  "sequence_number": 9,
  "response": {
    "id": "resp_1762401620194553092",
    "object": "response",
    "created_at": 1762401620,
    "status": "incomplete",
    "model": "claude-3-5-sonnet-20241022"
  },
  "required_action": {
    "type": "submit_tool_outputs",
    "submit_tool_outputs": {
      "tool_calls": [
        {
          "id": "call_1762401621560363538",
          "type": "function",
          "function": {
            "name": "shell",
            "arguments": "{\"command\": [\"echo\",\"hello\"]}"
          }
        }
      ]
    }
  }
}
```

**Why this event matters:**
- Codex and other clients listen for it to begin executing tools.
- It must be emitted **before** the incomplete `response.completed`.
- Include the same `required_action` payload that will appear in the final response.

### 7. response.completed

**Final event with complete output array**:

```json
event: response.completed
data: {
  "type": "response.completed",
  "sequence_number": 9,
  "response": {
    "id": "resp_1762401620194553092",
    "object": "response",
    "created_at": 1762401620,
    "status": "completed",
    "model": "claude-3-5-sonnet-20241022",
    "output": [
      {
        "id": "fc_1762401621560360328",
        "type": "function_call",
        "status": "completed",
        "name": "shell",
        "call_id": "call_1762401621560363538",
        "arguments": "{\"command\": [\"echo\",\"hello\"]}"
      }
    ],
    "reasoning": {
      "effort": null,
      "summary": null
    }
  }
}
```

**Key Fields**:
- `output`: Array containing complete function_call object
- `output[0].type`: "function_call"
- `output[0].arguments`: Complete JSON string
- `reasoning`: Must be present (even if null)
- `status`: `"incomplete"` while waiting for tool outputs, `"completed"` once resumed

## Critical Requirements

### 1. Consistent IDs

**All events must use the same IDs**:
- `item.id` from `output_item.added` → used as `item_id` in all delta/done events
- `call_id` must be consistent across all events
- Both IDs must appear in final `response.completed`

Example validation:
```bash
# All item_ids should be identical
grep -oP '"item_id":"[^"]+' output.txt | cut -d'"' -f4 | sort -u | wc -l
# Expected: 1

# All call_ids should be identical
grep -oP '"call_id":"[^"]+' output.txt | cut -d'"' -f4 | sort -u | wc -l
# Expected: 1
```

### 2. Status Progression

```
output_item.added:  status = "in_progress"
output_item.done:   status = "completed"
response.completed: status = "completed"
```

### 3. Arguments Accumulation

The `arguments` field must progress:
```
output_item.added:              arguments = ""
function_call_arguments.delta:  delta = (chunks)
function_call_arguments.done:   arguments = (full JSON)
output_item.done:               arguments = (full JSON)
response.completed:             arguments = (full JSON)
```

### 4. Type Consistency

All events must have `type: "function_call"`:
```
output_item.added:      item.type = "function_call"
output_item.done:       item.type = "function_call"
response.completed:     output[0].type = "function_call"
```

### 5. Sequence Numbers

Must auto-increment on every event (including ping):
```
response.created:                    sequence_number: 0
response.output_item.added:          sequence_number: 1
response.function_call_arguments.*:  sequence_number: 2+
ping:                                sequence_number: (increments)
response.completed:                  sequence_number: (final)
```

## Differences from Text Responses

| Field | Text Response | Tool Call Response |
|-------|--------------|-------------------|
| `output_item.added.item.type` | "message" | **"function_call"** |
| `output_item.added.item` fields | `role`, `content` | `name`, `call_id`, `arguments` |
| Delta event | `response.output_text.delta` | `response.function_call_arguments.delta` |
| Done event | `response.output_text.done` | `response.function_call_arguments.done` |
| Content part events | Yes (added/done) | **No** (not used for tool calls) |

## Testing Tool Call Format

Use the validation script:

```bash
bash tests/test_streaming_detailed.sh
```

Expected output:
```
✓ All events use same item_id
✓ All events use same call_id
✓ Function name correct in both events
✓ output_item.added status: in_progress
✓ output_item.done status: completed
✓ Arguments match in done and completed events
✓ All type fields are function_call
```

## Implementation in Go

```go
// Generate IDs
itemID := fmt.Sprintf("fc_%d", time.Now().UnixNano())
callID := fmt.Sprintf("call_%d", time.Now().UnixNano())
toolName := "shell"  // from tool definition

// 1. Emit output_item.added with type=function_call
emit("response.output_item.added", map[string]any{
    "type": "response.output_item.added",
    "output_index": 0,
    "item": map[string]any{
        "id":        itemID,
        "type":      "function_call",
        "status":    "in_progress",
        "name":      toolName,
        "call_id":   callID,
        "arguments": "",
    },
})

// 2. Emit function_call_arguments.delta for each chunk
var argsBuilder strings.Builder
for _, argChunk := range argumentChunks {
    emit("response.function_call_arguments.delta", map[string]any{
        "type":         "response.function_call_arguments.delta",
        "item_id":      itemID,
        "output_index": 0,
        "delta":        argChunk,
    })
    argsBuilder.WriteString(argChunk)
}

// 3. Emit function_call_arguments.done with complete arguments
completeArgs := argsBuilder.String()
emit("response.function_call_arguments.done", map[string]any{
    "type":         "response.function_call_arguments.done",
    "item_id":      itemID,
    "output_index": 0,
    "arguments":    completeArgs,
})

// 4. Emit output_item.done with status=completed
emit("response.output_item.done", map[string]any{
    "type":         "response.output_item.done",
    "output_index": 0,
    "item": map[string]any{
        "id":        itemID,
        "type":      "function_call",
        "status":    "completed",
        "name":      toolName,
        "call_id":   callID,
        "arguments": completeArgs,
    },
})

// 5. Emit response.completed with output array
emit("response.completed", map[string]any{
    "type": "response.completed",
    "response": map[string]any{
        // ... response fields ...
        "output": []any{
            map[string]any{
                "id":        itemID,
                "type":      "function_call",
                "status":    "completed",
                "name":      toolName,
                "call_id":   callID,
                "arguments": completeArgs,
            },
        },
        "reasoning": map[string]any{
            "effort":  nil,
            "summary": nil,
        },
    },
})
```

## Key Mappings from Anthropic to OpenAI

| Anthropic | OpenAI Response API |
|-----------|-------------------|
| `content_block_start` (tool_use) | `response.output_item.added` (function_call) |
| `content_block_delta` (input_json_delta) | `response.function_call_arguments.delta` |
| `message_delta` (stop_reason: tool_use) | Triggers `.done` events |
| `tool_use.id` | `item.call_id` |
| `tool_use.name` | `item.name` |
| `tool_use.input` | `item.arguments` (as JSON string) |

## Validation Checklist

- [ ] All events have `type` field
- [ ] All events have `sequence_number` (auto-incrementing)
- [ ] All delta/done events have `item_id`
- [ ] Same `item_id` used across all events
- [ ] Same `call_id` used across all events
- [ ] `type: "function_call"` in all item objects
- [ ] Status transitions: in_progress → completed
- [ ] Arguments progress: "" → deltas → complete JSON
- [ ] Function name consistent in all events
- [ ] Final `response.completed` includes complete output array
- [ ] `reasoning` field present in response objects

## References

- Implementation: `internal/httpserver/server.go:427-670`
- Anthropic Adapter: `internal/adapter/anthropic/anthropic.go:343-369`
- Test Suite: `tests/test_streaming_detailed.sh`
- Format docs: `docs/openai-response-api.md`
