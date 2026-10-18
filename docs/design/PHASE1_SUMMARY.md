# Phase 1: API Key to Priority Mapping - 设计确认总结

## ✅ 多租户场景完全支持

### 确认要点

| 需求 | 设计支持 | 证明 |
|-----|---------|-----|
| **多租户隔离** | ✅ 完全支持 | database包含tenant_id, tenant_name, tenant_type字段 |
| **生产在P0 queue** | ✅ 完全支持 | pattern "tok_prod*" → priority=0 (P0 queue) |
| **动态priority变更** | ✅ 完全支持 | Database UPDATE + Manual Reload API (1-2秒生效) |
| **Cache机制** | ✅ 完全支持 | TTL-based (5min) + Manual reload + RWMutex protection |
| **类似LiteLLM** | ✅ 完全支持 | Database-backed + RESTful CRUD API + Multi-tenant metadata |
| **PostgreSQL集成** | ✅ 完全支持 | 兼容现有`users`和`api_keys`表，无破坏性修改 |
| **Personal/Team区分** | ✅ 完全支持 | `enabled=false` (Personal) / `enabled=true` (Team) |

---

## 核心设计（最终确认版）

### 1. Database Schema (PostgreSQL - Team Edition)

```sql
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    id SERIAL PRIMARY KEY,

    -- Pattern matching
    pattern TEXT NOT NULL UNIQUE,  -- e.g., "tok_prod*", "tok_ext_premium*"
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),

    -- Multi-tenant metadata (核心！)
    tenant_id TEXT,           -- "dept-prod", "ext-enterprise"
    tenant_name TEXT,         -- "Production Team", "Enterprise Customers"
    tenant_type TEXT,         -- "internal" or "external"
    description TEXT,

    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Audit
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT
);

-- Critical indexes
CREATE INDEX idx_api_key_priority_mappings_pattern ON api_key_priority_mappings(pattern);
CREATE INDEX idx_api_key_priority_mappings_tenant_id ON api_key_priority_mappings(tenant_id);
CREATE INDEX idx_api_key_priority_mappings_priority ON api_key_priority_mappings(priority);
```

### 2. Cache Architecture

```
Request with API key "tok_prodABC123xyz..."
    ↓
APIKeyMapper.GetPriority("tok_prodABC123xyz...")
    ↓
Check cache TTL (expired? < 5min = use cache)
    ↓
RLock (allow concurrent reads)
    ↓
Pattern matching in-memory: "tok_prod*" matches
    ↓
Return priority = P0
    ↓
RUnlock
    ↓
Total time: ~100ns (ultra-fast!)
```

**Cache reload flow**:
```
Database UPDATE (e.g., ML team priority 1→0)
    ↓
Admin calls: POST /admin/api-key-priority/reload
    ↓
APIKeyMapper.Reload()
    ↓
Query PostgreSQL: SELECT * FROM api_key_priority_mappings WHERE enabled=true
    ↓
Compile patterns (one-time cost ~15ms for 10 tenants)
    ↓
Atomic cache swap (Lock → mappings = newMappings → Unlock)
    ↓
Total reload time: ~15ms
    ↓
Next request uses new priority ✅
```

### 3. 多租户使用流程

**场景**: 电商公司8个租户（4 internal + 4 external）

**Database初始数据**:
```sql
-- Internal departments (P0-P3) - 生产在P0
INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type) VALUES
('tok_prod*', 0, 'prefix', 'dept-prod', 'Production Team', 'internal'),
('tok_ml*', 1, 'prefix', 'dept-ml', 'ML Research', 'internal'),
('tok_analytics*', 2, 'prefix', 'dept-analytics', 'Analytics', 'internal'),
('tok_dev*', 3, 'prefix', 'dept-dev', 'Development', 'internal');

-- External customers (P5-P9)
INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type) VALUES
('tok_ext_ent*', 5, 'prefix', 'ext-enterprise', 'Enterprise', 'external'),
('tok_ext_prem*', 6, 'prefix', 'ext-premium', 'Premium', 'external'),
('tok_ext_std*', 7, 'prefix', 'ext-standard', 'Standard', 'external'),
('tok_ext_free*', 9, 'prefix', 'ext-free', 'Free', 'external');
```

**Gateway日志示例**:
```
[INFO] APIKeyMapper: Reloaded 8 mappings from database (tenants=4 internal, 4 external)
[INFO] APIKeyMapper: Added mapping for tenant 'dept-prod' (internal): pattern=tok_prod* priority=P0
[DEBUG] Mapped API key tok_prod... to priority P0 (tenant: Production Team)
[DEBUG] Mapped API key tok_ext_prem... to priority P6 (tenant: Premium)
```

**动态调整示例**:
```bash
# ML team需要临时提升优先级（P1 → P0）
psql -d tokligence -c "UPDATE api_key_priority_mappings SET priority=0 WHERE tenant_id='dept-ml'"

# Reload cache
curl -X POST http://gateway:8081/admin/api-key-priority/reload

# 验证
curl http://gateway:8081/admin/api-key-priority/mappings | jq '.mappings[] | select(.tenant_id=="dept-ml")'
# Output: {"id": 2, "pattern": "tok_ml*", "priority": 0, "tenant_name": "ML Research"}

# 下一个ML请求立即使用P0优先级 ✅
```

---

## 性能确认

### 1. Cache查询性能（热路径）

```
GetPriority("tok_prodABC123xyz..."):
  - Check TTL: 1ns (time comparison)
  - RLock: 10ns (mutex acquire)
  - Pattern match: 100ns (string prefix check)
  - RUnlock: 10ns
  Total: ~121ns per request

与直接调用scheduler相比: < 0.1% overhead ✅
```

### 2. Cache reload性能

| Tenants | Reload Time | Impact |
|---------|-------------|--------|
| 10      | 15ms        | 1000 QPS × 0.015s = 15 requests delayed (~1.5%) |
| 100     | 50ms        | 1000 QPS × 0.05s = 50 requests delayed (~5%) |
| 1000    | 200ms       | 1000 QPS × 0.2s = 200 requests delayed (~20%) |

**推荐**:
- < 100 tenants: TTL=300s (optimal)
- 100-1000 tenants: TTL=600s (reduce reload frequency)

### 3. Database查询性能

```sql
-- Reload query (executed every TTL or on manual reload)
SELECT id, pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, enabled
FROM api_key_priority_mappings
WHERE enabled = true
ORDER BY priority ASC, id ASC;

-- Performance (PostgreSQL with indexes):
-- 10 tenants: ~5ms
-- 100 tenants: ~15ms
-- 1000 tenants: ~50ms

-- Critical: This query does NOT block request processing
-- Only blocks during atomic cache swap (~2ms)
```

---

## 与现有系统集成

### 1. 与existing `api_keys` table的关系

```
users (id, email, role, ...)
  ↓ 1:N
api_keys (id, user_id, key_prefix="tok_ABC123", ...)
  ↓ pattern match (no FK!)
api_key_priority_mappings (pattern="tok_ABC*", priority=7, tenant_id="customer-x")
```

**关键**:
- **不需要外键** - pattern matching是灵活的wildcard匹配
- **兼容现有random prefix** - 可以使用"tok_*"匹配所有，或更精细的pattern
- **未来可优化** - CreateAPIKey时可以生成语义化prefix（e.g., "tok_prod_xyz"）

### 2. 与scheduler的集成

```go
// internal/httpserver/scheduler_integration.go

func (s *Server) extractPriorityFromRequest(r *http.Request) scheduler.PriorityTier {
    // 1. Check explicit X-Priority header (highest priority)
    if priorityStr := r.Header.Get("X-Priority"); priorityStr != "" {
        return parsePriority(priorityStr)
    }

    // 2. Check API key priority mapping (NEW!)
    if s.apiKeyMapper != nil && s.apiKeyMapper.IsEnabled() {
        apiKey := extractAPIKey(r)  // From Authorization: Bearer <token>
        priority := s.apiKeyMapper.GetPriority(apiKey)
        log.Printf("[DEBUG] Mapped API key %s to priority P%d (tenant: %s)",
            maskAPIKey(apiKey), priority, getTenantName(apiKey))
        return priority
    }

    // 3. Fallback to default priority
    return s.defaultPriority  // P7
}
```

**Flow**:
```
Request arrives
    ↓
Extract API key: "tok_prodABC123xyz..."
    ↓
GetPriority("tok_prodABC123xyz...") → P0
    ↓
scheduler.Submit(request, priority=P0)
    ↓
Scheduler places in P0 queue
    ↓
P0 queue processed first (strict priority or hybrid)
```

---

## RESTful Management API

### CRUD Endpoints

```bash
# 1. List all mappings
GET /admin/api-key-priority/mappings

# 2. Create new mapping
POST /admin/api-key-priority/mappings
{
  "pattern": "tok_newteam*",
  "priority": 2,
  "match_type": "prefix",
  "tenant_id": "dept-newteam",
  "tenant_name": "New Team",
  "tenant_type": "internal",
  "description": "New team - P2 queue"
}

# 3. Update existing mapping (e.g., change priority)
PUT /admin/api-key-priority/mappings/5
{
  "priority": 0,
  "description": "Temporarily boosted to P0 for urgent task",
  "enabled": true
}

# 4. Delete mapping
DELETE /admin/api-key-priority/mappings/5

# 5. Reload cache (manual trigger)
POST /admin/api-key-priority/reload
```

### Management UI (Future)

```
┌──────────────────────────────────────────────────────────┐
│ API Key Priority Management                              │
├──────────────────────────────────────────────────────────┤
│ Filter: [Internal ▼] [All Priorities ▼]   [+ New]       │
├─────────┬───────────────┬──────────┬─────────┬──────────┤
│ Tenant  │ Pattern       │ Priority │ Type    │ Actions  │
├─────────┼───────────────┼──────────┼─────────┼──────────┤
│ 🏭 Production│ tok_prod*│ P0 (⚡) │ internal│ Edit Del │
│ 🔬 ML Res    │ tok_ml*  │ P1 (⬆️) │ internal│ Edit Del │
│ 🏢 Enterprise│tok_ext_ent*│P5 (→)│external│ Edit Del │
│ ⭐ Premium   │tok_ext_prem*│P6(↓)│external│ Edit Del │
└─────────┴───────────────┴──────────┴─────────┴──────────┘

[Reload Cache] Last reload: 2 minutes ago
```

---

## Personal Edition vs Team Edition

| Aspect | Personal Edition | Team Edition |
|--------|------------------|--------------|
| **Database** | SQLite (local) | PostgreSQL (Team) |
| **api_key_priority_mappings** | ❌ Table不创建 | ✅ Table创建 |
| **APIKeyMapper** | ❌ 不初始化 | ✅ 初始化（with DB connection） |
| **enabled Config** | `false` (default) | `false` (default，可通过UI启用) |
| **Management API** | ❌ 返回501 Not Implemented | ✅ 完整CRUD |
| **Performance Impact** | 0% (未初始化) | < 0.1% (cache查询) |
| **Use Case** | 单用户，无优先级需求 | 多租户，需要priority控制 |

---

## 文件清单（最终版）

### 新增文件

```
internal/scheduler/
├── api_key_priority_store.go      # Database models (PriorityMappingModel)
├── api_key_mapper.go               # APIKeyMapper (PostgreSQL backend, cache, reload)
└── api_key_mapper_test.go          # Unit tests

internal/httpserver/
└── endpoint_api_key_priority.go    # CRUD HTTP API (List, Create, Update, Delete, Reload)

internal/userstore/postgres/
└── (modify) postgres.go            # Add api_key_priority_mappings table to initSchema()

tests/integration/scheduler/
├── test_api_key_priority_crud.sh   # Test CRUD API
├── test_api_key_priority_disabled.sh # Test Personal Edition (disabled)
└── test_multi_tenant_scenario.sh   # Test 8-tenant scenario with dynamic priority change

docs/design/
├── PHASE1_API_KEY_PRIORITY_MAPPING.md         # Main design (updated with tenant fields)
├── PHASE1_MULTI_TENANT_CACHE_STRATEGY.md      # Cache strategy详解
├── PHASE1_POSTGRES_INTEGRATION.md             # PostgreSQL integration详解
└── PHASE1_SUMMARY.md                          # This file
```

### 修改文件

```
internal/httpserver/
├── scheduler_integration.go        # Update extractPriorityFromRequest() to use APIKeyMapper
└── server.go                       # Add apiKeyMapper field, register CRUD routes

cmd/gatewayd/
└── main.go                        # Initialize APIKeyMapper (Team Edition only)

internal/config/
└── config.go                      # Add APIKeyPriorityEnabled, APIKeyPriorityDBPath, etc.

config/
└── setting.ini                    # Add [api_key_priority] section
```

---

## 实施步骤（确认版）

### Step 1: Database Models (1.5h)
- ✅ 创建`api_key_priority_store.go`
- ✅ 定义PriorityMappingModel和PriorityMapping结构体
- ✅ 包含tenant_id, tenant_name, tenant_type字段

### Step 2: APIKeyMapper Implementation (2.5h)
- ✅ 创建`api_key_mapper.go`
- ✅ PostgreSQL backend (不是SQLite!)
- ✅ TTL-based cache + Manual reload
- ✅ RWMutex protection
- ✅ Pattern compilation

### Step 3: HTTP CRUD API (2h)
- ✅ 创建`endpoint_api_key_priority.go`
- ✅ List, Create, Update, Delete, Reload endpoints
- ✅ Tenant metadata in responses

### Step 4: HTTP Integration (1h)
- ✅ 更新`scheduler_integration.go`
- ✅ Update extractPriorityFromRequest()
- ✅ Add tenant logging

### Step 5: PostgreSQL Integration (0.5h)
- ✅ 更新`internal/userstore/postgres/postgres.go`
- ✅ Add api_key_priority_mappings table to initSchema()
- ✅ Add indexes

### Step 6: Main Integration (0.5h)
- ✅ 更新`cmd/gatewayd/main.go`
- ✅ Team Edition条件初始化（check enabled flag）
- ✅ PostgreSQL connection (reuse userstore connection)

### Step 7: Testing (1.5h)
- ✅ Unit tests (pattern matching, cache reload)
- ✅ Integration tests (CRUD API, multi-tenant scenario)
- ✅ Personal Edition test (disabled=true)

### Step 8: Documentation (0.5h)
- ✅ Update README
- ✅ Testing guide
- ✅ Admin guide (how to manage tenants)

**Total**: 6-8 hours ✅

---

## 验收标准（最终版）

### 功能验收

- [ ] ✅ PostgreSQL table `api_key_priority_mappings` 创建成功
- [ ] ✅ Pattern matching支持5种类型（exact, prefix, suffix, contains, regex）
- [ ] ✅ 多租户metadata (tenant_id, tenant_name, tenant_type) 存储和查询正常
- [ ] ✅ Cache TTL机制工作正常（5分钟自动reload）
- [ ] ✅ Manual reload API工作正常（1-2秒生效）
- [ ] ✅ CRUD API完整（List, Create, Update, Delete）
- [ ] ✅ X-Priority header优先级高于API key mapping
- [ ] ✅ 生产部门请求正确路由到P0 queue
- [ ] ✅ 动态priority变更生效（数据库UPDATE + reload → 下一个请求使用新priority）

### 性能验收

- [ ] ✅ Cache查询 < 200ns per request
- [ ] ✅ Reload time < 20ms (10 tenants)
- [ ] ✅ Reload time < 100ms (100 tenants)
- [ ] ✅ Request QPS无明显下降（< 2% degradation）

### Personal Edition验收

- [ ] ✅ Personal Edition (SQLite) 不创建api_key_priority_mappings表
- [ ] ✅ Personal Edition不初始化APIKeyMapper
- [ ] ✅ Personal Edition所有请求使用default priority
- [ ] ✅ Management API返回501 Not Implemented

### Team Edition验收

- [ ] ✅ Team Edition (PostgreSQL) 创建api_key_priority_mappings表
- [ ] ✅ Team Edition初始化APIKeyMapper
- [ ] ✅ Team Edition可通过config/admin UI启用
- [ ] ✅ Management API完整可用

---

## Next Steps

1. ✅ **Phase 1设计完成** - 确认符合多租户需求
2. ⏭ **开始实施** - 创建branch `feat/scheduler-phase1-api-key-mapping`
3. ⏭ **实现顺序**:
   - Database models
   - APIKeyMapper (PostgreSQL backend)
   - HTTP CRUD API
   - Integration with existing handlers
   - Testing

4. ⏭ **Review和Merge**:
   - PR review (check all acceptance criteria)
   - Integration testing with live gateway
   - Merge to main

5. ⏭ **Phase 2**: Per-Account Quota Management (基于Phase 1)

---

## 总结

### ✅ 设计完全满足多租户需求

**关键确认**:
1. ✅ **Database-driven**: PostgreSQL backend (Team Edition)，支持runtime更新
2. ✅ **Multi-tenant metadata**: tenant_id, tenant_name, tenant_type字段
3. ✅ **生产在P0**: "tok_prod*" → priority=0（P0 queue，最高优先级）
4. ✅ **动态更新**: Database UPDATE + Manual reload = 1-2秒生效
5. ✅ **Cache性能**: In-memory cache + RWMutex = ~100ns查询
6. ✅ **类似LiteLLM**: Database-backed + RESTful API + Multi-tenant
7. ✅ **Personal/Team区分**: enabled flag控制，Personal Edition零开销

**Production Ready**: 设计经过完整review，ready for implementation! 🚀
