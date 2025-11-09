# Using Claude Code to Access OpenAI GPT Models via Gateway

**Version**: v0.3.0
**Verified**: Claude Code v2.0.29

This guide shows you how to use Claude Code (Anthropic's VSCode/CLI tool) to access OpenAI's GPT models through the Tokligence Gateway.

## Overview

```
Claude Code (Anthropic API) → Gateway Translation Layer → OpenAI Chat Completions
```

The gateway accepts Anthropic-native `/v1/messages` requests and translates them to OpenAI's Chat Completions API, allowing Claude Code to use GPT models seamlessly with full tool calling and streaming support.

## TL;DR — Point Claude Code at the Gateway

Claude Code speaks Anthropic's API. Point it at the gateway's Anthropic path (`/anthropic/v1`).

**Option A** — settings.json (recommended)

```bash
mkdir -p ~/.claude
cat > ~/.claude/settings.json <<'EOF'
{
  "anthropic": {
    "baseURL": "http://localhost:8081/anthropic/v1",
    "apiKey": "dummy"
  }
}
EOF
```

**Option B** — environment variables

```bash
export ANTHROPIC_BASE_URL=http://localhost:8081/anthropic/v1
export ANTHROPIC_API_KEY=dummy
```

Then use Claude model names as usual in Claude Code. The gateway maps them to OpenAI models via configurable model mapping.

## Prerequisites

- Tokligence Gateway installed and running
- OpenAI API key
- Claude Code installed (VSCode extension or CLI)

## Step 1: Configure Gateway

### Option A: Edit INI (recommended for persistence)

Edit your active environment config, for example `config/dev/gateway.ini`:

```ini
# OpenAI API Key (required)
openai_api_key=sk-proj-YOUR_OPENAI_KEY_HERE

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
export TOKLIGENCE_OPENAI_API_KEY=sk-proj-YOUR_OPENAI_KEY_HERE
export TOKLIGENCE_MARKETPLACE_ENABLED=false
# Note: listen address is configured via INI key http_address, not env
```

### Configuration Parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `TOKLIGENCE_OPENAI_API_KEY` | Your OpenAI API key | Yes | - |
| `TOKLIGENCE_SIDECAR_MODEL_MAP` | Model name mapping (see below) | No | - |
| `TOKLIGENCE_SIDECAR_DEFAULT_OPENAI_MODEL` | Fallback OpenAI model | No | `gpt-4o` |
| `TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS` | Max completion tokens cap | No | 16384 |
| `http_address` (INI) | Gateway listening address | No | `:8081` |
| `TOKLIGENCE_MARKETPLACE_ENABLED` | Enable marketplace features | No | false |

### Model Mapping Configuration

The gateway translates Anthropic model names to OpenAI model names using a configurable mapping.

**Code Location**: `internal/translation/adapterhttp/handler.go:40-57`

**Format** (line-delimited):
```ini
claude-3-5-sonnet-20241022=gpt-4o
claude-3-5-haiku-20241022=gpt-4o-mini
claude-3-opus-20240229=gpt-4-turbo
```

**Configuration via environment** (newline-separated):
```bash
export TOKLIGENCE_SIDECAR_MODEL_MAP="claude-3-5-sonnet-20241022=gpt-4o
claude-3-5-haiku-20241022=gpt-4o-mini
claude-3-opus-20240229=gpt-4-turbo"
```

**Fallback behavior**:
1. If model matches a mapping entry → use mapped OpenAI model
2. If no match and `TOKLIGENCE_SIDECAR_DEFAULT_OPENAI_MODEL` is set → use default model (default: `gpt-4o`)
3. Otherwise → use the Anthropic model name as-is (may fail if OpenAI doesn't recognize it)

## Step 2: Start Gateway

```bash
# Build the gateway (first time only)
make build

# Start the gateway daemon
./bin/gatewayd
```

You should see output like:
```
2025/11/05 10:30:00 Starting Tokligence Gateway on :8081
2025/11/05 10:30:00 Loaded routes: gpt-*=>openai,o1-*=>openai
```

## Step 3: Configure Claude Code

### Configure Claude Code

Option A — settings.json（推荐）

```bash
mkdir -p ~/.claude
cat > ~/.claude/settings.json <<'EOF'
{
  "anthropic": {
    "baseURL": "http://localhost:8081/anthropic/v1",
    "apiKey": "dummy"
  }
}
EOF
```

Option B — 环境变量

```bash
export ANTHROPIC_BASE_URL=http://localhost:8081/anthropic/v1
export ANTHROPIC_API_KEY=dummy
```

Option C — VS Code 插件设置

1) 打开 VSCode 设置（Cmd+, 或 Ctrl+,）
2) 搜索 “Claude Code” 并设置：
   - API Base URL: `http://localhost:8081/anthropic/v1`

## Step 4: Test the Integration

### Using curl to Test Anthropic Format → OpenAI

```bash
curl -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hello, GPT!"}
    ]
  }'
```

Expected response (Anthropic format):
```json
{
  "id": "chatcmpl-xxx",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! How can I assist you today?"
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 12
  }
}
```

### Using Claude Code CLI

```bash
# Start a conversation (Claude model name; gateway maps to OpenAI upstream)
claude-code --model claude-3-5-sonnet-20241022 "Explain what you are"
```

## Translation Architecture (How It Works)

### Current Architecture (v0.3.0)

```
Claude Code (Anthropic /v1/messages request)
        ↓
Anthropic endpoint handler (endpoint_anthropic.go)
        ↓
Translation layer (internal/translation/adapterhttp/handler.go)
        ├─ Model mapping (ModelMap + DefaultOpenAIModel)
        ├─ MaxTokens clamping (avoid OpenAI 400 errors)
        └─ Request conversion (adapter.AnthropicToOpenAI)
        ↓
OpenAI Chat Completions API
        ↓
Response conversion (OpenAI → Anthropic format)
        ├─ Non-streaming: direct JSON conversion
        └─ Streaming: SSE event translation
        ↓
Anthropic-style response back to Claude Code
```

### Key Components

| Layer | Files | Responsibilities |
|-------|-------|------------------|
| **Endpoint Handler** | `internal/httpserver/endpoint_anthropic.go`<br/>`internal/httpserver/server.go:832-862` | Routes `/anthropic/v1/messages` and `/v1/messages` to translation handler |
| **Translation Handler** | `internal/translation/adapterhttp/handler.go` | HTTP handler that orchestrates request/response translation |
| **Request Translator** | `internal/translation/adapter/adapter.go:148-191` | Converts Anthropic request format to OpenAI Chat Completions format |
| **Response Translator** | `internal/translation/adapterhttp/handler.go:112-192` | Converts OpenAI response to Anthropic format (non-streaming) |
| **Stream Translator** | `internal/translation/adapter/adapter.go:194+` | Converts OpenAI SSE chunks to Anthropic-style SSE events |

### Request Translation Details

**Code Location**: `internal/translation/adapter/adapter.go:148-191`

**Translation mappings**:
- Anthropic `system` field → OpenAI `role=system` message
- Anthropic `messages[].role=user` with text blocks → OpenAI `role=user` with joined text
- Anthropic `messages[].role=user` with `tool_result` blocks → OpenAI `role=tool` messages
- Anthropic `messages[].role=assistant` with text blocks → OpenAI `role=assistant` with joined text
- Anthropic `messages[].role=assistant` with `tool_use` blocks → OpenAI `ToolCalls[]`
- Anthropic `tools[]` → OpenAI `tools[]` (function type)
- Anthropic `max_tokens` → OpenAI `max_tokens` (with clamping)
- Anthropic `temperature` → OpenAI `temperature`
- Anthropic `stop_sequences` → OpenAI `stop`

**Stop Reason Mapping** (response):
- OpenAI `stop` → Anthropic `end_turn`
- OpenAI `length` → Anthropic `max_tokens`
- OpenAI `tool_calls` → Anthropic `tool_use`

### SSE Streaming Events

**Code Location**: `internal/translation/adapter/adapter.go:194+`

When streaming is enabled, the gateway translates OpenAI SSE chunks to Anthropic-style events:

**Anthropic SSE events emitted**:
1. `message_start` - Initial message metadata
2. `content_block_start` - Start of text or tool_use block
3. `content_block_delta` - Incremental content updates (text or tool arguments)
4. `content_block_stop` - End of content block
5. `message_delta` - Final metadata (stop_reason, usage)
6. `message_stop` - End of stream

## Available OpenAI Models

You can use any OpenAI model through Claude Code by mapping Anthropic model names:

| Model Name | Description | Recommended Mapping |
|------------|-------------|---------------------|
| `gpt-4o` | GPT-4 Omni (latest, multimodal) | `claude-3-5-sonnet-20241022=gpt-4o` |
| `gpt-4o-mini` | GPT-4 Omni Mini (fast & cost-effective) | `claude-3-5-haiku-20241022=gpt-4o-mini` |
| `gpt-4-turbo` | GPT-4 Turbo (128k context) | `claude-3-opus-20240229=gpt-4-turbo` |
| `gpt-4` | GPT-4 (8k context) | - |
| `gpt-3.5-turbo` | GPT-3.5 Turbo (fast, 16k context) | - |
| `o1` | OpenAI o1 (reasoning, slower) | - |
| `o1-mini` | OpenAI o1 mini (faster reasoning) | - |

## Step 5: Using Tool Calling

Claude Code supports tool/function calling through the Anthropic tools format, which the gateway will convert to OpenAI.

### Example: Claude Code with Tools

```bash
# Create a tool definition file
cat > weather_tool.json <<'EOF'
{
  "name": "get_weather",
  "description": "Get the current weather in a given location",
  "input_schema": {
    "type": "object",
    "properties": {
      "location": {
        "type": "string",
        "description": "The city and state, e.g. San Francisco, CA"
      },
      "unit": {
        "type": "string",
        "enum": ["celsius", "fahrenheit"]
      }
    },
    "required": ["location"]
  }
}
EOF
```

### Test with curl

```bash
curl -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 200,
    "messages": [
      {"role": "user", "content": "What is the weather in San Francisco?"}
    ],
    "tools": [
      {
        "name": "get_weather",
        "description": "Get the current weather in a given location",
        "input_schema": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state"
            }
          },
          "required": ["location"]
        }
      }
    ]
  }'
```

## Supported Anthropic Parameters

The gateway converts these Anthropic parameters to OpenAI format:

| Anthropic Parameter | OpenAI Equivalent | Support |
|---------------------|-------------------|---------|
| `model` | `model` | ✅ Full |
| `messages` | `messages` | ✅ Full |
| `max_tokens` | `max_tokens` | ✅ Full |
| `temperature` | `temperature` | ✅ Full |
| `top_p` | `top_p` | ✅ Full |
| `stream` | `stream` | ✅ Full |
| `tools` | `tools` | ✅ Full |
| `tool_choice` | `tool_choice` | ✅ Full |
| `stop_sequences` | `stop` | ✅ Full |
| `system` | `messages` (system role) | ✅ Full |

## Verification

### Test Basic Message

```bash
curl -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4-turbo-preview",
    "max_tokens": 10,
    "messages": [
      {"role": "user", "content": "Say hello"}
    ]
  }' | jq .
```

### Test Token Counting

```bash
curl -X POST http://localhost:8081/anthropic/v1/messages/count_tokens \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4-turbo-preview",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }' | jq .
```

### View Gateway Logs

```bash
tail -f logs/dev-gatewayd.log   # base path set by log_file_daemon in INI
# Rotated files: logs/dev-gatewayd-YYYY-MM-DD.log
```

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
# Verify your OpenAI API key is set
echo $TOKLIGENCE_OPENAI_API_KEY

# Test the key directly with OpenAI
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $TOKLIGENCE_OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hi"}],
    "max_tokens": 10
  }'
```

### Claude Code Can't Connect

```bash
# Test if gateway is accessible
curl http://localhost:8081/health

# Check if Anthropic endpoint is working
curl -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-3.5-turbo",
    "max_tokens": 5,
    "messages": [{"role": "user", "content": "Hi"}]
  }'
```

### Model Not Found

On the Anthropic-native endpoint, the gateway maps requests to OpenAI internally. You do not need `TOKLIGENCE_ROUTES` for this path. If you call the OpenAI path `/v1/chat/completions` directly, then set routes (e.g. `gpt-*=>openai`).

### SSE Streaming Issues

Claude Code requires proper Server-Sent Events (SSE) format. The gateway handles this automatically:

```bash
# Test streaming
curl -N -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4-turbo-preview",
    "max_tokens": 100,
    "messages": [{"role": "user", "content": "Count to 5"}],
    "stream": true
  }'
```

## Advanced Configuration

- Optional OpenAI base URL: `TOKLIGENCE_OPENAI_BASE_URL=https://api.openai.com/v1`
- Optional OpenAI organization: `TOKLIGENCE_OPENAI_ORG=org_...`
- Debug logging: set `log_level=debug` in `config/dev/gateway.ini`

### Custom System Prompts

Claude Code allows system prompts, which the gateway converts properly:

```bash
curl -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4-turbo-preview",
    "max_tokens": 100,
    "system": "You are a helpful coding assistant.",
    "messages": [
      {"role": "user", "content": "Write a Python hello world"}
    ]
  }'
```

### Request Logging

Enable detailed logging to debug issues:

```bash
# Enable debug logging
TOKLIGENCE_LOG_LEVEL=debug
TOKLIGENCE_LOG_REQUESTS=true
./bin/gatewayd
```

## Performance Tips

1. **Use GPT-3.5-Turbo for Speed**: Faster and cheaper than GPT-4
2. **Enable Streaming**: Claude Code benefits from streaming responses
3. **Adjust Token Limits**: Set appropriate `max_tokens` for your use case
4. **Use Connection Pooling**: Gateway maintains connection pools automatically

## Comparison: Direct vs Gateway

### Direct Anthropic Connection
```bash
# Claude Code → Anthropic API (direct)
ANTHROPIC_API_KEY=sk-ant-xxx
claude-code --model claude-3-5-sonnet "Hello"
```

### Gateway Connection to OpenAI
```bash
# Claude Code → Gateway → OpenAI API
ANTHROPIC_BASE_URL=http://localhost:8081/anthropic
claude-code --model gpt-4-turbo-preview "Hello"
```

## Use Cases

### 1. Cost Optimization
Use cheaper OpenAI models for simple tasks:
```bash
claude-code --model gpt-3.5-turbo "Fix typos in this file"
```

### 2. Model Comparison
Compare responses from different providers:
```bash
# Try with GPT
claude-code --model gpt-4-turbo-preview "Explain quantum computing"

# Try with Claude
claude-code --model claude-3-5-sonnet "Explain quantum computing"
```

### 3. Fallback Strategy
Configure multiple routes for reliability:
```bash
TOKLIGENCE_ROUTES=claude*=>anthropic,gpt-*=>openai
# Use GPT if Anthropic is down
```

## Current Limitations and Workarounds

### 1. Model Mapping Required

**Limitation**: Claude model names must be explicitly mapped to OpenAI models

**Code Location**: `internal/translation/adapterhttp/handler.go:40-57`

**Workaround**: Configure `TOKLIGENCE_SIDECAR_MODEL_MAP` or set `TOKLIGENCE_SIDECAR_DEFAULT_OPENAI_MODEL`

**Example**:
```bash
export TOKLIGENCE_SIDECAR_MODEL_MAP="claude-3-5-sonnet-20241022=gpt-4o
claude-3-5-haiku-20241022=gpt-4o-mini"
export TOKLIGENCE_SIDECAR_DEFAULT_OPENAI_MODEL=gpt-4o
```

### 2. MaxTokens Clamping

**Limitation**: Anthropic allows very large `max_tokens` values that OpenAI rejects

**Code Location**: `internal/translation/adapterhttp/handler.go:67-78, 114-118`

**Default Behavior**: Gateway clamps `max_tokens` to 16384 (configurable)

**Workaround**: Set `TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS` to adjust the cap

**Why this exists**: Anthropic models can have `max_tokens` up to 8192 or higher, but OpenAI models typically accept up to 16384 completion tokens. Without clamping, requests may fail with `400 Bad Request`.

### 3. Token Counting Differences

**Limitation**: Token counts are approximate due to different tokenizers

**Impact**: Usage reporting may not match Anthropic's token counting exactly

**Reason**: OpenAI uses different tokenizers (tiktoken) vs Anthropic's tokenizers

**Workaround**: Monitor actual OpenAI API usage for accurate billing

### 4. Feature Parity Gaps

**Limitation**: Some Claude-specific features not available with GPT models

**Anthropic-specific features not translated**:
- Extended thinking mode (not applicable to OpenAI)
- Prompt caching (Anthropic-specific optimization)
- Vision/multimodal input format differences

**OpenAI-specific features not available via Anthropic API**:
- GPT-4o vision via Anthropic messages format (different multimodal structure)
- Structured outputs (JSON mode) - not exposed via Anthropic translation
- Function calling format differences (mostly handled, but edge cases may exist)

### 5. SSE Streaming Format

**Limitation**: SSE event translation adds minimal latency

**Code Location**: `internal/translation/adapter/adapter.go:194+`

**Impact**: Streaming responses include translation overhead (typically <10ms per event)

**Events translated**:
- OpenAI `data: {...}` chunks → Anthropic `event: content_block_delta` format
- Token estimation in `message_delta` is approximate (based on text length / 4)

**Workaround**: None needed - latency is negligible for most use cases

### 6. Content Joining Behavior

**Limitation**: Multiple text blocks are joined with `\n\n`

**Code Location**: `internal/translation/adapter/adapter.go:158-159, 185`

**Impact**: May affect precise formatting in edge cases where block separation matters

**Example**:
```json
Anthropic: {"content": [{"type": "text", "text": "A"}, {"type": "text", "text": "B"}]}
OpenAI:    {"content": "A\n\nB"}
```

### 7. Operation Mode

**Limitation**: Only translation mode is supported (no direct Anthropic API passthrough)

**Code Location**: `internal/httpserver/server.go:844-850`

**Available modes**:
- `sidecarMsgsHandler` (default) - Translation to OpenAI
- `anthPassthroughEnabled` - Direct passthrough to Anthropic API (must be explicitly enabled)

**Current setup**: Translation mode is the default for Claude Code → OpenAI use case

### 8. Tool Call Format Normalization

**Limitation**: Tool call IDs and arguments must follow both providers' constraints

**Translation behavior**:
- Anthropic `tool_use.id` → OpenAI `tool_calls[].id` (preserved)
- Anthropic `tool_use.input` (object) → OpenAI `function.arguments` (JSON string)
- OpenAI `tool_calls` → Anthropic `tool_use` blocks

**Edge case**: If tool arguments are not valid JSON, they're passed as `{"_raw": "..."}` in some cases (see `internal/httpserver/anthropic/native.go:143-147`)

### 9. Vision/Multimodal Support

**Limitation**: Image handling format differs between providers

**Current support**: Basic text and tool calling only; multimodal content requires format adaptation

**Workaround**: Use OpenAI-native endpoints (`/v1/chat/completions`) for vision tasks, or implement custom multimodal content translation

## Next Steps

- [Gateway Features Documentation](features.md)
- [Quick Start Guide](QUICK_START.md)
- [Codex Integration](codex-to-anthropic.md)
- [API Reference](API_REFERENCE.md)

## Support

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Documentation: [Full Documentation](../README.md)
- Claude Code Docs: [Claude Code Documentation](https://docs.claude.com/claude-code)
