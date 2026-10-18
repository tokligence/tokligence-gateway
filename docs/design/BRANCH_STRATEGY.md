# Branch Strategy: Three-Phase Implementation

## Branch Structure

```
main (or develop)
  ↓
feat/priority-scheduling (base: already merged or will merge first)
  ↓
feat/scheduler-phase1-api-key-mapping
  ↓
feat/scheduler-phase2-account-quota
  ↓
feat/scheduler-phase3-time-rules
  ↓
main (final merge)
```

---

## Phase 1: API Key to Priority Mapping

### Branch Info

- **Branch Name**: `feat/scheduler-phase1-api-key-mapping`
- **Base Branch**: `feat/priority-scheduling` (or `main` if priority-scheduling已合并)
- **Estimated Time**: 6-8 hours

### Branch Creation

```bash
# Ensure on latest priority-scheduling branch
git checkout feat/priority-scheduling
git pull origin feat/priority-scheduling

# Create Phase 1 branch
git checkout -b feat/scheduler-phase1-api-key-mapping

# Verify current branch
git branch --show-current
# Output: feat/scheduler-phase1-api-key-mapping
```

### Deliverables

**New Files**:
```
internal/scheduler/
├── api_key_priority_store.go      # Database models with UUID
├── api_key_mapper.go               # APIKeyMapper (PostgreSQL, cache, soft delete)
└── api_key_mapper_test.go          # Unit tests

internal/httpserver/
└── endpoint_api_key_priority.go    # CRUD API (UUID-based)

internal/userstore/postgres/
└── migrations/
    └── 002_api_key_priority.sql    # Migration with UUID and soft delete

tests/integration/scheduler/
├── test_api_key_priority_crud.sh
├── test_api_key_priority_disabled.sh
└── test_multi_tenant_scenario.sh

docs/design/
├── DATABASE_STANDARDS.md           # UUID and soft delete standards
├── PHASE1_API_KEY_PRIORITY_MAPPING.md (updated with UUID)
├── PHASE1_POSTGRES_INTEGRATION.md (updated with UUID)
└── PHASE1_SUMMARY.md (updated with UUID)
```

**Modified Files**:
```
internal/httpserver/
├── scheduler_integration.go        # extractPriorityFromRequest() with APIKeyMapper
└── server.go                       # Add apiKeyMapper field, CRUD routes

internal/userstore/postgres/
└── postgres.go                     # Add api_key_priority_mappings to initSchema()

cmd/gatewayd/
└── main.go                        # Initialize APIKeyMapper (Team Edition)

internal/config/
└── config.go                      # Add APIKeyPriority* fields

config/
└── setting.ini                    # Add [api_key_priority] section
```

### Commit Strategy

```bash
# Commit 1: Database schema and models
git add internal/scheduler/api_key_priority_store.go
git add internal/userstore/postgres/migrations/002_api_key_priority.sql
git add docs/design/DATABASE_STANDARDS.md
git commit -m "feat(phase1): add database schema with UUID and soft delete

- Create api_key_priority_mappings table (UUID primary key)
- Create api_key_priority_config table
- Add soft delete support (deleted_at field)
- Add standard audit fields (created_at, updated_at)
- Create PriorityMappingModel with UUID
"

# Commit 2: APIKeyMapper implementation
git add internal/scheduler/api_key_mapper.go
git add internal/scheduler/api_key_mapper_test.go
git commit -m "feat(phase1): implement APIKeyMapper with PostgreSQL backend

- TTL-based cache (default 5min)
- Manual reload API support
- Pattern matching (exact, prefix, suffix, contains, regex)
- Multi-tenant metadata support
- Soft delete queries (deleted_at IS NULL)
- RWMutex for concurrent access
"

# Commit 3: HTTP CRUD API
git add internal/httpserver/endpoint_api_key_priority.go
git commit -m "feat(phase1): add HTTP CRUD API for priority mappings

- GET /admin/api-key-priority/mappings (list, UUID response)
- POST /admin/api-key-priority/mappings (create)
- PUT /admin/api-key-priority/mappings/:id (update, UUID param)
- DELETE /admin/api-key-priority/mappings/:id (soft delete)
- POST /admin/api-key-priority/reload (reload cache)
"

# Commit 4: HTTP integration
git add internal/httpserver/scheduler_integration.go
git add internal/httpserver/server.go
git commit -m "feat(phase1): integrate APIKeyMapper into HTTP handlers

- Update extractPriorityFromRequest() to use APIKeyMapper
- Add apiKeyMapper field to Server struct
- Register CRUD routes
- Add tenant logging
"

# Commit 5: PostgreSQL integration
git add internal/userstore/postgres/postgres.go
git commit -m "feat(phase1): integrate priority mappings into PostgreSQL schema

- Add api_key_priority_mappings table to initSchema()
- Add api_key_priority_config table
- Create partial indexes for soft delete queries
- Enable pgcrypto extension for gen_random_uuid()
"

# Commit 6: Main integration
git add cmd/gatewayd/main.go
git add internal/config/config.go
git add config/setting.ini
git commit -m "feat(phase1): integrate APIKeyMapper into main

- Initialize APIKeyMapper for Team Edition (PostgreSQL)
- Add configuration fields (APIKeyPriorityEnabled, etc.)
- Add [api_key_priority] section to setting.ini
- Personal Edition: disabled by default
"

# Commit 7: Tests
git add tests/integration/scheduler/test_api_key_priority*.sh
git commit -m "test(phase1): add integration tests for API key priority

- Test CRUD API with UUID
- Test multi-tenant scenario (8 tenants)
- Test Personal Edition (disabled)
- Test dynamic priority changes with cache reload
"

# Commit 8: Documentation
git add docs/design/PHASE1*.md
git add docs/design/DATABASE_STANDARDS.md
git commit -m "docs(phase1): update design docs with UUID and soft delete

- Add DATABASE_STANDARDS.md
- Update all Phase 1 docs with UUID
- Add soft delete patterns
- Add PostgreSQL integration guide
"
```

### PR Title and Description

```markdown
### PR Title
feat(phase1): API Key to Priority Mapping with UUID and soft delete

### Description

Implements Phase 1 of scheduler enhancements: automatic API key to priority mapping for multi-tenant scenarios.

**Key Features**:
- ✅ Database-backed (PostgreSQL, Team Edition)
- ✅ UUID primary keys (not int!)
- ✅ Soft delete support (deleted_at field)
- ✅ Multi-tenant metadata (tenant_id, tenant_name, tenant_type)
- ✅ Pattern matching (exact, prefix, suffix, contains, regex)
- ✅ TTL-based cache (default 5min) + Manual reload
- ✅ RESTful CRUD API (UUID-based)
- ✅ Personal Edition: disabled by default (zero impact)

**Database Schema**:
- `api_key_priority_mappings` (UUID, created_at, updated_at, deleted_at)
- `api_key_priority_config` (UUID, audit fields)

**HTTP API**:
- `GET /admin/api-key-priority/mappings`
- `POST /admin/api-key-priority/mappings`
- `PUT /admin/api-key-priority/mappings/:uuid`
- `DELETE /admin/api-key-priority/mappings/:uuid` (soft delete)
- `POST /admin/api-key-priority/reload`

**Performance**:
- Cache query: ~100ns per request
- Reload time: ~15ms (10 tenants)
- Impact: < 0.1% overhead

**Testing**:
- Unit tests: Pattern matching, cache reload
- Integration tests: CRUD, multi-tenant, dynamic changes
- Personal Edition test: Disabled mode

**Closes**: #XXX (if applicable)
```

---

## Phase 2: Per-Account Quota Management

### Branch Info

- **Branch Name**: `feat/scheduler-phase2-account-quota`
- **Base Branch**: `feat/scheduler-phase1-api-key-mapping`
- **Estimated Time**: 6-8 hours

### Branch Creation

```bash
# Ensure Phase 1 is merged or at least committed
git checkout feat/scheduler-phase1-api-key-mapping
git pull origin feat/scheduler-phase1-api-key-mapping

# Create Phase 2 branch
git checkout -b feat/scheduler-phase2-account-quota

git branch --show-current
# Output: feat/scheduler-phase2-account-quota
```

### Deliverables

**New Files**:
```
internal/scheduler/
├── account_quota.go                # AccountQuota with atomic operations (UUID)
├── account_quota_manager.go        # Manager with soft delete
└── account_quota_test.go           # Unit tests

internal/httpserver/
└── endpoint_account_quota.go       # CRUD API (UUID-based)

internal/userstore/postgres/
└── migrations/
    └── 003_account_quotas.sql      # Migration with UUID and soft delete

tests/integration/scheduler/
├── test_account_quota_crud.sh
├── test_quota_exhaustion.sh
└── test_orthogonal_priority.sh
```

**Modified Files**:
```
internal/scheduler/scheduler_channel.go  # Add quota check (orthogonal)
internal/httpserver/scheduler_integration.go
internal/httpserver/server.go
cmd/gatewayd/main.go
```

### Commit Strategy

```bash
# Similar to Phase 1, but 6-8 commits for:
# 1. Database schema (UUID, soft delete)
# 2. AccountQuota implementation (atomic operations)
# 3. AccountQuotaManager (soft delete support)
# 4. HTTP CRUD API
# 5. Scheduler integration (orthogonal check)
# 6. Main integration
# 7. Tests
# 8. Documentation
```

---

## Phase 3: Time-Based Dynamic Rules

### Branch Info

- **Branch Name**: `feat/scheduler-phase3-time-rules`
- **Base Branch**: `feat/scheduler-phase2-account-quota`
- **Estimated Time**: 10-12 hours

### Branch Creation

```bash
# Ensure Phase 2 is merged or committed
git checkout feat/scheduler-phase2-account-quota
git pull origin feat/scheduler-phase2-account-quota

# Create Phase 3 branch
git checkout -b feat/scheduler-phase3-time-rules

git branch --show-current
# Output: feat/scheduler-phase3-time-rules
```

### Deliverables

**New Files**:
```
internal/scheduler/
├── time_window.go                  # TimeWindow implementation
├── time_rules.go                   # RuleEngine with UUID
├── rule_engine.go                  # Core rule engine
└── time_rules_test.go              # Unit tests

internal/httpserver/
└── endpoint_time_rules.go          # CRUD API (UUID-based)

internal/userstore/postgres/
└── migrations/
    ├── 004_time_based_rules.sql    # UUID, soft delete, JSONB
    └── 005_rule_execution_history.sql

tests/integration/scheduler/
├── test_time_rules_crud.sh
├── test_weight_adjustment.sh
├── test_quota_adjustment.sh
└── test_day_night_transition.sh
```

**Modified Files**:
```
internal/scheduler/scheduler_channel.go        # AdjustWeights(), AdjustCapacity()
internal/scheduler/account_quota_manager.go    # UpdateQuota(), FindAccountsByPattern()
cmd/gatewayd/main.go
```

---

## Merge Strategy

### Step 1: Merge Phase 1 to base

```bash
# On feat/scheduler-phase1-api-key-mapping
git checkout feat/scheduler-phase1-api-key-mapping
git pull origin feat/scheduler-phase1-api-key-mapping

# Create PR to base branch (feat/priority-scheduling or main)
gh pr create \
  --base feat/priority-scheduling \
  --title "feat(phase1): API Key to Priority Mapping with UUID and soft delete" \
  --body "See commit messages and PHASE1_SUMMARY.md for details"

# After PR approved and merged, update Phase 2 base
git checkout feat/scheduler-phase2-account-quota
git rebase feat/priority-scheduling  # or main if Phase 1 merged there
```

### Step 2: Merge Phase 2 to base

```bash
# Similar process for Phase 2
git checkout feat/scheduler-phase2-account-quota
gh pr create \
  --base feat/priority-scheduling \
  --title "feat(phase2): Per-Account Quota Management with UUID" \
  --body "..."
```

### Step 3: Merge Phase 3 to main

```bash
# Final merge of Phase 3
git checkout feat/scheduler-phase3-time-rules
gh pr create \
  --base main \
  --title "feat(phase3): Time-Based Dynamic Rules with UUID" \
  --body "..."
```

---

## Review Checklist (Per Phase)

### Code Review

- [ ] ✅ UUID primary keys (not int/serial)
- [ ] ✅ Soft delete implemented (deleted_at field)
- [ ] ✅ Standard audit fields (created_at, updated_at, deleted_at)
- [ ] ✅ Partial indexes for active records (WHERE deleted_at IS NULL)
- [ ] ✅ Soft delete in all queries (WHERE deleted_at IS NULL)
- [ ] ✅ Update operations set updated_at = NOW()
- [ ] ✅ Delete operations use UPDATE (not DELETE)
- [ ] ✅ Go models use string for UUID (not int64)
- [ ] ✅ Go models use *time.Time for deleted_at (NULL support)

### Testing

- [ ] ✅ Unit tests cover UUID operations
- [ ] ✅ Unit tests cover soft delete
- [ ] ✅ Integration tests with UUID params
- [ ] ✅ Integration tests for soft delete/restore
- [ ] ✅ Personal Edition test (disabled mode)
- [ ] ✅ Team Edition test (enabled mode)

### Documentation

- [ ] ✅ Design docs updated with UUID
- [ ] ✅ Database schema shows UUID and soft delete
- [ ] ✅ API docs show UUID in requests/responses
- [ ] ✅ Migration scripts use gen_random_uuid()
- [ ] ✅ DATABASE_STANDARDS.md referenced

### Performance

- [ ] ✅ No performance regression
- [ ] ✅ Indexes include partial index for soft delete
- [ ] ✅ UUID generation uses gen_random_uuid() (PostgreSQL native)

---

## Git Commands Cheat Sheet

### Create Branch

```bash
git checkout -b feat/scheduler-phase1-api-key-mapping
```

### Check Current Branch

```bash
git branch --show-current
```

### Push Branch

```bash
git push -u origin feat/scheduler-phase1-api-key-mapping
```

### Update from Base Branch

```bash
git checkout feat/scheduler-phase1-api-key-mapping
git fetch origin
git rebase origin/feat/priority-scheduling
```

### Create PR

```bash
gh pr create \
  --base feat/priority-scheduling \
  --title "feat(phase1): API Key to Priority Mapping" \
  --body "Implements Phase 1 with UUID and soft delete"
```

### Check PR Status

```bash
gh pr status
gh pr view
```

### Merge PR (after approval)

```bash
gh pr merge --squash  # or --merge or --rebase
```

---

## Summary

### ✅ Branch Structure

```
main
  ↓
feat/priority-scheduling
  ↓
feat/scheduler-phase1-api-key-mapping  (6-8h, UUID, soft delete)
  ↓
feat/scheduler-phase2-account-quota    (6-8h, UUID, soft delete)
  ↓
feat/scheduler-phase3-time-rules       (10-12h, UUID, soft delete, JSONB)
  ↓
main (final merge)
```

### ✅ Key Standards (All Phases)

1. **UUID Primary Keys**: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
2. **Soft Delete**: `deleted_at TIMESTAMPTZ` (NULL = active)
3. **Audit Fields**: `created_at`, `updated_at`, `deleted_at`
4. **Partial Indexes**: `WHERE deleted_at IS NULL`
5. **Go Models**: UUID as string, deleted_at as *time.Time

### ✅ Ready to Start

所有设计文档已更新：
- DATABASE_STANDARDS.md ✅
- BRANCH_STRATEGY.md ✅ (this file)
- Phase 1-3 design docs updated with UUID ✅

**可以开始实施Phase 1！** 🚀
