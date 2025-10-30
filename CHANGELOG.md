# Changelog

All notable changes to this project will be documented in this file.

## [v0.2.0] - 2025-10-31 (reissued)

This reissue supersedes the earlier v0.2.0 build and finalizes the Anthropic→OpenAI translation path used by Claude Code clients.

- Replace legacy bridge code with a single in‑process translation module under `internal/translation`
- Sidecar‑only Anthropic handler for `/anthropic/v1/messages`
- Correct SSE event sequence for text and tools; fixes Claude Code SSE parsing errors
- Configurable OpenAI completion `max_tokens` clamp to avoid upstream 400s
- Rotating logs (daily + size) and separate CLI/daemon log files
- Config: add `TOKLIGENCE_AUTH_DISABLED`, `TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS`, `TOKLIGENCE_ANTHROPIC_FORCE_SSE`, `TOKLIGENCE_ANTHROPIC_TOKEN_CHECK_ENABLED`, `TOKLIGENCE_ANTHROPIC_MAX_TOKENS`

[v0.2.0]: https://github.com/tokligence/tokligence-gateway/releases/tag/v0.2.0

