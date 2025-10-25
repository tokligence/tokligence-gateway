# Tokligence Gateway Feature Matrix

Roadmap milestones (P0 → E2) translated into a phase-by-phase checklist. Use this to track progress and prioritise upcoming work. Status codes: ✅ Done, 🚧 In progress, 📝 Planned/TODO.

## Phase C0 – Community CLI Alpha (Weeks 1-2)

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Core Gateway | OpenAI-compatible `/v1/chat/completions` loopback | ✅ | Basic loopback adapter with token accounting. |
| Core Gateway | Token accounting (SQLite ledger) | ✅ | Ledger API + usage snapshot. |
| CLI | `gateway init` config scaffolding | ✅ | Generates setting.ini and env overrides. |
| CLI | Ensure account, publish service | ✅ | Auto-provisions user/provider. |
| Config | INI + env loader | ✅ | `LoadGatewayConfig` merges defaults/env. |
| Hooks | User lifecycle dispatcher (script bridge) | ✅ | CLI provisions emit JSON events for external scripts. |
| Frontend | Vite React SPA skeleton (login, dashboard) | ✅ | Desktop/H5 build targets with routing + layout. |
| Frontend | Provider/service list, usage summary | ✅ | Fetch hooks + pages render provider/service tables and usage widgets. |
| Distribution | `make dist-go` cross-compile matrix | ✅ | Linux/macOS/Windows builds with configs. |
| Distribution | `make dist-frontend` (web + H5) | ✅ | Bundles land under `dist/frontend`. |
| Documentation | README product matrix + WIP notice | ✅ | Highlights Go core, optional frontend. |
| Documentation | Hook usage guide | ✅ | `docs/hook.md` (local only). |
| Documentation | Dev onboarding guide | ✅ | `docs/dev_guide.md` (local only). |

## Phase C1 – Community GA (Weeks 3-4)

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Gateway | SSE streaming for `/v1/chat/completions` | 📝 | Not yet implemented. |
| Gateway | `/v1/models`, `/v1/embeddings` endpoints | 📝 | Stubbed? needs verification. |
| Gateway | Free-credit exhaustion warnings | 📝 | TODO. |
| CLI | Auto update channel (Homebrew/Scoop) | 📝 | Needs packaging automation. |
| Web UI | Usage ledger, request log viewer | 🚧 | Partial coverage; needs completion. |
| Web UI | Config editor / route switching UI | 📝 | Not started. |
| Edge Plugin | Marketplace publish MVP | 📝 | TODO. |
| Hooks | Gateway-centric RAG hook (user/keys) | 🚧 | User hooks in place; API-key events TBD. |
| Distribution | Binary installers (tar/zip + checksums) | 🚧 | Base builds done; release automation pending (GoReleaser). |
| Docs | API mapping (OpenAI ↔ Anthropic) | ✅ | `docs/api_mapping.md` covers tool bridge and normalization. |

## Phase C1 – Community Beta (Weeks 5-8)

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Auth | API key service, org/team budgets | 📝 | Not started. |
| Frontend | Team management, quota dashboards | 📝 |
| Deployment | Docker image + Helm templates | 📝 |
| Storage | Postgres option + migration preview | 📝 |
| Integrations | Hook triggers for external vector DBs | 🚧 | Script handler scaffolding available; needs CLI/HTTP endpoints. |
| Observability | Prometheus metrics, Grafana dashboards | 📝 |
| Adapter SDK | Contract tests + contribution guide | 📝 |
| Exchange | Shared free-credit pool wiring | 📝 |

## Phase C2 – Community GA (Weeks 9-12)

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Observability | Full suite (Grafana, OpenTelemetry) | 📝 |
| Migration | `gateway migrate` tool GA | 📝 |
| Edge Plugin | Health checks, rate limits, revenue board | 📝 |
| Frontend | Guardrails, routing rules UI, request replay | 📝 |
| Config | Feature flags, rollout/rollback guides | 📝 |
| Security | Key rotation, config encryption, audit export | 📝 |

## Phase E1 – Enterprise Early Access (Weeks 13-18)

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Governance | Org/Project/User token hierarchy, audit logs | 📝 |
| Routing | Intelligent routing (latency/cost), circuit breakers | 📝 |
| Ledger | Dual-book (Postgres + ClickHouse) | 📝 |
| Compliance | Data residency, alerting | 📝 |
| Exchange | Real-time usage streaming, revenue share callbacks | 📝 |

## Phase E2 – Enterprise GA (Weeks 19-24)

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Scalability | HA deployments, connection pooling, perf tuning | 📝 |
| Security | SAML/SCIM, approval workflows | 📝 |
| Resilience | Chaos testing, DR runbooks | 📝 |
| Compliance | SOC2 prep, deletion API, incident response | 📝 |
| Marketplace | GMV dashboards, SLA notifications | 📝 |
| Frontend | Enterprise ops centre (multi-region analytics) | 📝 |

## Continuous Initiatives

| Area | Feature | Status | Notes |
| --- | --- | --- | --- |
| Token Accounting | Provider formula calibration | 📝 | Revisit each release. |
| Adapter Ecosystem | New providers + regression suite | 📝 |
| Security | CVE patch cadence, binary signing | 📝 |
| Community | Tutorials, FAQ, office hours | 📝 |
| Exchange Alignment | Quota policy sync, version matrix | 📝 |

_Last updated: `2025-02-17`._
