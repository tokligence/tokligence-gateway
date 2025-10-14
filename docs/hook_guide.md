# Gateway Hook Integration Guide

Tokligence Gateway can broadcast lifecycle events so external systems—such as RAG stacks built on Weaviate, Dgraph, Milvus, or pgvector—mirror user state without bespoke polling. This document explains how to enable the hook dispatcher, wire it to scripts, and prototype an end-to-end sync.

## 1. Overview

Hooks fire when notable identity transitions occur:

| Event | Description |
| --- | --- |
| `gateway.user.provisioned` | Emitted after `gateway` ensures a user exists (CLI onboarding, automatic login). |
| `gateway.user.updated` | Emitted when the user’s profile or roles change. |
| `gateway.user.deleted` | Reserved for future account removal flows. |
| `gateway.api_key.issued` | Reserved for API key creation. |
| `gateway.api_key.revoked` | Reserved for API key revocation. |

Each event travels through the dispatcher defined in `internal/hooks/spec.go`. Handlers receive an `Event` struct with:

```go
ID         string         // unique event identifier (UUID)
Type       hooks.EventType
OccurredAt time.Time      // UTC timestamp
TenantID   string         // optional future field when multi-tenant lands
UserID     string         // Tokligence user ID
ActorID    string         // Who triggered the event (user/service)
Metadata   map[string]any // Extra JSON-friendly fields
```

The default JSON payload looks like:

```json
{
  "id": "7d66dcb5-8a62-4b62-91bb-32f4d1dc3f53",
  "type": "gateway.user.provisioned",
  "occurred_at": "2025-02-18T10:03:23Z",
  "tenant_id": "",
  "user_id": "42",
  "actor_id": "42",
  "metadata": {
    "email": "agent@example.com",
    "roles": ["consumer"]
  }
}
```

## 2. Enable hooks via configuration

Hooks are configured alongside the usual `config/setting.ini` and per-environment overlays. Relevant keys:

```ini
# config/setting.ini
hooks_enabled=true
hooks_script_path=/usr/local/bin/sync-rag
hooks_script_args=--vector-store,weaviate
hooks_script_env=WEAVIATE_URL=https://weaviate.internal,API_KEY=super-secret
hooks_timeout=30s
```

Environment variables override any file values:

- `TOKLIGENCE_HOOKS_ENABLED`
- `TOKLIGENCE_HOOK_SCRIPT`
- `TOKLIGENCE_HOOK_SCRIPT_ARGS`
- `TOKLIGENCE_HOOK_SCRIPT_ENV`
- `TOKLIGENCE_HOOK_TIMEOUT`

When the CLI boots, `cmd/gateway/main.go` builds a `hooks.Dispatcher`, registers the script handler, and attaches it to the core gateway:

```text
$ TOKLIGENCE_HOOK_SCRIPT_ARGS="--sync" gateway
[gateway/main][dev][INFO] hooks dispatcher enabled script=/usr/local/bin/sync-rag
```

If `hooks_enabled=false` or the script path is blank, the dispatcher stays disabled.

## 3. Example provisioning script

Create a lightweight script that bridges gateway events into Weaviate (via its REST API). Save it as `/usr/local/bin/sync-rag` and make it executable.

```bash
#!/usr/bin/env bash
set -euo pipefail

payload=$(cat)
event_type=$(echo "$payload" | jq -r '.type')
user_id=$(echo "$payload" | jq -r '.user_id')
email=$(echo "$payload" | jq -r '.metadata.email')
roles=$(echo "$payload" | jq -c '.metadata.roles')

case "$event_type" in
  gateway.user.provisioned|gateway.user.updated)
    curl -sS -X POST "$WEAVIATE_URL/v1/batch" \
      -H "Authorization: Bearer $API_KEY" \
      -H 'Content-Type: application/json' \
      -d "{\"objects\":[{\"class\":\"GatewayUser\",\"id\":$user_id,\"properties\":{\"email\":\"$email\",\"roles\":$roles}}]}"
    ;;
  gateway.user.deleted)
    curl -sS -X DELETE "$WEAVIATE_URL/v1/objects/GatewayUser/$user_id" \
      -H "Authorization: Bearer $API_KEY"
    ;;
  *)
    echo "ignoring event $event_type" >&2
    ;;
 esac
```

The script receives the JSON payload on STDIN. Environment variables `WEAVIATE_URL` and `API_KEY` come from `hooks_script_env` or the process environment.

## 4. Local dry run

1. Configure hooks in `config/setting.ini` as shown above.
2. Export any sensitive environment values:
   ```bash
   export TOKLIGENCE_HOOKS_ENABLED=true
   export TOKLIGENCE_HOOK_SCRIPT=/usr/local/bin/sync-rag
   export TOKLIGENCE_HOOK_SCRIPT_ENV="WEAVIATE_URL=https://weaviate.internal,API_KEY=$(pass show weaviate/api-key)"
   ```
3. Start the CLI: `gateway`.
4. When `gateway` provisions your account, the script runs. Monitor logs or add `set -x` to the script for debugging.

## 5. Failure handling

- Scripts execute sequentially per event; return a non-zero exit code to surface errors.
- The dispatcher aggregates errors and logs them, but does not retry automatically. Add retry logic inside scripts if needed.
- Configure `hooks_timeout` to stop runaway scripts; exceeding the deadline cancels the process and records an error.

## 6. Future extensions

Upcoming work will:

- Emit API key lifecycle events (`gateway.api_key.*`).
- Support multiple handlers (script + gRPC/webhook) through configuration.
- Offer a `gateway sync rag` sub-command that bootstraps Weaviate/Dgraph schemas and seeds historical users.

Until then, the script bridge provides a flexible path for early adopters to wire their RAG access control into the gateway’s identity system.
