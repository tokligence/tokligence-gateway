# Using OpenAI Codex to Access Anthropic Claude via Gateway

This guide shows you how to use OpenAI Codex (or any OpenAI-compatible client) to access Anthropic's Claude models through the Tokligence Gateway.

## Overview

```
OpenAI Codex → Gateway (OpenAI API format) → Anthropic Claude
```

The gateway accepts OpenAI-compatible requests and translates them to Anthropic's API, including full support for tool calling (function calling).

## Prerequisites

- Tokligence Gateway installed and running
- Anthropic API key
- OpenAI Codex or OpenAI-compatible client

## Step 1: Configure Gateway

### Create or Edit `.env` File

```bash
cd /path/to/tokligence-gateway

# Create .env file with your configuration
cat > .env <<'EOF'
# Anthropic API Key (required for accessing Claude models)
TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-api03-YOUR_ANTHROPIC_KEY_HERE

# Route all claude* models to Anthropic
TOKLIGENCE_ROUTES=claude*=>anthropic

# Optional: Disable marketplace features
TOKLIGENCE_MARKETPLACE_ENABLED=false

# Optional: Gateway port (default: 8081)
TOKLIGENCE_PORT=8081
EOF
```

### Configuration Parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `TOKLIGENCE_ANTHROPIC_API_KEY` | Your Anthropic API key | Yes | - |
| `TOKLIGENCE_ROUTES` | Model routing rules | Yes | - |
| `TOKLIGENCE_PORT` | Gateway listening port | No | 8081 |
| `TOKLIGENCE_MARKETPLACE_ENABLED` | Enable marketplace features | No | false |

## Step 2: Start Gateway

```bash
# Build the gateway (first time only)
make build

# Start the gateway daemon
./bin/gatewayd
```

Or use environment variables directly:

```bash
export TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-api03-YOUR_KEY_HERE
export TOKLIGENCE_ROUTES='claude*=>anthropic'
export TOKLIGENCE_MARKETPLACE_ENABLED=false
./bin/gatewayd
```

You should see output like:
```
2025/11/05 10:30:00 Starting Tokligence Gateway on :8081
2025/11/05 10:30:00 Loaded routes: claude*=>anthropic
```

## Step 3: Configure Your OpenAI Client

### Option A: Using OpenAI Python SDK

```python
from openai import OpenAI

# Point the client to your gateway
client = OpenAI(
    base_url="http://localhost:8081/v1",
    api_key="dummy"  # Gateway doesn't require this when auth is disabled
)

# Use Claude models with OpenAI API format
response = client.chat.completions.create(
    model="claude-3-haiku-20240307",  # Will be routed to Anthropic
    messages=[
        {"role": "user", "content": "Hello, Claude!"}
    ],
    max_tokens=100
)

print(response.choices[0].message.content)
```

### Option B: Using OpenAI Node.js SDK

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8081/v1',
  apiKey: 'dummy'  // Gateway doesn't require this when auth is disabled
});

async function main() {
  const response = await client.chat.completions.create({
    model: 'claude-3-haiku-20240307',
    messages: [
      { role: 'user', content: 'Hello, Claude!' }
    ],
    max_tokens: 100
  });

  console.log(response.choices[0].message.content);
}

main();
```

### Option C: Using curl

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-haiku-20240307",
    "messages": [
      {"role": "user", "content": "Hello, Claude!"}
    ],
    "max_tokens": 100
  }'
```

## Step 4: Using Tool Calling (Function Calling)

The gateway fully supports OpenAI's function calling with automatic conversion to Anthropic's tools format.

### Python Example

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8081/v1",
    api_key="dummy"
)

# Define tools (functions)
tools = [
    {
        "type": "function",
        "function": {
            "name": "get_weather",
            "description": "Get the current weather in a given location",
            "parameters": {
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
    }
]

# Request with tool calling
response = client.chat.completions.create(
    model="claude-3-haiku-20240307",
    messages=[
        {"role": "user", "content": "What's the weather in San Francisco?"}
    ],
    tools=tools,
    tool_choice="auto",
    max_tokens=200
)

# Check if Claude wants to call a tool
if response.choices[0].message.tool_calls:
    tool_call = response.choices[0].message.tool_calls[0]
    print(f"Claude wants to call: {tool_call.function.name}")
    print(f"With arguments: {tool_call.function.arguments}")
else:
    print(response.choices[0].message.content)
```

### curl Example

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-haiku-20240307",
    "messages": [
      {"role": "user", "content": "What is the weather in San Francisco?"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get the current weather in a given location",
          "parameters": {
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
      }
    ],
    "tool_choice": "auto",
    "max_tokens": 200
  }'
```

Expected response:
```json
{
  "id": "msg_xxx",
  "choices": [{
    "finish_reason": "tool_calls",
    "message": {
      "role": "assistant",
      "content": "Okay, let me check the weather in San Francisco:",
      "tool_calls": [{
        "id": "toolu_xxx",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\":\"San Francisco, CA\",\"unit\":\"celsius\"}"
        }
      }]
    }
  }]
}
```

## Available Claude Models

You can use any Claude model with the OpenAI API format:

| Model Name | Description |
|------------|-------------|
| `claude-3-5-sonnet-20241022` | Latest Claude 3.5 Sonnet |
| `claude-3-5-sonnet-20240620` | Previous Claude 3.5 Sonnet |
| `claude-3-opus-20240229` | Claude 3 Opus (most capable) |
| `claude-3-sonnet-20240229` | Claude 3 Sonnet (balanced) |
| `claude-3-haiku-20240307` | Claude 3 Haiku (fastest) |

## Verification

### Test Basic Chat

```bash
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-haiku-20240307",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_tokens": 10
  }' | jq .
```

### Check Available Models

```bash
curl http://localhost:8081/v1/models | jq .
```

### View Gateway Logs

The gateway logs all requests in the `logs/` directory:

```bash
# View daemon logs
tail -f logs/gatewayd.log

# View specific date logs
cat logs/gatewayd-2025-11-05.log
```

## Supported OpenAI Parameters

The gateway supports these OpenAI chat completion parameters:

| Parameter | Support | Notes |
|-----------|---------|-------|
| `model` | ✅ Full | Routes based on `TOKLIGENCE_ROUTES` |
| `messages` | ✅ Full | Converted to Anthropic format |
| `max_tokens` | ✅ Full | Passed to Anthropic |
| `temperature` | ✅ Full | Passed to Anthropic |
| `top_p` | ✅ Full | Passed to Anthropic |
| `stream` | ✅ Full | SSE streaming supported |
| `tools` | ✅ Full | Converted to Anthropic tools |
| `tool_choice` | ✅ Full | Converted to Anthropic format |
| `response_format` | ⚠️ Partial | JSON mode supported |
| `stop` | ✅ Full | Converted to Anthropic stop_sequences |

## Troubleshooting

### Gateway Not Starting

```bash
# Check if port 8081 is already in use
lsof -i :8081

# Kill existing process if needed
killall gatewayd

# Or use a different port
export TOKLIGENCE_PORT=8082
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

Make sure you're using the gateway with the tool calling feature:

```bash
# Rebuild after updating
make build

# Verify version includes tool calling
./bin/gatewayd --version
```

## Advanced Configuration

### Multiple Anthropic Keys (Load Balancing)

```bash
# Rotate between multiple keys
TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-key1,sk-ant-key2,sk-ant-key3
```

### Enable Request Logging

```bash
# Log all requests to database
TOKLIGENCE_LOG_REQUESTS=true
./bin/gatewayd
```

### Custom Timeout

```bash
# Set request timeout (in seconds)
TOKLIGENCE_TIMEOUT=60
./bin/gatewayd
```

## Performance Tips

1. **Use Haiku for Fast Responses**: `claude-3-haiku-20240307` is fastest and cheapest
2. **Enable Streaming**: Add `"stream": true` for better perceived latency
3. **Batch Requests**: Use async/concurrent requests for multiple queries
4. **Cache System Messages**: Anthropic supports prompt caching (gateway will pass through)

## Next Steps

- [Gateway Features Documentation](features.md)
- [Quick Start Guide](QUICK_START.md)
- [Claude Code Integration](claude_code-to-openai.md)
- [API Reference](API_REFERENCE.md)

## Support

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Documentation: [Full Documentation](../README.md)
