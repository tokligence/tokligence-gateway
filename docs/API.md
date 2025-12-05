# Tokligence Gateway API Specification

**Version:** v0.3.0+
**Last Updated:** 2025-11-23

This document describes the public HTTP APIs exposed by Tokligence Gateway for external clients.

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Core LLM Endpoints](#core-llm-endpoints)
   - [OpenAI Chat Completions](#openai-chat-completions)
   - [OpenAI Responses API](#openai-responses-api)
   - [Anthropic Messages API](#anthropic-messages-api)
4. [Priority Scheduling (Optional)](#priority-scheduling-optional)
5. [Health & Admin Endpoints](#health--admin-endpoints)
6. [Error Responses](#error-responses)
7. [Rate Limiting](#rate-limiting)

---

## Overview

Tokligence Gateway provides unified access to multiple LLM providers (OpenAI, Anthropic, Gemini) with:
- **Protocol Translation**: OpenAI ↔ Anthropic bidirectional conversion
- **Model Routing**: Route requests to different providers based on model patterns
- **Priority Scheduling**: Optional request queuing and prioritization (v0.3.0+)
- **Usage Tracking**: Built-in ledger for token accounting

**Base URL:** `http://localhost:8081` (default, configurable via `TOKLIGENCE_FACADE_PORT`)

**Content-Type:** `application/json` for all POST requests

---

## Authentication

### Development Mode (Default)

Authentication is **disabled by default** for local development:

```bash
# config/setting.ini or environment variable
auth_disabled=true
# or
export TOKLIGENCE_AUTH_DISABLED=true
```

Requests can be made without authentication headers.

### Production Mode

When `auth_disabled=false`, all requests require an `Authorization` header:

```http
Authorization: Bearer <your-api-key>
```

**API Key Management:**
- Keys are managed via the identity store (`~/.tokligence/identity.db`)
- Contact admin to provision API keys

---

## Core LLM Endpoints

### OpenAI Chat Completions

**Endpoint:** `POST /v1/chat/completions`

**Description:** OpenAI-compatible chat completions endpoint with model routing.

**Request Headers:**
```http
Content-Type: application/json
Authorization: Bearer <key>  # if auth enabled
X-Priority: 0-9              # optional, for priority scheduling (see below)
```

**Request Body:**
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 150,
  "stream": false
}
```

**Response (Non-Streaming):**
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 20,
    "completion_tokens": 10,
    "total_tokens": 30
  }
}
```

**Streaming Response (`stream: true`):**

Server-Sent Events (SSE) format:

```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","choices":[{"delta":{"content":"!"}}]}

data: [DONE]
```

**Model Routing:**

Gateway automatically routes based on model prefix (configurable via `TOKLIGENCE_ROUTES`):
- `gpt-*`, `o1-*` → OpenAI
- `claude-*` → Anthropic (with protocol translation)
- `gemini-*` → Google Gemini

**Supported Fields:**
- ✅ `messages`, `model`, `temperature`, `max_tokens`, `stream`
- ✅ `tools` (function calling, translated for Anthropic)
- ✅ `response_format` (JSON mode)
- ✅ `top_p`, `frequency_penalty`, `presence_penalty`
- ⚠️  `n > 1` (multiple completions): Only supported for native OpenAI models

---

### OpenAI Responses API

**Endpoint:** `POST /v1/responses`

**Description:** OpenAI's new Responses API for agentic workflows with multi-turn tool calling. Supports stateful sessions.

**Request Headers:**
```http
Content-Type: application/json
Authorization: Bearer <key>  # if auth enabled
X-Priority: 0-9              # optional, for priority scheduling
```

**Request Body:**
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {"role": "user", "content": "What's the weather in SF?"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          }
        }
      }
    }
  ],
  "stream": true
}
```

**Response (SSE Stream):**

```
event: response.created
data: {"type":"response.created","response":{"id":"resp_abc"}}

event: output_item.added
data: {"type":"output_item.added","item_id":"item_1","item":{"type":"message"}}

event: content.delta
data: {"type":"content.delta","item_id":"item_1","delta":{"type":"text","text":"I'll"}}

event: response.done
data: {"type":"response.done","response":{"status":"completed"}}
```

**Session Management:**

Gateway maintains in-memory sessions for multi-turn tool calling:
1. Client sends initial request
2. Gateway streams back tool calls (if any)
3. Client submits tool results via `tool_outputs`
4. Gateway continues conversation automatically

Sessions are ephemeral (cleared on restart).

**Model Translation:**

When using Anthropic models (e.g., `claude-3-5-sonnet-20241022`), Gateway:
- Translates OpenAI Responses API format → Anthropic Messages API
- Converts SSE events back to OpenAI Responses format
- Filters unsupported tools (`apply_patch`, `update_plan` removed for Anthropic)

---

### Anthropic Messages API

**Endpoint:** `POST /anthropic/v1/messages`

**Description:** Native Anthropic Messages API endpoint with optional OpenAI backend translation.

**Request Headers:**
```http
Content-Type: application/json
anthropic-version: 2023-06-01
x-api-key: <anthropic-key>  # or Authorization: Bearer <key>
X-Priority: 0-9               # optional, for priority scheduling
```

**Request Body:**
```json
{
  "model": "gpt-4o",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "Hello!"}
  ]
}
```

**Response:**
```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "Hello! How can I help?"}
  ],
  "model": "gpt-4o",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 8
  }
}
```

**Sidecar Translation:**

When using OpenAI models (e.g., `gpt-4o`) via Anthropic API:
- Gateway translates Anthropic format → OpenAI Chat Completions
- Converts response back to Anthropic format
- Useful for tools/clients that only speak Anthropic protocol

**Model Mapping (Configurable):**

```bash
# config/setting.ini or env var
sidecar_model_map=claude-3-5-sonnet-20241022=gpt-4o
```

---

## Priority Scheduling (Optional)

**Feature Status:** ✅ Available in v0.3.0+ (disabled by default)

When scheduler is enabled (`scheduler_enabled=true` in `config/scheduler.ini`), all LLM endpoints support request prioritization.

### Setting Priority

Add `X-Priority` header to any LLM request:

```http
X-Priority: 0    # P0 = Critical (highest)
X-Priority: 2    # P2 = High
X-Priority: 5    # P5 = Normal (default if header missing)
X-Priority: 9    # P9 = Background (lowest)
```

**Priority Levels (Default 10-level system):**

| Level | Name | Use Case | Example |
|-------|------|----------|---------|
| **P0** | Critical | Internal services, health checks | Monitoring, alerts |
| **P1** | Urgent | VIP user emergencies | Enterprise escalations |
| **P2** | High | Premium users | Paid tier customers |
| **P3** | Elevated | Business tier | - |
| **P4** | Above Normal | Pro tier | - |
| **P5** | Normal | Standard users (default) | Free tier |
| **P6** | Below Normal | Rate-limited users | - |
| **P7** | Low | Batch API | - |
| **P8** | Bulk | Batch processing | Data exports |
| **P9** | Background | Background jobs | Analytics, cleanup |

### Scheduling Behavior

**When Capacity Available:**
- Request executes immediately regardless of priority

**When At Capacity:**
- Request enters priority queue
- Higher priority requests scheduled first (P0 before P9)
- Weighted Fair Queuing (WFQ) prevents starvation of low-priority requests
- Response includes queue position in logs

**Queue Timeout:**
- Default: 30 seconds (configurable via `scheduler_queue_timeout_sec`)
- If request waits beyond timeout, returns `503 Service Unavailable`

### Response Headers (When Queued)

```http
X-Tokligence-Queue-Position: 3
X-Tokligence-Wait-Time-Ms: 1250
```

### Error Responses

**503 Service Unavailable - Queue Full:**
```json
{
  "error": {
    "message": "Queue full: priority queue P5 exceeded max depth 1000",
    "type": "queue_full",
    "code": 503
  }
}
```

**503 Service Unavailable - Queue Timeout:**
```json
{
  "error": {
    "message": "Request timeout: waited 31s in queue (timeout: 30s)",
    "type": "queue_timeout",
    "code": 503
  }
}
```

**429 Too Many Requests - Capacity Exceeded:**
```json
{
  "error": {
    "message": "Capacity exceeded: concurrent limit reached (100/100)",
    "type": "capacity_exceeded",
    "code": 429
  }
}
```

### Configuration

See `config/scheduler.ini` for full configuration options:
- Priority levels (5-20, default: 10)
- Scheduling policy (strict/wfq/hybrid)
- Capacity limits (tokens/sec, RPS, concurrent)
- Queue depth and timeout

**Environment Variable Override:**
```bash
export TOKLIGENCE_SCHEDULER_ENABLED=true
export TOKLIGENCE_SCHEDULER_MAX_CONCURRENT=50
export TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS=10
```

---

## Health & Admin Endpoints

### Health Check

**Endpoint:** `GET /health`

**Response:**
```json
{
  "status": "ok",
  "version": "v0.3.0",
  "uptime": "24h15m30s"
}
```

**Status Codes:**
- `200 OK` - Gateway is healthy
- `503 Service Unavailable` - Gateway is unhealthy

---

## Error Responses

All error responses follow OpenAI error format:

```json
{
  "error": {
    "message": "Invalid request: missing required field 'model'",
    "type": "invalid_request_error",
    "param": "model",
    "code": null
  }
}
```

**Common Error Types:**

| HTTP Code | Error Type | Description |
|-----------|------------|-------------|
| 400 | `invalid_request_error` | Malformed request body or missing required fields |
| 401 | `authentication_error` | Missing or invalid API key |
| 403 | `permission_denied` | API key does not have permission |
| 404 | `not_found` | Endpoint or model not found |
| 429 | `rate_limit_exceeded` | Too many requests (rate limiting) |
| 429 | `capacity_exceeded` | Scheduler capacity limit reached |
| 500 | `internal_error` | Internal server error |
| 503 | `service_unavailable` | Temporarily unavailable (e.g., queue full, timeout) |

---

## Rate Limiting

Gateway supports token bucket rate limiting (configured per account).

**Rate Limit Headers:**
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1234567890
```

**429 Response:**
```json
{
  "error": {
    "message": "Rate limit exceeded: 100 requests per minute",
    "type": "rate_limit_exceeded",
    "code": 429
  }
}
```

**Configuration:**

Rate limits are managed via gateway configuration, not exposed in public API.

---

## Notes

### Model Names

Gateway supports model name aliases and routing:
- Use OpenAI model names (`gpt-4o`, `gpt-4o-mini`) for OpenAI backend
- Use Anthropic model names (`claude-3-5-sonnet-20241022`) for Anthropic backend
- Gateway automatically routes based on configured patterns

### Token Accounting

All requests are logged to the internal ledger (`~/.tokligence/ledger.db`) with:
- Model name
- Prompt tokens
- Completion tokens
- Latency
- Account ID

### Streaming Best Practices

1. Always set `stream: true` for better latency perception
2. Handle SSE connection drops gracefully with retry logic
3. Parse `data: [DONE]` to detect stream completion

### Tool Calling

- Gateway supports OpenAI function calling format
- Automatically translates to Anthropic tool use format when needed
- Some tools may be filtered for compatibility (e.g., `apply_patch` not supported on Anthropic)

---

## Version History

- **v0.3.0** (2025-11-23): Added priority scheduling support
- **v0.2.0** (2024-11): Added OpenAI Responses API, Gemini integration
- **v0.1.0** (2024-10): Initial release with OpenAI ↔ Anthropic translation

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/tokligence/tokligence-gateway/issues
- Documentation: https://github.com/tokligence/tokligence-gateway/docs/

---

**End of API Specification**
