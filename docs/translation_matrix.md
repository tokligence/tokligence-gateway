# Translation Support Matrix & Limits

## Current Translation Paths (built-in)
- `POST /anthropic/v1/messages` (and `/v1/messages` variants) → **OpenAI Chat Completions** (includes streaming, tools, simple `count_tokens` heuristic).
- `POST /v1/responses` (OpenAI Responses API, Codex) → **Anthropic Messages API** (streaming Responses events back, tool conversion, duplicate-tool guard optional).
- Other OpenAI endpoints (`/v1/chat/completions`, `/v1/embeddings`) are **native** passthrough only; no protocol translation is performed.
- Anthropic `/v1/messages/count_tokens` is a lightweight local estimator; it is not forwarded upstream.

## Key Behaviors & Limitations
- **Model-first routing**: `model_provider_routes` decides provider by model prefix (e.g., `gpt*→openai`, `claude*→anthropic`). Endpoint only decides whether to translate or passthrough. Missing provider credentials force translation via the other side with defaults.
- **Max tokens (current)**:
  - Anthropic→OpenAI bridge clamps `max_tokens` with `OpenAICompletionMaxTokens` (default 16384) to avoid OpenAI 400s.
  - Responses→Anthropic bridge injects default `anthropic_max_tokens` (default 8192) if absent.
  - No per-model context awareness yet; see “Proposed model metadata” below.
- **Duplicate tool guard**: now toggleable via `duplicate_tool_detection` / `TOKLIGENCE_DUPLICATE_TOOL_DETECTION` (default **off**). When enabled, warns at 3–4 identical tool outputs and errors at 5+ to stop infinite loops in Responses flows.
- **Anthropic beta/tool headers**: basic `/v1/messages` translation path does not yet expose Anthropic beta headers (web search, computer use, MCP) from the upstream translator module; only core chat/tool/caching behavior is used.

## Upstream Translator Capabilities (module `github.com/tokligence/openai-anthropic-endpoint-translation`)
The bundled translator library supports richer Anthropic features (from `docs/analysis.md` in the module):
- Tool calling (incl. MCP/hosted tools), JSON mode, reasoning/thinking, web search beta, prompt caching headers, files API, code execution tool injection.
- Streaming normalization of Anthropic SSE to OpenAI-style chunks.
- Responses API adaptations mirroring chat adapters.

**Gap:** Gateway currently uses the translator for Anthropic Messages↔OpenAI Chat and Responses→Anthropic, but advanced beta headers (web_search/computer_use/MCP) are not surfaced in HTTP handlers. If needed, wire these flags through the request payload to the translator and expose required headers.

## Proposed Model Metadata (context + recommended completion cap)
Static starter table (approximate public specs; to be refined/overwritten by dynamic reload):

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
- **Local baseline**: ship a versioned JSON/YAML in `config/model_metadata.json` (or `.d` directory) with the table above; hot-reload similar to model aliases.
- **Remote refresh (optional)**: allow `TOKLIGENCE_MODEL_METADATA_URL` to fetch signed/validated metadata from a Tokligence endpoint/GitHub release; cache on disk with TTL and fallback to local baseline on failure.
- **Lookup API**: add a small internal helper to query by model and return `context_tokens` and `max_completion_cap`. Use it in translation bridges instead of hardcoded caps.
- **Config overrides**: support env/INI overrides per model or provider for emergency patches.
- **Logging**: debug log when clamping uses table values vs. default caps.

This approach keeps translation logic safe by default while allowing timely updates as providers change limits.
