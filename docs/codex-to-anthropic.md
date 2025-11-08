# Using OpenAI Codex to Access Anthropic Claude via Gateway

This guide shows you how to use OpenAI Codex (or any OpenAI-compatible client) to access Anthropic's Claude models through the Tokligence Gateway.

## Overview

```
OpenAI Codex → Gateway (OpenAI API format) → Anthropic Claude
```

The gateway accepts OpenAI-compatible requests and translates them to Anthropic's API, including full support for tool calling (function calling).

## TL;DR — Point Codex at the Gateway

Codex talks OpenAI’s API. Point it at the gateway’s OpenAI path (`/v1`).

Option A — environment variables (recommended)

```bash
# Tell Codex to use the gateway instead of api.openai.com
export OPENAI_BASE_URL=http://localhost:8081/v1

# Codex will send this as Authorization: Bearer ...
# If gateway auth is disabled (dev), any non-empty string is fine
export OPENAI_API_KEY=dummy

# Example: run Codex as usual; choose a Claude model name so the gateway routes to Anthropic
codex --config model="claude-3-haiku-20240307"

# If you see 404 Not Found, Codex is likely using the OpenAI Responses API.
# Force Chat Completions by registering a provider with wire_api="chat":
codex \
  --config 'model="claude-3-haiku-20240307"' \
  --config 'model_provider="openai-gateway"' \
  --config 'model_providers.openai-gateway={ name="OpenAI via Gateway", base_url="http://localhost:8081/v1", env_key="OPENAI_API_KEY", wire_api="chat" }'
```

Option B — config file (~/.codex/config.toml)

```toml
# Use a Claude model name so the gateway routes to Anthropic
model = "claude-3-5-sonnet-20241022"

# Keep built-in provider = "openai"; override its base URL via env var OPENAI_BASE_URL
# Alternatively, you can register a new provider key:
#
# model_provider = "openai-gateway"
# [model_providers.openai-gateway]
# name = "OpenAI via Gateway"
# base_url = "http://localhost:8081/v1"
# env_key  = "OPENAI_API_KEY"
# wire_api = "chat"
```

## Prerequisites

- Tokligence Gateway installed and running
- Anthropic API key
- OpenAI Codex or OpenAI-compatible client

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
# Note: listen address is configured via INI key http_address, not env
```

### Key parameters

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| `TOKLIGENCE_ANTHROPIC_API_KEY` | Your Anthropic API key | Yes | - |
| `TOKLIGENCE_ROUTES` | Model routing rules | Yes | - |
| `http_address` (INI) | Gateway listening address | No | `:8081` |
| `TOKLIGENCE_MARKETPLACE_ENABLED` | Enable marketplace features | No | false |

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

### Streaming example (curl)

```bash
curl -N -X POST http://localhost:8081/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-haiku-20240307",
    "messages": [{"role": "user", "content": "Count from 1 to 5"}],
    "stream": true,
    "max_tokens": 100
  }'
```

## Step 4: Using Tool Calling (Function Calling)

The gateway fully supports OpenAI's function calling with automatic conversion to Anthropic's tools format.

---

# Translation Architecture (How It Works)

The current bridge is the outcome of the refactor plan described in `refactor.md` / `wip.md`. The high‑level pipeline looks like:

```
OpenAI request (Chat / Responses)            ─► ingress parser
                                            └─► canonical Conversation (Base + Chat clone)
Conversation + session state                ─► responses_stream orchestrator
                                            └─► StreamProvider (Anthropic messages)
Anthropic SSE (Messages API)                ─► translator (anthropic.Stream* helpers)
                                            └─► OpenAI-style SSE back to Codex
```

Key components:

| Layer | Files | Responsibilities |
|-------|-------|------------------|
| Ingress / parsing | `internal/openai/response.go`, `internal/httpserver/endpoint_responses.go` | normalize OpenAI Chat / Responses payloads, convert `response_format`, etc. |
| Session manager | `internal/httpserver/responses_handler.go` | stores per-`resp_*` state (adapter, translated chat history, pending tool outputs). Handles `/v1/responses/{id}/submit_tool_outputs` and continuation heuristics. |
| Stream orchestrator | `internal/httpserver/responses_stream.go` | owns the SSE writer, emits `response.*` events, and coordinates tool-call pauses vs completions. |
| Provider / translator | `internal/httpserver/responses/provider_anthropic.go`, `internal/httpserver/openai/responses/translator.go`, `internal/httpserver/anthropic/*.go` | convert canonical chat to Anthropic Native Messages, stream back into OpenAI deltas. |

Both Chat Completions and Responses API eventually flow through the same `responses_stream` code-path when streaming is enabled. Non-streaming responses short-circuit after translation (`forwardResponsesToAnthropic`).

## Where to Touch When the API Changes

| Change type | Recommended touch points |
|-------------|-------------------------|
| **New / removed OpenAI request fields** | `internal/openai/response.go` (struct definitions + `ToChatCompletionRequest`) and `internal/httpserver/endpoint_responses.go` (validation). |
| **New event types / streaming semantics** | `internal/httpserver/responses_stream.go` (SSE orchestration), plus tests in `internal/httpserver/responses_stream_test.go`. |
| **Anthropic Messages schema changes** | `internal/httpserver/anthropic/native.go` (conversion helpers) and `internal/httpserver/anthropic/stream*.go`. |
| **Tool call JSON structure** | `internal/httpserver/responses_stream.go` (argument normalization) and `internal/httpserver/responses_handler.go` (session bookkeeping). |
| **Provider routing / multi-port config** | `internal/httpserver/server.go`, `internal/httpserver/endpoint_responses.go`, and the config INI/ENV parsing in `cmd/gatewayd/main.go`. |

Adding new upstream fields is usually easiest if you extend the canonical OpenAI structures first, then propagate them through the `Conversation` clone and the translator. Regression tests live close to the code they guard—use the existing ones as templates.

## Current Limitations

- **apply_patch / update_plan**: The gateway only exposes OpenAI-compatible tool adapters. Codex’s higher-level abstractions (`apply_patch`, `update_plan`, MCP steps, etc.) are not translated today, so Claude cannot directly mutate files or plans through those helpers. Contributors can add support by extending the tool adapter layer (`internal/httpserver/tooladapter/…`) and teaching `responses_handler.go` to surface the new tools.
- **Missing POSIX helpers**: We only normalize shell commands that arrive as strings or single-element arrays. Complex multi-command arrays (e.g., multiple argv entries with redirection handled outside the shell) will run as-is; if they fail under Linux sandbox, they must be rewritten by the model.
- **Model coverage**: The router currently rewrites `claude-*` names to the Anthropic adapter. If OpenAI releases new `Responses` variants or Anthropic introduces structured reasoning fields, the translator must be updated manually.
- **Session TTL**: Sessions live in-memory inside `gatewayd`. A process restart clears all pending tool calls; Codex must retry from scratch. There is no persistent store.
- **Error filtering**: We drop tool outputs that contain phrases like “unsupported call” or argument parsing errors to avoid infinite loops. This is heuristic—if Anthropic changes the wording, we may need to adjust.

## Future Improvements (Roadmap Ideas)

These mirror the “Phase” sections in `refactor.md`:

1. **Persistent session store** – back sessions by redis/sqlite so restarts don’t strand Codex.
2. **Pluggable tool adapters** – expose a registry where contributors can declare synthetic tools (e.g., `apply_patch`, `update_plan`, MCP) and map them to Anthropic-safe equivalents.
3. **Dynamic routing strategy** – use translator hints to decide between passthrough vs translation vs OpenAI delegation (`responsesDelegateOpenAI` currently hard-codes behaviors).
4. **Schema versioning / diff-friendly translators** – wrap Anthropic converter output in versioned structs so when fields change we can toggle behavior via config.
5. **Better sandbox hinting** – detect when Anthropic returns commands requiring “justification” / `with_escalated_permissions` and surface that to Codex earlier (currently logged only).

Contributors interested in these areas should start by reading `wip.md` (session-aware refactor status) and reviewing the relevant packages listed above. Each package includes targeted tests; extend them when adding new behaviors so future API changes stay easy to diff and reason about.

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

Log file base path is controlled by `log_file_daemon` (see INI). With the dev config above:

```bash
tail -f logs/dev-gatewayd.log
# Rotated files look like: logs/dev-gatewayd-YYYY-MM-DD.log
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

- Ensure you are calling the OpenAI-compatible endpoint `/v1/chat/completions` (not the Anthropic `/v1/messages`).
- Verify your request includes a valid `tools` array and optional `tool_choice`.

## Advanced Configuration

- Optional Anthropic base URL: `TOKLIGENCE_ANTHROPIC_BASE_URL=https://api.anthropic.com`
- Anthropic API version (header): `TOKLIGENCE_ANTHROPIC_VERSION=2023-06-01`
- Debug logging: set `log_level=debug` in `config/dev/gateway.ini` (or remove `log_file_daemon` to only log to stdout)

## Performance Tips

1. Use Haiku for speed/cost: `claude-3-haiku-20240307`
2. Enable streaming for lower perceived latency: `"stream": true`
3. Prefer persistent clients (connection reuse) for throughput
4. Keep prompts compact; `max_tokens` budgets latency and costs

## Next Steps

- [Gateway Features Documentation](features.md)
- [Quick Start Guide](QUICK_START.md)
- [Claude Code Integration](claude_code-to-openai.md)
- [API Reference](API_REFERENCE.md)

## Support

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Documentation: [Full Documentation](../README.md)
