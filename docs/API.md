# Tokligence Gateway API Specification

**Version:** v0.3.4+
**Last Updated:** 2025-11-24

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
5. [Account Quota Management (Team Edition)](#account-quota-management-team-edition)
   - [List Account Quotas](#list-account-quotas)
   - [Create Account Quota](#create-account-quota)
   - [Update Account Quota](#update-account-quota)
   - [Delete Account Quota](#delete-account-quota)
   - [Get Account Quota Status](#get-account-quota-status)
6. [Time-Based Dynamic Rules (Phase 3)](#time-based-dynamic-rules-phase-3)
   - [Get Time Rules Status](#get-time-rules-status)
   - [Manually Trigger Rule Evaluation](#manually-trigger-rule-evaluation)
7. [Health & Admin Endpoints](#health--admin-endpoints)
8. [Error Responses](#error-responses)
9. [Rate Limiting](#rate-limiting)

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

## Account Quota Management (Team Edition)

**Feature Status:** ✅ Available in v0.3.4+ (Team Edition only, requires PostgreSQL)

Account quota management allows administrators to define and enforce per-account usage limits across multiple dimensions (tokens, USD, TPS, RPM). Quotas support hierarchical scoping (account → team → environment) and multiple enforcement types.

**Prerequisites:**
- PostgreSQL database (configured via `TOKLIGENCE_IDENTITY_PATH`)
- `account_quota_enabled=true` in configuration
- Admin API access (`/admin/account-quotas` endpoints)

### Configuration

**Enable quota management in `config/setting.ini` or environment variables:**

```bash
export TOKLIGENCE_ACCOUNT_QUOTA_ENABLED=true
export TOKLIGENCE_ACCOUNT_QUOTA_SYNC_SEC=60  # Sync interval for in-memory → DB persistence
```

**Quota Types:**
- `hard` - Strict limit, reject at boundary
- `soft` - Warning at limit, reject at 120%
- `reserved` - Guaranteed minimum capacity
- `burstable` - Can borrow from others

**Limit Dimensions:**
- `tokens_per_month` - Monthly token quota
- `tokens_per_day` - Daily token quota
- `tokens_per_hour` - Hourly token quota
- `usd_per_month` - Monthly cost quota (USD)
- `tps` - Tokens per second (rate limit)
- `rpm` - Requests per minute (rate limit)

---

### List Account Quotas

**Endpoint:** `GET /admin/account-quotas`

**Query Parameters:**
- `account_id` (optional) - Filter by account ID

**Request:**
```bash
curl "http://localhost:8081/admin/account-quotas?account_id=test-account-1" \
  -H "Authorization: Bearer <admin-key>"
```

**Response:**
```json
{
  "count": 2,
  "quotas": [
    {
      "id": "7424be8e-df97-4ce7-a72a-f2a761460947",
      "account_id": "test-account-1",
      "team_id": null,
      "environment": null,
      "quota_type": "hard",
      "limit_dimension": "tokens_per_day",
      "limit_value": 100000,
      "allow_borrow": false,
      "max_borrow_pct": 0,
      "window_type": "daily",
      "window_start": "2025-11-24T00:00:00+08:00",
      "window_end": null,
      "used_value": 35420,
      "last_sync_at": "2025-11-24T08:30:00+08:00",
      "alert_at_pct": 0.80,
      "alert_webhook_url": "https://example.com/alerts",
      "alert_triggered": false,
      "last_alert_at": null,
      "description": "Daily token quota for production account",
      "enabled": true,
      "created_at": "2025-11-23T10:00:00+08:00",
      "updated_at": "2025-11-24T08:30:00+08:00",
      "created_by": "admin@example.com",
      "updated_by": "admin@example.com",
      "utilization_pct": 35.42,
      "remaining": 64580,
      "is_expired": false,
      "can_borrow": false
    }
  ]
}
```

**Field Descriptions:**
- `utilization_pct` - Current usage percentage (computed)
- `remaining` - Remaining quota capacity (computed)
- `is_expired` - Whether quota window has expired (computed)
- `can_borrow` - Whether quota can borrow capacity (computed)

---

### Create Account Quota

**Endpoint:** `POST /admin/account-quotas`

**Request:**
```bash
curl "http://localhost:8081/admin/account-quotas" -X POST \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "test-account-1",
    "team_id": "team-alpha",
    "environment": "production",
    "quota_type": "hard",
    "limit_dimension": "tokens_per_day",
    "limit_value": 100000,
    "allow_borrow": false,
    "max_borrow_pct": 0.0,
    "window_type": "daily",
    "alert_at_pct": 0.80,
    "alert_webhook_url": "https://example.com/alerts",
    "description": "Daily token quota for production team",
    "enabled": true,
    "created_by": "admin@example.com"
  }'
```

**Response:**
```json
{
  "id": "7424be8e-df97-4ce7-a72a-f2a761460947",
  "message": "Quota created successfully"
}
```

**Required Fields:**
- `account_id` - Account identifier
- `quota_type` - Enforcement type (`hard`, `soft`, `reserved`, `burstable`)
- `limit_dimension` - Quota dimension (e.g., `tokens_per_day`)
- `limit_value` - Quota limit (positive integer)

**Optional Fields:**
- `team_id` - Team identifier (null for account-level quota)
- `environment` - Environment identifier (e.g., `production`, `staging`)
- `allow_borrow` - Whether quota can borrow capacity (default: false)
- `max_borrow_pct` - Max borrow percentage (default: 0.0)
- `window_type` - Time window (`hourly`, `daily`, `monthly`, `custom`)
- `window_start` - Window start time (default: NOW())
- `window_end` - Window end time (null for recurring windows)
- `alert_at_pct` - Alert threshold percentage (default: 0.80)
- `alert_webhook_url` - Webhook URL for alerts
- `description` - Human-readable description
- `enabled` - Whether quota is active (default: true)
- `created_by` - Creator identifier

---

### Update Account Quota

**Endpoint:** `PUT /admin/account-quotas/{id}`

**Request:**
```bash
curl "http://localhost:8081/admin/account-quotas/7424be8e-df97-4ce7-a72a-f2a761460947" -X PUT \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "limit_value": 200000,
    "description": "Increased daily quota for Black Friday",
    "updated_by": "admin@example.com"
  }'
```

**Response:**
```json
{
  "id": "7424be8e-df97-4ce7-a72a-f2a761460947",
  "message": "Quota updated successfully"
}
```

**Updatable Fields:**
- `limit_value` - New quota limit
- `description` - Updated description
- `enabled` - Enable/disable quota
- `alert_at_pct` - Alert threshold
- `alert_webhook_url` - Webhook URL
- `window_start` - Window start time
- `window_end` - Window end time
- `updated_by` - Updater identifier

---

### Delete Account Quota

**Endpoint:** `DELETE /admin/account-quotas/{id}`

**Description:** Performs soft delete (sets `deleted_at` timestamp). Quota remains in database but is excluded from active enforcement.

**Request:**
```bash
curl "http://localhost:8081/admin/account-quotas/7424be8e-df97-4ce7-a72a-f2a761460947" -X DELETE \
  -H "Authorization: Bearer <admin-key>"
```

**Response:**
```json
{
  "message": "Quota deleted successfully"
}
```

---

### Get Account Quota Status

**Endpoint:** `GET /admin/account-quotas/status/{account_id}`

**Description:** Returns all active quotas for an account with current usage statistics.

**Request:**
```bash
curl "http://localhost:8081/admin/account-quotas/status/test-account-1" \
  -H "Authorization: Bearer <admin-key>"
```

**Response:**
```json
{
  "account_id": "test-account-1",
  "count": 2,
  "quotas": [
    {
      "id": "7424be8e-df97-4ce7-a72a-f2a761460947",
      "account_id": "test-account-1",
      "quota_type": "hard",
      "limit_dimension": "tokens_per_day",
      "limit_value": 100000,
      "used_value": 85420,
      "utilization_pct": 85.42,
      "remaining": 14580,
      "is_expired": false,
      "enabled": true,
      "description": "Daily token quota"
    }
  ]
}
```

**Use Case:** Check quota status before making requests, or for dashboard displays.

---

### Error Responses (Quota Management)

**501 Not Implemented - Feature Disabled:**
```json
{
  "error": "Not Implemented",
  "message": "Account quota management is not enabled (Personal Edition)"
}
```

**400 Bad Request - Invalid Input:**
```json
{
  "error": "Bad Request",
  "message": "account_id is required"
}
```

**404 Not Found - Quota Not Found:**
```json
{
  "error": "Not Found",
  "message": "Quota not found or deleted: 7424be8e-df97-4ce7-a72a-f2a761460947"
}
```

**500 Internal Server Error - Database Error:**
```json
{
  "error": "Internal Server Error",
  "message": "Failed to create quota: database connection failed"
}
```

---

### Quota Enforcement Integration

When quota management is enabled, all LLM endpoints (`/v1/chat/completions`, `/v1/responses`, `/anthropic/v1/messages`) automatically check quotas before processing requests:

1. **Pre-Request Check:** Estimate token usage and check against applicable quotas
2. **Hard Limit Rejection:** Return `429 Too Many Requests` if hard limit exceeded
3. **Soft Limit Warning:** Log warning if soft limit exceeded, allow up to 120%
4. **Post-Request Commit:** Update actual token usage after completion

**429 Response (Quota Exceeded):**
```json
{
  "error": {
    "message": "Quota exceeded: account test-account-1 has exhausted daily token quota (100000/100000)",
    "type": "quota_exceeded",
    "code": 429
  }
}
```

**Request Headers for Quota Context:**
- `X-Team-ID` (optional) - Team identifier for team-level quota
- `X-Environment` (optional) - Environment identifier for environment-level quota

---

## Time-Based Dynamic Rules (Phase 3)

**Feature Status:** ✅ Available in v0.3.4+ (All Editions)

Time-based dynamic rules allow automatic adjustment of scheduler behavior, quotas, and capacity based on time of day, day of week, and timezones. This enables organizations to optimize resource allocation between internal departments and external customers, and to respond to predictable usage patterns.

**Use Cases:**
- Prioritize internal departments during business hours, external customers at night
- Increase capacity during peak lunch hours
- Reduce capacity during early morning to save resources
- Adjust quotas based on time-of-day patterns

**Prerequisites:**
- Scheduler enabled (`TOKLIGENCE_SCHEDULER_ENABLED=true`)
- Rule configuration file (`config/scheduler_time_rules.ini`)
- `time_rules_enabled=true` in configuration

### Configuration

**Enable time-based rules in `config/setting.ini` or environment variables:**

```bash
export TOKLIGENCE_TIME_RULES_ENABLED=true
export TOKLIGENCE_TIME_RULES_CONFIG=config/scheduler_time_rules.ini
```

**Rule configuration file (`config/scheduler_time_rules.ini`):**

```ini
[time_rules]
enabled = true
check_interval_sec = 60           # How often to evaluate rules
default_timezone = Asia/Singapore # Default timezone for all rules

# Weight Adjustment Rule (affects scheduler priority weights)
[rule.weights.daytime]
type = weight_adjustment
name = Internal Priority (Daytime)
description = Boost internal department priorities during business hours
enabled = true
start_time = 08:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri
# Weights for P0-P9 (higher = more share of capacity)
weights = 256,128,64,32,16,8,4,2,1,1

# Quota Adjustment Rule (affects per-account quotas)
[rule.quota.daytime]
type = quota_adjustment
name = Daytime Quotas
description = Reserve most capacity for internal departments
enabled = true
start_time = 08:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri
# Pattern matching supports wildcards: prefix-*, *-suffix, *substring*
quota.dept-a-* = concurrent:50,rps:100,tokens_per_sec:2000
quota.premium-* = concurrent:10,rps:20,tokens_per_sec:300

# Capacity Adjustment Rule (affects global scheduler capacity)
[rule.capacity.peak_hours]
type = capacity_adjustment
name = Peak Hours Capacity
description = Increase capacity during lunch hours
enabled = true
start_time = 12:00
end_time = 14:00
days_of_week = Mon,Tue,Wed,Thu,Fri
max_concurrent = 200
max_rps = 500
max_tokens_per_sec = 10000
```

**Rule Types:**
1. **Weight Adjustment** (`weight_adjustment`) - Modifies scheduler priority weights for P0-P9
2. **Quota Adjustment** (`quota_adjustment`) - Adjusts per-account quota limits dynamically
3. **Capacity Adjustment** (`capacity_adjustment`) - Changes global scheduler capacity limits

**Time Window Features:**
- Time-of-day ranges (`start_time`, `end_time` in HH:MM format)
- Midnight-wrapping support (e.g., `18:00-08:00` wraps to next day)
- Day-of-week filtering (`Mon,Tue,Wed,Thu,Fri,Sat,Sun` or omit for all days)
- Timezone support (per-rule or uses `default_timezone`)

---

### Get Time Rules Status

**Endpoint:** `GET /admin/time-rules/status`

Returns the current status of all configured time-based rules, including which rules are currently active.

**Request:**
```bash
curl "http://localhost:8081/admin/time-rules/status" \
  -H "Authorization: Bearer <admin-key>"
```

**Response:**
```json
{
  "enabled": true,
  "count": 7,
  "rules": [
    {
      "name": "Internal Priority (Daytime)",
      "type": "weight_adjustment",
      "active": false,
      "window": "08:00-18:00 [Monday Tuesday Wednesday Thursday Friday] (Asia/Singapore)",
      "description": "Boost internal department priorities during business hours",
      "last_applied": "0001-01-01T00:00:00Z"
    },
    {
      "name": "External Priority (Nighttime)",
      "type": "weight_adjustment",
      "active": true,
      "window": "18:00-08:00 all days (Asia/Singapore)",
      "description": "Flatten priorities to favor external customers at night",
      "last_applied": "2025-11-24T01:04:32.893085058+08:00"
    }
  ]
}
```

**Field Descriptions:**
- `enabled` - Whether rule engine is enabled
- `count` - Total number of configured rules
- `rules[].active` - Whether this rule is currently active (time window matches)
- `rules[].window` - Human-readable time window description
- `rules[].last_applied` - Timestamp when rule was last applied

---

### Manually Trigger Rule Evaluation

**Endpoint:** `POST /admin/time-rules/apply`

Manually triggers immediate evaluation and application of all rules. Useful for testing or forcing an update outside the normal check interval.

**Request:**
```bash
curl "http://localhost:8081/admin/time-rules/apply" -X POST \
  -H "Authorization: Bearer <admin-key>"
```

**Response:**
```json
{
  "message": "Rules applied successfully",
  "active_count": 2,
  "active_rules": [
    {
      "name": "External Priority (Nighttime)",
      "type": "weight_adjustment",
      "active": true,
      "window": "18:00-08:00 all days (Asia/Singapore)",
      "description": "Flatten priorities to favor external customers at night",
      "last_applied": "2025-11-24T01:04:32.893085058+08:00"
    },
    {
      "name": "Nighttime Quotas",
      "type": "quota_adjustment",
      "active": true,
      "window": "18:00-08:00 all days (Asia/Singapore)",
      "description": "Release capacity to external customers",
      "last_applied": "2025-11-24T01:04:32.893085058+08:00"
    }
  ]
}
```

**Field Descriptions:**
- `active_count` - Number of rules that are currently active
- `active_rules` - Array of currently active rules

---

### Error Responses (Time Rules)

**501 Not Implemented - Rule engine not enabled:**
```json
{
  "error": "Not Implemented",
  "message": "Time-based rules are not enabled"
}
```

This occurs when:
- `time_rules_enabled=false` in configuration
- Rule configuration file failed to load
- Rule engine initialization failed

**Example Configuration Error Handling:**
```bash
# If config file doesn't exist or is invalid, gateway starts normally
# but rule engine is disabled. Check logs for:
# [WARN] Failed to load time rules config: ...
# [INFO] Time-based rule engine disabled due to config error
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
