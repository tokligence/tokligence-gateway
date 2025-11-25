# Phase 1: PostgreSQL Integration (Team Edition)

## 现有PostgreSQL Schema

当前gateway已经实现的PostgreSQL tables（Team Edition）:

### 1. users 表
```sql
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,
    display_name TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 2. api_keys 表
```sql
CREATE TABLE IF NOT EXISTS api_keys (
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

**现有索引**:
```sql
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
```

---

## Phase 1新增表：api_key_priority_mappings

### 设计原则

**关键关系**:
- `api_keys.key_prefix` (e.g., "tok_ABC123") → `api_key_priority_mappings.pattern` (e.g., "tok_ABC*")
- Pattern matching决定priority queue (P0-P9)
- **不需要外键关联** - pattern是通配符匹配，不是1:1关系

### PostgreSQL表结构

```sql
-- Phase 1: API Key to Priority Mapping
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    id SERIAL PRIMARY KEY,

    -- API Key pattern (wildcard matching)
    pattern TEXT NOT NULL UNIQUE,  -- e.g., "tok_dept_prod*", "tok_ext_premium*"

    -- Target priority queue (P0-P9)
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),

    -- Pattern match type
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),

    -- Multi-tenant metadata (类似LiteLLM)
    tenant_id TEXT,           -- Tenant identifier (e.g., "dept-prod", "ext-enterprise")
    tenant_name TEXT,         -- Human-readable name (e.g., "Production Team")
    tenant_type TEXT,         -- "internal" or "external"
    description TEXT,

    -- Status
    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT
);

-- Indexes for fast lookup
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_pattern ON api_key_priority_mappings(pattern);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_enabled ON api_key_priority_mappings(enabled);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_tenant_id ON api_key_priority_mappings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_priority ON api_key_priority_mappings(priority);
```

### 配置表 (可选，也可以用环境变量)

```sql
CREATE TABLE IF NOT EXISTS api_key_priority_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO api_key_priority_config (key, value, description) VALUES
('enabled', 'false', 'Enable/disable API key priority mapping (false for Personal Edition, true for Team Edition)'),
('default_priority', '7', 'Default priority for unmapped keys (P7 = Standard tier)'),
('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds (5 minutes)');
```

---

## 与现有Schema的集成

### 场景1: 基于user_id的租户隔离

**需求**: 每个user（或user group）有不同的priority

**方案**: 通过api_key的key_prefix pattern映射到tenant

```sql
-- 示例：Production team的所有API keys都有"tok_prod"前缀
INSERT INTO api_key_priority_mappings
(pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by)
VALUES
('tok_prod*', 0, 'prefix', 'dept-prod', 'Production Team', 'internal', 'Production workloads - P0 queue', 'admin');

-- Gateway在处理请求时：
-- 1. 从Authorization header提取token: "tok_prodABC123xyz..."
-- 2. APIKeyMapper.GetPriority("tok_prodABC123xyz...") → matches "tok_prod*" → returns P0
-- 3. Submit to scheduler with priority=P0
```

### 场景2: 动态添加新租户

**场景**: 新增一个ML Research team，需要P1优先级

```sql
-- Step 1: 管理员创建user (已有功能)
INSERT INTO users (email, role, display_name, status)
VALUES ('ml-team-lead@company.com', 'gateway_admin', 'ML Research Team Lead', 'active');

-- Step 2: 为该user创建API key with custom prefix (需要minor enhancement)
-- 当前: key_prefix = "tok_ABC123" (random)
-- 增强: key_prefix = "tok_ml_ABC123" (with tenant prefix)

-- Step 3: 添加priority mapping
INSERT INTO api_key_priority_mappings
(pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by)
VALUES
('tok_ml*', 1, 'prefix', 'dept-ml', 'ML Research Team', 'internal', 'ML research - P1 queue', 'admin');

-- Step 4: Reload cache
-- POST /admin/api-key-priority/reload
```

---

## API Key Prefix命名规范（推荐）

为了更好地支持多租户pattern matching，推荐使用有语义的prefix：

### 当前实现
```go
// internal/userstore/postgres/postgres.go
func generateAPIKey() (token, prefix, hash string, err error) {
    var buf [32]byte
    if _, err = rand.Read(buf[:]); err != nil {
        return "", "", "", err
    }
    token = apiKeyTokenPrefix + base64.RawURLEncoding.EncodeToString(buf[:])
    // token example: "tok_xYz9AbC..." (random, no semantic meaning)
    prefix = token[:apiKeyPrefixLength]  // "tok_xYz9AbC1"
    // ...
}
```

### Phase 1增强（可选，未来优化）

```go
// Option 1: Add tenant_prefix parameter to CreateAPIKey
func (s *Store) CreateAPIKeyWithTenant(ctx context.Context, userID int64, tenantPrefix string, scopes []string, expiresAt *time.Time) (*userstore.APIKey, string, error) {
    // token example: "tok_prod_xYz9AbC..." (with tenant prefix)
    token, prefix, hash, err := generateAPIKeyWithTenant(tenantPrefix)
    // prefix = "tok_prod_xYz"
    // ...
}

// Option 2: Derive tenant from user metadata
func (s *Store) CreateAPIKey(ctx context.Context, userID int64, scopes []string, expiresAt *time.Time) (*userstore.APIKey, string, error) {
    // Query user to get tenant info
    user, err := s.userByID(ctx, userID)
    tenantPrefix := deriveTenantPrefix(user)  // e.g., "prod", "ml", "ext_premium"
    token, prefix, hash, err := generateAPIKeyWithTenant(tenantPrefix)
    // ...
}
```

**Trade-off**:
- ✅ Pro: 语义化prefix，更容易管理和pattern matching
- ❌ Con: 需要修改现有CreateAPIKey实现
- ⚠️ **Phase 1暂不实现**，使用pattern matching即可兼容现有random prefix

---

## 实际使用流程

### 管理员视角

**Step 1: 创建租户mapping**

```bash
# 添加Production team mapping
curl -X POST http://gateway:8081/admin/api-key-priority/mappings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin_token" \
  -d '{
    "pattern": "tok_prod*",
    "priority": 0,
    "match_type": "prefix",
    "tenant_id": "dept-prod",
    "tenant_name": "Production Team",
    "tenant_type": "internal",
    "description": "Production workloads - Highest priority (P0 queue)"
  }'

# 添加External Premium mapping
curl -X POST http://gateway:8081/admin/api-key-priority/mappings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin_token" \
  -d '{
    "pattern": "tok_ext_prem*",
    "priority": 6,
    "match_type": "prefix",
    "tenant_id": "ext-premium",
    "tenant_name": "Premium Customers",
    "tenant_type": "external",
    "description": "Premium tier - P6 queue"
  }'
```

**Step 2: 查看所有mappings**

```bash
curl http://gateway:8081/admin/api-key-priority/mappings \
  -H "Authorization: Bearer admin_token" | jq .

# Response
{
  "mappings": [
    {
      "id": 1,
      "pattern": "tok_prod*",
      "priority": 0,
      "match_type": "prefix",
      "tenant_id": "dept-prod",
      "tenant_name": "Production Team",
      "tenant_type": "internal",
      "enabled": true,
      "created_at": "2025-01-23T10:00:00Z"
    },
    {
      "id": 2,
      "pattern": "tok_ext_prem*",
      "priority": 6,
      "match_type": "prefix",
      "tenant_id": "ext-premium",
      "tenant_name": "Premium Customers",
      "tenant_type": "external",
      "enabled": true,
      "created_at": "2025-01-23T10:05:00Z"
    }
  ],
  "total": 2
}
```

**Step 3: 动态调整tenant priority**

```bash
# ML team临时需要提升到P0（重要训练任务）
curl -X PUT http://gateway:8081/admin/api-key-priority/mappings/3 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin_token" \
  -d '{
    "priority": 0,
    "description": "ML team - Urgent training (temporarily boosted to P0)",
    "enabled": true
  }'

# Reload cache to pick up changes immediately
curl -X POST http://gateway:8081/admin/api-key-priority/reload \
  -H "Authorization: Bearer admin_token"

# Response
{
  "success": true,
  "message": "Mappings reloaded successfully",
  "reloaded_count": 3,
  "reload_time_ms": 12
}
```

### 用户（API调用者）视角

用户完全无感知，只需使用API key即可：

```bash
# Production team request (automatically routed to P0 queue)
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "Authorization: Bearer tok_prodABC123xyz..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Analyze production data"}]
  }'

# Gateway内部处理:
# 1. Extract token: "tok_prodABC123xyz..."
# 2. Pattern match: "tok_prod*" → priority = P0
# 3. Submit to P0 queue (highest priority)
# 4. Process request immediately (P0 always processed first)
```

---

## Migration Script (Team Edition)

### 初始化Phase 1表

```sql
-- File: internal/userstore/postgres/migrations/002_api_key_priority_mappings.sql

BEGIN;

-- Create api_key_priority_mappings table
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    id SERIAL PRIMARY KEY,
    pattern TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),
    tenant_id TEXT,
    tenant_name TEXT,
    tenant_type TEXT,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_pattern ON api_key_priority_mappings(pattern);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_enabled ON api_key_priority_mappings(enabled);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_tenant_id ON api_key_priority_mappings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_priority ON api_key_priority_mappings(priority);

-- Create config table
CREATE TABLE IF NOT EXISTS api_key_priority_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default config
INSERT INTO api_key_priority_config (key, value, description) VALUES
('enabled', 'false', 'Enable/disable API key priority mapping (false for Personal Edition)'),
('default_priority', '7', 'Default priority for unmapped keys'),
('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds')
ON CONFLICT (key) DO NOTHING;

-- Insert example mappings (commented out, for reference only)
-- Uncomment and modify for production use

-- -- Internal departments (P0-P3)
-- INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by) VALUES
-- ('tok_prod*', 0, 'prefix', 'dept-prod', 'Production Team', 'internal', 'Production workloads - P0 queue', 'system'),
-- ('tok_ml*', 1, 'prefix', 'dept-ml', 'ML Research Team', 'internal', 'ML research - P1 queue', 'system'),
-- ('tok_analytics*', 2, 'prefix', 'dept-analytics', 'Analytics Team', 'internal', 'Business analytics - P2 queue', 'system'),
-- ('tok_dev*', 3, 'prefix', 'dept-dev', 'Development Team', 'internal', 'Development - P3 queue', 'system');
--
-- -- External customers (P5-P9)
-- INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by) VALUES
-- ('tok_ext_ent*', 5, 'prefix', 'ext-enterprise', 'Enterprise Customers', 'external', 'Enterprise tier - P5 queue', 'system'),
-- ('tok_ext_prem*', 6, 'prefix', 'ext-premium', 'Premium Customers', 'external', 'Premium tier - P6 queue', 'system'),
-- ('tok_ext_std*', 7, 'prefix', 'ext-standard', 'Standard Customers', 'external', 'Standard tier - P7 queue', 'system'),
-- ('tok_ext_free*', 9, 'prefix', 'ext-free', 'Free Tier Users', 'external', 'Free tier - P9 queue', 'system');

COMMIT;
```

### 集成到postgres.go

```go
// internal/userstore/postgres/postgres.go

func (s *Store) initSchema() error {
    const schema = `
-- Existing tables
CREATE TABLE IF NOT EXISTS users (...);
CREATE TABLE IF NOT EXISTS api_keys (...);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);

-- Phase 1: API Key Priority Mappings (Team Edition only)
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    id SERIAL PRIMARY KEY,
    pattern TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),
    tenant_id TEXT,
    tenant_name TEXT,
    tenant_type TEXT,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_pattern ON api_key_priority_mappings(pattern);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_enabled ON api_key_priority_mappings(enabled);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_tenant_id ON api_key_priority_mappings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_priority ON api_key_priority_mappings(priority);

CREATE TABLE IF NOT EXISTS api_key_priority_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO api_key_priority_config (key, value, description) VALUES
('enabled', 'false', 'Enable/disable API key priority mapping'),
('default_priority', '7', 'Default priority for unmapped keys'),
('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds')
ON CONFLICT (key) DO NOTHING;
`
    if _, err := s.db.Exec(schema); err != nil {
        return fmt.Errorf("apply schema: %w", err)
    }

    // ... existing ensureColumn calls ...

    return nil
}
```

---

## Personal Edition vs Team Edition

### Personal Edition (SQLite)
- `api_key_priority_mappings` table **不创建**
- Mapper **不初始化**
- `enabled = false` in config
- 所有请求使用default priority (P7)
- **零开销**

### Team Edition (PostgreSQL)
- `api_key_priority_mappings` table **创建**
- Mapper **初始化**（with database connection）
- `enabled = true` in config (optional, can be set via admin UI)
- 动态priority mapping
- TTL-based cache (default 5min)
- RESTful CRUD API

---

## Summary

### ✅ PostgreSQL Integration完全支持多租户

| Feature | 实现方式 | 位置 |
|---------|---------|-----|
| **Database Schema** | PostgreSQL (Team Edition) | `internal/userstore/postgres/postgres.go` |
| **Table Integration** | `api_key_priority_mappings` table added | Phase 1 migration |
| **Pattern Matching** | Prefix/Suffix/Regex support | `APIKeyMapper.GetPriority()` |
| **Cache Strategy** | TTL-based + Manual reload | `APIKeyMapper.Reload()` |
| **Multi-tenant Metadata** | tenant_id, tenant_name, tenant_type fields | Database columns |
| **Dynamic Updates** | Database UPDATE + Reload API | 1-2秒生效 |
| **RESTful API** | CRUD endpoints | `endpoint_api_key_priority.go` |
| **Backward Compatible** | Existing api_keys table不变 | No breaking changes |

### 关键设计

1. **No Foreign Key**: `api_key_priority_mappings`独立表，通过pattern matching关联
2. **Flexible Patterns**: 支持wildcard (`tok_prod*`)，兼容现有random prefix
3. **Team Edition Only**: Personal Edition (SQLite)不创建该表
4. **Cache Performance**: In-memory cache + RWMutex，查询~100ns
5. **Dynamic Updates**: Database-driven，1-2秒生效（Manual reload）

### 下一步

1. ✅ Phase 1设计完成（支持PostgreSQL）
2. ⏭ 实现Migration script
3. ⏭ 实现APIKeyMapper (PostgreSQL backend)
4. ⏭ 实现CRUD HTTP API
5. ⏭ 集成到main.go (Team Edition条件初始化)
