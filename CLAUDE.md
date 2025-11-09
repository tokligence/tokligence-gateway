# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Tokligence Gateway is a high-performance LLM gateway that provides unified access to multiple AI providers (OpenAI, Anthropic) with bidirectional protocol translation. It's designed to work seamlessly with OpenAI Codex CLI (v0.55.0+) and Claude Code (v2.0.29).

**Current Version**: v0.3.0

## Essential Commands

### Build & Run
```bash
make build              # Build both gateway and gatewayd binaries
make bg                 # Build gateway CLI only
make bgd                # Build gatewayd daemon only

# Start daemon (common shortcuts)
make gfr                # Force restart (kills :8081, rotates logs, restarts)
make gds                # Start gatewayd daemon
make gdx                # Stop gatewayd daemon
make gdr                # Restart gatewayd daemon
make gst                # Show daemon status

# Legacy long commands (use shortcuts above)
make gd-force-restart   # Same as `make gfr`
make gd-start           # Same as `make gds`
```

### Testing
```bash
make test               # Run all tests (backend + frontend)
make bt                 # Backend Go tests (shortcut for be-test)
make ft                 # Frontend tests (shortcut for fe-test)
go test ./...           # Run all Go tests directly

# Integration tests
cd tests && ./run_all_tests.sh
./tests/integration/tool_calls/test_tool_call_basic.sh
```

### Development Profiles
```bash
make ansi               # Start with Anthropic sidecar (Codex→Anthropic translation)
make ode                # Start with OpenAI delegation (gpt*/o1* passthrough)
make adp                # Clean logs, rebuild, and run gatewayd
```

## Architecture Overview

### Core Translation Flows

**1. Codex CLI → Anthropic (Translation Mode)**
```
Codex (OpenAI Responses API)
  → /v1/responses endpoint
  → Provider Abstraction Layer
  → Tool Adapter (filters apply_patch, update_plan)
  → Anthropic Provider (translation)
  → SSE Orchestrator
  → OpenAI Responses API SSE events
```

**2. Claude Code → OpenAI (Sidecar Mode)**
```
Claude Code (Anthropic /v1/messages)
  → /anthropic/v1/messages endpoint
  → Translation Layer (internal/translation/)
  → Model Mapping (claude* → gpt*)
  → OpenAI Chat Completions API
  → Anthropic SSE event translation
```

### Key Components by Layer

| Layer | Location | Purpose |
|-------|----------|---------|
| **Responses API Handler** | `internal/httpserver/endpoint_responses.go`<br/>`internal/httpserver/responses_handler.go` | Main entry point for `/v1/responses` |
| **SSE Streaming** | `internal/httpserver/responses_stream.go` | Converts provider responses to OpenAI Responses API SSE events |
| **Provider Abstraction** | `internal/httpserver/responses/provider.go`<br/>`internal/httpserver/responses/provider_anthropic.go` | Interface for different providers (Anthropic, OpenAI delegation) |
| **Tool Adapter** | `internal/httpserver/tool_adapter/adapter.go` | Filters unsupported tools, injects guidance into system messages |
| **Anthropic Translator** | `internal/httpserver/anthropic/native.go`<br/>`internal/httpserver/anthropic/stream.go` | OpenAI ↔ Anthropic format conversion |
| **Sidecar Translation** | `internal/translation/adapterhttp/handler.go`<br/>`internal/translation/adapter/adapter.go` | Anthropic → OpenAI translation for Claude Code |
| **Conversation Builder** | `internal/httpserver/responses/conversation.go` | Builds canonical conversation from sessions |

### Session Management

- **Location**: `internal/httpserver/responses_handler.go`
- **Storage**: In-memory only (cleared on restart)
- **Purpose**: Maintains state for multi-turn tool calling workflows
- **Flow**: Keeps SSE connection open across `required_action.submit_tool_outputs` cycles

### Duplicate Detection

- **Location**: `internal/httpserver/responses_handler.go:detectDuplicateToolCalls()`
- **Thresholds**: 3 duplicates = warning, 5 = emergency stop
- **Purpose**: Prevents infinite loops when models repeatedly call the same tool with same arguments

### Tool Adapter Filtering

- **Location**: `internal/httpserver/tool_adapter/adapter.go`
- **Filtered Tools**: `apply_patch`, `update_plan` (Codex-specific, not supported by Anthropic)
- **Behavior**: Removes filtered tools, injects guidance to use shell alternatives
- **System Message Cleaning**: Automatically removes references to filtered tools from prompts

## Configuration

### Three-Layer Config System
1. Global defaults: `config/setting.ini`
2. Environment overlays: `config/{dev,test,live}/gateway.ini`
3. Environment variables: `TOKLIGENCE_*` (highest priority)

### Critical Environment Variables
```bash
# API Keys
TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-...
TOKLIGENCE_OPENAI_API_KEY=sk-proj-...

# Routing
TOKLIGENCE_ROUTES="claude*=>anthropic,gpt*=>openai"

# Responses API Mode
TOKLIGENCE_RESPONSES_DELEGATE=auto    # auto|always|never

# Sidecar Translation (for Claude Code)
TOKLIGENCE_SIDECAR_MODEL_MAP="claude-3-5-sonnet-20241022=gpt-4o
claude-3-5-haiku-20241022=gpt-4o-mini"
TOKLIGENCE_SIDECAR_DEFAULT_OPENAI_MODEL=gpt-4o
TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS=16384

# Development
TOKLIGENCE_LOG_LEVEL=debug
TOKLIGENCE_MARKETPLACE_ENABLED=false
TOKLIGENCE_AUTH_DISABLED=true
```

## Repository Rules & Conventions

### Git Commit Guidelines

**IMPORTANT: Never add Co-Authored-By in commit messages**
- Do NOT add `Co-Authored-By: Claude <noreply@anthropic.com>` or similar co-author tags
- Keep commit messages clean and without attribution footers
- Commit messages should be concise and descriptive only

### Release Notes Location

**Release notes MUST be in `docs/releases/` directory, NOT root**
- ✅ Correct: `docs/releases/RELEASE_NOTES.md` or `docs/releases/v0.3.0.md`
- ❌ Wrong: `RELEASE_NOTES.md` (root directory)
- Root directory should only contain `README.md` and `README_zh.md` for markdown files
- All other .md files in root are ignored by `.gitignore` via `/*.md` pattern

## Code Modification Guidelines

### When Adding/Modifying Endpoints

1. Create endpoint in `internal/httpserver/endpoint_*.go`
2. Implement handler in `internal/httpserver/` (e.g., `responses_handler.go`)
3. Update endpoint keys in `internal/httpserver/server.go` (default*EndpointKeys)
4. Register in `internal/httpserver/protocol/` if needed

### When Modifying Tool Translation

1. **OpenAI → Anthropic**: Update `internal/httpserver/anthropic/native.go:ConvertChatToNative()`
2. **Anthropic → OpenAI**: Update `internal/translation/adapter/adapter.go:AnthropicToOpenAI()`
3. **Tool Filtering**: Update `internal/httpserver/tool_adapter/adapter.go` FilteredTools map
4. Update guidance text for filtered tools in same file

### When Modifying SSE Streaming

1. **Responses API SSE**: Modify `internal/httpserver/responses_stream.go`
2. **Anthropic SSE**: Modify `internal/httpserver/anthropic/stream.go`
3. **Event sequence**: Maintain proper order (created → deltas → done → completed)
4. **Field requirements**: Always include `item_id` for tool call deltas, `sequence_number` for all events

## Testing Strategy

### Unit Tests
```bash
go test ./internal/httpserver/...
go test ./internal/translation/...
```

### Integration Tests
```bash
# All integration tests
cd tests && ./run_all_tests.sh

# Specific test categories
./tests/integration/tool_calls/test_tool_call_basic.sh
./tests/integration/duplicate_detection/test_duplicate_emergency_stop.sh
./tests/integration/responses_api/test_responses_basic.sh
```

### Manual Testing
```bash
# Test Responses API with Anthropic
curl -N -X POST http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'

# Test Anthropic native endpoint
curl -N -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4o",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## Common Pitfalls & Solutions

### Issue: SSE events not recognized by client
- **Cause**: Missing required fields (`item_id`, `sequence_number`)
- **Solution**: Check `responses_stream.go` event emission, ensure all required fields present

### Issue: Tool calls showing as raw JSON
- **Cause**: Missing `item_id` in `function_call_arguments.delta` events
- **Solution**: Add `item_id` field linking delta to `output_item.added` event

### Issue: Infinite tool call loops
- **Cause**: Model repeatedly calling same tool with same arguments
- **Solution**: Duplicate detection automatically stops at 5 duplicates; check `detectDuplicateToolCalls()` thresholds

### Issue: Anthropic rejects tool calls
- **Cause**: Filtered tools (`apply_patch`, `update_plan`) in request
- **Solution**: Tool adapter should filter automatically; check `tool_adapter/adapter.go` configuration

### Issue: MaxTokens error from OpenAI
- **Cause**: Anthropic allows larger max_tokens than OpenAI accepts
- **Solution**: Set `TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS=16384` (already default)

## Documentation References

- **Codex Integration**: `docs/codex-to-anthropic.md`
- **Claude Code Integration**: `docs/claude_code-to-openai.md`
- **Tool Call Translation**: `docs/tool-call-translation.md`
- **Session Architecture**: `docs/responses-session-architecture.md`
- **Quick Start**: `docs/QUICK_START.md`

## Database Schema

- **Identity Store**: `~/.tokligence/identity.db` (SQLite) or PostgreSQL
- **Ledger Store**: `~/.tokligence/ledger.db` (SQLite) or PostgreSQL
- **Migrations**: Auto-run on startup via `internal/userstore/sqlite/migrations.go`

## Build System

- Go version: 1.24+
- Build timezone: Configurable via `BUILD_TZ` (default: Asia/Singapore)
- Multi-platform support: Linux/macOS/Windows (amd64, arm64)
- Docker: Multi-arch images (personal and team editions)

## Key Behavioral Notes

1. **Sessions are ephemeral**: In-memory only, no persistence across restarts
2. **Tool filtering is automatic**: `apply_patch` and `update_plan` always filtered for Anthropic
3. **Model mapping required**: Claude Code → OpenAI requires explicit model mapping
4. **Dual-mode Responses API**: Auto-detects based on model routing (translation vs delegation)
5. **Streaming always preferred**: Enable `stream: true` for best latency
