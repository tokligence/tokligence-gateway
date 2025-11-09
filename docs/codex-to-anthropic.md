# Using OpenAI Codex to Access Anthropic Claude via Gateway

This guide shows you how to use OpenAI Codex CLI (v0.55.0+) to access Anthropic's Claude models through the Tokligence Gateway.

## Overview

```
OpenAI Codex → Gateway (Responses API) → Anthropic Claude
```

The gateway accepts OpenAI Responses API requests and translates them to Anthropic's Messages API, including full support for tool calling, streaming, and intelligent duplicate detection.

## TL;DR — Point Codex at the Gateway

**✅ Verified with Codex CLI v0.55.0+**

Codex CLI uses the OpenAI Responses API (`/v1/responses`). The gateway automatically detects and translates to Anthropic.

```bash
# Tell Codex to use the gateway instead of api.openai.com
export OPENAI_BASE_URL=http://localhost:8081/v1

# Codex will send this as Authorization: Bearer ...
# If gateway auth is disabled (dev), any non-empty string is fine
export OPENAI_API_KEY=dummy

# Run Codex with a Claude model name (will be routed to Anthropic)
codex --full-auto --config 'model="claude-3-5-sonnet-20241022"'
```

## Prerequisites

- **Tokligence Gateway v0.3.0+** installed and running
- **Anthropic API key**
- **OpenAI Codex CLI v0.55.0+** or any OpenAI-compatible client

## Step 1: Configure Gateway

### Option A: Edit INI (recommended for persistence)

Edit your active environment config, for example `config/dev/gateway.ini`:

```ini
# Anthropic API Key (required)
anthropic_api_key=sk-ant-api03-YOUR_ANTHROPIC_KEY_HERE

# Route claude* models to the Anthropic adapter
routes=claude*=>anthropic

# HTTP listen address (default :8081)
http_address=:8081

# Optional: disable marketplace and auth for local dev
marketplace_enabled=false
auth_disabled=true

# Optional: daily rotating daemon log
log_file_daemon=logs/dev-gatewayd.log
log_level=info
```

### Option B: Use environment variables (ephemeral)

```bash
export TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-api03-YOUR_ANTHROPIC_KEY_HERE
export TOKLIGENCE_ROUTES='claude*=>anthropic'
export TOKLIGENCE_MARKETPLACE_ENABLED=false
export TOKLIGENCE_AUTH_DISABLED=true
# Note: listen address is configured via INI key http_address, not env
```

### Key parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `TOKLIGENCE_ANTHROPIC_API_KEY` | Your Anthropic API key | Yes | - |
| `TOKLIGENCE_ROUTES` | Model routing rules | Yes | - |
| `http_address` (INI) | Gateway listening address | No | `:8081` |
| `TOKLIGENCE_MARKETPLACE_ENABLED` | Enable marketplace features | No | false |
| `TOKLIGENCE_AUTH_DISABLED` | Disable API key auth for dev | No | false |

## Step 2: Start Gateway

```bash
# Build the gateway (first time only)
make build

# Start the gateway daemon
./bin/gatewayd
```

You should see output like:
```
[gatewayd][DEBUG] Tokligence Gateway version=v0.3.0 commit=... built_at=...
[gatewayd][DEBUG] adapters registered: [loopback openai anthropic]
[gatewayd][DEBUG] routes configured: map[claude*:anthropic]
[gatewayd][DEBUG] gateway server listening on :8081
```

## Step 3: Using Codex CLI with Claude Models

### Basic Usage

```bash
# Set base URL to gateway
export OPENAI_BASE_URL=http://localhost:8081/v1
export OPENAI_API_KEY=dummy

# Run Codex with Claude model (full-auto mode recommended)
codex --full-auto --config 'model="claude-3-5-sonnet-20241022"'
```

### How It Works

1. **Codex sends** OpenAI Responses API request to `/v1/responses`
2. **Gateway detects** `claude*` model and routes to Anthropic adapter
3. **Gateway translates**:
   - OpenAI Responses API format → Anthropic Messages API
   - OpenAI tools → Anthropic tools (with filtering)
   - Anthropic SSE events → OpenAI Responses API SSE events
4. **Codex receives** OpenAI-compatible streaming responses
5. **Tool calling** works seamlessly with automatic normalization

## Step 4: Available Features

### ✅ Supported Features

| Feature | Support | Notes |
|---------|---------|-------|
| **OpenAI Responses API** | ✅ Full | `/v1/responses` endpoint with SSE streaming |
| **Tool Calling** | ✅ Full | Automatic OpenAI ↔ Anthropic conversion |
| **Tool Adapter Filtering** | ✅ Yes | Filters `apply_patch`, `update_plan` (unsupported by Anthropic) |
| **Streaming (SSE)** | ✅ Full | Real-time token streaming |
| **Duplicate Detection** | ✅ Yes | Prevents infinite tool call loops |
| **Session Management** | ✅ Yes | Maintains state for tool workflows |
| **Shell Command Normalization** | ✅ Yes | Auto-fixes common shell patterns |

### 🔧 Translation Features

**Intelligent Duplicate Detection**
- Detects repeated identical tool calls
- Injects warnings after multiple attempts
- Prevents infinite loops automatically
- Codex-compatible: scans `role=tool` messages

**Tool Adapter Filtering**
- Automatically filters tools unsupported by Anthropic:
  - `apply_patch` - Not available in Anthropic
  - `update_plan` - Not available in Anthropic
- Preserves all other tools (shell, file operations, etc.)

**Shell Command Normalization**
- Converts string commands to array format when needed
- Handles common shell patterns automatically
- Works with Anthropic's sandbox requirements

## Translation Architecture (How It Works)

### Current Architecture (v0.3.0)

```
Codex CLI (Responses API request)
        ↓
OpenAI ingress parser
        ↓
Canonical Conversation (Base + Chat)
        ↓
Provider Abstraction Layer
        ├─ Anthropic Provider (translation)
        └─ OpenAI Provider (delegation/passthrough)
        ↓
SSE Orchestrator (responses_stream.go)
        ↓
OpenAI Responses API SSE back to Codex
```

### Key Components

| Layer | Files | Responsibilities |
|-------|-------|------------------|
| **Ingress** | `internal/openai/response.go`<br/>`internal/httpserver/endpoint_responses.go` | Parse and validate Responses API requests |
| **Session Manager** | `internal/httpserver/responses_handler.go` | Store session state, handle tool outputs, duplicate detection |
| **Stream Orchestrator** | `internal/httpserver/responses_stream.go` | SSE writer, event emission, tool-call coordination |
| **Provider Layer** | `internal/httpserver/responses/provider.go`<br/>`internal/httpserver/responses/provider_anthropic.go` | Abstract provider interface, Anthropic translation |
| **Tool Adapter** | `internal/httpserver/tool_adapter/adapter.go` | Filter unsupported tools for compatibility |
| **Translator** | `internal/httpserver/anthropic/native.go`<br/>`internal/httpserver/anthropic/stream*.go` | Convert between OpenAI and Anthropic formats |

### Dual-Mode Operation

The gateway supports two modes for Responses API:

1. **Translation Mode** (default for claude* models)
   - Translates OpenAI Responses API → Anthropic Messages API
   - Full SSE streaming support
   - Tool call conversion with filtering

2. **Delegation Mode** (auto-detect for gpt*/o1* models)
   - Direct passthrough/proxy to OpenAI `/v1/responses`
   - Controlled by `TOKLIGENCE_RESPONSES_DELEGATE` (auto/always/never)
   - Note: Streaming currently disabled in delegation mode

## Available Claude Models

You can use any Claude model with the OpenAI Responses API format:

| Model Name | Description |
|------------|-------------|
| `claude-3-5-sonnet-20241022` | Latest Claude 3.5 Sonnet (recommended) |
| `claude-3-5-haiku-20241022` | Latest Claude 3.5 Haiku |
| `claude-3-5-sonnet-20240620` | Previous Claude 3.5 Sonnet |
| `claude-3-opus-20240229` | Claude 3 Opus (most capable) |
| `claude-3-sonnet-20240229` | Claude 3 Sonnet (balanced) |
| `claude-3-haiku-20240307` | Claude 3 Haiku (fastest) |

## Verification

### Test Basic Responses API

```bash
curl -N -X POST http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Say hello"}],
    "stream": true
  }'
```

Expected SSE events:
```
event: response.created
data: {"id":"resp_...","status":"in_progress",...}

event: response.output_text.delta
data: {"delta":"Hello","output_index":0,...}

event: response.completed
data: {"status":"completed","output":[...],...}

event: done
data: [DONE]
```

### Test Tool Calling

```bash
curl -N -X POST http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Run: ls -la"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "execute_bash",
        "description": "Execute bash command",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {"type": "string"}
          },
          "required": ["command"]
        }
      }
    }],
    "stream": true
  }'
```

### Check Available Models

```bash
curl http://localhost:8081/v1/models | jq .
```

### View Gateway Logs

```bash
tail -f logs/dev-gatewayd.log
# Rotated files: logs/dev-gatewayd-YYYY-MM-DD.log
```

## Current Limitations and Workarounds

### 1. Tool Adapter Limitations

**Limitation**: Codex-specific tools are filtered
- `apply_patch` - Not supported by Anthropic
- `update_plan` - Not supported by Anthropic
- **Workaround**: Codex CLI can still write files using `write_to_file` or bash commands

**Code Location**: `internal/httpserver/tool_adapter/adapter.go`

### 2. Session Persistence

**Limitation**: Sessions stored in-memory only
- Gateway restart clears all pending tool calls
- Codex must retry from scratch after gateway restart
- **Workaround**: Use Docker for stable deployments, implement graceful shutdown

**Future**: Persistent session store (Redis/SQLite) planned for v0.4.0

### 3. Duplicate Detection Heuristics

**Limitation**: Detection relies on content matching
- May miss duplicates if output format changes slightly
- Threshold is hardcoded (not configurable)
- **Current Thresholds**: 3 duplicates = warning, 5 = emergency stop

**Code Location**: `internal/httpserver/responses_handler.go:detectDuplicateToolCalls()`

### 4. Delegation Mode Streaming

**Limitation**: OpenAI delegation disables streaming
- When using gpt*/o1* models, streaming is disabled
- Logged as warning: "responses: disabling stream for openai delegation"
- **Reason**: Avoids unsupported streaming behavior

**Workaround**: Use non-streaming mode for OpenAI models

### 5. Model Coverage

**Limitation**: Routing is pattern-based
- Only `claude*` routes to Anthropic (configurable)
- New Anthropic models must match pattern
- **Workaround**: Update `TOKLIGENCE_ROUTES` for new patterns

### 6. Anthropic-Specific Features

**Limitation**: Some Anthropic features not exposed
- Extended thinking (`extended_thinking` parameter)
- Prompt caching hints
- **Reason**: Not part of OpenAI Responses API spec

**Future**: May expose via custom headers or extended API

## Troubleshooting

### Gateway Not Starting

```bash
# Check if :8081 is in use
lsof -i :8081 || ss -ltnp | grep 8081 || true

# Change listen address via INI, then restart
sed -i 's/^http_address=.*/http_address=:8082/' config/dev/gateway.ini
./bin/gatewayd
```

### Authentication Errors

```bash
# Verify your Anthropic API key is set
echo $TOKLIGENCE_ANTHROPIC_API_KEY

# Test the key directly with Anthropic
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $TOKLIGENCE_ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{
    "model": "claude-3-haiku-20240307",
    "max_tokens": 10,
    "messages": [{"role": "user", "content": "Hi"}]
  }'
```

### Model Not Found

Ensure your routing configuration includes the model pattern:

```bash
# This will NOT work with claude models
TOKLIGENCE_ROUTES=gpt*=>openai

# This WILL work with claude models
TOKLIGENCE_ROUTES=claude*=>anthropic,gpt*=>openai
```

### Tool Calls Not Working

- Verify you're using Responses API endpoint `/v1/responses` (not `/v1/chat/completions`)
- Check that tools are defined in the request
- Review logs for tool adapter filtering messages
- Some Codex tools (`apply_patch`, `update_plan`) are filtered automatically

### Infinite Loop Detection

If you see warnings about duplicate tool calls:
```
⚠️ CRITICAL WARNING: You have executed the same tool 3 times in a row...
```

**What it means**: The gateway detected repeated identical tool calls
**Action**: Review the tool output - it may contain errors the model is ignoring
**Emergency stop**: After 5 duplicates, the request is rejected automatically

## Advanced Configuration

### Responses API Delegation Mode

```bash
# Auto-detect based on model routing (default)
TOKLIGENCE_RESPONSES_DELEGATE=auto

# Force all Responses API to OpenAI (passthrough)
TOKLIGENCE_RESPONSES_DELEGATE=always

# Force all to translation (even gpt* models)
TOKLIGENCE_RESPONSES_DELEGATE=never
```

### Anthropic API Settings

```bash
# Custom Anthropic endpoint
TOKLIGENCE_ANTHROPIC_BASE_URL=https://api.anthropic.com

# Anthropic API version
TOKLIGENCE_ANTHROPIC_VERSION=2023-06-01

# Max tokens for Anthropic
TOKLIGENCE_ANTHROPIC_MAX_TOKENS=4096
```

### Debug Logging

```bash
# Enable debug logging
TOKLIGENCE_LOG_LEVEL=debug
./bin/gatewayd

# or via INI
log_level=debug
```

## Performance Tips

1. **Use Haiku for speed/cost**: `claude-3-5-haiku-20241022`
2. **Enable streaming for lower latency**: Always use `"stream": true`
3. **Use full-auto mode in Codex**: `codex --full-auto` for best results
4. **Keep prompts compact**: Reduces latency and token costs
5. **Monitor duplicate detection**: Prevents wasted API calls

## Testing & Verification

The gateway includes comprehensive tests:

```bash
# Run all tests
go test ./...

# Run integration tests
cd tests && ./run_all_tests.sh

# Test Responses API specifically
./tests/integration/responses_api/test_responses_basic.sh
./tests/integration/tool_calls/test_tool_call_basic.sh

# Test duplicate detection
./tests/integration/duplicate_detection/test_duplicate_emergency_stop.sh
```

## Next Steps

- [Responses Session Architecture](responses-session-architecture.md) - How sessions work
- [Tool Call Translation](tool-call-translation.md) - Detailed translation logic
- [Gateway Features Documentation](features.md) - Full feature matrix
- [Quick Start Guide](QUICK_START.md) - General setup

## Support

- **Issues**: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- **Documentation**: [Full Documentation](../README.md)
- **Codex CLI**: Tested with v0.55.0+ - see [data/images/codex-to-anthropic.png](../data/images/codex-to-anthropic.png)
