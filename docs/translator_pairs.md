# Translator Coverage (OpenAI ↔ Anthropic)

This summarizes the API-to-API pairs covered by the bundled translator library (`github.com/tokligence/openai-anthropic-endpoint-translation`) and what the gateway currently uses.

## Core Pairs (implemented in the library)
- **OpenAI Chat Completions → Anthropic Messages**  
  - Maps messages, system prompts, tools/tool_choice, response_format/JSON mode, stop, temperature/top_p, metadata, and streaming chunks.
  - Supports Anthropic beta headers (web_search/computer_use/MCP) and prompt caching in the library.
- **Anthropic Messages → OpenAI Chat Completions**  
  - Converts content blocks (text/tool_use/tool_result/thinking) to OpenAI assistant messages + tool_calls; normalizes streaming SSE to OpenAI-style chunks.
- **OpenAI Responses API → Anthropic Messages**  
  - Translates Responses request/messages/tools to Anthropic payload, streams Anthropic SSE back as Responses events (the gateway uses this path for Codex).
- **Anthropic Messages → OpenAI Responses API**  
  - Converts Anthropic responses into Responses format/events (supported in the translator; gateway can reuse if exposing Anthropic→Responses).

## Gateway Usage Today
- Uses **Anthropic → OpenAI Chat** for `/anthropic/v1/messages` translation to OpenAI Chat Completions (sidecar handler used by Claude Code).
- Uses **OpenAI Responses → Anthropic** for `/v1/responses` translation to Anthropic Messages (Codex path), including streaming and tool-call translation.
- Uses **OpenAI Chat Completions → Anthropic Messages** for `/v1/chat/completions` when the requested model routes to Anthropic (e.g., `claude*`) and `TOKLIGENCE_CHAT_TO_ANTHROPIC=on`:
  - Non-streaming: translates the request and returns raw Anthropic message JSON.
  - Streaming: normalizes Anthropic SSE back into OpenAI `chat.completion.chunk` events.
- Surfaces Anthropic beta capabilities via:
  - JSON fields on `NativeRequest` (web_search/computer_use/MCP/prompt_caching/JSON mode/reasoning) which are stripped when translating toward OpenAI.
  - Optional `anthropic-beta` header when the gateway itself calls Anthropic (Chat→Messages, Responses→Messages), controlled by `anthropic_*` toggles or an explicit `anthropic_beta_header` override.

## Not Covered by the Translator (out of scope)
- Embeddings, audio, image APIs.
- Provider-specific admin/management endpoints.
- Payment/ledger/billing APIs.
