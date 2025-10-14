# Development Notes

## 2025-10-12
- Planned work: make Exchange optional during gateway startup, ensure root admin local login exists, update docs.
- Current status: groundwork before implementation.

## Progress
- Exchange integration now optional; gateway and gatewayd fall back to local-only mode when the marketplace is disabled or unreachable.
- HTTP profile response includes marketplace connectivity flag; frontend shows a banner when Exchange is offline.
- Added `marketplace_enabled` config option with env override.
- Implemented local root admin identity store (SQLite + Postgres schemas), ensured CLI/gatewayd create the admin account, and bypassed Exchange failures gracefully.
- UI login now recognises root admin sessions and gateway profile response exposes marketplace connectivity.

- Non-root gateway users never sync to Tokligence Marketplace; only the configured root admin may link to marketplace features.
- OSS user store stays single-layer (users + api_keys). Enterprise migrations in `tokligence-gateway-enterprise` extend the same tables with org/team state so editions remain compatible.
- CLI gains `gateway admin users import` for CSV bulk provisioning; HTTP admin API mirrors the workflow so the UI can batch onboard operator/consumer accounts.
- Ledger entries now capture the API key responsible for each call, enabling token usage traces per credential (required for future quota work).
