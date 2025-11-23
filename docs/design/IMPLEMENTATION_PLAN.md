# Three-Phase Implementation Plan: Priority-Based Scheduler Enhancements

## Overview

本文档规划了基于priority queue scheduler的三个增强阶段的实施顺序和分支策略。

**基础**: 已完成的Channel-Based Priority Scheduler（feat/priority-scheduling分支）

**目标**: 分三个独立分支逐步实现多租户GPU共享场景的完整功能。

---

## Phase 1: API Key to Priority Mapping

### 分支信息

- **Branch Name**: `feat/scheduler-phase1-api-key-mapping`
- **Base Branch**: `feat/priority-scheduling` (或 main如果priority-scheduling已合并)
- **Estimated Effort**: 6-8 hours

### 核心功能

自动将API Key映射到优先级，无需客户端手动设置X-Priority头。

### 关键特性

- ✅ Database-backed storage (SQLite)
- ✅ Pattern matching (exact, prefix, suffix, contains, regex)
- ✅ RESTful CRUD API for management
- ✅ TTL-based cache (default 5min)
- ✅ Personal Edition: disabled by default
- ✅ Team Edition: optional enable

### 数据库表

```sql
CREATE TABLE api_key_priority_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pattern TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL,
    description TEXT,
    account_name TEXT,
    enabled BOOLEAN DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT
);

CREATE TABLE api_key_priority_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT
);
```

### HTTP API Endpoints

```
GET    /admin/api-key-priority/mappings      - List all mappings
POST   /admin/api-key-priority/mappings      - Create mapping
PUT    /admin/api-key-priority/mappings/:id  - Update mapping
DELETE /admin/api-key-priority/mappings/:id  - Delete mapping
POST   /admin/api-key-priority/reload        - Reload cache
```

### 配置示例

```ini
[api_key_priority]
enabled = false  # Personal Edition default
default_priority = 7
db_path = ~/.tokligence/identity.db
cache_ttl_sec = 300
```

### 实施步骤

1. 数据库表和Models (1.5h)
2. 核心Mapper实现 (2.5h)
3. HTTP Management API (2h)
4. HTTP Integration (1h)
5. Main集成 (0.5h)
6. 配置文件 (0.5h)
7. 测试和文档 (1-2h)

### 验收标准

- [ ] Database tables创建成功
- [ ] Pattern matching支持5种类型
- [ ] CRUD API功能完整
- [ ] Personal Edition默认禁用
- [ ] Team Edition可启用
- [ ] Cache TTL正常工作
- [ ] X-Priority header优先级高于mapping
- [ ] 集成测试通过

### 依赖文件

**新增**:
- `internal/scheduler/api_key_priority_store.go`
- `internal/scheduler/api_key_mapper.go`
- `internal/scheduler/api_key_mapper_test.go`
- `internal/httpserver/endpoint_api_key_priority.go`

**修改**:
- `internal/httpserver/scheduler_integration.go`
- `internal/httpserver/server.go`
- `cmd/gatewayd/main.go`
- `internal/config/config.go`
- `config/setting.ini`

---

## Phase 2: Per-Account Quota Management

### 分支信息

- **Branch Name**: `feat/scheduler-phase2-account-quota`
- **Base Branch**: `feat/scheduler-phase1-api-key-mapping`
- **Estimated Effort**: 6-8 hours

### 核心功能

为每个账户（API key）设置独立的资源配额限制。

**关键设计原则**: **Orthogonal to Priority Queue** - 配额限制和优先级调度相互独立

### 关键特性

- ✅ Per-account resource limits (concurrent, RPS, tokens/sec, daily requests)
- ✅ Atomic operations (no locks)
- ✅ Sliding window rate limiting
- ✅ Database-backed configuration
- ✅ RESTful CRUD API
- ✅ Real-time quota monitoring

### 架构流程

```
Request arrives
    ↓
1. Check Account Quota (Phase 2) ← NEW
    ├─ Quota exceeded → Reject (429)
    └─ Quota OK → Reserve slot
        ↓
2. Check Priority & Schedule (Phase 0)
    ├─ Queue full → Reject (503)
    └─ Accepted → Enter priority queue
        ↓
3. Global Capacity Check (Phase 0)
    ├─ Capacity exceeded → Wait in queue
    └─ Capacity OK → Process request
        ↓
4. Release quota slot (Phase 2)
```

### 数据库表

```sql
CREATE TABLE account_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id TEXT NOT NULL UNIQUE,

    -- Quota limits
    max_concurrent INTEGER NOT NULL DEFAULT 10,
    max_rps INTEGER NOT NULL DEFAULT 10,
    max_tokens_per_sec INTEGER NOT NULL DEFAULT 1000,
    max_requests_per_day INTEGER NOT NULL DEFAULT 10000,

    -- Metadata
    account_name TEXT,
    description TEXT,
    enabled BOOLEAN DEFAULT 1,

    -- Audit
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT
);

CREATE INDEX idx_account_id ON account_quotas(account_id);
CREATE INDEX idx_enabled ON account_quotas(enabled);
```

### HTTP API Endpoints

```
GET    /admin/scheduler/quotas             - List all quotas
POST   /admin/scheduler/quotas             - Create quota
PUT    /admin/scheduler/quotas/:id         - Update quota
DELETE /admin/scheduler/quotas/:id         - Delete quota
GET    /admin/scheduler/quotas/:account_id/status  - Real-time status
POST   /admin/scheduler/quotas/reset       - Reset daily counters
```

### 实施步骤

1. 数据库表和Models (1.5h)
2. AccountQuota实现 (atomic operations) (2h)
3. AccountQuotaManager实现 (1.5h)
4. Scheduler集成（orthogonal check） (1h)
5. HTTP Management API (1.5h)
6. 测试和文档 (1.5h)

### 验收标准

- [ ] Atomic quota operations (no locks)
- [ ] Sliding window rate limiting正常工作
- [ ] Per-account limits独立于global limits
- [ ] Quota exceeded返回429 (not 503)
- [ ] 与Priority Queue正交（独立工作）
- [ ] CRUD API功能完整
- [ ] Real-time监控正常
- [ ] Daily reset功能正常

### 依赖文件

**新增**:
- `internal/scheduler/account_quota.go`
- `internal/scheduler/account_quota_manager.go`
- `internal/scheduler/account_quota_test.go`
- `internal/httpserver/endpoint_account_quota.go`

**修改**:
- `internal/scheduler/scheduler_channel.go` (add quota check)
- `internal/httpserver/scheduler_integration.go`
- `internal/httpserver/server.go`
- `cmd/gatewayd/main.go`

---

## Phase 3: Time-Based Dynamic Rules

### 分支信息

- **Branch Name**: `feat/scheduler-phase3-time-rules`
- **Base Branch**: `feat/scheduler-phase2-account-quota`
- **Estimated Effort**: 10-12 hours

### 核心功能

根据时间段自动调整调度器行为（权重、配额、容量）。

### 关键特性

- ✅ Time window definitions (start/end hour, days of week, timezone)
- ✅ Three rule types: Weight adjustment, Quota adjustment, Capacity adjustment
- ✅ Cron-like evaluation engine (configurable interval, default 60s)
- ✅ Database-backed rules
- ✅ Time override for testing
- ✅ RESTful management API

### 使用场景

**电商公司GPU共享**:
- **白天 (08:00-18:00)**: 内部部门80%容量，外部客户20%
- **夜晚 (18:00-08:00)**: 内部部门20%容量，外部客户80%

### 数据库表

```sql
CREATE TABLE time_based_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_name TEXT NOT NULL UNIQUE,
    rule_type TEXT NOT NULL CHECK(rule_type IN ('weight_adjustment', 'quota_adjustment', 'capacity_adjustment')),

    -- Time window
    start_hour INTEGER NOT NULL CHECK(start_hour >= 0 AND start_hour <= 23),
    start_minute INTEGER NOT NULL DEFAULT 0,
    end_hour INTEGER NOT NULL CHECK(end_hour >= 0 AND end_hour <= 23),
    end_minute INTEGER NOT NULL DEFAULT 0,
    days_of_week TEXT,  -- JSON array: [1,2,3,4,5] for Mon-Fri
    timezone TEXT DEFAULT 'UTC',

    -- Rule parameters (JSON)
    parameters TEXT NOT NULL,  -- Rule-specific parameters

    -- Metadata
    description TEXT,
    enabled BOOLEAN DEFAULT 1,

    -- Audit
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT
);
```

### Rule Types

#### 1. Weight Adjustment Rule

```json
{
  "rule_type": "weight_adjustment",
  "parameters": {
    "weights": [256, 128, 64, 32, 16, 8, 4, 2, 1, 1]
  }
}
```

#### 2. Quota Adjustment Rule

```json
{
  "rule_type": "quota_adjustment",
  "parameters": {
    "adjustments": [
      {"account_pattern": "dept-a-*", "max_concurrent": 50},
      {"account_pattern": "premium-*", "max_concurrent": 10}
    ]
  }
}
```

#### 3. Capacity Adjustment Rule

```json
{
  "rule_type": "capacity_adjustment",
  "parameters": {
    "max_concurrent": 200,
    "max_rps": 400,
    "max_tokens_per_sec": 8000
  }
}
```

### HTTP API Endpoints

```
GET    /admin/scheduler/time-rules           - List all rules
POST   /admin/scheduler/time-rules           - Create rule
PUT    /admin/scheduler/time-rules/:id       - Update rule
DELETE /admin/scheduler/time-rules/:id       - Delete rule
GET    /admin/scheduler/time-rules/active    - Show active rules
POST   /admin/scheduler/time-rules/apply     - Manually trigger evaluation
POST   /admin/scheduler/time-rules/override  - Override system time (testing)
```

### 实施步骤

1. 数据库表和Models (1.5h)
2. TimeWindow实现 (1.5h)
3. RuleEngine核心 (2.5h)
4. Scheduler集成（AdjustWeights等方法） (2h)
5. Quota Manager集成 (1.5h)
6. HTTP Management API (2h)
7. 测试和文档 (2h)

### 验收标准

- [ ] Time window logic正确（包括wrap-around）
- [ ] Rule evaluation正常工作
- [ ] Weight adjustment生效
- [ ] Quota adjustment生效
- [ ] Capacity adjustment生效
- [ ] Cron evaluation间隔可配置
- [ ] Time override用于测试
- [ ] Rule activation日志清晰
- [ ] 集成测试通过

### 依赖文件

**新增**:
- `internal/scheduler/time_window.go`
- `internal/scheduler/time_rules.go`
- `internal/scheduler/rule_engine.go`
- `internal/scheduler/time_rules_test.go`
- `internal/httpserver/endpoint_time_rules.go`

**修改**:
- `internal/scheduler/scheduler_channel.go` (add AdjustWeights, AdjustCapacity)
- `internal/scheduler/account_quota_manager.go` (add UpdateQuota, FindAccountsByPattern)
- `cmd/gatewayd/main.go`
- `internal/config/config.go`

---

## Implementation Timeline

### Week 1: Phase 1 (API Key Priority Mapping)

**Day 1-2**: Core implementation
- Database tables and models
- APIKeyMapper implementation
- Pattern matching logic
- Unit tests

**Day 2-3**: HTTP API and Integration
- CRUD endpoints
- HTTP integration
- Main initialization
- Configuration

**Day 3**: Testing and Documentation
- Integration tests
- Personal/Team edition tests
- Documentation updates

### Week 2: Phase 2 (Per-Account Quota Management)

**Day 1-2**: Core implementation
- Database tables
- AccountQuota with atomic operations
- AccountQuotaManager
- Unit tests

**Day 2-3**: Integration
- Scheduler integration (orthogonal check)
- HTTP CRUD API
- Real-time monitoring

**Day 3**: Testing and Documentation
- Integration tests
- Quota exhaustion tests
- Documentation

### Week 3-4: Phase 3 (Time-Based Dynamic Rules)

**Day 1-2**: Time Window and Rule Engine
- TimeWindow implementation
- RuleEngine core
- Rule evaluation loop

**Day 3-4**: Integration
- Scheduler integration (dynamic adjustments)
- Quota manager integration
- HTTP API

**Day 4-5**: Testing and Documentation
- Time override testing
- Day/night transition tests
- End-to-end scenarios
- Documentation

---

## Testing Strategy

### Per-Phase Testing

每个Phase都需要完整的测试：

1. **Unit Tests**:
   - Database operations
   - Core logic (pattern matching, quota tracking, time windows)
   - Edge cases

2. **Integration Tests**:
   - HTTP API endpoints
   - End-to-end flows
   - Error handling

3. **Cross-Phase Integration Tests**:
   - Phase 1 + Phase 2: API key mapping → account quota
   - Phase 2 + Phase 3: Time rules → quota adjustment
   - Phase 1 + Phase 3: Time rules → priority mapping override

### Test Files Structure

```
tests/integration/scheduler/
├── phase1/
│   ├── test_api_key_mapping.sh
│   ├── test_crud_api.sh
│   └── test_personal_edition.sh
├── phase2/
│   ├── test_account_quota.sh
│   ├── test_quota_exhaustion.sh
│   └── test_orthogonal_priority.sh
├── phase3/
│   ├── test_time_window.sh
│   ├── test_weight_adjustment.sh
│   ├── test_quota_adjustment.sh
│   └── test_day_night_transition.sh
└── integration/
    ├── test_all_phases.sh
    └── test_multi_tenant_scenario.sh
```

---

## Branch Merge Strategy

### Merge Order

```
feat/priority-scheduling (base)
    ↓
feat/scheduler-phase1-api-key-mapping
    ↓
feat/scheduler-phase2-account-quota
    ↓
feat/scheduler-phase3-time-rules
    ↓
main (or develop)
```

### PR Review Checklist

每个Phase的PR都需要检查：

- [ ] 代码质量（无重复代码、清晰命名）
- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试全部通过
- [ ] 文档完整（设计文档、API文档、使用指南）
- [ ] 向后兼容（不破坏现有功能）
- [ ] Performance测试（无显著性能退化）
- [ ] Personal Edition不受影响
- [ ] Configuration defaults正确

---

## Configuration Management

### Phase 1 Configuration

```ini
[api_key_priority]
enabled = false  # Personal Edition default
default_priority = 7
db_path = ~/.tokligence/identity.db
cache_ttl_sec = 300
```

### Phase 2 Configuration

```ini
[account_quota]
enabled = false  # Personal Edition default
default_max_concurrent = 10
default_max_rps = 10
default_max_tokens_per_sec = 1000
db_path = ~/.tokligence/identity.db
```

### Phase 3 Configuration

```ini
[time_rules]
enabled = false  # Personal Edition default
check_interval_sec = 60
default_timezone = UTC
db_path = ~/.tokligence/identity.db
```

### Environment Variables

```bash
# Phase 1
export TOKLIGENCE_API_KEY_PRIORITY_ENABLED=false
export TOKLIGENCE_API_KEY_PRIORITY_DEFAULT=7

# Phase 2
export TOKLIGENCE_ACCOUNT_QUOTA_ENABLED=false
export TOKLIGENCE_ACCOUNT_QUOTA_DEFAULT_CONCURRENT=10

# Phase 3
export TOKLIGENCE_TIME_RULES_ENABLED=false
export TOKLIGENCE_TIME_RULES_CHECK_INTERVAL=60
```

---

## Risk Management

### Technical Risks

| Risk | Impact | Mitigation |
|------|--------|-----------|
| Database schema changes | 破坏现有数据 | Migration scripts, backward compatibility |
| Performance regression | 高QPS下延迟增加 | Performance benchmarks, caching |
| Phase dependencies | 后续phase被block | 每个phase独立可用 |
| Configuration complexity | 用户配置错误 | 良好的defaults, validation |

### Mitigation Strategies

1. **Backward Compatibility**: 所有新功能默认禁用
2. **Progressive Enhancement**: 每个phase独立可用
3. **Testing**: 完整的单元测试和集成测试
4. **Documentation**: 详细的配置说明和示例
5. **Performance**: 每个phase都做performance测试

---

## Success Criteria

### Phase 1 Success

- [ ] API key自动映射到优先级
- [ ] Personal Edition不受影响（disabled）
- [ ] Team Edition可启用并正常工作
- [ ] CRUD API功能完整
- [ ] Performance impact < 1% (QPS降低 < 1%)

### Phase 2 Success

- [ ] Per-account quota独立工作
- [ ] 与priority queue正交（orthogonal）
- [ ] Atomic operations (no lock contention)
- [ ] Quota exhausted返回429
- [ ] Real-time监控正常

### Phase 3 Success

- [ ] Time-based rules自动生效
- [ ] Day/night切换正常
- [ ] Weight adjustment立即生效
- [ ] Quota adjustment立即生效
- [ ] Rule evaluation overhead < 0.2% CPU

### Overall Success (Multi-Tenant GPU Sharing Scenario)

- [ ] 内部部门和外部客户可共享GPU
- [ ] 白天/夜晚自动调整优先级和配额
- [ ] Per-department quota enforcement
- [ ] Real-time监控和管理
- [ ] 100% 满足电商公司场景需求

---

## Documentation Deliverables

### Per-Phase Documentation

1. **Design Document** (已完成)
   - `PHASE1_API_KEY_PRIORITY_MAPPING.md`
   - `PHASE2_PER_ACCOUNT_QUOTA.md`
   - `PHASE3_TIME_BASED_DYNAMIC_RULES.md`

2. **API Documentation**
   - Swagger/OpenAPI specs for each phase
   - cURL examples
   - Response schemas

3. **User Guide**
   - Configuration examples
   - Common scenarios
   - Troubleshooting

4. **Testing Guide**
   - How to run tests
   - How to add new tests
   - Test coverage reports

---

## Summary

**三阶段实施计划总览**:

| Phase | Effort | Key Feature | Dependencies | Edition |
|-------|--------|-------------|--------------|---------|
| Phase 1 | 6-8h | API Key → Priority Mapping | Phase 0 (scheduler) | Personal: disabled, Team: optional |
| Phase 2 | 6-8h | Per-Account Quota | Phase 1 | Personal: disabled, Team: optional |
| Phase 3 | 10-12h | Time-Based Dynamic Rules | Phase 1 + Phase 2 | Personal: disabled, Team: optional |
| **Total** | **22-28h** | **Multi-Tenant GPU Sharing** | - | - |

**关键原则**:
1. ✅ 每个phase独立可用
2. ✅ Personal Edition不受影响（默认禁用）
3. ✅ Team Edition可选启用
4. ✅ Database-backed management (like LiteLLM)
5. ✅ RESTful CRUD APIs for all phases
6. ✅ Comprehensive testing
7. ✅ Detailed documentation

**最终效果**:
支持完整的多租户GPU共享场景，满足电商公司内部部门和外部客户共享GPU资源的需求，实现白天/夜晚自动切换，per-department配额控制，实时监控和管理。
