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

