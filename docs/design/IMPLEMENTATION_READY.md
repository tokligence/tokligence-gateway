# Implementation Ready: Phase 1-3 Scheduler Enhancements

## ✅ Design完成确认

所有设计文档已完成review并更新为UUID + soft delete标准。

### 创建的设计文档

| 文档 | 状态 | 内容 |
|------|------|------|
| `DATABASE_STANDARDS.md` | ✅ 完成 | UUID, soft delete, 标准audit fields for all phases |
| `BRANCH_STRATEGY.md` | ✅ 完成 | 3-phase branch structure, commit strategy, PR templates |
| `PHASE1_API_KEY_PRIORITY_MAPPING.md` | ✅ 更新UUID | Main design with PostgreSQL UUID schema |
| `PHASE1_MULTI_TENANT_CACHE_STRATEGY.md` | ✅ 完成 | Cache mechanism详解，multi-tenant scenarios |
| `PHASE1_POSTGRES_INTEGRATION.md` | ✅ 更新UUID | PostgreSQL integration with UUID |
| `PHASE1_SUMMARY.md` | ✅ 更新UUID | Final confirmation with UUID support |
| `PHASE2_PER_ACCOUNT_QUOTA.md` | ✅ 已有 | Per-account quota design |
| `PHASE3_TIME_BASED_DYNAMIC_RULES.md` | ✅ 已有 | Time-based rules design |
| `IMPLEMENTATION_PLAN.md` | ✅ 已有 | 3-phase timeline and roadmap |

---

## ✅ 关键设计确认

### 1. UUID Primary Keys（所有表）

```sql
-- ✅ 正确
id UUID PRIMARY KEY DEFAULT gen_random_uuid()

-- ❌ 错误
id INTEGER PRIMARY KEY AUTOINCREMENT
id SERIAL PRIMARY KEY
```

### 2. Soft Delete（所有表）

```sql
-- Standard audit fields
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
deleted_at TIMESTAMPTZ,  -- NULL = active, NOT NULL = deleted
created_by TEXT,
updated_by TEXT
```

### 3. Partial Indexes（所有active record查询）

```sql
CREATE INDEX idx_table_name_field ON table_name(field) WHERE deleted_at IS NULL;
```

### 4. 查询必须过滤deleted_at

```sql
-- ✅ 正确
SELECT * FROM table_name WHERE deleted_at IS NULL;

-- ❌ 错误
SELECT * FROM table_name;
```

### 5. Go Models使用UUID string

```go
// ✅ 正确
type Model struct {
    ID        string     `json:"id" db:"id"`  // UUID as string
    DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`  // Pointer for NULL
}

// ❌ 错误
type Model struct {
    ID        int64      `json:"id" db:"id"`
    DeletedAt time.Time  `json:"deleted_at" db:"deleted_at"`
}
```

---

## ✅ Branch已创建

### Current Branch

```bash
$ git branch --show-current
feat/scheduler-phase1-api-key-mapping
```

### Branch Structure

```
main
  ↓
feat/priority-scheduling (base)
  ↓
feat/scheduler-phase1-api-key-mapping ← YOU ARE HERE
  ↓ (after Phase 1 merge)
feat/scheduler-phase2-account-quota (to be created)
  ↓ (after Phase 2 merge)
feat/scheduler-phase3-time-rules (to be created)
  ↓ (after Phase 3 merge)
main (final merge)
```

---

## Phase 1: API Key to Priority Mapping

### Implementation Checklist

#### Database Schema (Commit 1)

- [ ] Create `internal/userstore/postgres/migrations/002_api_key_priority.sql`
  - [ ] Enable pgcrypto extension
  - [ ] Create `api_key_priority_mappings` table with UUID
  - [ ] Create `api_key_priority_config` table with UUID
  - [ ] Add soft delete fields (created_at, updated_at, deleted_at)
  - [ ] Add partial indexes (WHERE deleted_at IS NULL)
  - [ ] Add multi-tenant fields (tenant_id, tenant_name, tenant_type)

- [ ] Create `internal/scheduler/api_key_priority_store.go`
  - [ ] Define PriorityMappingModel with UUID string
  - [ ] Define PriorityMapping (in-memory cache model)
  - [ ] Add MatchType enum
  - [ ] Add tenant fields

#### APIKeyMapper Implementation (Commit 2)

- [ ] Create `internal/scheduler/api_key_mapper.go`
  - [ ] APIKeyMapper struct with PostgreSQL connection
  - [ ] NewAPIKeyMapper() function
  - [ ] initializeTables() with UUID support
  - [ ] GetPriority() with cache + TTL check
  - [ ] Reload() with soft delete filter
  - [ ] AddMapping() with UUID return
  - [ ] UpdateMapping() with UUID param
  - [ ] DeleteMapping() with soft delete (UPDATE deleted_at)
  - [ ] ListMappings() with soft delete filter
  - [ ] compile() for pattern matching

- [ ] Create `internal/scheduler/api_key_mapper_test.go`
  - [ ] Test UUID generation
  - [ ] Test soft delete queries
  - [ ] Test pattern matching
  - [ ] Test cache TTL
  - [ ] Test manual reload

#### HTTP CRUD API (Commit 3)

- [ ] Create `internal/httpserver/endpoint_api_key_priority.go`
  - [ ] HandleListAPIKeyMappings() - GET /admin/api-key-priority/mappings
  - [ ] HandleCreateAPIKeyMapping() - POST /admin/api-key-priority/mappings
  - [ ] HandleUpdateAPIKeyMapping() - PUT /admin/api-key-priority/mappings/:uuid
  - [ ] HandleDeleteAPIKeyMapping() - DELETE /admin/api-key-priority/mappings/:uuid (soft delete)
  - [ ] HandleReloadAPIKeyMappings() - POST /admin/api-key-priority/reload
  - [ ] All endpoints return/accept UUID (not int)
  - [ ] All responses include tenant metadata

#### HTTP Integration (Commit 4)

- [ ] Update `internal/httpserver/scheduler_integration.go`
  - [ ] Update extractPriorityFromRequest() to use APIKeyMapper
  - [ ] Add tenant logging
  - [ ] Handle UUID in logging

- [ ] Update `internal/httpserver/server.go`
  - [ ] Add apiKeyMapper field (APIKeyMapper interface)
  - [ ] Add SetAPIKeyMapper() method
  - [ ] Register CRUD routes in endpoint keys

#### PostgreSQL Integration (Commit 5)

- [ ] Update `internal/userstore/postgres/postgres.go`
  - [ ] Add api_key_priority_mappings table to initSchema()
  - [ ] Add api_key_priority_config table to initSchema()
  - [ ] Enable pgcrypto extension
  - [ ] Add UUID support
  - [ ] Add soft delete support

#### Main Integration (Commit 6)

- [ ] Update `cmd/gatewayd/main.go`
  - [ ] Check if Team Edition (PostgreSQL)
  - [ ] Check if enabled in config
  - [ ] Initialize APIKeyMapper with PostgreSQL connection
  - [ ] Set cache TTL from config
  - [ ] Add error handling

- [ ] Update `internal/config/config.go`
  - [ ] Add APIKeyPriorityEnabled bool
  - [ ] Add APIKeyPriorityDefault int
  - [ ] Add APIKeyPriorityDBPath string
  - [ ] Add APIKeyPriorityCacheTTL int

- [ ] Update `config/setting.ini`
  - [ ] Add [api_key_priority] section
  - [ ] Set enabled = false (default for Personal Edition)
  - [ ] Set default_priority = 7
  - [ ] Set cache_ttl_sec = 300

#### Testing (Commit 7)

- [ ] Create `tests/integration/scheduler/test_api_key_priority_crud.sh`
  - [ ] Test LIST (verify UUID in response)
  - [ ] Test CREATE (verify UUID generated)
  - [ ] Test UPDATE with UUID param
  - [ ] Test soft DELETE (verify deleted_at set)
  - [ ] Test hard delete (optional)
  - [ ] Test reload cache

- [ ] Create `tests/integration/scheduler/test_multi_tenant_scenario.sh`
  - [ ] Create 8 tenants (4 internal + 4 external)
  - [ ] Test production → P0 queue
  - [ ] Test dynamic priority change (ML team 1→0)
  - [ ] Test cache reload (1-2s propagation)
  - [ ] Verify UUID in all operations

- [ ] Create `tests/integration/scheduler/test_api_key_priority_disabled.sh`
  - [ ] Test Personal Edition (enabled=false)
  - [ ] Verify APIKeyMapper not initialized
  - [ ] Verify all requests use default priority
  - [ ] Verify management API returns 501

#### Documentation (Commit 8)

- [ ] Update all design docs with UUID examples
- [ ] Add DATABASE_STANDARDS.md reference
- [ ] Add migration guide
- [ ] Add troubleshooting section

---

## Commit Message Templates

### Commit 1: Database Schema

```
feat(phase1): add database schema with UUID and soft delete

- Create api_key_priority_mappings table (UUID primary key)
- Create api_key_priority_config table (UUID primary key)
- Add soft delete support (deleted_at field)
- Add standard audit fields (created_at, updated_at, deleted_at)
- Add multi-tenant metadata (tenant_id, tenant_name, tenant_type)
- Add partial indexes for active records (WHERE deleted_at IS NULL)
- Enable pgcrypto extension for gen_random_uuid()
```

### Commit 2: APIKeyMapper

```
feat(phase1): implement APIKeyMapper with PostgreSQL backend

- UUID-based operations (string ID, not int64)
- TTL-based cache (default 5min)
- Manual reload API support
- Pattern matching (exact, prefix, suffix, contains, regex)
- Multi-tenant metadata support
- Soft delete queries (WHERE deleted_at IS NULL)
- Soft delete in DeleteMapping (UPDATE deleted_at, not DELETE)
- RWMutex for concurrent access
```

### Commit 3: HTTP CRUD API

```
feat(phase1): add HTTP CRUD API for priority mappings

- GET /admin/api-key-priority/mappings (list, UUID in response)
- POST /admin/api-key-priority/mappings (create, UUID returned)
- PUT /admin/api-key-priority/mappings/:uuid (update with UUID param)
- DELETE /admin/api-key-priority/mappings/:uuid (soft delete, UUID param)
- POST /admin/api-key-priority/reload (reload cache)
- All endpoints accept/return UUID (not int)
- Include tenant metadata in responses
```

---

## 验收标准

### Phase 1 Acceptance Criteria

#### Database

- [ ] ✅ api_key_priority_mappings table uses UUID primary key
- [ ] ✅ api_key_priority_config table uses UUID primary key
- [ ] ✅ Both tables have created_at, updated_at, deleted_at
- [ ] ✅ Partial indexes created with WHERE deleted_at IS NULL
- [ ] ✅ Multi-tenant fields present (tenant_id, tenant_name, tenant_type)
- [ ] ✅ pgcrypto extension enabled
- [ ] ✅ gen_random_uuid() works

#### APIKeyMapper

- [ ] ✅ NewAPIKeyMapper returns UUID in models
- [ ] ✅ GetPriority() filters deleted_at IS NULL
- [ ] ✅ Reload() filters deleted_at IS NULL
- [ ] ✅ AddMapping() returns UUID (not int)
- [ ] ✅ UpdateMapping() accepts UUID param
- [ ] ✅ DeleteMapping() uses soft delete (UPDATE deleted_at = NOW())
- [ ] ✅ ListMappings() filters deleted_at IS NULL
- [ ] ✅ Cache TTL works correctly
- [ ] ✅ Manual reload works (1-2s propagation)

#### HTTP API

- [ ] ✅ List endpoint returns UUID in id field
- [ ] ✅ Create endpoint returns UUID
- [ ] ✅ Update endpoint accepts UUID in path
- [ ] ✅ Delete endpoint accepts UUID in path
- [ ] ✅ Delete endpoint performs soft delete (not hard delete)
- [ ] ✅ All responses include tenant metadata
- [ ] ✅ Reload endpoint works

#### Integration

- [ ] ✅ extractPriorityFromRequest() uses APIKeyMapper
- [ ] ✅ Production keys → P0 queue
- [ ] ✅ Dynamic priority change works with UUID
- [ ] ✅ Personal Edition disabled by default
- [ ] ✅ Team Edition can be enabled

#### Testing

- [ ] ✅ Unit tests pass (UUID operations)
- [ ] ✅ Integration tests pass (CRUD with UUID)
- [ ] ✅ Multi-tenant scenario test passes
- [ ] ✅ Personal Edition test passes (disabled)
- [ ] ✅ Performance test: cache query < 200ns
- [ ] ✅ Performance test: reload < 20ms (10 tenants)

#### Documentation

- [ ] ✅ DATABASE_STANDARDS.md exists and is referenced
- [ ] ✅ All Phase 1 docs updated with UUID
- [ ] ✅ Migration guide includes UUID
- [ ] ✅ Go model examples show UUID string
- [ ] ✅ SQL examples use gen_random_uuid()

---

## Next Steps

### 1. Start Implementation (Now)

```bash
# Verify current branch
git branch --show-current
# Output: feat/scheduler-phase1-api-key-mapping

# Start with database schema
# Create internal/userstore/postgres/migrations/002_api_key_priority.sql
```

### 2. Follow Commit Strategy

按照BRANCH_STRATEGY.md中的8个commit顺序实施：
1. Database schema
2. APIKeyMapper
3. HTTP CRUD API
4. HTTP integration
5. PostgreSQL integration
6. Main integration
7. Tests
8. Documentation

### 3. Create PR (After all commits)

```bash
# Push branch
git push -u origin feat/scheduler-phase1-api-key-mapping

# Create PR
gh pr create \
  --base feat/priority-scheduling \
  --title "feat(phase1): API Key to Priority Mapping with UUID and soft delete" \
  --body "See PHASE1_SUMMARY.md for details"
```

### 4. After Phase 1 Merged

```bash
# Create Phase 2 branch
git checkout feat/scheduler-phase1-api-key-mapping
git pull origin feat/scheduler-phase1-api-key-mapping
git checkout -b feat/scheduler-phase2-account-quota
```

---

## 总结

### ✅ Ready to Implement

所有准备工作完成：
- ✅ 设计文档完成（UUID + soft delete）
- ✅ Branch已创建 (`feat/scheduler-phase1-api-key-mapping`)
- ✅ 标准已定义 (`DATABASE_STANDARDS.md`)
- ✅ Branch策略已定义 (`BRANCH_STRATEGY.md`)
- ✅ Commit策略已定义
- ✅ 验收标准已定义

### ✅ Key Standards

1. **UUID Primary Keys**: All tables use UUID (not int)
2. **Soft Delete**: All tables have deleted_at field
3. **Audit Fields**: created_at, updated_at, deleted_at (standard)
4. **Partial Indexes**: WHERE deleted_at IS NULL
5. **Queries**: Must filter deleted_at IS NULL
6. **Go Models**: UUID as string, deleted_at as *time.Time

**可以开始Phase 1实施！** 🚀

按照DATABASE_STANDARDS.md和BRANCH_STRATEGY.md的指导，逐个commit完成Phase 1的实现。
