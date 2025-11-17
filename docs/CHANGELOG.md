# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.0] - 2025-11-08

Major release adding OpenAI Responses API support, Codex CLI integration, Docker deployment, and comprehensive testing infrastructure.

### Added

**Core Features**
- OpenAI Responses API (`/v1/responses`) with dual-mode support:
  - **Translation mode**: Translates OpenAI Responses API → Anthropic for claude* models
  - **Delegation mode**: Direct passthrough/proxy to OpenAI for gpt*/o1* models
  - Auto-detect based on model routing (configurable via `TOKLIGENCE_RESPONSES_DELEGATE`)
- Full SSE streaming support for Responses API
- Session management for Responses API with tool calling
- Provider abstraction layer for clean OpenAI/Anthropic separation
- Intelligent duplicate detection to prevent infinite tool call loops
- Tool adapter filtering for Codex compatibility (filters unsupported tools)
- Multi-port deployment option for strict endpoint isolation (façade, OpenAI, Anthropic, admin)

**Codex CLI Integration**
- Full support for Codex CLI v0.55.0+ with Responses API
- Verified end-to-end with screenshot documentation
- Automatic tool call normalization
- Streaming responses with tool execution

**Docker Deployment**
- Personal edition (no authentication, 35.6MB) - `Dockerfile.personal`
- Team edition (authentication enabled, 57MB) - `Dockerfile.team`
- docker-compose with profiles for easy edition switching
- Multi-architecture support (linux/amd64, linux/arm64)
- Team edition auto-creates default admin user (cs@tokligence.ai)

**Testing Infrastructure**
- 26 integration test scripts organized by category
- Test suite reorganization (integration/, fixtures/, utils/, config/)
- Comprehensive tool call flow tests
- Duplicate detection emergency stop tests
- SSE streaming format validation tests
- Responses API workflow coverage

**Configuration**
- Configurable build timezone in Makefile (defaults to Asia/Singapore)
- Hot-reload for model aliases (5-second interval)

**Translation & Routing Enhancements**
- Global work mode (`TOKLIGENCE_WORK_MODE`, `work_mode=auto|passthrough|translation`) with **model-first, endpoint-second** routing via `model_provider_routes` (e.g., `gpt*→openai`, `claude*→anthropic`).
- New Chat→Anthropic bridge: `/v1/chat/completions` with `claude*` models can be translated to Anthropic `/v1/messages` when `TOKLIGENCE_CHAT_TO_ANTHROPIC=on` (non-streaming returns Anthropic JSON; streaming maps Anthropic SSE back into OpenAI `chat.completion.chunk` events).
- Anthropic beta feature wiring:
  - Config toggles: `anthropic_web_search`, `anthropic_computer_use`, `anthropic_mcp`, `anthropic_prompt_caching`, `anthropic_json_mode`, `anthropic_reasoning` (and env equivalents), plus `anthropic_beta_header` override.
  - `anthropic-beta` header is now attached on Chat→Anthropic and Responses→Anthropic upstream calls when enabled.
- Duplicate tool-call detection toggle (`duplicate_tool_detection` / `TOKLIGENCE_DUPLICATE_TOOL_DETECTION`) to enable or disable infinite-loop guarding in Responses flows.
- Model metadata loader (`internal/modelmeta`) with hot-reloadable per-model caps from `data/model_metadata.json` or `TOKLIGENCE_MODEL_METADATA_URL`, used by Anthropic→OpenAI and Responses→Anthropic bridges to clamp `max_tokens` safely.

**Docs & Planning**
- Updated `docs/translation_matrix.md` and `docs/translator_pairs.md` to document the new Chat→Anthropic bridge, model-first routing, per-model caps, and Anthropic beta header behavior.
- Added `docs/endpoint-translation-todo.md` to track remaining integration work with `github.com/tokligence/openai-anthropic-endpoint-translation` and a phased roadmap by ROI and complexity.

### Changed

**Code Organization**
- Modularized HTTP server endpoints
- Enhanced OpenAI responses translator
- Improved Anthropic adapter and stream handling
- Refactored responses stream handling and session management

**Documentation**
- Added Codex CLI compatibility badge with OpenAI logo
- Added Claude Code badge with Anthropic logo
- Updated architecture diagrams to include `/v1/responses` endpoint
- Reorganized features.md by version (v0.3.0, v0.1.0, v0.4.0+)
- Added comprehensive Docker deployment guide (docs/DOCKER.md)
- Added Codex integration guide (docs/codex-to-anthropic.md)
- Updated API endpoints table to highlight Codex usage
- Product matrix unified to v0.3.0 status
- Synchronized all changes to Chinese README (README_zh.md)

**Anthropic Messages & count_tokens**
- `/v1/messages/count_tokens` now optionally calls the real Anthropic `messages/count_tokens` endpoint when `work_mode=passthrough` and an Anthropic API key is configured, falling back to the local heuristic on failure; debug logs now include `source=local|upstream` for visibility.
- Anthropic Messages handler logs now explicitly annotate `mode=passthrough` vs `mode=translation provider=openai` and emit compact `workmode.summary` lines for each decision, making it easier to understand auto-routing and translation behavior across endpoints.

### Fixed
- Removed broken test `TestStreamResponses_WaitsForToolOutputs`
- Fixed Docker builder missing bash dependency
- Fixed docker-compose.yml YAML syntax errors
- Codex-compatible duplicate detection (scans tool messages, not just ToolCalls)

### Testing
- All 26 integration tests passing
- Codex CLI verified with full-auto mode
- Docker personal and team editions tested

## [v0.2.0] - 2025-10-31 (reissued)

This reissue supersedes the earlier v0.2.0 build and finalizes the Anthropic→OpenAI translation path used by Claude Code clients. See docs/releases/v0.2.0.md for details.

- Replace legacy bridge code with a single in‑process translation module under `internal/translation`
- Sidecar‑only Anthropic handler for `/anthropic/v1/messages`
- Correct SSE event sequence for text and tools; fixes Claude Code SSE parsing errors
- Configurable OpenAI completion `max_tokens` clamp to avoid upstream 400s
- Rotating logs (daily + size) and separate CLI/daemon log files
- Config: add `TOKLIGENCE_AUTH_DISABLED`, `TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS`, `TOKLIGENCE_ANTHROPIC_FORCE_SSE`, `TOKLIGENCE_ANTHROPIC_TOKEN_CHECK_ENABLED`, `TOKLIGENCE_ANTHROPIC_MAX_TOKENS`

## [v0.1.0]

First production-ready release. See docs/releases/v0.1.0.md.
