# OpenAI Responses API SSE Implementation Guide

## Overview

The OpenAI Responses API is a newer API endpoint (March 2025) that provides richer streaming semantics compared to Chat Completions API. This document details the **exact** SSE format required for compatibility with OpenAI Codex CLI and other clients.

**Critical Discovery**: Through testing with real OpenAI API and debugging Codex CLI v0.55.0, we found that **every field matters**. Missing or incorrectly formatted fields cause client-side errors like "ReasoningSummaryDelta without active item" and "stream disconnected before completion".

## Key Differences from Chat Completions API

| Feature | Chat Completions API | Responses API |
|---------|---------------------|---------------|
| Endpoint | `/v1/chat/completions` | `/v1/responses` |
| Request Key | `messages` | `input` |
| SSE Format | `data: {...}` only | `event: xxx` + `data: {...}` |
| Event Types | Single chunk format | 20+ semantic event types |
| Sequence Numbers | ❌ Not required | ✅ **Required on every event** |
| Item IDs | ❌ Not applicable | ✅ **Required on delta/done events** |
| Termination | `data: [DONE]` | `event: response.completed` |

## Complete Event Sequence

### Required Event Flow for Text Response

```
1. response.created                    # Stream start, full response object
2. response.output_item.added          # Output item begins (with id, status, role)
3. response.content_part.added         # Content part begins (with type: output_text)
4. response.output_text.delta          # Text deltas (multiple, with item_id)
5. response.output_text.done           # Text complete (with full text, item_id)
6. response.content_part.done          # Content part complete (with item_id, full part)
7. response.output_item.done           # Item complete (with full item)
8. response.completed                  # Stream complete (with output array, reasoning)
```

Plus `ping` events every 2-6 seconds during generation (critical for HTTP keepalive).

## Critical Fields Required on All Events

### 1. `type` Field
**Every event payload MUST include a `type` field** matching the event name:
```json
{
  "type": "response.output_text.delta",
  ...other fields
}
```

### 2. `sequence_number` Field
**Every event MUST include an auto-incrementing `sequence_number`**:
```json
{
  "type": "response.created",
  "sequence_number": 0,
  ...
}
```
Start at 0, increment by 1 for each event (including ping events).

### 3. `item_id` Field
**All delta and done events MUST include `item_id`**:
```json
{
  "type": "response.output_text.delta",
  "sequence_number": 4,
  "item_id": "msg_1762398047422434553",
  ...
}
```
Generated when `response.output_item.added` is emitted, then used in all subsequent events.

## Detailed Event Structures

### 1. response.created

**First event in stream**. Must include full response object with `reasoning` field:

```json
event: response.created
data: {
  "type": "response.created",
  "sequence_number": 0,
  "response": {
    "id": "resp_1762398047422434553",
    "object": "response",
    "created_at": 1762398047,
    "status": "in_progress",
    "model": "claude-3-5-sonnet-20241022",
    "reasoning": {
      "effort": null,
      "summary": null
    }
  }
}
```

**Key Points**:
- `response` is nested object (not top-level)
- `reasoning` field is **required** (even if null)
- `status`: "in_progress" for created, "completed"/"failed" for final

### 2. response.output_item.added

**Before first content**. Establishes the message item with full structure:

```json
event: response.output_item.added
data: {
  "type": "response.output_item.added",
  "sequence_number": 1,
  "output_index": 0,
  "item": {
    "id": "msg_1762398048023087442",
    "type": "message",
    "status": "in_progress",
    "content": [],
    "role": "assistant"
  }
}
```

**Key Points**:
- `item.id` is critical - save this for use in all subsequent events
- `content` starts as empty array
- `role`: "assistant" for model output
- `output_index`: 0 for single output (future: multiple outputs)

### 3. response.content_part.added

**Before first text delta**. Establishes the text content part:

```json
event: response.content_part.added
data: {
  "type": "response.content_part.added",
  "sequence_number": 2,
  "item_id": "msg_1762398048023087442",
  "output_index": 0,
  "content_index": 0,
  "part": {
    "type": "output_text",
    "annotations": [],
    "logprobs": [],
    "text": ""
  }
}
```

**Key Points**:
- **`item_id`** must match the id from `output_item.added`
- `part.type`: "output_text" for text content
- `content_index`: 0 for first content part
- `annotations`, `logprobs`: empty arrays required

### 4. response.output_text.delta

**For each piece of text**. Multiple events with incremental content:

```json
event: response.output_text.delta
data: {
  "type": "response.output_text.delta",
  "sequence_number": 3,
  "item_id": "msg_1762398048023087442",
  "output_index": 0,
  "content_index": 0,
  "delta": "Hello",
  "logprobs": []
}
```

**Key Points**:
- **`item_id`** is critical - without it, Codex shows "ReasoningSummaryDelta without active item"
- `delta`: the incremental text
- `logprobs`: empty array (or actual logprobs if enabled)
- Sent multiple times as text is generated

### 5. response.output_text.done

**After all deltas**. Includes complete text:

```json
event: response.output_text.done
data: {
  "type": "response.output_text.done",
  "sequence_number": 10,
  "item_id": "msg_1762398048023087442",
  "output_index": 0,
  "content_index": 0,
  "text": "Hello! How can I assist you today?",
  "logprobs": []
}
```

**Key Points**:
- `text`: **full accumulated text** (not just last delta)
- Must include `item_id`

### 6. response.content_part.done

**After text done**. Includes complete part object:

```json
event: response.content_part.done
data: {
  "type": "response.content_part.done",
  "sequence_number": 11,
  "item_id": "msg_1762398048023087442",
  "output_index": 0,
  "content_index": 0,
  "part": {
    "type": "output_text",
    "annotations": [],
    "logprobs": [],
    "text": "Hello! How can I assist you today?"
  }
}
```

**Key Points**:
- `part` must include full text
- Must match structure from `content_part.added`

### 7. response.output_item.done

**After content parts done**. Includes complete item with all content:

```json
event: response.output_item.done
data: {
  "type": "response.output_item.done",
  "sequence_number": 12,
  "output_index": 0,
  "item": {
    "id": "msg_1762398048023087442",
    "type": "message",
    "status": "completed",
    "content": [
      {
        "type": "output_text",
        "annotations": [],
        "logprobs": [],
        "text": "Hello! How can I assist you today?"
      }
    ],
    "role": "assistant"
  }
}
```

**Key Points**:
- `status`: "completed" (was "in_progress" in added event)
- `content`: array with full content parts
- Must include same `id` as in added event

### 8. response.completed

**Final event**. Includes complete response with output array:

```json
event: response.completed
data: {
  "type": "response.completed",
  "sequence_number": 13,
  "response": {
    "id": "resp_1762398047422434553",
    "object": "response",
    "created_at": 1762398047,
    "status": "completed",
    "model": "claude-3-5-sonnet-20241022",
    "output": [
      {
        "id": "msg_1762398048023087442",
        "type": "message",
        "status": "completed",
        "content": [
          {
            "type": "output_text",
            "annotations": [],
            "logprobs": [],
            "text": "Hello! How can I assist you today?"
          }
        ],
        "role": "assistant"
      }
    ],
    "reasoning": {
      "effort": null,
      "summary": null
    }
  }
}
```

**Key Points**:
- `output`: **complete array of all output items** (not just IDs)
- `reasoning`: must be present
- `status`: "completed" or "failed"
- This event is what Codex waits for to confirm stream completion

### 9. ping Events

**HTTP Keepalive**. Sent periodically during generation:

```json
event: ping
data: {}
```

**Critical Importance**:
- **Required for Codex CLI** - prevents "stream disconnected before completion"
- See OpenAI Codex issue #3267: "prevent idle disconnects via HTTP keepalives + heartbeats"
- Send every 2-6 seconds during generation
- Even empty ping events keep the HTTP connection alive

## Implementation in Go

### Complete Working Example

```go
// 1. Set SSE headers BEFORE any writes
w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
flusher, _ := w.(http.Flusher)

// 2. Initialize state
var sequenceNum int
var itemID string
var fullText strings.Builder

// 3. Define emit helper with auto sequence_number
emit := func(event string, payload any) {
    fmt.Fprintf(w, "event: %s\n", event)
    if payload != nil {
        if m, ok := payload.(map[string]any); ok {
            m["sequence_number"] = sequenceNum
            sequenceNum++
        }
        b, _ := json.Marshal(payload)
        fmt.Fprintf(w, "data: %s\n\n", string(b))
    } else {
        fmt.Fprint(w, "data: {}\n\n")
    }
    if flusher != nil { flusher.Flush() }
}

// 4. Generate IDs
respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
respCreated := time.Now().Unix()

// 5. Emit response.created
emit("response.created", map[string]any{
    "type": "response.created",
    "response": map[string]any{
        "id": respID,
        "object": "response",
        "status": "in_progress",
        "created_at": respCreated,
        "model": model,
        "reasoning": map[string]any{
            "effort": nil,
            "summary": nil,
        },
    },
})

// 6. Setup ping ticker (IMPORTANT for Codex)
ticker := time.NewTicker(2 * time.Second)
defer ticker.Stop()

// 7. On first delta, emit output_item.added and content_part.added
itemID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
emit("response.output_item.added", map[string]any{
    "type": "response.output_item.added",
    "output_index": 0,
    "item": map[string]any{
        "id": itemID,
        "type": "message",
        "status": "in_progress",
        "content": []any{},
        "role": "assistant",
    },
})

emit("response.content_part.added", map[string]any{
    "type": "response.content_part.added",
    "item_id": itemID,
    "output_index": 0,
    "content_index": 0,
    "part": map[string]any{
        "type": "output_text",
        "annotations": []any{},
        "logprobs": []any{},
        "text": "",
    },
})

// 8. Stream loop with ping events
for {
    select {
    case delta, ok := <-deltaChan:
        if !ok { break }

        // Emit delta with item_id
        emit("response.output_text.delta", map[string]any{
            "type": "response.output_text.delta",
            "item_id": itemID,
            "output_index": 0,
            "content_index": 0,
            "delta": delta,
            "logprobs": []any{},
        })
        fullText.WriteString(delta)

    case <-ticker.C:
        // Emit ping for keepalive
        emit("ping", map[string]any{})
    }
}

// 9. Finalize
completeText := fullText.String()

emit("response.output_text.done", map[string]any{
    "type": "response.output_text.done",
    "item_id": itemID,
    "output_index": 0,
    "content_index": 0,
    "text": completeText,
    "logprobs": []any{},
})

emit("response.content_part.done", map[string]any{
    "type": "response.content_part.done",
    "item_id": itemID,
    "output_index": 0,
    "content_index": 0,
    "part": map[string]any{
        "type": "output_text",
        "annotations": []any{},
        "logprobs": []any{},
        "text": completeText,
    },
})

emit("response.output_item.done", map[string]any{
    "type": "response.output_item.done",
    "output_index": 0,
    "item": map[string]any{
        "id": itemID,
        "type": "message",
        "status": "completed",
        "content": []any{
            map[string]any{
                "type": "output_text",
                "annotations": []any{},
                "logprobs": []any{},
                "text": completeText,
            },
        },
        "role": "assistant",
    },
})

emit("response.completed", map[string]any{
    "type": "response.completed",
    "response": map[string]any{
        "id": respID,
        "object": "response",
        "created_at": respCreated,
        "status": "completed",
        "model": model,
        "output": []any{
            map[string]any{
                "id": itemID,
                "type": "message",
                "status": "completed",
                "content": []any{
                    map[string]any{
                        "type": "output_text",
                        "annotations": []any{},
                        "logprobs": []any{},
                        "text": completeText,
                    },
                },
                "role": "assistant",
            },
        },
        "reasoning": map[string]any{
            "effort": nil,
            "summary": nil,
        },
    },
})

// 10. Final flush with delay (CRITICAL)
if flusher != nil { flusher.Flush() }
time.Sleep(50 * time.Millisecond)
```

## Common Errors and Fixes

### Error: "ReasoningSummaryDelta without active item"

**Cause**: Missing `item_id` field in delta events

**Fix**: Add `item_id` to all `response.output_text.delta`, `.done`, `content_part.done` events

```diff
emit("response.output_text.delta", map[string]any{
    "type": "response.output_text.delta",
+   "item_id": itemID,  // REQUIRED
    "output_index": 0,
    "delta": text,
})
```

### Error: "stream disconnected before completion"

**Possible Causes**:
1. Missing `ping` events
2. Missing `response.completed` event
3. TCP buffers not flushed before handler returns

**Fix**:
```go
// 1. Add ping ticker
ticker := time.NewTicker(2 * time.Second)
defer ticker.Stop()

// 2. Always emit response.completed
emit("response.completed", ...)

// 3. Add delay before return
if flusher != nil { flusher.Flush() }
time.Sleep(50 * time.Millisecond)
```

### Error: Events not recognized by client

**Cause**: Missing `sequence_number` or `type` fields

**Fix**: Ensure every event has both fields:
```go
{
    "type": "response.output_text.delta",  // REQUIRED
    "sequence_number": 4,                  // REQUIRED
    ...
}
```

### Error: Missing content_part events

**Symptom**: Stream works but Codex shows errors on every delta

**Cause**: Must emit both `content_part.added` and `content_part.done`

**Fix**:
```go
// Before first delta
emit("response.content_part.added", ...)

// After all deltas
emit("response.content_part.done", ...)
```

## Tool Calls (TODO)

Tool call events use function call argument deltas:

```json
event: response.function_call_arguments.delta
data: {
  "type": "response.function_call_arguments.delta",
  "sequence_number": 5,
  "item_id": "msg_...",
  "output_index": 0,
  "name": "calculate",
  "arguments": "{\"expression\": \"5+6\"}"
}
```

**Status**: Basic text streaming works. Tool calls need additional implementation.

## Testing

### Test with curl

```bash
curl -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hi"}],
    "stream": true
  }'
```

### Compare with Real OpenAI API

```bash
curl -N https://api.openai.com/v1/responses \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "input": [{"role": "user", "content": "hi"}],
    "stream": true
  }'
```

Look for:
- `sequence_number` on every event
- `item_id` on all delta/done events
- `reasoning` field in response objects
- Complete `output` array in response.completed

## References

- OpenAI Responses API Docs: https://platform.openai.com/docs/api-reference/responses
- Codex CLI Issue #3267: https://github.com/openai/codex/issues/3267
- SSE Specification: https://html.spec.whatwg.org/multipage/server-sent-events.html
- Gateway Implementation: `internal/httpserver/server.go`

## Version History

- **2025-11-06**: Complete rewrite with all required fields from real API testing
  - Added `sequence_number`, `item_id`, `reasoning` fields
  - Added `content_part.added` and `content_part.done` events
  - Documented Codex CLI requirements and ping events importance
- **2025-11-05**: Initial Responses API implementation
