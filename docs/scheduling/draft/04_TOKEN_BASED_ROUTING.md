# Token-Based Routing and Queue Assignment System

**Version:** 2.0 (With Degradation Strategies & Snapshot Cache)
**Date:** 2025-02-01
**Status:** Design Proposal - Production-Ready with Availability Guarantees
**Updates:**
- v1.1: Added HTTP header-based routing for multi-gateway architectures
- v2.0: Added degradation strategies (fail-open/fail-close), snapshot cache, and availability design

**Related Documents:**
- `00_REVISED_OVERVIEW.md` - Overall scheduling architecture (see degradation strategies)
- `01_PRIORITY_BASED_SCHEDULING.md` - Priority queuing and scheduling algorithms
- `06_MARKETPLACE_INTEGRATION.md` - Provider SPI and degradation patterns
- `arc_design/01_comprehensive_system_architecture.md` - Overall system architecture
- `arc_design/02_user_system_and_access_control.md` - User and access control
- `CLAUDE.md` - Project overview and development guidelines

**v2.0 Critical Additions:**
- ✅ **Snapshot Cache** - Read-only fallback when Redis/PostgreSQL are down
- ✅ **Fail-Open Mode** - Allow requests with strict limits when all stores down
- ✅ **Fail-Close Mode** - Reject all requests when all stores down (safer for compliance)
- ✅ **Layered Degradation** - LRU → Redis → Snapshot → PostgreSQL → Fail-Open/Close
- ✅ **Availability SLA** - System remains operational even with partial infrastructure failure

---

## Executive Summary

This document designs a **database-driven, token-based routing system** for Tokligence Gateway, enabling API token-level control over request scheduling, priority assignment, and quota management. The system supports both internal (multi-environment) and external (customer) workloads through a unified token management framework.

**Core Value Proposition:**
- **For Gateway Operators:** Centralized token management with granular control over priority, quota, and routing rules
- **For Internal Teams:** Environment-specific API tokens (prod/staging/dev) with automatic priority assignment
- **For External Customers:** Self-service token provisioning with SLA-differentiated pricing

**Key Capabilities:**
1. **Database-Driven Token Management** - All tokens stored in PostgreSQL, not hardcoded in YAML
2. **Three-Layer Cache Architecture** - Local LRU → Redis → PostgreSQL for sub-millisecond lookups
3. **HTTP Header-Based Routing** - Multi-gateway architectures can use `X-TGW-Source` header for direct routing (NEW)
4. **Flexible Routing Rules Engine** - Match tokens by tier/environment/account and assign priority/quota dynamically
5. **Hot-Reload Configuration** - Update routing rules without restarting Gateway
6. **Complete Admin APIs** - CRUD operations for tokens, rules, and usage monitoring
7. **Internal vs External Separation** - Automatic capacity protection (e.g., stop external when internal > 90%)

---

## 1. Architecture Overview

### 1.1 System Components

```
┌─────────────────────────────────────────────────────────────────┐
│                      Client Request                              │
│  Headers: Authorization: Bearer tok_internal_prod_xxx           │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              Gateway: Token Lookup (3-Layer Cache)              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │ Local LRU    │→ │ Redis        │→ │ PostgreSQL   │         │
│  │ < 1μs        │  │ < 1ms        │  │ < 10ms       │         │
│  └──────────────┘  └──────────────┘  └──────────────┘         │
│                                                                  │
│  Result: TokenMetadata {                                        │
│    account_id, priority_tier, environment,                      │
│    quota_tps_limit, quota_tokens_per_month                      │
│  }                                                               │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│            Routing Rules Engine (from database)                 │
│  Match: priority_tier + environment + account_type              │
│  Assign: priority, weight, queue, max_tps, timeout             │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│              Capacity Guard (Internal vs External)              │
│  IF token.priority_tier == "external":                          │
│    IF internal_usage >= 90%:                                    │
│      REJECT with 503                                            │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                Priority Queue Assignment                         │
│  Based on: assigned_priority + time_window adjustments         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
                  Request Execution
```

### 1.2 Data Flow

```
1. Request arrives with Bearer token
   │
   ├─> Extract token string
   ├─> SHA256 hash → token_hash
   └─> Lookup: tokenCache.GetTokenMetadata(token_hash)
       │
       ├─> Check Local LRU (10K entries, TTL unlimited)
       │   └─> HIT → return in < 1μs
       │
       ├─> Check Redis (key: "token_meta:{hash}", TTL 5min)
       │   └─> HIT → return in < 1ms
       │
       └─> Check PostgreSQL (api_tokens JOIN accounts)
           └─> Load and cache to Redis + Local → return in < 10ms

2. Validate token
   ├─> Check status (active | expired | revoked)
   ├─> Check expires_at
   └─> IF invalid → return 401/403

3. Match routing rule
   ├─> routingEngine.MatchRule(tokenMeta)
   ├─> Match by: priority_tier, environment, account_type
   └─> Return: assign_priority, assign_weight, max_tps, queue_timeout

4. Apply time-window adjustments (if configured)
   └─> timeWindowManager.ApplyAdjustments(...)

5. Capacity check
   ├─> IF token.priority_tier == "external":
   │   └─> capacityGuard.CanAcceptExternal()
   │       └─> IF internal_tps / max_tps >= 0.90 → REJECT 503
   └─> ELSE (internal): always allow

6. Quota check
   ├─> quotaManager.CheckAndReserve(token_id, estimated_tokens)
   └─> DECRBY quota:{token_id}:month {tokens}

7. Enqueue to priority queue
   └─> scheduler.Enqueue(request, assigned_priority)
```

### 1.3 Header-Based Routing (Multi-Gateway Architecture)

**Purpose:** For multi-tier gateway architectures where upstream gateways (department gateways, external gateways) forward requests to Tokligence Gateway, HTTP headers provide a faster and more convenient routing mechanism than token lookups.

**Use Case Example:**
```
Self-hosted LLM
    ↑
Tokligence Gateway (final gateway)
    ↑
    ├── Internal Department Gateway → X-TGW-Source: internal
    │       ↑
    │       └── Internal services (dev, test, prod)
    │
    └── External API Gateway → X-TGW-Source: external
            ↑
            └── External customers
```

**Routing Priority (High to Low):**
```
1. HTTP Headers (X-TGW-Source, X-TGW-Priority)
   ↓ If header exists, use directly
2. API Token Metadata (database lookup)
   ↓ If no header, lookup token from cache/DB
3. Default Policy (fallback)
   ↓ If no header and no token, use default
```

**Supported Headers:**

| Header | Type | Example Values | Description |
|--------|------|----------------|-------------|
| `X-TGW-Source` | string | `internal`, `external`, `premium`, `spot` | Source tier classification |
| `X-TGW-Priority` | int | `0`-`4` (0=highest) | Explicit priority override |
| `X-TGW-Environment` | string | `production`, `staging`, `dev` | Environment tag |
| `X-TGW-Workload` | string | `realtime`, `batch`, `interactive` | Workload type hint |

**Data Flow with Headers:**

```
┌────────────────────────────────────────────────────┐
│ Request from Department Gateway                    │
│ POST /v1/chat/completions                         │
│ Authorization: Bearer tok_dept_prod_abc123        │
│ X-TGW-Source: internal                            │ ← Header routing
│ X-TGW-Environment: production                     │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
┌────────────────────────────────────────────────────┐
│ Tokligence Gateway: Check routing source           │
│                                                     │
│ IF X-TGW-Source header exists:                    │
│   ├─> Validate source IP is trusted              │
│   ├─> Route by header (fast path)                │
│   └─> Still extract token for billing/audit      │
│                                                     │
│ ELSE:                                              │
│   └─> Route by token lookup (standard path)      │
└──────────────────┬─────────────────────────────────┘
                   │
                   ▼
           Queue Assignment
```

**Implementation Preview:**

```go
func (rr *RequestRouter) RouteRequest(r *http.Request) (*RoutingDecision, error) {
    // Priority 1: Check HTTP header
    if source := r.Header.Get("X-TGW-Source"); source != "" {
        // Security: only trusted IPs can use header routing
        if !rr.config.IsTrustedSource(r) {
            return nil, ErrHeaderRoutingNotAllowed
        }
        return rr.routeByHeader(source, r)
    }

    // Priority 2: Check API token
    token := extractBearerToken(r)
    if token != "" {
        return rr.routeByToken(token)
    }

    // Priority 3: Default policy
    return rr.defaultRoute(), nil
}

func (rr *RequestRouter) routeByHeader(source string, r *http.Request) (*RoutingDecision, error) {
    decision := &RoutingDecision{}

    switch source {
    case "internal":
        decision.PriorityTier = "internal"
        decision.Priority = 0  // P0 - highest
        decision.QueueName = "queue_internal"

    case "external":
        // Capacity protection: reject if internal >= 90%
        if !rr.capacityGuard.CanAcceptExternal() {
            return nil, ErrCapacityLimitReached
        }
        decision.PriorityTier = "external"
        decision.Priority = 3  // P3 - lower priority
        decision.QueueName = "queue_external"

    case "premium":
        decision.PriorityTier = "premium"
        decision.Priority = 1  // P1 - high priority

    case "spot":
        decision.PriorityTier = "spot"
        decision.Priority = 4  // P4 - lowest priority (preemptible)
    }

    // Optional: explicit priority override
    if priorityStr := r.Header.Get("X-TGW-Priority"); priorityStr != "" {
        if priority, err := strconv.Atoi(priorityStr); err == nil {
            decision.Priority = priority
        }
    }

    // Optional: environment tag
    if env := r.Header.Get("X-TGW-Environment"); env != "" {
        decision.Environment = env
        decision.QueueName = fmt.Sprintf("queue_%s_%s", source, env)
    }

    // Still extract token for billing (if present)
    if token := extractBearerToken(r); token != "" {
        decision.TokenHash = sha256Hash(token)
        decision.BillingEnabled = true
    }

    return decision, nil
}
```

**Security Configuration:**

```yaml
# config/gateway.ini
[header_routing]
enabled = true

# Only these IPs can use header-based routing
trusted_cidrs = 10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

# Require valid token even when using header routing
require_token = true  # Recommended for billing/audit

# Log all header routing decisions
audit_log_enabled = true
```

**Benefits:**
- **Performance:** Skip token cache lookup (saves 100μs - 10ms)
- **Simplicity:** Department gateways don't need complex token management
- **Flexibility:** Header + token hybrid allows billing while using header routing
- **Security:** IP-based trust model ensures only authorized gateways can set headers

**See Section 6.3 for complete implementation details.**

---

## 2. Database Schema

### 2.1 Core Tables

```sql
-- ============================================================================
-- Accounts Table
-- ============================================================================
CREATE TABLE accounts (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email             TEXT NOT NULL UNIQUE,
  name              TEXT,
  account_type      TEXT NOT NULL,  -- internal | external
  organization_id   UUID,            -- Optional: parent organization

  status            TEXT NOT NULL DEFAULT 'active',  -- active | suspended | deleted

  metadata          JSONB,           -- Flexible metadata storage

  created_at        TIMESTAMPTZ DEFAULT now(),
  updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_accounts_type ON accounts(account_type, status);
CREATE INDEX idx_accounts_org ON accounts(organization_id);
CREATE INDEX idx_accounts_email ON accounts(email) WHERE status = 'active';

COMMENT ON TABLE accounts IS 'User and organization accounts';
COMMENT ON COLUMN accounts.account_type IS 'internal = company internal use, external = paying customers';

-- ============================================================================
-- API Tokens Table (Core of routing system)
-- ============================================================================
CREATE TABLE api_tokens (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  -- Token storage (security)
  token_hash        TEXT NOT NULL UNIQUE,  -- SHA256(token), never store plaintext
  token_prefix      TEXT NOT NULL,          -- Display prefix, e.g., "tok_internal_prod_****"

  -- Ownership
  account_id        UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,

  -- Routing Configuration (determines queue assignment)
  priority_tier     TEXT NOT NULL,          -- internal | external | premium | spot
  environment       TEXT,                    -- production | staging | dev | test | uat
  workload_tag      TEXT,                    -- batch | realtime | experimental (optional)

  -- Priority Overrides (optional, overrides routing rules)
  priority_override INT CHECK (priority_override BETWEEN 0 AND 4),  -- NULL = use rules
  weight_override   NUMERIC CHECK (weight_override > 0),             -- NULL = use rules

  -- Quota Configuration (per token)
  quota_tokens_per_month BIGINT,            -- Monthly token limit, NULL = unlimited
  quota_tps_limit        INT,                -- Tokens per second limit, NULL = unlimited
  quota_reset_day        INT DEFAULT 1 CHECK (quota_reset_day BETWEEN 1 AND 28),

  -- Metadata
  name              TEXT,                    -- Human-readable name, e.g., "Production API Key"
  description       TEXT,                    -- Optional description

  -- Usage tracking
  last_used_at      TIMESTAMPTZ,
  total_requests    BIGINT DEFAULT 0,

  -- Lifecycle
  expires_at        TIMESTAMPTZ,            -- NULL = never expires
  revoked_at        TIMESTAMPTZ,            -- NULL = active
  revoked_reason    TEXT,

  created_at        TIMESTAMPTZ DEFAULT now(),
  updated_at        TIMESTAMPTZ DEFAULT now()
);

-- Indexes for fast lookups
CREATE UNIQUE INDEX idx_api_tokens_hash ON api_tokens(token_hash);
CREATE INDEX idx_api_tokens_account ON api_tokens(account_id);
CREATE INDEX idx_api_tokens_tier_env ON api_tokens(priority_tier, environment);

-- Active tokens only (most common query)
CREATE INDEX idx_api_tokens_active ON api_tokens(account_id, revoked_at)
  WHERE revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now());

-- Expiring soon (for cleanup jobs)
CREATE INDEX idx_api_tokens_expiring ON api_tokens(expires_at)
  WHERE expires_at IS NOT NULL AND revoked_at IS NULL;

COMMENT ON TABLE api_tokens IS 'API tokens for authentication and routing control';
COMMENT ON COLUMN api_tokens.token_hash IS 'SHA256 hash of token, never store plaintext';
COMMENT ON COLUMN api_tokens.priority_tier IS 'Routing tier: internal (company), external (customers), premium, spot';
COMMENT ON COLUMN api_tokens.environment IS 'Environment tag for internal tokens: production, staging, dev';

-- ============================================================================
-- Token Routing Rules Table (Flexible rule engine)
-- ============================================================================
CREATE TABLE token_routing_rules (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  rule_name         TEXT NOT NULL UNIQUE,
  description       TEXT,
  priority_order    INT NOT NULL,           -- Lower = higher priority in matching

  -- Match Conditions (all are optional, NULL = match any)
  match_tier        TEXT,                    -- Match priority_tier
  match_environment TEXT,                    -- Match environment
  match_account_type TEXT,                   -- Match account.account_type
  match_workload_tag TEXT,                   -- Match workload_tag

  -- Scheduling Assignment
  assign_priority   INT NOT NULL CHECK (assign_priority BETWEEN 0 AND 4),
  assign_weight     NUMERIC NOT NULL DEFAULT 1.0 CHECK (assign_weight > 0),
  assign_queue      TEXT,                    -- Optional: explicit queue name

  -- Capacity Limits (per rule)
  max_tps           INT,                     -- Max tokens/sec for this rule
  max_qps           INT,                     -- Max queries/sec for this rule
  max_queue_depth   INT,                     -- Max queue depth
  queue_timeout_sec INT,                     -- Queue timeout in seconds

  -- Quota multipliers (applied to token quotas)
  quota_multiplier  NUMERIC DEFAULT 1.0,     -- Multiply token quota by this

  -- Metadata
  enabled           BOOLEAN DEFAULT true,
  created_at        TIMESTAMPTZ DEFAULT now(),
  updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE UNIQUE INDEX idx_token_routing_rules_name ON token_routing_rules(rule_name);
CREATE INDEX idx_token_routing_rules_priority ON token_routing_rules(priority_order, enabled);
CREATE INDEX idx_token_routing_rules_tier ON token_routing_rules(match_tier, match_environment);

COMMENT ON TABLE token_routing_rules IS 'Flexible rules for mapping tokens to priority/queue/quota';
COMMENT ON COLUMN token_routing_rules.priority_order IS 'Lower number = higher priority in rule matching';
COMMENT ON COLUMN token_routing_rules.assign_priority IS 'Priority level (0-4) to assign to matched tokens';

-- ============================================================================
-- Token Usage Stats Table (Real-time tracking)
-- ============================================================================
CREATE TABLE token_usage_stats (
  token_id          UUID PRIMARY KEY REFERENCES api_tokens(id) ON DELETE CASCADE,

  -- Current billing period
  period_start      TIMESTAMPTZ NOT NULL,
  period_end        TIMESTAMPTZ NOT NULL,

  -- Aggregated usage
  tokens_used       BIGINT DEFAULT 0,
  requests_count    BIGINT DEFAULT 0,
  tokens_cached     BIGINT DEFAULT 0,       -- Prompt cache hits

  -- Real-time metrics (synced from Redis every minute)
  current_tps       INT DEFAULT 0,
  current_qps       INT DEFAULT 0,
  last_request_at   TIMESTAMPTZ,

  -- Cost tracking (if pricing configured)
  total_cost_usd    NUMERIC(10,4) DEFAULT 0,

  updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_token_usage_stats_period ON token_usage_stats(period_start, period_end);

COMMENT ON TABLE token_usage_stats IS 'Real-time usage statistics per token, synced from Redis';

-- ============================================================================
-- Token Usage History Table (Detailed records)
-- ============================================================================
CREATE TABLE token_usage_history (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_id          UUID NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
  account_id        UUID NOT NULL REFERENCES accounts(id),

  -- Request details
  request_id        TEXT NOT NULL,
  model             TEXT NOT NULL,

  -- Token counts
  prompt_tokens     BIGINT NOT NULL,
  completion_tokens BIGINT NOT NULL,
  cached_tokens     BIGINT DEFAULT 0,

  -- Scheduling info
  priority_assigned INT,
  queue_wait_ms     INT,

  -- Cost
  cost_usd          NUMERIC(10,6),

  timestamp         TIMESTAMPTZ DEFAULT now()
);

-- Partition by month for efficient querying
CREATE INDEX idx_token_usage_history_token_time ON token_usage_history(token_id, timestamp DESC);
CREATE INDEX idx_token_usage_history_account_time ON token_usage_history(account_id, timestamp DESC);

-- Partition table by month (example for PostgreSQL 12+)
-- CREATE TABLE token_usage_history_2025_01 PARTITION OF token_usage_history
--   FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

COMMENT ON TABLE token_usage_history IS 'Detailed usage records per request, partitioned by month';
```

### 2.2 Initial Data: E-commerce Company Example

```sql
-- ============================================================================
-- Sample Data: E-commerce Company with Internal + External Tokens
-- ============================================================================

-- 1. Create internal organization account
INSERT INTO accounts (id, email, name, account_type, status)
VALUES
  ('00000000-0000-0000-0000-000000000001',
   'internal@ecommerce-company.com',
   'E-commerce Company (Internal)',
   'internal',
   'active');

-- 2. Create internal API tokens (production, staging, dev)
INSERT INTO api_tokens (
  token_hash, token_prefix, account_id,
  priority_tier, environment, name,
  quota_tokens_per_month, quota_tps_limit
)
VALUES
  -- Production token
  (encode(sha256('tok_internal_prod_secret_abc123'::bytea), 'hex'),
   'tok_internal_prod_****',
   '00000000-0000-0000-0000-000000000001',
   'internal', 'production', 'Production Environment API Key',
   NULL, 300),  -- Unlimited monthly tokens, max 300 TPS

  -- Staging token
  (encode(sha256('tok_internal_staging_secret_def456'::bytea), 'hex'),
   'tok_internal_staging_****',
   '00000000-0000-0000-0000-000000000001',
   'internal', 'staging', 'Staging Environment API Key',
   NULL, 200),  -- Unlimited monthly tokens, max 200 TPS

  -- Dev token
  (encode(sha256('tok_internal_dev_secret_ghi789'::bytea), 'hex'),
   'tok_internal_dev_****',
   '00000000-0000-0000-0000-000000000001',
   'internal', 'dev', 'Development Environment API Key',
   NULL, 100);  -- Unlimited monthly tokens, max 100 TPS

-- 3. Create external customer accounts
INSERT INTO accounts (email, name, account_type, status)
VALUES
  ('customer1@external-company.com', 'External Customer 1', 'external', 'active'),
  ('customer2@startup.io', 'External Customer 2 (Startup)', 'external', 'active');

-- 4. Create external customer tokens
INSERT INTO api_tokens (
  token_hash, token_prefix, account_id,
  priority_tier, name,
  quota_tokens_per_month, quota_tps_limit,
  expires_at
)
SELECT
  encode(sha256('tok_customer1_secret_jkl012'::bytea), 'hex'),
  'tok_customer1_****',
  id,
  'external',
  'Customer 1 Production API Key',
  10000000,  -- 10M tokens/month
  50,        -- Max 50 TPS
  '2025-12-31 23:59:59'::timestamptz
FROM accounts WHERE email = 'customer1@external-company.com'

UNION ALL

SELECT
  encode(sha256('tok_customer2_secret_mno345'::bytea), 'hex'),
  'tok_customer2_****',
  id,
  'external',
  'Customer 2 Startup API Key',
  5000000,   -- 5M tokens/month
  30,        -- Max 30 TPS
  '2025-12-31 23:59:59'::timestamptz
FROM accounts WHERE email = 'customer2@startup.io';

-- 5. Create routing rules
INSERT INTO token_routing_rules (
  rule_name, description, priority_order,
  match_tier, match_environment,
  assign_priority, assign_weight,
  max_tps, max_queue_depth, queue_timeout_sec
)
VALUES
  -- Internal rules (highest priority)
  ('Internal Production',
   'Production environment - highest priority',
   10,
   'internal', 'production',
   0, 20.0,  -- Priority 0, weight 20x
   300, 500, 30),

  ('Internal Staging',
   'Staging environment - high priority',
   20,
   'internal', 'staging',
   1, 5.0,   -- Priority 1, weight 5x
   200, 300, 60),

  ('Internal Dev',
   'Development environment - medium priority',
   30,
   'internal', 'dev',
   2, 3.0,   -- Priority 2, weight 3x
   100, 200, 120),

  -- External rules (lower priority)
  ('External Standard',
   'External customers - standard tier',
   40,
   'external', NULL,
   3, 1.0,   -- Priority 3, weight 1x
   100, 1000, 300);

-- 6. Initialize usage stats for all tokens
INSERT INTO token_usage_stats (token_id, period_start, period_end)
SELECT
  id,
  date_trunc('month', now()),
  date_trunc('month', now()) + interval '1 month'
FROM api_tokens
WHERE revoked_at IS NULL;
```

---

## 3. Token Cache Implementation

### 3.1 Three-Layer Cache Architecture

```go
// internal/tokenstore/cache.go

package tokenstore

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "time"

    "github.com/go-redis/redis/v8"
    lru "github.com/hashicorp/golang-lru/v2"
)

// TokenCache implements a three-layer caching strategy:
// 1. Local LRU (in-memory, no TTL) - fastest
// 2. Redis (shared across instances, 5min TTL) - fast
// 3. PostgreSQL (source of truth) - fallback
type TokenCache struct {
    // Layer 1: Local in-memory cache
    localCache *lru.Cache[string, *TokenMetadata]

    // Layer 2: Redis shared cache
    redis *redis.Client

    // Layer 3: PostgreSQL database
    db *sql.DB

    // Configuration
    localCacheSize int
    redisTTL       time.Duration
    syncInterval   time.Duration
}

// TokenMetadata contains all information needed for routing and quota decisions
type TokenMetadata struct {
    // Identity
    TokenID         string    `json:"token_id"`
    AccountID       string    `json:"account_id"`
    AccountType     string    `json:"account_type"`  // internal | external

    // Routing configuration
    PriorityTier    string    `json:"priority_tier"`  // internal | external | premium | spot
    Environment     string    `json:"environment"`    // production | staging | dev
    WorkloadTag     string    `json:"workload_tag"`   // batch | realtime

    // Priority overrides (NULL in DB = use routing rules)
    PriorityOverride *int      `json:"priority_override,omitempty"`
    WeightOverride   *float64  `json:"weight_override,omitempty"`

    // Quota configuration
    QuotaTokensPerMonth int64  `json:"quota_tokens_per_month"`
    QuotaTPSLimit       int    `json:"quota_tps_limit"`
    QuotaResetDay       int    `json:"quota_reset_day"`

    // Status
    Status      string     `json:"status"`      // active | expired | revoked
    ExpiresAt   *time.Time `json:"expires_at,omitempty"`
    RevokedAt   *time.Time `json:"revoked_at,omitempty"`

    // Metadata
    Name        string     `json:"name"`
    LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

// NewTokenCache creates a new token cache instance
func NewTokenCache(redis *redis.Client, db *sql.DB) (*TokenCache, error) {
    localCache, err := lru.New[string, *TokenMetadata](10000)  // 10K entries
    if err != nil {
        return nil, err
    }

    return &TokenCache{
        localCache:     localCache,
        redis:          redis,
        db:             db,
        localCacheSize: 10000,
        redisTTL:       5 * time.Minute,
        syncInterval:   5 * time.Minute,
    }, nil
}

// GetTokenMetadata retrieves token metadata with three-layer fallback
func (tc *TokenCache) GetTokenMetadata(tokenHash string) (*TokenMetadata, error) {
    // Layer 1: Check local LRU cache
    if meta, ok := tc.localCache.Get(tokenHash); ok {
        return meta, nil
    }

    // Layer 2: Check Redis
    ctx := context.Background()
    redisKey := fmt.Sprintf("token_meta:%s", tokenHash)

    data, err := tc.redis.Get(ctx, redisKey).Result()
    if err == nil {
        // Redis hit - parse and cache locally
        var meta TokenMetadata
        if err := json.Unmarshal([]byte(data), &meta); err == nil {
            tc.localCache.Add(tokenHash, &meta)
            return &meta, nil
        }
    }

    // Layer 3: Check PostgreSQL (cache miss)
    return tc.loadFromDBAndCache(tokenHash)
}

// loadFromDBAndCache queries PostgreSQL and populates both caches
func (tc *TokenCache) loadFromDBAndCache(tokenHash string) (*TokenMetadata, error) {
    var meta TokenMetadata
    var expiresAt, revokedAt sql.NullTime

    err := tc.db.QueryRow(`
        SELECT
            t.id, t.account_id, a.account_type,
            t.priority_tier,
            COALESCE(t.environment, ''),
            COALESCE(t.workload_tag, ''),
            t.priority_override, t.weight_override,
            COALESCE(t.quota_tokens_per_month, 0),
            COALESCE(t.quota_tps_limit, 0),
            t.quota_reset_day,
            t.name,
            t.last_used_at,
            t.expires_at,
            t.revoked_at,
            CASE
                WHEN t.revoked_at IS NOT NULL THEN 'revoked'
                WHEN t.expires_at IS NOT NULL AND t.expires_at < now() THEN 'expired'
                ELSE 'active'
            END as status
        FROM api_tokens t
        JOIN accounts a ON t.account_id = a.id
        WHERE t.token_hash = $1
    `, tokenHash).Scan(
        &meta.TokenID, &meta.AccountID, &meta.AccountType,
        &meta.PriorityTier, &meta.Environment, &meta.WorkloadTag,
        &meta.PriorityOverride, &meta.WeightOverride,
        &meta.QuotaTokensPerMonth, &meta.QuotaTPSLimit, &meta.QuotaResetDay,
        &meta.Name, &meta.LastUsedAt,
        &expiresAt, &revokedAt,
        &meta.Status,
    )

    if err != nil {
        return nil, fmt.Errorf("token not found: %w", err)
    }

    // Convert nullable times
    if expiresAt.Valid {
        meta.ExpiresAt = &expiresAt.Time
    }
    if revokedAt.Valid {
        meta.RevokedAt = &revokedAt.Time
    }

    // Cache to Redis (Layer 2)
    data, _ := json.Marshal(meta)
    tc.redis.Set(context.Background(),
        fmt.Sprintf("token_meta:%s", tokenHash),
        data,
        tc.redisTTL)

    // Cache to Local (Layer 1)
    tc.localCache.Add(tokenHash, &meta)

    return &meta, nil
}

// InvalidateToken removes a token from all cache layers
func (tc *TokenCache) InvalidateToken(tokenHash string) {
    // Remove from local cache
    tc.localCache.Remove(tokenHash)

    // Remove from Redis
    tc.redis.Del(context.Background(), fmt.Sprintf("token_meta:%s", tokenHash))
}

// StartSyncWorker periodically syncs all active tokens from DB to Redis
// This ensures cache is warm and reduces DB load
func (tc *TokenCache) StartSyncWorker() {
    ticker := time.NewTicker(tc.syncInterval)

    go func() {
        // Initial sync
        tc.syncAllTokens()

        // Periodic sync
        for range ticker.C {
            tc.syncAllTokens()
        }
    }()
}

func (tc *TokenCache) syncAllTokens() {
    rows, err := tc.db.Query(`
        SELECT
            t.token_hash,
            t.id, t.account_id, a.account_type,
            t.priority_tier,
            COALESCE(t.environment, ''),
            COALESCE(t.workload_tag, ''),
            t.priority_override, t.weight_override,
            COALESCE(t.quota_tokens_per_month, 0),
            COALESCE(t.quota_tps_limit, 0),
            t.quota_reset_day,
            t.name,
            t.last_used_at,
            t.expires_at,
            t.revoked_at,
            CASE
                WHEN t.revoked_at IS NOT NULL THEN 'revoked'
                WHEN t.expires_at IS NOT NULL AND t.expires_at < now() THEN 'expired'
                ELSE 'active'
            END as status
        FROM api_tokens t
        JOIN accounts a ON t.account_id = a.id
        WHERE t.revoked_at IS NULL
          AND (t.expires_at IS NULL OR t.expires_at > now())
    `)

    if err != nil {
        log.Error("Failed to sync tokens", "error", err)
        return
    }
    defer rows.Close()

    ctx := context.Background()
    pipe := tc.redis.Pipeline()
    count := 0

    for rows.Next() {
        var tokenHash string
        var meta TokenMetadata
        var expiresAt, revokedAt sql.NullTime

        rows.Scan(
            &tokenHash,
            &meta.TokenID, &meta.AccountID, &meta.AccountType,
            &meta.PriorityTier, &meta.Environment, &meta.WorkloadTag,
            &meta.PriorityOverride, &meta.WeightOverride,
            &meta.QuotaTokensPerMonth, &meta.QuotaTPSLimit, &meta.QuotaResetDay,
            &meta.Name, &meta.LastUsedAt,
            &expiresAt, &revokedAt,
            &meta.Status,
        )

        if expiresAt.Valid {
            meta.ExpiresAt = &expiresAt.Time
        }
        if revokedAt.Valid {
            meta.RevokedAt = &revokedAt.Time
        }

        // Batch write to Redis
        data, _ := json.Marshal(meta)
        pipe.Set(ctx,
            fmt.Sprintf("token_meta:%s", tokenHash),
            data,
            tc.redisTTL)

        count++

        // Execute batch every 1000 tokens
        if count%1000 == 0 {
            pipe.Exec(ctx)
            pipe = tc.redis.Pipeline()
        }
    }

    // Execute remaining
    if count%1000 != 0 {
        pipe.Exec(ctx)
    }

    log.Info("Token cache synced to Redis", "count", count)
}

// SubscribeToChanges listens to Redis pub/sub for real-time cache invalidation
func (tc *TokenCache) SubscribeToChanges() {
    ctx := context.Background()
    pubsub := tc.redis.Subscribe(ctx, "token_invalidate", "token_revoked")

    go func() {
        for msg := range pubsub.Channel() {
            switch msg.Channel {
            case "token_invalidate", "token_revoked":
                tokenHash := msg.Payload
                tc.InvalidateToken(tokenHash)
                log.Info("Token invalidated from cache", "token_hash", tokenHash[:16]+"...")
            }
        }
    }()
}

// HashToken computes SHA256 hash of a token string
func HashToken(token string) string {
    hash := sha256.Sum256([]byte(token))
    return hex.EncodeToString(hash[:])
}
```

### 3.2 Cache Performance Characteristics

| Layer | Hit Rate | Latency | Capacity | TTL | Shared |
|-------|----------|---------|----------|-----|--------|
| **Local LRU** | ~95% | < 1μs | 10K tokens | Unlimited | No (per-instance) |
| **Redis** | ~99% | < 1ms | Unlimited | 5 min | Yes (all instances) |
| **PostgreSQL** | 100% | < 10ms | Unlimited | N/A | Yes (source of truth) |

**Expected Performance:**
- **Cold start:** 10ms (DB query)
- **Warm cache:** < 1μs (local hit)
- **Cross-instance:** < 1ms (Redis hit)
- **Cache memory:** ~500 bytes/token × 10K = ~5MB per Gateway instance

### 3.3 Degradation Strategies and Availability (v2.0)

**CRITICAL FOR PRODUCTION:** Token routing has a dependency chain (LRU → Redis → PostgreSQL). If Redis or PostgreSQL fails, the gateway must degrade gracefully.

See `00_REVISED_OVERVIEW.md` for full degradation design.

#### 3.3.1 Layered Degradation Flow

```go
// internal/cache/token_cache.go (v2.0)

type TokenCache struct {
    localCache    *LRUCache         // Layer 1: Always available
    redis         *RedisClient      // Layer 2: May fail
    snapshotCache *SnapshotCache    // Layer 2.5: Read-only fallback (NEW)
    db            *PostgresClient   // Layer 3: May fail
    config        *DegradationConfig
}

type DegradationConfig struct {
    EnableSnapshotCache   bool
    SnapshotRefreshInterval time.Duration  // e.g., 1 hour
    FailMode              string          // "fail_open" | "fail_close"
    FailOpenQuotaLimit    int             // Quota limit in fail-open mode
}

func (tc *TokenCache) GetTokenMetadata(hash string) (*TokenMetadata, error) {
    // Layer 1: LRU cache (always available)
    if meta, ok := tc.localCache.Get(hash); ok {
        metrics.TokenCacheHit.WithLabelValues("local").Inc()
        return meta, nil  // < 1μs
    }

    // Layer 2: Redis (may fail)
    meta, err := tc.getFromRedis(hash)
    if err == nil {
        tc.localCache.Set(hash, meta)  // Populate local cache
        metrics.TokenCacheHit.WithLabelValues("redis").Inc()
        return meta, nil  // < 1ms
    }

    // Redis failed - log and continue to fallback
    log.Warn("Redis token lookup failed, trying fallback",
        "error", err,
        "hash", hash[:8])  // Only log first 8 chars for security

    // Layer 2.5: Snapshot cache (read-only fallback) - NEW!
    if tc.config.EnableSnapshotCache {
        if meta, ok := tc.snapshotCache.Get(hash); ok {
            tc.localCache.Set(hash, meta)  // Populate local cache
            metrics.TokenCacheHit.WithLabelValues("snapshot").Inc()
            log.Warn("Using snapshot cache (Redis down)",
                "hash", hash[:8],
                "snapshot_age", time.Since(tc.snapshotCache.LastRefresh))
            return meta, nil  // Degraded but working
        }
    }

    // Layer 3: PostgreSQL (may fail)
    meta, err = tc.getFromDB(hash)
    if err == nil {
        // Populate all caches
        tc.localCache.Set(hash, meta)
        _ = tc.setToRedis(hash, meta)  // Best effort
        if tc.config.EnableSnapshotCache {
            tc.snapshotCache.Set(hash, meta)
        }
        metrics.TokenCacheHit.WithLabelValues("db").Inc()
        return meta, nil  // < 10ms
    }

    // ALL STORES DOWN - choose failure mode
    log.Error("All token stores down (LRU miss + Redis down + DB down)",
        "fail_mode", tc.config.FailMode,
        "hash", hash[:8])

    metrics.TokenCacheMiss.WithLabelValues("all_down").Inc()

    return tc.handleAllStoresDown(hash)
}

func (tc *TokenCache) handleAllStoresDown(hash string) (*TokenMetadata, error) {
    switch tc.config.FailMode {
    case "fail_open":
        // Fail-open: Allow requests with STRICT limits
        log.Error("All token stores down, failing OPEN with strict limits")
        metrics.TokenRoutingDegradation.WithLabelValues("fail_open").Inc()

        return &TokenMetadata{
            TokenHash:    hash,
            PriorityTier: "external",     // Treat as external (lowest priority)
            Priority:     4,               // P4 = lowest
            QuotaLimit:   tc.config.FailOpenQuotaLimit,  // e.g., 100 requests
            Environment:  "unknown",
            AccountID:    "degraded",
        }, nil

    case "fail_close":
        // Fail-close: Reject ALL requests (safer for compliance/security)
        log.Error("All token stores down, failing CLOSED (rejecting request)")
        metrics.TokenRoutingDegradation.WithLabelValues("fail_closed").Inc()

        return nil, ErrAllTokenStoresDown

    default:
        log.Error("Unknown fail mode, defaulting to fail_closed",
            "fail_mode", tc.config.FailMode)
        return nil, ErrAllTokenStoresDown
    }
}

var (
    ErrAllTokenStoresDown = errors.New("all token stores unavailable (Redis and PostgreSQL down)")
)
```

#### 3.3.2 Snapshot Cache Implementation

**Purpose:** Snapshot cache is a **read-only, periodically-refreshed** copy of all active tokens. When Redis and PostgreSQL are both down, gateway can still serve requests using slightly stale data.

```go
// internal/cache/snapshot_cache.go (NEW in v2.0)

package cache

import (
    "sync"
    "time"
)

// SnapshotCache is a read-only cache refreshed periodically from DB
type SnapshotCache struct {
    mu           sync.RWMutex
    data         map[string]*TokenMetadata
    lastRefresh  time.Time
    refreshInterval time.Duration
    db           *PostgresClient
}

func NewSnapshotCache(db *PostgresClient, refreshInterval time.Duration) *SnapshotCache {
    sc := &SnapshotCache{
        data:            make(map[string]*TokenMetadata),
        refreshInterval: refreshInterval,
        db:              db,
    }

    // Start background refresh goroutine
    go sc.backgroundRefresh()

    return sc
}

func (sc *SnapshotCache) Get(hash string) (*TokenMetadata, bool) {
    sc.mu.RLock()
    defer sc.mu.RUnlock()

    meta, ok := sc.data[hash]
    return meta, ok
}

func (sc *SnapshotCache) Set(hash string, meta *TokenMetadata) {
    sc.mu.Lock()
    defer sc.mu.Unlock()

    sc.data[hash] = meta
}

func (sc *SnapshotCache) LastRefresh() time.Time {
    sc.mu.RLock()
    defer sc.mu.RUnlock()
    return sc.lastRefresh
}

func (sc *SnapshotCache) backgroundRefresh() {
    ticker := time.NewTicker(sc.refreshInterval)
    defer ticker.Stop()

    // Initial refresh
    sc.refresh()

    for range ticker.C {
        sc.refresh()
    }
}

func (sc *SnapshotCache) refresh() {
    log.Info("Refreshing snapshot cache from database")

    // Load ALL active tokens from database
    tokens, err := sc.db.GetAllActiveTokens()
    if err != nil {
        log.Error("Failed to refresh snapshot cache", "error", err)
        metrics.SnapshotCacheRefreshErrors.Inc()
        return
    }

    // Replace snapshot data (atomic swap)
    newData := make(map[string]*TokenMetadata, len(tokens))
    for _, token := range tokens {
        hash := hashToken(token.Token)
        newData[hash] = &TokenMetadata{
            TokenHash:    hash,
            AccountID:    token.AccountID,
            PriorityTier: token.PriorityTier,
            Environment:  token.Environment,
            Priority:     token.Priority,
            QuotaLimit:   token.QuotaLimit,
        }
    }

    sc.mu.Lock()
    sc.data = newData
    sc.lastRefresh = time.Now()
    sc.mu.Unlock()

    log.Info("Snapshot cache refreshed",
        "token_count", len(newData),
        "memory_mb", estimateMemoryMB(newData))

    metrics.SnapshotCacheSize.Set(float64(len(newData)))
}

func estimateMemoryMB(data map[string]*TokenMetadata) float64 {
    // Rough estimate: ~500 bytes per token metadata
    bytes := len(data) * 500
    return float64(bytes) / (1024 * 1024)
}
```

#### 3.3.3 Configuration

```ini
# config/gateway.ini

[token_routing]
# Snapshot cache (read-only fallback)
enable_snapshot_cache = true
snapshot_refresh_interval = 1h  # Refresh every hour

# Failure mode when all stores are down
fail_mode = fail_open  # or: fail_close

# Fail-open settings
fail_open_quota_limit = 100  # Max requests per token in degraded mode
fail_open_priority = 4       # Lowest priority (P4)

# Cache sizes
local_lru_size = 10000
redis_ttl = 5m

# Health check
redis_health_check_interval = 10s
db_health_check_interval = 30s
```

#### 3.3.4 Degradation Modes Comparison

| Mode | Behavior When All Stores Down | Use Case | Security Risk |
|------|------------------------------|----------|---------------|
| **fail_open** | Allow requests with strict limits (P4, quota=100) | Maximize availability | ⚠️ Medium - unknown tokens can make limited requests |
| **fail_closed** | Reject ALL requests (503) | Maximize security/compliance | ✅ Low - no unauthorized access |

**Recommendation:**
- **Startups/SaaS:** Use `fail_open` (availability > strict security)
- **Finance/Healthcare:** Use `fail_closed` (compliance > availability)
- **E-commerce:** Use `fail_open` during peak (Black Friday), `fail_closed` during maintenance

#### 3.3.5 Observability Metrics

```prometheus
# Token cache hit rates by layer
token_cache_hit_total{layer="local"} 950000     # 95% hit rate
token_cache_hit_total{layer="redis"} 49000      # 4.9% hit rate
token_cache_hit_total{layer="snapshot"} 500     # 0.05% (only when Redis down)
token_cache_hit_total{layer="db"} 500           # 0.05% (cold starts)

# Token cache misses
token_cache_miss_total{reason="all_down"} 10    # Critical - all stores down

# Degradation events
token_routing_degradation_total{mode="fail_open"} 10
token_routing_degradation_total{mode="fail_closed"} 0

# Snapshot cache metrics
snapshot_cache_size 50000                       # Number of tokens in snapshot
snapshot_cache_refresh_errors_total 0           # Refresh failures
snapshot_cache_age_seconds 1800                 # Time since last refresh (30min)
```

**Alerts:**

```yaml
# Alert when snapshot cache is stale
- alert: SnapshotCacheStale
  expr: time() - snapshot_cache_last_refresh_timestamp > 7200  # 2 hours
  for: 5m
  annotations:
    summary: "Snapshot cache hasn't refreshed in >2 hours"

# Alert when degradation mode activates
- alert: TokenRoutingDegradation
  expr: rate(token_routing_degradation_total[5m]) > 0
  for: 1m
  annotations:
    summary: "Token routing is in degraded mode ({{ $labels.mode }})"

# Alert when all stores are down
- alert: AllTokenStoresDown
  expr: rate(token_cache_miss_total{reason="all_down"}[5m]) > 0
  for: 1m
  severity: critical
  annotations:
    summary: "All token stores down (Redis + PostgreSQL)"
```

---

## 4. Routing Rules Engine

### 4.1 Rule Matching Logic

```go
// internal/scheduler/routing_engine.go

package scheduler

import (
    "database/sql"
    "sync"
    "time"
)

type RoutingEngine struct {
    rules          []*RoutingRule
    rulesMu        sync.RWMutex
    db             *sql.DB
    reloadInterval time.Duration
}

type RoutingRule struct {
    ID              string
    RuleName        string
    Description     string
    PriorityOrder   int

    // Match conditions (NULL = match any)
    MatchTier       *string  // priority_tier
    MatchEnvironment *string  // environment
    MatchAccountType *string  // account.account_type
    MatchWorkloadTag *string  // workload_tag

    // Scheduling assignment
    AssignPriority  int
    AssignWeight    float64
    AssignQueue     *string

    // Capacity limits
    MaxTPS          *int
    MaxQPS          *int
    MaxQueueDepth   *int
    QueueTimeoutSec *int

    // Quota multiplier
    QuotaMultiplier float64

    Enabled         bool
}

func NewRoutingEngine(db *sql.DB, reloadInterval time.Duration) *RoutingEngine {
    return &RoutingEngine{
        db:             db,
        reloadInterval: reloadInterval,
    }
}

// Start begins the rule reloading background worker
func (re *RoutingEngine) Start() {
    // Initial load
    re.reloadRules()

    // Periodic reload (hot reload support)
    ticker := time.NewTicker(re.reloadInterval)
    go func() {
        for range ticker.C {
            re.reloadRules()
        }
    }()
}

func (re *RoutingEngine) reloadRules() {
    rows, err := re.db.Query(`
        SELECT
            id, rule_name, description, priority_order,
            match_tier, match_environment, match_account_type, match_workload_tag,
            assign_priority, assign_weight, assign_queue,
            max_tps, max_qps, max_queue_depth, queue_timeout_sec,
            quota_multiplier,
            enabled
        FROM token_routing_rules
        WHERE enabled = true
        ORDER BY priority_order ASC
    `)

    if err != nil {
        log.Error("Failed to reload routing rules", "error", err)
        return
    }
    defer rows.Close()

    newRules := []*RoutingRule{}

    for rows.Next() {
        var rule RoutingRule
        var matchTier, matchEnv, matchAccType, matchWorkload sql.NullString
        var assignQueue sql.NullString
        var maxTPS, maxQPS, maxQueueDepth, queueTimeout sql.NullInt32

        err := rows.Scan(
            &rule.ID, &rule.RuleName, &rule.Description, &rule.PriorityOrder,
            &matchTier, &matchEnv, &matchAccType, &matchWorkload,
            &rule.AssignPriority, &rule.AssignWeight, &assignQueue,
            &maxTPS, &maxQPS, &maxQueueDepth, &queueTimeout,
            &rule.QuotaMultiplier,
            &rule.Enabled,
        )

        if err != nil {
            continue
        }

        // Convert nullable fields
        if matchTier.Valid {
            s := matchTier.String
            rule.MatchTier = &s
        }
        if matchEnv.Valid {
            s := matchEnv.String
            rule.MatchEnvironment = &s
        }
        if matchAccType.Valid {
            s := matchAccType.String
            rule.MatchAccountType = &s
        }
        if matchWorkload.Valid {
            s := matchWorkload.String
            rule.MatchWorkloadTag = &s
        }
        if assignQueue.Valid {
            s := assignQueue.String
            rule.AssignQueue = &s
        }
        if maxTPS.Valid {
            i := int(maxTPS.Int32)
            rule.MaxTPS = &i
        }
        if maxQPS.Valid {
            i := int(maxQPS.Int32)
            rule.MaxQPS = &i
        }
        if maxQueueDepth.Valid {
            i := int(maxQueueDepth.Int32)
            rule.MaxQueueDepth = &i
        }
        if queueTimeout.Valid {
            i := int(queueTimeout.Int32)
            rule.QueueTimeoutSec = &i
        }

        newRules = append(newRules, &rule)
    }

    re.rulesMu.Lock()
    re.rules = newRules
    re.rulesMu.Unlock()

    log.Info("Routing rules reloaded", "count", len(newRules))

    // Emit metric
    metrics.RoutingRulesLoaded.Set(float64(len(newRules)))
}

// MatchRule finds the first matching rule for given token metadata
// Rules are evaluated in priority_order (lower = higher priority)
func (re *RoutingEngine) MatchRule(meta *tokenstore.TokenMetadata) *RoutingRule {
    re.rulesMu.RLock()
    defer re.rulesMu.RUnlock()

    for _, rule := range re.rules {
        if !rule.Enabled {
            continue
        }

        // Check match_tier
        if rule.MatchTier != nil && *rule.MatchTier != meta.PriorityTier {
            continue
        }

        // Check match_environment
        if rule.MatchEnvironment != nil && *rule.MatchEnvironment != meta.Environment {
            continue
        }

        // Check match_account_type
        if rule.MatchAccountType != nil && *rule.MatchAccountType != meta.AccountType {
            continue
        }

        // Check match_workload_tag
        if rule.MatchWorkloadTag != nil && *rule.MatchWorkloadTag != meta.WorkloadTag {
            continue
        }

        // All conditions matched!
        log.Debug("Routing rule matched",
            "rule", rule.RuleName,
            "tier", meta.PriorityTier,
            "environment", meta.Environment)

        return rule
    }

    // No rule matched - should not happen if default rule exists
    log.Warn("No routing rule matched for token",
        "tier", meta.PriorityTier,
        "environment", meta.Environment)

    return nil
}

// GetEffectivePriority determines final priority, considering token overrides
func GetEffectivePriority(meta *tokenstore.TokenMetadata, rule *RoutingRule) int {
    // Token-level override takes precedence
    if meta.PriorityOverride != nil {
        return *meta.PriorityOverride
    }

    // Otherwise use rule-assigned priority
    if rule != nil {
        return rule.AssignPriority
    }

    // Fallback to default (should never reach here)
    return 3  // Standard priority
}

// GetEffectiveWeight determines final WFQ weight
func GetEffectiveWeight(meta *tokenstore.TokenMetadata, rule *RoutingRule) float64 {
    // Token-level override takes precedence
    if meta.WeightOverride != nil {
        return *meta.WeightOverride
    }

    // Otherwise use rule-assigned weight
    if rule != nil {
        return rule.AssignWeight
    }

    // Fallback to default
    return 1.0
}
```

### 4.2 Rule Evaluation Flow

```
Token Metadata: {
  priority_tier: "internal",
  environment: "production",
  account_type: "internal"
}

Rules (ordered by priority_order):
  [1] match_tier="internal", match_environment="production"
      → MATCH! assign_priority=0, assign_weight=20.0

  [2] match_tier="internal", match_environment="staging"
      → NO MATCH (environment mismatch)

  [3] match_tier="external"
      → NO MATCH (tier mismatch)

Result: Use Rule [1]
  → Priority: 0 (critical)
  → Weight: 20.0 (20x fairness share)
  → Max TPS: 300
  → Queue Timeout: 30 seconds
```

---

## 5. Capacity Guard (Internal vs External Protection)

### 5.1 90% Threshold Implementation

```go
// internal/scheduler/capacity_guard.go

package scheduler

import (
    "sync/atomic"
    "time"
)

// CapacityGuard enforces capacity limits and protects internal workloads
// Key feature: Stops external requests when internal usage >= 90%
type CapacityGuard struct {
    // Global capacity
    maxTPS int  // e.g., 1000 tokens/second

    // Buffer configuration
    bufferPct      float64  // e.g., 0.10 (10%)
    internalLimitPct float64  // e.g., 0.90 (90%)

    // Current usage (atomic counters)
    currentInternalTPS atomic.Int64
    currentExternalTPS atomic.Int64

    // Sliding window for accurate TPS calculation
    internalWindow *SlidingWindow
    externalWindow *SlidingWindow
}

type SlidingWindow struct {
    buckets    []int64
    bucketSize time.Duration
    mu         sync.Mutex
}

func NewCapacityGuard(maxTPS int) *CapacityGuard {
    return &CapacityGuard{
        maxTPS:           maxTPS,
        bufferPct:        0.10,  // 10% buffer
        internalLimitPct: 0.90,  // 90% internal limit
        internalWindow:   NewSlidingWindow(60, 1*time.Second),
        externalWindow:   NewSlidingWindow(60, 1*time.Second),
    }
}

// CanAcceptExternal checks if external requests can be accepted
// Returns false if internal usage >= 90%
func (cg *CapacityGuard) CanAcceptExternal() bool {
    internalTPS := cg.internalWindow.Rate()
    internalUsagePct := float64(internalTPS) / float64(cg.maxTPS)

    // If internal usage >= 90%, reject external
    if internalUsagePct >= cg.internalLimitPct {
        log.Warn("Internal capacity limit reached, rejecting external requests",
            "internal_tps", internalTPS,
            "max_tps", cg.maxTPS,
            "usage_pct", internalUsagePct*100)

        // Emit metric
        metrics.ExternalRequestsBlockedTotal.Inc()

        return false
    }

    return true
}

// TryAcquire attempts to acquire capacity for a request
func (cg *CapacityGuard) TryAcquire(req *Request, meta *tokenstore.TokenMetadata) error {
    isExternal := meta.PriorityTier == "external"

    // External requests: check 90% threshold
    if isExternal {
        if !cg.CanAcceptExternal() {
            return ErrInternalCapacityExhausted
        }
    }

    // Check global capacity (internal + external + buffer <= 100%)
    totalTPS := cg.internalWindow.Rate() + cg.externalWindow.Rate()
    totalUsagePct := float64(totalTPS) / float64(cg.maxTPS)

    if totalUsagePct >= (1.0 - cg.bufferPct) {
        // 90% threshold reached (100% - 10% buffer)
        return ErrCapacityExhausted
    }

    // Acquire capacity
    if isExternal {
        cg.externalWindow.Add(req.EstimatedTokens)
    } else {
        cg.internalWindow.Add(req.EstimatedTokens)
    }

    return nil
}

// Release returns capacity after request completion
func (cg *CapacityGuard) Release(req *Request, meta *tokenstore.TokenMetadata, actualTokens int64) {
    isExternal := meta.PriorityTier == "external"

    // Adjust capacity (actual may differ from estimated)
    delta := actualTokens - req.EstimatedTokens

    if isExternal {
        cg.externalWindow.Add(delta)
    } else {
        cg.internalWindow.Add(delta)
    }
}

// GetStatus returns current capacity status
func (cg *CapacityGuard) GetStatus() map[string]interface{} {
    internalTPS := cg.internalWindow.Rate()
    externalTPS := cg.externalWindow.Rate()
    totalTPS := internalTPS + externalTPS

    bufferTPS := cg.maxTPS - totalTPS

    return map[string]interface{}{
        "max_tps":            cg.maxTPS,
        "internal_tps":       internalTPS,
        "external_tps":       externalTPS,
        "buffer_tps":         bufferTPS,
        "internal_usage_pct": float64(internalTPS) / float64(cg.maxTPS) * 100,
        "total_usage_pct":    float64(totalTPS) / float64(cg.maxTPS) * 100,
        "external_allowed":   cg.CanAcceptExternal(),
    }
}

// SlidingWindow implementation for accurate TPS calculation
func NewSlidingWindow(bucketCount int, bucketSize time.Duration) *SlidingWindow {
    return &SlidingWindow{
        buckets:    make([]int64, bucketCount),
        bucketSize: bucketSize,
    }
}

func (sw *SlidingWindow) Add(tokens int64) {
    sw.mu.Lock()
    defer sw.mu.Unlock()

    now := time.Now()
    bucketIdx := int(now.Unix() / int64(sw.bucketSize.Seconds())) % len(sw.buckets)

    sw.buckets[bucketIdx] += tokens
}

func (sw *SlidingWindow) Rate() int64 {
    sw.mu.Lock()
    defer sw.mu.Unlock()

    now := time.Now()
    currentBucket := int(now.Unix() / int64(sw.bucketSize.Seconds()))

    var sum int64
    for i := 0; i < len(sw.buckets); i++ {
        bucketIdx := (currentBucket - i) % len(sw.buckets)
        if bucketIdx < 0 {
            bucketIdx += len(sw.buckets)
        }
        sum += sw.buckets[bucketIdx]
    }

    // Return tokens per second
    return sum / int64(len(sw.buckets))
}

var (
    ErrInternalCapacityExhausted = errors.New("internal capacity limit reached (>= 90%)")
    ErrCapacityExhausted         = errors.New("total capacity exhausted")
)
```

### 5.2 Capacity Status API

```go
// GET /v1/admin/capacity/status
func (h *Handler) GetCapacityStatus(w http.ResponseWriter, r *http.Request) {
    status := h.capacityGuard.GetStatus()

    // Add time window info
    if activeWindow := h.timeWindowManager.GetActiveWindow(); activeWindow != nil {
        status["active_time_window"] = activeWindow.Name
    }

    json.NewEncoder(w).Encode(status)
}

// Example response:
{
  "max_tps": 1000,
  "internal_tps": 650,
  "external_tps": 80,
  "buffer_tps": 270,
  "internal_usage_pct": 65.0,
  "total_usage_pct": 73.0,
  "external_allowed": true,
  "active_time_window": "business_hours"
}
```

---

## 6. Request Handler Integration

### 6.1 Complete Request Flow

```go
// internal/httpserver/request_handler.go

package httpserver

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "net/http"
    "strings"
)

type RequestHandler struct {
    tokenCache      *tokenstore.TokenCache
    routingEngine   *scheduler.RoutingEngine
    capacityGuard   *scheduler.CapacityGuard
    quotaManager    *scheduler.QuotaManager
    scheduler       *scheduler.Scheduler
    timeWindowMgr   *scheduler.TimeWindowManager
}

func (h *RequestHandler) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
    // ========================================================================
    // Step 1: Extract and validate Bearer token
    // ========================================================================
    authHeader := r.Header.Get("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        http.Error(w, `{"error":"Missing Authorization header"}`, 401)
        return
    }

    tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
    tokenHash := hashToken(tokenStr)

    // ========================================================================
    // Step 2: Lookup token metadata (3-layer cache)
    // ========================================================================
    tokenMeta, err := h.tokenCache.GetTokenMetadata(tokenHash)
    if err != nil {
        log.Warn("Token not found", "error", err, "token_hash", tokenHash[:16]+"...")
        http.Error(w, `{"error":"Invalid API token"}`, 401)
        return
    }

    // Check token status
    if tokenMeta.Status != "active" {
        log.Info("Token not active", "status", tokenMeta.Status, "token_id", tokenMeta.TokenID)
        http.Error(w, fmt.Sprintf(`{"error":"Token is %s"}`, tokenMeta.Status), 403)
        return
    }

    // Update last_used_at (async, don't block request)
    go h.updateTokenLastUsed(tokenMeta.TokenID)

    // ========================================================================
    // Step 3: Parse request body
    // ========================================================================
    var reqBody struct {
        Model    string `json:"model"`
        Messages []struct {
            Role    string `json:"role"`
            Content string `json:"content"`
        } `json:"messages"`
        Stream bool `json:"stream"`
    }

    if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
        http.Error(w, `{"error":"Invalid request body"}`, 400)
        return
    }

    // ========================================================================
    // Step 4: Match routing rule
    // ========================================================================
    rule := h.routingEngine.MatchRule(tokenMeta)
    if rule == nil {
        log.Error("No routing rule matched",
            "tier", tokenMeta.PriorityTier,
            "environment", tokenMeta.Environment)
        http.Error(w, `{"error":"No routing rule configured for this token"}`, 500)
        return
    }

    // ========================================================================
    // Step 5: Determine effective priority and weight
    // ========================================================================
    basePriority := scheduler.GetEffectivePriority(tokenMeta, rule)
    baseWeight := scheduler.GetEffectiveWeight(tokenMeta, rule)

    // Apply time-window adjustments
    adjustedPriority, _, _ := h.timeWindowMgr.ApplyAdjustments(
        "priority_tier",
        tokenMeta.PriorityTier,
        basePriority,
        0, 0,  // quota/capacity handled separately
    )

    // ========================================================================
    // Step 6: Estimate token usage
    // ========================================================================
    estimatedTokens := estimateTokens(reqBody.Messages)

    // ========================================================================
    // Step 7: Capacity check (90% internal protection)
    // ========================================================================
    req := &scheduler.Request{
        ID:              generateRequestID(),
        TokenID:         tokenMeta.TokenID,
        AccountID:       tokenMeta.AccountID,
        PriorityTier:    tokenMeta.PriorityTier,
        Environment:     tokenMeta.Environment,
        Model:           reqBody.Model,
        EstimatedTokens: estimatedTokens,
        Priority:        adjustedPriority,
        Weight:          baseWeight,
        Timestamp:       time.Now(),
    }

    if err := h.capacityGuard.TryAcquire(req, tokenMeta); err != nil {
        if err == scheduler.ErrInternalCapacityExhausted {
            log.Info("External request blocked due to internal capacity limit",
                "token_id", tokenMeta.TokenID,
                "account_id", tokenMeta.AccountID)

            http.Error(w, `{
                "error": "Service temporarily unavailable",
                "message": "Internal capacity limit reached. Please try again later.",
                "retry_after": 60
            }`, 503)
            return
        }

        http.Error(w, `{"error":"Capacity exhausted"}`, 429)
        return
    }

    // Ensure capacity is released on error
    defer func() {
        if err := recover(); err != nil {
            h.capacityGuard.Release(req, tokenMeta, 0)
            panic(err)
        }
    }()

    // ========================================================================
    // Step 8: Quota check
    // ========================================================================
    if err := h.quotaManager.CheckAndReserve(tokenMeta.TokenID, estimatedTokens); err != nil {
        h.capacityGuard.Release(req, tokenMeta, 0)

        log.Info("Quota exceeded",
            "token_id", tokenMeta.TokenID,
            "account_id", tokenMeta.AccountID)

        http.Error(w, `{"error":"Quota exceeded"}`, 429)
        return
    }

    // ========================================================================
    // Step 9: Enqueue to scheduler
    // ========================================================================
    if err := h.scheduler.Enqueue(req); err != nil {
        h.capacityGuard.Release(req, tokenMeta, 0)
        h.quotaManager.Rollback(tokenMeta.TokenID, estimatedTokens)

        http.Error(w, `{"error":"Queue full"}`, 503)
        return
    }

    // ========================================================================
    // Step 10: Execute request (dequeued by scheduler)
    // ========================================================================
    // This would be handled by the scheduler's worker pool
    // See REQUEST_SCHEDULING_DESIGN.md for full implementation

    // For now, simplified execution:
    response, actualTokens := h.executeUpstream(req, reqBody)

    // ========================================================================
    // Step 11: Release capacity and commit quota
    // ========================================================================
    h.capacityGuard.Release(req, tokenMeta, actualTokens)
    h.quotaManager.Commit(tokenMeta.TokenID, actualTokens, estimatedTokens)

    // ========================================================================
    // Step 12: Return response
    // ========================================================================
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("X-Token-ID", tokenMeta.TokenID)
    w.Header().Set("X-Priority", fmt.Sprintf("%d", adjustedPriority))
    w.Header().Set("X-Tokens-Used", fmt.Sprintf("%d", actualTokens))

    json.NewEncoder(w).Encode(response)
}

func hashToken(token string) string {
    hash := sha256.Sum256([]byte(token))
    return hex.EncodeToString(hash[:])
}

func estimateTokens(messages []struct{ Role, Content string }) int64 {
    // Simple estimation: ~4 chars per token
    total := 0
    for _, msg := range messages {
        total += len(msg.Content)
    }
    return int64(total / 4)
}

func generateRequestID() string {
    return fmt.Sprintf("req_%d_%s", time.Now().UnixNano(), randomString(8))
}

func (h *RequestHandler) updateTokenLastUsed(tokenID string) {
    h.db.Exec(`
        UPDATE api_tokens
        SET last_used_at = now(), total_requests = total_requests + 1
        WHERE id = $1
    `, tokenID)
}
```

### 6.2 Header-Based Routing Configuration

```go
// internal/config/header_routing.go

package config

import (
    "net"
    "net/http"
)

type HeaderRoutingConfig struct {
    // Enable header-based routing
    Enabled bool `ini:"enabled"`

    // Trusted IP CIDRs (only these IPs can use header routing)
    TrustedCIDRs []string `ini:"trusted_cidrs,delim:,"`

    // Require valid token even when using header routing
    RequireToken bool `ini:"require_token"`

    // Log all header routing decisions
    AuditLogEnabled bool `ini:"audit_log_enabled"`

    // Parsed CIDR networks
    trustedNetworks []*net.IPNet
}

func (cfg *HeaderRoutingConfig) Init() error {
    cfg.trustedNetworks = make([]*net.IPNet, 0, len(cfg.TrustedCIDRs))

    for _, cidr := range cfg.TrustedCIDRs {
        _, ipnet, err := net.ParseCIDR(cidr)
        if err != nil {
            return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
        }
        cfg.trustedNetworks = append(cfg.trustedNetworks, ipnet)
    }

    return nil
}

func (cfg *HeaderRoutingConfig) IsTrustedIP(ip net.IP) bool {
    for _, network := range cfg.trustedNetworks {
        if network.Contains(ip) {
            return true
        }
    }
    return false
}

func (cfg *HeaderRoutingConfig) IsTrustedRequest(r *http.Request) bool {
    if !cfg.Enabled {
        return false
    }

    clientIP := getClientIP(r)
    return cfg.IsTrustedIP(clientIP)
}
```

**Configuration File Example:**

```ini
# config/dev/gateway.ini

[header_routing]
enabled = true

# Internal network CIDRs
trusted_cidrs = 10.0.0.0/8,172.16.0.0/12,192.168.0.0/16

# Require token for billing/audit
require_token = true

# Log all header routing decisions
audit_log_enabled = true
```

### 6.3 Header-Based Request Handler

```go
// internal/httpserver/request_router.go

package httpserver

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net/http"
    "strconv"
    "strings"
)

const (
    // Header names
    HeaderTGWSource      = "X-TGW-Source"       // internal|external|premium|spot
    HeaderTGWPriority    = "X-TGW-Priority"     // 0-4 (explicit override)
    HeaderTGWEnvironment = "X-TGW-Environment"  // production|staging|dev
    HeaderTGWWorkload    = "X-TGW-Workload"     // realtime|batch|interactive
)

var (
    ErrHeaderRoutingNotAllowed = fmt.Errorf("header routing not allowed from this source")
    ErrInvalidHeaderValue      = fmt.Errorf("invalid header value")
)

type RoutingDecision struct {
    // Source information
    PriorityTier string
    Priority     int
    Environment  string
    WorkloadTag  string
    QueueName    string

    // Token information (for billing/audit)
    TokenHash      string
    TokenID        string
    BillingEnabled bool

    // Routing source
    RoutedBy string // "header" | "token" | "default"
}

type RequestRouter struct {
    config         *config.HeaderRoutingConfig
    tokenCache     *tokenstore.TokenCache
    routingEngine  *scheduler.RoutingEngine
    capacityGuard  *scheduler.CapacityGuard
    auditLogger    *AuditLogger
}

func NewRequestRouter(
    config *config.HeaderRoutingConfig,
    tokenCache *tokenstore.TokenCache,
    routingEngine *scheduler.RoutingEngine,
    capacityGuard *scheduler.CapacityGuard,
) *RequestRouter {
    return &RequestRouter{
        config:        config,
        tokenCache:    tokenCache,
        routingEngine: routingEngine,
        capacityGuard: capacityGuard,
        auditLogger:   NewAuditLogger(),
    }
}

// RouteRequest determines routing based on priority:
// 1. HTTP headers (if enabled and from trusted source)
// 2. API token metadata
// 3. Default policy
func (rr *RequestRouter) RouteRequest(r *http.Request) (*RoutingDecision, error) {
    // ========================================================================
    // Priority 1: Check HTTP headers
    // ========================================================================
    if source := r.Header.Get(HeaderTGWSource); source != "" {
        return rr.routeByHeader(source, r)
    }

    // ========================================================================
    // Priority 2: Check API token
    // ========================================================================
    token := extractBearerToken(r)
    if token != "" {
        return rr.routeByToken(token, r)
    }

    // ========================================================================
    // Priority 3: Default policy
    // ========================================================================
    return rr.defaultRoute(), nil
}

func (rr *RequestRouter) routeByHeader(source string, r *http.Request) (*RoutingDecision, error) {
    // Security check: only trusted IPs can use header routing
    if !rr.config.IsTrustedRequest(r) {
        log.Warn("Header routing attempt from untrusted source",
            "client_ip", getClientIP(r),
            "source_header", source)
        return nil, ErrHeaderRoutingNotAllowed
    }

    decision := &RoutingDecision{
        RoutedBy: "header",
    }

    // Map source to priority tier and priority level
    switch source {
    case "internal":
        decision.PriorityTier = "internal"
        decision.Priority = 0  // P0 - highest priority
        decision.QueueName = "queue_internal"

    case "external":
        // Capacity protection: reject if internal usage >= 90%
        if !rr.capacityGuard.CanAcceptExternal() {
            log.Info("Rejecting external request due to capacity limit",
                "internal_usage_pct", rr.capacityGuard.GetInternalUsagePercent())
            return nil, ErrCapacityLimitReached
        }
        decision.PriorityTier = "external"
        decision.Priority = 3  // P3 - lower priority
        decision.QueueName = "queue_external"

    case "premium":
        decision.PriorityTier = "premium"
        decision.Priority = 1  // P1 - high priority
        decision.QueueName = "queue_premium"

    case "spot":
        decision.PriorityTier = "spot"
        decision.Priority = 4  // P4 - lowest priority (preemptible)
        decision.QueueName = "queue_spot"

    default:
        log.Warn("Unknown X-TGW-Source value", "source", source)
        return nil, fmt.Errorf("%w: unknown source %s", ErrInvalidHeaderValue, source)
    }

    // Optional: explicit priority override via X-TGW-Priority
    if priorityStr := r.Header.Get(HeaderTGWPriority); priorityStr != "" {
        if priority, err := strconv.Atoi(priorityStr); err == nil && priority >= 0 && priority <= 4 {
            log.Debug("Priority override via header",
                "original", decision.Priority,
                "override", priority)
            decision.Priority = priority
        }
    }

    // Optional: environment tag via X-TGW-Environment
    if env := r.Header.Get(HeaderTGWEnvironment); env != "" {
        decision.Environment = env
        decision.QueueName = fmt.Sprintf("queue_%s_%s", decision.PriorityTier, env)
    }

    // Optional: workload type via X-TGW-Workload
    if workload := r.Header.Get(HeaderTGWWorkload); workload != "" {
        decision.WorkloadTag = workload
    }

    // Still extract token for billing/audit (if present and required)
    token := extractBearerToken(r)
    if token != "" {
        tokenHash := sha256Hash(token)

        // If config requires token, validate it
        if rr.config.RequireToken {
            tokenMeta, err := rr.tokenCache.GetTokenMetadata(tokenHash)
            if err != nil {
                log.Warn("Token validation failed for header routing",
                    "error", err,
                    "token_hash", tokenHash[:16]+"...")
                return nil, fmt.Errorf("token validation required but failed: %w", err)
            }

            decision.TokenHash = tokenHash
            decision.TokenID = tokenMeta.TokenID
            decision.BillingEnabled = true
        } else {
            // Optional token for billing
            decision.TokenHash = tokenHash
            decision.BillingEnabled = true
        }
    } else if rr.config.RequireToken {
        return nil, fmt.Errorf("token required for header routing but not provided")
    }

    // Audit log
    if rr.config.AuditLogEnabled {
        rr.auditLogger.LogHeaderRouting(r, decision)
    }

    log.Info("Request routed via header",
        "source", source,
        "priority", decision.Priority,
        "queue", decision.QueueName,
        "environment", decision.Environment,
        "client_ip", getClientIP(r))

    return decision, nil
}

func (rr *RequestRouter) routeByToken(token string, r *http.Request) (*RoutingDecision, error) {
    tokenHash := sha256Hash(token)

    // Lookup token metadata from cache
    tokenMeta, err := rr.tokenCache.GetTokenMetadata(tokenHash)
    if err != nil {
        log.Warn("Token lookup failed", "error", err, "token_hash", tokenHash[:16]+"...")
        return nil, fmt.Errorf("invalid API token: %w", err)
    }

    // Check token status
    if tokenMeta.Status != "active" {
        return nil, fmt.Errorf("token is %s", tokenMeta.Status)
    }

    // Match routing rule
    rule := rr.routingEngine.MatchRule(tokenMeta)
    if rule == nil {
        return nil, fmt.Errorf("no routing rule configured for token tier=%s env=%s",
            tokenMeta.PriorityTier, tokenMeta.Environment)
    }

    decision := &RoutingDecision{
        PriorityTier:   tokenMeta.PriorityTier,
        Priority:       rule.AssignPriority,
        Environment:    tokenMeta.Environment,
        WorkloadTag:    tokenMeta.WorkloadTag,
        QueueName:      fmt.Sprintf("queue_%s_%s", tokenMeta.PriorityTier, tokenMeta.Environment),
        TokenHash:      tokenHash,
        TokenID:        tokenMeta.TokenID,
        BillingEnabled: true,
        RoutedBy:       "token",
    }

    // Apply priority/weight overrides if specified in token metadata
    if tokenMeta.PriorityOverride != nil {
        decision.Priority = *tokenMeta.PriorityOverride
    }

    log.Debug("Request routed via token",
        "token_id", tokenMeta.TokenID,
        "tier", decision.PriorityTier,
        "priority", decision.Priority,
        "queue", decision.QueueName)

    return decision, nil
}

func (rr *RequestRouter) defaultRoute() *RoutingDecision {
    log.Warn("Using default routing policy (no header or token)")

    return &RoutingDecision{
        PriorityTier:   "default",
        Priority:       4,  // Lowest priority
        QueueName:      "queue_default",
        BillingEnabled: false,
        RoutedBy:       "default",
    }
}

// Helper functions

func extractBearerToken(r *http.Request) string {
    authHeader := r.Header.Get("Authorization")
    if !strings.HasPrefix(authHeader, "Bearer ") {
        return ""
    }
    return strings.TrimPrefix(authHeader, "Bearer ")
}

func sha256Hash(input string) string {
    hash := sha256.Sum256([]byte(input))
    return hex.EncodeToString(hash[:])
}

func getClientIP(r *http.Request) net.IP {
    // Check X-Forwarded-For header first
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        ips := strings.Split(xff, ",")
        if len(ips) > 0 {
            ip := strings.TrimSpace(ips[0])
            return net.ParseIP(ip)
        }
    }

    // Check X-Real-IP header
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return net.ParseIP(xri)
    }

    // Fall back to RemoteAddr
    host, _, _ := net.SplitHostPort(r.RemoteAddr)
    return net.ParseIP(host)
}
```

### 6.4 Audit Logging

```go
// internal/httpserver/audit_logger.go

package httpserver

import (
    "encoding/json"
    "time"
)

type AuditLogger struct {
    logger *log.Logger
}

func NewAuditLogger() *AuditLogger {
    return &AuditLogger{
        logger: log.With("component", "audit"),
    }
}

func (al *AuditLogger) LogHeaderRouting(r *http.Request, decision *RoutingDecision) {
    entry := map[string]interface{}{
        "timestamp":    time.Now().UTC().Format(time.RFC3339Nano),
        "routing_type": "header",
        "client_ip":    getClientIP(r).String(),
        "headers": map[string]string{
            "X-TGW-Source":      r.Header.Get(HeaderTGWSource),
            "X-TGW-Priority":    r.Header.Get(HeaderTGWPriority),
            "X-TGW-Environment": r.Header.Get(HeaderTGWEnvironment),
            "X-TGW-Workload":    r.Header.Get(HeaderTGWWorkload),
        },
        "decision": map[string]interface{}{
            "priority_tier": decision.PriorityTier,
            "priority":      decision.Priority,
            "queue":         decision.QueueName,
            "environment":   decision.Environment,
            "workload":      decision.WorkloadTag,
        },
        "has_token": decision.TokenHash != "",
        "token_id":  decision.TokenID,
    }

    data, _ := json.Marshal(entry)
    al.logger.Info("header_routing", "audit_entry", string(data))
}
```

---

## 7. Admin APIs

### 7.1 Token Management APIs

```go
// ============================================================================
// POST /v1/admin/tokens - Create new API token
// ============================================================================
func (h *AdminHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
    var req struct {
        AccountID           string     `json:"account_id"`
        PriorityTier        string     `json:"priority_tier"`
        Environment         string     `json:"environment"`
        WorkloadTag         string     `json:"workload_tag"`
        Name                string     `json:"name"`
        Description         string     `json:"description"`
        QuotaTokensPerMonth int64      `json:"quota_tokens_per_month"`
        QuotaTPSLimit       int        `json:"quota_tps_limit"`
        ExpiresAt           *time.Time `json:"expires_at"`
        PriorityOverride    *int       `json:"priority_override"`
        WeightOverride      *float64   `json:"weight_override"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", 400)
        return
    }

    // Validate priority_tier
    validTiers := map[string]bool{"internal": true, "external": true, "premium": true, "spot": true}
    if !validTiers[req.PriorityTier] {
        http.Error(w, "Invalid priority_tier", 400)
        return
    }

    // Generate secure token
    token := generateSecureToken(req.PriorityTier, req.Environment)
    tokenHash := hashToken(token)
    tokenPrefix := token[:20] + "****"

    // Insert into database
    var tokenID string
    err := h.db.QueryRow(`
        INSERT INTO api_tokens (
            token_hash, token_prefix, account_id,
            priority_tier, environment, workload_tag,
            name, description,
            quota_tokens_per_month, quota_tps_limit,
            priority_override, weight_override,
            expires_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
        RETURNING id
    `, tokenHash, tokenPrefix, req.AccountID,
       req.PriorityTier, req.Environment, req.WorkloadTag,
       req.Name, req.Description,
       req.QuotaTokensPerMonth, req.QuotaTPSLimit,
       req.PriorityOverride, req.WeightOverride,
       req.ExpiresAt).Scan(&tokenID)

    if err != nil {
        log.Error("Failed to create token", "error", err)
        http.Error(w, "Failed to create token", 500)
        return
    }

    // Initialize usage stats
    h.db.Exec(`
        INSERT INTO token_usage_stats (token_id, period_start, period_end)
        VALUES ($1, date_trunc('month', now()), date_trunc('month', now()) + interval '1 month')
    `, tokenID)

    // Warm cache
    h.tokenCache.loadFromDBAndCache(tokenHash)

    // Return token (ONLY SHOWN ONCE!)
    response := map[string]interface{}{
        "token_id": tokenID,
        "token":    token,  // Full token - save it now!
        "message":  "Please save this token securely. It will not be shown again.",
    }

    w.WriteHeader(201)
    json.NewEncoder(w).Encode(response)

    log.Info("Token created",
        "token_id", tokenID,
        "account_id", req.AccountID,
        "tier", req.PriorityTier,
        "environment", req.Environment)
}

// ============================================================================
// GET /v1/admin/tokens - List tokens
// ============================================================================
func (h *AdminHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
    accountID := r.URL.Query().Get("account_id")

    query := `
        SELECT
            id, token_prefix, account_id,
            priority_tier, environment, workload_tag,
            name, description,
            quota_tokens_per_month, quota_tps_limit,
            last_used_at, total_requests,
            expires_at, revoked_at,
            created_at
        FROM api_tokens
        WHERE 1=1
    `

    args := []interface{}{}

    if accountID != "" {
        query += " AND account_id = $1"
        args = append(args, accountID)
    }

    query += " ORDER BY created_at DESC LIMIT 100"

    rows, err := h.db.Query(query, args...)
    if err != nil {
        http.Error(w, "Database error", 500)
        return
    }
    defer rows.Close()

    tokens := []map[string]interface{}{}

    for rows.Next() {
        var (
            id, tokenPrefix, accountID, tier, env, workload, name, desc string
            quotaTokens int64
            quotaTPS, totalRequests int
            lastUsedAt, expiresAt, revokedAt sql.NullTime
            createdAt time.Time
        )

        rows.Scan(
            &id, &tokenPrefix, &accountID,
            &tier, &env, &workload,
            &name, &desc,
            &quotaTokens, &quotaTPS,
            &lastUsedAt, &totalRequests,
            &expiresAt, &revokedAt,
            &createdAt,
        )

        token := map[string]interface{}{
            "id":           id,
            "token_prefix": tokenPrefix,
            "account_id":   accountID,
            "priority_tier": tier,
            "environment":  env,
            "workload_tag": workload,
            "name":         name,
            "description":  desc,
            "quota": map[string]interface{}{
                "tokens_per_month": quotaTokens,
                "tps_limit":        quotaTPS,
            },
            "total_requests": totalRequests,
            "created_at":     createdAt,
        }

        if lastUsedAt.Valid {
            token["last_used_at"] = lastUsedAt.Time
        }
        if expiresAt.Valid {
            token["expires_at"] = expiresAt.Time
        }
        if revokedAt.Valid {
            token["revoked_at"] = revokedAt.Time
            token["status"] = "revoked"
        } else if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
            token["status"] = "expired"
        } else {
            token["status"] = "active"
        }

        tokens = append(tokens, token)
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "tokens": tokens,
        "count":  len(tokens),
    })
}

// ============================================================================
// DELETE /v1/admin/tokens/{token_id} - Revoke token
// ============================================================================
func (h *AdminHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
    tokenID := chi.URLParam(r, "token_id")

    var reason struct {
        Reason string `json:"reason"`
    }
    json.NewDecoder(r.Body).Decode(&reason)

    // Get token hash before revoking (for cache invalidation)
    var tokenHash string
    h.db.QueryRow(`SELECT token_hash FROM api_tokens WHERE id = $1`, tokenID).Scan(&tokenHash)

    // Revoke in database
    result, err := h.db.Exec(`
        UPDATE api_tokens
        SET revoked_at = now(),
            revoked_reason = $2,
            updated_at = now()
        WHERE id = $1 AND revoked_at IS NULL
    `, tokenID, reason.Reason)

    if err != nil {
        http.Error(w, "Failed to revoke token", 500)
        return
    }

    rows, _ := result.RowsAffected()
    if rows == 0 {
        http.Error(w, "Token not found or already revoked", 404)
        return
    }

    // Invalidate cache
    h.tokenCache.InvalidateToken(tokenHash)

    // Notify other gateway instances via Redis pub/sub
    h.redis.Publish(context.Background(), "token_revoked", tokenHash)

    log.Info("Token revoked", "token_id", tokenID, "reason", reason.Reason)

    w.Write([]byte(`{"message":"Token revoked successfully"}`))
}

// ============================================================================
// GET /v1/admin/tokens/{token_id}/usage - Get token usage statistics
// ============================================================================
func (h *AdminHandler) GetTokenUsage(w http.ResponseWriter, r *http.Request) {
    tokenID := chi.URLParam(r, "token_id")

    var stats struct {
        TokenID       string
        PeriodStart   time.Time
        PeriodEnd     time.Time
        TokensUsed    int64
        RequestsCount int64
        CurrentTPS    int
        LastRequestAt *time.Time
        TotalCostUSD  float64
    }

    err := h.db.QueryRow(`
        SELECT
            token_id, period_start, period_end,
            tokens_used, requests_count,
            current_tps, last_request_at,
            total_cost_usd
        FROM token_usage_stats
        WHERE token_id = $1
    `, tokenID).Scan(
        &stats.TokenID, &stats.PeriodStart, &stats.PeriodEnd,
        &stats.TokensUsed, &stats.RequestsCount,
        &stats.CurrentTPS, &stats.LastRequestAt,
        &stats.TotalCostUSD,
    )

    if err != nil {
        http.Error(w, "Token not found", 404)
        return
    }

    // Get quota info
    var quota struct {
        TokensPerMonth int64
        TPSLimit       int
    }

    h.db.QueryRow(`
        SELECT quota_tokens_per_month, quota_tps_limit
        FROM api_tokens
        WHERE id = $1
    `, tokenID).Scan(&quota.TokensPerMonth, &quota.TPSLimit)

    // Calculate usage percentage
    usagePct := 0.0
    if quota.TokensPerMonth > 0 {
        usagePct = float64(stats.TokensUsed) / float64(quota.TokensPerMonth) * 100
    }

    response := map[string]interface{}{
        "token_id":   stats.TokenID,
        "period": map[string]interface{}{
            "start": stats.PeriodStart,
            "end":   stats.PeriodEnd,
        },
        "usage": map[string]interface{}{
            "tokens_used":    stats.TokensUsed,
            "requests_count": stats.RequestsCount,
            "current_tps":    stats.CurrentTPS,
            "total_cost_usd": stats.TotalCostUSD,
        },
        "quota": map[string]interface{}{
            "tokens_per_month": quota.TokensPerMonth,
            "tps_limit":        quota.TPSLimit,
            "usage_pct":        usagePct,
        },
        "last_request_at": stats.LastRequestAt,
    }

    json.NewEncoder(w).Encode(response)
}

func generateSecureToken(tier, environment string) string {
    // Generate cryptographically secure random token
    // Format: tok_{tier}_{environment}_{random32}

    randomBytes := make([]byte, 24)
    rand.Read(randomBytes)
    randomStr := base64.URLEncoding.EncodeToString(randomBytes)

    prefix := "tok"
    if tier != "" {
        prefix += "_" + tier
    }
    if environment != "" {
        prefix += "_" + environment
    }

    return fmt.Sprintf("%s_%s", prefix, randomStr)
}
```

### 7.2 Routing Rules Management APIs

```go
// ============================================================================
// POST /v1/admin/routing-rules - Create routing rule
// ============================================================================
func (h *AdminHandler) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
    var req struct {
        RuleName         string   `json:"rule_name"`
        Description      string   `json:"description"`
        PriorityOrder    int      `json:"priority_order"`
        MatchTier        *string  `json:"match_tier"`
        MatchEnvironment *string  `json:"match_environment"`
        MatchAccountType *string  `json:"match_account_type"`
        AssignPriority   int      `json:"assign_priority"`
        AssignWeight     float64  `json:"assign_weight"`
        MaxTPS           *int     `json:"max_tps"`
        MaxQueueDepth    *int     `json:"max_queue_depth"`
        QueueTimeoutSec  *int     `json:"queue_timeout_sec"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", 400)
        return
    }

    var ruleID string
    err := h.db.QueryRow(`
        INSERT INTO token_routing_rules (
            rule_name, description, priority_order,
            match_tier, match_environment, match_account_type,
            assign_priority, assign_weight,
            max_tps, max_queue_depth, queue_timeout_sec
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id
    `, req.RuleName, req.Description, req.PriorityOrder,
       req.MatchTier, req.MatchEnvironment, req.MatchAccountType,
       req.AssignPriority, req.AssignWeight,
       req.MaxTPS, req.MaxQueueDepth, req.QueueTimeoutSec).Scan(&ruleID)

    if err != nil {
        http.Error(w, "Failed to create rule", 500)
        return
    }

    // Routing engine will auto-reload within 5 minutes
    // Or trigger immediate reload
    h.routingEngine.reloadRules()

    w.WriteHeader(201)
    json.NewEncoder(w).Encode(map[string]string{
        "rule_id": ruleID,
        "message": "Rule created successfully",
    })
}

// ============================================================================
// GET /v1/admin/routing-rules - List routing rules
// ============================================================================
func (h *AdminHandler) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
    rows, _ := h.db.Query(`
        SELECT
            id, rule_name, description, priority_order,
            match_tier, match_environment, match_account_type,
            assign_priority, assign_weight,
            max_tps, max_queue_depth, queue_timeout_sec,
            enabled, created_at
        FROM token_routing_rules
        ORDER BY priority_order ASC
    `)
    defer rows.Close()

    rules := []map[string]interface{}{}

    for rows.Next() {
        var (
            id, name, desc string
            priorityOrder, assignPriority int
            assignWeight float64
            matchTier, matchEnv, matchAccType sql.NullString
            maxTPS, maxQueueDepth, queueTimeout sql.NullInt32
            enabled bool
            createdAt time.Time
        )

        rows.Scan(
            &id, &name, &desc, &priorityOrder,
            &matchTier, &matchEnv, &matchAccType,
            &assignPriority, &assignWeight,
            &maxTPS, &maxQueueDepth, &queueTimeout,
            &enabled, &createdAt,
        )

        rule := map[string]interface{}{
            "id":             id,
            "rule_name":      name,
            "description":    desc,
            "priority_order": priorityOrder,
            "assign_priority": assignPriority,
            "assign_weight":  assignWeight,
            "enabled":        enabled,
            "created_at":     createdAt,
        }

        if matchTier.Valid {
            rule["match_tier"] = matchTier.String
        }
        if matchEnv.Valid {
            rule["match_environment"] = matchEnv.String
        }
        if matchAccType.Valid {
            rule["match_account_type"] = matchAccType.String
        }
        if maxTPS.Valid {
            rule["max_tps"] = maxTPS.Int32
        }
        if maxQueueDepth.Valid {
            rule["max_queue_depth"] = maxQueueDepth.Int32
        }
        if queueTimeout.Valid {
            rule["queue_timeout_sec"] = queueTimeout.Int32
        }

        rules = append(rules, rule)
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "rules": rules,
        "count": len(rules),
    })
}

// ============================================================================
// PATCH /v1/admin/routing-rules/{rule_id} - Update routing rule
// ============================================================================
func (h *AdminHandler) UpdateRoutingRule(w http.ResponseWriter, r *http.Request) {
    ruleID := chi.URLParam(r, "rule_id")

    var req struct {
        Enabled         *bool    `json:"enabled"`
        AssignPriority  *int     `json:"assign_priority"`
        AssignWeight    *float64 `json:"assign_weight"`
        MaxTPS          *int     `json:"max_tps"`
        MaxQueueDepth   *int     `json:"max_queue_depth"`
        QueueTimeoutSec *int     `json:"queue_timeout_sec"`
    }

    json.NewDecoder(r.Body).Decode(&req)

    // Build dynamic UPDATE query
    updates := []string{}
    args := []interface{}{ruleID}
    argIdx := 2

    if req.Enabled != nil {
        updates = append(updates, fmt.Sprintf("enabled = $%d", argIdx))
        args = append(args, *req.Enabled)
        argIdx++
    }
    if req.AssignPriority != nil {
        updates = append(updates, fmt.Sprintf("assign_priority = $%d", argIdx))
        args = append(args, *req.AssignPriority)
        argIdx++
    }
    if req.AssignWeight != nil {
        updates = append(updates, fmt.Sprintf("assign_weight = $%d", argIdx))
        args = append(args, *req.AssignWeight)
        argIdx++
    }
    if req.MaxTPS != nil {
        updates = append(updates, fmt.Sprintf("max_tps = $%d", argIdx))
        args = append(args, *req.MaxTPS)
        argIdx++
    }

    if len(updates) == 0 {
        http.Error(w, "No updates provided", 400)
        return
    }

    query := fmt.Sprintf(`
        UPDATE token_routing_rules
        SET %s, updated_at = now()
        WHERE id = $1
    `, strings.Join(updates, ", "))

    _, err := h.db.Exec(query, args...)
    if err != nil {
        http.Error(w, "Failed to update rule", 500)
        return
    }

    // Trigger reload
    h.routingEngine.reloadRules()

    w.Write([]byte(`{"message":"Rule updated successfully"}`))
}
```

---

## 8. E-commerce Company Configuration Example

### 8.1 Complete Setup Script

```bash
#!/bin/bash
# setup_ecommerce_tokens.sh
# Sets up API tokens and routing rules for e-commerce company

API_BASE="http://localhost:8080/v1/admin"
ADMIN_TOKEN="admin_secret_token"

# ============================================================================
# Step 1: Create internal tokens
# ============================================================================
echo "Creating internal tokens..."

# Production token
PROD_TOKEN=$(curl -s -X POST "$API_BASE/tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "00000000-0000-0000-0000-000000000001",
    "priority_tier": "internal",
    "environment": "production",
    "name": "Production API Key",
    "description": "Production environment - highest priority",
    "quota_tps_limit": 300
  }' | jq -r '.token')

echo "Production token: $PROD_TOKEN"

# Staging token
STAGING_TOKEN=$(curl -s -X POST "$API_BASE/tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "00000000-0000-0000-0000-000000000001",
    "priority_tier": "internal",
    "environment": "staging",
    "name": "Staging API Key",
    "quota_tps_limit": 200
  }' | jq -r '.token')

echo "Staging token: $STAGING_TOKEN"

# Dev token
DEV_TOKEN=$(curl -s -X POST "$API_BASE/tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "00000000-0000-0000-0000-000000000001",
    "priority_tier": "internal",
    "environment": "dev",
    "name": "Development API Key",
    "quota_tps_limit": 100
  }' | jq -r '.token')

echo "Dev token: $DEV_TOKEN"

# ============================================================================
# Step 2: Create external customer tokens
# ============================================================================
echo "Creating external customer tokens..."

CUSTOMER1_TOKEN=$(curl -s -X POST "$API_BASE/tokens" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": "customer-1-uuid",
    "priority_tier": "external",
    "name": "Customer 1 Production Key",
    "quota_tokens_per_month": 10000000,
    "quota_tps_limit": 50,
    "expires_at": "2025-12-31T23:59:59Z"
  }' | jq -r '.token')

echo "Customer 1 token: $CUSTOMER1_TOKEN"

# ============================================================================
# Step 3: Save tokens to .env file
# ============================================================================
cat > .env.tokens <<EOF
# E-commerce Company API Tokens
# Generated: $(date)

# Internal Tokens
TOKLIGENCE_PRODUCTION_TOKEN="$PROD_TOKEN"
TOKLIGENCE_STAGING_TOKEN="$STAGING_TOKEN"
TOKLIGENCE_DEV_TOKEN="$DEV_TOKEN"

# External Customer Tokens
CUSTOMER1_TOKEN="$CUSTOMER1_TOKEN"
EOF

echo "Tokens saved to .env.tokens"

# ============================================================================
# Step 4: Test tokens
# ============================================================================
echo "Testing production token..."
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer $PROD_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3-70b",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

echo "Setup complete!"
```

### 8.2 Usage Examples

```bash
# Internal Production (Priority 0, always served first)
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer $TOKLIGENCE_PRODUCTION_TOKEN" \
  -d '{"model":"llama-3-70b","messages":[{"role":"user","content":"Process order #12345"}]}'

# Internal Dev (Priority 2, lower than production)
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer $TOKLIGENCE_DEV_TOKEN" \
  -d '{"model":"llama-3-70b","messages":[{"role":"user","content":"Test query"}]}'

# External Customer (Priority 3, blocked if internal >= 90%)
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer $CUSTOMER1_TOKEN" \
  -d '{"model":"llama-3-70b","messages":[{"role":"user","content":"Customer query"}]}'
```

### 8.3 Monitoring

```bash
# Check capacity status
curl http://localhost:8080/v1/admin/capacity/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Response:
{
  "max_tps": 1000,
  "internal_tps": 650,
  "external_tps": 80,
  "buffer_tps": 270,
  "internal_usage_pct": 65.0,
  "total_usage_pct": 73.0,
  "external_allowed": true,
  "active_time_window": "business_hours"
}

# Get token usage
curl http://localhost:8080/v1/admin/tokens/$TOKEN_ID/usage \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 8.2 Multi-Gateway Architecture Setup

**Scenario:** E-commerce company has multiple gateway layers:
- **Department Gateway** - Internal services (dev, test, prod)
- **External Gateway** - External customers
- **Tokligence Gateway** - Final gateway with self-hosted LLM

```
Internal Services (dev/test/prod)
    ↓
Department Gateway (10.10.1.10)
    ↓ (adds X-TGW-Source: internal)
    ↓
Tokligence Gateway (10.10.1.100)
    ↓
Self-hosted LLM

External Customers
    ↓
External Gateway (10.20.1.10)
    ↓ (adds X-TGW-Source: external)
    ↓
Tokligence Gateway (10.10.1.100)
    ↓
Self-hosted LLM
```

#### Department Gateway Configuration

```yaml
# department-gateway/config.yaml

upstream:
  tokligence_gateway: "http://10.10.1.100:8081"

# Routing configuration
routes:
  - path: "/v1/chat/completions"
    upstream: tokligence_gateway
    headers:
      # Tag all forwarded requests as internal
      X-TGW-Source: "internal"
      X-TGW-Environment: "${SERVICE_ENVIRONMENT}"  # dev|staging|production
      # Preserve original token for billing
      Authorization: "${ORIGINAL_AUTH_HEADER}"

  - path: "/v1/completions"
    upstream: tokligence_gateway
    headers:
      X-TGW-Source: "internal"
      X-TGW-Environment: "${SERVICE_ENVIRONMENT}"
      Authorization: "${ORIGINAL_AUTH_HEADER}"

# Service environment mapping
environment_detection:
  # Map service name to environment
  service_to_env:
    "checkout-service": "production"
    "recommendation-service": "production"
    "analytics-service": "staging"
    "ml-experiment": "dev"
```

**Example curl from internal service through department gateway:**

```bash
# Production service calling department gateway
curl -X POST http://department-gateway.internal:8080/v1/chat/completions \
  -H "Authorization: Bearer tok_dept_prod_abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Analyze this product review"}]
  }'

# Department gateway transforms to:
# POST http://10.10.1.100:8081/v1/chat/completions
# X-TGW-Source: internal
# X-TGW-Environment: production
# Authorization: Bearer tok_dept_prod_abc123

# Tokligence Gateway sees:
# - Header routing: X-TGW-Source=internal → Priority 0 (highest)
# - Token billing: tok_dept_prod_abc123 → account tracking
# - Queue assignment: queue_internal_production
```

#### External Gateway Configuration

```yaml
# external-gateway/config.yaml

upstream:
  tokligence_gateway: "http://10.10.1.100:8081"

# Routing configuration
routes:
  - path: "/api/v1/ai/*"
    upstream: tokligence_gateway
    rewrite: "/v1/chat/completions"
    headers:
      # Tag all forwarded requests as external
      X-TGW-Source: "external"
      # Map customer API key to Tokligence token
      Authorization: "Bearer ${CUSTOMER_TOKEN_MAPPING[${CUSTOMER_API_KEY}]}"

# Customer API key → Tokligence token mapping
# (loaded from database or config)
customer_token_mapping:
  "cust_api_xyz789": "tok_external_customer1_xyz"
  "cust_api_abc456": "tok_external_customer2_abc"

# Rate limiting at external gateway level
rate_limits:
  - customer: "customer1"
    limit: 100  # requests/sec
  - customer: "customer2"
    limit: 50
```

**Example curl from external customer through external gateway:**

```bash
# External customer calling public API
curl -X POST https://api.example.com/api/v1/ai/chat \
  -H "X-API-Key: cust_api_xyz789" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# External gateway transforms to:
# POST http://10.10.1.100:8081/v1/chat/completions
# X-TGW-Source: external
# Authorization: Bearer tok_external_customer1_xyz

# Tokligence Gateway sees:
# - Header routing: X-TGW-Source=external → Priority 3
# - Capacity check: internal usage < 90%? → allow/reject
# - Token billing: tok_external_customer1_xyz → customer account
# - Queue assignment: queue_external
```

#### Tokligence Gateway Configuration

```ini
# config/dev/gateway.ini

[header_routing]
enabled = true

# Trust internal network (department gateway, external gateway)
trusted_cidrs = 10.0.0.0/8,172.16.0.0/12

# Require token for billing/audit
require_token = true

# Log all header routing decisions
audit_log_enabled = true

[capacity_guard]
max_tps = 1000
internal_limit_pct = 0.90  # 90% threshold for internal
buffer_pct = 0.10          # 10% buffer

[time_windows]
enabled = true

# Business hours: internal 80%, external 10%, buffer 10%
[[time_window]]
name = "business_hours"
schedule_type = "cron"
schedule_expr = "0 9-18 * * 1-5"  # 9am-6pm weekdays
duration_sec = 32400  # 9 hours
target_type = "all"
capacity_multiplier = 1.0

# Night hours: internal 30%, external 60%, buffer 10%
[[time_window]]
name = "night_hours"
schedule_type = "cron"
schedule_expr = "0 22-6 * * *"  # 10pm-6am daily
duration_sec = 28800  # 8 hours
target_type = "external"
capacity_multiplier = 6.0  # 6x external capacity (10% → 60%)
```

#### Complete Request Flow Example

**Scenario 1: Internal production request during business hours**

```
1. Production service → Department Gateway
   POST /v1/chat/completions
   Authorization: Bearer tok_dept_prod_abc123

2. Department Gateway → Tokligence Gateway
   POST /v1/chat/completions
   X-TGW-Source: internal
   X-TGW-Environment: production
   Authorization: Bearer tok_dept_prod_abc123

3. Tokligence Gateway processing:
   - Check header: X-TGW-Source=internal → routeByHeader()
   - Validate source IP: 10.10.1.10 ∈ trusted_cidrs ✓
   - Assign: Priority=0, Queue=queue_internal_production
   - Extract token: tok_dept_prod_abc123 → billing enabled
   - Time window: business_hours (9am-6pm) → no adjustment
   - Capacity check: internal can always proceed ✓
   - Enqueue to queue_internal_production (P0)

4. Execute request → Self-hosted LLM

5. Record billing:
   - Token: tok_dept_prod_abc123
   - Account: internal_production
   - Tokens used: 150 prompt + 200 completion
```

**Scenario 2: External request during business hours (internal at 92%)**

```
1. External customer → External Gateway
   POST /api/v1/ai/chat
   X-API-Key: cust_api_xyz789

2. External Gateway → Tokligence Gateway
   POST /v1/chat/completions
   X-TGW-Source: external
   Authorization: Bearer tok_external_customer1_xyz

3. Tokligence Gateway processing:
   - Check header: X-TGW-Source=external → routeByHeader()
   - Validate source IP: 10.20.1.10 ∈ trusted_cidrs ✓
   - Assign: Priority=3, Queue=queue_external
   - Extract token: tok_external_customer1_xyz → billing enabled
   - Capacity check: CanAcceptExternal()?
     → internal_tps=920, max_tps=1000
     → internal_usage_pct=92% >= 90% ✗
   - REJECT with 503 Service Unavailable

4. Response to customer:
   HTTP/1.1 503 Service Unavailable
   {
     "error": "Capacity limit reached",
     "message": "Internal capacity protection active",
     "retry_after": 60
   }
```

**Scenario 3: External request at night (internal at 25%)**

```
1. External customer → External Gateway (22:30)
   POST /api/v1/ai/chat
   X-API-Key: cust_api_xyz789

2. External Gateway → Tokligence Gateway
   POST /v1/chat/completions
   X-TGW-Source: external
   Authorization: Bearer tok_external_customer1_xyz

3. Tokligence Gateway processing:
   - Check header: X-TGW-Source=external → routeByHeader()
   - Validate source IP: 10.20.1.10 ∈ trusted_cidrs ✓
   - Assign: Priority=3, Queue=queue_external
   - Time window: night_hours (10pm-6am) → external capacity 6x
   - Capacity check: CanAcceptExternal()?
     → internal_tps=250, max_tps=1000
     → internal_usage_pct=25% < 90% ✓
   - Enqueue to queue_external (P3)

4. Execute request → Self-hosted LLM ✓

5. Record billing:
   - Token: tok_external_customer1_xyz
   - Account: customer1
   - Tokens used: 100 prompt + 150 completion
   - Price: $0.50 per 1K tokens
   - Charge: (250/1000) * $0.50 = $0.125
```

#### Monitoring & Observability

**Metrics to track for multi-gateway setup:**

```go
// Prometheus metrics
var (
    // Header routing metrics
    headerRoutingTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tgw_header_routing_total",
            Help: "Total requests routed via header",
        },
        []string{"source", "environment", "gateway_ip"},
    )

    // Capacity rejection metrics
    externalRejectedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tgw_external_rejected_total",
            Help: "External requests rejected due to capacity limit",
        },
        []string{"internal_usage_pct_bucket"},  // 90-91, 91-92, 92-93, etc.
    )

    // Gateway-specific billing
    billingByGateway = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tgw_billing_tokens_total",
            Help: "Total tokens billed by gateway source",
        },
        []string{"source_gateway", "account", "tier"},
    )
)
```

**Grafana dashboard queries:**

```promql
# Requests by gateway source
sum(rate(tgw_header_routing_total[5m])) by (source, gateway_ip)

# External rejection rate during business hours
sum(rate(tgw_external_rejected_total[5m]))
/ sum(rate(tgw_header_routing_total{source="external"}[5m]))

# Internal vs external capacity usage
tgw_capacity_internal_tps / tgw_capacity_max_tps
tgw_capacity_external_tps / tgw_capacity_max_tps

# Token billing by account
sum(rate(tgw_billing_tokens_total[1h])) by (account, tier)
```

---

## 9. Performance & Scalability

### 9.1 Expected Performance

| Metric | Target | Notes |
|--------|--------|-------|
| **Token Lookup Latency** | < 1ms P99 | 95% local cache hit |
| **Rule Matching Latency** | < 100μs | In-memory rule evaluation |
| **Total Overhead** | < 2ms P99 | Token lookup + routing + capacity check |
| **Throughput** | 10K RPS | Per Gateway instance |
| **Token Cache Size** | 10K tokens | ~5MB memory per instance |
| **Cache Hit Rate** | > 95% | Local LRU + Redis |

### 9.2 Scaling Considerations

**Horizontal Scaling:**
- Each Gateway instance has independent local cache
- Redis shared across all instances (capacity checks, token revocation)
- PostgreSQL handles writes (token creation, usage stats sync)

**Token Count Scaling:**
- 10K tokens: ~5MB per instance (default)
- 100K tokens: Increase local cache to 50K (25MB)
- 1M tokens: Redis becomes primary cache, increase sync frequency

**Database Optimization:**
- `api_tokens` table: Add hash index on `token_hash` (already done)
- `token_usage_stats`: Partition by month
- `token_usage_history`: Partition by month, archive old data to S3

---

## 10. Security Considerations

### 10.1 Token Security

**Storage:**
- ✅ Never store plaintext tokens in database (only SHA256 hash)
- ✅ Show full token only once during creation
- ✅ Display only prefix in UI (`tok_internal_prod_****`)

**Transmission:**
- ✅ Always use HTTPS in production
- ✅ Tokens transmitted in `Authorization: Bearer` header
- ✅ No tokens in URL query parameters

**Rotation:**
```bash
# Revoke old token
curl -X DELETE http://localhost:8080/v1/admin/tokens/$OLD_TOKEN_ID \
  -d '{"reason":"Scheduled rotation"}'

# Create new token
curl -X POST http://localhost:8080/v1/admin/tokens \
  -d '{"account_id":"...","priority_tier":"internal","environment":"production"}'
```

### 10.2 Cache Invalidation

**Real-time invalidation via Redis Pub/Sub:**
```go
// On token revocation
redis.Publish("token_revoked", tokenHash)

// All Gateway instances subscribe
pubsub.Subscribe("token_revoked")
// → Invalidate local cache immediately
```

**Worst-case latency:**
- Token revoked in DB → Redis pub/sub → All instances invalidate cache
- **Total time:** < 100ms

---

## 11. Migration & Rollout Plan

### 11.1 Phase 1: Database Setup (Week 1)
- [ ] Create tables: `accounts`, `api_tokens`, `token_routing_rules`, `token_usage_stats`
- [ ] Run initial data migration script
- [ ] Set up PostgreSQL indexes
- [ ] Configure Redis for cache layer

### 11.2 Phase 2: Token Cache Implementation (Week 2)
- [ ] Implement `TokenCache` with three-layer architecture
- [ ] Add background sync worker
- [ ] Set up Redis pub/sub for invalidation
- [ ] Load testing: verify < 1ms P99 latency

### 11.3 Phase 3: Routing Engine (Week 3)
- [ ] Implement `RoutingEngine` with rule matching
- [ ] Add hot-reload capability
- [ ] Create default routing rules for e-commerce use case
- [ ] Test rule priority and fallback

### 11.4 Phase 4: Request Handler Integration (Week 4)
- [ ] Integrate token lookup into request handler
- [ ] Add capacity guard with 90% threshold
- [ ] Connect to existing quota manager
- [ ] End-to-end testing

### 11.5 Phase 5: Admin APIs (Week 5)
- [ ] Implement CRUD APIs for tokens
- [ ] Implement CRUD APIs for routing rules
- [ ] Add usage statistics endpoint
- [ ] Create admin dashboard UI

### 11.6 Phase 6: Production Rollout (Week 6)
- [ ] Migrate existing hardcoded tokens to database
- [ ] Enable token-based routing in canary deployment
- [ ] Monitor performance and error rates
- [ ] Full production rollout

---

## 12. Success Criteria

- [ ] **Token Management:** Create, list, revoke tokens via Admin API
- [ ] **Performance:** < 1ms P99 token lookup latency
- [ ] **Capacity Protection:** External requests blocked when internal >= 90%
- [ ] **Hot Reload:** Routing rules update without Gateway restart
- [ ] **Cache Hit Rate:** > 95% local cache hit rate
- [ ] **Scalability:** Support 10K+ active tokens per instance
- [ ] **Security:** No plaintext tokens in database or logs
- [ ] **Observability:** Real-time capacity and usage metrics

---

## 13. References

This design integrates with:
- [REQUEST_SCHEDULING_DESIGN.md](REQUEST_SCHEDULING_DESIGN.md) - Priority queuing and time windows
- [arc_design/01_comprehensive_system_architecture.md](../../arc_design/01_comprehensive_system_architecture.md) - Overall architecture
- [arc_design/02_user_system_and_access_control.md](../../arc_design/02_user_system_and_access_control.md) - User accounts and access

**External References:**
- PostgreSQL Partitioning: https://www.postgresql.org/docs/current/ddl-partitioning.html
- Redis Pub/Sub: https://redis.io/docs/manual/pubsub/
- LRU Cache (Golang): https://github.com/hashicorp/golang-lru

---

**Document Status:** Ready for implementation
**Authors:** Tokligence Architecture Team
**Version:** 1.0
**Last Updated:** 2025-02-01
