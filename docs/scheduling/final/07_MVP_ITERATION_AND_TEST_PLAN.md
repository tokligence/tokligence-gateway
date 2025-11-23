# MVP Iteration and Test Plan

**Version:** 1.0
**Date:** 2025-11-23
**Status:** DRAFT
**Type:** Implementation roadmap with detailed test cases

---

## Table of Contents

1. [Current State Assessment](#1-current-state-assessment)
2. [MVP Iteration Phases](#2-mvp-iteration-phases)
3. [Detailed Test Plans by Phase](#3-detailed-test-plans-by-phase)
4. [Test Infrastructure](#4-test-infrastructure)
5. [Quality Gates](#5-quality-gates)
6. [Rollback Strategy](#6-rollback-strategy)

---

## 1. Current State Assessment

### 1.1 Existing Capabilities (v0.3.0)

**✅ Already Implemented:**
- Protocol translation (OpenAI ↔ Anthropic)
- Responses API with SSE streaming
- Tool call translation and filtering
- Session management (in-memory)
- Duplicate detection (emergency stop at 5 duplicates)
- Token bucket rate limiting (`internal/ratelimit/token_bucket.go`)
- Basic routing by model prefix
- Health checks and metrics
- SQLite/PostgreSQL identity and ledger stores

**⚠️ Partially Implemented:**
- Provider abstraction (`internal/httpserver/responses/provider.go` interface exists)
- Token-based routing (basic, no capacity awareness)
- Observability (metrics exist, but no scheduling-specific metrics)

**❌ Not Yet Implemented (Scheduling v2.0 Features):**
- Multi-dimensional capacity model (tokens/sec, RPS, concurrent, context length)
- Priority-based scheduling with 5-tier system
- Provider SPI (LocalProvider, MarketplaceProvider, HybridProvider)
- Token metadata routing with PostgreSQL/Redis/snapshot cache
- Degradation strategies (snapshot cache, fail-open/fail-closed)
- LLM protection layer integration with scheduling
- Bucket-based scheduling (intentionally deprioritized)
- Marketplace integration (opt-in)

### 1.2 Technical Debt

1. **Session persistence:** Currently in-memory only, lost on restart
2. **Capacity model:** No tokens/sec tracking, RPS-focused
3. **No priority queue:** FIFO only
4. **No provider health tracking:** Circuit breaker exists in design but not implemented
5. **No snapshot cache:** Redis/PostgreSQL failure = hard failure

---

## 2. MVP Iteration Phases

### Phase 0: Foundation (Weeks 1-4)

**Goal:** Implement core scheduling infrastructure without marketplace dependency

**Deliverables:**
1. Multi-dimensional capacity model
2. Priority-based scheduler (5 tiers)
3. LocalProvider implementation
4. Token routing with capacity awareness
5. Basic degradation (LRU → fail-open/fail-closed)

**Success Criteria:**
- 5-tier priority scheduling works end-to-end
- Capacity limits enforced (tokens/sec + RPS)
- Graceful degradation when Redis/PostgreSQL down
- All existing tests still pass (backward compatibility)

---

### Phase 1: Observability & Protection (Weeks 5-6)

**Goal:** Production-ready monitoring and LLM protection integration

**Deliverables:**
1. Scheduling-specific Prometheus metrics
2. LLM protection layer integration
3. Circuit breaker for providers
4. Snapshot cache implementation

**Success Criteria:**
- Prometheus metrics expose priority queue depths, latencies, capacity usage
- LLM protection blocks malicious requests before scheduling
- System operates in degraded mode when all stores down
- P95 latency < 10ms for scheduling decision

---

### Phase 2: Advanced Routing (Weeks 7-8)

**Goal:** Token metadata routing with degradation layers

**Deliverables:**
1. PostgreSQL token metadata store
2. Redis cache layer
3. Snapshot cache (periodic refresh)
4. Fail-open/fail-closed modes

**Success Criteria:**
- Token routing latency: LRU <1μs, Redis <1ms, PostgreSQL <10ms
- Snapshot cache refreshes every 5 minutes
- System survives all-stores-down scenario
- Zero data loss in fail-closed mode

---

### Phase 3: Marketplace Opt-In (Weeks 9-12) - OPTIONAL

**Goal:** Marketplace integration as opt-in feature (disabled by default)

**Deliverables:**
1. MarketplaceProvider implementation
2. Supply discovery API client
3. Multi-dimensional routing (price/latency/throughput)
4. Transaction billing integration (5% commission)
5. Opt-in workflow and documentation

**Success Criteria:**
- Marketplace disabled by default in config
- Opt-in enables marketplace without code changes
- Transaction commission calculated correctly (5%)
- Multi-dimensional supplier selection working
- Privacy policy compliance (no PII sent)

---

## 3. Detailed Test Plans by Phase

### 3.1 Phase 0: Foundation Tests

#### 3.1.1 Multi-Dimensional Capacity Model

**Test Cases:**

**TC-P0-CAP-001: Basic Capacity Limits**
```yaml
Objective: Verify tokens/sec capacity limit enforced
Precondition: LocalProvider configured with max_tokens_per_sec=1000
Steps:
  1. Send 10 requests, each consuming 100 tokens/sec
  2. Send 11th request (would exceed 1000 tokens/sec)
Expected:
  - First 10 requests: HTTP 200, scheduled immediately
  - 11th request: HTTP 429 "Capacity exceeded" or queued
Validation:
  - Prometheus metric: tokligence_capacity_usage{dimension="tokens_per_sec"} = 1.0
  - Response header: X-Tokligence-Queue-Position: 1
```

**TC-P0-CAP-002: Multi-Dimensional Capacity**
```yaml
Objective: Verify tokens/sec, RPS, concurrent all enforced
Precondition:
  max_tokens_per_sec: 1000
  max_rps: 50
  max_concurrent: 10
Steps:
  1. Send 10 concurrent long-running requests (each 50 tokens/sec, 1 RPS)
     → Total: 500 tokens/sec, 10 RPS, 10 concurrent
  2. Send 11th concurrent request
Expected:
  - 11th request queued (concurrent limit hit)
  - Prometheus: tokligence_capacity_usage{dimension="concurrent"} = 1.0
```

**TC-P0-CAP-003: Context Length Limit**
```yaml
Objective: Verify context length capacity check
Precondition: max_context_length: 128000 (Claude Sonnet)
Steps:
  1. Send request with 100K context length → Accept
  2. Send request with 150K context length → Reject or route to different model
Expected:
  - 100K request: Scheduled to claude-3-5-sonnet
  - 150K request: HTTP 400 "Context too long" or routed to claude-opus
```

---

#### 3.1.2 Priority-Based Scheduling

**Test Cases:**

**TC-P0-PRI-001: Priority Ordering (FIFO within tier)**
```yaml
Objective: Higher priority requests scheduled first, FIFO within same tier
Precondition: Queue empty, capacity available
Steps:
  1. Send request A (priority=4, low)
  2. Send request B (priority=1, critical)
  3. Send request C (priority=4, low)
  4. Send request D (priority=2, high)
Expected Scheduling Order: B (p1) → D (p2) → A (p4) → C (p4)
Validation:
  - Check X-Tokligence-Scheduled-Timestamp headers
  - B timestamp < D timestamp < A timestamp < C timestamp
```

**TC-P0-PRI-002: Priority Starvation Prevention**
```yaml
Objective: Low-priority requests eventually scheduled (no infinite starvation)
Precondition: Continuous stream of high-priority requests
Steps:
  1. Send low-priority request A (priority=4) at T=0
  2. Send 100 high-priority requests (priority=1) at T=1 to T=100
  3. Monitor when request A gets scheduled
Expected:
  - Request A scheduled within 5 minutes (configurable starvation_timeout)
  - OR: Implement aging (priority increases over time)
Validation:
  - Prometheus: tokligence_scheduling_starvation_prevented_total > 0
```

**TC-P0-PRI-003: Token-Based Priority Assignment**
```yaml
Objective: Verify priority assigned from token metadata
Precondition:
  - Token "internal-api-key" → priority_tier=1 (critical)
  - Token "external-partner" → priority_tier=4 (low)
Steps:
  1. Request with Authorization: Bearer internal-api-key
  2. Request with Authorization: Bearer external-partner
Expected:
  - Internal request: priority=1, scheduled first
  - External request: priority=4, scheduled later
Validation:
  - Check token metadata lookup latency (should be <1ms from Redis)
```

---

#### 3.1.3 LocalProvider Implementation

**Test Cases:**

**TC-P0-LOC-001: Basic Provider Registration**
```yaml
Objective: LocalProvider registers and provides capacity
Setup:
  gateway.ini:
    [provider.local]
    enabled = true
    models = "gpt-4,claude-3-5-sonnet"
    max_tokens_per_sec = 5000
    max_rps = 100
    max_concurrent = 50
Steps:
  1. Start gateway
  2. Query /v1/models endpoint
Expected:
  - gpt-4 and claude-3-5-sonnet listed
  - Provider: local
  - Capacity: 5000 tokens/sec, 100 RPS, 50 concurrent
```

**TC-P0-LOC-002: Multi-Model Capacity**
```yaml
Objective: Different models have different capacity limits
Setup:
  [provider.local.gpt-4]
  max_tokens_per_sec = 10000

  [provider.local.claude-3-5-sonnet]
  max_tokens_per_sec = 5000
Steps:
  1. Send gpt-4 request consuming 8000 tokens/sec → Accept
  2. Send claude request consuming 8000 tokens/sec → Reject (exceeds 5000)
Expected:
  - GPT-4: Scheduled
  - Claude: HTTP 429 or queued
```

---

#### 3.1.4 Token Routing with Capacity Awareness

**Test Cases:**

**TC-P0-TOK-001: Token Metadata Lookup**
```yaml
Objective: Token hash → priority/quota lookup works
Precondition: PostgreSQL has token metadata
Setup:
  INSERT INTO token_metadata VALUES (
    sha256('sk-test-123'),
    'internal',
    1,  -- priority
    10000,  -- quota_limit (tokens/day)
    1000   -- current_usage
  );
Steps:
  1. Request with Authorization: Bearer sk-test-123
Expected:
  - Priority: 1 (critical)
  - Quota remaining: 9000 tokens
  - Scheduled immediately if capacity available
Validation:
  - Prometheus: tokligence_token_lookup_duration_seconds{layer="postgresql"} < 0.01
```

**TC-P0-TOK-002: Quota Enforcement**
```yaml
Objective: Request rejected when quota exceeded
Precondition: Token has quota_limit=1000, current_usage=950
Steps:
  1. Send request consuming 30 tokens → Accept (usage=980)
  2. Send request consuming 30 tokens → Reject (would exceed 1000)
Expected:
  - First request: HTTP 200
  - Second request: HTTP 429 "Quota exceeded"
  - Response header: X-Tokligence-Quota-Remaining: 20, then 0
```

---

#### 3.1.5 Degradation (Basic)

**Test Cases:**

**TC-P0-DEG-001: Fail-Open Mode**
```yaml
Objective: System continues operating when Redis/PostgreSQL down
Setup:
  degradation_mode = "fail_open"
Steps:
  1. Stop Redis and PostgreSQL
  2. Send request with unknown token
Expected:
  - Request accepted with default priority=4 (low/external)
  - Quota: LIMITED (100 tokens/request, max 1000 tokens/day to prevent abuse)
  - Warning logged: "All token stores down, using fail-open mode"
  - Rate limit: Max 10 RPS per IP during fail-open (abuse prevention)
Validation:
  - Prometheus: tokligence_degradation_mode_active{mode="fail_open"} = 1
Security Note:
  - Fail-open quota MUST be minimal to limit abuse during outages
  - Otherwise conflicts with fail-open guardrails (attackers exploit outages)
```

**TC-P0-DEG-002: Fail-Closed Mode**
```yaml
Objective: System rejects requests when stores down (secure default)
Setup:
  degradation_mode = "fail_closed"
Steps:
  1. Stop Redis and PostgreSQL
  2. Send request
Expected:
  - HTTP 503 "Service temporarily unavailable"
  - Error: "Token validation unavailable"
  - Health check: /health/ready returns 503
```

---

### 3.2 Phase 1: Observability & Protection Tests

#### 3.2.1 Scheduling Metrics

**Test Cases:**

**TC-P1-OBS-001: Queue Depth Metrics**
```yaml
Objective: Prometheus exposes queue depth by priority tier
Setup: Send 10 requests to each priority tier, all queued
Steps:
  1. Fill capacity completely
  2. Send 10 × priority=1, 10 × priority=2, ..., 10 × priority=5
  3. Query Prometheus
Expected Metrics:
  tokligence_scheduling_queue_depth{priority="1"} = 10
  tokligence_scheduling_queue_depth{priority="2"} = 10
  ...
  tokligence_scheduling_queue_depth{priority="5"} = 10
  tokligence_scheduling_queue_depth_total = 50
```

**TC-P1-OBS-002: Scheduling Latency**
```yaml
Objective: Track time from request arrival to scheduling decision
Steps:
  1. Send 1000 requests across all priority tiers
  2. Measure scheduling latency (not LLM inference latency)
Expected:
  - P50 latency < 1ms
  - P95 latency < 10ms
  - P99 latency < 50ms
Prometheus Metrics:
  tokligence_scheduling_latency_seconds_bucket{priority="1", le="0.001"} > 500
  tokligence_scheduling_latency_seconds_bucket{priority="1", le="0.01"} > 950
```

**TC-P1-OBS-003: Capacity Utilization**
```yaml
Objective: Track real-time capacity usage across dimensions
Precondition: max_tokens_per_sec=1000, currently using 650
Expected Metrics:
  tokligence_capacity_limit{dimension="tokens_per_sec", model="gpt-4"} = 1000
  tokligence_capacity_usage{dimension="tokens_per_sec", model="gpt-4"} = 650
  tokligence_capacity_utilization{dimension="tokens_per_sec", model="gpt-4"} = 0.65
```

---

#### 3.2.2 LLM Protection Integration

**Test Cases:**

**TC-P1-PROT-001: Firewall Blocks Before Scheduling**
```yaml
Objective: Malicious requests blocked before consuming scheduling resources
Setup:
  firewall.enabled = true
  firewall.rules = ["block_pii", "block_sql_injection"]
Steps:
  1. Send request with PII (SSN: 123-45-6789)
  2. Send request with SQL injection attempt
Expected:
  - Both requests: HTTP 400 "Request blocked by firewall"
  - Scheduling queue: Not entered
  - Prometheus: tokligence_firewall_blocked_total{rule="block_pii"} = 1
Validation:
  - Scheduling metrics should NOT increment for blocked requests
```

**TC-P1-PROT-002: Rate Limit Before Scheduling**
```yaml
Objective: Rate-limited requests don't consume scheduling slots
Setup:
  rate_limit.enabled = true
  rate_limit.max_rps = 10  # Per-token limit
Steps:
  1. Send 15 requests/sec with same token for 2 seconds (30 total)
Expected:
  - First 10 requests: Scheduled
  - Remaining 5 requests/sec: HTTP 429 "Rate limit exceeded"
  - Queue depth: Only 10 (rate-limited requests not queued)
```

---

#### 3.2.3 Circuit Breaker

**Test Cases:**

**TC-P1-CB-001: Circuit Opens on Failures**
```yaml
Objective: Circuit breaker opens after N consecutive provider failures
Setup:
  circuit_breaker.threshold = 3
  circuit_breaker.timeout = 30s
Steps:
  1. Configure Anthropic API to return 500 errors
  2. Send 3 requests to claude-3-5-sonnet
  3. All fail with 500
  4. Send 4th request
Expected:
  - First 3 requests: HTTP 500 (tried and failed)
  - 4th request: HTTP 503 "Circuit breaker open"
  - No actual API call made for 4th request
Prometheus:
  tokligence_circuit_breaker_state{provider="anthropic"} = 1  # Open
```

**TC-P1-CB-002: Circuit Half-Open Recovery**
```yaml
Objective: Circuit transitions to half-open, then closed on success
Setup: Circuit currently open (from TC-P1-CB-001)
Steps:
  1. Wait 30 seconds (timeout expires)
  2. Fix Anthropic API (return 200)
  3. Send request
Expected:
  - Circuit state: Half-Open
  - Request succeeds → Circuit closes
  - Subsequent requests: Normal operation
Prometheus:
  tokligence_circuit_breaker_state{provider="anthropic"} = 0  # Closed
```

---

#### 3.2.4 Snapshot Cache

**Test Cases:**

**TC-P1-SNAP-001: Snapshot Cache Refresh**
```yaml
Objective: Snapshot cache periodically refreshes from PostgreSQL
Setup:
  snapshot_cache.enabled = true
  snapshot_cache.refresh_interval = 5m
Steps:
  1. Start gateway → Snapshot cache populated
  2. Add new token to PostgreSQL (priority=1)
  3. Wait 5 minutes
  4. Use new token
Expected:
  - Before 5 min: Token not found in snapshot (uses fail-open)
  - After 5 min: Token found in snapshot with priority=1
Prometheus:
  tokligence_snapshot_cache_refresh_total > 0
  tokligence_snapshot_cache_size = <num_tokens>
```

**TC-P1-SNAP-002: Snapshot Cache Degradation**
```yaml
Objective: Snapshot cache used when Redis + PostgreSQL down
Setup:
  snapshot_cache.enabled = true
  Snapshot contains 1000 tokens
Steps:
  1. Stop Redis and PostgreSQL
  2. Send request with token in snapshot
Expected:
  - Request succeeds using snapshot data
  - Priority/quota from snapshot (may be stale)
  - Warning: "Using snapshot cache (stores down)"
Prometheus:
  tokligence_token_lookup_duration_seconds{layer="snapshot"} < 0.000001  # <1μs
```

---

### 3.3 Phase 2: Advanced Routing Tests

#### 3.3.1 Layered Token Lookup

**Test Cases:**

**TC-P2-LAYER-001: LRU Cache Hit**
```yaml
Objective: LRU cache serves hot tokens in <1μs
Setup:
  LRU cache size: 1000 entries
  Token "sk-hot-token" in LRU cache
Steps:
  1. Send 1000 requests with sk-hot-token
  2. Measure lookup latency
Expected:
  - All 1000 requests: LRU cache hit
  - P99 lookup latency < 1μs
Prometheus:
  tokligence_token_lookup_total{layer="lru", result="hit"} = 1000
```

**TC-P2-LAYER-002: Redis Fallback**
```yaml
Objective: Redis cache serves tokens not in LRU
Setup:
  Token "sk-warm-token" in Redis (not in LRU)
Steps:
  1. Send request with sk-warm-token
Expected:
  - Lookup order: LRU (miss) → Redis (hit)
  - Latency: <1ms
  - Token promoted to LRU cache
Prometheus:
  tokligence_token_lookup_total{layer="lru", result="miss"} = 1
  tokligence_token_lookup_total{layer="redis", result="hit"} = 1
```

**TC-P2-LAYER-003: PostgreSQL Fallback**
```yaml
Objective: PostgreSQL serves tokens not in LRU or Redis
Setup:
  Token "sk-cold-token" only in PostgreSQL
Steps:
  1. Send request with sk-cold-token
Expected:
  - Lookup order: LRU (miss) → Redis (miss) → PostgreSQL (hit)
  - Latency: <10ms
  - Token promoted to Redis and LRU
Prometheus:
  tokligence_token_lookup_total{layer="postgresql", result="hit"} = 1
```

**TC-P2-LAYER-004: All Layers Miss → Fail-Open**
```yaml
Objective: Unknown token handled by fail-open mode
Setup:
  degradation_mode = "fail_open"
  Token "sk-unknown" not in any store
Steps:
  1. Send request with sk-unknown
Expected:
  - Lookup: LRU → Redis → PostgreSQL → Snapshot → All miss
  - Fallback: Default priority=4, quota=10000
  - Request accepted
Prometheus:
  tokligence_degradation_mode_invocations{mode="fail_open"} = 1
```

---

#### 3.3.2 Fail-Open vs Fail-Closed

**Test Cases:**

**TC-P2-FAIL-001: Fail-Open Availability**
```yaml
Objective: Fail-open maximizes availability with minimal security risk
Setup:
  degradation_mode = "fail_open"
  All stores down (Redis + PostgreSQL + Snapshot cache disabled)
Steps:
  1. Send 100 requests with random tokens
Expected:
  - All 100 requests: HTTP 200 (accepted with strict limits)
  - Priority: 4 (external/lowest)
  - Quota: 100 tokens/request, 1000 tokens/day (minimal to prevent abuse)
  - Rate limit: 10 RPS per IP (abuse prevention)
  - Logs: "WARN: All stores down, using fail-open with strict limits"
Business Impact:
  - ✅ Service remains available for legitimate users
  - ✅ Abuse limited by strict quotas (100 tok/req × 10 RPS = max 1000 tok/sec)
  - ⚠️ Reduced capacity during outage (acceptable trade-off)
Security Rationale:
  - Attackers cannot exploit outages to bypass quotas
  - Fail-open quota << normal quota (orders of magnitude lower)
  - P4 priority ensures legitimate high-priority users still served first
```

**TC-P2-FAIL-002: Fail-Closed Security**
```yaml
Objective: Fail-closed prioritizes security over availability
Setup:
  degradation_mode = "fail_closed"
  All stores down
Steps:
  1. Send 100 requests
Expected:
  - All 100 requests: HTTP 503 "Service unavailable"
  - Error: "Token validation temporarily unavailable"
  - /health/ready: 503 (not ready)
Business Impact:
  - ✅ No unauthorized access
  - ❌ Service outage (depends on stores)
```

---

### 3.4 Phase 3: Marketplace Opt-In Tests (OPTIONAL)

#### 3.4.1 Privacy & Compliance

**Test Cases:**

**TC-P3-PRIV-001: Marketplace Disabled by Default**
```yaml
Objective: Fresh installation has marketplace disabled
Steps:
  1. Install gateway (default config)
  2. Check gateway.ini
  3. Send request
Expected:
  - Config: [provider.marketplace] enabled = false
  - Request routed to LocalProvider only
  - No outbound calls to marketplace API
Compliance:
  - ✅ GDPR compliant (no external data transfer)
  - ✅ No dial-home on first run
```

**TC-P3-PRIV-002: Opt-In Workflow**
```yaml
Objective: Marketplace enabled only after explicit opt-in
Steps:
  1. Set [provider.marketplace] enabled = true
  2. Restart gateway
  3. Send request
Expected:
  - First request: Prompts for marketplace consent (CLI/UI)
  - Consent given → Marketplace API called
  - Consent denied → Falls back to LocalProvider
Logs:
  - "INFO: Marketplace opt-in consent granted by user"
```

**TC-P3-PRIV-003: Data Minimization**
```yaml
Objective: Only necessary data sent to marketplace (no PII)
Setup:
  marketplace.enabled = true
Steps:
  1. Send request: "My SSN is 123-45-6789, please help"
  2. Capture outbound marketplace API call
Expected Data Sent:
  - ✅ Model name: "gpt-4"
  - ✅ Preferred region: "us-west-2"
  - ✅ Max price: 0.05
  - ❌ Request content: NOT sent
  - ❌ SSN or PII: NOT sent
Validation:
  - Wireshark/tcpdump: Verify payload
```

---

#### 3.4.2 Transaction Billing and Commission Calculation

**Test Cases:**

**TC-P3-BILLING-001: Transaction Commission Calculation (5%)**
```yaml
Objective: Verify 5% commission calculated correctly on transactions
Setup:
  marketplace.enabled = true
  marketplace.api_key = "test_key_123"
  Supplier price: $8/Mtok for gpt-4
Steps:
  1. Send request consuming 1500 tokens
  2. Check billing API call
Expected:
  - Supplier cost: $0.012 (1500 tokens × $8/Mtok)
  - User cost: $0.0126 ($0.012 × 1.05)
  - Commission: $0.0006 (5%)
  - Billing API called with correct amounts
  - Response header: X-Tokligence-Commission: $0.0006
```

**TC-P3-BILLING-002: GMV Accounting and Settlement**
```yaml
Objective: Verify three-way settlement (user/supplier/Tokligence)
Setup:
  marketplace.enabled = true
  Multiple transactions over 1 day
Steps:
  1. Execute 10 transactions totaling $100 supplier cost
  2. Check accounting records
Expected:
  - Total supplier credit: $100.00
  - Total user debit: $105.00
  - Total Tokligence commission: $5.00 (5% GMV)
  - All transactions recorded in ledger
  - Daily GMV = $105.00
```

**TC-P3-BILLING-003: No Billing Without API Key**
```yaml
Objective: Marketplace requires API key for billing identity
Setup:
  marketplace.enabled = true
  marketplace.api_key = "" (empty)
Steps:
  1. Send marketplace request
Expected:
  - HTTP 400 "API key required for billing"
  - Error message: "Get your API key at marketplace.tokligence.com/dashboard"
  - No marketplace call made
```

---

#### 3.4.3 Multi-Dimensional Supplier Selection

**Test Cases:**

**TC-P3-ROUTING-001: Price-Optimized Selection**
```yaml
Objective: Select cheapest supplier when other factors equal
Setup:
  marketplace.enabled = true
  3 suppliers available:
    - SupplierA: $10/Mtok, P99=500ms, throughput=10Ktok/s, availability=0.99
    - SupplierB: $8/Mtok, P99=500ms, throughput=10Ktok/s, availability=0.99
    - SupplierC: $12/Mtok, P99=500ms, throughput=10Ktok/s, availability=0.99
Steps:
  1. Request gpt-4 completion
  2. Check selected supplier
Expected:
  - SupplierB selected (lowest price)
  - Log: "Selected supply: SupplierB (price=$8/Mtok, score=0.XX)"
```

**TC-P3-ROUTING-002: Latency vs Price Tradeoff**
```yaml
Objective: Balance latency and price with configurable weights
Setup:
  marketplace.enabled = true
  Routing weights: price=40%, latency=30%, throughput=10%, availability=15%, load=5%
  2 suppliers available:
    - SupplierA: $8/Mtok, P99=2000ms (slow), throughput=20Ktok/s, availability=0.99
    - SupplierB: $12/Mtok, P99=200ms (fast), throughput=20Ktok/s, availability=0.99
Steps:
  1. Request gpt-4 completion
  2. Check selected supplier
Expected:
  - Scoring:
    SupplierA: 0.40×(1-8/30) + 0.30×(1-2000/5000) = 0.473
    SupplierB: 0.40×(1-12/30) + 0.30×(1-200/5000) = 0.530
  - SupplierB selected (higher score despite higher price)
  - Rationale: Better latency outweighs price difference
```

**TC-P3-ROUTING-003: Multi-Region Selection**
```yaml
Objective: Select supplier in user's preferred region
Setup:
  marketplace.enabled = true
  2 suppliers available:
    - SupplierA: $8/Mtok, region=us-east-1, P99=200ms
    - SupplierB: $8/Mtok, region=eu-west-1, P99=200ms
  User location: us-east-1
Steps:
  1. Request with region preference: us-east-1
  2. Check selected supplier
Expected:
  - SupplierA selected (same region as user)
  - Lower actual latency due to geographic proximity
```

**TC-P3-ROUTING-004: Throughput-Based Selection Under Load**
```yaml
Objective: Select supplier with higher throughput when all are loaded
Setup:
  marketplace.enabled = true
  2 suppliers available:
    - SupplierA: $8/Mtok, throughput=10Ktok/s, current_load=0.8
    - SupplierB: $8/Mtok, throughput=50Ktok/s, current_load=0.8
Steps:
  1. Request large completion (100K tokens)
  2. Check selected supplier
Expected:
  - SupplierB selected (5x higher throughput)
  - Faster completion despite same load percentage
```

---

### 3.5 Integration Tests (All Phases)

**Test Cases:**

**TC-INT-001: End-to-End Priority Scheduling**
```yaml
Objective: Full workflow from request to LLM response with priority
Setup:
  LocalProvider with gpt-4 (max 10 concurrent)
  10 concurrent long-running requests (priority=4, low)
Steps:
  1. Start 10 low-priority requests (occupy all slots)
  2. Send high-priority request (priority=1)
Expected:
  - High-priority request queued
  - One low-priority request completes
  - High-priority request immediately scheduled
  - Total wait time < 30 seconds
Validation:
  - Response header: X-Tokligence-Queue-Wait-Ms < 30000
```

**TC-INT-002: Multi-Provider Failover**
```yaml
Objective: Failover from Anthropic to OpenAI on provider failure
Setup:
  Primary: Anthropic (claude-3-5-sonnet)
  Fallback: OpenAI (gpt-4)
Steps:
  1. Request claude-3-5-sonnet
  2. Anthropic returns 503
  3. Circuit breaker opens
Expected:
  - Request automatically retried with gpt-4
  - Response header: X-Tokligence-Provider: openai
  - User receives successful response (different model)
Logs:
  - "WARN: Anthropic unavailable, failing over to OpenAI"
```

---

## 4. Test Infrastructure

### 4.1 Test Environments

| Environment | Purpose | Infrastructure |
|-------------|---------|----------------|
| **Unit** | Component tests | In-memory mocks, no external dependencies |
| **Integration** | API contract tests | Docker Compose (Redis, PostgreSQL, mock LLM) |
| **Staging** | Pre-production validation | Kubernetes cluster, real LLM APIs (limited quota) |
| **Load** | Performance & capacity tests | Dedicated cluster, synthetic traffic generators |

### 4.2 Test Automation

**Continuous Integration (CI):**
```bash
# .github/workflows/test.yml
name: Test Pipeline
on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: go test ./... -v -race -cover

  integration-tests:
    runs-on: ubuntu-latest
    services:
      redis:
        image: redis:7-alpine
      postgres:
        image: postgres:15
    steps:
      - run: ./tests/run_all_tests.sh

  load-tests:
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - run: ./tests/load/run_load_test.sh --duration=10m --rps=1000
```

### 4.3 Mock Services

**Mock LLM API (for testing without real API costs):**
```go
// tests/mocks/llm_server.go
type MockLLM struct {
    latency      time.Duration
    errorRate    float64
    tokenRate    int  // tokens/sec
}

func (m *MockLLM) HandleChatCompletion(w http.ResponseWriter, r *http.Request) {
    // Simulate latency
    time.Sleep(m.latency)

    // Simulate errors
    if rand.Float64() < m.errorRate {
        http.Error(w, "Mock LLM error", 500)
        return
    }

    // Simulate streaming response at configured token rate
    w.Header().Set("Content-Type", "text/event-stream")
    for i := 0; i < 100; i++ {
        fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"word%d \"}}]}\n\n", i)
        time.Sleep(time.Second / time.Duration(m.tokenRate))
    }
}
```

### 4.4 Test Data Generation

**Synthetic Token Database:**
```sql
-- tests/fixtures/tokens.sql
-- Generate 10,000 test tokens across priority tiers
INSERT INTO token_metadata (token_hash, tier, priority, quota_limit)
SELECT
    sha256('test-token-' || generate_series),
    CASE (random() * 4)::int
        WHEN 0 THEN 'internal'
        WHEN 1 THEN 'partner'
        WHEN 2 THEN 'standard'
        ELSE 'external'
    END,
    CASE (random() * 4)::int
        WHEN 0 THEN 1  -- Critical
        WHEN 1 THEN 2  -- High
        WHEN 2 THEN 3  -- Medium
        ELSE 4         -- Low
    END,
    (1000 + random() * 99000)::int  -- 1K-100K tokens/day
FROM generate_series(1, 10000);
```

---

## 5. Quality Gates

### 5.1 Phase 0 Quality Gates

**Must Pass Before Phase 1:**
- [ ] All unit tests pass (100% of existing + new tests)
- [ ] Integration tests pass (priority scheduling, capacity limits, degradation)
- [ ] Backward compatibility: Existing Responses API tests still pass
- [ ] Performance: P95 scheduling latency < 10ms under 1000 RPS load
- [ ] No regressions: Protocol translation accuracy ≥99.9%

### 5.2 Phase 1 Quality Gates

**Must Pass Before Phase 2:**
- [ ] Prometheus metrics exposed and validated
- [ ] Circuit breaker prevents cascading failures
- [ ] Snapshot cache survives Redis+PostgreSQL outage
- [ ] Load test: 10,000 RPS sustained for 1 hour without memory leaks

### 5.3 Phase 2 Quality Gates

**Must Pass Before Production:**
- [ ] Token lookup latency: LRU <1μs, Redis <1ms, PostgreSQL <10ms
- [ ] Fail-open and fail-closed modes both tested in staging
- [ ] Security review: No token leakage in logs/metrics
- [ ] Chaos engineering: Random store failures handled gracefully

---

## 6. Rollback Strategy

### 6.1 Feature Flags

All new scheduling features controlled by flags:
```ini
# gateway.ini
[scheduling]
enabled = false  # Master kill switch
priority_enabled = false
capacity_model = "simple"  # simple | multi_dimensional
degradation_mode = "disabled"  # disabled | fail_open | fail_closed

[provider.local]
enabled = true

[provider.marketplace]
enabled = false  # Marketplace always opt-in
```

### 6.2 Rollback Checklist

**If Phase 0 Deployment Fails:**
1. Set `[scheduling] enabled = false` in config
2. Restart gateway → Falls back to FIFO scheduling
3. All requests handled as priority=3 (medium)
4. Investigate root cause in staging environment
5. Fix and redeploy with feature flag still disabled
6. Gradual rollout: Enable for 1% traffic → 10% → 50% → 100%

**Database Rollback (if Phase 2 schema changes cause issues):**
```sql
-- Rollback migration
DROP TABLE IF EXISTS token_metadata_v2;
ALTER TABLE token_metadata RENAME TO token_metadata_v1;
```

---

## 7. Success Metrics (KPIs)

### 7.1 Phase 0 Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Scheduling Latency (P95)** | <10ms | Prometheus histogram |
| **Priority Violation Rate** | <0.01% | Low-priority scheduled before high-priority |
| **Capacity Enforcement Accuracy** | 100% | No requests exceed configured limits |
| **Test Coverage** | >80% | Go test coverage report |
| **Backward Compatibility** | 100% | All existing integration tests pass |

### 7.2 Phase 1 Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Circuit Breaker MTTR** | <30s | Time to recover from provider failure |
| **Snapshot Cache Staleness** | <5min | Age of snapshot data when used |
| **Firewall Block Rate** | >99% | Malicious requests blocked before scheduling |
| **Metrics Cardinality** | <10K | Prometheus time series count |

### 7.3 Phase 2 Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| **Token Lookup Latency (P99)** | <10ms | All layers combined |
| **Cache Hit Rate (LRU)** | >90% | For hot tokens |
| **Degradation Activation** | <1/day | Fail-open mode invocations (production) |
| **Zero Data Loss** | 100% | Fail-closed mode never grants unauthorized access |

---

## 8. Next Steps

### 8.1 Immediate Actions (Week 1)

1. **Create test fixtures:**
   - `tests/fixtures/tokens.sql` (10,000 synthetic tokens)
   - `tests/mocks/llm_server.go` (mock LLM API)

2. **Set up test infrastructure:**
   - Docker Compose for integration tests (Redis + PostgreSQL + mock LLM)
   - GitHub Actions workflow for CI

3. **Implement Phase 0 MVP:**
   - Start with `internal/scheduler/capacity_model.go`
   - Then `internal/scheduler/priority_queue.go`
   - Then `internal/httpserver/responses/provider_local.go`

### 8.2 Documentation Updates

- [ ] Update `CLAUDE.md` with scheduling feature flags
- [ ] Create `docs/scheduling/08_TESTING_GUIDE.md` (link to this doc)
- [ ] Update `docs/QUICK_START.md` with priority configuration examples

---

**Document Status:** Ready for review and implementation

**Approval Required From:**
- [ ] Tech Lead (architecture review)
- [ ] QA Lead (test plan review)
- [ ] DevOps (infrastructure requirements)

**Estimated Effort:**
- Phase 0: 4 weeks (2 engineers)
- Phase 1: 2 weeks (2 engineers)
- Phase 2: 2 weeks (2 engineers)
- **Total: 8 weeks (16 person-weeks)**
