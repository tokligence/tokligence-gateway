# LiteLLM Feature Survey

LiteLLM 是一个以 OpenAI 格式为统一接口的多供应商 LLM 代理与 SDK。本节梳理出我们在 Tokligence Gateway P0 阶段需要重点参考的能力、组件与数据模型。

## 1. OpenAI 兼容抽象
- **请求适配**：`litellm.completion/acompletion` 将调用映射到不同供应商（OpenAI、Anthropic、Azure、Bedrock、Groq、VertexAI 等）的 chat、completion、embedding、image、audio 接口。
- **统一响应**：无论上游格式如何，输出统一落在 `choices[].message` 结构，并补齐 `usage.prompt_tokens/completion_tokens/total_tokens`。
- **Streaming 支持**：所有支持流式的供应商在 LiteLLM 中通过 `stream=True` 暴露为 OpenAI SSE chunk (`chat.completion.chunk`)。

## 2. Proxy / Gateway 功能
- **多上游路由**：Router 通过权重、优先级、健康检查实现跨部署的重试与故障转移（`router_architecture.md`、`routing.md`）。
- **项目/团队隔离**：Proxy 引入 Organization → Team → User → Key 的层级，配合别名模型、模型访问控制，实现多租户管理（`projects.md`、`team_based_routing.md`）。
- **API Key 管理**：支持创建主账户密钥、团队密钥、虚拟/临时密钥，控制访问模型集合与预算（`user_keys.md`、`virtual_keys.md`）。
- **预算与配额**：可配置总预算、模型级预算、RPM/TPM 限额，以及预算重置周期和软/硬阈值告警（`budget_manager.md`、`rate_limit_tiers.md`）。
- **审计与日志**：请求日志、成功/失败事件、流式片段都可存储并暴露给 UI；支持动态采样、Webhook 回调、Prometheus 指标（`logging.md`、`metrics.md`、`streaming_logging.md`）。
- **配置管理**：Proxy 允许热更新模型别名、供应商凭证、路由策略，支持 GitHub 同步与命令行管理（`config_management.md`、`sync_models_github.md`、`management_cli.md`）。
- **Guardrails**：内置输入输出审核、规则引擎、第三方审核（`guardrails/` 系列）。

## 3. 前端体验
- **Admin Console (`ui/`)**：基于 Next.js + React，覆盖：
  - 工作区概览（请求量、失败率、费用）。
  - 团队 / 用户管理与邀请流程。
  - 模型目录与路由配置编辑。
  - 密钥、预算、速率限制的创建/修改。
  - 实时日志与对话回放，支持筛选、导出。
  - SSO/OAuth2 登录、角色切换（`proxy/ui.md`、`proxy/ui_logs.md`、`proxy/admin_ui_sso.md`）。
- **Self-serve Onboarding**：注册 → 创建组织 → 发放密钥 → 快速测试接口的引导流程（`proxy/user_onboarding.md`、`proxy/self_serve.md`）。
- **可插拔主题与白标**：UI 支持自定义 Logo、域名、登录字段（`proxy/custom_root_ui.md`）。

## 4. 数据库存储 (Prisma Schema)
LiteLLM 的 `schema.prisma` 基于 Postgres，核心表结构：
- `LiteLLM_OrganizationTable` / `LiteLLM_TeamTable` / `LiteLLM_UserTable`：管理多租户层级、角色权限、预算指标、模型访问列表。
- `LiteLLM_BudgetTable`：存储预算、速率限制、模型分级限额，并关联组织、用户、标签、团队成员关系。
- `LiteLLM_VerificationToken`：存储 API Key / Token，附带别名、预算、限流配置与状态。
- `LiteLLM_ProxyModelTable` / `LiteLLM_CredentialsTable`：维护上游模型别名与凭证 JSON。
- `LiteLLM_RequestLog`, `LiteLLM_UsageTable` *(在后续 migration 中)*：保存请求日志、token 使用、成本估算，用于计费与审计。
- `LiteLLM_ObjectPermissionTable`：集中保存对象级权限（MCP Server、Vector Store、团队等）。
- 其他：MCP Server 配置、邀请链接、团队成员权限、标签路由、guardrail 规则等表。

> **取舍参考**：Tokligence Personal 版可复用其中的核心概念（Organization/User/Key/Budget、模型凭证、请求日志），后续 Community/Enterprise 再扩展团队、多租户、MCP、Guardrail 等表。

## 5. 部署与运维
- 提供 Docker 镜像、Docker Compose、Helm、Render/Railway 等模板；Proxy 启动后即提供 REST + SSE + Web UI（`proxy/deploy.md`、`docker_quick_start.md`）。
- 支持多节点部署、主从数据库、缓存（Redis）以及 Prometheus/Grafana 监控整合（`proxy/prometheus.md`、`proxy/perf.md`）。
- 提供 CLI (`litellm --config`) 管理与自动同步配置。

## 6. Tokligence 对标要点
- **网关目标**：Tokligence Gateway Personal 版需至少实现 OpenAI 兼容层、模型/供应商映射、基础预算/usage 记账（可先嵌入 SQLite），并预留升级路径以接入组织/团队结构。
- **前端参考**：P0 前端应复制 LiteLLM 的核心模块（密钥管理、模型目录、账本/Usage、简单日志），保证后端接口设计与 UI 功能对齐。
- **扩展考虑**：保留 guardrails、路由、MCP 等高级能力的接口与数据模型占位，以便在 Community/Enterprise 阶段平滑升级。

此文档将作为规划 Tokligence Gateway P0/P1 功能、数据库 schema 与 UI 骨架的对标依据。
