# User System Overview

Tokligence Gateway (OSS) deliberately ships a **single-layer** identity model so self-hosters can run the gateway without the Tokligence Marketplace. Enterprise builds layer additional organisation/team concepts on top of the same primitives.

## Identity Roles

| Role | Scope | Description |
| --- | --- | --- |
| Root admin (`root_admin`) | Local gateway only | Default administrator (`admin@local`) seeded in the local identity store. No email verification required. |
| Gateway operators (`gateway_admin`) | Local gateway | Created by the root admin to manage configuration, usage and hooks. Never synced to Tokligence Marketplace. |
| Gateway consumers (`gateway_user`) | Local gateway | API users managed by the root admin. They only exist in the OSS identity store. |
| Exchange identities | Tokligence Marketplace | Optional account used for marketplace publishing/billing. Only the root admin mailbox may be linked here; regular gateway users never become Exchange users via the OSS product. |

## Login Behaviour

1. **Root admin**
   - Email defaults to `admin@local` (configurable with `admin_email`).
   - CLI/HTTP login skips verification and issues a session immediately.
   - `/api/v1/profile` surfaces `marketplace.connected` so the UI can display Exchange availability.

2. **Gateway admins / users**
   - Stored locally in SQLite or Postgres through `internal/userstore`.
   - Provisioned via `gateway admin users ...` CLI (including `gateway admin users import` for CSV batches) or the Admin UI.
   - Receive API keys for programmatic access; keys authenticate against `/v1/chat/completions`.

3. **Tokligence Marketplace**
   - OSS gateway checks connectivity during startup. If unreachable, the process logs a warning and keeps running in local-only mode.
   - Only the configured root admin may be linked to Exchange capabilities. Downstream users must register separately on toklicence.ai if they wish to participate in the marketplace.

## Database Schema (OSS)

Identity data lives in `~/.tokligence/identity.db` by default. Operators can point at Postgres by changing `identity_path`.

### SQLite

```
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL,
  display_name TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key_hash TEXT NOT NULL UNIQUE,
  key_prefix TEXT NOT NULL,
  scopes TEXT,
  expires_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Postgres

```
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  role TEXT NOT NULL,
  display_name TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE api_keys (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key_hash TEXT NOT NULL UNIQUE,
  key_prefix TEXT NOT NULL,
  scopes TEXT,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

The columns line up with LiteLLM’s user/key model so we can add higher-order constructs later without rewriting existing data.

## Configuration Hooks

- `admin_email` / `TOKLIGENCE_ADMIN_EMAIL` – root admin mailbox (default `admin@local`).
- `identity_path` / `TOKLIGENCE_IDENTITY_PATH` – SQLite path or Postgres DSN.
- `marketplace_enabled` / `TOKLIGENCE_MARKETPLACE_ENABLED` – toggles marketplace checks.

Startup steps (`gateway` and `gatewayd`):

1. Open the configured identity store.
2. Call `EnsureRootAdmin` (creating the root user if needed).
3. Attempt to contact Tokligence Marketplace when enabled; fall back gracefully otherwise.
4. Expose marketplace health via API/UI while keeping the LLM gateway fully operational.

## Enterprise Extension Path

Enterprise adds the multi-layer model (organisations, teams, usage quotas) by applying migrations in **tokligence-gateway-enterprise**. The OSS tables above act as the foundation:

- Additional tables (`organizations`, `teams`, `api_key_memberships`) reference the OSS `users` and `api_keys` primary keys.
- Enterprise services extend the `userstore.Store` interface without breaking OSS binaries.
- Hooks already emit user lifecycle events so downstream RAG systems can keep in sync regardless of edition.

Document the expectation in `notes.md`: only the root admin mailbox can be dual-homed with Tokligence Marketplace; downstream users stay local unless they opt into the marketplace separately.

This split keeps the community build simple while leaving a low-friction upgrade path for enterprise deployments.
