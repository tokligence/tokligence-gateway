# Phase 1: API Key to Priority Mapping Design

**Branch**: `feat/scheduler-phase1-api-key-mapping`
**Base**: `feat/priority-scheduling`
**估算工作量**: 6-8小时 (increased due to database implementation)
**依赖**: Priority Queue Scheduler (已完成)

## 目标

**核心需求**: 根据API Key自动映射到优先级，无需客户端手动设置`X-Priority`头。

**使用场景**:
- 公司内部有多个部门，每个部门有独立的API key
- 不同tier的外部客户（Premium/Standard/Free）有不同API key
- Gateway管理员通过Web UI/API配置API key → priority映射关系
- 客户端只需提供`Authorization: Bearer <api_key>`，无需关心优先级

**版本差异**:
- **Personal Edition**: 该功能默认**禁用**（通过配置关闭）
- **Team Edition**: 该功能可选**启用**（企业多租户场景）

## 架构设计

### 1. 数据存储层 (Database Layer) - 类似LiteLLM

#### 1.1 数据库表结构

**表名**: `api_key_priority_mappings`

```sql
CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    -- UUID primary key (NOT int!)
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- API Key pattern
    pattern TEXT NOT NULL UNIQUE,  -- e.g., "tok_dept_prod*", "tok_ext_premium*"

    -- Target priority (0-9)
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),

    -- Pattern match type
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),

    -- Multi-tenant metadata (类似LiteLLM)
    tenant_id TEXT,        -- Tenant identifier (e.g., "dept-prod", "ext-enterprise")
    tenant_name TEXT,      -- Human-readable name (e.g., "Production Team")
    tenant_type TEXT,      -- "internal" or "external"
    description TEXT,      -- Additional description

    -- Status
    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Audit fields (标准字段，所有表都需要)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,            -- Soft delete (NULL = not deleted)
    created_by TEXT,                   -- Admin user who created this mapping

    -- Indexes for fast lookup
    INDEX idx_api_key_priority_mappings_pattern (pattern),
    INDEX idx_api_key_priority_mappings_enabled (enabled),
    INDEX idx_api_key_priority_mappings_tenant_id (tenant_id),
    INDEX idx_api_key_priority_mappings_priority (priority),
    INDEX idx_api_key_priority_mappings_deleted_at (deleted_at)  -- For soft delete queries
);
```

**示例数据（多租户场景）**:
```sql
-- Internal departments (P0-P3) - Production in P0 queue
INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description) VALUES
('sk-dept-prod-*', 0, 'prefix', 'dept-prod', 'Production Team', 'internal', 'Production workloads - Highest priority (P0 queue)'),
('sk-dept-ml-*', 1, 'prefix', 'dept-ml', 'ML Research Team', 'internal', 'ML research and training (P1 queue)'),
('sk-dept-analytics-*', 2, 'prefix', 'dept-analytics', 'Analytics Team', 'internal', 'Business analytics (P2 queue)'),
('sk-dept-dev-*', 3, 'prefix', 'dept-dev', 'Development Team', 'internal', 'Development and testing (P3 queue)');

-- External customers (P5-P9)
INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description) VALUES
('sk-ext-enterprise-*', 5, 'prefix', 'ext-enterprise', 'Enterprise Customers', 'external', 'Enterprise tier (P5 queue)'),
('sk-ext-premium-*', 6, 'prefix', 'ext-premium', 'Premium Customers', 'external', 'Premium tier (P6 queue)'),
('sk-ext-standard-*', 7, 'prefix', 'ext-standard', 'Standard Customers', 'external', 'Standard tier (P7 queue)'),
('sk-ext-free-*', 9, 'prefix', 'ext-free', 'Free Tier Users', 'external', 'Free tier (P9 queue)');
```

#### 1.2 配置表 (全局设置)

**表名**: `api_key_priority_config`

```sql
CREATE TABLE IF NOT EXISTS api_key_priority_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO api_key_priority_config (key, value, description) VALUES
('enabled', 'false', 'Enable/disable API key priority mapping (false for Personal Edition)'),
('default_priority', '7', 'Default priority for unmapped keys'),
('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds');
```

**重要**: `enabled` 字段控制整个功能开关
- Personal Edition: `enabled = false` (默认)
- Team Edition: `enabled = true` (可选启用)

### 2. 配置文件 (简化为开关)

**位置**: `config/setting.ini` (全局配置)

```ini
[api_key_priority]
# Enable/disable API key to priority mapping feature
# Personal Edition: Set to false (default)
# Team Edition: Set to true (for multi-tenant scenarios)
enabled = false

# Default priority for unmapped keys (0-9)
default_priority = 7

# Database path for mappings (SQLite)
# Uses identity store database by default
db_path = ~/.tokligence/identity.db

# Cache TTL in seconds (reduce database load)
cache_ttl_sec = 300
```

**重要配置说明**:
- `enabled = false`: Personal Edition默认关闭，不需要多租户管理
- `enabled = true`: Team Edition可启用，支持企业场景

#### 2.2 环境变量覆盖

```bash
# Enable/disable API key mapping (CRITICAL for Personal vs Team)
export TOKLIGENCE_API_KEY_PRIORITY_ENABLED=false  # Personal Edition
export TOKLIGENCE_API_KEY_PRIORITY_ENABLED=true   # Team Edition

# Default priority for unmapped keys
export TOKLIGENCE_API_KEY_PRIORITY_DEFAULT=7

# Custom database path
export TOKLIGENCE_API_KEY_PRIORITY_DB_PATH=/custom/path/identity.db

# Cache TTL (seconds)
export TOKLIGENCE_API_KEY_PRIORITY_CACHE_TTL=300
```

### 3. 数据结构 (Data Structures)

#### 3.1 Database Models

```go
// internal/scheduler/api_key_priority_store.go

package scheduler

import (
    "database/sql"
    "fmt"
    "regexp"
    "strings"
    "sync"
    "time"
)

// PriorityMappingModel represents a database row
type PriorityMappingModel struct {
    ID          int64
    Pattern     string
    Priority    int
    MatchType   string

    // Multi-tenant fields
    TenantID    string    // Tenant identifier (e.g., "dept-prod", "ext-enterprise")
    TenantName  string    // Human-readable name
    TenantType  string    // "internal" or "external"
    Description string

    Enabled     bool
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CreatedBy   string
}

// PriorityMapping represents a compiled mapping rule (in-memory cache)
type PriorityMapping struct {
    ID           int64
    Pattern      string        // Original pattern (e.g., "sk-dept-prod-*")
    Priority     PriorityTier  // Target priority (0-9, mapped to queue)
    MatchType    MatchType     // Exact, Prefix, Suffix, Contains, Regex

    // Multi-tenant metadata
    TenantID     string        // Tenant identifier
    TenantName   string        // Human-readable tenant name
    TenantType   string        // "internal" or "external"
    Description  string

    Enabled      bool
    matchFunc    func(string) bool // Compiled match function (for fast lookup)
}

type MatchType int

const (
    MatchExact MatchType = iota
    MatchPrefix
    MatchSuffix
    MatchContains
    MatchRegex
)

func (mt MatchType) String() string {
    switch mt {
    case MatchExact:
        return "exact"
    case MatchPrefix:
        return "prefix"
    case MatchSuffix:
        return "suffix"
    case MatchContains:
        return "contains"
    case MatchRegex:
        return "regex"
    default:
        return "unknown"
    }
}

func ParseMatchType(s string) MatchType {
    switch s {
    case "exact":
        return MatchExact
    case "prefix":
        return MatchPrefix
    case "suffix":
        return MatchSuffix
    case "contains":
        return MatchContains
    case "regex":
        return MatchRegex
    default:
        return MatchExact
    }
}
```

#### 3.2 API Key Mapper (with Database Backend)

```go
// internal/scheduler/api_key_mapper.go

package scheduler

import (
    "database/sql"
    "fmt"
    "log"
    "sync"
    "time"
)

// APIKeyMapper handles API key to priority mapping with database backend
type APIKeyMapper struct {
    db              *sql.DB
    mappings        []*PriorityMapping  // Cached mappings
    defaultPriority PriorityTier
    enabled         bool
    cacheTTL        time.Duration
    lastReload      time.Time
    mu              sync.RWMutex
}

// NewAPIKeyMapper creates a new API key mapper with database backend
func NewAPIKeyMapper(db *sql.DB, defaultPriority PriorityTier, enabled bool, cacheTTL time.Duration) (*APIKeyMapper, error) {
    mapper := &APIKeyMapper{
        db:              db,
        mappings:        make([]*PriorityMapping, 0),
        defaultPriority: defaultPriority,
        enabled:         enabled,
        cacheTTL:        cacheTTL,
    }

    // Create tables if not exist
    if err := mapper.initializeTables(); err != nil {
        return nil, fmt.Errorf("failed to initialize tables: %w", err)
    }

    // Load mappings from database
    if enabled {
        if err := mapper.Reload(); err != nil {
            return nil, fmt.Errorf("failed to load mappings: %w", err)
        }
    }

    return mapper, nil
}

// initializeTables creates required database tables
func (m *APIKeyMapper) initializeTables() error {
    createMappingsTable := `
    CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        pattern TEXT NOT NULL UNIQUE,
        priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
        match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),

        -- Multi-tenant metadata
        tenant_id TEXT,
        tenant_name TEXT,
        tenant_type TEXT,
        description TEXT,

        enabled BOOLEAN NOT NULL DEFAULT 1,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        created_by TEXT
    );
    CREATE INDEX IF NOT EXISTS idx_pattern ON api_key_priority_mappings(pattern);
    CREATE INDEX IF NOT EXISTS idx_enabled ON api_key_priority_mappings(enabled);
    CREATE INDEX IF NOT EXISTS idx_tenant_id ON api_key_priority_mappings(tenant_id);
    CREATE INDEX IF NOT EXISTS idx_priority ON api_key_priority_mappings(priority);
    `

    createConfigTable := `
    CREATE TABLE IF NOT EXISTS api_key_priority_config (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        description TEXT,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    INSERT OR IGNORE INTO api_key_priority_config (key, value, description) VALUES
    ('enabled', 'false', 'Enable/disable API key priority mapping'),
    ('default_priority', '7', 'Default priority for unmapped keys'),
    ('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds');
    `

    if _, err := m.db.Exec(createMappingsTable); err != nil {
        return fmt.Errorf("failed to create mappings table: %w", err)
    }

    if _, err := m.db.Exec(createConfigTable); err != nil {
        return fmt.Errorf("failed to create config table: %w", err)
    }

    return nil
}

// GetPriority returns the priority for a given API key
func (m *APIKeyMapper) GetPriority(apiKey string) PriorityTier {
    if !m.enabled {
        return m.defaultPriority
    }

    // Check if cache needs reload
    if time.Since(m.lastReload) > m.cacheTTL {
        if err := m.Reload(); err != nil {
            log.Printf("[WARN] APIKeyMapper: Failed to reload cache: %v", err)
        }
    }

    m.mu.RLock()
    defer m.mu.RUnlock()

    // Try each mapping in order (first match wins)
    for _, mapping := range m.mappings {
        if !mapping.Enabled {
            continue
        }
        if mapping.matchFunc(apiKey) {
            return mapping.Priority
        }
    }

    // No match, return default
    return m.defaultPriority
}

// Reload reloads mappings from database (with tenant fields)
func (m *APIKeyMapper) Reload() error {
    rows, err := m.db.Query(`
        SELECT id, pattern, priority, match_type,
               tenant_id, tenant_name, tenant_type, description, enabled
        FROM api_key_priority_mappings
        WHERE enabled = 1
        ORDER BY priority ASC, id ASC  -- P0 first (production), then by creation order
    `)
    if err != nil {
        return fmt.Errorf("failed to query mappings: %w", err)
    }
    defer rows.Close()

    var newMappings []*PriorityMapping

    for rows.Next() {
        var model PriorityMappingModel
        if err := rows.Scan(&model.ID, &model.Pattern, &model.Priority, &model.MatchType,
            &model.TenantID, &model.TenantName, &model.TenantType, &model.Description, &model.Enabled); err != nil {
            log.Printf("[WARN] APIKeyMapper: Failed to scan row: %v", err)
            continue
        }

        mapping := &PriorityMapping{
            ID:          model.ID,
            Pattern:     model.Pattern,
            Priority:    PriorityTier(model.Priority),
            MatchType:   ParseMatchType(model.MatchType),
            TenantID:    model.TenantID,
            TenantName:  model.TenantName,
            TenantType:  model.TenantType,
            Description: model.Description,
            Enabled:     model.Enabled,
        }

        // Compile pattern into match function (one-time cost during reload)
        if err := mapping.compile(); err != nil {
            log.Printf("[WARN] APIKeyMapper: Failed to compile pattern %q: %v", model.Pattern, err)
            continue
        }

        newMappings = append(newMappings, mapping)
    }

    // Atomic cache swap (mutex-protected)
    m.mu.Lock()
    m.mappings = newMappings
    m.lastReload = time.Now()
    m.mu.Unlock()

    log.Printf("[INFO] APIKeyMapper: Reloaded %d mappings from database (tenants=%d internal, %d external)",
        len(newMappings),
        countTenantsByType(newMappings, "internal"),
        countTenantsByType(newMappings, "external"))
    return nil
}

// countTenantsByType counts mappings by tenant type
func countTenantsByType(mappings []*PriorityMapping, tenantType string) int {
    count := 0
    for _, m := range mappings {
        if m.TenantType == tenantType {
            count++
        }
    }
    return count
}

// AddMapping adds a new pattern-to-priority mapping to database (with tenant fields)
func (m *APIKeyMapper) AddMapping(pattern string, priority PriorityTier, matchType MatchType,
    tenantID, tenantName, tenantType, description, createdBy string) error {

    _, err := m.db.Exec(`
        INSERT INTO api_key_priority_mappings
        (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, pattern, int(priority), matchType.String(), tenantID, tenantName, tenantType, description, createdBy)

    if err != nil {
        return fmt.Errorf("failed to insert mapping: %w", err)
    }

    log.Printf("[INFO] APIKeyMapper: Added mapping for tenant '%s' (%s): pattern=%s priority=P%d",
        tenantID, tenantType, pattern, priority)

    // Reload cache to pick up new mapping
    return m.Reload()
}

// UpdateMapping updates an existing mapping
func (m *APIKeyMapper) UpdateMapping(id int64, priority PriorityTier, description string, enabled bool) error {
    _, err := m.db.Exec(`
        UPDATE api_key_priority_mappings
        SET priority = ?, description = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
        WHERE id = ?
    `, int(priority), description, enabled, id)

    if err != nil {
        return fmt.Errorf("failed to update mapping: %w", err)
    }

    // Reload cache
    return m.Reload()
}

// DeleteMapping deletes a mapping from database
func (m *APIKeyMapper) DeleteMapping(id int64) error {
    _, err := m.db.Exec(`DELETE FROM api_key_priority_mappings WHERE id = ?`, id)
    if err != nil {
        return fmt.Errorf("failed to delete mapping: %w", err)
    }

    // Reload cache
    return m.Reload()
}

// ListMappings returns all mappings (for admin UI)
func (m *APIKeyMapper) ListMappings() ([]*PriorityMappingModel, error) {
    rows, err := m.db.Query(`
        SELECT id, pattern, priority, match_type, description, account_name, enabled, created_at, updated_at, created_by
        FROM api_key_priority_mappings
        ORDER BY priority ASC, id ASC
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to query mappings: %w", err)
    }
    defer rows.Close()

    var mappings []*PriorityMappingModel

    for rows.Next() {
        var m PriorityMappingModel
        if err := rows.Scan(&m.ID, &m.Pattern, &m.Priority, &m.MatchType,
            &m.Description, &m.AccountName, &m.Enabled, &m.CreatedAt, &m.UpdatedAt, &m.CreatedBy); err != nil {
            return nil, fmt.Errorf("failed to scan row: %w", err)
        }
        mappings = append(mappings, &m)
    }

    return mappings, nil
}

// compile compiles a pattern into an efficient match function
func (pm *PriorityMapping) compile() error {
    pattern := pm.Pattern

    // Use match type from database if specified
    switch pm.MatchType {
    case MatchExact:
        pm.matchFunc = func(key string) bool {
            return key == pattern
        }
        return nil

    case MatchPrefix:
        prefix := strings.TrimSuffix(pattern, "*")
        pm.matchFunc = func(key string) bool {
            return strings.HasPrefix(key, prefix)
        }
        return nil

    case MatchSuffix:
        suffix := strings.TrimPrefix(pattern, "*")
        pm.matchFunc = func(key string) bool {
            return strings.HasSuffix(key, suffix)
        }
        return nil

    case MatchContains:
        substr := strings.Trim(pattern, "*")
        pm.matchFunc = func(key string) bool {
            return strings.Contains(key, substr)
        }
        return nil

    case MatchRegex:
        regex, err := regexp.Compile(pattern)
        if err != nil {
            return fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
        }
        pm.matchFunc = func(key string) bool {
            return regex.MatchString(key)
        }
        return nil

    default:
        return fmt.Errorf("unknown match type: %v", pm.MatchType)
    }
}
```

### 4. HTTP API Endpoints (Management UI)

新增RESTful API用于管理映射规则（类似LiteLLM的管理接口）

#### 4.1 CRUD Endpoints

```go
// internal/httpserver/endpoint_api_key_priority.go

// GET /admin/api-key-priority/mappings
// List all mappings
func (s *Server) HandleListAPIKeyMappings(w http.ResponseWriter, r *http.Request) {
    if s.apiKeyMapper == nil {
        http.Error(w, "API key priority mapping not enabled", http.StatusNotImplemented)
        return
    }

    mappings, err := s.apiKeyMapper.ListMappings()
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to list mappings: %v", err), http.StatusInternalServerError)
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "mappings": mappings,
        "total":    len(mappings),
    })
}

// POST /admin/api-key-priority/mappings
// Create a new mapping
func (s *Server) HandleCreateAPIKeyMapping(w http.ResponseWriter, r *http.Request) {
    if s.apiKeyMapper == nil {
        http.Error(w, "API key priority mapping not enabled", http.StatusNotImplemented)
        return
    }

    var req struct {
        Pattern     string `json:"pattern"`
        Priority    int    `json:"priority"`
        MatchType   string `json:"match_type"`
        Description string `json:"description"`
        AccountName string `json:"account_name"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
        return
    }

    // Validate priority
    if req.Priority < 0 || req.Priority > 9 {
        http.Error(w, "Priority must be between 0 and 9", http.StatusBadRequest)
        return
    }

    // Validate match type
    matchType := scheduler.ParseMatchType(req.MatchType)

    // Extract creator from auth context (future enhancement)
    createdBy := "admin"  // TODO: Extract from JWT/session

    err := s.apiKeyMapper.AddMapping(
        req.Pattern,
        scheduler.PriorityTier(req.Priority),
        matchType,
        req.Description,
        req.AccountName,
        createdBy,
    )

    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to create mapping: %v", err), http.StatusInternalServerError)
        return
    }

    s.respondJSON(w, http.StatusCreated, map[string]interface{}{
        "success": true,
        "message": "Mapping created successfully",
    })
}

// PUT /admin/api-key-priority/mappings/:id
// Update an existing mapping
func (s *Server) HandleUpdateAPIKeyMapping(w http.ResponseWriter, r *http.Request) {
    if s.apiKeyMapper == nil {
        http.Error(w, "API key priority mapping not enabled", http.StatusNotImplemented)
        return
    }

    // Extract ID from URL
    idStr := r.URL.Query().Get("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid mapping ID", http.StatusBadRequest)
        return
    }

    var req struct {
        Priority    int    `json:"priority"`
        Description string `json:"description"`
        Enabled     bool   `json:"enabled"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
        return
    }

    err = s.apiKeyMapper.UpdateMapping(
        id,
        scheduler.PriorityTier(req.Priority),
        req.Description,
        req.Enabled,
    )

    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to update mapping: %v", err), http.StatusInternalServerError)
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "message": "Mapping updated successfully",
    })
}

// DELETE /admin/api-key-priority/mappings/:id
// Delete a mapping
func (s *Server) HandleDeleteAPIKeyMapping(w http.ResponseWriter, r *http.Request) {
    if s.apiKeyMapper == nil {
        http.Error(w, "API key priority mapping not enabled", http.StatusNotImplemented)
        return
    }

    idStr := r.URL.Query().Get("id")
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil {
        http.Error(w, "Invalid mapping ID", http.StatusBadRequest)
        return
    }

    err = s.apiKeyMapper.DeleteMapping(id)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to delete mapping: %v", err), http.StatusInternalServerError)
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "message": "Mapping deleted successfully",
    })
}

// POST /admin/api-key-priority/reload
// Manually reload cache from database
func (s *Server) HandleReloadAPIKeyMappings(w http.ResponseWriter, r *http.Request) {
    if s.apiKeyMapper == nil {
        http.Error(w, "API key priority mapping not enabled", http.StatusNotImplemented)
        return
    }

    if err := s.apiKeyMapper.Reload(); err != nil {
        http.Error(w, fmt.Sprintf("Failed to reload mappings: %v", err), http.StatusInternalServerError)
        return
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "message": "Mappings reloaded successfully",
    })
}
```

### 6. HTTP层集成 (HTTP Integration)

#### 6.1 提取API Key和Priority

```go
// internal/httpserver/scheduler_integration.go

// extractPriorityFromRequest extracts priority from request
// Priority sources (in order):
//   1. X-Priority header (explicit, highest priority)
//   2. API key mapping (implicit, via APIKeyMapper)
//   3. Default priority (fallback)
func (s *Server) extractPriorityFromRequest(r *http.Request) scheduler.PriorityTier {
    // 1. Check explicit X-Priority header
    if priorityStr := r.Header.Get("X-Priority"); priorityStr != "" {
        if priority, err := strconv.Atoi(priorityStr); err == nil {
            if priority >= 0 && priority <= 9 {
                log.Printf("[DEBUG] Using explicit priority from X-Priority header: P%d", priority)
                return scheduler.PriorityTier(priority)
            }
        }
    }

    // 2. Map from API key (implicit)
    if s.apiKeyMapper != nil && s.apiKeyMapper.IsEnabled() {
        apiKey := extractAPIKey(r)
        if apiKey != "" {
            priority := s.apiKeyMapper.GetPriority(apiKey)
            log.Printf("[DEBUG] Mapped API key %s... to priority P%d",
                maskAPIKey(apiKey), priority)
            return priority
        }
    }

    // 3. Fallback to default
    log.Printf("[DEBUG] Using default priority: P%d", s.defaultPriority)
    return s.defaultPriority
}

// extractAPIKey extracts API key from Authorization header
func extractAPIKey(r *http.Request) string {
    auth := r.Header.Get("Authorization")
    if auth == "" {
        return ""
    }

    // Bearer token
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }

    // Direct API key
    return auth
}

// maskAPIKey masks API key for logging (show first 8 chars only)
func maskAPIKey(apiKey string) string {
    if len(apiKey) <= 8 {
        return apiKey
    }
    return apiKey[:8] + "..."
}
```

#### 3.2 Server初始化集成

```go
// internal/httpserver/server.go

type Server struct {
    // ... existing fields ...
    apiKeyMapper    *scheduler.APIKeyMapper
    defaultPriority scheduler.PriorityTier
}

// SetAPIKeyMapper sets the API key mapper
func (s *Server) SetAPIKeyMapper(mapper *scheduler.APIKeyMapper) {
    s.apiKeyMapper = mapper
    log.Printf("[INFO] Server: API key mapper configured (enabled=%v)", mapper.IsEnabled())
}
```

```go
// cmd/gatewayd/main.go

// Initialize API key mapper (only if enabled in config)
if cfg.APIKeyPriorityEnabled {
    log.Printf("Initializing API key priority mapper (database-backed)...")

    // Open database (use identity store database)
    dbPath := cfg.APIKeyPriorityDBPath
    if dbPath == "" {
        dbPath = filepath.Join(cfg.DataDir, "identity.db")
    }

    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        log.Fatalf("Failed to open database for API key mapper: %v", err)
    }

    // Create API key mapper with database backend
    mapper, err := scheduler.NewAPIKeyMapper(
        db,
        scheduler.PriorityTier(cfg.APIKeyPriorityDefault),
        cfg.APIKeyPriorityEnabled,
        time.Duration(cfg.APIKeyPriorityCacheTTL)*time.Second,
    )
    if err != nil {
        log.Fatalf("Failed to initialize API key mapper: %v", err)
    }

    httpSrv.SetAPIKeyMapper(mapper)

    log.Printf("API key priority mapper initialized (cache_ttl=%ds)", cfg.APIKeyPriorityCacheTTL)
} else {
    log.Printf("[INFO] API key priority mapping disabled (Personal Edition)")
}
```

### 5. 测试策略 (Testing)

#### 5.1 单元测试

```go
// internal/scheduler/api_key_mapper_test.go

func TestAPIKeyMapper_ExactMatch(t *testing.T) {
    mapper := NewAPIKeyMapper(PriorityNormal, true)
    mapper.AddMapping("sk-exact-key-123", PriorityCritical)

    priority := mapper.GetPriority("sk-exact-key-123")
    assert.Equal(t, PriorityCritical, priority)

    priority = mapper.GetPriority("sk-exact-key-456")
    assert.Equal(t, PriorityNormal, priority) // Default
}

func TestAPIKeyMapper_PrefixMatch(t *testing.T) {
    mapper := NewAPIKeyMapper(PriorityNormal, true)
    mapper.AddMapping("sk-dept-a-*", PriorityCritical)

    priority := mapper.GetPriority("sk-dept-a-user123")
    assert.Equal(t, PriorityCritical, priority)

    priority = mapper.GetPriority("sk-dept-b-user123")
    assert.Equal(t, PriorityNormal, priority) // Default
}

func TestAPIKeyMapper_Priority(t *testing.T) {
    mapper := NewAPIKeyMapper(PriorityNormal, true)

    // Add mappings (order matters - first match wins)
    mapper.AddMapping("sk-admin-*", PriorityCritical)          // P0
    mapper.AddMapping("sk-dept-*", PriorityHigh)               // P2
    mapper.AddMapping("*", PriorityLow)                        // P7 (catch-all)

    // Test priority order
    assert.Equal(t, PriorityCritical, mapper.GetPriority("sk-admin-user1"))
    assert.Equal(t, PriorityHigh, mapper.GetPriority("sk-dept-a-user1"))
    assert.Equal(t, PriorityLow, mapper.GetPriority("sk-other-user1"))
}
```

#### 5.2 集成测试

```bash
# tests/integration/scheduler/test_api_key_priority.sh

#!/bin/bash
# Test API key to priority mapping

# Setup: Create config with mappings
cat > /tmp/api_key_priority_test.ini <<EOF
[api_key_priority]
sk-dept-a-* = 0
sk-premium-* = 5
sk-free-* = 9

[api_key_priority_default]
default_priority = 7
enabled = true
EOF

# Start gateway with API key mapping
export TOKLIGENCE_API_KEY_PRIORITY_ENABLED=true
export TOKLIGENCE_API_KEY_PRIORITY_CONFIG=/tmp/api_key_priority_test.ini
./bin/gatewayd &

# Test 1: Department A key -> P0
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-dept-a-user123" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'

# Verify in logs: "Mapped API key sk-dept-a... to priority P0"

# Test 2: Premium key -> P5
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-premium-customer456" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'

# Verify in logs: "Mapped API key sk-premiu... to priority P5"

# Test 3: Unknown key -> P7 (default)
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-unknown-xyz" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'

# Verify in logs: "Using default priority: P7"

# Check scheduler stats
curl http://localhost:8081/admin/scheduler/stats | jq '.queue_stats[] | select(.current_depth > 0)'
```

### 6. 配置示例 (Configuration Examples)

#### 6.1 电商公司场景

```ini
[api_key_priority]
# Internal departments (high priority)
sk-dept-ml-* = 0          # ML team - Critical
sk-dept-analytics-* = 2   # Analytics - High
sk-dept-recs-* = 3        # Recommendations - Normal-high

# External customers (lower priority)
sk-external-enterprise-* = 5   # Enterprise customers
sk-external-premium-* = 6      # Premium tier
sk-external-standard-* = 7     # Standard tier
sk-external-free-* = 9         # Free tier

# Admin/Special
sk-admin-* = 0            # Admin keys - Highest priority

[api_key_priority_default]
default_priority = 7
enabled = true
```

### 7. 监控和调试 (Monitoring & Debugging)

#### 7.1 日志输出

```
[INFO] APIKeyMapper: Loaded 8 API key priority mappings
[DEBUG] Mapped API key sk-dept-m... to priority P0
[DEBUG] Mapped API key sk-premiu... to priority P5
[DEBUG] Using default priority: P7 for unmapped key
```

#### 7.2 HTTP Endpoint

新增endpoint查看mappings：

```bash
# GET /admin/scheduler/api-key-mappings
curl http://localhost:8081/admin/scheduler/api-key-mappings

# Response:
{
  "enabled": true,
  "default_priority": 7,
  "mappings": [
    {"pattern": "sk-dept-a-*", "priority": 0, "match_type": "prefix"},
    {"pattern": "sk-premium-*", "priority": 5, "match_type": "prefix"},
    {"pattern": "sk-free-*", "priority": 9, "match_type": "prefix"}
  ],
  "total_mappings": 3
}
```

### 8. 向后兼容性 (Backward Compatibility)

- ✅ `X-Priority` header优先级最高（保持现有行为）
- ✅ API key mapping可选（默认禁用）
- ✅ 未匹配的key使用default priority
- ✅ 不影响现有scheduler逻辑

### 9. 性能考虑 (Performance)

- **Pattern匹配优化**:
  - Exact match: O(1) hash lookup
  - Prefix match: O(n) string comparison（编译时优化）
  - Regex match: 仅在必要时使用

- **并发安全**: RWMutex保护，读多写少场景性能好

- **热加载**: 支持运行时reload配置，无需重启

### 10. Personal vs Team Edition 差异

| Feature | Personal Edition | Team Edition |
|---------|------------------|--------------|
| **API Key Priority Mapping** | ❌ Disabled by default | ✅ Can be enabled |
| **Configuration** | `enabled = false` | `enabled = true` |
| **Database Tables** | Created but unused | Active |
| **Management API** | Returns 501 Not Implemented | Fully functional |
| **Use Case** | Single user, no priority needed | Multi-tenant, priority control |

**重要提示**:
- Personal Edition用户看不到任何性能影响（mapper未初始化）
- Team Edition用户需要显式设置`enabled = true`
- 如果enabled=false，所有请求使用default_priority

### 11. 文件清单 (Files to Create/Modify)

**新增文件**:
- `internal/scheduler/api_key_priority_store.go` - 数据库Models和表结构
- `internal/scheduler/api_key_mapper.go` - 核心mapper实现（database-backed）
- `internal/scheduler/api_key_mapper_test.go` - 单元测试
- `internal/httpserver/endpoint_api_key_priority.go` - CRUD Management API
- `tests/integration/scheduler/test_api_key_priority_crud.sh` - 集成测试（CRUD）
- `tests/integration/scheduler/test_api_key_priority_disabled.sh` - 测试Personal Edition

**修改文件**:
- `internal/httpserver/scheduler_integration.go` - 更新extractPriorityFromRequest
- `internal/httpserver/server.go` - 添加apiKeyMapper字段和CRUD routes
- `cmd/gatewayd/main.go` - Database initialization + mapper初始化
- `internal/config/config.go` - 添加配置字段（Enabled, DBPath, CacheTTL）
- `config/setting.ini` - 添加api_key_priority section

## 实施步骤 (Implementation Steps)

### Step 1: 数据库表和Models (1.5小时)
1. 创建`internal/scheduler/api_key_priority_store.go`
2. 定义数据库表结构（SQLite）
3. 实现PriorityMappingModel结构体
4. 单元测试（数据库CRUD）

### Step 2: 核心Mapper实现 (2.5小时)
1. 创建`internal/scheduler/api_key_mapper.go`
2. 实现APIKeyMapper结构体（with database backend）
3. 实现pattern matching逻辑（compile()方法）
4. 实现缓存机制（TTL-based reload）
5. 单元测试（pattern matching）

### Step 3: HTTP Management API (2小时)
1. 创建`internal/httpserver/endpoint_api_key_priority.go`
2. 实现CRUD endpoints（List, Create, Update, Delete）
3. 实现Reload endpoint
4. 添加请求验证和错误处理

### Step 4: HTTP Integration (1小时)
1. 修改`internal/httpserver/scheduler_integration.go`
2. 更新`extractPriorityFromRequest`支持database mapper
3. 修改`internal/httpserver/server.go`添加apiKeyMapper字段
4. Server初始化集成

### Step 5: Main集成 (0.5小时)
1. 修改`cmd/gatewayd/main.go`
2. 添加database initialization
3. 添加mapper initialization（with enabled check）
4. 添加日志

### Step 6: 配置文件 (0.5小时)
1. 更新`config/setting.ini`
2. 添加api_key_priority section（with enabled=false default）
3. 更新`internal/config/config.go`添加配置字段

### Step 7: 测试和文档 (1-2小时)
1. 集成测试脚本（测试CRUD API）
2. 测试Personal Edition（enabled=false）
3. 测试Team Edition（enabled=true）
4. 更新README和testing guide

## 验收标准 (Acceptance Criteria)

- [ ] API key pattern matching支持: exact, prefix, suffix, contains, regex
- [ ] `X-Priority` header优先级高于API key mapping
- [ ] 未匹配key使用default priority
- [ ] 配置文件热加载
- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试通过
- [ ] 性能测试: 1000 QPS下mapping开销 < 100μs
- [ ] 向后兼容现有行为

## 风险和缓解 (Risks & Mitigation)

| 风险 | 影响 | 缓解措施 |
|-----|------|---------|
| Pattern匹配性能开销 | 高QPS下延迟增加 | 优化为O(1) exact match, 缓存regex |
| 配置错误导致优先级混乱 | 关键请求被降级 | 配置验证, default safe priority |
| API key泄漏在日志中 | 安全风险 | Mask API key (only show first 8 chars) |

## 下一步 (Next Phase)

Phase 1完成后，进入**Phase 2: Per-Account Quota Management**，基于API key实现per-account限流。
