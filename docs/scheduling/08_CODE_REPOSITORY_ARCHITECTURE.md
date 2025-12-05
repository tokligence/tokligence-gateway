# Code Repository Architecture

**Version:** 1.0
**Date:** 2025-11-23
**Status:** REFERENCE
**Type:** Repository structure and code distribution guide

---

## Overview

Tokligence项目采用**多仓库(multi-repo)架构**，核心调度功能分布在两个主要仓库中：

1. **`tokligence-gateway`** - 网关核心 (本仓库)
2. **`tokligence-marketplace`** - 市场API服务 (独立仓库)

---

## 1. Repository Structure

### 1.1 tokligence-gateway (Core Repository)

**Location:** `/home/alejandroseaah/tokligence/sell_dev/tokligence-gateway`

**Purpose:** LLM网关核心功能，包括协议转换、调度、路由、认证、计费

**Current Status:** v0.3.0 (生产中)

**Key Directories:**
```
tokligence-gateway/
├── cmd/
│   ├── gateway/           # CLI工具 (客户端命令)
│   └── gatewayd/          # 守护进程 (HTTP服务器)
├── internal/
│   ├── httpserver/        # HTTP服务器和端点处理
│   │   ├── responses/     # Responses API (Provider抽象层)
│   │   ├── anthropic/     # Anthropic协议转换
│   │   └── tool_adapter/  # 工具调用适配器
│   ├── adapter/           # LLM提供商适配器 (OpenAI, Anthropic, Gemini)
│   ├── translation/       # 协议转换库 (sidecar模式)
│   ├── ratelimit/         # 令牌桶限流
│   ├── auth/              # 认证和授权
│   ├── ledger/            # 使用记录账本
│   ├── userstore/         # 用户身份存储 (SQLite/PostgreSQL)
│   ├── config/            # 配置管理
│   ├── firewall/          # LLM防护层 (PII检测等)
│   ├── metrics/           # Prometheus指标
│   ├── health/            # 健康检查
│   └── telemetry/         # 遥测上报 (连接marketplace)
├── config/
│   ├── setting.ini        # 全局配置
│   ├── dev/gateway.ini    # 开发环境配置
│   ├── test/gateway.ini   # 测试环境配置
│   └── live/gateway.ini   # 生产环境配置
├── tests/
│   └── integration/       # 集成测试 (36个测试脚本)
├── docs/
│   ├── scheduling/        # 调度功能设计文档 (本文档所在目录)
│   ├── codex-to-anthropic.md
│   ├── claude_code-to-openai.md
│   └── QUICK_START.md
└── Makefile               # 构建和部署命令

快捷命令:
  make gfr   # 强制重启守护进程
  make bt    # 运行后端测试
  make test  # 运行所有测试
```

---

### 1.2 tokligence-marketplace (Marketplace API Service)

**Location:** `/home/alejandroseaah/tokligence/tokligence-marketplace`

**Purpose:** 市场协调层 - 供应商目录、使用统计、遥测、计费

**Current Status:** MVP已交付 (Iteration 1 complete)

**Key Directories:**
```
tokligence-marketplace/
├── cmd/
│   ├── server/            # 市场API服务器
│   └── mvp/               # MVP演示程序
├── internal/
│   ├── handlers/          # HTTP处理器
│   │   ├── account.go     # 用户账户管理
│   │   ├── service.go     # 供应商服务发布
│   │   ├── telemetry.go   # 遥测数据接收
│   │   └── admin.go       # 管理接口 (DAU/WAU统计)
│   ├── pricing/           # 计费和账本
│   │   ├── calculator.go  # 定价计算器
│   │   ├── ledger.go      # 交易账本
│   │   └── usage.go       # 使用统计
│   ├── compliance/        # 合规检查 (自交易防护等)
│   ├── domain/            # 领域模型
│   ├── router/            # HTTP路由
│   └── middleware/        # 认证中间件
├── config/
│   ├── setting.ini        # 全局配置
│   └── dev/app.ini        # 开发环境配置
├── migrations/            # 数据库迁移脚本
├── scripts/               # 部署和管理脚本
├── docs/
│   ├── arch.md            # 架构文档
│   ├── SPEC.md            # API规范
│   └── ROADMAP.md         # 产品路线图
└── Makefile

快捷命令:
  make start       # 启动市场API
  make d-start     # Docker启动
  make check       # 冒烟测试
```

---

## 2. Code Distribution by Feature

### 2.1 Phase 0: Foundation (Weeks 1-4)

**所有代码都在 `tokligence-gateway` 仓库:**

| Component | Location | Files to Create/Modify |
|-----------|----------|------------------------|
| **Multi-dimensional Capacity Model** | `internal/scheduler/` | `capacity_model.go` (new) |
| **Priority Queue** | `internal/scheduler/` | `priority_queue.go` (new) |
| **LocalProvider** | `internal/httpserver/responses/` | `provider_local.go` (new) |
| **Token Routing (Basic)** | `internal/httpserver/responses/` | `conversation.go` (modify) |
| **Degradation (fail-open/fail-closed)** | `internal/httpserver/responses/` | `degradation.go` (new) |

**Key Files to Create:**
```go
// internal/scheduler/capacity_model.go
package scheduler

type CapacityDimension string

const (
    DimTokensPerSec   CapacityDimension = "tokens_per_sec"   // PRIMARY
    DimRPS            CapacityDimension = "rps"               // SECONDARY
    DimConcurrent     CapacityDimension = "concurrent"
    DimContextLength  CapacityDimension = "context_length"
)

type Capacity struct {
    // Limits
    MaxTokensPerSec  int
    MaxRPS           int
    MaxConcurrent    int
    MaxContextLength int

    // Current usage (atomic counters)
    CurrentTokensPerSec int64
    CurrentRPS          int64
    CurrentConcurrent   int64
}

func (c *Capacity) CanAccept(req *Request) bool { ... }
```

```go
// internal/scheduler/priority_queue.go
package scheduler

type PriorityTier int

// Configurable number of priority levels (default: 10)
// Can be configured via TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS
const (
    // Default priority tier mappings (for 10-level system)
    PriorityCritical   PriorityTier = 0  // P0: Internal critical services
    PriorityUrgent     PriorityTier = 1  // P1: High-value urgent requests
    PriorityHigh       PriorityTier = 2  // P2: Partner/premium users
    PriorityElevated   PriorityTier = 3  // P3: Elevated priority
    PriorityAboveNormal PriorityTier = 4 // P4: Above normal
    PriorityNormal     PriorityTier = 5  // P5: Standard users (default)
    PriorityBelowNormal PriorityTier = 6 // P6: Below normal
    PriorityLow        PriorityTier = 7  // P7: Low priority
    PriorityBulk       PriorityTier = 8  // P8: Bulk/batch processing
    PriorityBackground PriorityTier = 9  // P9: Background jobs

    DefaultPriorityLevels = 10  // Default number of priority buckets
)

type SchedulerConfig struct {
    NumPriorityLevels int  // Number of priority buckets (default: 10)
    DefaultPriority   PriorityTier  // Default priority for requests (default: 5)
}

type PriorityQueue struct {
    queues []*Queue  // Dynamic number of queues (configurable)
    config *SchedulerConfig
    mu     sync.RWMutex
}

func NewPriorityQueue(config *SchedulerConfig) *PriorityQueue {
    if config == nil {
        config = &SchedulerConfig{
            NumPriorityLevels: DefaultPriorityLevels,
            DefaultPriority:   PriorityNormal,
        }
    }

    queues := make([]*Queue, config.NumPriorityLevels)
    for i := 0; i < config.NumPriorityLevels; i++ {
        queues[i] = NewQueue()
    }

    return &PriorityQueue{
        queues: queues,
        config: config,
    }
}

func (pq *PriorityQueue) Enqueue(req *Request) { ... }
func (pq *PriorityQueue) Dequeue() *Request { ... }
```

```go
// internal/httpserver/responses/provider_local.go
package responses

type LocalProvider struct {
    capacity  *scheduler.Capacity
    models    map[string]*ModelConfig  // model name -> config
    apiKeys   map[string]string        // model -> API key
    baseURLs  map[string]string        // model -> base URL
}

func NewLocalProvider(cfg *config.LocalProviderConfig) *LocalProvider { ... }
func (lp *LocalProvider) Stream(ctx context.Context, conv Conversation) (StreamInit, error) { ... }
```

**Configuration Updates:**
```ini
# config/dev/gateway.ini (新增)
[scheduling]
enabled = false  # 默认关闭，逐步启用
priority_enabled = false
capacity_model = "simple"  # simple | multi_dimensional

[provider.local]
enabled = true
models = "gpt-4,gpt-4o,claude-3-5-sonnet,claude-3-5-haiku"

[provider.local.gpt-4]
max_tokens_per_sec = 10000
max_rps = 100
max_concurrent = 50
max_context_length = 128000

[provider.local.claude-3-5-sonnet]
max_tokens_per_sec = 5000
max_rps = 50
max_concurrent = 25
max_context_length = 200000
```

**Tests to Create:**
```bash
# tests/integration/scheduling/
test_capacity_limits.sh          # TC-P0-CAP-001~003
test_priority_ordering.sh         # TC-P0-PRI-001~003
test_local_provider.sh            # TC-P0-LOC-001~002
test_token_routing.sh             # TC-P0-TOK-001~002
test_degradation.sh               # TC-P0-DEG-001~002
```

---

### 2.2 Phase 1: Observability & Protection (Weeks 5-6)

**所有代码都在 `tokligence-gateway` 仓库:**

| Component | Location | Files to Create/Modify |
|-----------|----------|------------------------|
| **Scheduling Metrics** | `internal/metrics/` | `scheduling.go` (new) |
| **LLM Protection Integration** | `internal/firewall/` | `firewall.go` (modify) |
| **Circuit Breaker** | `internal/httpserver/responses/` | `circuit_breaker.go` (new) |
| **Snapshot Cache** | `internal/httpserver/responses/` | `snapshot_cache.go` (new) |

**Key Files to Create:**
```go
// internal/metrics/scheduling.go
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
    // Queue depth by priority tier
    SchedulingQueueDepth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "tokligence_scheduling_queue_depth",
            Help: "Number of requests queued by priority tier",
        },
        []string{"priority"},  // 1, 2, 3, 4, 5
    )

    // Scheduling latency histogram
    SchedulingLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "tokligence_scheduling_latency_seconds",
            Help: "Time from request arrival to scheduling decision",
            Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5},  // 1ms, 10ms, 50ms, ...
        },
        []string{"priority"},
    )

    // Capacity usage by dimension
    CapacityUsage = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "tokligence_capacity_usage",
            Help: "Current capacity usage (absolute value)",
        },
        []string{"dimension", "model"},  // tokens_per_sec, rps, concurrent
    )

    // Capacity utilization (0.0-1.0)
    CapacityUtilization = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "tokligence_capacity_utilization",
            Help: "Capacity utilization ratio (0.0-1.0)",
        },
        []string{"dimension", "model"},
    )
)
```

```go
// internal/httpserver/responses/circuit_breaker.go
package responses

type CircuitState int

const (
    CircuitClosed   CircuitState = 0  // Normal operation
    CircuitOpen     CircuitState = 1  // Failing, reject requests
    CircuitHalfOpen CircuitState = 2  // Testing recovery
)

type CircuitBreaker struct {
    threshold        int            // Open after N failures
    timeout          time.Duration  // Keep open for this long
    consecutiveFails int
    state            CircuitState
    lastFailTime     time.Time
    mu               sync.RWMutex
}

func (cb *CircuitBreaker) Call(fn func() error) error { ... }
```

**Tests to Create:**
```bash
# tests/integration/scheduling/
test_queue_metrics.sh             # TC-P1-OBS-001~003
test_protection_integration.sh    # TC-P1-PROT-001~002
test_circuit_breaker.sh           # TC-P1-CB-001~002
test_snapshot_cache.sh            # TC-P1-SNAP-001~002
```

---

### 2.3 Phase 2: Advanced Routing (Weeks 7-8)

**主要代码在 `tokligence-gateway`，数据库交互:**

| Component | Location | Files to Create/Modify |
|-----------|----------|------------------------|
| **Token Metadata Store** | `internal/userstore/` | `token_metadata.go` (new) |
| **Redis Cache Layer** | `internal/cache/` | `redis_token_cache.go` (new) |
| **Snapshot Cache** | `internal/httpserver/responses/` | `snapshot_cache.go` (enhance from Phase 1) |
| **Layered Lookup** | `internal/httpserver/responses/` | `token_lookup.go` (new) |

**Database Schema (PostgreSQL):**
```sql
-- migrations/004_token_metadata.sql
CREATE TABLE IF NOT EXISTS token_metadata (
    token_hash      BYTEA PRIMARY KEY,          -- sha256(api_key)
    tier            VARCHAR(32) NOT NULL,        -- internal | partner | standard | external
    priority        INT NOT NULL DEFAULT 3,      -- 1-5 (maps to PriorityTier)
    quota_limit     BIGINT NOT NULL DEFAULT 0,   -- tokens/day (0 = unlimited, anti-abuse only)
    current_usage   BIGINT NOT NULL DEFAULT 0,   -- current usage today
    reset_at        TIMESTAMP NOT NULL,          -- quota reset time
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_token_tier ON token_metadata(tier);
CREATE INDEX idx_token_priority ON token_metadata(priority);
```

**Key Files to Create:**
```go
// internal/httpserver/responses/token_lookup.go
package responses

type TokenMetadata struct {
    Tier         string
    Priority     int
    QuotaLimit   int64
    CurrentUsage int64
}

type LayeredTokenCache struct {
    lru         *lrucache.Cache          // Layer 1: LRU (hot tokens)
    redis       *cache.RedisClient       // Layer 2: Redis
    db          *userstore.PostgresClient // Layer 3: PostgreSQL
    snapshot    *SnapshotCache           // Layer 4: Snapshot (read-only)
    degradeMode string                   // fail_open | fail_closed
}

func (ltc *LayeredTokenCache) Lookup(tokenHash []byte) (*TokenMetadata, error) {
    // Layer 1: LRU cache (<1μs)
    if meta, ok := ltc.lru.Get(tokenHash); ok {
        metrics.TokenLookupTotal.WithLabelValues("lru", "hit").Inc()
        return meta, nil
    }

    // Layer 2: Redis (<1ms)
    if ltc.redis != nil {
        meta, err := ltc.redis.GetTokenMetadata(tokenHash)
        if err == nil {
            ltc.lru.Set(tokenHash, meta)  // Promote to LRU
            metrics.TokenLookupTotal.WithLabelValues("redis", "hit").Inc()
            return meta, nil
        }
    }

    // Layer 3: PostgreSQL (<10ms)
    if ltc.db != nil {
        meta, err := ltc.db.GetTokenMetadata(tokenHash)
        if err == nil {
            ltc.redis.SetTokenMetadata(tokenHash, meta)  // Promote to Redis
            ltc.lru.Set(tokenHash, meta)                  // Promote to LRU
            metrics.TokenLookupTotal.WithLabelValues("postgresql", "hit").Inc()
            return meta, nil
        }
    }

    // Layer 4: Snapshot cache (degraded mode)
    if ltc.snapshot != nil {
        if meta, ok := ltc.snapshot.Get(tokenHash); ok {
            log.Warn("Using snapshot cache (stores down)")
            metrics.TokenLookupTotal.WithLabelValues("snapshot", "hit").Inc()
            return meta, nil
        }
    }

    // All layers miss → Fail-open or fail-closed
    return ltc.handleAllStoresDown(tokenHash)
}
```

**Tests to Create:**
```bash
# tests/integration/scheduling/
test_layered_lookup.sh            # TC-P2-LAYER-001~004
test_fail_modes.sh                # TC-P2-FAIL-001~002
```

---

### 2.4 Phase 3: Marketplace Opt-In (Weeks 9-12, OPTIONAL)

**跨两个仓库:**

#### 2.4.1 Gateway侧 (tokligence-gateway)

| Component | Location | Files to Create/Modify |
|-----------|----------|------------------------|
| **MarketplaceProvider** | `internal/httpserver/responses/` | `provider_marketplace.go` (new) |
| **Marketplace Client** | `internal/client/` | `marketplace_client.go` (new) |
| **Supply Discovery** | `internal/httpserver/responses/` | `supply_discovery.go` (new) |

```go
// internal/httpserver/responses/provider_marketplace.go
package responses

type MarketplaceProvider struct {
    client       *client.MarketplaceClient
    circuitBreaker *CircuitBreaker
    supplyCache  *SupplyCache
    config       *config.MarketplaceConfig
}

func NewMarketplaceProvider(cfg *config.MarketplaceConfig) *MarketplaceProvider { ... }

func (mp *MarketplaceProvider) DiscoverSupply(model string, region string) ([]*Supply, error) {
    // Circuit breaker protection
    return mp.circuitBreaker.Call(func() error {
        return mp.client.DiscoverSupply(model, region)
    })
}
```

```go
// internal/client/marketplace_client.go
package client

type MarketplaceClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}

// POST /v1/marketplace/discover
func (mc *MarketplaceClient) DiscoverSupply(req *DiscoverRequest) (*DiscoverResponse, error) {
    // Send: model, region, max_price
    // Receive: list of providers with endpoints, pricing
}

// POST /v1/marketplace/usage
func (mc *MarketplaceClient) ReportUsage(req *UsageReport) error {
    // Report token usage for billing
}
```

**Configuration:**
```ini
# config/dev/gateway.ini
[provider.marketplace]
enabled = false  # 默认禁用 (CRITICAL)
base_url = "https://marketplace.tokligence.ai"
api_key = ""  # User must set this to opt-in
free_tier_limit = 100  # requests/day
offline_mode = false   # Use supply cache only
degradation_mode = "fail_open"  # fail_open | fail_closed | cached

[provider.marketplace.privacy]
send_request_content = false  # NEVER send user prompts
send_pii = false              # NEVER send PII
data_sent = "model,region,max_price"  # Only metadata
```

#### 2.4.2 Marketplace侧 (tokligence-marketplace)

**已存在的API端点 (无需修改):**

| Endpoint | Handler | Purpose |
|----------|---------|---------|
| `POST /users` | `internal/handlers/account.go` | 用户账户创建 |
| `POST /services` | `internal/handlers/service.go` | 供应商服务发布 |
| `GET /providers` | `internal/handlers/service.go` | 查询供应商列表 |
| `GET /services` | `internal/handlers/service.go` | 查询服务列表 |
| `POST /usage` | `internal/handlers/account.go` | 上报使用量 |
| `GET /usage/summary` | `internal/handlers/account.go` | 使用统计摘要 |
| `POST /v1/telemetry` | `internal/handlers/telemetry.go` | 遥测数据接收 |
| `GET /v1/telemetry/stats` | `internal/handlers/admin.go` | DAU/WAU统计 |

**需要新增的API端点:**

```go
// internal/handlers/marketplace.go (NEW)
package handlers

// POST /v1/marketplace/discover
// Request: { model, region, max_price }
// Response: { providers: [...] }
func (h *Handler) DiscoverSupply(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    // 2. Query services table filtered by model + region
    // 3. Apply max_price filter
    // 4. Return top 5 providers sorted by (price, latency, health)
}

// POST /v1/marketplace/health
// Request: { provider_id, endpoint, latency_ms, success_rate }
// Response: { acknowledged: true }
func (h *Handler) ReportProviderHealth(w http.ResponseWriter, r *http.Request) {
    // Update provider health metrics in database
}
```

**Database Schema (PostgreSQL):**
```sql
-- migrations/005_marketplace_supply.sql (NEW)
CREATE TABLE IF NOT EXISTS provider_health (
    provider_id     UUID REFERENCES users(id),
    endpoint        VARCHAR(512) NOT NULL,
    model           VARCHAR(128) NOT NULL,
    region          VARCHAR(64) NOT NULL,
    latency_p50_ms  INT NOT NULL,
    latency_p99_ms  INT NOT NULL,
    success_rate    DECIMAL(5,4) NOT NULL,  -- 0.0000-1.0000
    last_check      TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider_id, endpoint, model)
);

CREATE INDEX idx_provider_health_model ON provider_health(model, region);
```

**Tests to Create (Gateway):**
```bash
# tests/integration/marketplace/
test_marketplace_disabled_by_default.sh  # TC-P3-PRIV-001
test_marketplace_opt_in.sh               # TC-P3-PRIV-002
test_data_minimization.sh                # TC-P3-PRIV-003
test_free_tier_quota.sh                  # TC-P3-RATE-001
```

**Tests to Create (Marketplace):**
```bash
# tokligence-marketplace/tests/
test_supply_discovery.sh
test_health_reporting.sh
test_free_tier_enforcement.sh
```

---

## 3. Integration Points

### 3.1 Gateway → Marketplace (Current, v0.3.0)

**已实现的集成:**

| Gateway Component | Marketplace Endpoint | Purpose | Frequency |
|-------------------|---------------------|---------|-----------|
| `internal/telemetry/client.go` | `POST /v1/telemetry` | 上报安装心跳 (匿名) | 每24小时 |
| `internal/client/exchange.go` | `POST /users` | 确保用户账户存在 | 首次启动 |
| `internal/client/exchange.go` | `POST /services` | 发布供应商服务 | `enable_provider=true`时 |
| `internal/client/exchange.go` | `GET /providers` | 查询供应商列表 | 用户选择供应商时 |

**配置 (gateway.ini):**
```ini
[marketplace]
marketplace_enabled = true  # 当前默认true (需改为false)
base_url = "https://marketplace.tokligence.ai"
```

### 3.2 Gateway → Marketplace (Phase 3 新增)

**需要新增的集成:**

| Gateway Component | Marketplace Endpoint | Purpose | Frequency |
|-------------------|---------------------|---------|-----------|
| `provider_marketplace.go` | `POST /v1/marketplace/discover` | 供应发现 (opt-in) | 每次请求或缓存5分钟 |
| `provider_marketplace.go` | `POST /v1/marketplace/health` | 上报提供商健康 | 每60秒 |
| `marketplace_client.go` | `POST /v1/marketplace/usage` | 上报使用量 (计费) | 每次请求完成 |

---

## 4. Testing Strategy by Repository

### 4.1 Gateway Tests (tokligence-gateway/tests/)

**Current Tests (已有36个):**
- `integration/tool_calls/` - 工具调用测试
- `integration/responses_api/` - Responses API测试
- `integration/duplicate_detection/` - 重复检测测试
- `integration/firewall/` - 防火墙测试
- `integration/routing/` - 路由测试

**New Tests for Scheduling (需新增):**
```
tests/integration/scheduling/
├── phase0/
│   ├── test_capacity_limits.sh
│   ├── test_priority_ordering.sh
│   ├── test_local_provider.sh
│   ├── test_token_routing.sh
│   └── test_degradation.sh
├── phase1/
│   ├── test_queue_metrics.sh
│   ├── test_protection_integration.sh
│   ├── test_circuit_breaker.sh
│   └── test_snapshot_cache.sh
├── phase2/
│   ├── test_layered_lookup.sh
│   └── test_fail_modes.sh
└── phase3/
    ├── test_marketplace_disabled.sh
    ├── test_marketplace_opt_in.sh
    ├── test_data_minimization.sh
    └── test_free_tier_quota.sh
```

### 4.2 Marketplace Tests (tokligence-marketplace/tests/)

**Current Tests (MVP已有):**
- 基础账户管理测试
- 服务发布测试
- 使用统计测试

**New Tests for Scheduling (需新增):**
```
tokligence-marketplace/tests/
├── test_supply_discovery.sh
├── test_health_reporting.sh
├── test_free_tier_enforcement.sh
└── test_privacy_compliance.sh
```

---

## 5. Deployment Architecture

### 5.1 Single-Node Deployment (Development/Small Scale)

```
┌─────────────────────────────────────┐
│      Single Server (Docker)         │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  tokligence-gateway:8081     │  │
│  │  - LocalProvider             │  │
│  │  - Priority Scheduler        │  │
│  │  - Token Routing             │  │
│  └──────────────────────────────┘  │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  PostgreSQL:5432             │  │
│  │  - token_metadata            │  │
│  │  - identity store            │  │
│  └──────────────────────────────┘  │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  Redis:6379                  │  │
│  │  - Token cache               │  │
│  └──────────────────────────────┘  │
└─────────────────────────────────────┘
```

**Docker Compose:**
```yaml
# docker-compose.yml (gateway repo)
services:
  gateway:
    image: tokligence/gateway:latest
    ports:
      - "8081:8081"
    environment:
      - TOKLIGENCE_SCHEDULING_ENABLED=true
      - TOKLIGENCE_PROVIDER_LOCAL_ENABLED=true
      - TOKLIGENCE_MARKETPLACE_ENABLED=false  # Default
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=tokligence

  redis:
    image: redis:7-alpine
```

### 5.2 Multi-Node Deployment (Production)

```
                    ┌─────────────────────┐
                    │  Load Balancer      │
                    │  (nginx/HAProxy)    │
                    └─────────┬───────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
    ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
    │  Gateway #1 │ │  Gateway #2 │ │  Gateway #3 │
    │  :8081      │ │  :8081      │ │  :8081      │
    └─────┬───────┘ └─────┬───────┘ └─────┬───────┘
          │               │               │
          └───────────────┼───────────────┘
                          │
              ┌───────────┼───────────────┐
              ▼           ▼               ▼
    ┌─────────────┐ ┌──────────┐ ┌──────────────────┐
    │  PostgreSQL │ │  Redis   │ │  Marketplace API │
    │  (primary)  │ │ Cluster  │ │  (optional)      │
    └─────────────┘ └──────────┘ └──────────────────┘
```

---

## 6. Configuration Management

### 6.1 Gateway Configuration Layers

**3-layer override system:**

1. **Global defaults:** `config/setting.ini`
2. **Environment-specific:** `config/{dev,test,live}/gateway.ini`
3. **Environment variables:** `TOKLIGENCE_*` (highest priority)

**Example:**
```bash
# Override in production
export TOKLIGENCE_SCHEDULING_ENABLED=true
export TOKLIGENCE_PROVIDER_LOCAL_MAX_TOKENS_PER_SEC=50000
export TOKLIGENCE_MARKETPLACE_ENABLED=false
```

### 6.2 Marketplace Configuration Layers

**Similar 3-layer system:**

1. **Global defaults:** `config/setting.ini`
2. **Environment-specific:** `config/{dev,test,live}/app.ini`
3. **Environment variables:** `MARKETPLACE_*`

---

## 7. Development Workflow

### 7.1 Local Development (Single Repo)

**Phase 0-2 (只需gateway仓库):**
```bash
cd /home/alejandroseaah/tokligence/sell_dev/tokligence-gateway

# 1. Start dependencies
docker-compose up -d postgres redis

# 2. Run migrations
make db-migrate

# 3. Build and run gateway
make gfr  # Force restart

# 4. Run tests
make test
```

### 7.2 Full Stack Development (Both Repos)

**Phase 3 (需要marketplace仓库):**

**Terminal 1 (Gateway):**
```bash
cd /home/alejandroseaah/tokligence/sell_dev/tokligence-gateway
make gfr
```

**Terminal 2 (Marketplace):**
```bash
cd /home/alejandroseaah/tokligence/tokligence-marketplace
make start
```

**Terminal 3 (Tests):**
```bash
# Test gateway
cd /home/alejandroseaah/tokligence/sell_dev/tokligence-gateway
./tests/integration/marketplace/test_marketplace_opt_in.sh

# Test marketplace
cd /home/alejandroseaah/tokligence/tokligence-marketplace
./tests/test_supply_discovery.sh
```

---

## 8. Summary Table

| Phase | Repo | New Files | Modified Files | Tests |
|-------|------|-----------|----------------|-------|
| **Phase 0** | gateway | 5 files (scheduler, provider_local, degradation) | 3 files (config, responses_handler) | 5 tests |
| **Phase 1** | gateway | 4 files (metrics, circuit_breaker, snapshot_cache) | 2 files (firewall, provider) | 4 tests |
| **Phase 2** | gateway | 4 files (token_lookup, redis_cache, migrations) | 2 files (provider, userstore) | 2 tests |
| **Phase 3** | gateway | 3 files (provider_marketplace, marketplace_client) | 1 file (config) | 4 tests |
| **Phase 3** | marketplace | 2 files (marketplace handler, migrations) | 1 file (router) | 4 tests |
| **Total** | | **18 new files** | **9 modified files** | **19 tests** |

---

## 9. Key Takeaways

1. **Phase 0-2只需要gateway仓库** - 核心调度功能全部在gateway内部实现
2. **Phase 3需要两个仓库** - Marketplace集成涉及跨仓库协作
3. **Marketplace始终是opt-in** - 默认禁用，用户显式启用
4. **清晰的职责划分:**
   - **Gateway:** 调度、路由、协议转换、本地Provider
   - **Marketplace:** 供应发现、计费、遥测、目录服务
5. **独立测试和部署** - 每个仓库有独立的测试套件和Docker镜像
6. **共享数据模型** - Token metadata schema在gateway定义，marketplace通过API访问

---

**Next Steps:**
1. Phase 0-2 implementation: 只需克隆 `tokligence-gateway` 仓库
2. Phase 3 implementation: 同时需要 `tokligence-gateway` 和 `tokligence-marketplace`
3. Integration testing: 使用docker-compose启动完整堆栈

**Repository URLs:**
- Gateway: `/home/alejandroseaah/tokligence/sell_dev/tokligence-gateway`
- Marketplace: `/home/alejandroseaah/tokligence/tokligence-marketplace`
