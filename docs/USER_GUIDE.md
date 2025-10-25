# Tokligence Gateway User Guide

This guide shows how to install, configure, and run the Tokligence Gateway, and how to route Claude Code (Anthropic) calls through the gateway to other OpenAI‑compatible providers. It also covers accounting (ledger), authentication, and streaming.

## 1. Prerequisites

- Go 1.24+ (for building from source)
- Node 18+ (only if you want to run the optional frontend)
- SQLite (embedded, no system install required)

## 2. Build & Run

```bash
# Build the binaries
make build

# Binaries are produced in ./bin/
./bin/gatewayd   # HTTP daemon
./bin/gateway    # CLI tool
```

Default HTTP address: `:8081`.

## 3. Configuration Model

Configuration loads from three layers (later wins):
- Global defaults: `config/setting.ini`
- Environment overlay: `config/{dev,test,live}/gateway.ini`
- Environment variables: `TOKLIGENCE_*`

You can scaffold initial config with:

```bash
./bin/gateway init --env dev --email you@example.com --display-name "Dev Gateway"
```

Key settings you may want to set (ENV notation used below):

- `TOKLIGENCE_OPENAI_API_KEY` – API key for OpenAI upstream
- `TOKLIGENCE_OPENAI_BASE_URL` – Custom OpenAI base URL (optional)
- `TOKLIGENCE_OPENAI_ORG` – OpenAI organization header (optional)
- `TOKLIGENCE_ANTHROPIC_API_KEY` – API key for Anthropic upstream
- `TOKLIGENCE_ANTHROPIC_BASE_URL` – Custom Anthropic base URL (optional)
- `TOKLIGENCE_ANTHROPIC_VERSION` – Anthropic API version (default `2023-06-01`)
- `TOKLIGENCE_ROUTES` – Model routing rules, e.g. `gpt-*=>openai, claude*=>anthropic, loopback=>loopback`
- `TOKLIGENCE_MARKETPLACE_ENABLED` – Enable/disable Tokligence Marketplace (default true)

You can set the same keys in INI files:

```ini
# config/dev/gateway.ini
openai_api_key=sk-...
anthropic_api_key=sk-ant-...
routes=gpt-*=>openai, claude*=>anthropic, loopback=>loopback
```

## 4. Endpoints

Gateway exposes two categories of endpoints:

- OpenAI‑compatible:
  - `POST /v1/chat/completions` (supports `stream=true` SSE)
  - `POST /v1/embeddings`
  - `GET  /v1/models`

- Anthropic‑native (proxy):
  - `POST /anthropic/v1/messages` (supports `stream=true` SSE)

All endpoints require gateway API keys for authorization (unless you plug in the session flow on `/api/v1/auth/*`).

## 5. Authentication & API Keys

Create a user and an API key with the CLI:

```bash
# Create a user
./bin/gateway admin users create --email user@example.com --role gateway_user --name "User"

# Create an API key for that user
./bin/gateway admin api-keys create --user <user_id>

# List keys
./bin/gateway admin api-keys list --user <user_id>
```

Use bearer auth:

```
Authorization: Bearer <token>
```

Or via `X-API-Key: <token>`.

## 6. Routing Models to Providers

Routing is pattern‑based using `TOKLIGENCE_ROUTES` (and INI `routes=`):

- Exact: `gpt-4=>openai`
- Prefix: `gpt-*=>openai`
- Suffix: `*-turbo=>openai`
- Contains: `*claude*=>anthropic`

The router selects the adapter per model name at runtime.

### Examples

- Conventional routing (OpenAI for GPT*, Anthropic for Claude*):

```
TOKLIGENCE_ROUTES="gpt-*=>openai, claude*=>anthropic, loopback=>loopback"
```

- Use Anthropic client (Claude Code) but route to OpenAI under the hood:

```
TOKLIGENCE_ROUTES="claude*=>openai, gpt-*=>openai, loopback=>loopback"
```

This lets Claude Code call the gateway using Anthropic protocol while the gateway actually talks to OpenAI upstreams.

## 7. Using the Gateway

### 7.1 OpenAI Chat Completions

```bash
curl -sS -N \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
        "model":"gpt-4",
        "messages":[{"role":"user","content":"Hello"}],
        "stream": true
      }' \
  http://localhost:8081/v1/chat/completions
```

### 7.2 Anthropic Native (Claude Code)

Point your Claude Code / Anthropic SDK to `http://localhost:8081/anthropic` and use `/v1/messages`:

```bash
curl -sS -N \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "claude-3-5-sonnet-20241022",
        "system": "You are a helpful assistant.",
        "messages": [
          {"role":"user","content":[{"type":"text","text":"Hello"}]}
        ],
        "stream": true
      }' \
  http://localhost:8081/anthropic/v1/messages
```

If your `routes` map `claude*=>openai`, the gateway will translate the Anthropic call to OpenAI, stream it back as Anthropic SSE events, and record usage in the ledger.

## 8. Accounting (Ledger)

The gateway keeps a per‑user ledger of token usage in SQLite (default path `~/.tokligence/ledger.db`).

- For OpenAI and Anthropic non‑streaming calls, usage is recorded using the upstream‑reported token counts.
- For streaming on the Anthropic native endpoint, the gateway records an approximate usage (chars ÷ 4) for the completion part and a minimal estimate for prompt tokens. This keeps the ledger consistent even when upstream doesn’t return final usage in streams.

Query usage via API:

```bash
curl -sS -H "Cookie: tokligence_session=<session_token>" \
  http://localhost:8081/api/v1/usage/summary | jq

curl -sS -H "Cookie: tokligence_session=<session_token>" \
  "http://localhost:8081/api/v1/usage/logs?limit=20" | jq
```

Or via CLI/DB tooling if needed.

## 9. Frontend (Optional)

```bash
cd fe && npm install && npm run dev
# Open http://localhost:5174
```

## 10. Security Notes

- Gateway API keys protect all model endpoints. Treat tokens as secrets.
- Provider credentials (OpenAI/Anthropic keys) live only on the server.
- Marketplace communication is optional and can be disabled via `TOKLIGENCE_MARKETPLACE_ENABLED=false`.

## 11. Troubleshooting

- No models listed: ensure routes and upstream keys are configured.
- Unauthorized 401: confirm gateway API key in `Authorization: Bearer ...`.
- Streaming stalls: verify upstream provider allows streaming and your route selects a streaming‑capable adapter.
- Ledger empty: confirm you used a gateway API key (ledger records per user) and the request completed; for streaming, approximate usage is recorded at stream end.

