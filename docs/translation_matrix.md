# Translation Support Matrix & Limits

## Current Translation Paths (built-in)
- `POST /anthropic/v1/messages` (and `/v1/messages` variants) → **OpenAI Chat Completions** (includes streaming, tools, simple `count_tokens` heuristic).
- `POST /v1/responses` (OpenAI Responses API, Codex) → **Anthropic Messages API** (streaming Responses events back, tool conversion, duplicate-tool guard optional).
- `POST /v1/chat/completions` with models routed to **Anthropic** (e.g., `claude*`) and `TOKLIGENCE_CHAT_TO_ANTHROPIC=on` → **Anthropic Messages API**  
  - Non‑streaming: request is translated to `/v1/messages` and the raw Anthropic JSON response is returned to the client.  
  - Streaming: Anthropic SSE is converted back into OpenAI `chat.completion.chunk` events.
- Other OpenAI endpoints (`/v1/chat/completions` with OpenAI models, `/v1/embeddings`) are **native** passthrough only; no protocol translation is performed.
- Anthropic `/v1/messages/count_tokens` is a lightweight local estimator; it is not forwarded upstream.

## Key Behaviors & Limitations
- **Model-first routing**: `model_provider_routes` decides provider by model prefix (e.g., `gpt*→openai`, `claude*→anthropic`). Endpoint only decides whether to translate or passthrough. If the inferred provider is unavailable, the gateway falls back to the other side via translation using configured defaults.
- **Max tokens (current)**:
  - Anthropic→OpenAI bridge clamps `max_tokens` with `OpenAICompletionMaxTokens` (default 16384) or per-model caps (metadata table) to avoid OpenAI 400s.
  - Responses→Anthropic bridge injects default `anthropic_max_tokens` (default 8192) or per-model caps if present.
  - Per-model metadata is hot-reloaded (local file + optional remote URL); falls back to defaults on failure.
- **Anthropic beta/tool toggles** (env/INI): `anthropic_web_search`, `anthropic_computer_use`, `anthropic_mcp`, `anthropic_prompt_caching`, `anthropic_json_mode`, `anthropic_reasoning` (all default off). Disabled fields are stripped from Anthropic payloads before translation; when enabled, the gateway will also emit an `anthropic-beta` header on flows that send traffic to Anthropic (Chat→Messages, Responses→Messages), or use an explicit `anthropic_beta_header` override if configured.
- **Duplicate tool guard**: now toggleable via `duplicate_tool_detection` / `TOKLIGENCE_DUPLICATE_TOOL_DETECTION` (default **off**). When enabled, warns at 3–4 identical tool outputs and errors at 5+ to stop infinite loops in Responses flows.
- **Anthropic beta/tool headers**: For Anthropic→OpenAI flows (e.g., Claude Code via `/anthropic/v1/messages`) the bridge still strips beta-only fields and does not emit `anthropic-beta` when calling OpenAI. Beta headers are only relevant when the gateway itself calls Anthropic.

## Upstream Translator Capabilities (module `github.com/tokligence/openai-anthropic-endpoint-translation`)
The bundled translator library supports richer Anthropic features (from `docs/analysis.md` in the module):
- Tool calling (incl. MCP/hosted tools), JSON mode, reasoning/thinking, web search beta, prompt caching headers, files API, code execution tool injection.
- Streaming normalization of Anthropic SSE to OpenAI-style chunks.
- Responses API adaptations mirroring chat adapters.

**Gap:** Gateway currently uses the translator for Anthropic Messages↔OpenAI Chat and Responses→Anthropic, but advanced beta headers (web_search/computer_use/MCP) are not surfaced in HTTP handlers. If needed, wire these flags through the request payload to the translator and expose required headers.

## Model Metadata (context + recommended completion cap)
Gateway ships a starter table in `data/model_metadata.json` (also published via a raw GitHub URL) that is loaded at startup and refreshed periodically. The format is:

```json
{"model":"gpt-4o","provider":"openai","context_tokens":128000,"max_completion_cap":16000}
```

Below is an example slice of that table (values are approximate public specs and may change over time):

| Provider | Model | Context (tokens) | Suggested `max_tokens` cap |
| --- | --- | --- | --- |
| OpenAI | gpt-4o | 128k | 16k |
| OpenAI | gpt-4o-mini | 128k | 16k |
| OpenAI | gpt-4-turbo-2024-04-09 | 128k | 16k |
| OpenAI | gpt-4-32k | 32k | 8k |
| OpenAI | gpt-3.5-turbo-0125 | 16k | 4k |
| OpenAI | o1-preview | 128k | 8k |
| OpenAI | o1-mini | 128k | 8k |
| Anthropic | claude-3-5-sonnet-20241022 | 200k | 8k |
| Anthropic | claude-3-5-haiku-20241022 | 200k | 8k |
| Anthropic | claude-3-opus-20240229 | 200k | 8k |
| Anthropic | claude-3-sonnet-20240229 | 200k | 8k |
| Anthropic | claude-3-haiku-20240307 | 200k | 8k |
| Meta | llama-3.1-405b | 131k | 8k |
| Meta | llama-3.1-70b | 131k | 8k |
| Meta | llama-3.1-8b | 131k | 4k |
| Meta | llama-3-70b | 8k–16k | 4k |
| Meta | llama-3-8b | 8k–16k | 4k |
| Mistral | mistral-large-2407 | 128k | 8k |
| Mistral | mistral-large-2402 | 128k | 8k |
| Mistral | mistral-small-2402 | 32k | 4k |
| Qwen | qwen-2.5-72b | 128k | 8k |
| Qwen | qwen-2.5-14b | 64k | 4k |
| Qwen | qwen-2.5-7b | 64k | 4k |
| Kimi | kimi-1.5-pro | 200k | 8k |
| Kimi | kimi-1.5-lite | 200k | 4k |
| Google | gemini-1.5-pro | 1M | 8k–16k |
| Google | gemini-1.5-flash | 1M | 8k |
| xAI | grok-2 | 128k | 8k |
| Cohere | command-r-plus | 128k | 8k |
| Cohere | command-r | 128k | 8k |

> Values are indicative; vendors revise limits frequently. Prefer dynamic refresh over hardcoding.

## Design Sketch for Dynamic Metadata Loading
- **Local baseline**: ship a versioned JSON in `data/model_metadata.json` (or `.d` directory) with the table above; hot-reload similar to model aliases.
- **Remote refresh (optional)**: allow `TOKLIGENCE_MODEL_METADATA_URL` (defaults to the Gateway GitHub raw URL) to fetch signed/validated metadata; cache on disk with TTL and fallback to local baseline on failure.
- **Lookup API**: add a small internal helper to query by model and return `context_tokens` and `max_completion_cap`. Use it in translation bridges instead of hardcoded caps.
- **Config overrides**: support env/INI overrides per model or provider for emergency patches.
- **Logging**: debug log when clamping uses table values vs. default caps.

This approach keeps translation logic safe by default while allowing timely updates as providers change limits.
