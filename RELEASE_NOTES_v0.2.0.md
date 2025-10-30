# Tokligence Gateway v0.2.0 (reissued)

Date: 2025-10-31

This reissue finalizes the Anthropic‑native endpoint (Claude Code) with a single, proven translation path and robust logging. It supersedes the earlier v0.2.0 build.

## Highlights

- Streaming Tool Bridge (Anthropic → OpenAI)
  - Stream handling is now performed by the in‑process translation module. Anthropic SSE (`message_start`, `content_block_*`, `message_delta`, `message_stop`) is emitted consistently and accepted by Claude Code.
  - New toggle: `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM` (INI: `openai_tool_bridge_stream`, default: false). For coding‑agent workflows, batch responses are preferred to improve action continuity.
  - First turns that only return `tool_calls` (no text) naturally produce no deltas; after your client posts a `tool_result`, assistant text streams as deltas (when enabled).

- Model Alias Consistency (with wildcards)
  - Adapter selection happens on the original incoming model, then provider-specific model alias rewriting is applied. This prevents alias rewrites (e.g., `claude* → gpt-4o`) from breaking route selection.
  - The tool bridge respects Router model aliases (exact match preferred, wildcard supported) and still falls back to a sensible default for Claude names when needed.

- Anthropic Passthrough Toggle (independent)
  - Config: `TOKLIGENCE_ANTHROPIC_PASSTHROUGH_ENABLED` (INI: `anthropic_passthrough_enabled`).
  - Useful for upstream usage accounting or comparison; during development, disable to exercise the translation path.

- Tolerant Parsing and Diagnostics
  - Normalizes `message.content` shapes: string, `{text}`, `{content:string}`, `{content:[blocks]}`.
  - `tool_result.content` accepts string / single block / block array; `tool_result.is_error` is parsed for observability (does not change bridge logic).
  - Added concise tool bridge debug logs for `tool_use`/`tool_result` previews.

- Documentation and Tests
  - README now includes Go version badge and verified compatibility with Claude Code v2.0.29.
  - Translation module unit tests cover text‑only and non‑stream JSON paths.

## Configuration

- New
  - `TOKLIGENCE_ANTHROPIC_PASSTHROUGH_ENABLED` (INI: `anthropic_passthrough_enabled`) — default: true
  - `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM` (INI: `openai_tool_bridge_stream`) — default: false (recommended for coding agents)
- Existing
  - `TOKLIGENCE_ANTHROPIC_NATIVE_ENABLED` — default: true

## Upgrade Notes

1. Pull the new version and run `go test ./...` — should pass.
2. To validate streaming tool bridge: route `claude*` to OpenAI, ensure an OpenAI key is configured, call `POST /anthropic/v1/messages` with `stream=true`. Pure text turns will stream deltas; after a `tool_result` turn, assistant text also streams.
3. For upstream Anthropic usage accounting, keep `TOKLIGENCE_ANTHROPIC_PASSTHROUGH_ENABLED=true`. Set to `false` to force translation for testing or custom accounting.

### Reissue Summary

- Replace legacy bridge code with a single in‑process translation module under `internal/translation`
- Sidecar‑only Anthropic handler for `/anthropic/v1/messages` (no legacy branches)
- Correct SSE delta mapping and close semantics (fixes Claude Code client parse errors)
- Configurable OpenAI completion `max_tokens` clamp to avoid upstream 400s
- Rotating logs (daily + size), separate CLI/daemon log files, and optional auth disable for development

## Known Limitations

- Tool bridge streaming covers text deltas. Turns that only contain `tool_calls` produce no deltas.
- Stop reasons are mapped conservatively. Multimodal content is not bridged yet.

## Notable Changes

- httpserver: streaming tool bridge; respect router aliases; tolerant parsing; tool diagnostics.
- router: select adapter before model alias rewrite; expose `RewriteModelPublic`.
- config/cmd: independent Anthropic passthrough toggle.
- docs/tests: updated and extended.
