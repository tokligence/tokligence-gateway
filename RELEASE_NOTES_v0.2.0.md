# Tokligence Gateway v0.2.0

Date: 2025-10-25

This release strengthens the Anthropic-native endpoint (Claude Code) end-to-end:
streaming tool-bridge support, model alias consistency, an independent passthrough toggle, more tolerant parsing, better diagnostics, and expanded tests and docs.

## Highlights

- Streaming Tool Bridge (Anthropic → OpenAI)
  - When the route resolves to OpenAI and tools are present (declared or `tool_*` blocks), `stream=true` can forward OpenAI SSE text deltas as Anthropic `content_block_delta` events and finishes with `message_stop`.
  - New toggle: `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM` (INI: `openai_tool_bridge_stream`, default: false). For coding‑agent workflows, batch responses are preferred to improve action continuity.
  - First turns that only return `tool_calls` (no text) naturally produce no deltas; after your client posts a `tool_result`, assistant text streams as deltas (when enabled).

- Model Alias Consistency (with wildcards)
  - Adapter selection happens on the original incoming model, then provider-specific model alias rewriting is applied. This prevents alias rewrites (e.g., `claude* → gpt-4o`) from breaking route selection.
  - The tool bridge respects Router model aliases (exact match preferred, wildcard supported) and still falls back to a sensible default for Claude names when needed.

- Anthropic Passthrough Toggle (independent)
  - New config: `TOKLIGENCE_ANTHROPIC_PASSTHROUGH_ENABLED` (INI: `anthropic_passthrough_enabled`, default: true).
  - Controls whether requests routed to `anthropic` are proxied directly to Anthropic (useful for upstream usage accounting). Disable to force translation through the generic adapter (for testing/custom accounting).

- Tolerant Parsing and Diagnostics
  - Normalizes `message.content` shapes: string, `{text}`, `{content:string}`, `{content:[blocks]}`.
  - `tool_result.content` accepts string / single block / block array; `tool_result.is_error` is parsed for observability (does not change bridge logic).
  - Added concise tool bridge debug logs for `tool_use`/`tool_result` previews.

- Documentation and Tests
  - Updated README and docs (User Guide, API Mapping) to describe tool bridge, streaming, normalization, and the new config toggle.
  - Added tests for normalization shapes and tool mapping (`tool_use → tool_calls`, `tool_result → tool`).

## Configuration

- New
  - `TOKLIGENCE_ANTHROPIC_PASSTHROUGH_ENABLED` (INI: `anthropic_passthrough_enabled`) — default: true
  - `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM` (INI: `openai_tool_bridge_stream`) — default: false (recommended for coding agents)
- Existing
  - `TOKLIGENCE_ANTHROPIC_NATIVE_ENABLED` — default: true

## Upgrade Notes

1. Pull the new version and run `go test ./...` — should pass.
2. To validate streaming tool bridge: route `claude*` to OpenAI, ensure an OpenAI key is configured, call `POST /anthropic/v1/messages` with `stream=true`. Pure text turns will stream deltas; after a `tool_result` turn, assistant text also streams.
3. If you rely on upstream Anthropic usage accounting, keep `TOKLIGENCE_ANTHROPIC_PASSTHROUGH_ENABLED=true`. Set to `false` to force translation for testing or custom accounting.

## Known Limitations

- Tool bridge streaming covers text deltas. Turns that only contain `tool_calls` produce no deltas.
- Stop reasons are mapped conservatively. Multimodal content is not bridged yet.

## Notable Changes

- httpserver: streaming tool bridge; respect router aliases; tolerant parsing; tool diagnostics.
- router: select adapter before model alias rewrite; expose `RewriteModelPublic`.
- config/cmd: independent Anthropic passthrough toggle.
- docs/tests: updated and extended.
