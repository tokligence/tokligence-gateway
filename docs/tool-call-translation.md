# Tool Call Translation: Anthropic ↔ OpenAI Response API

**Version**: v0.3.0
**Status**: Updated with current architecture

This document explains how tool calls are translated between Anthropic's Message API and OpenAI's Response API in the tokligence-gateway.

## ⚠️ Architecture Update Notice (v0.3.0)

**File paths in this document have been updated for v0.3.0 architecture.**

### Current Architecture Components (v0.3.0)

| Component | File Location | Purpose |
|-----------|---------------|---------|
| **Responses API Handler** | `internal/httpserver/endpoint_responses.go`<br/>`internal/httpserver/responses_handler.go` | Main entry point for `/v1/responses` |
| **SSE Streaming** | `internal/httpserver/responses_stream.go` | Converts provider responses to OpenAI Responses API SSE events |
| **Provider Abstraction** | `internal/httpserver/responses/provider.go`<br/>`internal/httpserver/responses/provider_anthropic.go` | Abstract interface for different providers (Anthropic, OpenAI) |
| **Anthropic Translator** | `internal/httpserver/anthropic/native.go`<br/>`internal/httpserver/anthropic/stream.go` | OpenAI ↔ Anthropic format conversion |
| **Tool Adapter** | `internal/httpserver/tooladapter/adapter.go` | Filters unsupported tools (`apply_patch`, `update_plan`) |
| **Conversation Builder** | `internal/httpserver/responses/conversation.go` | Builds canonical conversation from sessions |

### Deprecated Paths (Pre-v0.3.0)

The following paths are **no longer valid** in v0.3.0:
- ❌ `internal/adapter/anthropic/anthropic.go` → Use `internal/httpserver/anthropic/native.go`
- ❌ `internal/sidecar/*` → Merged into main server
- ❌ Direct Anthropic adapter in `internal/adapter/` → Moved to `internal/httpserver/anthropic/`

## Overview

The gateway acts as a bridge:
```
Codex CLI (OpenAI Response API) → Gateway → Anthropic API
                                     ↓
                          Provider Abstraction Layer
                          Tool Adapter (filters unsupported tools)
                          Translation & SSE Streaming
```

## 1. Request Flow: OpenAI Response API → Anthropic

### Input: OpenAI Response API Request
```json
{
  "model": "gpt-4o-mini",
  "input": [{"role": "user", "content": "What is 5+6?"}],
  "tools": [{
    "type": "function",
    "name": "calculate",
    "description": "Perform calculation",
    "parameters": {
      "type": "object",
      "properties": {
        "expression": {"type": "string"}
      }
    }
  }],
  "stream": true
}
```

### Conversion Step 1: Response API → Chat Completions Format
**File**: `internal/httpserver/server.go` (line ~280-340)

OpenAI Response API is first converted to Chat Completions format:
```go
// Convert "input" to "messages"
creq.Messages = rr.Input

// Convert tools from Response API to Chat Completions format
for _, tool := range rr.Tools {
    creq.Tools = append(creq.Tools, openai.Tool{
        Type: tool.Type,
        Function: openai.FunctionDefinition{
            Name: tool.Name,
            Description: tool.Description,
            Parameters: tool.Parameters,
        },
    })
}
```

### Conversion Step 2: Chat Completions → Anthropic Format
**File**: `internal/adapter/anthropic/anthropic.go` (line ~109-116)

```go
func convertTools(tools []openai.Tool) []anthropicTool {
    var result []anthropicTool
    for _, tool := range tools {
        if tool.Type != "function" {
            continue // Anthropic only supports function tools
        }
        result = append(result, anthropicTool{
            Name: tool.Function.Name,
            Description: tool.Function.Description,
            InputSchema: tool.Function.Parameters,
        })
    }
    return result
}
```

### Output: Anthropic API Request
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [{"role": "user", "content": "What is 5+6?"}],
  "tools": [{
    "name": "calculate",
    "description": "Perform calculation",
    "input_schema": {
      "type": "object",
      "properties": {
        "expression": {"type": "string"}
      }
    }
  }],
  "stream": true
}
```

## 2. Response Flow: Anthropic → OpenAI Response API

### Anthropic SSE Events (from Anthropic API)

When Anthropic wants to call a tool, it emits these events:

**Event 1: content_block_start** (tool_use type)
```json
{
  "type": "content_block_start",
  "index": 0,
  "content_block": {
    "type": "tool_use",
    "id": "toolu_01ABC123",
    "name": "calculate"
  }
}
```

**Event 2: content_block_delta** (partial JSON arguments)
```json
{
  "type": "content_block_delta",
  "index": 0,
  "delta": {
    "type": "input_json_delta",
    "partial_json": "{\"expression\":\""
  }
}
```

**Event 3: content_block_delta** (more arguments)
```json
{
  "type": "content_block_delta",
  "index": 0,
  "delta": {
    "type": "input_json_delta",
    "partial_json": "5+6"
  }
}
```

**Event 4: content_block_delta** (final arguments)
```json
{
  "type": "content_block_delta",
  "index": 0,
  "delta": {
    "type": "input_json_delta",
    "partial_json": "\"}"
  }
}
```

**Event 5: content_block_stop**
```json
{
  "type": "content_block_stop",
  "index": 0
}
```

**Event 6: message_delta** (finish_reason)
```json
{
  "type": "message_delta",
  "delta": {
    "stop_reason": "tool_use"
  }
}
```

### Conversion Step 1: Anthropic SSE → Chat Completions Deltas
**File**: `internal/adapter/anthropic/anthropic.go` (line ~285-310)

The Anthropic adapter converts tool_use events to OpenAI Chat Completions format:

```go
// Track tool_use content blocks by index
type toolState struct { id, name string }
toolBlocks := map[int]*toolState{}

// Handle tool_use start
if evt.Type == "content_block_start" && evt.ContentBlock.Type == "tool_use" {
    ts := &toolState{id: evt.ContentBlock.ID, name: evt.ContentBlock.Name}
    toolBlocks[evt.Index] = ts

    // Emit first delta with tool name
    delta.ToolCalls = []openai.ToolCallDelta{{
        Index: &evt.Index,
        ID: &ts.id,
        Type: "function",
        Function: &openai.ToolFunctionPart{Name: ts.name},
    }}
}

// Handle tool_use input JSON partial deltas
if evt.Type == "content_block_delta" && evt.Delta.Type == "input_json_delta" {
    if _, ok := toolBlocks[evt.Index]; ok {
        // Emit delta with partial JSON arguments
        delta.ToolCalls = []openai.ToolCallDelta{{
            Index: &evt.Index,
            Function: &openai.ToolFunctionPart{Arguments: evt.Delta.PartialJSON},
        }}
    }
}
```

**Output**: `openai.ChatCompletionChunk` with ToolCallDelta
```go
ChatCompletionChunk{
    Choices: []Choice{{
        Delta: Delta{
            ToolCalls: []ToolCallDelta{{
                Index: 0,
                ID: "toolu_01ABC123",
                Type: "function",
                Function: &ToolFunctionPart{
                    Name: "calculate",
                    Arguments: "{\"expression\":\"5+6\"}",
                },
            }},
        },
        FinishReason: "tool_calls",
    }},
}
```

### Conversion Step 2: Chat Completions Deltas → OpenAI Response API SSE
**File**: `internal/httpserver/server.go` (line ~627-695)

The gateway converts Chat Completions deltas to OpenAI Response API format:

```go
// State tracking
toolCallOutputItemAdded := false
var toolCallItemID string
var toolCallName string
var toolCallID string
var toolCallArgs strings.Builder

// When first tool call delta arrives
if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
    for _, tcd := range chunk.Choices[0].Delta.ToolCalls {
        // Emit output_item.added before first tool call delta
        if !toolCallOutputItemAdded {
            toolCallItemID = fmt.Sprintf("fc_%d", time.Now().UnixNano())
            toolCallID = fmt.Sprintf("call_%d", time.Now().UnixNano())
            if tcd.Function != nil && tcd.Function.Name != "" {
                toolCallName = tcd.Function.Name
            }
            emit("response.output_item.added", map[string]any{
                "type": "response.output_item.added",
                "output_index": 0,
                "item": map[string]any{
                    "id": toolCallItemID,           // "fc_1234567890"
                    "type": "function_call",
                    "status": "in_progress",
                    "arguments": "",
                    "call_id": toolCallID,          // "call_1234567890"
                    "name": toolCallName,           // "calculate"
                },
            })
            toolCallOutputItemAdded = true
        }

        // Emit function_call_arguments.delta with item_id
        if tcd.Function != nil && tcd.Function.Arguments != "" {
            argsChunk := tcd.Function.Arguments
            emit("response.function_call_arguments.delta", map[string]any{
                "type": "response.function_call_arguments.delta",
                "item_id": toolCallItemID,      // CRITICAL: links delta to item
                "output_index": 0,
                "delta": argsChunk,             // Partial JSON: "{\"expr..."
            })
            toolCallArgs.WriteString(argsChunk)
        }
    }
}

// When finish_reason == "tool_calls"
if fr == "tool_calls" {
    completeArgs := toolCallArgs.String()
    // Emit function_call_arguments.done
    emit("response.function_call_arguments.done", map[string]any{
        "type": "response.function_call_arguments.done",
        "item_id": toolCallItemID,
        "output_index": 0,
        "arguments": completeArgs,          // Complete JSON: "{\"expression\":\"5+6\"}"
    })
    // Emit output_item.done
    emit("response.output_item.done", map[string]any{
        "type": "response.output_item.done",
        "output_index": 0,
        "item": map[string]any{
            "id": toolCallItemID,
            "type": "function_call",
            "status": "completed",
            "arguments": completeArgs,
            "call_id": toolCallID,
            "name": toolCallName,
        },
    })
}
```

### Output: OpenAI Response API SSE Events

**Event 1: response.created**
```json
{
  "type": "response.created",
  "sequence_number": 0,
  "response": {
    "id": "resp_...",
    "status": "in_progress",
    "tools": [{
      "type": "function",
      "name": "calculate",
      "description": "Perform calculation",
      "parameters": {...}
    }]
  }
}
```

**Event 2: response.output_item.added**
```json
{
  "type": "response.output_item.added",
  "sequence_number": 2,
  "output_index": 0,
  "item": {
    "id": "fc_1762399067123456789",
    "type": "function_call",
    "status": "in_progress",
    "arguments": "",
    "call_id": "call_1762399067123456789",
    "name": "calculate"
  }
}
```

**Event 3-N: response.function_call_arguments.delta**
```json
{
  "type": "response.function_call_arguments.delta",
  "sequence_number": 3,
  "item_id": "fc_1762399067123456789",
  "output_index": 0,
  "delta": "{\"expression\":\""
}
```

**Event N+1: response.function_call_arguments.done**
```json
{
  "type": "response.function_call_arguments.done",
  "sequence_number": 10,
  "item_id": "fc_1762399067123456789",
  "output_index": 0,
  "arguments": "{\"expression\":\"5+6\"}"
}
```

**Event N+2: response.output_item.done**
```json
{
  "type": "response.output_item.done",
  "sequence_number": 11,
  "output_index": 0,
  "item": {
    "id": "fc_1762399067123456789",
    "type": "function_call",
    "status": "completed",
    "arguments": "{\"expression\":\"5+6\"}",
    "call_id": "call_1762399067123456789",
    "name": "calculate"
  }
}
```

**Event N+3: response.completed**
```json
{
  "type": "response.completed",
  "sequence_number": 12,
  "response": {
    "id": "resp_...",
    "status": "completed",
    "output": [{
      "id": "fc_1762399067123456789",
      "type": "function_call",
      "status": "completed",
      "arguments": "{\"expression\":\"5+6\"}",
      "call_id": "call_1762399067123456789",
      "name": "calculate"
    }]
  }
}
```

## 3. Key Differences Between APIs

### Tool Definition Format

| Field | Anthropic | OpenAI Response API |
|-------|-----------|-------------------|
| Name | `name` | `name` |
| Description | `description` | `description` |
| Parameters | `input_schema` | `parameters` |

### Tool Use Format

| Field | Anthropic | OpenAI Response API |
|-------|-----------|-------------------|
| ID | `id` (e.g., "toolu_01ABC") | `id` (function_call item)<br>`call_id` (OpenAI-style) |
| Type | `tool_use` | `function_call` |
| Name | `name` | `name` |
| Arguments | `input` (streaming: `partial_json`) | `arguments` (streaming: `delta`) |

### Streaming Events

| Stage | Anthropic Event | OpenAI Response API Event |
|-------|----------------|-------------------------|
| Start | `content_block_start` (type: tool_use) | `response.output_item.added` (type: function_call) |
| Delta | `content_block_delta` (input_json_delta) | `response.function_call_arguments.delta` |
| Done | `content_block_stop` | `response.function_call_arguments.done`<br>`response.output_item.done` |
| Finish | `message_delta` (stop_reason: tool_use) | `response.completed` (finish_reason implicit) |

## 4. Critical Implementation Details

### item_id Field
**CRITICAL**: Every delta/done event MUST include `item_id` to link it to the output item. Without this:
- Codex CLI shows error: "ReasoningSummaryDelta without active item"
- Tool calls won't be recognized

### sequence_number Field
**REQUIRED**: Every event (including ping) must have an auto-incrementing `sequence_number`.

### Event Lifecycle
Tool calls must follow this exact sequence:
1. `response.created` (with tools array)
2. `response.output_item.added` (type: function_call, with call_id)
3. Multiple `response.function_call_arguments.delta` (with item_id)
4. `response.function_call_arguments.done` (complete arguments)
5. `response.output_item.done` (complete function_call item)
6. `response.completed` (output array with complete function_call)

### ID Generation
- `item_id`: Format `fc_<unix_nano>` (function call)
- `call_id`: Format `call_<unix_nano>` (OpenAI-style call ID)
- `id` (response): Format `resp_<random_hex>`

## 5. Common Issues and Fixes

### Issue: Tool calls display as raw JSON in Codex
**Cause**: Missing `item_id` field in delta events
**Fix**: Add `item_id` to all `response.function_call_arguments.delta` and `.done` events

### Issue: "ReasoningSummaryDelta without active item"
**Cause**: Missing `item_id` field
**Fix**: Same as above

### Issue: Tool calls not recognized
**Cause**:
1. Not emitting `response.output_item.added` before first delta
2. Using wrong item type (should be "function_call", not "message")
3. Missing `call_id` field

**Fix**: Emit proper lifecycle events with correct structure

### Issue: Incomplete tool call arguments
**Cause**: Not accumulating arguments from deltas
**Fix**: Use `strings.Builder` to accumulate all deltas, emit complete arguments in `.done` event

## 6. Testing

### Test with real OpenAI API:
```bash
curl -N https://api.openai.com/v1/responses \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "input": [{"role": "user", "content": "What is 5+6?"}],
    "tools": [{
      "type": "function",
      "name": "calculate",
      "description": "Perform calculation",
      "parameters": {
        "type": "object",
        "properties": {
          "expression": {"type": "string"}
        }
      }
    }],
    "stream": true
  }' | tee openai_tool_sse.txt
```

### Test with gateway:
```bash
curl -N http://localhost:8081/v1/responses \
  -H "Authorization: Bearer test" \
  -H "Content-Type: application/json" \
  -d @tool_req.json | tee gateway_tool_sse.txt
```

Compare outputs to verify correct translation.

## 7. Tool Adapter: Filtering and Compatibility (v0.3.0)

**Code Location**: `internal/httpserver/tooladapter/adapter.go`

### Problem: Different Tool Ecosystems

Anthropic and OpenAI have **different tool ecosystems**. Some Codex-specific tools are not supported by Anthropic.

**Codex CLI Built-in Tools** (OpenAI Response API format):
- `apply_patch` - Apply unified diff patches to files (**FILTERED** - not supported by Anthropic)
- `update_plan` - Update execution plan (**FILTERED** - not supported by Anthropic)
- `cat` - Read file contents (✅ Supported)
- `search_files` - Search for files by pattern (✅ Supported)
- `grep` - Search file contents (✅ Supported)
- `ls` - List directory contents (✅ Supported)
- `edit_file` - Edit files with find/replace (✅ Supported)

### Current Gateway Behavior (v0.3.0)

The gateway uses a **Tool Adapter** to filter unsupported tools and inject guidance into system messages.

**Filtering mechanism** (`internal/httpserver/tooladapter/adapter.go:79-125`):
```go
// Filtered tools for openai->anthropic translation
FilteredTools: map[string]bool{
    "apply_patch": true,  // Codex-specific, not supported by Anthropic
    "update_plan": true,  // Codex-specific, not supported by Anthropic
}
```

**Flow**:
```
Codex tools → Tool Adapter → Filtered tools → Anthropic API
  (all)         (removes unsupported)    (supported only)
```

**What happens**:
1. Codex sends all tools including `apply_patch` and `update_plan`
2. Gateway filters out `apply_patch` and `update_plan` before sending to Anthropic
3. Gateway injects guidance into system message telling Claude to use alternatives (e.g., shell instead of apply_patch)
4. Claude only sees supported tools and uses shell/other alternatives
5. Tool calls come back → Gateway converts to OpenAI format
6. Codex receives tool calls and executes them

### Guidance Injection

**Code Location**: `internal/httpserver/tooladapter/adapter.go:127-166`

When tools are filtered, the gateway injects guidance into system messages to inform Claude about alternatives:

**Example for `apply_patch`** (lines 56):
```
⚠️ CRITICAL SYSTEM OVERRIDE - HIGHEST PRIORITY DIRECTIVE ⚠️

TOOL AVAILABILITY AND PRIORITY:
- apply_patch: DISABLED (Priority: 0 - DO NOT USE)
- shell: ENABLED (Priority: 100 - USE THIS INSTEAD)

The apply_patch tool has been PERMANENTLY DISABLED in this environment.

MANDATORY ALTERNATIVE - Use 'shell' tool for ALL file operations:

1. CREATE new files (Priority 100):
   shell: cat > filename.ext << 'EOF'
   [file content]
   EOF

2. MODIFY files (Priority 100):
   shell: sed -i 's/old/new/' filename.ext
   OR rewrite entire file with cat/echo
```

This ensures Claude uses shell commands instead of trying to call the unavailable `apply_patch` tool.

### System Message Cleaning

**Code Location**: `internal/httpserver/tooladapter/adapter.go:169-219`

The tool adapter also **cleans system messages** to remove references to filtered tools:
- Detects mentions of filtered tools in system prompts
- Replaces them with guidance text
- Prevents confusion when Codex's system prompt mentions tools that won't be available

### Tool Choice Adaptation

**Code Location**: `internal/httpserver/tooladapter/adapter.go:222-278`

If Codex requests a specific tool via `tool_choice` that has been filtered:
```json
{"tool_choice": {"type": "function", "function": {"name": "apply_patch"}}}
```

The adapter converts it to `"auto"` to prevent errors:
```json
{"tool_choice": "auto"}
```

### Solution: Tool Name Mapping (Future Enhancement)

To properly handle tool differences, the gateway should implement a tool mapping layer:

```go
// internal/adapter/anthropic/tool_mapper.go (FUTURE)

// ToolMapper maps between OpenAI and Anthropic tool formats
type ToolMapper struct {
    // Map OpenAI tool names to Anthropic equivalents
    nameMap map[string]string
    // Map parameters between formats
    paramMap map[string]func(any) any
}

// Example mapping
var DefaultMapper = ToolMapper{
    nameMap: map[string]string{
        "apply_patch": "apply_diff",     // Example: different name
        "search_files": "find_files",     // Example: different name
        "cat": "read_file",               // Example: different name
        // ... more mappings
    },
}

func (m *ToolMapper) ToAnthropic(openaiTool openai.Tool) anthropicTool {
    name := openaiTool.Function.Name
    if mapped, ok := m.nameMap[name]; ok {
        name = mapped
    }
    return anthropicTool{
        Name: name,
        Description: openaiTool.Function.Description,
        InputSchema: m.mapParameters(openaiTool.Function.Parameters),
    }
}

func (m *ToolMapper) ToOpenAI(toolName string, args string) (string, string) {
    // Reverse mapping
    for openaiName, anthropicName := range m.nameMap {
        if anthropicName == toolName {
            return openaiName, m.mapArgumentsBack(args)
        }
    }
    return toolName, args // Pass-through if no mapping
}
```

### Current Workaround

Since the gateway currently does pass-through, **both APIs must support the same tool names and parameter schemas**. This works when:

1. **Custom tools**: User-defined tools that are consistent across both APIs
2. **Standard tools**: Tools that both APIs recognize (e.g., web search, calculator)
3. **Codex handles execution**: Codex executes the tools, so as long as the tool_call format is correct, it works

### Why It Works Today

Even without explicit mapping, the current implementation works because:

1. **Tool definitions are passed through**: Codex's tool definitions go to Anthropic as-is
2. **Claude is flexible**: Claude can understand and use tools even if they're not in its standard set
3. **Codex executes tools**: Codex doesn't care if Claude "knows" about `apply_patch` - it just needs the tool call back in correct format

### When Mapping Becomes Critical

Mapping becomes necessary when:

1. **Native Anthropic tools**: If Anthropic adds native tools (like web search) with different names
2. **Parameter differences**: If tool parameters need transformation
3. **Optimization**: If Anthropic has better equivalents for certain Codex tools
4. **Tool unavailability**: If certain tools should be blocked or replaced when using Anthropic

### Debugging Tool Issues

To debug tool-related issues:

1. **Check tool definitions in request**:
```bash
# See what tools Codex is sending
curl -v http://localhost:8081/v1/responses ... 2>&1 | grep -A20 "tools"
```

2. **Check Anthropic sees correct tools**:
```bash
# Enable debug logging
export DEBUG=1
./bin/gatewayd
# Look for "converted X tools" messages
```

3. **Check tool call translation**:
```bash
# Compare tool_use from Anthropic with function_call to Codex
# Look for name/arguments differences
```

### Future Enhancement: Smart Tool Mapping

Potential improvements:

1. **Auto-detect tool compatibility**:
```go
func (m *ToolMapper) IsCompatible(tool openai.Tool) bool {
    // Check if Anthropic can handle this tool
}
```

2. **Tool parameter validation**:
```go
func (m *ToolMapper) ValidateParameters(tool anthropicTool) error {
    // Ensure parameters match expected schema
}
```

3. **Bidirectional mapping**:
```go
// Map tool calls back from Anthropic to OpenAI format
func (m *ToolMapper) MapToolCall(anthropicCall ToolUse) openai.ToolCall {
    // Handle name and parameter differences
}
```

## 8. References

- Anthropic Messages API: https://docs.anthropic.com/claude/reference/messages-streaming
- OpenAI Response API: https://platform.openai.com/docs/guides/streaming-responses?api-mode=responses
- Gateway SSE Implementation: `internal/httpserver/server.go` (line ~350-700)
- Anthropic Adapter: `internal/adapter/anthropic/anthropic.go` (line ~240-320)
- Codex CLI Tools: https://github.com/openai/codex (built-in tools documentation)
