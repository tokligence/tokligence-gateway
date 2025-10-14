# Gateway Frontend & API Plan

## 1. Requirements Recap
- Web UI 与后端 Go 服务分离部署，通过 HTTPS REST API 交互。
- UI 需支持桌面浏览器与手机端 H5 自适应，P0 聚焦响应式布局，无需原生 App。
- 功能：账户信息展示、供应商/服务目录浏览、发布服务、Usage/账本概览，后续迭代逐步扩展。
- 与 Tokligence Token Marketplace 交互仍由后端负责；前端只调用 Gateway 自身 API。

## 2. 技术栈评估
| 方案 | 优点 | 潜在挑战 | 适配度 |
| --- | --- | --- | --- |
| **React + TypeScript + Vite** | 全球社区领先、生态成熟；LiteLLM UI 即基于 React，迁移经验可复用；配合 React Query/TanStack Router 构建数据层；与 Tailwind/Chakra 等响应式框架结合广泛。 | 需要自行处理状态管理（推荐 TanStack Query + Zustand），SSR 如需可后续引入 Next.js。 | ⭐⭐⭐⭐⭐ |
| Vue 3 + TypeScript + Vite | 在国内社区流行，上手曲线平滑，官方生态（Pinia、Vue Router）完整；Composition API 容易编写响应式逻辑。 | 对照 LiteLLM（React）时无法直接复用组件/经验；海外贡献者比例相对较低。 | ⭐⭐⭐⭐ |
| SvelteKit | 打包体积小、学习曲线低。 | 社区规模小，第三方组件数量有限，不利于外部贡献。 | ⭐⭐ |

> 结论：选用 **React + TypeScript + Vite**。对应生态最广，且能直接参考 LiteLLM 的前端模式，降低设计迁移成本，同时满足移动端响应式需求。

### 辅助库
- UI：Tailwind CSS + Headless UI（可快速响应式布局），后续如需组件库可评估 Radix UI/Chakra UI。
- 状态管理：TanStack Query 管理 API 数据，Zustand 存储轻量 UI 状态（抽屉、模态框等）。
- 表单：React Hook Form。
- 国际化：i18next（后续 Community/Enterprise 需求）。
- 测试：Vitest + React Testing Library。

## 3. 项目结构
```
/fe
  /src
    /app (路由)
    /components
    /features
    /services (API SDK)
    /styles
  vite.config.ts
  package.json
```
- 与 Go 后端同仓库，但作为独立 package (`fe`)，使用 pnpm/npm/yarn。
- `pnpm build` 输出静态资源（默认 dist），可由 Nginx/S3/CloudFront 或 Tokligence Gateway 内置静态服务托管。

## 4. 构建与部署
- 开发：`pnpm dev` (Vite dev server, 默认端口 5173)。
- 构建：`pnpm build` 生成静态文件。
- 部署：
  - P0：静态文件直接由 Nginx/`gateway serve-ui` 提供。
  - 生产：推荐反向代理 (nginx/caddy) 或 CDN。
- CI：GitHub Action/Drone 运行 `pnpm lint && pnpm test && pnpm build`，产出 artifact。

## 5. 后端 API 约定 (P0 范围)
| 方法 | 路径 | 描述 | 请求体 (简) | 响应体 (简) |
| --- | --- | --- | --- | --- |
| `GET` | `/api/v1/profile` | 当前登录账户信息、已启用角色 | - | `{ "user": {...}, "provider": {...} }` |
| `GET` | `/api/v1/providers` | 供应商目录 | - | `{ "providers": [ {"id": 1, ...} ] }` |
| `GET` | `/api/v1/services` | 服务列表（可按 provider/self 过滤） | `?scope=all|mine` | `{ "services": [...] }` |
| `POST` | `/api/v1/services` | 发布/更新本地服务 | `{ "name": "local", "model_family": "claude", "price_per_1k": 0.5 }` | `{ "service": {...} }` |
| `GET` | `/api/v1/usage/summary` | Usage 汇总（代替 CLI 的 `UsageSnapshot`） | - | `{ "summary": {"consumed_tokens": 100, ...} }` |
| `POST` | `/api/v1/token-accounting/report` | （预留）手动上报 usage；P0 供 CLI/前端测试 | `{ "service_id": 1, "direction": "consume", ... }` | `{ "ok": true }` |
| `POST` | `/api/v1/auth/login` | 邮箱验证码 / Token Marketplace 登录代理 | `{ "email": "..." }` | `{ "challenge_id": "..." }` |
| `POST` | `/api/v1/auth/verify` | 校验验证码，返回 JWT | `{ "challenge_id": "...", "code": "123456" }` | `{ "token": "jwt" }` |

> 认证：P0 可使用 Email + Magic Link（与 Token Marketplace 对齐），后端验证后签发短期 JWT（存储在 Cookie `HttpOnly`）。

## 6. 前端页面概要 (P0)
1. **Login / Onboarding**
   - 邮箱输入 → 触发 `/api/v1/auth/login` → 验证码输入 → `/api/v1/auth/verify`。
2. **Dashboard**
   - 显示 Usage Summary、免费额度、最新供给/消费记录。
3. **Providers & Services**
   - Tab 显示所有供应商列表、我的服务列表。
   - 发布服务弹窗（调用 `POST /api/v1/services`）。
4. **Settings**
   - 展示账户信息、配置 hints（邮箱、Display Name、Provider 状态）。

移动端通过响应式布局（Tailwind `md:` 断点）适配。

## 7. API 文档交付方式
- 维护 `docs/api/gateway_openapi.yaml`（后续补充）。
- 前端 `services/api.ts` 使用 `fetch`/`axios` 封装，配合 TypeScript 类型。
- 每个端点在仓库 `docs/api/README.md` 中描述用途、字段、示例。

## 8. 后续迭代建议
- 接入 SSE (`/v1/chat/completions` streaming) 的测试工作台页面。
- Usage 账本分页列表 + 导出 CSV。
- 多租户/Team 管理（对应 C1 里程碑）。
- PWA 支持（添加离线缓存、桌面快捷方式）。
