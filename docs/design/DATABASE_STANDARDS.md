# Database Standards for All Phases

## 标准字段规范

**所有表必须包含以下字段**:

```sql
-- Primary Key: UUID (NOT int/serial!)
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

-- Audit fields (所有表必备)
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
deleted_at TIMESTAMPTZ,  -- Soft delete (NULL = active, NOT NULL = deleted)

-- Optional: Track who created/modified
created_by TEXT,
updated_by TEXT
```

---

## Phase 1: API Key Priority Mapping

### Table: api_key_priority_mappings

```sql
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Pattern matching
    pattern TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),

    -- Multi-tenant metadata
    tenant_id TEXT,
    tenant_name TEXT,
    tenant_type TEXT CHECK(tenant_type IN ('internal', 'external')),
    description TEXT,

    -- Status
    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete
    created_by TEXT,
    updated_by TEXT
);

-- Indexes
CREATE INDEX idx_api_key_priority_mappings_pattern ON api_key_priority_mappings(pattern) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_enabled ON api_key_priority_mappings(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_tenant_id ON api_key_priority_mappings(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_priority ON api_key_priority_mappings(priority) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_deleted_at ON api_key_priority_mappings(deleted_at);
```

### Table: api_key_priority_config

```sql
CREATE TABLE IF NOT EXISTS api_key_priority_config (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Config key-value
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by TEXT,
    updated_by TEXT
);

CREATE INDEX idx_api_key_priority_config_key ON api_key_priority_config(key) WHERE deleted_at IS NULL;
```

---

## Phase 2: Per-Account Quota Management

### Table: account_quotas

```sql
CREATE TABLE IF NOT EXISTS account_quotas (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Account identification
    account_id TEXT NOT NULL UNIQUE,
    account_name TEXT,
    account_type TEXT CHECK(account_type IN ('internal', 'external', 'trial')),

    -- Quota limits
    max_concurrent INTEGER NOT NULL DEFAULT 10 CHECK(max_concurrent > 0),
    max_rps INTEGER NOT NULL DEFAULT 10 CHECK(max_rps > 0),
    max_tokens_per_sec INTEGER NOT NULL DEFAULT 1000 CHECK(max_tokens_per_sec > 0),
    max_requests_per_day INTEGER NOT NULL DEFAULT 10000 CHECK(max_requests_per_day > 0),

    -- Metadata
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete
    created_by TEXT,
    updated_by TEXT
);

-- Indexes
CREATE INDEX idx_account_quotas_account_id ON account_quotas(account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_account_quotas_enabled ON account_quotas(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_account_quotas_account_type ON account_quotas(account_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_account_quotas_deleted_at ON account_quotas(deleted_at);
```

### Table: account_quota_usage (实时使用情况)

```sql
CREATE TABLE IF NOT EXISTS account_quota_usage (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Foreign key to account_quotas (UUID!)
    account_quota_id UUID NOT NULL REFERENCES account_quotas(id) ON DELETE CASCADE,
    account_id TEXT NOT NULL,

    -- Current usage (updated in real-time)
    current_concurrent INTEGER NOT NULL DEFAULT 0,
    current_rps INTEGER NOT NULL DEFAULT 0,
    current_tokens_per_sec INTEGER NOT NULL DEFAULT 0,
    requests_today INTEGER NOT NULL DEFAULT 0,

    -- Window tracking
    window_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_request_at TIMESTAMPTZ,

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_account_quota_usage_account_id ON account_quota_usage(account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_account_quota_usage_account_quota_id ON account_quota_usage(account_quota_id) WHERE deleted_at IS NULL;
```

---

## Phase 3: Time-Based Dynamic Rules

### Table: time_based_rules

```sql
CREATE TABLE IF NOT EXISTS time_based_rules (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Rule identification
    rule_name TEXT NOT NULL UNIQUE,
    rule_type TEXT NOT NULL CHECK(rule_type IN ('weight_adjustment', 'quota_adjustment', 'capacity_adjustment')),

    -- Time window
    start_hour INTEGER NOT NULL CHECK(start_hour >= 0 AND start_hour <= 23),
    start_minute INTEGER NOT NULL DEFAULT 0 CHECK(start_minute >= 0 AND start_minute <= 59),
    end_hour INTEGER NOT NULL CHECK(end_hour >= 0 AND end_hour <= 23),
    end_minute INTEGER NOT NULL DEFAULT 0 CHECK(end_minute >= 0 AND end_minute <= 59),
    days_of_week TEXT,  -- JSON array: [1,2,3,4,5] for Mon-Fri
    timezone TEXT NOT NULL DEFAULT 'UTC',

    -- Rule parameters (JSON)
    parameters JSONB NOT NULL,  -- Rule-specific parameters

    -- Metadata
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INTEGER NOT NULL DEFAULT 0,  -- For rule conflict resolution

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete
    created_by TEXT,
    updated_by TEXT
);

-- Indexes
CREATE INDEX idx_time_based_rules_rule_name ON time_based_rules(rule_name) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_based_rules_enabled ON time_based_rules(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_based_rules_rule_type ON time_based_rules(rule_type) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_based_rules_priority ON time_based_rules(priority) WHERE deleted_at IS NULL;
CREATE INDEX idx_time_based_rules_deleted_at ON time_based_rules(deleted_at);
```

### Table: rule_execution_history (Audit log)

```sql
CREATE TABLE IF NOT EXISTS rule_execution_history (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Foreign key to time_based_rules (UUID!)
    rule_id UUID NOT NULL REFERENCES time_based_rules(id) ON DELETE CASCADE,
    rule_name TEXT NOT NULL,

    -- Execution details
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    execution_result TEXT NOT NULL CHECK(execution_result IN ('success', 'failed', 'skipped')),
    error_message TEXT,

    -- Changes applied (JSON)
    changes_applied JSONB,

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_rule_execution_history_rule_id ON rule_execution_history(rule_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_rule_execution_history_executed_at ON rule_execution_history(executed_at);
```

---

## Soft Delete Pattern

### 查询时必须过滤deleted_at

```sql
-- ✅ 正确：只查询未删除的记录
SELECT * FROM api_key_priority_mappings
WHERE deleted_at IS NULL
  AND enabled = TRUE;

-- ❌ 错误：未过滤deleted_at
SELECT * FROM api_key_priority_mappings
WHERE enabled = TRUE;
```

### 删除操作使用UPDATE（软删除）

```sql
-- Soft delete (推荐)
UPDATE api_key_priority_mappings
SET deleted_at = NOW(), updated_at = NOW(), updated_by = 'admin'
WHERE id = 'uuid-here';

-- Hard delete (仅在必要时使用，例如GDPR compliance)
DELETE FROM api_key_priority_mappings
WHERE id = 'uuid-here';
```

### 恢复已删除记录

```sql
-- Restore soft-deleted record
UPDATE api_key_priority_mappings
SET deleted_at = NULL, updated_at = NOW(), updated_by = 'admin'
WHERE id = 'uuid-here';
```

---

## Go Data Models

### Phase 1: PriorityMappingModel

```go
type PriorityMappingModel struct {
    // UUID primary key
    ID          string    `json:"id" db:"id"`  // UUID as string

    // Pattern matching
    Pattern     string    `json:"pattern" db:"pattern"`
    Priority    int       `json:"priority" db:"priority"`
    MatchType   string    `json:"match_type" db:"match_type"`

    // Multi-tenant
    TenantID    string    `json:"tenant_id" db:"tenant_id"`
    TenantName  string    `json:"tenant_name" db:"tenant_name"`
    TenantType  string    `json:"tenant_type" db:"tenant_type"`
    Description string    `json:"description" db:"description"`

    // Status
    Enabled     bool      `json:"enabled" db:"enabled"`

    // Standard audit fields
    CreatedAt   time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`  // Pointer for NULL
    CreatedBy   string     `json:"created_by" db:"created_by"`
    UpdatedBy   string     `json:"updated_by" db:"updated_by"`
}
```

### Phase 2: AccountQuotaModel

```go
type AccountQuotaModel struct {
    // UUID primary key
    ID          string    `json:"id" db:"id"`

    // Account identification
    AccountID   string    `json:"account_id" db:"account_id"`
    AccountName string    `json:"account_name" db:"account_name"`
    AccountType string    `json:"account_type" db:"account_type"`

    // Quota limits
    MaxConcurrent     int    `json:"max_concurrent" db:"max_concurrent"`
    MaxRPS            int    `json:"max_rps" db:"max_rps"`
    MaxTokensPerSec   int    `json:"max_tokens_per_sec" db:"max_tokens_per_sec"`
    MaxRequestsPerDay int    `json:"max_requests_per_day" db:"max_requests_per_day"`

    // Metadata
    Description string    `json:"description" db:"description"`
    Enabled     bool      `json:"enabled" db:"enabled"`

    // Standard audit fields
    CreatedAt   time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
    CreatedBy   string     `json:"created_by" db:"created_by"`
    UpdatedBy   string     `json:"updated_by" db:"updated_by"`
}
```

### Phase 3: TimeBasedRuleModel

```go
type TimeBasedRuleModel struct {
    // UUID primary key
    ID          string    `json:"id" db:"id"`

    // Rule identification
    RuleName    string    `json:"rule_name" db:"rule_name"`
    RuleType    string    `json:"rule_type" db:"rule_type"`

    // Time window
    StartHour   int       `json:"start_hour" db:"start_hour"`
    StartMinute int       `json:"start_minute" db:"start_minute"`
    EndHour     int       `json:"end_hour" db:"end_hour"`
    EndMinute   int       `json:"end_minute" db:"end_minute"`
    DaysOfWeek  string    `json:"days_of_week" db:"days_of_week"`  // JSON array
    Timezone    string    `json:"timezone" db:"timezone"`

    // Rule parameters
    Parameters  string    `json:"parameters" db:"parameters"`  // JSON

    // Metadata
    Description string    `json:"description" db:"description"`
    Enabled     bool      `json:"enabled" db:"enabled"`
    Priority    int       `json:"priority" db:"priority"`

    // Standard audit fields
    CreatedAt   time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
    CreatedBy   string     `json:"created_by" db:"created_by"`
    UpdatedBy   string     `json:"updated_by" db:"updated_by"`
}
```

---

## CRUD Operations标准模式

### Create

```go
func (m *Manager) Create(ctx context.Context, data *Model) (*Model, error) {
    query := `
        INSERT INTO table_name (pattern, priority, ..., created_by, updated_by)
        VALUES ($1, $2, ..., $N, $N)
        RETURNING id, created_at, updated_at
    `
    var id string
    var createdAt, updatedAt time.Time

    err := m.db.QueryRowContext(ctx, query, data.Pattern, data.Priority, ..., data.CreatedBy, data.CreatedBy).
        Scan(&id, &createdAt, &updatedAt)
    if err != nil {
        return nil, fmt.Errorf("failed to create: %w", err)
    }

    data.ID = id
    data.CreatedAt = createdAt
    data.UpdatedAt = updatedAt
    return data, nil
}
```

### Read (with soft delete filter)

```go
func (m *Manager) GetByID(ctx context.Context, id string) (*Model, error) {
    query := `
        SELECT id, pattern, priority, ..., created_at, updated_at, deleted_at
        FROM table_name
        WHERE id = $1 AND deleted_at IS NULL
    `
    var model Model
    err := m.db.QueryRowContext(ctx, query, id).Scan(&model.ID, &model.Pattern, ...)
    if err == sql.ErrNoRows {
        return nil, nil  // Not found
    }
    if err != nil {
        return nil, fmt.Errorf("failed to get by id: %w", err)
    }
    return &model, nil
}

func (m *Manager) List(ctx context.Context) ([]*Model, error) {
    query := `
        SELECT id, pattern, priority, ..., created_at, updated_at, deleted_at
        FROM table_name
        WHERE deleted_at IS NULL
        ORDER BY created_at DESC
    `
    rows, err := m.db.QueryContext(ctx, query)
    // ... scan rows
}
```

### Update

```go
func (m *Manager) Update(ctx context.Context, id string, updates *Model, updatedBy string) (*Model, error) {
    query := `
        UPDATE table_name
        SET priority = $1, description = $2, updated_at = NOW(), updated_by = $3
        WHERE id = $4 AND deleted_at IS NULL
        RETURNING updated_at
    `
    var updatedAt time.Time
    err := m.db.QueryRowContext(ctx, query, updates.Priority, updates.Description, updatedBy, id).
        Scan(&updatedAt)
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("record not found or already deleted")
    }
    if err != nil {
        return nil, fmt.Errorf("failed to update: %w", err)
    }

    updates.UpdatedAt = updatedAt
    return updates, nil
}
```

### Delete (Soft Delete)

```go
func (m *Manager) Delete(ctx context.Context, id string, deletedBy string) error {
    query := `
        UPDATE table_name
        SET deleted_at = NOW(), updated_at = NOW(), updated_by = $1
        WHERE id = $2 AND deleted_at IS NULL
    `
    result, err := m.db.ExecContext(ctx, query, deletedBy, id)
    if err != nil {
        return fmt.Errorf("failed to soft delete: %w", err)
    }

    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("record not found or already deleted")
    }
    return nil
}
```

### Hard Delete (仅在必要时)

```go
func (m *Manager) HardDelete(ctx context.Context, id string) error {
    query := `DELETE FROM table_name WHERE id = $1`
    result, err := m.db.ExecContext(ctx, query, id)
    if err != nil {
        return fmt.Errorf("failed to hard delete: %w", err)
    }

    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("record not found")
    }
    return nil
}
```

---

## Migration Scripts

### Phase 1 Migration

```sql
-- File: internal/userstore/postgres/migrations/002_api_key_priority_mappings.sql

BEGIN;

-- Create extension for UUID generation (if not exists)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create api_key_priority_mappings table
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pattern TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),
    tenant_id TEXT,
    tenant_name TEXT,
    tenant_type TEXT CHECK(tenant_type IN ('internal', 'external')),
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by TEXT,
    updated_by TEXT
);

-- Indexes (with partial index for active records)
CREATE INDEX idx_api_key_priority_mappings_pattern ON api_key_priority_mappings(pattern) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_enabled ON api_key_priority_mappings(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_tenant_id ON api_key_priority_mappings(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_priority ON api_key_priority_mappings(priority) WHERE deleted_at IS NULL;
CREATE INDEX idx_api_key_priority_mappings_deleted_at ON api_key_priority_mappings(deleted_at);

-- Create config table
CREATE TABLE IF NOT EXISTS api_key_priority_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by TEXT,
    updated_by TEXT
);

CREATE INDEX idx_api_key_priority_config_key ON api_key_priority_config(key) WHERE deleted_at IS NULL;

-- Insert default config
INSERT INTO api_key_priority_config (key, value, description, created_by) VALUES
('enabled', 'false', 'Enable/disable API key priority mapping', 'system'),
('default_priority', '7', 'Default priority for unmapped keys', 'system'),
('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds', 'system')
ON CONFLICT (key) DO NOTHING;

COMMIT;
```

---

## 总结

### ✅ 所有表的标准要求

1. **UUID Primary Key**: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
2. **Audit Fields**: `created_at`, `updated_at`, `deleted_at`
3. **Soft Delete**: `deleted_at TIMESTAMPTZ` (NULL = active)
4. **Partial Indexes**: `WHERE deleted_at IS NULL` for active records
5. **Tracking Fields**: `created_by`, `updated_by` (可选)

### ✅ CRUD操作标准

1. **Create**: 返回UUID id和timestamps
2. **Read**: 必须过滤`deleted_at IS NULL`
3. **Update**: 自动更新`updated_at = NOW()`
4. **Delete**: 使用soft delete (UPDATE deleted_at)

### ✅ PostgreSQL Requirements

- Enable `pgcrypto` extension for `gen_random_uuid()`
- Use `TIMESTAMPTZ` (not TIMESTAMP)
- Use partial indexes for soft delete queries
- Use CHECK constraints for validation

这套标准将应用于所有三个Phase的表设计。
