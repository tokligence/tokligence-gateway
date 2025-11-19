# Google Gemini Integration Guide

This guide explains how to use Tokligence Gateway with Google Gemini API, including both native Gemini endpoints and OpenAI-compatible endpoints.

## Overview

Tokligence Gateway provides **pass-through proxy** support for Google Gemini API, enabling you to:

- Access Gemini models through a unified gateway interface
- Use native Gemini API endpoints (`/v1beta/models/*`)
- Use OpenAI-compatible endpoints with Gemini models
- Centralize logging and usage tracking for Gemini requests
- Run Gemini endpoints on a dedicated port (8084) for isolation

### Architecture

```
┌──────────────────────────────────────────┐
│         Your Application                 │
│  (Gemini SDK or OpenAI SDK)              │
└──────────────────────────────────────────┘
                    ▼
┌──────────────────────────────────────────┐
│   Tokligence Gateway (:8084)             │
│  ─────────────────────────────────       │
│  • Native Gemini API                     │
│    /v1beta/models/{model}:method         │
│  • OpenAI-compatible API                 │
│    /v1beta/openai/chat/completions       │
└──────────────────────────────────────────┘
                    ▼
          ┌──────────────────┐
          │  Google Gemini   │
          │       API        │
          └──────────────────┘
```

## Configuration

### Environment Variables

Set up your Gemini API credentials and port configuration:

```bash
# Gemini API Key (required)
export TOKLIGENCE_GEMINI_API_KEY=your_gemini_api_key_here

# Optional: Custom Gemini API base URL (defaults to Google's official URL)
export TOKLIGENCE_GEMINI_BASE_URL=https://generativelanguage.googleapis.com

# Multi-port mode configuration
export TOKLIGENCE_MULTIPORT_MODE=true
export TOKLIGENCE_GEMINI_PORT=8084
export TOKLIGENCE_GEMINI_ENDPOINTS=gemini_native,health
```

### Configuration File

Alternatively, add to your `config/dev/gateway.ini`:

```ini
[gateway]
gemini_api_key = your_gemini_api_key_here
gemini_base_url = https://generativelanguage.googleapis.com
gemini_port = 8084
gemini_endpoints = gemini_native,health

multiport_mode = true
```

### Get a Gemini API Key

1. Visit [Google AI Studio](https://ai.google.dev/)
2. Sign in with your Google account
3. Click "Get API Key"
4. Copy your API key to use in configuration

## Available Endpoints

### Native Gemini API Endpoints

The gateway exposes all standard Gemini API endpoints at `/v1beta/`:

#### 1. Generate Content (Non-streaming)

**Endpoint:** `POST /v1beta/models/{model}:generateContent`

**Example:**
```bash
curl -X POST 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp:generateContent' \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{
        "text": "Explain quantum computing in simple terms"
      }]
    }]
  }'
```

**Response:**
```json
{
  "candidates": [{
    "content": {
      "parts": [{
        "text": "Quantum computing is..."
      }],
      "role": "model"
    },
    "finishReason": "STOP"
  }],
  "usageMetadata": {
    "promptTokenCount": 8,
    "candidatesTokenCount": 150,
    "totalTokenCount": 158
  }
}
```

#### 2. Stream Generate Content (Streaming)

**Endpoint:** `POST /v1beta/models/{model}:streamGenerateContent?alt=sse`

**Example:**
```bash
curl -N -X POST 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp:streamGenerateContent?alt=sse' \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{
        "text": "Write a haiku about coding"
      }]
    }]
  }'
```

**SSE Response:**
```
data: {"candidates":[{"content":{"parts":[{"text":"Code"}]}}]}
data: {"candidates":[{"content":{"parts":[{"text":" flows"}]}}]}
data: {"candidates":[{"content":{"parts":[{"text":" like"}]}}]}
...
```

#### 3. Count Tokens

**Endpoint:** `POST /v1beta/models/{model}:countTokens`

**Example:**
```bash
curl -X POST 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp:countTokens' \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{
        "text": "How many tokens is this?"
      }]
    }]
  }'
```

**Response:**
```json
{
  "totalTokens": 5
}
```

#### 4. List Models

**Endpoint:** `GET /v1beta/models`

**Example:**
```bash
curl 'http://localhost:8084/v1beta/models'
```

**Response:**
```json
{
  "models": [
    {
      "name": "models/gemini-2.0-flash-exp",
      "displayName": "Gemini 2.0 Flash Experimental",
      "description": "Experimental version of Gemini 2.0 Flash"
    },
    {
      "name": "models/gemini-1.5-pro",
      "displayName": "Gemini 1.5 Pro",
      "description": "Mid-size multimodal model"
    }
  ]
}
```

#### 5. Get Model Info

**Endpoint:** `GET /v1beta/models/{model}`

**Example:**
```bash
curl 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp'
```

### OpenAI-Compatible Endpoint

Google Gemini provides an OpenAI-compatible endpoint for easier migration from OpenAI.

#### Chat Completions (Non-streaming)

**Endpoint:** `POST /v1beta/openai/chat/completions`

**Example:**
```bash
curl -X POST 'http://localhost:8084/v1beta/openai/chat/completions' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [
      {"role": "user", "content": "Say hello in French"}
    ]
  }'
```

**Response (OpenAI format):**
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gemini-2.0-flash-exp",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Bonjour!"
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 5,
    "completion_tokens": 3,
    "total_tokens": 8
  }
}
```

#### Chat Completions (Streaming)

**Endpoint:** `POST /v1beta/openai/chat/completions` with `"stream": true`

**Example:**
```bash
curl -N -X POST 'http://localhost:8084/v1beta/openai/chat/completions' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [
      {"role": "user", "content": "Count to 5"}
    ],
    "stream": true
  }'
```

**SSE Response (OpenAI format):**
```
data: {"id":"chatcmpl-...","choices":[{"delta":{"content":"1"}}]}
data: {"id":"chatcmpl-...","choices":[{"delta":{"content":" 2"}}]}
data: {"id":"chatcmpl-...","choices":[{"delta":{"content":" 3"}}]}
data: [DONE]
```

## Supported Models

Common Gemini models available:

| Model Name | Description | Context Window |
|------------|-------------|----------------|
| `gemini-2.0-flash-exp` | Experimental Gemini 2.0 Flash | Large |
| `gemini-1.5-pro` | Gemini 1.5 Pro (production) | 2M tokens |
| `gemini-1.5-flash` | Gemini 1.5 Flash (fast) | 1M tokens |
| `gemini-1.5-flash-8b` | Gemini 1.5 Flash 8B (small) | 1M tokens |

For the latest model list, use the List Models endpoint.

## Usage Examples

### Python with Google Generative AI SDK

```python
import google.generativeai as genai

# Configure to use gateway
genai.configure(
    api_key="your_api_key",
    transport="rest",
    client_options={"api_endpoint": "http://localhost:8084"}
)

model = genai.GenerativeModel('gemini-2.0-flash-exp')
response = model.generate_content("Explain AI in simple terms")
print(response.text)
```

### Python with OpenAI SDK (via OpenAI-compatible endpoint)

```python
from openai import OpenAI

client = OpenAI(
    api_key="your_gemini_api_key",
    base_url="http://localhost:8084/v1beta/openai"
)

response = client.chat.completions.create(
    model="gemini-2.0-flash-exp",
    messages=[
        {"role": "user", "content": "Hello in Spanish"}
    ]
)

print(response.choices[0].message.content)
```

### cURL Examples

#### Native Gemini API
```bash
curl -X POST 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp:generateContent' \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "What is the capital of France?"}]
    }]
  }'
```

#### OpenAI-compatible API
```bash
curl -X POST 'http://localhost:8084/v1beta/openai/chat/completions' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemini-2.0-flash-exp",
    "messages": [{"role": "user", "content": "What is 2+2?"}]
  }'
```

## Authentication

The gateway handles authentication automatically. You configure your Gemini API key once in the gateway, and clients don't need to provide it:

### Native Gemini Endpoints
- Gateway adds `?key=YOUR_API_KEY` query parameter automatically
- Clients don't send API keys

### OpenAI-Compatible Endpoint
- Gateway adds `Authorization: Bearer YOUR_API_KEY` header automatically
- Clients don't send API keys

This centralizes credential management and improves security.

## Features

### ✅ Supported Features

- **Non-streaming Generation**: Full support for `generateContent`
- **Streaming Generation**: Full support for `streamGenerateContent` with SSE
- **Token Counting**: Native `countTokens` endpoint
- **Model Listing**: List and get model metadata
- **OpenAI Compatibility**: Use Gemini with OpenAI SDK
- **Error Handling**: Proper error translation from Gemini API
- **Timeout Configuration**: Configurable request timeout (default: 120s)

### 🔄 Pass-through Proxy

The Gemini integration uses a **pass-through proxy** approach:

- **No protocol translation**: Requests are forwarded directly to Gemini
- **Minimal latency**: No transformation overhead
- **API fidelity**: 100% compatible with official Gemini API
- **Dual format support**: Both native and OpenAI-compatible formats

## Rate Limits and Quotas

Gemini API has rate limits and quotas. If you encounter quota errors:

```json
{
  "error": {
    "code": 429,
    "message": "You exceeded your current quota...",
    "status": "RESOURCE_EXHAUSTED"
  }
}
```

**Solutions:**
1. Wait for the retry period specified in the error
2. Upgrade your Gemini API quota at [Google AI Studio](https://ai.google.dev/)
3. Implement retry logic with exponential backoff

## Troubleshooting

### Issue: "Missing API key" error

**Cause:** Gemini API key not configured

**Solution:**
```bash
export TOKLIGENCE_GEMINI_API_KEY=your_api_key_here
make gfr  # Force restart gateway
```

### Issue: Port 8084 already in use

**Cause:** Another service is using the Gemini port

**Solution:**
```bash
# Check what's using the port
sudo lsof -i :8084

# Change to a different port
export TOKLIGENCE_GEMINI_PORT=8085
make gfr
```

### Issue: Quota exceeded

**Cause:** Gemini API free tier limits reached

**Solution:**
- Wait for quota reset (typically per minute)
- Check usage at [Google AI usage dashboard](https://ai.dev/usage)
- Consider upgrading to a paid tier

### Issue: Connection timeout

**Cause:** Gemini API is slow or network issues

**Solution:**
```bash
# Increase timeout (e.g., to 180 seconds)
# In config/dev/gateway.ini:
gemini_request_timeout = 180
```

## Best Practices

1. **API Key Security**: Store API keys in environment variables, never in code
2. **Error Handling**: Implement retry logic for transient errors (429, 503)
3. **Rate Limiting**: Respect Gemini's rate limits to avoid quota exhaustion
4. **Model Selection**: Use `gemini-1.5-flash` for speed, `gemini-1.5-pro` for quality
5. **Streaming**: Use streaming for long responses to improve perceived latency
6. **Token Counting**: Use `countTokens` before generation to estimate costs

## Performance Considerations

- **Latency**: Gateway adds minimal overhead (<1ms) due to pass-through design
- **Timeout**: Default 120s timeout accommodates Gemini's generation time
- **Concurrency**: Gateway handles multiple concurrent Gemini requests efficiently
- **SSE Buffering**: Streaming responses use 8KB buffers for optimal throughput

## Comparison: Native vs OpenAI-Compatible

| Feature | Native Gemini API | OpenAI-Compatible API |
|---------|-------------------|----------------------|
| **Endpoint** | `/v1beta/models/{model}:method` | `/v1beta/openai/chat/completions` |
| **Request Format** | Gemini `contents` format | OpenAI `messages` format |
| **Response Format** | Gemini `candidates` | OpenAI `choices` |
| **Authentication** | Query param `?key=` | Bearer token header |
| **Use Case** | Google SDK, native features | OpenAI SDK migration |
| **Streaming** | `streamGenerateContent?alt=sse` | `stream: true` parameter |

**Choose Native API when:**
- Using Google's official SDKs
- Need Gemini-specific features
- Building new applications

**Choose OpenAI-Compatible when:**
- Migrating from OpenAI
- Using OpenAI SDK
- Want unified code for multiple providers

## Related Documentation

- [Gemini API Official Docs](https://ai.google.dev/gemini-api/docs)
- [OpenAI Compatibility](https://ai.google.dev/gemini-api/docs/openai)
- [Rate Limits](https://ai.google.dev/gemini-api/docs/rate-limits)
- [Tokligence Gateway Quick Start](QUICK_START.md)

## Support

For issues specific to the gateway integration:
- [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)

For Gemini API issues:
- [Google AI Studio](https://ai.google.dev/)
- [Gemini API Documentation](https://ai.google.dev/gemini-api/docs)
