# Request Scheduling, Queuing, and Quota Management System Design

**Version:** 2.0 (Aligned with Provider SPI & Tokens/Sec Model)
**Date:** 2025-02-01
**Status:** Design Proposal - Aligned with v2.0 Architecture
**Related Documents:**
- `00_REVISED_OVERVIEW.md` - Overall scheduling system overview (READ THIS FIRST)
- `06_MARKETPLACE_INTEGRATION.md` - Provider SPI abstraction and capacity model
- `arc_design/01_comprehensive_system_architecture.md` - Overall system architecture
- `arc_design/04_trading_and_matching_engine.md` - Trading and routing systems
- `CLAUDE.md` - Project overview and development guidelines

**Version 2.0 Changes:**
- ✅ Integrated Provider SPI abstraction (LocalProvider, MarketplaceProvider, HybridProvider)
- ✅ Changed capacity model from RPS-focused to tokens/sec primary + multi-dimensional
- ✅ Added LLM Protection Layer integration (context length limits)
- ✅ Added degradation strategies (fail-open/fail-closed)
- ✅ Clarified that scheduling works with ANY provider (local, marketplace, or hybrid)

---

## Executive Summary

This document designs a comprehensive **Priority-based Request Scheduling, Multi-Queue Management, and Hierarchical Quota System** for Tokligence Gateway. The system enables gateway operators selling self-hosted LLM token throughput to serve multiple tenants (consumers) with differentiated service levels, fair resource allocation, and environment-based partitioning.

**Core Value Proposition:**
- **For Gateway Operators (Self-hosted LLM Providers):** Maximize revenue by serving multiple customers with SLA-differentiated pricing while protecting resource capacity
- **For Consumers:** Get guaranteed performance for production workloads while optimizing costs for dev/test/batch workloads
- **For Platform (Tokligence Marketplace):** Enable fair, auditable resource allocation across the token trading network

**Key Capabilities:**
1. **Priority-based Request Scheduling** - Ensure critical production traffic gets resources first
2. **Multi-Queue Management** - Isolate traffic by environment (dev/test/uat/staging/production)
3. **Hierarchical Quota System** - Allocate token budgets by organization/team/environment/user
4. **Fair Scheduling Algorithms** - Prevent starvation while honoring priority
5. **Dynamic Resource Ratio** - Adjust capacity allocation based on demand and pricing
6. **Time-Window Based Scheduling** - Dynamic priority/quota adjustments based on time of day, day of week, or seasonal patterns

---

## 1. Problem Statement & Use Cases

### 1.1 Core Problems to Solve

Based on research into LiteLLM scheduler, Kubernetes pod priority, and industry best practices, we identified these critical needs:

#### Problem 1: Production vs Non-Production Interference
**Scenario:** A company runs both production services and ML experiments through the same gateway to a self-hosted Llama 3 70B cluster.
- **Issue:** Heavy batch jobs from data science team consume all GPU capacity, causing production API latency spikes (P99 > 5s) and SLA violations
- **Current Workaround:** Deploy separate gateway instances (expensive, complex)
- **Desired Solution:** Single gateway with priority queues ensuring production always gets capacity first

#### Problem 2: Multi-Environment Resource Partitioning
**Scenario:** Enterprise with dev, test, UAT, staging, and production environments sharing limited LLM capacity (e.g., 1000 TPS on a self-hosted cluster).
- **Issue:** Cannot hard-partition capacity (wastes idle resources), but also cannot allow dev to starve production
- **Current Workaround:** Manual coordination, constant capacity firefighting
- **Desired Solution:** Hierarchical quotas with guaranteed minimums and elastic borrowing

#### Problem 3: Multi-Tenant Fair Allocation
**Scenario:** Gateway operator sells throughput to 10 different customers at different price tiers (Bronze/Silver/Gold).
- **Issue:** High-paying customers get same treatment as low-tier; no way to enforce SLA differences
- **Current Workaround:** Separate deployments per tier (not scalable)
- **Desired Solution:** Weighted fair queuing based on pricing tier with preemption

#### Problem 4: Cost-Performance Trade-offs
**Scenario:** Organization wants to optimize LLM spend across workload types:
- Critical user-facing API: needs P99 < 500ms (willing to pay premium)
- Internal chatbot: acceptable at P99 < 2s (mid-tier pricing)
- Batch embedding jobs: can tolerate P99 > 10s (spot pricing)
- **Desired Solution:** Priority-aware routing with cost-performance options

### 1.2 Real-World Use Cases

| Use Case | Description | Requirements |
|----------|-------------|--------------|
| **UC1: Environment Isolation** | Dev/Test/Prod environments share self-hosted GPUs | Guaranteed prod quota; dev can borrow idle prod capacity |
| **UC2: Tiered SLA Customers** | Gateway sells Gold/Silver/Bronze tier access | Gold gets 3x priority weight; Bronze has longer queue timeout |
| **UC3: Spot vs Reserved** | Mix of reserved capacity and spot market requests | Reserved requests bypass queue; spot only if capacity available |
| **UC4: Emergency Override** | Production incident needs immediate capacity | Admin header `X-Priority: critical` preempts lower priority |
| **UC5: Team Budget Management** | Finance team has $500/month LLM budget across 5 projects | Hierarchical quotas with alerts at 80% usage |
| **UC6: Rate Spike Protection** | Sudden traffic spike from experiment | Queue buffers requests; rejects beyond max_queue_depth |
| **UC7: Fair Multi-Tenant** | 10 customers share 1000 TPS cluster | Weighted Fair Queuing prevents one customer monopolizing |
| **UC8: Cost-Aware Routing** | Route cheap batch jobs to spot; critical to reserved | Scheduler picks queue based on budget × priority |
| **UC9: Time-Based Priority** | Batch jobs run at night with higher priority | Off-peak hours: batch gets P2, peak hours: batch gets P4 |
| **UC10: Business Hours Quota** | Dev team gets more quota during business hours | 9am-6pm: 500 TPS, off-hours: 100 TPS |

### 1.3 Time-Window Based Scheduling Use Cases

#### Problem 5: Peak vs Off-Peak Resource Allocation
**Scenario:** Self-hosted LLM cluster with predictable usage patterns:
- **Business Hours (9am-6pm):** High production load from customer-facing APIs
- **Off-Hours (6pm-9am):** Low production load, opportunity for batch processing
- **Issue:** Want to maximize utilization by allowing batch jobs at night without impacting production during the day
- **Desired Solution:** Time-based priority/quota adjustments

**Example:**
```yaml
# Production always gets priority, but batch can use idle capacity at night

time_windows:
  - name: business_hours
    schedule: "0 9 * * 1-5"  # Mon-Fri 9am
    duration: 9h
    rules:
      - workload: production
        priority: 1
        quota_tps: 800
      - workload: batch
        priority: 4
        quota_tps: 100

  - name: off_hours
    schedule: "0 18 * * *"  # Daily 6pm
    duration: 15h
    rules:
      - workload: production
        priority: 1
        quota_tps: 500
      - workload: batch
        priority: 2  # Higher priority at night!
        quota_tps: 400  # More capacity
```

#### Problem 6: Seasonal/Event-Driven Scaling
**Scenario:** E-commerce company with Black Friday traffic spike:
- **Normal:** Production needs 1000 TPS
- **Black Friday Week:** Production needs 5000 TPS, dev/staging can be deprioritized
- **Issue:** Hard to manually adjust quotas for events
- **Desired Solution:** Scheduled quota/priority changes

**Example:**
```yaml
time_windows:
  - name: black_friday_2025
    schedule: "2025-11-24T00:00:00Z"
    duration: 168h  # 1 week
    rules:
      - environment: production
        quota_multiplier: 5.0  # 5x normal quota
        priority: 0            # Highest priority
      - environment: staging
        quota_multiplier: 0.2  # Reduce to 20%
        priority: 3
      - environment: dev
        quota_multiplier: 0.1  # Reduce to 10%
        priority: 4
```

#### Problem 7: Cost Optimization via Time-of-Use Pricing
**Scenario:** Cloud GPU provider charges less during off-peak hours:
- **Peak (8am-10pm):** $10/hour per GPU
- **Off-peak (10pm-8am):** $5/hour per GPU
- **Desired:** Automatically shift batch workloads to off-peak hours

**Example:**
```yaml
time_windows:
  - name: off_peak_discount
    schedule: "0 22 * * *"  # 10pm daily
    duration: 10h
    rules:
      - workload: batch
        priority: 1  # Boost batch priority at night
        cost_multiplier: 0.5  # Pass savings to customers
        quota_multiplier: 2.0  # Encourage usage
```

#### Problem 8: Maintenance Windows
**Scenario:** Weekly cluster maintenance requires capacity reduction:
- **Issue:** Don't want to reject all requests, but need to reduce load
- **Desired:** Temporarily increase queue tolerance, reduce quotas

**Example:**
```yaml
time_windows:
  - name: maintenance_window
    schedule: "0 2 * * 0"  # Sunday 2am
    duration: 2h
    rules:
      - all_workloads: true
        capacity_multiplier: 0.5  # Reduce to 50% capacity
        queue_timeout_multiplier: 3.0  # Allow longer waits
        max_queue_depth_multiplier: 2.0  # Buffer more requests
```

### 1.4 Integration with Provider SPI and Capacity Model (v2.0)

**IMPORTANT:** This scheduling system is **provider-agnostic**. It works with:
- ✅ **LocalProvider** - Direct connection to self-hosted LLM
- ✅ **MarketplaceProvider** - Dynamic routing via marketplace
- ✅ **HybridProvider** - Local first, marketplace fallback

For full Provider SPI details, see `06_MARKETPLACE_INTEGRATION.md` §2.

#### Multi-Dimensional Capacity Model

**v1.0 (OLD - RPS-focused):**
```go
type Capacity struct {
    MaxRPS int  // Too simplistic!
}
```

**v2.0 (NEW - Multi-dimensional):**
```go
type Capacity struct {
    // PRIMARY metric (MOST IMPORTANT for scheduling)
    MaxTokensPerSec   int     // tokens/sec capacity

    // SECONDARY metrics
    MaxRPS            int     // requests/sec (fallback)
    MaxConcurrent     int     // concurrent requests
    MaxContextLength  int     // max context window

    // Model-specific
    ModelFamily       string  // "chat" | "embedding" | "completion"
    AvgContextLength  int     // affects scheduling decisions

    // Current state
    CurrentLoad       float64 // 0.0-1.0
    AvailableTokensPS int     // available tokens/sec RIGHT NOW
}
```

**Why tokens/sec instead of RPS?**
- ✅ RPS treats "generate 10 tokens" same as "generate 1000 tokens" (wrong!)
- ✅ Embedding requests use different GPU patterns than chat
- ✅ Long-context requests (100K tokens) need different scheduling than short (1K tokens)
- ✅ Token-based quotas align with LLM pricing (per-token costs)

**Scheduler Integration:**

```go
// internal/scheduler/priority/scheduler.go

func (s *PriorityScheduler) CanAcceptRequest(req *Request, provider provider.Provider) bool {
    // Get current capacity from provider
    capacity, err := provider.GetCapacity(context.Background(), req.Model)
    if err != nil {
        // Provider unavailable - apply degradation logic
        return s.handleProviderDegradation(err)
    }

    // Check tokens/sec capacity (PRIMARY)
    estimatedTokens := estimateTokenUsage(req)
    estimatedTokensPerSec := estimatedTokens / estimatedDuration(req)

    if capacity.AvailableTokensPS < estimatedTokensPerSec {
        // Not enough token capacity - queue or reject
        return false
    }

    // Check concurrent request limit (SECONDARY)
    if s.currentConcurrent >= capacity.MaxConcurrent {
        return false
    }

    // Check context length limit (PROTECTION)
    if estimateContextLength(req) > capacity.MaxContextLength {
        // Will be handled by LLM Protection Layer
        // See section 1.5
    }

    return true
}
```

#### How Scheduling Uses Provider Capacity

**Before (v1.0):**
```go
// Simple RPS check
if currentRPS < maxRPS {
    executeRequest(req)
}
```

**After (v2.0):**
```go
// Multi-dimensional check
capacity := provider.GetCapacity(req.Model)

// 1. Check token throughput (primary)
if estimatedTokensPerSec > capacity.AvailableTokensPS {
    // Queue or reject based on priority
    priorityQueue[req.Priority].Enqueue(req)
    return
}

// 2. Check concurrent limit (secondary)
if currentConcurrent >= capacity.MaxConcurrent {
    priorityQueue[req.Priority].Enqueue(req)
    return
}

// 3. Check model family compatibility
if req.RequiresChatModel && capacity.ModelFamily != "chat" {
    return errors.New("model mismatch")
}

// 4. Proceed with execution
executeRequest(req, provider)
```

### 1.5 Integration with LLM Protection Layer (v2.0)

**NEW:** Requests pass through Protection Layer BEFORE scheduling.

See `06_MARKETPLACE_INTEGRATION.md` §5 for full details.

```go
// Request flow (v2.0):
//
// HTTP Request
//   → Authentication
//   → LLM Protection Layer (NEW!)
//      ├─ Context Length Protection
//      ├─ Rate Limiting Protection
//      └─ Content Filter (optional)
//   → Priority Scheduler (THIS DOCUMENT)
//      ├─ Classify priority
//      ├─ Check quota
//      ├─ Enqueue if needed
//      └─ Dequeue when capacity available
//   → Provider (LocalProvider / MarketplaceProvider / HybridProvider)
//      └─ Execute LLM request

// Example: Context Length Protection
type ContextLengthProtector struct {
    modelLimits map[string]int
}

func (clp *ContextLengthProtector) Transform(req *Request) (*Request, error) {
    limit := clp.modelLimits[req.Model]
    actual := estimateTokens(req)

    if actual > limit {
        // Truncate BEFORE scheduling
        return truncateRequest(req, limit), nil
    }

    return req, nil
}
```

**Benefit:** Scheduler doesn't need to worry about invalid requests - they're filtered upstream.

### 1.6 Provider Degradation and Scheduling

**NEW:** When provider is unavailable, scheduler must handle gracefully.

See `06_MARKETPLACE_INTEGRATION.md` §4.3 for full degradation strategies.

```go
// Degradation modes:
// 1. fail_open  - Continue with reduced capacity (local only)
// 2. fail_closed - Reject new requests (return 503)
// 3. cached      - Use cached capacity estimates

func (s *PriorityScheduler) handleProviderDegradation(err error) bool {
    switch s.config.DegradationMode {
    case "fail_open":
        // Allow scheduling with conservative limits
        log.Warn("Provider degraded, using fail-open mode")
        return s.canAcceptWithDegradedCapacity()

    case "fail_closed":
        // Reject all new requests
        log.Error("Provider degraded, rejecting request (fail-closed)")
        return false

    case "cached":
        // Use last-known capacity (may be stale)
        if cachedCap := s.capacityCache.Get(); cachedCap != nil {
            log.Warn("Provider degraded, using cached capacity",
                "age", time.Since(cachedCap.CachedAt))
            return cachedCap.AvailableTokensPS > 0
        }
        // No cache - fall back to fail_closed
        return false
    }
}
```

**Configuration:**

```ini
[scheduler]
type = priority
degradation_mode = fail_open  # fail_open | fail_closed | cached

# Capacity estimation (when provider unavailable)
estimated_tokens_per_sec = 5000  # Conservative estimate
estimated_max_concurrent = 20
```

---

## 2. System Architecture

### 2.1 High-Level Components

```
┌──────────────────────────────────────────────────────────────────┐
│                  Client Requests (with metadata)                  │
│  Headers: X-Priority, X-Environment, X-Team-ID, X-Request-Class   │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│               Gateway: Request Admission Controller              │
│  • Extract priority/environment/tenant from request              │
│  • Quota pre-check (reject if account over limit)                │
│  • Assign to queue based on scheduling policy                    │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                  Multi-Level Priority Queues                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Queue P0     │  │ Queue P1     │  │ Queue P2     │          │
│  │ (Critical)   │  │ (High)       │  │ (Standard)   │  ...     │
│  │ Max: 100     │  │ Max: 500     │  │ Max: 1000    │          │
│  │ Timeout: 10s │  │ Timeout: 30s │  │ Timeout: 60s │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│                                                                   │
│  Per-Environment Sub-Queues (optional):                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  prod: [req1, req2]  uat: [req3]  dev: [req4, req5]    │    │
│  └─────────────────────────────────────────────────────────┘    │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Scheduler (Dequeue Logic)                      │
│  Algorithm Options:                                               │
│  • Strict Priority (P0 always first, can starve P2)             │
│  • Weighted Fair Queuing (WFQ with weights)                      │
│  • Deficit Round Robin (DRR for fairness)                        │
│  • Hybrid: P0 strict, P1/P2 use WFQ                              │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│              Capacity Manager & Rate Limiter                      │
│  • Token Bucket per queue/tenant (refill rate = quota/time)     │
│  • Check current_tps < max_tps                                   │
│  • Dynamic quota borrowing (if enabled)                          │
│  • Preemption: kill low-priority streaming if critical arrives  │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                 Upstream Provider (LLM Execution)                 │
│  • Self-hosted: vLLM/Ollama/LocalAI cluster                      │
│  • Public Cloud: OpenAI/Anthropic (pass-through mode)           │
└────────────────────────────┬─────────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                   Response & Metering                             │
│  • Record actual tokens consumed                                 │
│  • Deduct from quota (Redis atomic decrement)                   │
│  • Emit metrics: queue_wait_time, priority_level, tokens_used   │
│  • Update quota state in Marketplace                             │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 Data Flow

```
1. Request Arrival
   ├─> Extract: account_id, priority, environment, team_id
   ├─> Lookup Quota: GET quota:{account_id}:{environment} from Redis
   ├─> Pre-check: if remaining_tokens < estimated_cost → REJECT 429
   └─> Assign Queue: priority_queue[priority_level].enqueue(request)

2. Scheduler Cycle (every 10-50ms)
   ├─> For each priority level (P0 → P1 → P2):
   │   ├─> Check capacity: if current_tps < max_tps:
   │   │   ├─> Dequeue request
   │   │   ├─> Acquire capacity token
   │   │   └─> Execute request
   │   └─> Else: skip to next cycle
   └─> Apply fairness: if P0 empty, allow P1/P2 weighted share

3. Request Execution
   ├─> Call upstream LLM
   ├─> Stream response (if streaming)
   └─> On completion: record usage

4. Quota Update
   ├─> Calculate: cost_usd = (prompt_tokens * in_rate + completion_tokens * out_rate)
   ├─> Deduct: DECRBY quota:{account_id}:{env} {tokens}
   ├─> Async report to Marketplace: POST /v1/usage/batch
   └─> Check threshold: if remaining < 20% → emit warning

5. Preemption (if enabled)
   ├─> High-priority request arrives while capacity full
   ├─> Find: lowest-priority streaming request
   ├─> Cancel: send SIGTERM to worker, return 503 to client
   └─> Schedule: high-priority request
```

---

## 3. Detailed Component Design

### 3.1 Priority Classification

**Priority Levels (0-4, lower = higher priority):**

| Priority | Name | Use Case | Max Queue Depth | Queue Timeout | Preemption |
|----------|------|----------|-----------------|---------------|------------|
| **0** | Critical | Emergency overrides, SLA breach recovery | 100 | 10s | Can preempt all |
| **1** | High | Production user-facing APIs | 500 | 30s | Can preempt P2-P4 |
| **2** | Standard | Internal services, staging | 1000 | 60s | Can preempt P3-P4 |
| **3** | Low | Development, testing | 2000 | 120s | Cannot preempt |
| **4** | Batch | Background jobs, experiments | 5000 | 300s | Cannot preempt |

**Priority Assignment Logic:**

```go
type PriorityClassifier struct {
    defaultPriority int
    rules []PriorityRule
}

type PriorityRule struct {
    Condition  string  // "environment", "account_tier", "header", "model"
    Value      string  // "production", "gold", "X-Priority: critical", "gpt-4"
    Priority   int     // 0-4
    Weight     float64 // For weighted fair queuing
}

func (pc *PriorityClassifier) Classify(req *Request) int {
    // 1. Explicit header override (admin only)
    if req.Header.Get("X-Priority") == "critical" && isAdmin(req) {
        return 0
    }

    // 2. Match rules in order
    for _, rule := range pc.rules {
        switch rule.Condition {
        case "environment":
            if req.Environment == rule.Value {
                return rule.Priority
            }
        case "account_tier":
            if req.Account.Tier == rule.Value {
                return rule.Priority
            }
        case "model":
            if req.Model == rule.Value {
                return rule.Priority
            }
        case "tag":
            if contains(req.Tags, rule.Value) {
                return rule.Priority
            }
        }
    }

    // 3. Default
    return pc.defaultPriority
}
```

**Configuration Example:**

```yaml
# config/scheduling.yaml
priority_classifier:
  default_priority: 2

  rules:
    # Environment-based
    - condition: environment
      value: production
      priority: 1
      weight: 3.0

    - condition: environment
      value: staging
      priority: 2
      weight: 2.0

    - condition: environment
      value: dev
      priority: 3
      weight: 1.0

    # Tier-based
    - condition: account_tier
      value: gold
      priority: 1
      weight: 3.0

    - condition: account_tier
      value: silver
      priority: 2
      weight: 2.0

    - condition: account_tier
      value: bronze
      priority: 3
      weight: 1.0

    # Model-based (expensive models get priority)
    - condition: model
      value: gpt-4
      priority: 1
      weight: 2.0

    # Tag-based
    - condition: tag
      value: batch
      priority: 4
      weight: 0.5
```

### 3.2 Multi-Queue Architecture

**Queue Structure:**

```go
type PriorityQueue struct {
    Level        int               // 0-4
    Name         string            // "critical", "high", "standard"
    MaxDepth     int               // Maximum requests in queue
    Timeout      time.Duration     // How long to wait before 503
    Weight       float64           // For weighted fair scheduling

    // Per-environment sub-queues (optional)
    SubQueues    map[string]*FIFO  // "prod" → FIFO, "dev" → FIFO

    // Metrics
    CurrentDepth int
    TotalEnqueued uint64
    TotalDequeued uint64
    TotalTimeout  uint64
    TotalDropped  uint64

    mu           sync.RWMutex
}

type Request struct {
    ID           string
    AccountID    string
    Environment  string
    TeamID       string
    Model        string
    Priority     int
    EstimatedTokens int64

    Timestamp    time.Time
    Deadline     time.Time  // timestamp + timeout

    // For streaming preemption
    Cancelable   bool
    CancelFunc   context.CancelFunc

    // Response channel
    ResultChan   chan *Response
}

func (pq *PriorityQueue) Enqueue(req *Request) error {
    pq.mu.Lock()
    defer pq.mu.Unlock()

    // Check depth
    if pq.CurrentDepth >= pq.MaxDepth {
        pq.TotalDropped++
        return ErrQueueFull
    }

    // Sub-queue routing (if enabled)
    if len(pq.SubQueues) > 0 {
        subq := pq.SubQueues[req.Environment]
        if subq == nil {
            subq = NewFIFO(pq.MaxDepth / len(pq.SubQueues))
            pq.SubQueues[req.Environment] = subq
        }
        subq.Push(req)
    } else {
        // Single FIFO
        pq.queue.Push(req)
    }

    pq.CurrentDepth++
    pq.TotalEnqueued++

    return nil
}

func (pq *PriorityQueue) Dequeue() (*Request, error) {
    pq.mu.Lock()
    defer pq.mu.Unlock()

    if pq.CurrentDepth == 0 {
        return nil, ErrQueueEmpty
    }

    var req *Request

    // Round-robin across sub-queues (if enabled)
    if len(pq.SubQueues) > 0 {
        req = pq.dequeueFromSubQueues()
    } else {
        req = pq.queue.Pop()
    }

    if req == nil {
        return nil, ErrQueueEmpty
    }

    // Check timeout
    if time.Now().After(req.Deadline) {
        pq.TotalTimeout++
        return nil, ErrRequestTimeout
    }

    pq.CurrentDepth--
    pq.TotalDequeued++

    return req, nil
}

// Round-robin across environment sub-queues
func (pq *PriorityQueue) dequeueFromSubQueues() *Request {
    // Track last served environment
    envs := sortedKeys(pq.SubQueues)

    for i := 0; i < len(envs); i++ {
        env := envs[(pq.lastServedIdx + i) % len(envs)]
        subq := pq.SubQueues[env]

        if !subq.IsEmpty() {
            pq.lastServedIdx = (pq.lastServedIdx + i + 1) % len(envs)
            return subq.Pop()
        }
    }

    return nil
}
```

### 3.3 Scheduling Algorithms

We support **three scheduling algorithms**, configurable via `scheduling_policy`:

#### Algorithm 1: Strict Priority (Default for P0, P1)

```go
type StrictPriorityScheduler struct {
    queues []*PriorityQueue  // Sorted by priority (0=highest)
}

func (s *StrictPriorityScheduler) Next() (*Request, error) {
    // Always serve highest priority first
    for _, pq := range s.queues {
        if pq.CurrentDepth > 0 {
            req, err := pq.Dequeue()
            if err == nil {
                return req, nil
            }
        }
    }
    return nil, ErrNoRequests
}
```

**Pros:** Simple, guarantees high-priority latency
**Cons:** Can starve low-priority queues

#### Algorithm 2: Weighted Fair Queuing (WFQ)

```go
type WeightedFairScheduler struct {
    queues  []*PriorityQueue
    deficit map[int]float64  // Deficit counter per queue
}

func (s *WeightedFairScheduler) Next() (*Request, error) {
    // 1. Add quantum (proportional to weight)
    for _, pq := range s.queues {
        s.deficit[pq.Level] += pq.Weight
    }

    // 2. Find queue with highest deficit that has requests
    var selectedQueue *PriorityQueue
    maxDeficit := 0.0

    for _, pq := range s.queues {
        if pq.CurrentDepth > 0 && s.deficit[pq.Level] > maxDeficit {
            maxDeficit = s.deficit[pq.Level]
            selectedQueue = pq
        }
    }

    if selectedQueue == nil {
        return nil, ErrNoRequests
    }

    // 3. Dequeue and decrease deficit
    req, err := selectedQueue.Dequeue()
    if err != nil {
        return nil, err
    }

    // Decrease by request "cost" (estimated tokens or fixed 1.0)
    cost := float64(req.EstimatedTokens) / 1000.0
    if cost == 0 {
        cost = 1.0
    }
    s.deficit[selectedQueue.Level] -= cost

    return req, nil
}
```

**Pros:** Fairness, no starvation, honors weights
**Cons:** More complex, may have slightly higher latency for high-priority

**Weight Examples:**
- Gold tier (weight=3.0) gets 3x more throughput than Bronze (weight=1.0)
- Production (weight=5.0) gets 5x more than dev (weight=1.0)

#### Algorithm 3: Hybrid (Strict P0, WFQ for P1-P4)

```go
type HybridScheduler struct {
    criticalQueue *PriorityQueue
    wfqScheduler  *WeightedFairScheduler
}

func (s *HybridScheduler) Next() (*Request, error) {
    // 1. Always check critical queue first
    if s.criticalQueue.CurrentDepth > 0 {
        req, err := s.criticalQueue.Dequeue()
        if err == nil {
            return req, nil
        }
    }

    // 2. Use WFQ for P1-P4
    return s.wfqScheduler.Next()
}
```

**Pros:** Best of both worlds
**Cons:** Slightly more complex

**Recommended Defaults:**
- Use **Hybrid** for production gateways
- Use **Strict Priority** for single-tenant or simple use cases
- Use **WFQ** when fairness is critical (e.g., public marketplace)

### 3.4 Hierarchical Quota System

**Quota Hierarchy:**

```
Organization (Tokligence Inc.)
  ├─> Team (Engineering)
  │     ├─> Environment (Production)
  │     │     ├─> User (alice@example.com)
  │     │     └─> User (bob@example.com)
  │     ├─> Environment (Staging)
  │     └─> Environment (Dev)
  ├─> Team (Data Science)
  │     ├─> Environment (Production)
  │     └─> Environment (Dev)
  └─> Team (Product)
```

**Quota Types:**

| Type | Scope | Limit Dimension | Enforcement |
|------|-------|----------------|-------------|
| **Hard Quota** | Any level | Tokens/month or USD/month | Strict reject at limit |
| **Soft Quota** | Any level | Tokens/month | Warn at 80%, allow burst to 120% |
| **Reserved Quota** | Environment | Guaranteed min tokens/sec | Pre-allocated capacity |
| **Burstable Quota** | Environment | Max tokens/sec (can borrow) | Elastic, reclaimed if parent needs |

**Data Structure:**

```sql
-- Extension to existing quota table
CREATE TABLE quotas (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  -- Hierarchy
  account_id        UUID REFERENCES accounts(id),
  team_id           UUID,  -- optional
  environment       TEXT,  -- production, staging, dev, test, uat

  -- Limits
  quota_type        TEXT NOT NULL,  -- hard, soft, reserved, burstable
  limit_dimension   TEXT NOT NULL,  -- tokens_per_month, usd_per_month, tps
  limit_value       BIGINT NOT NULL,

  -- Borrowing (for burstable)
  allow_borrow      BOOLEAN DEFAULT false,
  max_borrow_pct    NUMERIC DEFAULT 0.0,  -- e.g., 0.50 = can borrow up to 50% from parent

  -- Time window
  window_start      TIMESTAMPTZ NOT NULL,
  window_end        TIMESTAMPTZ,

  -- Current usage (updated in real-time via Redis)
  used_value        BIGINT DEFAULT 0,

  -- Alerts
  alert_at_pct      NUMERIC DEFAULT 0.80,  -- Alert at 80%

  created_at        TIMESTAMPTZ DEFAULT now(),
  updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_quotas_account_env ON quotas(account_id, environment, window_start DESC);
CREATE INDEX idx_quotas_team ON quotas(team_id, environment);
```

**Quota Enforcement Logic:**

```go
type QuotaManager struct {
    redis  *redis.Client
    db     *sql.DB
}

func (qm *QuotaManager) CheckAndReserve(
    accountID, environment string,
    estimatedTokens int64,
) error {
    // 1. Build hierarchy keys
    keys := []string{
        fmt.Sprintf("quota:%s:%s:month", accountID, environment),  // User+Env
        fmt.Sprintf("quota:%s:month", accountID),                   // User total
        fmt.Sprintf("quota:team:%s:%s:month", teamID, environment), // Team+Env
        fmt.Sprintf("quota:org:%s:month", orgID),                   // Org total
    }

    // 2. Check each level
    for _, key := range keys {
        quota := qm.getQuota(key)
        if quota == nil {
            continue
        }

        used := qm.redis.Get(key + ":used").Int64()

        // Hard quota: strict
        if quota.Type == "hard" && used + estimatedTokens > quota.Limit {
            return ErrQuotaExceeded
        }

        // Soft quota: warn but allow
        if quota.Type == "soft" && used + estimatedTokens > quota.Limit {
            log.Warn("Soft quota exceeded", "key", key, "used", used, "limit", quota.Limit)
            if used + estimatedTokens > quota.Limit * 1.2 {
                return ErrQuotaExceeded  // Hard stop at 120%
            }
        }

        // Reserved quota: check TPS (different dimension)
        if quota.Type == "reserved" {
            // Check: current_tps < reserved_tps
            currentTPS := qm.getCurrentTPS(key)
            if currentTPS >= quota.Limit {
                // Try to borrow
                if !quota.AllowBorrow {
                    return ErrReservedCapacityExceeded
                }

                parentKey := getParentKey(key)
                parentQuota := qm.getQuota(parentKey)
                if parentQuota == nil || qm.getCurrentTPS(parentKey) >= parentQuota.Limit {
                    return ErrNoBorrowableCapacity
                }
            }
        }
    }

    // 3. Optimistically reserve (increment counters)
    for _, key := range keys {
        qm.redis.IncrBy(key + ":used", estimatedTokens)
        qm.redis.Expire(key + ":used", 31 * 24 * time.Hour)  // 1 month
    }

    return nil
}

func (qm *QuotaManager) CommitUsage(
    accountID, environment string,
    actualTokens int64,
    estimatedTokens int64,
) {
    // Adjust: if actual != estimated
    delta := actualTokens - estimatedTokens

    keys := []string{ /* same as above */ }

    for _, key := range keys {
        if delta != 0 {
            qm.redis.IncrBy(key + ":used", delta)
        }

        // Check alert threshold
        used := qm.redis.Get(key + ":used").Int64()
        quota := qm.getQuota(key)
        if quota != nil && float64(used) / float64(quota.Limit) >= quota.AlertAtPct {
            qm.sendAlert(accountID, key, used, quota.Limit)
        }
    }
}
```

**Dynamic Borrowing Example:**

```yaml
# Production environment has reserved 500 TPS, but currently using only 200 TPS
# Dev environment has reserved 100 TPS, but needs 150 TPS

quotas:
  - account_id: acct-123
    environment: production
    quota_type: reserved
    limit_dimension: tps
    limit_value: 500
    allow_borrow: true    # Can lend to siblings
    max_borrow_pct: 0.3   # Can lend up to 30% (150 TPS)

  - account_id: acct-123
    environment: dev
    quota_type: burstable
    limit_dimension: tps
    limit_value: 100
    allow_borrow: true    # Can borrow from siblings
    max_borrow_pct: 0.5   # Can borrow up to 50% extra (50 TPS)

# Result: Dev borrows 50 TPS from Production's idle capacity
# Production still has 450 TPS available (500 - 50 borrowed)
# Dev gets 150 TPS (100 reserved + 50 borrowed)
```

### 3.5 Capacity Management & Preemption

**Capacity Tracking:**

```go
type CapacityManager struct {
    maxTPS       int     // Max tokens per second
    maxQPS       int     // Max queries per second

    currentTPS   int64   // Atomic counter
    currentQPS   int64   // Atomic counter

    // Per-priority capacity allocation
    reservedTPS  map[int]int  // priority → reserved TPS

    // Active requests (for preemption)
    activeRequests sync.Map  // requestID → *Request
}

func (cm *CapacityManager) TryAcquire(req *Request) bool {
    // 1. Check global capacity
    if atomic.LoadInt64(&cm.currentTPS) >= int64(cm.maxTPS) {
        // Try preemption
        return cm.tryPreempt(req)
    }

    // 2. Check priority-specific reserved capacity
    if reserved, ok := cm.reservedTPS[req.Priority]; ok {
        priorityUsed := cm.getPriorityTPS(req.Priority)
        if priorityUsed >= reserved {
            // This priority is over its reserved capacity
            // Can still proceed if global capacity available
            if atomic.LoadInt64(&cm.currentTPS) >= int64(cm.maxTPS) {
                return false
            }
        }
    }

    // 3. Acquire
    atomic.AddInt64(&cm.currentTPS, int64(req.EstimatedTokens))
    atomic.AddInt64(&cm.currentQPS, 1)

    cm.activeRequests.Store(req.ID, req)

    return true
}

func (cm *CapacityManager) Release(req *Request, actualTokens int64) {
    // Adjust capacity (actual may differ from estimated)
    delta := actualTokens - int64(req.EstimatedTokens)
    atomic.AddInt64(&cm.currentTPS, delta)
    atomic.AddInt64(&cm.currentQPS, -1)

    cm.activeRequests.Delete(req.ID)
}

func (cm *CapacityManager) tryPreempt(newReq *Request) bool {
    // Only if new request is high priority
    if newReq.Priority >= 2 {
        return false  // Don't preempt for standard/low priority
    }

    // Find lowest-priority cancelable request
    var victimReq *Request
    lowestPriority := -1

    cm.activeRequests.Range(func(key, value interface{}) bool {
        req := value.(*Request)

        if req.Cancelable && req.Priority > newReq.Priority {
            if lowestPriority == -1 || req.Priority > lowestPriority {
                victimReq = req
                lowestPriority = req.Priority
            }
        }

        return true
    })

    if victimReq == nil {
        return false
    }

    // Preempt
    log.Info("Preempting request",
        "victim_id", victimReq.ID,
        "victim_priority", victimReq.Priority,
        "new_priority", newReq.Priority)

    victimReq.CancelFunc()

    cm.Release(victimReq, 0)

    // Emit metric
    metrics.PreemptionsTotal.WithLabelValues(
        strconv.Itoa(victimReq.Priority),
        strconv.Itoa(newReq.Priority),
    ).Inc()

    return true
}
```

**Rate Limiter (Token Bucket per Queue):**

```go
type TokenBucket struct {
    capacity   int64        // Max tokens
    tokens     int64        // Current tokens
    refillRate int64        // Tokens per second
    lastRefill time.Time
    mu         sync.Mutex
}

func (tb *TokenBucket) TryConsume(amount int64) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    // Refill
    now := time.Now()
    elapsed := now.Sub(tb.lastRefill).Seconds()
    refill := int64(elapsed * float64(tb.refillRate))

    tb.tokens = min(tb.capacity, tb.tokens + refill)
    tb.lastRefill = now

    // Consume
    if tb.tokens >= amount {
        tb.tokens -= amount
        return true
    }

    return false
}
```

### 3.6 Time-Window Based Scheduling

Time-window scheduling dynamically adjusts priorities, quotas, and capacity based on temporal patterns (time of day, day of week, special events).

#### 3.6.1 Time Window Definition

```sql
-- Extension to scheduling configuration
CREATE TABLE time_windows (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name              TEXT NOT NULL,

  -- Schedule (cron or one-time)
  schedule_type     TEXT NOT NULL,  -- cron | one_time
  schedule_expr     TEXT NOT NULL,  -- "0 9 * * 1-5" or ISO timestamp
  duration_sec      BIGINT NOT NULL,

  -- What to adjust
  target_type       TEXT NOT NULL,  -- environment | account_tier | workload_tag | all
  target_value      TEXT,            -- "production" | "gold" | "batch"

  -- Adjustments (all optional, applied multiplicatively or absolutely)
  priority_override INT,             -- Absolute priority (0-4)
  priority_delta    INT,             -- Relative adjustment (+1, -2)

  quota_multiplier  NUMERIC,         -- e.g., 2.0 = double quota
  capacity_multiplier NUMERIC,       -- e.g., 0.5 = half capacity

  weight_multiplier NUMERIC,         -- For WFQ scheduler

  queue_timeout_multiplier NUMERIC,  -- Allow longer/shorter waits
  max_queue_depth_multiplier NUMERIC,

  cost_multiplier   NUMERIC,         -- Pass-through pricing adjustment

  -- Metadata
  enabled           BOOLEAN DEFAULT true,
  created_at        TIMESTAMPTZ DEFAULT now(),
  updated_at        TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_time_windows_schedule ON time_windows(schedule_type, enabled);
```

#### 3.6.2 Time Window Matcher

```go
type TimeWindowManager struct {
    windows     []*TimeWindow
    currentRules map[string]*WindowRule  // Cache of active rules
    mu          sync.RWMutex
}

type TimeWindow struct {
    ID           string
    Name         string
    ScheduleType string  // "cron" | "one_time"
    ScheduleExpr string
    Duration     time.Duration

    Rules        []*WindowRule
    Enabled      bool
}

type WindowRule struct {
    TargetType   string   // "environment" | "account_tier" | "workload_tag"
    TargetValue  string

    PriorityOverride     *int
    PriorityDelta        *int
    QuotaMultiplier      *float64
    CapacityMultiplier   *float64
    WeightMultiplier     *float64
    QueueTimeoutMultiplier *float64
    MaxQueueDepthMultiplier *float64
    CostMultiplier       *float64
}

func (twm *TimeWindowManager) Start() {
    // Goroutine to check active windows every minute
    ticker := time.NewTicker(1 * time.Minute)

    go func() {
        for range ticker.C {
            twm.evaluateWindows()
        }
    }()
}

func (twm *TimeWindowManager) evaluateWindows() {
    twm.mu.Lock()
    defer twm.mu.Unlock()

    now := time.Now()
    activeRules := make(map[string]*WindowRule)

    for _, window := range twm.windows {
        if !window.Enabled {
            continue
        }

        // Check if window is currently active
        if window.ScheduleType == "cron" {
            schedule := cronexpr.MustParse(window.ScheduleExpr)
            lastTrigger := schedule.Previous(now)
            windowEnd := lastTrigger.Add(window.Duration)

            if now.Before(windowEnd) {
                // Window is active
                for _, rule := range window.Rules {
                    key := fmt.Sprintf("%s:%s", rule.TargetType, rule.TargetValue)
                    activeRules[key] = rule

                    log.Info("Time window active",
                        "window", window.Name,
                        "target", key,
                        "ends_at", windowEnd)
                }
            }
        } else if window.ScheduleType == "one_time" {
            startTime, _ := time.Parse(time.RFC3339, window.ScheduleExpr)
            endTime := startTime.Add(window.Duration)

            if now.After(startTime) && now.Before(endTime) {
                for _, rule := range window.Rules {
                    key := fmt.Sprintf("%s:%s", rule.TargetType, rule.TargetValue)
                    activeRules[key] = rule
                }
            }
        }
    }

    // Update active rules
    twm.currentRules = activeRules

    // Emit metrics
    metrics.TimeWindowsActive.Set(float64(len(activeRules)))
}

func (twm *TimeWindowManager) ApplyAdjustments(
    targetType, targetValue string,
    basePriority int,
    baseQuota int64,
    baseCapacity int64,
) (int, int64, int64) {
    twm.mu.RLock()
    defer twm.mu.RUnlock()

    key := fmt.Sprintf("%s:%s", targetType, targetValue)
    rule, exists := twm.currentRules[key]

    if !exists {
        return basePriority, baseQuota, baseCapacity
    }

    // Apply priority adjustment
    priority := basePriority
    if rule.PriorityOverride != nil {
        priority = *rule.PriorityOverride
    } else if rule.PriorityDelta != nil {
        priority = clamp(priority + *rule.PriorityDelta, 0, 4)
    }

    // Apply quota multiplier
    quota := baseQuota
    if rule.QuotaMultiplier != nil {
        quota = int64(float64(baseQuota) * *rule.QuotaMultiplier)
    }

    // Apply capacity multiplier
    capacity := baseCapacity
    if rule.CapacityMultiplier != nil {
        capacity = int64(float64(baseCapacity) * *rule.CapacityMultiplier)
    }

    return priority, quota, capacity
}
```

#### 3.6.3 Integration with Priority Classifier

```go
func (pc *PriorityClassifier) ClassifyWithTimeWindow(req *Request) int {
    // 1. Get base priority from rules
    basePriority := pc.Classify(req)

    // 2. Apply time-window adjustments
    adjustedPriority, _, _ := timeWindowMgr.ApplyAdjustments(
        "environment",
        req.Environment,
        basePriority,
        0, 0,  // quota/capacity not adjusted here
    )

    return adjustedPriority
}
```

#### 3.6.4 Integration with Quota Manager

```go
func (qm *QuotaManager) CheckAndReserveWithTimeWindow(
    accountID, environment string,
    estimatedTokens int64,
) error {
    // Get base quota
    baseQuota := qm.getBaseQuota(accountID, environment)

    // Apply time-window multiplier
    _, adjustedQuota, _ := timeWindowMgr.ApplyAdjustments(
        "environment",
        environment,
        0,  // priority not adjusted here
        baseQuota,
        0,
    )

    // Use adjusted quota for check
    return qm.checkWithQuota(accountID, environment, estimatedTokens, adjustedQuota)
}
```

#### 3.6.5 Dynamic Configuration Reload

```go
func (twm *TimeWindowManager) ReloadConfig() error {
    // Load from database
    rows, err := db.Query(`
        SELECT id, name, schedule_type, schedule_expr, duration_sec,
               target_type, target_value,
               priority_override, priority_delta,
               quota_multiplier, capacity_multiplier,
               weight_multiplier, cost_multiplier,
               queue_timeout_multiplier, max_queue_depth_multiplier
        FROM time_windows
        WHERE enabled = true
    `)

    if err != nil {
        return err
    }
    defer rows.Close()

    newWindows := []*TimeWindow{}

    for rows.Next() {
        var tw TimeWindow
        var rule WindowRule

        rows.Scan(
            &tw.ID, &tw.Name, &tw.ScheduleType, &tw.ScheduleExpr, &tw.Duration,
            &rule.TargetType, &rule.TargetValue,
            &rule.PriorityOverride, &rule.PriorityDelta,
            &rule.QuotaMultiplier, &rule.CapacityMultiplier,
            &rule.WeightMultiplier, &rule.CostMultiplier,
            &rule.QueueTimeoutMultiplier, &rule.MaxQueueDepthMultiplier,
        )

        tw.Rules = append(tw.Rules, &rule)
        newWindows = append(newWindows, &tw)
    }

    twm.mu.Lock()
    twm.windows = newWindows
    twm.mu.Unlock()

    log.Info("Time windows reloaded", "count", len(newWindows))

    // Immediately re-evaluate
    twm.evaluateWindows()

    return nil
}
```

#### 3.6.6 Example: Business Hours vs Off-Hours

**Configuration:**

```yaml
time_windows:
  # Business hours: Monday-Friday 9am-6pm
  - name: business_hours
    schedule_type: cron
    schedule_expr: "0 9 * * 1-5"  # Mon-Fri at 9am
    duration: 9h
    rules:
      - target_type: environment
        target_value: production
        priority_override: 1
        quota_multiplier: 1.0
        capacity_multiplier: 1.0

      - target_type: environment
        target_value: dev
        priority_override: 3
        quota_multiplier: 0.5  # Reduce dev quota during business hours

      - target_type: workload_tag
        target_value: batch
        priority_override: 4
        quota_multiplier: 0.2  # Severely restrict batch

  # Off hours: Nights and weekends
  - name: off_hours
    schedule_type: cron
    schedule_expr: "0 18 * * *"  # Daily at 6pm
    duration: 15h
    rules:
      - target_type: environment
        target_value: production
        priority_override: 1
        quota_multiplier: 0.6  # Less production traffic expected

      - target_type: environment
        target_value: dev
        priority_override: 2  # Boost dev at night
        quota_multiplier: 2.0  # Double dev quota

      - target_type: workload_tag
        target_value: batch
        priority_override: 2  # Batch gets high priority at night!
        quota_multiplier: 5.0  # 5x batch quota
```

**Effect:**
- During business hours (9am-6pm Mon-Fri):
  - Production: Priority 1, full quota
  - Dev: Priority 3, 50% quota
  - Batch: Priority 4, 20% quota

- During off-hours (6pm-9am + weekends):
  - Production: Priority 1, 60% quota (less traffic expected)
  - Dev: Priority 2 (boosted!), 200% quota
  - Batch: Priority 2 (boosted!), 500% quota

**Result:** Maximize cluster utilization by shifting batch workloads to nights/weekends.

#### 3.6.7 Example: Black Friday Event

**Configuration:**

```yaml
time_windows:
  - name: black_friday_2025
    schedule_type: one_time
    schedule_expr: "2025-11-24T00:00:00Z"
    duration: 604800  # 7 days (168 hours)
    rules:
      - target_type: environment
        target_value: production
        priority_override: 0  # Critical!
        quota_multiplier: 5.0  # 5x quota
        capacity_multiplier: 1.0  # Full capacity

      - target_type: environment
        target_value: staging
        priority_override: 3  # Deprioritize
        quota_multiplier: 0.2  # Reduce to 20%

      - target_type: environment
        target_value: dev
        priority_override: 4  # Lowest priority
        quota_multiplier: 0.1  # Reduce to 10%

      - target_type: workload_tag
        target_value: batch
        priority_override: 4
        quota_multiplier: 0.05  # Nearly disabled
```

**Admin API to create event:**

```bash
POST /v1/admin/time-windows

{
  "name": "black_friday_2025",
  "schedule_type": "one_time",
  "schedule_expr": "2025-11-24T00:00:00Z",
  "duration_sec": 604800,
  "rules": [
    {
      "target_type": "environment",
      "target_value": "production",
      "priority_override": 0,
      "quota_multiplier": 5.0
    }
  ]
}
```

---

## 4. Configuration & API

### 4.1 Gateway Configuration

```yaml
# config/scheduling.yaml
scheduling:
  enabled: true
  policy: hybrid  # strict_priority | weighted_fair | hybrid

  queues:
    - level: 0
      name: critical
      max_depth: 100
      timeout_sec: 10
      weight: 10.0
      enable_subqueues: false

    - level: 1
      name: high
      max_depth: 500
      timeout_sec: 30
      weight: 5.0
      enable_subqueues: true  # Split by environment

    - level: 2
      name: standard
      max_depth: 1000
      timeout_sec: 60
      weight: 2.0
      enable_subqueues: true

    - level: 3
      name: low
      max_depth: 2000
      timeout_sec: 120
      weight: 1.0
      enable_subqueues: false

    - level: 4
      name: batch
      max_depth: 5000
      timeout_sec: 300
      weight: 0.5
      enable_subqueues: false

  capacity:
    max_tps: 10000           # Global capacity (tokens/sec)
    max_qps: 1000            # Global capacity (requests/sec)

    reserved_tps:            # Per-priority reserved capacity
      0: 1000                # Critical gets 1000 TPS reserved
      1: 5000                # High gets 5000 TPS reserved

    preemption_enabled: true
    preempt_priorities: [3, 4]  # Only batch/low can be preempted

  priority_classifier:
    default_priority: 2

    rules:
      - condition: environment
        value: production
        priority: 1
        weight: 5.0

      - condition: environment
        value: dev
        priority: 3
        weight: 1.0

      - condition: account_tier
        value: gold
        priority: 1
        weight: 3.0

      - condition: header
        key: X-Priority
        value: critical
        priority: 0
        require_admin: true

  quotas:
    enabled: true
    check_on_enqueue: true
    redis_keys_ttl_days: 31
    alert_webhook_url: https://alerts.example.com/quota
```

### 4.2 API Extensions

**Request Headers:**

```http
POST /v1/chat/completions
Authorization: Bearer {api_key}
X-Environment: production         # Optional: override default
X-Priority: high                  # Optional: priority hint (validated against rules)
X-Team-ID: team-engineering       # Optional: for team-level quotas
X-Request-Class: realtime         # Optional: realtime | batch | spot

{
  "model": "gpt-4",
  "messages": [...]
}
```

**Response Headers (added by gateway):**

```http
HTTP/1.1 200 OK
X-Queue-Wait-Ms: 45               # Time spent in queue
X-Priority-Level: 1               # Assigned priority
X-Quota-Remaining: 4500000        # Tokens remaining in quota
X-Quota-Alert: false              # Alert if approaching limit
```

**New Admin APIs:**

```bash
# 1. Get queue status
GET /v1/admin/scheduler/status

Response:
{
  "queues": [
    {
      "level": 0,
      "name": "critical",
      "current_depth": 3,
      "total_enqueued": 1205,
      "total_dequeued": 1202,
      "total_timeout": 0,
      "total_dropped": 0,
      "avg_wait_ms": 12
    },
    {
      "level": 1,
      "name": "high",
      "current_depth": 87,
      "subqueues": {
        "production": 65,
        "staging": 22
      },
      "avg_wait_ms": 234
    }
  ],
  "capacity": {
    "current_tps": 7800,
    "max_tps": 10000,
    "utilization_pct": 78.0,
    "active_requests": 156
  },
  "preemptions_total": 3
}

# 2. Update queue configuration (hot reload)
PATCH /v1/admin/scheduler/queues/{level}

{
  "max_depth": 2000,
  "timeout_sec": 90
}

# 3. Emergency: pause queue
POST /v1/admin/scheduler/queues/{level}/pause

# 4. Emergency: clear queue
POST /v1/admin/scheduler/queues/{level}/clear

# 5. Get account quota status
GET /v1/admin/quotas/{account_id}

Response:
{
  "account_id": "acct-123",
  "quotas": [
    {
      "environment": "production",
      "limit_dimension": "tokens_per_month",
      "limit_value": 10000000,
      "used_value": 7800000,
      "usage_pct": 78.0,
      "alert_triggered": false,
      "allow_borrow": true,
      "borrowed_value": 0
    }
  ]
}

# 6. Adjust quota (emergency top-up)
POST /v1/admin/quotas/{account_id}/adjust

{
  "environment": "production",
  "delta_tokens": 5000000,  # Add 5M tokens
  "reason": "Emergency capacity for critical incident"
}
```

---

## 5. Metrics & Observability

### 5.1 Key Metrics

```yaml
# Prometheus metrics

# Queue metrics
tokligence_scheduler_queue_depth{priority="0|1|2|3|4",environment="prod|staging|dev"}
tokligence_scheduler_enqueued_total{priority="0|1|2|3|4"}
tokligence_scheduler_dequeued_total{priority="0|1|2|3|4"}
tokligence_scheduler_timeout_total{priority="0|1|2|3|4"}
tokligence_scheduler_dropped_total{priority="0|1|2|3|4"}

tokligence_scheduler_wait_time_seconds{priority="0|1|2|3|4",quantile="0.5|0.9|0.99"}

# Capacity metrics
tokligence_scheduler_capacity_tps{type="current|max"}
tokligence_scheduler_capacity_qps{type="current|max"}
tokligence_scheduler_utilization_pct

# Preemption
tokligence_scheduler_preemptions_total{victim_priority="3|4",new_priority="0|1"}

# Quota metrics
tokligence_quota_used{account_id,environment,dimension="tokens|usd"}
tokligence_quota_limit{account_id,environment,dimension="tokens|usd"}
tokligence_quota_usage_pct{account_id,environment}
tokligence_quota_alerts_total{account_id,environment,threshold="80|90|100"}
tokligence_quota_borrow_total{account_id,environment}

# Scheduling fairness
tokligence_scheduler_throughput_tokens{priority="0|1|2|3|4"}  # Actual tokens served
tokligence_scheduler_fairness_ratio  # Actual / Target ratio per priority

# Time window metrics
tokligence_time_windows_active  # Number of currently active time windows
tokligence_time_window_applied_total{window_name,target_type,target_value}  # How many times each window activated
tokligence_time_window_priority_adjustments_total{window_name,from_priority,to_priority}
tokligence_time_window_quota_adjustments_total{window_name,multiplier}
```

### 5.2 Alerts

```yaml
# alerts/scheduling.yaml

- alert: QueueDepthHigh
  expr: tokligence_scheduler_queue_depth > 500
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Priority {{ $labels.priority }} queue depth > 500"

- alert: QueueTimeoutSpike
  expr: rate(tokligence_scheduler_timeout_total[5m]) > 10
  for: 3m
  labels:
    severity: critical
  annotations:
    summary: "High rate of queue timeouts ({{ $value }}/s)"

- alert: CapacityExhausted
  expr: tokligence_scheduler_utilization_pct > 95
  for: 10m
  labels:
    severity: critical
  annotations:
    summary: "Capacity utilization > 95% for 10 minutes"

- alert: QuotaExceeded
  expr: tokligence_quota_usage_pct > 100
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Account {{ $labels.account_id }} exceeded quota"

- alert: QuotaWarning
  expr: tokligence_quota_usage_pct > 80
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Account {{ $labels.account_id }} at {{ $value }}% quota"

- alert: SchedulingUnfair
  expr: tokligence_scheduler_fairness_ratio < 0.5 OR tokligence_scheduler_fairness_ratio > 2.0
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "Scheduling fairness violation for priority {{ $labels.priority }}"
```

---

## 6. Implementation Phases

### Phase 1: Basic Priority Queues (Week 1-2)
**Goal:** Single-dimension priority classification

**Deliverables:**
- [ ] Implement configurable priority queues (default: 10 levels P0-P9, configurable via TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS)
- [ ] Environment-based priority rules (prod=P1, dev=P3)
- [ ] Strict priority scheduler
- [ ] Queue depth/timeout configuration
- [ ] Basic metrics (queue depth, wait time)

**Testing:**
- Create dev/prod requests, verify prod gets served first
- Fill queue to max_depth, verify 429 rejection
- Let request timeout, verify 503 response

### Phase 2: Weighted Fair Queuing (Week 3-4)
**Goal:** Fairness and multi-tenant support

**Deliverables:**
- [ ] Weighted Fair Queuing scheduler
- [ ] Hybrid scheduler (P0 strict, P1-P4 WFQ)
- [ ] Per-environment sub-queues
- [ ] Fairness metrics

**Testing:**
- Send mixed priority traffic, measure throughput ratio
- Verify no starvation (low-priority eventually served)
- Benchmark: 10K RPS with 3 priority levels

### Phase 3: Hierarchical Quotas (Week 5-7)
**Goal:** Budget management across org/team/environment

**Deliverables:**
- [ ] Quota schema (hierarchical)
- [ ] Redis-backed quota tracking
- [ ] Pre-check on enqueue
- [ ] Commit on completion
- [ ] Dynamic borrowing logic
- [ ] Alert webhooks (80%, 90%, 100%)

**Testing:**
- Set org quota to 1M tokens/month
- Allocate 500K to prod, 300K to staging, 200K to dev
- Verify prod can borrow from staging if idle
- Trigger alert at 80% usage

### Phase 4: Time-Window Scheduling (Week 8-10)
**Goal:** Dynamic priority/quota based on time patterns

**Deliverables:**
- [ ] Time window schema and database table
- [ ] TimeWindowManager with cron/one-time scheduling
- [ ] Integration with PriorityClassifier
- [ ] Integration with QuotaManager
- [ ] Dynamic configuration reload
- [ ] Time window metrics

**Testing:**
- Create business hours time window (9am-6pm)
- Verify batch priority drops to P4 during business hours
- Verify batch priority boosts to P2 at night
- Create one-time event (Black Friday)
- Verify multipliers apply correctly

### Phase 5: Capacity Preemption (Week 11-12)
**Goal:** Emergency handling and SLA protection

**Deliverables:**
- [ ] Capacity manager (TPS/QPS tracking)
- [ ] Token bucket rate limiter
- [ ] Preemption logic (cancel streaming requests)
- [ ] Preemption metrics

**Testing:**
- Fill capacity with P3 batch jobs
- Send P0 critical request
- Verify P3 request gets preempted
- Verify P0 request completes

### Phase 6: Production Hardening (Week 13-15)
**Goal:** Observability, hot reload, admin tools

**Deliverables:**
- [ ] Grafana dashboards (queue status, capacity, quotas, time windows)
- [ ] Admin APIs (pause/clear queue, adjust quota, manage time windows)
- [ ] Hot reload configuration
- [ ] Load testing (100K RPS, 1M active quotas, 100 time windows)
- [ ] Documentation

---

## 7. Advanced Features (Future)

### 7.1 Adaptive Priority

Automatically adjust priority based on latency SLO violations:

```go
// If P1 queue wait time > 30s, temporarily boost to P0
if queue[1].AvgWaitTime() > 30 * time.Second {
    priorityBoost[1] = 0  // Temporary promotion
}
```

### 7.2 Cost-Aware Scheduling

Route requests to cheapest available queue:

```yaml
routing_policy: cost_aware

# Prefer spot queue (cheap), fallback to reserved (expensive)
queues:
  - name: spot
    cost_multiplier: 0.7
    max_wait_sec: 120

  - name: reserved
    cost_multiplier: 1.0
    max_wait_sec: 10
```

### 7.3 Predictive Quota Scaling

Use ML to predict quota needs:

```python
# Predict next week's quota needs based on historical usage
predicted_quota = predict_usage(
    account_id=account_id,
    forecast_days=7,
    confidence=0.95
)

# Auto-scale quota
if predicted_quota > current_quota * 1.2:
    send_alert("Consider increasing quota for account {account_id}")
```

### 7.4 Multi-Region Quota Pooling

Share quota across regions:

```yaml
quota_pooling:
  enabled: true
  regions: [us-east-1, eu-west-1, ap-southeast-1]

  # EU region can borrow up to 30% from US if idle
  borrow_rules:
    - from: us-east-1
      to: eu-west-1
      max_borrow_pct: 0.3
```

---

## 8. Integration with Existing Architecture

### 8.1 Marketplace Integration

The scheduling system integrates with Tokligence Marketplace for quota sync:

```
Gateway (Scheduler)                 Marketplace
       │                                   │
       │  1. Periodic quota sync           │
       ├──────────────────────────────────►│
       │  GET /v1/quotas/{account_id}      │
       │                                   │
       │◄──────────────────────────────────┤
       │  { quotas: [...] }                │
       │                                   │
       │  2. Real-time usage report        │
       ├──────────────────────────────────►│
       │  POST /v1/usage/batch             │
       │  { events: [...] }                │
       │                                   │
       │◄──────────────────────────────────┤
       │  { accepted: [...],               │
       │    rejected: [...],               │
       │    quota_remaining: 450000 }      │
```

**Local Cache + Remote Sync:**
- Gateway caches quotas in Redis (TTL 5 min)
- Optimistic enforcement locally
- Async reconciliation with Marketplace
- Marketplace is source of truth for billing

### 8.2 Existing Routing System

Scheduler sits **before** existing routing engine:

```
Request → Scheduler → Routing Engine → Provider
          (queue)     (choose provider)
```

**No changes to routing logic**, scheduler just delays low-priority requests.

### 8.3 Backward Compatibility

```yaml
# Disable scheduling for gradual rollout
scheduling:
  enabled: false  # Default: passthrough mode
```

When disabled, all requests bypass queues (zero latency overhead).

---

## 9. Performance Considerations

### 9.1 Latency Overhead

**Target:** < 5ms P99 scheduling latency (queue check + dequeue)

**Optimizations:**
- Lock-free queue implementations (ring buffer)
- Batch dequeue (dequeue 10 at once, distribute to workers)
- Priority queue index (binary heap, O(log N) enqueue/dequeue)

### 9.2 Memory Footprint

**Estimate:**
- 1K requests in queue × 1KB per request = 1MB
- 10K active quotas × 500B per quota = 5MB
- **Total:** ~10MB per gateway instance (negligible)

### 9.3 Redis Load

**Quota checks:**
- 1000 QPS × 4 hierarchy levels = 4000 Redis GET/INCRBY ops/sec
- Use Redis pipelining: batch 100 ops → 40 pipeline requests/sec
- **Load:** Negligible for Redis (can handle 100K ops/sec)

---

## 10. Security & Compliance

### 10.1 Priority Abuse Prevention

**Risk:** Malicious user sends all requests with `X-Priority: critical`

**Mitigation:**
```go
func validatePriorityHeader(req *Request) error {
    if req.Header.Get("X-Priority") == "critical" {
        if !isAdmin(req) {
            return ErrUnauthorized
        }
    }

    // Rate limit priority overrides per account
    key := fmt.Sprintf("priority_override:%s", req.AccountID)
    count := redis.Incr(key)
    redis.Expire(key, 1 * time.Minute)

    if count > 10 {  // Max 10 priority overrides per minute
        return ErrRateLimitExceeded
    }

    return nil
}
```

### 10.2 Quota Bypass Prevention

**Risk:** User exploits quota borrowing to get unlimited tokens

**Mitigation:**
- Max borrow % is hard-capped (e.g., 50%)
- Borrowed capacity is reclaimed when parent needs it
- Audit log all borrowing events

### 10.3 Fair Scheduling Audit

**Compliance:** Prove to regulators that scheduling is fair (no discrimination)

**Audit Trail:**
```go
// Log every scheduling decision
auditLog.Info("Request scheduled",
    "request_id", req.ID,
    "account_id", req.AccountID,
    "priority", req.Priority,
    "wait_time_ms", waitTime,
    "queue_position", position,
    "timestamp", time.Now(),
)
```

Export to immutable log (S3, ClickHouse) for compliance.

---

## 11. References & Prior Art

### 11.1 Research Sources

This design is informed by:

1. **LiteLLM Scheduler** - Priority queuing for multi-tenant LLM proxies
   - [LiteLLM Scheduler Documentation](https://docs.litellm.ai/docs/scheduler)
   - [Feature Request: Priority-Based Request Handling](https://github.com/BerriAI/litellm/issues/13405)

2. **Kubernetes Pod Priority** - Industry-standard preemption patterns
   - [Pod Priority and Preemption | Kubernetes](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/)
   - [Pod Priority Blog | Kubernetes](https://kubernetes.io/blog/2019/04/16/pod-priority-and-preemption-in-kubernetes/)

3. **Network QoS Algorithms** - Fair queuing and token bucket
   - [Token Bucket Rate Limiting](https://intronetworks.cs.luc.edu/current/uhtml/tokenbucket.html)
   - [Weighted Fair Queuing](https://intronetworks.cs.luc.edu/current/uhtml/fairqueuing.html)

4. **LLM Gateway Best Practices**
   - [Rate Limiting in AI Gateway | TrueFoundry](https://www.truefoundry.com/blog/rate-limiting-in-llm-gateway)
   - [Apache APISIX AI Gateway Features](https://apisix.apache.org/blog/2025/02/24/apisix-ai-gateway-features/)
   - [Higress AI Gateway Documentation](https://higress.ai/en/)

### 11.2 Competitive Analysis

| Feature | Tokligence (Proposed) | LiteLLM | Azure API Mgmt | Apache APISIX |
|---------|----------------------|---------|----------------|---------------|
| **Priority Levels** | 5 (P0-P4) | Numeric (lower=higher) | Not native | Custom |
| **Scheduling Algorithm** | Hybrid (strict + WFQ) | Priority queue | FIFO | Round-robin |
| **Hierarchical Quotas** | Yes (org/team/env/user) | Per-key only | Per-user | Per-consumer |
| **Environment Isolation** | Yes (sub-queues) | No | No | No |
| **Dynamic Borrowing** | Yes | No | No | No |
| **Preemption** | Yes (configurable) | No | No | No |
| **Self-hosted LLM Support** | Yes (core use case) | Yes | Limited | Yes |
| **Token-level Quota** | Yes | Yes | Yes | Yes |
| **Fair Queuing** | Yes (WFQ) | No | No | Limited |

**Competitive Advantage:**
1. **Only solution designed for self-hosted LLM sellers** - Multi-tenant quota + environment isolation
2. **Hierarchical quota borrowing** - Maximize resource utilization
3. **True weighted fair queuing** - Provably fair scheduling
4. **Preemption for SLA protection** - Critical workloads never starve

---

## 12. Success Criteria

### 12.1 Functional Requirements

- [ ] Support 5 priority levels with configurable queue depth/timeout
- [ ] Implement 3 scheduling algorithms (strict, WFQ, hybrid)
- [ ] Enforce hierarchical quotas across 4 levels (org/team/env/user)
- [ ] Dynamic quota borrowing with 50% max
- [ ] Preemption of low-priority requests when critical arrives
- [ ] Environment-based sub-queues (prod/staging/dev isolation)
- [ ] Admin APIs for queue control and quota adjustment
- [ ] Real-time metrics for queue depth, wait time, quota usage

### 12.2 Performance Requirements

- [ ] Scheduling overhead: < 5ms P99
- [ ] Throughput: Support 10K RPS with 5 priority levels
- [ ] Queue capacity: 10K concurrent requests across all queues
- [ ] Quota check latency: < 2ms P99 (Redis)
- [ ] Memory footprint: < 100MB for 10K queued requests

### 12.3 Business Metrics

- [ ] Enable 3-tier pricing (Gold/Silver/Bronze) with measurable SLA differences
- [ ] Reduce production incident rate by 50% (via priority protection)
- [ ] Increase resource utilization by 30% (via quota borrowing)
- [ ] Support 100+ tenants on single gateway instance

---

## 13. Next Steps

1. **Week 1-2:** Implement Phase 1 (basic priority queues)
2. **Week 3-4:** Implement Phase 2 (WFQ scheduler)
3. **Week 5-7:** Implement Phase 3 (hierarchical quotas)
4. **Week 8-9:** Implement Phase 4 (preemption)
5. **Week 10-12:** Production hardening + documentation
6. **Week 13+:** Advanced features (adaptive priority, cost-aware routing)

**Parallel Tracks:**
- **Frontend:** Add quota management UI to admin portal
- **Marketplace:** Extend quota APIs for borrowing/alerts
- **Testing:** Load testing framework for 100K RPS scenarios
- **Documentation:** User guide, configuration reference, migration guide

---

## Appendix A: Configuration Examples

### A.1 Multi-Environment Setup

```yaml
# Typical SaaS company: prod/staging/dev
scheduling:
  enabled: true
  policy: hybrid

  priority_classifier:
    rules:
      - condition: environment
        value: production
        priority: 1
        weight: 5.0

      - condition: environment
        value: staging
        priority: 2
        weight: 2.0

      - condition: environment
        value: dev
        priority: 3
        weight: 1.0

quotas:
  - account_id: org-acme
    environment: production
    quota_type: reserved
    limit_dimension: tps
    limit_value: 500
    allow_borrow: false  # Production never lends

  - account_id: org-acme
    environment: staging
    quota_type: burstable
    limit_dimension: tps
    limit_value: 200
    allow_borrow: true
    max_borrow_pct: 0.3  # Can borrow 30% from prod

  - account_id: org-acme
    environment: dev
    quota_type: burstable
    limit_dimension: tps
    limit_value: 100
    allow_borrow: true
    max_borrow_pct: 0.5
```

### A.2 Multi-Tenant Marketplace

```yaml
# Gateway operator sells to 10 customers
scheduling:
  policy: weighted_fair

  priority_classifier:
    rules:
      - condition: account_tier
        value: gold
        priority: 1
        weight: 3.0

      - condition: account_tier
        value: silver
        priority: 2
        weight: 2.0

      - condition: account_tier
        value: bronze
        priority: 3
        weight: 1.0

quotas:
  # Gold customer: 1M tokens/month, 1000 TPS
  - account_id: customer-gold-1
    quota_type: hard
    limit_dimension: tokens_per_month
    limit_value: 1000000

  - account_id: customer-gold-1
    quota_type: reserved
    limit_dimension: tps
    limit_value: 1000

  # Bronze customer: 100K tokens/month, 100 TPS
  - account_id: customer-bronze-1
    quota_type: hard
    limit_dimension: tokens_per_month
    limit_value: 100000

  - account_id: customer-bronze-1
    quota_type: burstable
    limit_dimension: tps
    limit_value: 100
```

---

**Document Status:** Ready for review
**Authors:** Tokligence Architecture Team
**Reviewers:** Gateway Team, Marketplace Team
**Approval:** Pending
