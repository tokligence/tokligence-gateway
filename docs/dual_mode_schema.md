# Tokligence Gateway - DUAL Mode Database Schema

## Overview

Tokligence Gateway 支持 **DUAL 模式**，即同时作为 **Provider**（服务提供者）和 **Consumer**（服务消费者）运行。

**核心用例：转手生意（Reseller）**
```
用户配置外部 LLM Provider → Gateway 转售给下游用户
                          ↓
            同时跟踪 consumption 和 supply
```

---

## Architecture Principles

### 1. 统一的用户表
- `users` 表服务于两种角色
- 通过 `role` 和关联表区分用户类型

### 2. 双向 Ledger
- `usage_entries` 表记录所有 token 流动
- `direction` 字段区分：`consume`（消费）或 `supply`（提供）

### 3. 灵活的组织结构
- 支持三层层级：Organization → Project/Team → User
- Provider 视角：Organization → Project（服务）→ Admin
- Consumer 视角：Organization → Department/Team → Employee

### 4. Gateway 级别统计
- 聚合视图：整个 Gateway 的总体消费和供给
- 支持按时间、模型、provider 分组分析

---

## Core Tables

### 1. Identity & Auth

#### `users` (已存在，需扩展)
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL, -- 'root_admin', 'gateway_admin', 'gateway_user'
    display_name TEXT,
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'inactive'

    -- DUAL 模式扩展字段
    organization_id INTEGER REFERENCES organizations(id),
    team_id INTEGER REFERENCES teams(id),
    employee_id TEXT,  -- 内部员工编号（Consumer 模式）

    -- 配额管理
    quota_tokens BIGINT,  -- 个人配额（可选）
    quota_reset_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_org ON users(organization_id);
CREATE INDEX idx_users_team ON users(team_id);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
```

#### `api_keys` (已存在)
```sql
CREATE TABLE api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    scopes TEXT,  -- JSON array: ["read", "write", "admin"]

    -- 配额管理
    quota_tokens BIGINT,  -- API key 级别配额
    quota_reset_at TIMESTAMP,

    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_deleted_at ON api_keys(deleted_at);
```

---

### 2. Organizational Structure

#### `organizations`
```sql
CREATE TABLE organizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    description TEXT,

    -- 预算管理（Consumer 模式）
    budget_usd DECIMAL(12, 4),
    budget_period TEXT, -- 'monthly', 'quarterly', 'yearly'

    -- Marketplace 关联
    marketplace_provider_id BIGINT,  -- 对应 marketplace 的 provider ID

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_organizations_slug ON organizations(slug);
CREATE INDEX idx_organizations_deleted_at ON organizations(deleted_at);
```

#### `teams`
```sql
CREATE TABLE teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    parent_team_id INTEGER REFERENCES teams(id), -- 支持层级（可选）
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,

    -- 类型标识
    team_type TEXT NOT NULL DEFAULT 'generic', -- 'generic', 'department', 'project'

    -- 预算管理（Consumer 模式）
    budget_usd DECIMAL(12, 4),

    -- Provider 模式：如果这个 team 代表一个可售卖的服务
    marketplace_service_id BIGINT,  -- 对应 marketplace 的 service ID
    service_model TEXT,  -- 'gpt-4', 'claude-3-opus', etc.
    service_endpoint TEXT,  -- 上游 LLM provider 的 endpoint

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE(organization_id, slug)
);

CREATE INDEX idx_teams_org ON teams(organization_id);
CREATE INDEX idx_teams_parent ON teams(parent_team_id);
CREATE INDEX idx_teams_type ON teams(team_type);
CREATE INDEX idx_teams_marketplace_service ON teams(marketplace_service_id);
CREATE INDEX idx_teams_deleted_at ON teams(deleted_at);
```

#### `user_teams` (关联表)
```sql
CREATE TABLE user_teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member', -- 'owner', 'admin', 'member', 'viewer'

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(user_id, team_id)
);

CREATE INDEX idx_user_teams_user ON user_teams(user_id);
CREATE INDEX idx_user_teams_team ON user_teams(team_id);
```

---

### 3. Usage Ledger (已存在，需扩展)

#### `usage_entries`
```sql
CREATE TABLE usage_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,

    -- 用户关联
    user_id INTEGER NOT NULL REFERENCES users(id),
    api_key_id INTEGER REFERENCES api_keys(id),

    -- 组织关联（用于成本归因）
    organization_id INTEGER REFERENCES organizations(id),
    team_id INTEGER REFERENCES teams(id),

    -- Service/Provider 信息
    service_id INTEGER NOT NULL DEFAULT 0,  -- marketplace service ID (可选)
    provider_name TEXT,  -- 'openai', 'anthropic', 'azure', etc.
    model TEXT NOT NULL,  -- 'gpt-4', 'claude-3-opus', etc.
    endpoint TEXT,  -- 实际调用的 endpoint

    -- Token 使用量
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens INTEGER GENERATED ALWAYS AS (prompt_tokens + completion_tokens) VIRTUAL,

    -- 方向：consume（消费）或 supply（提供）
    direction TEXT NOT NULL CHECK(direction IN ('consume','supply')),

    -- 成本信息
    cost_usd DECIMAL(12, 6),
    pricing_model TEXT,  -- 'per_1k_tokens', 'per_request', etc.

    -- 请求元数据
    request_id TEXT,  -- 关联到具体的 API 请求
    request_duration_ms INTEGER,  -- 请求耗时（毫秒）
    status_code INTEGER,  -- HTTP 状态码
    error_message TEXT,  -- 如果有错误

    -- 审计字段
    memo TEXT,
    metadata TEXT,  -- JSON：存储额外信息（如 user agent, IP 等）

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_usage_entries_user_created ON usage_entries(user_id, created_at DESC);
CREATE INDEX idx_usage_entries_api_key_created ON usage_entries(api_key_id, created_at DESC);
CREATE INDEX idx_usage_entries_org_created ON usage_entries(organization_id, created_at DESC);
CREATE INDEX idx_usage_entries_team_created ON usage_entries(team_id, created_at DESC);
CREATE INDEX idx_usage_entries_direction ON usage_entries(direction);
CREATE INDEX idx_usage_entries_model ON usage_entries(model);
CREATE INDEX idx_usage_entries_provider ON usage_entries(provider_name);
CREATE INDEX idx_usage_entries_request ON usage_entries(request_id);
CREATE INDEX idx_usage_entries_deleted_at ON usage_entries(deleted_at);
```

---

### 4. Gateway-Level Aggregation Tables

#### `gateway_usage_summary`
**目的：Gateway 整体的消费和供给统计**

```sql
CREATE TABLE gateway_usage_summary (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- 时间维度
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    period_type TEXT NOT NULL, -- 'hour', 'day', 'week', 'month'

    -- 方向
    direction TEXT NOT NULL CHECK(direction IN ('consume','supply')),

    -- Provider/Model 维度
    provider_name TEXT,
    model TEXT,

    -- 聚合指标
    total_requests INTEGER NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    total_prompt_tokens BIGINT NOT NULL DEFAULT 0,
    total_completion_tokens BIGINT NOT NULL DEFAULT 0,
    total_cost_usd DECIMAL(12, 4),

    -- 性能指标
    avg_duration_ms INTEGER,
    p50_duration_ms INTEGER,
    p95_duration_ms INTEGER,
    p99_duration_ms INTEGER,

    -- 错误指标
    error_count INTEGER NOT NULL DEFAULT 0,
    error_rate DECIMAL(5, 4),  -- 0.0000 - 1.0000

    -- 唯一用户/API key 数量
    unique_users INTEGER,
    unique_api_keys INTEGER,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(period_start, period_type, direction, provider_name, model)
);

CREATE INDEX idx_gateway_summary_period ON gateway_usage_summary(period_start, period_type);
CREATE INDEX idx_gateway_summary_direction ON gateway_usage_summary(direction);
CREATE INDEX idx_gateway_summary_provider ON gateway_usage_summary(provider_name, model);
```

#### `provider_model_catalog`
**目的：跟踪 Gateway 连接的所有外部 providers 和 models**

```sql
CREATE TABLE provider_model_catalog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    provider_name TEXT NOT NULL,  -- 'openai', 'anthropic', etc.
    model TEXT NOT NULL,  -- 'gpt-4', 'claude-3-opus'

    -- 配置信息
    endpoint TEXT NOT NULL,
    api_key_prefix TEXT,  -- 前缀用于识别（不存储完整 key）

    -- 定价信息
    price_per_1k_prompt_tokens DECIMAL(10, 6),
    price_per_1k_completion_tokens DECIMAL(10, 6),
    currency TEXT DEFAULT 'USD',

    -- 状态
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    health_status TEXT DEFAULT 'unknown', -- 'healthy', 'degraded', 'down', 'unknown'
    last_health_check TIMESTAMP,

    -- 使用统计（快速查询）
    total_requests BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    last_used_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,

    UNIQUE(provider_name, model)
);

CREATE INDEX idx_catalog_provider ON provider_model_catalog(provider_name);
CREATE INDEX idx_catalog_enabled ON provider_model_catalog(is_enabled);
CREATE INDEX idx_catalog_health ON provider_model_catalog(health_status);
```

---

### 5. Performance & Monitoring Tables

#### `request_logs`
**目的：详细的请求日志（用于调试和性能分析）**

```sql
CREATE TABLE request_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,

    -- 请求标识
    request_id TEXT NOT NULL UNIQUE,
    trace_id TEXT,  -- 分布式追踪 ID

    -- 用户信息
    user_id INTEGER REFERENCES users(id),
    api_key_id INTEGER REFERENCES api_keys(id),

    -- 请求信息
    method TEXT NOT NULL,  -- 'POST', 'GET'
    path TEXT NOT NULL,  -- '/v1/chat/completions'

    -- Provider/Model
    provider_name TEXT,
    model TEXT,

    -- 时间指标
    request_started_at TIMESTAMP NOT NULL,
    request_completed_at TIMESTAMP,
    duration_ms INTEGER GENERATED ALWAYS AS (
        CAST((julianday(request_completed_at) - julianday(request_started_at)) * 86400000 AS INTEGER)
    ) VIRTUAL,

    -- 响应信息
    status_code INTEGER,
    error_message TEXT,

    -- Token 使用（从 usage_entries 冗余过来，便于快速查询）
    prompt_tokens INTEGER,
    completion_tokens INTEGER,

    -- 网络信息
    client_ip TEXT,
    user_agent TEXT,

    -- 审计
    metadata TEXT,  -- JSON：额外的请求上下文

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX idx_request_logs_user ON request_logs(user_id);
CREATE INDEX idx_request_logs_started_at ON request_logs(request_started_at DESC);
CREATE INDEX idx_request_logs_provider ON request_logs(provider_name, model);
CREATE INDEX idx_request_logs_status ON request_logs(status_code);
CREATE INDEX idx_request_logs_deleted_at ON request_logs(deleted_at);
```

#### `performance_metrics`
**目的：按时间窗口聚合的性能指标**

```sql
CREATE TABLE performance_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- 时间维度
    window_start TIMESTAMP NOT NULL,
    window_end TIMESTAMP NOT NULL,
    window_size TEXT NOT NULL, -- '1min', '5min', '1hour'

    -- 维度
    provider_name TEXT,
    model TEXT,

    -- 请求统计
    total_requests INTEGER NOT NULL DEFAULT 0,
    successful_requests INTEGER NOT NULL DEFAULT 0,
    failed_requests INTEGER NOT NULL DEFAULT 0,

    -- 延迟统计（毫秒）
    min_duration_ms INTEGER,
    max_duration_ms INTEGER,
    avg_duration_ms INTEGER,
    p50_duration_ms INTEGER,
    p95_duration_ms INTEGER,
    p99_duration_ms INTEGER,

    -- Token 统计
    total_tokens BIGINT NOT NULL DEFAULT 0,
    avg_tokens_per_request DECIMAL(10, 2),

    -- 吞吐量（requests per second）
    requests_per_second DECIMAL(10, 2),
    tokens_per_second DECIMAL(10, 2),

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(window_start, window_size, provider_name, model)
);

CREATE INDEX idx_perf_metrics_window ON performance_metrics(window_start, window_size);
CREATE INDEX idx_perf_metrics_provider ON performance_metrics(provider_name, model);
```

---

### 6. Cost Attribution & Budgeting

#### `budget_allocations`
**目的：预算分配和跟踪**

```sql
CREATE TABLE budget_allocations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- 分配对象
    organization_id INTEGER REFERENCES organizations(id),
    team_id INTEGER REFERENCES teams(id),

    -- 预算信息
    amount_usd DECIMAL(12, 4) NOT NULL,
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,

    -- 使用情况
    consumed_usd DECIMAL(12, 4) NOT NULL DEFAULT 0,
    remaining_usd DECIMAL(12, 4) GENERATED ALWAYS AS (amount_usd - consumed_usd) VIRTUAL,

    -- 预警阈值
    alert_threshold_pct INTEGER DEFAULT 80, -- 达到 80% 时预警

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_budget_org ON budget_allocations(organization_id);
CREATE INDEX idx_budget_team ON budget_allocations(team_id);
CREATE INDEX idx_budget_period ON budget_allocations(period_start, period_end);
```

#### `cost_attribution_daily`
**目的：每日成本归因汇总（用于快速查询）**

```sql
CREATE TABLE cost_attribution_daily (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    date DATE NOT NULL,

    -- 归因维度
    organization_id INTEGER REFERENCES organizations(id),
    team_id INTEGER REFERENCES teams(id),
    user_id INTEGER REFERENCES users(id),

    -- Provider/Model 维度
    provider_name TEXT,
    model TEXT,

    -- 方向
    direction TEXT NOT NULL CHECK(direction IN ('consume','supply')),

    -- 聚合指标
    total_requests INTEGER NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    total_cost_usd DECIMAL(12, 4) NOT NULL DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(date, organization_id, team_id, user_id, provider_name, model, direction)
);

CREATE INDEX idx_cost_daily_date ON cost_attribution_daily(date DESC);
CREATE INDEX idx_cost_daily_org ON cost_attribution_daily(organization_id, date DESC);
CREATE INDEX idx_cost_daily_team ON cost_attribution_daily(team_id, date DESC);
CREATE INDEX idx_cost_daily_user ON cost_attribution_daily(user_id, date DESC);
CREATE INDEX idx_cost_daily_provider ON cost_attribution_daily(provider_name, model);
```

---

## Data Flow Example: Reseller Scenario

### Setup: 用户配置为 Reseller

```sql
-- 1. 创建 Organization
INSERT INTO organizations (name, slug) VALUES ('AI Resale Corp', 'ai-resale-corp');

-- 2. 创建 Provider Team（上游）
INSERT INTO teams (
    organization_id,
    name,
    slug,
    team_type,
    service_endpoint
) VALUES (
    1,
    'OpenAI GPT-4 Service',
    'openai-gpt4',
    'project',
    'https://api.openai.com/v1/chat/completions'
);

-- 3. 配置外部 Provider
INSERT INTO provider_model_catalog (
    provider_name,
    model,
    endpoint,
    price_per_1k_prompt_tokens,
    price_per_1k_completion_tokens
) VALUES (
    'openai',
    'gpt-4',
    'https://api.openai.com/v1/chat/completions',
    0.03,  -- $0.03 per 1k prompt tokens
    0.06   -- $0.06 per 1k completion tokens
);

-- 4. 创建下游用户
INSERT INTO users (email, role, organization_id) VALUES ('customer@example.com', 'gateway_user', 1);

-- 5. 为下游用户创建 API Key
INSERT INTO api_keys (user_id, key_hash, key_prefix) VALUES (2, 'hash...', 'tok_abc123');
```

### Transaction Flow: 下游用户发起请求

```sql
-- 1. 记录 consumption（从上游 OpenAI 消费）
INSERT INTO usage_entries (
    user_id,
    api_key_id,
    organization_id,
    team_id,
    service_id,
    provider_name,
    model,
    prompt_tokens,
    completion_tokens,
    direction,
    cost_usd,
    request_id,
    request_duration_ms
) VALUES (
    1,  -- Gateway admin (代表 gateway 本身)
    NULL,
    1,
    1,
    0,
    'openai',
    'gpt-4',
    100,
    200,
    'consume',
    0.009,  -- (100 * 0.03 + 200 * 0.06) / 1000
    'req_abc123',
    1500
);

-- 2. 记录 supply（提供给下游用户）
INSERT INTO usage_entries (
    user_id,
    api_key_id,
    organization_id,
    team_id,
    service_id,
    provider_name,
    model,
    prompt_tokens,
    completion_tokens,
    direction,
    cost_usd,
    request_id,
    request_duration_ms
) VALUES (
    2,  -- 下游用户
    5,  -- 下游用户的 API key
    1,
    1,
    123,  -- marketplace service ID
    'openai',
    'gpt-4',
    100,
    200,
    'supply',
    0.012,  -- 加价 33%
    'req_abc123',
    1500
);

-- 3. 记录详细请求日志
INSERT INTO request_logs (
    request_id,
    user_id,
    api_key_id,
    method,
    path,
    provider_name,
    model,
    request_started_at,
    request_completed_at,
    status_code,
    prompt_tokens,
    completion_tokens,
    client_ip,
    user_agent
) VALUES (
    'req_abc123',
    2,
    5,
    'POST',
    '/v1/chat/completions',
    'openai',
    'gpt-4',
    '2025-01-15 10:30:00',
    '2025-01-15 10:30:01.5',
    200,
    100,
    200,
    '203.0.113.42',
    'python-requests/2.31.0'
);
```

---

## Key Queries

### Q1: Gateway 整体消费和供给情况

```sql
SELECT
    direction,
    SUM(total_tokens) as total_tokens,
    SUM(cost_usd) as total_cost,
    COUNT(*) as request_count
FROM usage_entries
WHERE created_at >= date('now', '-30 days')
  AND deleted_at IS NULL
GROUP BY direction;
```

### Q2: 按 Provider/Model 分组的消费统计

```sql
SELECT
    provider_name,
    model,
    direction,
    COUNT(*) as request_count,
    SUM(prompt_tokens) as total_prompt_tokens,
    SUM(completion_tokens) as total_completion_tokens,
    SUM(total_tokens) as total_tokens,
    SUM(cost_usd) as total_cost,
    AVG(request_duration_ms) as avg_duration_ms
FROM usage_entries
WHERE created_at >= date('now', '-7 days')
  AND deleted_at IS NULL
GROUP BY provider_name, model, direction
ORDER BY total_cost DESC;
```

### Q3: 某个 Team 的成本归因（用于 Reseller）

```sql
SELECT
    t.name as team_name,
    u.email as user_email,
    ue.direction,
    ue.model,
    SUM(ue.total_tokens) as total_tokens,
    SUM(ue.cost_usd) as total_cost
FROM usage_entries ue
JOIN teams t ON ue.team_id = t.id
JOIN users u ON ue.user_id = u.id
WHERE t.id = 1
  AND ue.created_at >= date('now', '-30 days')
  AND ue.deleted_at IS NULL
GROUP BY t.name, u.email, ue.direction, ue.model
ORDER BY total_cost DESC;
```

### Q4: Gateway 性能指标（最近 1 小时）

```sql
SELECT
    provider_name,
    model,
    COUNT(*) as request_count,
    AVG(request_duration_ms) as avg_latency_ms,
    MIN(request_duration_ms) as min_latency_ms,
    MAX(request_duration_ms) as max_latency_ms,
    SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) as error_count,
    CAST(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END) AS FLOAT) / COUNT(*) as error_rate
FROM request_logs
WHERE request_started_at >= datetime('now', '-1 hour')
  AND deleted_at IS NULL
GROUP BY provider_name, model
ORDER BY request_count DESC;
```

### Q5: 利润分析（Reseller 场景）

```sql
WITH consumption AS (
    SELECT SUM(cost_usd) as total_consumed
    FROM usage_entries
    WHERE direction = 'consume'
      AND created_at >= date('now', '-30 days')
      AND deleted_at IS NULL
),
supply AS (
    SELECT SUM(cost_usd) as total_supplied
    FROM usage_entries
    WHERE direction = 'supply'
      AND created_at >= date('now', '-30 days')
      AND deleted_at IS NULL
)
SELECT
    c.total_consumed,
    s.total_supplied,
    (s.total_supplied - c.total_consumed) as profit,
    ROUND((s.total_supplied - c.total_consumed) / c.total_consumed * 100, 2) as profit_margin_pct
FROM consumption c, supply s;
```

### Q6: 按时间段的流量趋势

```sql
SELECT
    date(created_at) as date,
    direction,
    provider_name,
    model,
    COUNT(*) as request_count,
    SUM(total_tokens) as total_tokens,
    AVG(request_duration_ms) as avg_latency_ms
FROM usage_entries
WHERE created_at >= date('now', '-30 days')
  AND deleted_at IS NULL
GROUP BY date(created_at), direction, provider_name, model
ORDER BY date DESC, request_count DESC;
```

### Q7: Top 消费用户（用于识别大客户）

```sql
SELECT
    u.email,
    u.employee_id,
    t.name as team_name,
    COUNT(*) as request_count,
    SUM(ue.total_tokens) as total_tokens,
    SUM(ue.cost_usd) as total_cost
FROM usage_entries ue
JOIN users u ON ue.user_id = u.id
LEFT JOIN teams t ON ue.team_id = t.id
WHERE ue.direction = 'supply'
  AND ue.created_at >= date('now', '-30 days')
  AND ue.deleted_at IS NULL
GROUP BY u.id, u.email, u.employee_id, t.name
ORDER BY total_cost DESC
LIMIT 10;
```

---

## Migration Strategy

### Phase 1: 扩展现有表

```sql
-- 1. 为 users 表添加组织字段
ALTER TABLE users ADD COLUMN organization_id INTEGER REFERENCES organizations(id);
ALTER TABLE users ADD COLUMN team_id INTEGER REFERENCES teams(id);
ALTER TABLE users ADD COLUMN employee_id TEXT;
ALTER TABLE users ADD COLUMN quota_tokens BIGINT;

-- 2. 为 usage_entries 表添加扩展字段
ALTER TABLE usage_entries ADD COLUMN organization_id INTEGER REFERENCES organizations(id);
ALTER TABLE usage_entries ADD COLUMN team_id INTEGER REFERENCES teams(id);
ALTER TABLE usage_entries ADD COLUMN provider_name TEXT;
ALTER TABLE usage_entries ADD COLUMN model TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE usage_entries ADD COLUMN cost_usd DECIMAL(12, 6);
ALTER TABLE usage_entries ADD COLUMN request_id TEXT;
ALTER TABLE usage_entries ADD COLUMN request_duration_ms INTEGER;
ALTER TABLE usage_entries ADD COLUMN metadata TEXT;

-- 3. 创建索引
CREATE INDEX idx_users_org ON users(organization_id);
CREATE INDEX idx_usage_entries_org ON usage_entries(organization_id);
CREATE INDEX idx_usage_entries_team ON usage_entries(team_id);
CREATE INDEX idx_usage_entries_model ON usage_entries(model);
CREATE INDEX idx_usage_entries_provider ON usage_entries(provider_name);
```

### Phase 2: 创建新表

```sql
-- 依次创建：
-- 1. organizations
-- 2. teams
-- 3. user_teams
-- 4. provider_model_catalog
-- 5. gateway_usage_summary
-- 6. request_logs
-- 7. performance_metrics
-- 8. budget_allocations
-- 9. cost_attribution_daily
```

### Phase 3: 数据迁移

```sql
-- 1. 创建默认 Organization（如果用户没有）
INSERT INTO organizations (name, slug)
SELECT 'Default Organization', 'default'
WHERE NOT EXISTS (SELECT 1 FROM organizations);

-- 2. 将现有用户关联到默认 Organization
UPDATE users SET organization_id = (SELECT id FROM organizations WHERE slug = 'default')
WHERE organization_id IS NULL;

-- 3. 回填 usage_entries 的 organization_id
UPDATE usage_entries ue
SET organization_id = (SELECT organization_id FROM users WHERE id = ue.user_id)
WHERE organization_id IS NULL;
```

---

## Best Practices

### 1. 数据保留策略

```sql
-- 定期清理旧的 request_logs（保留 30 天）
DELETE FROM request_logs
WHERE created_at < date('now', '-30 days');

-- 聚合旧数据到 gateway_usage_summary 后删除 usage_entries（保留 90 天）
-- 注意：只删除已聚合的数据
```

### 2. 性能优化

- 使用分区表（PostgreSQL）按月分区 `usage_entries` 和 `request_logs`
- 定期 VACUUM 和 ANALYZE
- 为高频查询创建物化视图

### 3. 实时统计

```go
// 使用 Redis 做实时缓存
type RealtimeStats struct {
    TotalRequests      int64
    RequestsPerSecond  float64
    TotalTokens        int64
    TokensPerSecond    float64
    AvgLatencyMs       float64
}

// 每秒更新一次，从 Redis 读取即时返回
```

---

## Conclusion

这个 schema 设计支持：

✅ **DUAL 模式**：同时作为 Provider 和 Consumer
✅ **Reseller 场景**：转手生意，跟踪上下游流量
✅ **三层组织结构**：Organization → Team → User
✅ **完整的成本归因**：按组织、团队、用户、模型分析
✅ **性能监控**：延迟、吞吐量、错误率
✅ **Gateway 级别统计**：整体消费和供给情况
✅ **灵活扩展**：支持未来添加更多维度

下一步建议：
1. 实现 Schema Migration 脚本
2. 编写 Store 接口的扩展方法
3. 实现聚合任务（定时将详细数据汇总到 summary 表）
4. 添加 Grafana Dashboard 配置
