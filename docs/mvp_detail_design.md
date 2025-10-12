# Tokligence Gateway MVP Detailed Design

## Objective
Deliver a minimal Tokligence Gateway client that can:
1. Register as a consumer and optionally provider against the Tokligence Token Exchange MVP.
2. Discover providers and services for downstream agent selection while guarding against vendor lock-in.
3. Publish local services (linking into the Exchange) when acting as a provider.
4. Record and review token consumption/supply to maintain an independent ledger.

## Architecture
- **`internal/client`** — HTTP client binding to Tokligence Token Exchange REST endpoints. Handles request/response marshalling, error translation, logging (`[gateway/http]`). Exposes typed operations (`RegisterUser`, `ListProviders`, `PublishService`, `ReportUsage`, `GetUsageSummary`).
- **`internal/core`** — Gateway orchestration layer maintaining session state (current user/provider), provider selection helpers, publish/report flows, and configuration flags (e.g., provider mode toggle).
- **`cmd/gateway`** — CLI harness that loads INI + environment configuration, ensures accounts, optionally publishes a demo service, and prints usage summaries.
- **`pkg/gateway`** — Reusable surface consumed by other binaries (including Enterprise wrapper) to avoid duplicating orchestrator logic.
- **`fe/`** — React + TypeScript + Vite + Tailwind 单页应用，提供登录/验证码流程、Dashboard、Provider/Service 目录、Usage 摘要、API Key 管理及请求日志入口，兼顾桌面与移动端。

## Key Flows
1. **Account bootstrap**
   - `Gateway.EnsureAccount` registers the email, enabling consumer/provider roles based on configuration.
   - The returned `User` and optional `ProviderProfile` are cached for later commands.
2. **Provider discovery & selection**
   - `Gateway.ListProviders` proxies the Exchange call.
  - `Gateway.ChooseProvider` accepts predicates (e.g., match name/model family) so CLI or higher-level agents can implement strategies.
3. **Publishing local services**
   - Requires provider mode; Gateway fills `provider_id`, uses defaults from config (`publish.name`, `publish.model_family`), and surfaces the created `ServiceOffering`.
   - `Gateway.ListMyServices` fetches scoped services to confirm publish status.
4. **Token accounting interface**
   - `RecordConsumption` / `RecordSupply` wrap Exchange usage API with proper direction flags.
   - `UsageSnapshot` aggregates totals for CLI display, aligning with Exchange ledger hashes.
   - Web Dashboard 调用 `/api/v1/usage/summary`、预留 `/api/v1/usage/logs` 展示成本趋势、请求明细，对齐 LiteLLM Usage/Logging 模块。
5. **Configuration loading**
   - `config/setting.ini` + environment overrides produce a `Config` struct shared by CLI and Enterprise wrapper. Fields include base URL, email, provider enable flag, publish defaults, and log level.
6. **Web interaction**
   - `/api/v1/auth/login` → 触发邮箱验证码；`/api/v1/auth/verify` → 校验并换取 Token Exchange 会话后签发 JWT（HttpOnly Cookie）。
   - `/api/v1/profile` 返回用户与 Provider 元数据、免费额度、角色标签，前端据此决定导航与权限。
   - `/api/v1/providers`、`/api/v1/services` 支持过滤、排序；`POST /api/v1/services` 发布/更新本地服务。
   - `/api/v1/keys`（预留）提供 API Key CRUD、标签、预算限制，延续 LiteLLM Proxy 自助管理体验。

## Compliance Awareness
- Gateway receives compliance failure responses (`400` with rule identifiers) and surfaces them in logs/CLI output.
- No local compliance rules are enforced in the OSS gateway; all enforcement resides in Tokligence Token Exchange.

## Testing Strategy
- `internal/client/exchange_test.go` uses `httptest.Server` to validate HTTP wiring without requiring a live Exchange.
- `internal/core/gateway_test.go` employs a fake exchange to drive the full lifecycle: registration, provider selection, publishing, and dual-direction usage reporting, ensuring a single account can play both roles.
- Tests are executed via `GOFLAGS=-mod=mod go test ./...` or Docker (`docker compose run --rm test`).
- `fe/` 采用 Vitest + React Testing Library 做组件/Hook 单测，引入 Playwright 覆盖登录、Dashboard、服务发布等 E2E 场景。

## Docker Compose Support
- `dev`: persistent Go development container (`docker compose up dev`).
- `cli`: demonstration run of the CLI (`docker compose --profile runtime up cli`).
- `test`: executes `go test ./...` in an isolated container.
- `web`: Vite 开发服务器 (`docker compose --profile web up web`)；`build-web`: 执行 `pnpm build` 产出静态资源供集成测试。

## Configurability
- Primary configuration resides in `config/setting.ini` with descriptive comments for each field (base URL, email, provider toggle, publish defaults, log verbosity).
- Environment-specific overrides live under `config/{dev,test,live}/gateway.ini`.
- Environment variables may override INI settings (e.g., `TOKLIGENCE_BASE_URL`, `TOKLIGENCE_EMAIL`), enabling CI/CD integration.

## Next Steps
- Replace ad-hoc CLI output with JSON/structured logging for easier integration.
- Add SSE streaming and adapter hot-reload to expand the client experience.
- Harden retry/circuit breaker logic in `internal/client` before high-throughput workloads.
- Extend tests to cover error paths, retry/backoff behaviour, and configuration edge cases.
