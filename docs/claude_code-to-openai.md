# Using Claude Code to Access OpenAI GPT Models via Gateway

This guide shows you how to use Claude Code (Anthropic's VSCode/CLI tool) to access OpenAI's GPT models through the Tokligence Gateway.

## Overview

```
Claude Code → Gateway (Anthropic API format) → OpenAI GPT
```

The gateway accepts Anthropic-native requests and translates them to OpenAI's API, allowing Claude Code to use GPT models seamlessly.

## TL;DR — Point Claude Code at the Gateway

Claude Code speaks Anthropic’s API. Point it at the gateway’s Anthropic path (`/anthropic/v1`).

Option A — settings.json (recommended)

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

Option B — environment variables

```bash
export ANTHROPIC_BASE_URL=http://localhost:8081/anthropic/v1
export ANTHROPIC_API_KEY=dummy
```

Then use Claude model names as usual in Claude Code. The gateway maps to OpenAI upstream (default gpt‑4o).

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
| `http_address` (INI) | Gateway listening address | No | `:8081` |
| `TOKLIGENCE_MARKETPLACE_ENABLED` | Enable marketplace features | No | false |

### Important: Model Naming Strategy

You can keep using Claude model names in Claude Code. The gateway’s Anthropic-native endpoint maps requests to OpenAI and defaults to `gpt-4o` upstream.

- Recommended: continue to use Claude model IDs (e.g. `claude-3-5-sonnet-20241022`).
- Advanced: forcing a specific OpenAI model on the Anthropic endpoint is not yet configurable; the default is `gpt-4o`.

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

## Available OpenAI Models

You can use any OpenAI model through Claude Code:

| Model Name | Description |
|------------|-------------|
| `gpt-4-turbo-preview` | Latest GPT-4 Turbo |
| `gpt-4-1106-preview` | GPT-4 Turbo (Nov 2023) |
| `gpt-4` | GPT-4 (8k context) |
| `gpt-4-32k` | GPT-4 (32k context) |
| `gpt-3.5-turbo` | GPT-3.5 Turbo (fast & cheap) |
| `gpt-3.5-turbo-16k` | GPT-3.5 Turbo (16k context) |
| `o1-preview` | OpenAI o1 (reasoning) |
| `o1-mini` | OpenAI o1 mini |

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

## Limitations

1. **Model Name Mismatch**: Claude Code expects Claude model names, but you'll use GPT names
2. **Feature Parity**: Some Claude-specific features may not work with GPT
3. **Token Counting**: Token counts are approximate due to different tokenizers
4. **Vision/Multimodal**: Image support depends on model capabilities

## Next Steps

- [Gateway Features Documentation](features.md)
- [Quick Start Guide](QUICK_START.md)
- [Codex Integration](codex-to-anthropic.md)
- [API Reference](API_REFERENCE.md)

## Support

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Documentation: [Full Documentation](../README.md)
- Claude Code Docs: [Claude Code Documentation](https://docs.claude.com/claude-code)
