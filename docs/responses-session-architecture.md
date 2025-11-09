# Responses Session-Aware Streaming Architecture

**Version**: v0.3.0
**Status**: Current architecture documentation

This document describes the session-aware streaming architecture that keeps `/v1/responses` streaming connections open across tool-call cycles while routing Anthropic traffic through a layered translation stack. It focuses on the runtime flow that Codex CLI v0.55.0+ exercises today.

## Goals

1. **Single SSE connection per conversation** – keep the HTTP stream alive even when the model pauses for `required_action.submit_tool_outputs`.
2. **Session-aware orchestration** – persist chat state, tool calls, and writers so `/submit_tool_outputs` can resume the right stream.
3. **Layered translation** – split ingress/orchestrator/provider responsibilities so Anthropic (and future providers) plug in via a common interface.

## Layered Model

```
OpenAI Responses ingress
        │
        ▼
Conversation (Base + Chat clone)
        │
        ▼
StreamProvider (Anthropic, OpenAI passthrough, etc.)
        │
        ▼
SSE Orchestrator (responses_stream.go)
        │
        ▼
Codex / Client
```

### Conversation Canonical Struct

File: `internal/httpserver/responses/conversation.go`

- `Conversation.Base` holds the canonical OpenAI Responses request (instructions, tools, response_format).
- `Conversation.Chat` is a deep clone of the derived `openai.ChatCompletionRequest`; cloning avoids sharing message/tool slices across runs.
- Helpers (`WithChat`, `WithBase`, `StructuredJSON`, `EnsureID`) allow orchestration to mutate state safely.

### StreamProvider Interface

File: `internal/httpserver/responses/provider.go`

```go
type StreamProvider interface {
    Stream(ctx context.Context, conv Conversation) (StreamInit, error)
}
```

Providers encapsulate upstream specifics (HTTP URLs, auth headers, streaming translators). They return a channel of `adapter.StreamEvent` plus an optional cleanup callback. This keeps `responses_stream` agnostic of Anthropic vs OpenAI vs future MCP targets.

#### Anthropic Implementation

File: `internal/httpserver/responses/provider_anthropic.go`

- Converts the cloned chat request into an Anthropic `NativeRequest` via the existing translator.
- Applies token guards, sets `anthropic-version`, `x-api-key`, and streams SSE back through `StreamNativeToOpenAI`.
- Emits `adapter.StreamEvent` values that the orchestrator already knows how to convert into OpenAI Responses SSE events.

## SSE Orchestrator Flow

File: `internal/httpserver/responses_stream.go`

1. **Session bootstrap** – assign `respID`, emit `response.created`, and (if routed through a translation adapter) store a `responseSession` containing the cloned chat request and base payload.
2. **Streaming loop** – for each iteration:
   - Build `Conversation` snapshot and call `StreamProvider.Stream`.
   - Stream Anthropic chunks into OpenAI Responses events. Tool-call arguments accumulate as before.
   - When finish_reason = `tool_calls`, emit `response.required_action` followed by an incomplete `response.completed` (with the same `required_action`) but **do not** close the HTTP response.
3. **Wait for tool outputs** – block on `waitForToolOutputs`, which receives data from `/v1/responses/{id}/submit_tool_outputs`. The handler simply pushes outputs into the session channel; no extra requests required.
4. **Resume** – apply the tool outputs to the stored chat request (`applyToolOutputsToSession`) and loop back to step 2 with a fresh `Conversation`. The same SSE writer is reused.
5. **Completion** – when the upstream returns a non-tool finish reason, emit the final `response.completed`, send `[DONE]`, and clear the session.

Error paths (provider failure, context cancellation, missing session) emit `error` events, flush pending data, and tear down the session to avoid leaks.

## Submitting Tool Outputs

File: `internal/httpserver/responses_handler.go` (`handleResponsesToolOutputs`)

- `deliverToolOutputs` pushes outputs into the session’s channel. If the session is missing or already closed, the handler returns `404` / `400`.
- The streaming loop waits on this channel and resumes once outputs arrive.
- Because the HTTP stream remains open, Codex can immediately continue sending tool results without polling or reconnecting.

## Testing Strategy

1. **Unit tests** – `internal/httpserver/responses_stream_test.go` contains comprehensive streaming tests. Note: `TestStreamResponses_WaitsForToolOutputs` was removed in v0.3.0 (see CHANGELOG.md).
2. **Go test suite** – `go test ./...` validates all packages and translator logic.
3. **Integration tests** – See `tests/integration/tool_calls/` for comprehensive tool call flow tests.
4. **Manual curl flow (recommended)**:
   ```bash
   make gd-force-restart
   tail -f logs/dev-gatewayd-*.log &
   curl -N http://localhost:8081/v1/responses -H 'Content-Type: application/json' -d @tests/fixtures/tool_call_req.json
   # After the SSE shows required_action, submit tool outputs:
   curl -X POST http://localhost:8081/v1/responses/<resp_id>/submit_tool_outputs \
     -H 'Content-Type: application/json' \
     -d '{"tool_outputs":[{"tool_call_id":"call_...","output":"{...}"}]}'
   ```
   Expect the original curl to continue streaming the follow-up assistant reply without reconnecting. Logs should show `responses.stream resuming after tool outputs count=1`.

## Current Limitations (v0.3.0)

### 1. In-Memory Session Storage

**Code Location**: `internal/httpserver/responses_handler.go`

**Limitation**: Sessions are stored in-memory only

**Impact**:
- Gateway restart clears all pending tool calls
- No persistence across restarts
- Codex must retry from scratch after gateway restart

**Workaround**: Use Docker for stable deployments, implement graceful shutdown

**Future**: Persistent session store (Redis/SQLite) planned for v0.4.0+

### 2. Tool Adapter Filtering

**Code Location**: `internal/httpserver/tool_adapter/adapter.go`

**Limitation**: Some Codex tools are filtered when using Anthropic

**Filtered tools**:
- `apply_patch` - Not supported by Anthropic
- `update_plan` - Not supported by Anthropic

**Workaround**: Gateway injects guidance to use shell alternatives

### 3. Duplicate Detection Heuristics

**Code Location**: `internal/httpserver/responses_handler.go:detectDuplicateToolCalls()`

**Limitation**: Detection relies on content matching

**Thresholds**:
- 3 duplicates = warning
- 5 duplicates = emergency stop

**May miss duplicates if**: Output format changes slightly between attempts

**Not configurable**: Thresholds are hardcoded

### 4. Delegation Mode Streaming

**Code Location**: `internal/httpserver/responses_handler.go`

**Limitation**: When using delegation mode (gpt*/o1* models), streaming is disabled

**Logged as**: "responses: disabling stream for openai delegation"

**Reason**: Avoids unsupported streaming behavior with OpenAI delegation

**Workaround**: Use non-streaming mode for OpenAI models or translation mode for Anthropic models

## Extensibility Notes

- Adding a new provider (e.g., OpenAI delegation, future MCP gateway) only requires implementing `StreamProvider` and optionally reusing `Conversation` helpers.
- Session management already stores adapters per response, so mixed routing (Anthropic ↔ OpenAI ↔ passthrough) can coexist.
- Future work: extract ingress parsing & egress encoding into explicit interfaces so non-OpenAI protocols can reuse the same orchestrator.

## Related Documentation

- [Tool Call Translation](tool-call-translation.md) - Detailed translation between Anthropic and OpenAI tool formats
- [Codex Integration Guide](codex-to-anthropic.md) - Using Codex CLI with Anthropic models via gateway
- [Claude Code Integration Guide](claude_code-to-openai.md) - Using Claude Code with OpenAI models via gateway
