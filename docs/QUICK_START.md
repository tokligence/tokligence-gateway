# Quick Start

This guide helps you get the gateway running quickly and covers the most common configuration points for development.

## Build

```bash
go version # 1.24+
make build
# Binaries are placed in ./bin
```

## Run the Daemon

```bash
./bin/gatewayd
# Default address: http://localhost:8081
```

## Initial Setup (Admin)

1. Login as root admin (admin@local) using the CLI or the web UI if enabled.
2. Create a user and generate an API key:
   ```bash
   ./bin/gateway admin users create --email user@example.com --role gateway_user
   ./bin/gateway admin api-keys create --user <user_id>
   ```

## Test the API

```bash
curl -H "Authorization: Bearer <api_key>" \
     -H "Content-Type: application/json" \
     -d '{"model":"loopback","messages":[{"role":"user","content":"Hello"}]}' \
     http://localhost:8081/v1/chat/completions
```

The built‑in `loopback` model echoes input without calling external LLMs, ideal for verifying authentication and connectivity.

## Understanding API Keys

Tokligence Gateway uses **two types of API keys** that serve different purposes:

### 1. Gateway API Keys (User Authentication)

These are keys that **you create** for users to access your gateway:

```bash
# Create a gateway API key for a user
./bin/gateway admin api-keys create --user <user_id>
# Returns: tok_xxxxxxxxxxxx
```

- **Purpose**: Authenticate users/applications to your Tokligence Gateway
- **Who creates them**: You (the gateway administrator)
- **Format**: `tok_xxxxxxxxxxxx`
- **Used in**: `Authorization: Bearer tok_xxxxxxxxxxxx` header when calling your gateway

### 2. LLM Provider API Keys (Backend Connection)

These are keys from **external LLM providers** that the gateway uses to forward requests:

```bash
# Configure LLM provider keys (environment variables)
export TOKLIGENCE_OPENAI_API_KEY=sk-...           # From OpenAI
export TOKLIGENCE_ANTHROPIC_API_KEY=sk-ant-...    # From Anthropic
export TOKLIGENCE_GOOGLE_API_KEY=AIza...          # From Google
```

- **Purpose**: Allow the gateway to connect to upstream LLM providers
- **Who creates them**: The LLM provider (OpenAI, Anthropic, Google, etc.)
- **Format**: Provider-specific (e.g., `sk-...` for OpenAI)
- **Used by**: The gateway internally when routing requests to providers

### How They Work Together

```
┌─────────────┐    Gateway API Key     ┌─────────────────────┐    LLM Provider Key    ┌──────────────┐
│   Client    │ ───────────────────▶  │  Tokligence Gateway │ ────────────────────▶ │  OpenAI/     │
│ (your app)  │   tok_xxxxxxxxxxxx    │                     │   sk-proj-...         │  Anthropic   │
└─────────────┘                        └─────────────────────┘                        └──────────────┘
```

1. Your application authenticates to the gateway using a **Gateway API Key**
2. The gateway validates the request and routes it to the appropriate provider
3. The gateway uses the **LLM Provider API Key** to call OpenAI/Anthropic/etc.
4. The response flows back through the gateway to your application

> **Note**: For local LLMs (Ollama, vLLM, LM Studio), no LLM Provider API Key is needed—just configure the endpoint URL.

## Configuration

Configuration loads in three layers:

1. Global defaults: `config/setting.ini`
2. Environment overlays: `config/{dev,test,live}/gateway.ini`
3. Environment variables: `TOKLIGENCE_*`

### Common Options

| Option | Env | Default | Description |
| --- | --- | --- | --- |
| `http_address` | — | `:8081` | HTTP bind address |
| `identity_path` | `TOKLIGENCE_IDENTITY_PATH` | `~/.tokligence/identity.db` | User DB (SQLite path or Postgres DSN) |
| `ledger_path` | `TOKLIGENCE_LEDGER_PATH` | `~/.tokligence/ledger.db` | Usage ledger DB |
| `log_file_cli`, `log_file_daemon` | `TOKLIGENCE_LOG_FILE_*` | — | Separate log files for CLI/daemon |

### Logging

- Daily UTC rotation and size‑based rollover; logs mirrored to stdout by default.
- Disable file output by setting `log_file` to `-`.

### Anthropic Translation

- The gateway supports Anthropic `/v1/messages` with SSE out of the box. It translates to OpenAI upstream when configured and streams correct Anthropic events back to clients like Claude Code.

## Developing

```bash
make test
make dist   # cross-compile binaries
```

Frontend build (optional):

```bash
cd fe
npm install
npm run build:web
```

