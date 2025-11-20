# Tokligence Gateway Configuration Guide

This guide summarizes how Tokligence Gateway reads configuration and how to choose the right settings for common scenarios. It is meant to complement, not replace:

- `docs/QUICK_START.md` — build, run, and first request
- `docs/USER_GUIDE.md` — routing, usage, and API examples
- `docs/codex-to-anthropic.md` and `docs/claude_code-to-openai.md` — end‑to‑end flows for Codex CLI and Claude Code

Use this document as a **reference handbook** when you are unsure which `TOKLIGENCE_*` env var or INI key to set.

---

## 1. Configuration Layers & How They Merge

The gateway merges configuration from four sources (later wins):

1. **Defaults in code**
2. **Base INI**: `config/setting.ini`
3. **Environment overlay**: `config/{dev,test,live}/gateway.ini`
4. **Environment variables**: `TOKLIGENCE_*` (highest priority)

The active overlay environment is chosen by the `environment=` value in `config/setting.ini` (default: `dev`).

> Tip: Start from `config/setting.ini` and `config/dev/gateway.ini`, then override only what you need using env vars in a local `.env` file.

---

## 2. Minimal “First Request” Setup

For a new install, you usually only need to configure:

1. **Environment & identity**
2. **Auth & ledger paths**
3. **Upstream API keys**
4. **Work mode & routing**

### 2.1 Environment & Identity

| Purpose | INI key | Env var | Default |
| --- | --- | --- | --- |
| Active overlay (`dev` / `test` / `live`) | `environment` (`config/setting.ini`) | — | `dev` |
| Exchange base URL | `base_url` | — | `DefaultExchangeBaseURL(env)` (dev/test/live/localhost) |
| Gateway display name | `display_name` | `TOKLIGENCE_DISPLAY_NAME` | `Tokligence Gateway Dev` |
| Owner / account email | `email` | `TOKLIGENCE_EMAIL` | — (must be set for marketplace flows) |

The CLI will scaffold config for you:

```bash
./bin/gateway init --env dev --email you@example.com --display-name "Dev Gateway"
```

### 2.2 Auth & Ledger / Identity Storage

| Purpose | INI key | Env var | Default / Notes |
| --- | --- | --- | --- |
| API auth secret | `auth_secret` | `TOKLIGENCE_AUTH_SECRET` | `"tokligence-dev-secret"` in code; **override in production** |
| Disable auth (local only) | `auth_disabled` | `TOKLIGENCE_AUTH_DISABLED` | `true` in `config/setting.ini` for dev |
| Admin email used by daemon | `admin_email` | `TOKLIGENCE_ADMIN_EMAIL` | `admin@local` |
| Ledger DB path | `ledger_path` | `TOKLIGENCE_LEDGER_PATH` | `~/.tokligence/ledger.db` or PostgreSQL DSN |
| Identity DB path | `identity_path` | `TOKLIGENCE_IDENTITY_PATH` | `~/.tokligence/identity.db` or PostgreSQL DSN |

**Database Backend Support:**

The gateway supports both **SQLite** (default) and **PostgreSQL** for ledger and identity storage. The backend is auto-detected based on the path format:

- **SQLite**: File path (e.g., `~/.tokligence/ledger.db`, `/var/lib/tokligence/ledger.db`)
- **PostgreSQL**: DSN starting with `postgresql://` (e.g., `postgresql://user:pass@localhost:5432/tokligence_ledger`)

**PostgreSQL Benefits:**
- Higher concurrency support (1000+ QPS)
- Better performance under load (9.6x faster than comparable solutions)
- Horizontal scalability with connection pooling
- Production-grade durability and replication

**Example PostgreSQL Configuration:**

```bash
# Ledger storage (usage tracking)
export TOKLIGENCE_LEDGER_PATH="postgresql://tokligence:password@localhost:5432/tokligence_ledger"

# Identity storage (users, API keys)
export TOKLIGENCE_IDENTITY_PATH="postgresql://tokligence:password@localhost:5432/tokligence_identity"
```

Recommended:

- **Local dev**: keep `auth_disabled=true`; tests and examples use `Authorization: Bearer test` as a convenient placeholder header.
- **Production / shared env**:
  - Set `TOKLIGENCE_AUTH_DISABLED=false`
  - Use a long random `TOKLIGENCE_AUTH_SECRET`
  - **Use PostgreSQL** for ledger storage for better performance and scalability
  - Put ledger and identity DBs on persistent storage (e.g. volume mount for SQLite, or PostgreSQL server)

### 2.3 Logging & Diagnostics

| Purpose | INI key | Env var | Default / Notes |
| --- | --- | --- | --- |
| Log level | `log_level` | `TOKLIGENCE_LOG_LEVEL` | `debug` in dev, `info` recommended for prod |
| Shared log file | `log_file` | `TOKLIGENCE_LOG_FILE` | empty = stdout only |
| CLI log file | `log_file_cli` | `TOKLIGENCE_LOG_FILE_CLI` | `logs/gateway-cli.log` in dev |
| Daemon log file | `log_file_daemon` | `TOKLIGENCE_LOG_FILE_DAEMON` | `logs/gatewayd.log` (or env‑specific override) |

> Tip: Set `TOKLIGENCE_LOG_LEVEL=debug` while debugging routing or translation, then go back to `info` once stable.

---

## 3. Work Modes: auto / passthrough / translation

The **global work mode** determines whether the gateway is allowed to perform protocol translation or only act as a straight proxy.

- INI: `work_mode=auto|passthrough|translation`
- Env: `TOKLIGENCE_WORK_MODE=auto|passthrough|translation`

### 3.1 Mode Summary

| Mode | Behavior | Typical use |
| --- | --- | --- |
| `auto` (default) | Smart routing; decide between passthrough and translation based on endpoint + model | Mixed workloads (Codex, Claude Code, and raw SDKs together) |
| `passthrough` | Only native provider calls; **rejects** anything that would require translation | Harden production so only provider‑native APIs are allowed |
| `translation` | Only cross‑protocol translation; **rejects** direct passthrough | Testing, debugging translators, or forcing Claude↔GPT conversion |

### 3.2 How Auto Mode Decides

Auto mode is **model‑first, endpoint‑second**:

1. Resolve provider using `model_provider_routes` (see §4)
2. Look at the endpoint:
   - If endpoint already matches provider’s native protocol → **passthrough**
   - If endpoint does not match → **translation** between OpenAI and Anthropic

Examples (assuming default `model_provider_routes=gpt*=>openai,claude*=>anthropic`):

- `/v1/responses` + `gpt-4o` → provider = OpenAI, endpoint = OpenAI Responses → **passthrough to OpenAI**
- `/v1/responses` + `claude-3-5-sonnet` → provider = Anthropic, endpoint = OpenAI Responses → **translation to Anthropic Messages**
- `/anthropic/v1/messages` + `claude-3-5-sonnet` → provider = Anthropic, endpoint = Anthropic → **passthrough**
- `/anthropic/v1/messages` + `gpt-4o` → provider = OpenAI, endpoint = Anthropic → **translation to OpenAI**

See `tests/integration/work_modes/` for executable examples.

### 3.3 When to Use Each Mode

- **Local dev / most users**: `auto`
  - Works out of the box with Codex CLI and Claude Code.
  - Lets you mix `gpt*` and `claude*` models without thinking about endpoints.

- **Provider‑native only**: `passthrough`
  - Use when you want to ensure:
    - `/v1/chat/completions` with `gpt*` only talks to OpenAI
    - `/anthropic/v1/messages` with `claude*` only talks to Anthropic
  - Any cross‑protocol combination (e.g. `/v1/messages` + `gpt*`) fails with a clear `"work_mode=passthrough does not support translation"` error.

- **Translation‑only sandbox**: `translation`
  - Useful for:
    - Testing OpenAI ↔ Anthropic translation without touching native provider APIs.
    - Forcing Codex or Claude Code to always go through a translation path.
  - Any native‑native pair (endpoint and model both belong to the same provider) is rejected.

---

## 4. Routing: Providers & Model Patterns

Routing happens in two layers:

1. **Model‑first provider routing** (`model_provider_routes`)
2. **Legacy adapter mapping** (`routes` and `fallback_adapter`)

### 4.1 Model‑First Provider Routing

**INI key:** `model_provider_routes`  
**Env var:** `TOKLIGENCE_MODEL_PROVIDER_ROUTES`

Format: comma or newline‑separated `pattern=>provider` entries, applied in order.

```ini
model_provider_routes = gpt*=>openai,claude*=>anthropic
```

Defaults in code, if nothing is configured:

```text
gpt*    => openai
claude* => anthropic
```

You can extend this for more vendors:

```ini
model_provider_routes = gpt*=>openai,claude*=>anthropic,o1*=>openai,qwen*=>ali
```

Each request:

1. Looks up provider by matching the requested `model` against patterns.
2. If nothing matches, falls back to `fallback_adapter` (default: `loopback`).

### 4.2 Legacy Routes & Fallback Adapter

**INI key:** `routes`  
**Env var:** `TOKLIGENCE_ROUTES`

Format examples:

```bash
TOKLIGENCE_ROUTES="gpt-*=>openai, claude*=>anthropic, loopback=>loopback"
TOKLIGENCE_ROUTES="gpt-* = openai
claude* = anthropic
loopback = loopback"
```

These routes are mainly used for:

- Backward compatibility with older configs
- Explicitly mapping special model IDs to an adapter name

If both `model_provider_routes` and `routes` are present:

- `model_provider_routes` expresses **provider intent** (e.g. `gpt*=>openai`, `claude*=>anthropic`) and is used by work‑mode decisions.
- `routes` is applied **last** when registering patterns in the router and can override earlier mappings for advanced adapter setups (including non‑standard adapter names).

**Fallback adapter**:

- INI: `fallback_adapter=loopback`
- Env: `TOKLIGENCE_FALLBACK_ADAPTER`

When no route matches, the gateway uses the fallback adapter (the built‑in `loopback` model is ideal for local testing).

---

## 5. Ports & Endpoints (Single vs Multi‑Port)

By default, the daemon runs a single listener that exposes all endpoints on **`:8081`**.

**Key options:**

| Purpose | INI key | Env var | Default |
| --- | --- | --- | --- |
| Enable facade listener | `enable_facade` | `TOKLIGENCE_ENABLE_FACADE` | `true` |
| Enable multi‑port mode | `multiport_mode` | `TOKLIGENCE_MULTIPORT_MODE` | `false` |
| Facade port (all endpoints) | `facade_port` | `TOKLIGENCE_FACADE_PORT` | `8081` |
| Admin port | `admin_port` | `TOKLIGENCE_ADMIN_PORT` | `8079` |
| OpenAI port | `openai_port` | `TOKLIGENCE_OPENAI_PORT` | `8082` |
| Anthropic port | `anthropic_port` | `TOKLIGENCE_ANTHROPIC_PORT` | `8083` |
| Gemini port | `gemini_port` | `TOKLIGENCE_GEMINI_PORT` | `8084` |

Endpoint selection per port:

| Purpose | INI key | Env var | Default behavior |
| --- | --- | --- | --- |
| Facade | `facade_endpoints` | `TOKLIGENCE_FACADE_ENDPOINTS` | openai_core, openai_responses, anthropic, admin, health |
| OpenAI‑only | `openai_endpoints` | `TOKLIGENCE_OPENAI_ENDPOINTS` | openai_core, health |
| Anthropic‑only | `anthropic_endpoints` | `TOKLIGENCE_ANTHROPIC_ENDPOINTS` | anthropic, health |
| Gemini‑only | `gemini_endpoints` | `TOKLIGENCE_GEMINI_ENDPOINTS` | gemini_native, health |
| Admin‑only | `admin_endpoints` | `TOKLIGENCE_ADMIN_ENDPOINTS` | admin, health |

Underlying endpoint keys:

- `openai_core`: `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`, etc.
- `openai_responses`: `/v1/responses`
- `anthropic`: `/anthropic/v1/messages`, `/v1/messages`, and `count_tokens` variants
- `gemini_native`: `/v1beta/models/*` (native Gemini API and OpenAI-compatible endpoints)
- `admin`: `/api/v1/admin/...`
- `health`: `/health`

### 5.1 Recommended Patterns

- **Simple local dev**: keep `multiport_mode=false`, use only `:8081`.
- **Separation of concerns**:
  - Use `multiport_mode=true`
  - Point internal services to `:8082` (OpenAI) or `:8083` (Anthropic)
  - Expose only `:8081` externally behind your reverse proxy.

See `README.md` (“Multi‑port architecture”) for more background and diagrams.

---

## 6. Upstream Providers: OpenAI, Anthropic & Gemini

Set upstream credentials via env vars (strongly recommended) or INI.

| Purpose | INI key | Env var | Notes |
| --- | --- | --- | --- |
| OpenAI API key | `openai_api_key` | `TOKLIGENCE_OPENAI_API_KEY` | Required for OpenAI routes |
| OpenAI base URL | `openai_base_url` | `TOKLIGENCE_OPENAI_BASE_URL` | Default: `https://api.openai.com/v1` |
| OpenAI org | `openai_org` | `TOKLIGENCE_OPENAI_ORG` | Optional header |
| Anthropic API key | `anthropic_api_key` | `TOKLIGENCE_ANTHROPIC_API_KEY` | Required for Anthropic routes |
| Anthropic base URL | `anthropic_base_url` | `TOKLIGENCE_ANTHROPIC_BASE_URL` | Default: `https://api.anthropic.com` |
| Anthropic version | `anthropic_version` | `TOKLIGENCE_ANTHROPIC_VERSION` | Default: `2023-06-01` |
| Gemini API key | `gemini_api_key` | `TOKLIGENCE_GEMINI_API_KEY` | Required for Gemini routes |
| Gemini base URL | `gemini_base_url` | `TOKLIGENCE_GEMINI_BASE_URL` | Default: `https://generativelanguage.googleapis.com` |

> Security: never commit real keys into INI files. Use environment variables or a secrets manager; INI files should keep placeholders.

Common patterns:

- **OpenAI‑only gateway**:
  - Configure `TOKLIGENCE_OPENAI_API_KEY`
  - Ensure `model_provider_routes` maps your models (e.g. `gpt*`, `o1*`) to `openai`.

- **Anthropic‑only gateway**:
  - Configure `TOKLIGENCE_ANTHROPIC_API_KEY`
  - Ensure `model_provider_routes=claude*=>anthropic`

- **Gemini‑only gateway**:
  - Configure `TOKLIGENCE_GEMINI_API_KEY`
  - Enable multi-port mode with `TOKLIGENCE_MULTIPORT_MODE=true`
  - Set `TOKLIGENCE_GEMINI_PORT=8084` (or your preferred port)
  - Get your API key from [Google AI Studio](https://ai.google.dev/)

- **Hybrid (multi-provider)**:
  - Configure all needed keys
  - Example routes: `model_provider_routes=gpt*=>openai,claude*=>anthropic,gemini*=>gemini`

---

## 7. Model Aliases & Metadata

### 7.1 Model Aliases (Rewrite Incoming Model IDs)

Model aliases let you map incoming model names to provider‑specific ones. They are applied **after** routing has picked a provider.

- INI: `model_aliases=...`
- Env: `TOKLIGENCE_MODEL_ALIASES`

Format (comma or newline‑separated, `incoming=>target` or `incoming=target`):

```bash
TOKLIGENCE_MODEL_ALIASES="claude-3-5-sonnet-20241022=>gpt-4o, claude*=>gpt-4o"
```

Additional sources:

- INI: `model_aliases_file`, `model_aliases_dir`
- Env: `TOKLIGENCE_MODEL_ALIASES_FILE`, `TOKLIGENCE_MODEL_ALIASES_DIR`

Where each file contains one rule per line:

```text
claude-3-5-sonnet-20241022 = gpt-4o
claude* = gpt-4o
```

Best practices:

- Combine with `model_provider_routes` and `routes`:
  - Route `claude*` to `openai`
  - Alias `claude*` → `gpt-4o`
- Use prefixes/suffixes rather than enumerating every version.

### 7.2 Model Metadata

Model metadata drives internal caps (e.g. `max_tokens` limits).

| Purpose | INI key | Env var | Default |
| --- | --- | --- | --- |
| Local metadata file | `model_metadata_file` | `TOKLIGENCE_MODEL_METADATA_FILE` | `data/model_metadata.json` |
| Remote metadata URL | `model_metadata_url` | `TOKLIGENCE_MODEL_METADATA_URL` | Official GitHub raw URL |
| Refresh interval | `model_metadata_refresh` | `TOKLIGENCE_MODEL_METADATA_REFRESH` | `24h` if unset |

You rarely need to change these unless you run fully offline and want to ship your own metadata file.

---

## 8. Anthropic Integration & Tool Bridge

This section is relevant when you use **Anthropic endpoints** directly or **Codex/Claude Code → OpenAI** bridges. For full details, see:

- `docs/codex-to-anthropic.md`
- `docs/claude_code-to-openai.md`

### 8.1 Native Anthropic Endpoint Toggles

| Purpose | INI key | Env var | Default |
| --- | --- | --- | --- |
| Enable Anthropic native endpoints | `anthropic_native_enabled` | `TOKLIGENCE_ANTHROPIC_NATIVE_ENABLED` | `true` |
| Force SSE for Anthropic | `anthropic_force_sse` | `TOKLIGENCE_ANTHROPIC_FORCE_SSE` | `true` |
| Token limit enforcement | `anthropic_token_check_enabled` | `TOKLIGENCE_ANTHROPIC_TOKEN_CHECK_ENABLED` | `false` |
| Anthropic max tokens cap | `anthropic_max_tokens` | `TOKLIGENCE_ANTHROPIC_MAX_TOKENS` | `8192` default |

Beta feature toggles:

| Feature | INI key | Env var |
| --- | --- | --- |
| Web search | `anthropic_web_search` | `TOKLIGENCE_ANTHROPIC_WEB_SEARCH` |
| Computer use | `anthropic_computer_use` | `TOKLIGENCE_ANTHROPIC_COMPUTER_USE` |
| MCP tools | `anthropic_mcp` | `TOKLIGENCE_ANTHROPIC_MCP` |
| Prompt caching | `anthropic_prompt_caching` | `TOKLIGENCE_ANTHROPIC_PROMPT_CACHING` |
| JSON mode | `anthropic_json_mode` | `TOKLIGENCE_ANTHROPIC_JSON_MODE` |
| Reasoning | `anthropic_reasoning` | `TOKLIGENCE_ANTHROPIC_REASONING` |
| Custom beta header | `anthropic_beta_header` | `TOKLIGENCE_ANTHROPIC_BETA_HEADER` |

### 8.2 OpenAI Tool Bridge & Sidecar Model Map

When Anthropic requests are translated to OpenAI (e.g. Claude Code → GPT‑4o), the gateway uses a model map:

| Purpose | INI key | Env var | Notes |
| --- | --- | --- | --- |
| Sidecar model map | `sidecar_model_map` | `TOKLIGENCE_SIDECAR_MODEL_MAP` | `claude-x=gpt-y` rules |
| Sidecar model map file | `sidecar_model_map_file` | `TOKLIGENCE_SIDECAR_MODEL_MAP_FILE` | Loaded and concatenated with string map |
| OpenAI completion max tokens | `openai_completion_max_tokens` | `TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS` | Default `16384` |
| Enable streaming for tool bridge | `openai_tool_bridge_stream` | `TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM` | Default `false` (batch) |

> Recommendation: use `sidecar_model_map` / `sidecar_model_map_file` to explicitly map Claude models to GPT models in Claude Code setups.

### 8.3 Chat → Anthropic Translation

For OpenAI Chat endpoint translation to Anthropic:

- INI: `chat_to_anthropic=true`
- Env: `TOKLIGENCE_CHAT_TO_ANTHROPIC=on|true|1`

Effect:

- `/v1/chat/completions` with a `claude*` model:
  - Routed to Anthropic based on `model_provider_routes`
  - Translated from OpenAI Chat format to Anthropic `/v1/messages`
  - Streams Anthropic SSE back as OpenAI `chat.completion.chunk` events.

This toggle is especially useful for Codex CLI when you want to **treat Claude models as first‑class citizens** on the OpenAI Chat endpoint.

---

## 9. Hooks & Lifecycle Scripts

Hooks allow you to run an external script on significant lifecycle events (for example, to sync usage to a separate system).

Key settings (also summarized in `docs/hook_guide.md` and `docs/dev_guide.md`):

| Purpose | INI key | Env var |
| --- | --- | --- |
| Enable hooks | `hooks_enabled` | `TOKLIGENCE_HOOKS_ENABLED` |
| Script path | `hooks_script_path` | `TOKLIGENCE_HOOK_SCRIPT` |
| Script args (CSV) | `hooks_script_args` | `TOKLIGENCE_HOOK_SCRIPT_ARGS` |
| Extra env (k=v, comma‑sep) | `hooks_script_env` | `TOKLIGENCE_HOOK_SCRIPT_ENV` |
| Timeout (duration) | `hooks_timeout` | `TOKLIGENCE_HOOK_TIMEOUT` |

Example INI snippet:

```ini
hooks_enabled=true
hooks_script_path=/usr/local/bin/gateway-hook
hooks_timeout=30s
hooks_script_args=--env=dev,--mode=ledger
hooks_script_env=RUNTIME_ENV=dev,OWNER=tokligence
```

The CLI and daemon validate hook config on startup; misconfiguration will surface as an error.

---

## 10. Advanced Operational Toggles

These options are rarely needed but useful in advanced deployments.

### 10.1 Database Connection Pool Settings

For production deployments, especially when using PostgreSQL, you should configure database connection pool limits to prevent resource exhaustion.

| Purpose | INI key | Env var | Default | Notes |
| --- | --- | --- | --- | --- |
| Max open connections | `db_max_open_conns` | `TOKLIGENCE_DB_MAX_OPEN_CONNS` | `1000` | Maximum number of open connections to the database |
| Max idle connections | `db_max_idle_conns` | `TOKLIGENCE_DB_MAX_IDLE_CONNS` | `100` | Maximum number of idle connections in the pool |
| Connection max lifetime | `db_conn_max_lifetime` | `TOKLIGENCE_DB_CONN_MAX_LIFETIME` | `60` | Maximum lifetime of a connection in minutes |
| Connection max idle time | `db_conn_max_idle_time` | `TOKLIGENCE_DB_CONN_MAX_IDLE_TIME` | `10` | Maximum idle time of a connection in minutes |

**Connection Pool Best Practices:**

- **SQLite**: Use conservative limits (100 open, 10 idle) to avoid file descriptor leaks
- **PostgreSQL**: Can handle higher concurrency (1000 open, 100 idle configurable)
- **High-load scenarios**: Increase `db_max_open_conns` based on your QPS requirements
- **Connection lifetime**: Set `db_conn_max_lifetime` to force connection rotation and prevent stale connections

**Example configurations:**

```bash
# Low traffic (< 100 QPS)
export TOKLIGENCE_DB_MAX_OPEN_CONNS=100
export TOKLIGENCE_DB_MAX_IDLE_CONNS=10

# Medium traffic (1K QPS)
export TOKLIGENCE_DB_MAX_OPEN_CONNS=500
export TOKLIGENCE_DB_MAX_IDLE_CONNS=50

# High traffic (10K+ QPS)
export TOKLIGENCE_DB_MAX_OPEN_CONNS=1000
export TOKLIGENCE_DB_MAX_IDLE_CONNS=100
```

### 10.2 Async Ledger Batch Writer

The async ledger writer enables **non-blocking database writes** for usage tracking, eliminating ledger write blocking on the HTTP response path. This dramatically reduces P99 latency (from seconds to milliseconds) under high load.

**Configuration Parameters:**

| Purpose | INI key | Env var | Default | Notes |
| --- | --- | --- | --- | --- |
| Batch size | `ledger_async_batch_size` | `TOKLIGENCE_LEDGER_ASYNC_BATCH_SIZE` | `100` | Entries per batch before forcing a write |
| Flush interval | `ledger_async_flush_ms` | `TOKLIGENCE_LEDGER_ASYNC_FLUSH_MS` | `1000` | Flush interval in milliseconds |
| Buffer size | `ledger_async_buffer_size` | `TOKLIGENCE_LEDGER_ASYNC_BUFFER_SIZE` | `10000` | Channel buffer size (entries) |
| Number of workers | `ledger_async_num_workers` | `TOKLIGENCE_LEDGER_ASYNC_NUM_WORKERS` | `1` | Parallel worker goroutines |

**How it Works:**

```
HTTP Request → Record() → Buffered Channel → Worker Pool → Batch INSERT
     ↓
  Immediate Response (non-blocking)
```

**Tuning Guidelines by Traffic Level:**

| QPS Range | Batch Size | Flush (ms) | Buffer Size | Workers |
|-----------|------------|------------|-------------|---------|
| < 100     | 50         | 2000       | 5,000       | 1       |
| 100-1K    | 100        | 1000       | 10,000      | 2       |
| 1K-10K    | 500        | 500        | 50,000      | 5       |
| 10K-100K  | 2,000      | 200        | 200,000     | 10      |
| 100K-1M   | 10,000     | 100        | 1,000,000   | 50      |

**Buffer Sizing Formula:**
```
buffer_size ≥ peak_qps × burst_duration_seconds
```

For example, to handle 1M QPS with 1-second burst tolerance:
```
buffer_size ≥ 1,000,000 × 1 = 1,000,000 entries
```

**Example: High-Traffic Production (50K QPS)**

```bash
export TOKLIGENCE_LEDGER_ASYNC_BATCH_SIZE=5000
export TOKLIGENCE_LEDGER_ASYNC_FLUSH_MS=200
export TOKLIGENCE_LEDGER_ASYNC_BUFFER_SIZE=500000
export TOKLIGENCE_LEDGER_ASYNC_NUM_WORKERS=20
```

**Trade-offs:**

- **Memory usage**: Buffer size × entry size (~1-2KB) = total memory overhead
- **Data loss risk**: On crash, up to `buffer_size` entries may be lost (graceful shutdown flushes all)
- **Latency**: Entries written within `flush_ms` interval (not real-time)

**Monitoring:**

Watch for these log messages to tune parameters:

```
[async-ledger] worker-0 flushed 5000/5000 entries in 123ms (40650 entries/sec)
[async-ledger] WARNING: channel full, dropping entry
```

- If batch always full → increase `batch_size`
- If flush time > 100ms → optimize database or reduce `batch_size`
- If entries/sec < target QPS → increase `num_workers`
- If seeing "channel full" → increase `buffer_size` or `num_workers`

**Performance Impact:**

- Eliminates ledger write blocking on HTTP response path
- Reduces P99 latency from 5+ seconds to milliseconds
- Enables 100K-1M+ QPS with proper tuning
- Required for high-performance PostgreSQL deployments

For comprehensive tuning guide, see `docs/async-ledger-tuning.md`.

### 10.3 Bridge Session Management (Responses API)

| Purpose | INI key | Env var | Default |
| --- | --- | --- | --- |
| Enable dedup sessions | `bridge_session_enabled` | `TOKLIGENCE_BRIDGE_SESSION_ENABLED` | `true` |
| Session TTL | `bridge_session_ttl` | `TOKLIGENCE_BRIDGE_SESSION_TTL` | `5m` |
| Max session count | `bridge_session_max_count` | `TOKLIGENCE_BRIDGE_SESSION_MAX_COUNT` | `1000` |
These settings are currently a **no‑op** in `gatewayd` (the bridge runs in stateless mode); they are kept for configuration compatibility and can generally be ignored unless future releases repurpose them.

### 10.4 Responses Streaming Tuning

Controlled only by env vars:

| Purpose | Env var | Default |
| --- | --- | --- |
| Stream mode (aggregate vs immediate) | `TOKLIGENCE_RESPONSES_STREAM_MODE=aggregate|buffered|auto` | non‑aggregate by default |
| SSE ping interval (ms) | `TOKLIGENCE_RESPONSES_SSE_PING_MS` | `2000` ms if unset |

Use these only when you need to fine‑tune SSE heartbeat behavior with specific clients.

### 10.5 Marketplace & Telemetry

| Purpose | INI key | Env var | Default |
| --- | --- | --- | --- |
| Enable marketplace integration | `marketplace_enabled` | `TOKLIGENCE_MARKETPLACE_ENABLED` | `true` |
| Enable telemetry | `telemetry_enabled` | `TOKLIGENCE_TELEMETRY_ENABLED` | `true` |

You can disable marketplace/telemetry features by setting the corresponding env vars to `false`. The core gateway continues to operate offline.

### 10.6 Update Check

Env only:

- `TOKLIGENCE_UPDATE_CHECK_ENABLED=true|false`

If present, this env var controls an optional daily update check. Setting it to `false` is always safe; core gateway functionality does not depend on update checks and works fully offline.

### 10.7 Duplicate Tool Detection

| Purpose | INI key | Env var | Notes |
| --- | --- | --- | --- |
| Enable duplicate tool guard | `duplicate_tool_detection` | `TOKLIGENCE_DUPLICATE_TOOL_DETECTION` | Used by Responses API to prevent infinite loops |

This is primarily relevant for Codex CLI “full‑auto” runs; see `docs/codex-to-anthropic.md` for behavior details.

---

## 11. Configuration Recipes

This section collects practical “copy‑paste” starting points. Adjust emails, ports, and model names as needed.

### 11.1 Local Dev (Loopback‑Only, No External LLM)

Use this when you just want to verify auth, routing, and logging without calling any upstream provider.

```ini
; config/dev/gateway.ini
display_name = Local Loopback Dev
auth_disabled = true
log_level = debug

; Keep no provider keys set
; openai_api_key =
; anthropic_api_key =

; Route everything to loopback
fallback_adapter = loopback
routes = *=>loopback
work_mode = auto
```

Then:

```bash
./bin/gatewayd
```

### 11.2 OpenAI‑First Gateway (Codex CLI)

Use this when you mainly use Codex CLI with OpenAI models, and optionally allow Claude models via translation.

```bash
export TOKLIGENCE_OPENAI_API_KEY=sk-proj-...
export TOKLIGENCE_WORK_MODE=auto
export TOKLIGENCE_MODEL_PROVIDER_ROUTES="gpt*=>openai,o1*=>openai,claude*=>anthropic"
export TOKLIGENCE_ROUTES="gpt*=>openai,claude*=>anthropic,loopback=>loopback"
```

### 11.3 Claude Code → OpenAI (Anthropic Front, GPT Back)

This is the typical “Claude Code talks Anthropic, gateway talks OpenAI” setup. For a full walkthrough, see `docs/claude_code-to-openai.md`.

```bash
export TOKLIGENCE_OPENAI_API_KEY=sk-proj-...
export TOKLIGENCE_WORK_MODE=auto

# Treat claude* as OpenAI on the backend
export TOKLIGENCE_MODEL_PROVIDER_ROUTES="claude*=>openai,gpt*=>openai"
export TOKLIGENCE_MODEL_ALIASES="claude*=>gpt-4o"

# Optional: make Chat endpoint translate claude* to Anthropic format
export TOKLIGENCE_CHAT_TO_ANTHROPIC=on
```

Point Claude Code to:

- Base URL: `http://localhost:8081/anthropic`
- API key: any non‑empty token (gateway enforces auth separately).

### 11.4 Google Gemini Integration

Use this to enable Google Gemini API through the gateway with pass-through proxy.

```bash
# Gemini API key (required)
export TOKLIGENCE_GEMINI_API_KEY=AIzaSy...

# Enable multi-port mode for dedicated Gemini port
export TOKLIGENCE_MULTIPORT_MODE=true
export TOKLIGENCE_GEMINI_PORT=8084
export TOKLIGENCE_GEMINI_ENDPOINTS=gemini_native,health

# Optional: custom base URL (defaults to Google's official URL)
export TOKLIGENCE_GEMINI_BASE_URL=https://generativelanguage.googleapis.com
```

**Available endpoints** on port 8084:

- **Native Gemini API**:
  - `POST /v1beta/models/{model}:generateContent` - Standard generation
  - `POST /v1beta/models/{model}:streamGenerateContent?alt=sse` - Streaming generation
  - `POST /v1beta/models/{model}:countTokens` - Token counting
  - `GET /v1beta/models` - List models
  - `GET /v1beta/models/{model}` - Get model info

- **OpenAI-compatible** (use OpenAI SDK with Gemini models):
  - `POST /v1beta/openai/chat/completions` - Chat completions (streaming & non-streaming)

**Example usage**:

```bash
# Native Gemini API
curl -X POST 'http://localhost:8084/v1beta/models/gemini-2.0-flash-exp:generateContent' \
  -H 'Content-Type: application/json' \
  -d '{"contents":[{"parts":[{"text":"Hello"}]}]}'

# OpenAI-compatible endpoint with Gemini model
curl -X POST 'http://localhost:8084/v1beta/openai/chat/completions' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gemini-2.0-flash-exp","messages":[{"role":"user","content":"Hello"}]}'
```

**Get API key**: Visit [Google AI Studio](https://ai.google.dev/) to obtain your Gemini API key.

For detailed Gemini integration guide, see `docs/gemini-integration.md`.

### 11.5 High-Performance PostgreSQL Production (10K+ QPS)

Use this configuration for production deployments requiring high throughput and scalability with PostgreSQL backend.

```bash
# Database backend (PostgreSQL for high performance)
export TOKLIGENCE_LEDGER_PATH="postgresql://tokligence:password@postgres-host:5432/tokligence_ledger"
export TOKLIGENCE_IDENTITY_PATH="postgresql://tokligence:password@postgres-host:5432/tokligence_identity"

# Connection pool settings for high concurrency
export TOKLIGENCE_DB_MAX_OPEN_CONNS=1000
export TOKLIGENCE_DB_MAX_IDLE_CONNS=100
export TOKLIGENCE_DB_CONN_MAX_LIFETIME=60    # minutes
export TOKLIGENCE_DB_CONN_MAX_IDLE_TIME=10   # minutes

# Async ledger for non-blocking writes (tuned for 50K QPS)
export TOKLIGENCE_LEDGER_ASYNC_BATCH_SIZE=5000
export TOKLIGENCE_LEDGER_ASYNC_FLUSH_MS=200
export TOKLIGENCE_LEDGER_ASYNC_BUFFER_SIZE=500000
export TOKLIGENCE_LEDGER_ASYNC_NUM_WORKERS=20

# Security
export TOKLIGENCE_AUTH_DISABLED=false
export TOKLIGENCE_AUTH_SECRET=$(openssl rand -hex 32)

# Logging
export TOKLIGENCE_LOG_LEVEL=info

# Provider routing
export TOKLIGENCE_WORK_MODE=auto
export TOKLIGENCE_MODEL_PROVIDER_ROUTES="gpt*=>openai,claude*=>anthropic"
```

**Performance characteristics:**
- Supports 10K-100K QPS with proper PostgreSQL tuning
- Sub-10ms P50 latency maintained under load
- Non-blocking ledger writes eliminate response path blocking
- 9.6x faster than comparable solutions (LiteLLM)

**PostgreSQL requirements:**
- High-performance instance (16+ cores, 64GB+ RAM recommended for 100K+ QPS)
- SSD/NVMe storage for fast writes
- Connection pooler (PgBouncer/pgpool) recommended for very high loads
- Consider table partitioning for `usage_entries` by time

**Monitoring:**
```bash
# Watch async ledger performance
tail -f /var/log/tokligence/gatewayd.log | grep "async-ledger"

# PostgreSQL connection monitoring
psql -h postgres-host -U tokligence -d tokligence_ledger \
  -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';"
```

For detailed async ledger tuning, see `docs/async-ledger-tuning.md`.
For benchmark results, see `scripts/benchmark/performance-comparison-sqlite-vs-postgresql.md`.

### 11.6 Strict Provider‑Native Production

Use this when you want **no translation at all** in production—only native provider calls are allowed.

```bash
export TOKLIGENCE_WORK_MODE=passthrough
export TOKLIGENCE_AUTH_DISABLED=false
export TOKLIGENCE_LOG_LEVEL=info
export TOKLIGENCE_MODEL_PROVIDER_ROUTES="gpt*=>openai,claude*=>anthropic"
```

In this mode:

- `/v1/chat/completions` + `gpt*` → OpenAI (allowed)
- `/anthropic/v1/messages` + `claude*` → Anthropic (allowed)
- `/v1/responses` + `claude*` or `/v1/messages` + `gpt*` → rejected with a clear work‑mode error.

---

## 12. Where to Go Next

- For **step‑by‑step setup**, see `docs/QUICK_START.md`.
- For **detailed usage and routing examples**, see `docs/USER_GUIDE.md`.
- For **Codex CLI and Claude Code scenarios**, see:
  - `docs/codex-to-anthropic.md`
  - `docs/claude_code-to-openai.md`

If you are unsure which setting controls a behavior, search for the corresponding `TOKLIGENCE_*` name in:

- `internal/config/config.go`
- `internal/httpserver/server.go`
- `config/*.ini`

and then add the override either in your environment or the appropriate INI file. 
