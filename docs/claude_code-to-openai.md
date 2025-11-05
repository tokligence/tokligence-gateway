# Using Claude Code to Access OpenAI GPT Models via Gateway

This guide shows you how to use Claude Code (Anthropic's VSCode/CLI tool) to access OpenAI's GPT models through the Tokligence Gateway.

## Overview

```
Claude Code → Gateway (Anthropic API format) → OpenAI GPT
```

The gateway accepts Anthropic-native requests and translates them to OpenAI's API, allowing Claude Code to use GPT models seamlessly.

## Prerequisites

- Tokligence Gateway installed and running
- OpenAI API key
- Claude Code installed (VSCode extension or CLI)

## Step 1: Configure Gateway

### Create or Edit `.env` File

```bash
cd /path/to/tokligence-gateway

# Create .env file with your configuration
cat > .env <<'EOF'
# OpenAI API Key (required for accessing GPT models)
TOKLIGENCE_OPENAI_API_KEY=sk-proj-YOUR_OPENAI_KEY_HERE

# Route GPT models to OpenAI
# Note: Use a pattern that won't conflict with actual Claude model names
TOKLIGENCE_ROUTES=gpt-*=>openai,o1-*=>openai,text-*=>openai

# Optional: Disable marketplace features
TOKLIGENCE_MARKETPLACE_ENABLED=false

# Optional: Gateway port (default: 8081)
TOKLIGENCE_PORT=8081
EOF
```

### Configuration Parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `TOKLIGENCE_OPENAI_API_KEY` | Your OpenAI API key | Yes | - |
| `TOKLIGENCE_ROUTES` | Model routing rules | Yes | - |
| `TOKLIGENCE_PORT` | Gateway listening port | No | 8081 |
| `TOKLIGENCE_MARKETPLACE_ENABLED` | Enable marketplace features | No | false |

### Important: Model Naming Strategy

Since Claude Code expects to use Claude model names, you need to create a mapping strategy:

**Option A: Use GPT Model Names Directly**
```bash
TOKLIGENCE_ROUTES=gpt-*=>openai,o1-*=>openai
```

Then in Claude Code, you'll specify GPT model names like `gpt-4-turbo-preview`.

**Option B: Create Aliases (Recommended)**
```bash
# This would require modifying the gateway to support model aliases
# For now, use Option A and specify GPT model names
```

## Step 2: Start Gateway

```bash
# Build the gateway (first time only)
make build

# Start the gateway daemon
./bin/gatewayd
```

Or use environment variables directly:

```bash
export TOKLIGENCE_OPENAI_API_KEY=sk-proj-YOUR_KEY_HERE
export TOKLIGENCE_ROUTES='gpt-*=>openai,o1-*=>openai'
export TOKLIGENCE_MARKETPLACE_ENABLED=false
./bin/gatewayd
```

You should see output like:
```
2025/11/05 10:30:00 Starting Tokligence Gateway on :8081
2025/11/05 10:30:00 Loaded routes: gpt-*=>openai,o1-*=>openai
```

## Step 3: Configure Claude Code

### Option A: Using Claude Code CLI

Create or edit your Claude Code configuration:

```bash
# Create Claude Code config directory
mkdir -p ~/.config/claude-code

# Create config file
cat > ~/.config/claude-code/config.json <<'EOF'
{
  "api": {
    "baseURL": "http://localhost:8081/anthropic",
    "model": "gpt-4-turbo-preview"
  }
}
EOF
```

### Option B: Using Environment Variables

```bash
# Set Claude Code to use gateway
export ANTHROPIC_BASE_URL=http://localhost:8081/anthropic
export ANTHROPIC_MODEL=gpt-4-turbo-preview
```

### Option C: Claude Code VSCode Extension

1. Open VSCode Settings (Cmd+, or Ctrl+,)
2. Search for "Claude Code"
3. Set the following:
   - **API Base URL**: `http://localhost:8081/anthropic`
   - **Default Model**: `gpt-4-turbo-preview`

## Step 4: Test the Integration

### Using curl to Test Anthropic Format → OpenAI

```bash
curl -X POST http://localhost:8081/anthropic/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4-turbo-preview",
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
  "model": "gpt-4-turbo-preview",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 12
  }
}
```

### Using Claude Code CLI

```bash
# Start a conversation using GPT-4
claude-code --model gpt-4-turbo-preview "Explain what you are"
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
    "model": "gpt-4-turbo-preview",
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
# View daemon logs
tail -f logs/gatewayd.log

# View specific date logs
cat logs/gatewayd-2025-11-05.log
```

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

Ensure your routing configuration includes the model pattern:

```bash
# This will NOT work with GPT models
TOKLIGENCE_ROUTES=claude*=>anthropic

# This WILL work with GPT models
TOKLIGENCE_ROUTES=gpt-*=>openai,claude*=>anthropic
```

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

### Support Multiple Providers

You can route different models to different providers:

```bash
# Claude models → Anthropic, GPT models → OpenAI
TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-xxx
TOKLIGENCE_OPENAI_API_KEY=sk-proj-xxx
TOKLIGENCE_ROUTES=claude*=>anthropic,gpt-*=>openai,o1-*=>openai

./bin/gatewayd
```

Then you can switch between providers by changing the model name in Claude Code:
- Use `gpt-4-turbo-preview` for OpenAI
- Use `claude-3-5-sonnet-20241022` for Anthropic

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
