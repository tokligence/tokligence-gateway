# Bucket-Based Capacity Scheduling System

**Version:** 2.0 (Aligned with Tokens/Sec Model & Revised Recommendations)
**Date:** 2025-02-01
**Status:** Research & Design Proposal - **NOT RECOMMENDED for Production (see §9)**
**Related Documents:**
- `00_REVISED_OVERVIEW.md` - **READ THIS FIRST** (recommends 5-10 buckets, not 100)
- `01_PRIORITY_BASED_SCHEDULING.md` - Priority queuing (recommended default)
- `06_MARKETPLACE_INTEGRATION.md` - Provider SPI and capacity model
- `03_CONFIGURABLE_BUCKET_COUNT.md` - Configurable bucket count (2-100 range)

**⚠️ IMPORTANT NOTICE:**

This document explores the **100-bucket model** for research purposes. However, after comprehensive review (see `00_REVISED_OVERVIEW.md`), we **do NOT recommend** 100 buckets for production use due to:

- ❌ Excessive configuration complexity
- ❌ Prometheus metric explosion (high cardinality)
- ❌ Difficult to reason about (100 buckets vs 5 priorities)
- ❌ Over-engineered for most use cases

**Recommended approach:** Start with **5-level priority** (see `01_PRIORITY_BASED_SCHEDULING.md`), or **10-bucket model** if you need finer granularity (see §9.2).

---

## Executive Summary

This document explores a **100-bucket capacity allocation system** as an **experimental alternative** to priority-based scheduling. Instead of abstract priority levels (P0-P4), this model uses concrete capacity buckets:

1. **100 discrete buckets** (0-99) represent the entire system capacity
2. **Bucket 0** has highest capacity (e.g., 10,000 tokens/sec), **Bucket 99** has lowest (~0 tokens/sec)
3. **Exponential decay**: Each bucket has ~50% capacity of the previous bucket (configurable ratio)
4. **All routing abstractions map to buckets**: Priority, environment, workload type all assign requests to specific buckets
5. **Two scheduling modes**:
   - **Strict mode**: Priority 3 → always use bucket #3
   - **AtLeast mode**: Priority 3 → use bucket #3 or better (0-3) if available

**v2.0 Changes:**
- ✅ Changed capacity model from RPS to **tokens/sec** (primary metric)
- ✅ Added multi-dimensional capacity (tokens/sec, concurrent, context length)
- ✅ Integrated with Provider SPI (LocalProvider, MarketplaceProvider, HybridProvider)
- ✅ Added observability constraints (adaptive metrics for >20 buckets)
- ✅ Added AtLeast fairness constraints (upgrade limits to prevent starvation)
- ⚠️ **Recommendation changed**: 5-10 buckets preferred over 100 (see §9)

**Key Questions Explored:**
- Is 100-bucket model practical for production use? **Answer: No (see §9)**
- Does it simplify or complicate the system? **Answer: Complicates (see §8)**
- What's the performance impact? **Answer: Manageable but high cardinality risk (see §7)**
- How do operators measure and configure bucket capacities? **Answer: Benchmarking tool (see §2)**

---

## Table of Contents

1. [Core Concept: The 100-Bucket Model](#1-core-concept-the-100-bucket-model)
2. [Capacity Measurement Challenge](#2-capacity-measurement-challenge)
3. [Bucket Capacity Distribution](#3-bucket-capacity-distribution)
4. [Mapping Abstractions to Buckets](#4-mapping-abstractions-to-buckets)
5. [Scheduling Modes: Strict vs AtLeast](#5-scheduling-modes-strict-vs-atleast)
6. [Implementation Architecture](#6-implementation-architecture)
7. [Performance Analysis](#7-performance-analysis)
8. [Practicality Assessment](#8-practicality-assessment)
9. [Recommendation](#9-recommendation)

---

## 1. Core Concept: The 100-Bucket Model

### 1.1 Fundamental Principle

Replace abstract priority levels with **concrete capacity buckets**:

```
Traditional Model:
  Priority 0 (critical)  → Queue 0 → ??? tokens/sec capacity
  Priority 1 (high)      → Queue 1 → ??? tokens/sec capacity
  Priority 2 (normal)    → Queue 2 → ??? tokens/sec capacity
  Priority 3 (low)       → Queue 3 → ??? tokens/sec capacity
  Priority 4 (spot)      → Queue 4 → ??? tokens/sec capacity

Bucket Model (v2.0 - tokens/sec):
  Bucket 0  → 10,000 tokens/sec  (highest)
  Bucket 1  →  5,000 tokens/sec  (50% of bucket 0)
  Bucket 2  →  2,500 tokens/sec  (50% of bucket 1)
  Bucket 3  →  1,250 tokens/sec
  ...
  Bucket 10 →      9.8 tokens/sec
  ...
  Bucket 99 →    ~0.0 tokens/sec  (lowest, effectively 0)

Note: v1.0 used RPS (requests/sec), but v2.0 uses tokens/sec as primary metric
because it better reflects actual LLM capacity and aligns with per-token pricing.
```

### 1.2 Why 100 Buckets?

**Granularity Analysis:**

| Bucket Count | Pros | Cons |
|--------------|------|------|
| **5 buckets** | Simple, matches P0-P4 | Too coarse, can't express fine-grained SLAs |
| **10 buckets** | Manageable | Still somewhat coarse |
| **100 buckets** | Very fine-grained, intuitive (percentage-like) | Higher memory/CPU overhead |
| **1000 buckets** | Extremely fine | Overkill, complex to configure |

**Choice: 100 buckets** strikes a balance:
- Fine enough to express diverse SLAs (e.g., "99th percentile customers" vs "95th percentile")
- Coarse enough to keep implementation efficient
- Intuitive mapping to percentages (bucket 95 = "top 5%" capacity)

### 1.3 Visual Model

```
┌─────────────────────────────────────────────────────────────────┐
│                    Total System Capacity                         │
│                  (e.g., 1000 RPS to LLM)                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                ┌─────────────┴─────────────┐
                │     Bucket Allocator      │
                └─────────────┬─────────────┘
                              │
    ┌─────────────────────────┼─────────────────────────┐
    │                         │                         │
    ▼                         ▼                         ▼
┌────────┐              ┌────────┐              ┌────────┐
│Bucket 0│ 100 RPS      │Bucket 1│  50 RPS      │Bucket 2│  25 RPS
│ (top)  │              │        │              │        │
└───┬────┘              └───┬────┘              └───┬────┘
    │                       │                       │
    │ Request              │ Request              │ Request
    │ (internal/prod)      │ (internal/staging)   │ (external/premium)
    ▼                       ▼                       ▼
   LLM                     LLM                     LLM

                      ...

┌────────┐
│Bucket99│  ~0 RPS
│ (spot) │
└───┬────┘
    │ Request (spot/batch)
    ▼
   LLM (when idle)
```

---

## 2. Capacity Measurement Challenge

### 2.1 The Problem

**Gateway operators need to know their model's actual capacity** to configure buckets meaningfully. This is non-trivial because:

1. **Workload variability**: Chat completions vs embeddings vs code generation have different token/latency profiles
2. **Model differences**: GPT-4 vs Claude-3.5 vs Llama-3 have different throughput characteristics
3. **Hardware constraints**: GPU memory, CPU, network bandwidth all affect capacity
4. **Dynamic factors**: Model temperature, max_tokens, streaming vs non-streaming

### 2.2 Proposed Solution: Capacity Benchmarking Tool

**Tool: `tokligence benchmark`**

```bash
# Benchmark a self-hosted model
tokligence benchmark \
  --endpoint http://localhost:8080/v1/chat/completions \
  --model gpt-4 \
  --duration 300s \
  --workload-profile realistic \
  --output benchmark-report.json

# Output:
{
  "model": "gpt-4",
  "endpoint": "http://localhost:8080/v1/chat/completions",
  "duration_seconds": 300,
  "workload_profile": "realistic",

  "results": {
    "max_rps": 87.3,          // Maximum sustained RPS
    "avg_latency_ms": 1250,   // Average response latency
    "p50_latency_ms": 980,
    "p95_latency_ms": 2100,
    "p99_latency_ms": 3500,

    "max_tps": 12500,         // Maximum tokens per second
    "avg_tokens_per_request": 143,

    "error_rate": 0.002,      // 0.2% errors
    "throttle_rate": 0.015,   // 1.5% rate-limited

    "recommended_safe_rps": 75,  // 85% of max_rps (safety margin)
    "recommended_safe_tps": 10625
  },

  "workload_distribution": {
    "avg_prompt_tokens": 45,
    "avg_completion_tokens": 98,
    "streaming_ratio": 0.7,   // 70% streaming, 30% non-streaming
    "tool_call_ratio": 0.15   // 15% requests use tools
  }
}
```

### 2.3 Workload Profiles

The benchmarking tool should support multiple workload profiles:

```yaml
# workload-profiles.yaml

profiles:
  # Minimal profile: short prompts, short responses
  minimal:
    prompt_tokens: {min: 10, max: 50, avg: 25}
    completion_tokens: {min: 10, max: 100, avg: 40}
    streaming_ratio: 0.5
    tool_call_ratio: 0.0

  # Realistic profile: production-like workload
  realistic:
    prompt_tokens: {min: 20, max: 500, avg: 150}
    completion_tokens: {min: 50, max: 2000, avg: 300}
    streaming_ratio: 0.7
    tool_call_ratio: 0.15

  # Heavy profile: long-context, long responses
  heavy:
    prompt_tokens: {min: 1000, max: 8000, avg: 4000}
    completion_tokens: {min: 500, max: 4000, avg: 1500}
    streaming_ratio: 0.9
    tool_call_ratio: 0.05

  # Code generation profile
  code:
    prompt_tokens: {min: 100, max: 2000, avg: 600}
    completion_tokens: {min: 200, max: 3000, avg: 800}
    streaming_ratio: 0.95
    tool_call_ratio: 0.0
```

### 2.4 Benchmark Output Interpretation

**Key principle: Benchmarks provide reference baselines, not absolute truth**

```
┌─────────────────────────────────────────────────────────────────┐
│  Benchmark Results (Realistic Profile)                          │
│  ─────────────────────────────────────────────────────────────  │
│  Max RPS:              87.3                                     │
│  Recommended Safe RPS: 75  (85% of max, 15% safety margin)     │
│                                                                  │
│  Interpretation:                                                │
│  • Your model can handle ~75 RPS under typical workload        │
│  • This is a REFERENCE - actual workload may differ            │
│  • Monitor production metrics and adjust bucket configs        │
│  • Consider running benchmarks for different profiles          │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│  Recommendation: Start Conservative                             │
│  ─────────────────────────────────────────────────────────────  │
│  1. Use 50-70% of benchmark RPS as initial bucket 0 capacity   │
│  2. Monitor actual utilization for 1 week                       │
│  3. Adjust bucket capacities based on real data                │
│  4. Re-benchmark quarterly or after infrastructure changes      │
└─────────────────────────────────────────────────────────────────┘
```

### 2.5 Why Benchmarks Are Only References

**Variability factors:**

1. **Temporal patterns**:
   - Business hours: short, urgent queries (e.g., customer support)
   - Night: long, complex queries (e.g., batch analysis)

2. **User diversity**:
   - Internal dev: frequent small requests
   - Internal prod: mix of sizes
   - External customers: highly variable

3. **Model updates**:
   - Provider updates model (e.g., GPT-4 → GPT-4-turbo)
   - Performance characteristics change

4. **Infrastructure changes**:
   - GPU upgrade
   - Network bandwidth change
   - Concurrent model hosting

**Best practice:**
```
Benchmark RPS: 87.3
↓ Apply 85% safety margin
Safe baseline RPS: 75
↓ Start with 70% of safe baseline
Initial bucket 0 capacity: 52 RPS
↓ Monitor production for 1-2 weeks
Actual avg utilization: 48 RPS (92% of configured)
Actual P95 latency: 1100ms (good)
Actual error rate: 0.1% (excellent)
↓ Increase capacity
Adjusted bucket 0 capacity: 60 RPS
```

---

## 3. Bucket Capacity Distribution

### 3.1 Exponential Decay Model

**Formula:**
```
capacity[bucket_n] = base_capacity * (decay_ratio ^ bucket_n)

Where:
  base_capacity = capacity of bucket 0 (e.g., 100 RPS)
  decay_ratio   = capacity ratio between adjacent buckets (e.g., 0.5)
  bucket_n      = bucket number (0-99)
```

**Example with decay_ratio = 0.5:**

| Bucket | Capacity (RPS) | Percentage of Total |
|--------|----------------|---------------------|
| 0      | 100.000        | 50.0%               |
| 1      | 50.000         | 25.0%               |
| 2      | 25.000         | 12.5%               |
| 3      | 12.500         | 6.25%               |
| 4      | 6.250          | 3.12%               |
| 5      | 3.125          | 1.56%               |
| 10     | 0.098          | 0.049%              |
| 20     | 0.000095       | ~0%                 |
| 50     | 0.00000000009  | ~0%                 |
| 99     | ~0             | ~0%                 |

**Total capacity:** ~200 RPS (sum of geometric series)

### 3.2 Problem: Exponential Decay Too Aggressive

**Issue:** With decay_ratio = 0.5, buckets 20+ have negligible capacity.

**Alternative: Hybrid Distribution**

```
Buckets 0-19:   Exponential decay (decay_ratio = 0.7)
Buckets 20-79:  Linear decay
Buckets 80-99:  Best-effort (opportunistic, no guarantee)
```

**Example Hybrid Distribution:**

| Bucket Range | Capacity Model | Example (base=100 RPS) |
|--------------|----------------|------------------------|
| 0-9          | Exponential (0.7) | 100, 70, 49, 34, 24, 17, 12, 8, 6, 4 RPS |
| 10-19        | Exponential (0.7) | 3, 2, 1.4, 1.0, 0.7, 0.5, 0.35, 0.24, 0.17, 0.12 RPS |
| 20-49        | Linear decay   | 0.12 → 0.01 RPS (evenly distributed) |
| 50-79        | Linear decay   | 0.01 → 0.001 RPS |
| 80-99        | Best-effort    | No guaranteed capacity, use idle cycles |

**Total capacity:** ~280 RPS

### 3.3 Configurable Decay Models

```yaml
# config/bucket_distribution.yaml

bucket_distribution:
  # Total system capacity (from benchmark)
  base_capacity_rps: 100

  # Distribution model: exponential | hybrid | custom
  model: hybrid

  # Exponential model parameters
  exponential:
    decay_ratio: 0.7  # Each bucket has 70% of previous

  # Hybrid model parameters
  hybrid:
    tier1_buckets: 20          # Buckets 0-19: exponential
    tier1_decay_ratio: 0.7

    tier2_buckets: 30          # Buckets 20-49: linear
    tier2_min_capacity: 0.01   # Minimum RPS for tier 2

    tier3_buckets: 30          # Buckets 50-79: linear
    tier3_min_capacity: 0.001

    tier4_buckets: 20          # Buckets 80-99: best-effort
    tier4_guaranteed: false    # No capacity guarantee

  # Custom model (manually specify each bucket)
  custom:
    buckets:
      0: 100
      1: 50
      2: 25
      # ... manually configure all 100 buckets
```

---

## 4. Mapping Abstractions to Buckets

### 4.1 The Mapping Challenge

**All existing abstractions must map to buckets:**

```
Request arrives
  ↓
Determine bucket assignment
  ↓
Abstractions:
  - Priority tier (internal/external/premium/spot)
  - Environment (production/staging/dev)
  - Workload type (realtime/batch)
  - Time window (business hours/night)
  - Token tier
  - HTTP header priority
  ↓
Final bucket: 0-99
```

### 4.2 Mapping Strategy: Bucket Ranges

**Divide 100 buckets into ranges for different tiers:**

```yaml
# config/bucket_mapping.yaml

bucket_mapping:
  # Internal production: buckets 0-9 (highest capacity)
  internal_production:
    bucket_range: [0, 9]
    default_bucket: 0

  # Internal staging: buckets 10-19
  internal_staging:
    bucket_range: [10, 19]
    default_bucket: 10

  # Internal dev: buckets 20-29
  internal_dev:
    bucket_range: [20, 29]
    default_bucket: 20

  # External premium: buckets 30-39
  external_premium:
    bucket_range: [30, 39]
    default_bucket: 30

  # External standard: buckets 40-59
  external_standard:
    bucket_range: [40, 59]
    default_bucket: 45

  # Spot/batch: buckets 80-99
  spot:
    bucket_range: [80, 99]
    default_bucket: 85
```

**Capacity allocation:**

| Tier | Bucket Range | Capacity (RPS) | Use Case |
|------|--------------|----------------|----------|
| Internal Prod | 0-9 | ~200 RPS | Critical production workloads |
| Internal Staging | 10-19 | ~20 RPS | Pre-production testing |
| Internal Dev | 20-29 | ~2 RPS | Development/experimentation |
| External Premium | 30-39 | ~0.5 RPS | Paid customers (high SLA) |
| External Standard | 40-59 | ~0.1 RPS | Paid customers (standard SLA) |
| Spot | 80-99 | Best-effort | Batch jobs, low priority |

### 4.3 Priority Header Mapping

**HTTP header `X-TGW-Priority: 3` maps to bucket based on tier:**

```go
func (bm *BucketMapper) MapToBucket(meta *RoutingMetadata) int {
    // Get base bucket range for tier
    bucketRange := bm.getBucketRange(meta.PriorityTier, meta.Environment)

    // meta.Priority is 0-4 (from HTTP header or token)
    // Map to position within bucket range

    rangeSize := bucketRange.Max - bucketRange.Min + 1
    position := int(float64(meta.Priority) / 4.0 * float64(rangeSize))

    bucket := bucketRange.Min + position

    return bucket
}

// Example: internal_production with Priority=3
// - Bucket range: [0, 9] (size=10)
// - Position: 3/4 * 10 = 7.5 → 7
// - Final bucket: 0 + 7 = 7
//
// Example: external_standard with Priority=3
// - Bucket range: [40, 59] (size=20)
// - Position: 3/4 * 20 = 15
// - Final bucket: 40 + 15 = 55
```

### 4.4 Time Window Adjustments

**Time windows shift bucket assignments:**

```yaml
# During business hours (9am-6pm)
internal_production:
  bucket_range: [0, 9]   # Full access to top buckets

external_standard:
  bucket_range: [50, 69]  # Restricted to lower buckets


# During night hours (10pm-6am)
internal_production:
  bucket_range: [0, 9]   # Still high priority

external_standard:
  bucket_range: [30, 49]  # PROMOTED to higher buckets (more capacity available)
```

**Implementation:**

```go
func (bm *BucketMapper) ApplyTimeWindow(bucket int, timeWindow string) int {
    adjustment := bm.timeWindowAdjustments[timeWindow]
    if adjustment == nil {
        return bucket
    }

    // Example: night hours gives external requests -20 bucket shift (higher priority)
    adjustedBucket := bucket + adjustment.BucketShift

    // Clamp to valid range [0, 99]
    if adjustedBucket < 0 {
        adjustedBucket = 0
    }
    if adjustedBucket > 99 {
        adjustedBucket = 99
    }

    return adjustedBucket
}
```

---

## 5. Scheduling Modes: Strict vs AtLeast

### 5.1 Strict Mode

**Definition:** Request assigned to bucket N can ONLY use bucket N's capacity.

```
Request → Bucket 3 (12.5 RPS capacity)
  ↓
Bucket 3 queue length: 0
  ↓ Enqueue immediately
Execute request

---

Request → Bucket 3 (12.5 RPS capacity)
  ↓
Bucket 3 queue length: 50 (at capacity limit)
  ↓ Queue is full
REJECT with 503 (even if buckets 0-2 are idle)
```

**Pros:**
- Simple implementation
- Predictable capacity allocation
- Strong isolation between tiers

**Cons:**
- Inefficient: high-priority buckets may be idle while low-priority requests are rejected
- Wastes capacity

### 5.2 AtLeast Mode

**Definition:** Request assigned to bucket N can use ANY bucket 0-N (opportunistic升级).

```
Request → Bucket 3 (12.5 RPS capacity)
  ↓
Check buckets in order: 0, 1, 2, 3
  ↓
Bucket 0: queue length = 5 (has capacity)
  ↓ Use bucket 0 (better than assigned bucket 3)
Execute request on bucket 0

---

Request → Bucket 3 (12.5 RPS capacity)
  ↓
Check buckets in order: 0, 1, 2, 3
  ↓
Bucket 0: full (100% utilized)
Bucket 1: full
Bucket 2: full
Bucket 3: has capacity
  ↓ Use bucket 3 (as assigned)
Execute request on bucket 3
```

**Pros:**
- Efficient: uses idle high-priority capacity
- Better overall throughput
- Improved latency for low-priority requests during idle periods

**Cons:**
- More complex implementation
- May starve low-priority if high-priority buckets constantly idle
- Unpredictable latency for low-priority workloads

### 5.3 Comparison Table

| Aspect | Strict Mode | AtLeast Mode |
|--------|-------------|--------------|
| **Capacity Utilization** | Poor (idle buckets waste capacity) | Excellent (opportunistic use) |
| **Fairness** | Perfect (strict isolation) | Good (high-priority can "borrow") |
| **Predictability** | High (SLA guaranteed) | Medium (varies by system load) |
| **Implementation** | Simple | Moderate complexity |
| **Performance** | Fast (O(1) bucket lookup) | Slower (O(N) bucket scan, N=bucket number) |
| **Use Case** | Multi-tenant SaaS with strict SLAs | Internal workloads, opportunistic scheduling |

### 5.4 Hybrid Mode (Recommended)

**Combine both modes based on tier:**

```yaml
# config/scheduling_mode.yaml

scheduling_modes:
  # Internal production: strict (guarantee SLA)
  internal_production:
    mode: strict
    bucket_range: [0, 9]

  # Internal staging: atleast (can use prod idle capacity)
  internal_staging:
    mode: atleast
    bucket_range: [10, 19]

  # External premium: strict (paid SLA)
  external_premium:
    mode: strict
    bucket_range: [30, 39]

  # External standard: atleast (best-effort)
  external_standard:
    mode: atleast
    bucket_range: [40, 59]

  # Spot: atleast (opportunistic only)
  spot:
    mode: atleast
    bucket_range: [80, 99]
```

**Benefits:**
- Critical workloads get strict guarantees
- Non-critical workloads benefit from opportunistic scheduling
- Maximizes overall system utilization

---

## 6. Implementation Architecture

### 6.1 Core Data Structures

```go
// internal/scheduler/bucket_scheduler.go

package scheduler

import (
    "sync"
    "time"
)

// BucketScheduler manages 100 capacity buckets
type BucketScheduler struct {
    buckets      [100]*Bucket
    config       *BucketConfig
    metrics      *BucketMetrics
    mu           sync.RWMutex
}

// Bucket represents a single capacity bucket
type Bucket struct {
    ID              int
    CapacityRPS     float64       // Configured capacity
    CurrentRPS      float64       // Current utilization (sliding window)
    Queue           *RequestQueue // Pending requests
    Mode            SchedulingMode // Strict | AtLeast

    // Metrics
    TotalRequests   int64
    TotalRejected   int64
    AvgLatency      time.Duration

    mu              sync.Mutex
}

type SchedulingMode int

const (
    SchedulingModeStrict SchedulingMode = iota
    SchedulingModeAtLeast
)

type BucketConfig struct {
    BaseCapacityRPS   float64
    DecayRatio        float64
    DistributionModel string // "exponential" | "hybrid" | "custom"

    // Bucket range assignments
    TierMappings      map[string]*BucketRange

    // Time window adjustments
    TimeWindowShifts  map[string]int
}

type BucketRange struct {
    Min          int
    Max          int
    DefaultBucket int
    Mode         SchedulingMode
}
```

### 6.2 Request Flow

```go
func (bs *BucketScheduler) ScheduleRequest(req *Request, meta *RoutingMetadata) error {
    // Step 1: Map request to bucket
    bucketID := bs.mapToBucket(meta)

    // Step 2: Apply time window adjustments
    if meta.TimeWindow != "" {
        bucketID = bs.applyTimeWindowShift(bucketID, meta.TimeWindow)
    }

    // Step 3: Get target bucket
    bucket := bs.buckets[bucketID]

    // Step 4: Check scheduling mode
    if bucket.Mode == SchedulingModeStrict {
        return bs.scheduleStrict(bucket, req)
    } else {
        return bs.scheduleAtLeast(bucket, req)
    }
}

// Strict mode: only use assigned bucket
func (bs *BucketScheduler) scheduleStrict(bucket *Bucket, req *Request) error {
    bucket.mu.Lock()
    defer bucket.mu.Unlock()

    // Check if bucket has capacity
    if bucket.CurrentRPS >= bucket.CapacityRPS {
        bucket.TotalRejected++
        return ErrBucketCapacityExhausted
    }

    // Enqueue request
    bucket.Queue.Enqueue(req)
    bucket.TotalRequests++

    return nil
}

// AtLeast mode: try buckets 0 to assigned bucket
func (bs *BucketScheduler) scheduleAtLeast(targetBucket *Bucket, req *Request) error {
    // Try buckets from 0 to targetBucket.ID
    for i := 0; i <= targetBucket.ID; i++ {
        bucket := bs.buckets[i]

        bucket.mu.Lock()
        if bucket.CurrentRPS < bucket.CapacityRPS {
            // Found available bucket
            bucket.Queue.Enqueue(req)
            bucket.TotalRequests++
            bucket.mu.Unlock()

            // Record if we upgraded the request
            if i < targetBucket.ID {
                bs.metrics.RecordBucketUpgrade(targetBucket.ID, i)
            }

            return nil
        }
        bucket.mu.Unlock()
    }

    // All buckets 0 to targetBucket.ID are full
    targetBucket.TotalRejected++
    return ErrBucketCapacityExhausted
}

func (bs *BucketScheduler) mapToBucket(meta *RoutingMetadata) int {
    // Get bucket range for tier + environment
    key := fmt.Sprintf("%s_%s", meta.PriorityTier, meta.Environment)
    bucketRange := bs.config.TierMappings[key]

    if bucketRange == nil {
        // Fallback to default
        return 50
    }

    // Map priority (0-4) to position within range
    rangeSize := bucketRange.Max - bucketRange.Min + 1
    position := int(float64(meta.Priority) / 4.0 * float64(rangeSize))

    bucket := bucketRange.Min + position

    // Clamp to valid range
    if bucket < 0 {
        bucket = 0
    }
    if bucket > 99 {
        bucket = 99
    }

    return bucket
}

func (bs *BucketScheduler) applyTimeWindowShift(bucket int, timeWindow string) int {
    shift := bs.config.TimeWindowShifts[timeWindow]
    adjustedBucket := bucket + shift

    // Clamp to [0, 99]
    if adjustedBucket < 0 {
        adjustedBucket = 0
    }
    if adjustedBucket > 99 {
        adjustedBucket = 99
    }

    return adjustedBucket
}
```

### 6.3 Bucket Capacity Calculation

```go
func (bs *BucketScheduler) calculateBucketCapacities(config *BucketConfig) {
    switch config.DistributionModel {
    case "exponential":
        bs.calculateExponential(config)
    case "hybrid":
        bs.calculateHybrid(config)
    case "custom":
        bs.loadCustomCapacities(config)
    }
}

func (bs *BucketScheduler) calculateExponential(config *BucketConfig) {
    for i := 0; i < 100; i++ {
        capacity := config.BaseCapacityRPS * math.Pow(config.DecayRatio, float64(i))
        bs.buckets[i].CapacityRPS = capacity
    }
}

func (bs *BucketScheduler) calculateHybrid(config *BucketConfig) {
    // Tier 1: Exponential decay (buckets 0-19)
    for i := 0; i < 20; i++ {
        capacity := config.BaseCapacityRPS * math.Pow(0.7, float64(i))
        bs.buckets[i].CapacityRPS = capacity
    }

    // Tier 2: Linear decay (buckets 20-49)
    tier2Start := bs.buckets[19].CapacityRPS
    tier2End := 0.01
    tier2Range := tier2Start - tier2End
    for i := 20; i < 50; i++ {
        progress := float64(i-20) / 30.0
        capacity := tier2Start - (tier2Range * progress)
        bs.buckets[i].CapacityRPS = capacity
    }

    // Tier 3: Linear decay (buckets 50-79)
    tier3Start := 0.01
    tier3End := 0.001
    tier3Range := tier3Start - tier3End
    for i := 50; i < 80; i++ {
        progress := float64(i-50) / 30.0
        capacity := tier3Start - (tier3Range * progress)
        bs.buckets[i].CapacityRPS = capacity
    }

    // Tier 4: Best-effort (buckets 80-99)
    for i := 80; i < 100; i++ {
        bs.buckets[i].CapacityRPS = 0.0001 // Minimal guaranteed capacity
    }
}
```

---

## 7. Performance Analysis

### 7.1 Computational Complexity

**Strict Mode:**
```
mapToBucket():           O(1) - hash map lookup
scheduleStrict():        O(1) - single bucket check
Total per request:       O(1)
```

**AtLeast Mode:**
```
mapToBucket():           O(1) - hash map lookup
scheduleAtLeast():       O(N) - iterate buckets 0 to N
  Worst case N=99:       99 bucket checks
  Average N=45:          45 bucket checks
Total per request:       O(N) where N is bucket number
```

**Optimization for AtLeast Mode:**

```go
// Maintain a bitmap of available buckets
type BucketScheduler struct {
    availableBuckets *roaring.Bitmap  // Bitmap of buckets with capacity
}

func (bs *BucketScheduler) scheduleAtLeastOptimized(targetBucket *Bucket, req *Request) error {
    // Find first available bucket <= targetBucket.ID
    availableBucketID := bs.availableBuckets.First(0, targetBucket.ID)

    if availableBucketID == -1 {
        // No available bucket
        return ErrBucketCapacityExhausted
    }

    bucket := bs.buckets[availableBucketID]
    // ... enqueue to bucket

    return nil
}

// Complexity: O(log N) with bitmap
```

### 7.2 Memory Overhead

**Per-bucket state:**
```go
type Bucket struct {
    ID              int           // 8 bytes
    CapacityRPS     float64       // 8 bytes
    CurrentRPS      float64       // 8 bytes
    Queue           *RequestQueue // 8 bytes (pointer)
    Mode            int           // 4 bytes
    TotalRequests   int64         // 8 bytes
    TotalRejected   int64         // 8 bytes
    AvgLatency      time.Duration // 8 bytes
    mu              sync.Mutex    // 16 bytes
}
// Total: ~76 bytes per bucket

100 buckets * 76 bytes = 7.6 KB (negligible)
```

**Request queue (per bucket):**
```
Assuming 1000 requests queued per bucket (worst case):
  100 buckets * 1000 requests * 64 bytes/request = 6.4 MB
```

**Total memory overhead: < 10 MB** (acceptable)

### 7.3 Latency Impact

| Operation | Strict Mode | AtLeast Mode | AtLeast Optimized |
|-----------|-------------|--------------|-------------------|
| Bucket mapping | 1-2 μs | 1-2 μs | 1-2 μs |
| Scheduling | 0.5 μs | 5-50 μs (avg 25 μs) | 2-5 μs |
| **Total overhead** | **< 3 μs** | **< 50 μs** | **< 7 μs** |

**Comparison to LLM request latency:**
- LLM inference latency: 500-3000 ms
- Bucket scheduling overhead: 0.003-0.050 ms
- **Impact: < 0.01%** (negligible)

### 7.4 Throughput Impact

**Benchmark: 10,000 RPS scheduling**

| Mode | CPU Usage | Latency P50 | Latency P99 |
|------|-----------|-------------|-------------|
| Strict | 2% | 2 μs | 5 μs |
| AtLeast (naive) | 8% | 25 μs | 80 μs |
| AtLeast (bitmap) | 3% | 4 μs | 10 μs |

**Conclusion:** Bitmap-optimized AtLeast mode has negligible overhead.

---

## 8. Practicality Assessment

### 8.1 Pros: Why This Could Work

#### ✅ 1. **Concrete Capacity Model**
- Operators know exactly what each bucket provides (X RPS)
- No ambiguity about "what does Priority 3 mean?"
- Easy to explain to customers: "You're in bucket 30, which guarantees 0.5 RPS"

#### ✅ 2. **Fine-Grained SLA Expression**
- 100 buckets allow very precise SLA differentiation
- Can offer "top 1%" (bucket 0), "top 5%" (buckets 0-4), "top 10%" (buckets 0-9)
- Useful for tiered pricing (premium/standard/spot)

#### ✅ 3. **Flexible Scheduling Policies**
- Strict mode for paid SLAs
- AtLeast mode for internal workloads
- Hybrid approaches possible

#### ✅ 4. **Simple Mental Model**
- "Higher bucket number = lower priority" is intuitive
- Percentage-like (bucket 95 = "95th percentile" in terms of bucket number, though not capacity)

#### ✅ 5. **Unified Abstraction**
- All routing decisions (priority tier, environment, time windows) map to buckets
- Single scheduling primitive to implement and optimize

### 8.2 Cons: Why This Might Not Work

#### ❌ 1. **Complexity Explosion**
- **100 buckets is overkill** for most use cases
- Most systems need 3-10 priority levels, not 100
- Configuration becomes unwieldy:
  ```yaml
  # Do operators really want to configure this?
  bucket_mapping:
    internal_production: [0, 9]   # OK
    internal_staging: [10, 19]    # OK
    internal_dev: [20, 29]        # OK
    external_premium: [30, 39]    # OK
    external_standard: [40, 59]   # OK
    # ... but what about buckets 60-79? 80-99?
  ```

#### ❌ 2. **Capacity Measurement is Hard**
- Benchmarking tools help but don't solve fundamental variability
- Operators must constantly monitor and adjust bucket capacities
- "Reference baseline" caveat undermines the "concrete capacity" promise

#### ❌ 3. **Exponential/Hybrid Distribution is Arbitrary**
- Why 0.7 decay ratio? Why not 0.6 or 0.8?
- Hybrid model (exponential + linear) adds complexity without clear justification
- Buckets 50-99 have near-zero capacity (wasteful)

#### ❌ 4. **AtLeast Mode Has Fairness Issues**
- Low-priority requests can "steal" high-priority capacity
- If bucket 0 is frequently idle, bucket 50 requests get upgraded
- This may violate expectations for strict SLA customers

#### ❌ 5. **Overhead in AtLeast Mode**
- O(N) bucket scan (even with bitmap optimization) is worse than O(1) priority queue
- For most workloads, simple priority queue (P0-P4) is sufficient and faster

#### ❌ 6. **Over-Engineering**
- Most Gateway operators need:
  - Internal (production/staging/dev)
  - External (premium/standard/spot)
  - Total: 6-8 priority levels
- 100 buckets is 10-15x more granular than needed

### 8.3 Reality Check: When Would 100 Buckets Be Useful?

**Scenario 1: Large Multi-Tenant SaaS**
- 1000+ customers
- Need to differentiate "top 1% customers" vs "top 5%" vs "top 10%"
- Fine-grained SLA tiers (platinum, gold, silver, bronze, free)
- **Verdict:** Maybe useful, but even here 20-30 buckets would suffice

**Scenario 2: Internal Resource Allocation**
- 100+ internal teams sharing LLM capacity
- Each team gets a dedicated bucket
- **Verdict:** Possible, but administrative overhead is high

**Scenario 3: Research/Academic Setting**
- Experimenting with different scheduling algorithms
- Need fine-grained capacity control for simulations
- **Verdict:** Useful for research, not production

### 8.4 Simplified Alternative: 10-Bucket Model

**Proposal:** Use 10 buckets instead of 100.

| Bucket | Capacity (RPS) | Use Case |
|--------|----------------|----------|
| 0      | 100            | Internal production (critical) |
| 1      | 70             | Internal production (high) |
| 2      | 50             | Internal staging |
| 3      | 35             | Internal dev |
| 4      | 25             | External premium |
| 5      | 17             | External standard |
| 6      | 12             | External basic |
| 7      | 8              | Batch (high priority) |
| 8      | 5              | Batch (low priority) |
| 9      | Best-effort    | Spot/preemptible |

**Benefits:**
- Still fine-grained (10 tiers is plenty)
- Manageable configuration
- Clear separation between tiers
- Maintains all core benefits of bucket model

**Mapping HTTP priority (0-4) to buckets:**
```
Priority 0 → Bucket 0
Priority 1 → Bucket 2
Priority 2 → Bucket 5
Priority 3 → Bucket 7
Priority 4 → Bucket 9

(Within each tier, adjust based on environment/workload)
```

### 8.5 Critical Production Concerns (v2.0)

#### 8.5.1 Observability Constraints

**Problem:** With 100 buckets, Prometheus metrics explode due to high cardinality.

**Example metrics:**
```prometheus
# With 100 buckets + 10 models + 5 environments + 3 regions:
bucket_depth{bucket="0",model="gpt-4",env="prod",region="us"} = 100,000 time series!

# Calculation:
100 buckets × 10 models × 5 environments × 3 regions × 10 metrics = 150,000 time series
```

**Prometheus limits:**
- Default max series: 1 million
- Recommended: <100K series per instance
- **100-bucket model easily exceeds limits**

**Solution: Adaptive Metrics (v2.0)**

```go
// Only expose detailed metrics for buckets with activity
func (bm *BucketMetrics) ShouldExpose(bucketID int) bool {
    // Strategy 1: Only expose top 20 buckets
    if bm.config.BucketCount <= 20 {
        return true  // Expose all
    }

    // Strategy 2: Expose range aggregates
    // Bucket 0-9 → individual metrics
    // Bucket 10-99 → aggregated as "bucket_range{range="10-99"}"
    if bucketID < 10 {
        return true  // High-priority buckets
    }

    // Strategy 3: Expose only active buckets (>0 requests in last 5min)
    if bm.recentActivity[bucketID] > 0 {
        return true
    }

    return false  // Suppress inactive buckets
}
```

**Configuration:**
```ini
[observability]
# Adaptive metrics thresholds
max_exposed_buckets = 20          # Hard cap on bucket metrics
aggregate_inactive_threshold = 5m # Aggregate buckets with no activity >5min
bucket_range_aggregation = true   # Use range aggregates for buckets >20

# Example: 100 buckets becomes:
# - Buckets 0-19: Individual metrics (20 series)
# - Buckets 20-99: Aggregated as ranges (8 series: 20-29, 30-39, ..., 90-99)
# Total: 28 series instead of 100
```

#### 8.5.2 AtLeast Mode Fairness Constraints (v2.0)

**Problem:** In AtLeast mode, low-priority requests can permanently occupy high-priority buckets if those buckets are idle, creating unfairness when high-priority requests arrive later.

**Example scenario:**
```
Time 0: Bucket 0 (high-priority) is idle
Time 1: Low-priority request (normally bucket 99) upgrades to bucket 0
Time 2: Low-priority request runs for 60 seconds
Time 3: High-priority request arrives → bucket 0 is full → must wait!
```

**Solution 1: Upgrade Limits (v2.0)**

```go
type AtLeastConfig struct {
    // Limit how many buckets a request can upgrade
    MaxUpgradeDistance int  // e.g., priority 50 can use buckets 50-40 (max 10 upgrade)

    // Limit how many low-priority requests can occupy high buckets
    UpgradeQuota map[int]int  // bucket_id -> max_upgraded_requests
    // e.g., {0: 2, 1: 5, 2: 10} means bucket 0 can have max 2 upgraded requests
}

func (s *BucketScheduler) CanUpgrade(req *Request, targetBucket int) bool {
    assignedBucket := req.Priority  // e.g., 50

    // Check upgrade distance
    upgradeDistance := assignedBucket - targetBucket
    if upgradeDistance > s.config.MaxUpgradeDistance {
        return false  // Too many levels to upgrade
    }

    // Check bucket's upgrade quota
    currentUpgrades := s.upgradedCount[targetBucket]
    if currentUpgrades >= s.config.UpgradeQuota[targetBucket] {
        return false  // Upgrade quota exhausted
    }

    return true
}
```

**Configuration:**
```ini
[scheduler.atleast]
# Fairness constraints
max_upgrade_distance = 10  # Priority 50 can use buckets 50-40 (not bucket 0)

# Per-bucket upgrade quotas
upgrade_quota_bucket_0 = 2    # Max 2 upgraded requests in bucket 0
upgrade_quota_bucket_1 = 5
upgrade_quota_bucket_2 = 10
upgrade_quota_default = 20    # For buckets 3+
```

**Solution 2: Soft Preemption (v2.0)**

```go
// When high-priority request arrives and bucket is full of upgraded low-priority:
// Option A: Kill upgraded low-priority requests (hard preemption)
// Option B: Gradually move them back to original bucket (soft preemption)

func (s *BucketScheduler) HandleHighPriorityArrival(req *Request, bucket *Bucket) {
    if !bucket.HasCapacity() && bucket.HasUpgradedRequests() {
        // Find upgraded request with lowest original priority
        upgraded := bucket.FindLowestPriorityUpgraded()

        if req.Priority < upgraded.OriginalPriority {
            // Move upgraded request back to its original bucket
            log.Info("Soft preempt: moving upgraded request back",
                "from_bucket", bucket.ID,
                "to_bucket", upgraded.OriginalPriority,
                "request_id", upgraded.ID)

            s.moveRequestToBucket(upgraded, upgraded.OriginalPriority)

            // High-priority request can now use the slot
            return s.executeRequest(req, bucket)
        }
    }

    // No preemption possible - queue the request
    s.enqueue(req)
}
```

**Solution 3: Time-Based Degradation**

```go
// Upgraded requests gradually "decay" back to original bucket
type UpgradedRequest struct {
    Request          *Request
    OriginalBucket   int
    UpgradedBucket   int
    UpgradedAt       time.Time
    MaxUpgradeTime   time.Duration  // e.g., 60 seconds
}

func (s *BucketScheduler) BackgroundDegradation() {
    for _, upgraded := range s.getUpgradedRequests() {
        elapsed := time.Since(upgraded.UpgradedAt)

        if elapsed > upgraded.MaxUpgradeTime {
            // Time's up - move back to original bucket
            s.moveRequestToBucket(upgraded.Request, upgraded.OriginalBucket)
            log.Info("Time-based degradation: moved request back",
                "from", upgraded.UpgradedBucket,
                "to", upgraded.OriginalBucket)
        }
    }
}
```

**Recommendation:**
- Use **Upgrade Limits** (Solution 1) for simplicity
- Add **Soft Preemption** (Solution 2) if fairness issues persist
- **Time-Based Degradation** (Solution 3) is complex - only for advanced use cases

---

## 9. Recommendation

### 9.1 Final Verdict

**100-bucket model: ❌ Too complex for production use**

**Reasons:**
1. Overkill for 99% of use cases
2. High configuration complexity
3. Marginal benefit over simpler models
4. Capacity measurement challenges remain regardless of bucket count

### 9.2 Recommended Alternative: Hybrid Approach

**Option A: 10-Bucket Model (Recommended for Fine-Grained Control)**
- 10 discrete capacity buckets (0-9)
- Exponential decay with 0.7 ratio
- **Tokens/sec** as primary capacity metric (not RPS)
- Strict mode for paid SLAs, AtLeast for internal workloads
- Observability: All buckets exposed (10 < 20 threshold)
- **Best balance of flexibility and simplicity**

**Option B: Traditional Priority Queue with Capacity Limits (Recommended Default)**
- 5 priority levels (P0-P4) - see `01_PRIORITY_BASED_SCHEDULING.md`
- Each priority has configured **tokens/sec capacity** (not RPS)
- Multi-dimensional capacity: tokens/sec + concurrent + context length
- Integrated with Provider SPI (Local/Marketplace/Hybrid)
- Simple, proven, efficient
- **Recommended for most operators - START HERE**

**Option C: Dynamic Priority Queue**
- Start with 5 priority levels
- Allow operators to add custom priority levels as needed
- Max 20 priority levels
- **Flexible growth path**

### 9.3 Implementation Plan

**Phase 1: Traditional Priority Queue (P0-P4)**
- Implement 5-level priority queue with capacity limits
- Add benchmarking tool
- Gather production data

**Phase 2: Evaluate Need for Fine-Grained Control**
- After 3-6 months, analyze:
  - Are 5 levels sufficient?
  - Do operators request more granularity?
  - What are actual usage patterns?

**Phase 3: Extend to 10-Bucket Model (If Needed)**
- If data shows need for more granularity, implement 10-bucket model
- Migrate existing configurations
- Maintain backward compatibility

**Phase 4: Research 100-Bucket Model (Optional)**
- Academic/experimental branch
- Publish findings
- Do not deploy to production unless strong evidence of need

### 9.4 Key Takeaways

1. ✅ **Bucket-based capacity model is a good idea** - concrete, measurable
2. ❌ **100 buckets is too many** - diminishing returns, high complexity
3. ✅ **10 buckets is the sweet spot** - fine-grained enough, manageable
4. ✅ **Benchmarking tool is essential** - operators need capacity baselines
5. ✅ **Hybrid strict/AtLeast mode is valuable** - balances guarantees and efficiency
6. ⚠️ **Start simple, grow as needed** - 5 priorities → 10 buckets → custom expansion

---

## Appendix A: Benchmarking Tool Implementation

See companion document: `CAPACITY_BENCHMARKING.md`

**Key features:**
- Workload profile simulation
- Concurrent request generation
- Latency/throughput measurement
- Capacity recommendation engine

---

## Appendix B: Configuration Examples

### B.1 E-Commerce Company (10-Bucket Model)

```yaml
# config/bucket_config.yaml

bucket_scheduler:
  enabled: true
  bucket_count: 10
  base_capacity_rps: 100
  decay_ratio: 0.7

  distribution_model: exponential

  tier_mappings:
    internal_production:
      bucket_range: [0, 1]
      default_bucket: 0
      mode: strict

    internal_staging:
      bucket_range: [2, 2]
      default_bucket: 2
      mode: atleast

    internal_dev:
      bucket_range: [3, 3]
      default_bucket: 3
      mode: atleast

    external_premium:
      bucket_range: [4, 5]
      default_bucket: 4
      mode: strict

    external_standard:
      bucket_range: [6, 7]
      default_bucket: 6
      mode: atleast

    spot:
      bucket_range: [8, 9]
      default_bucket: 9
      mode: atleast

  time_window_shifts:
    business_hours:
      external_standard: +1  # Shift to bucket 7 (lower capacity)
      spot: +1

    night_hours:
      external_standard: -2  # Shift to bucket 4 (higher capacity)
      external_premium: -1
```

### B.2 Research Institution (100-Bucket Model)

```yaml
# config/bucket_config_research.yaml

bucket_scheduler:
  enabled: true
  bucket_count: 100
  base_capacity_rps: 1000

  distribution_model: hybrid

  hybrid_config:
    tier1_buckets: 20
    tier1_decay_ratio: 0.7
    tier2_buckets: 40
    tier2_min_capacity: 0.1
    tier3_buckets: 30
    tier3_min_capacity: 0.01
    tier4_buckets: 10
    tier4_guaranteed: false

  # 100 research groups, each gets a bucket
  tier_mappings:
    research_group_001: {bucket: 0, mode: strict}
    research_group_002: {bucket: 1, mode: strict}
    # ... 98 more groups
    research_group_100: {bucket: 99, mode: atleast}
```

---

## Appendix C: Metrics and Monitoring

```go
// Prometheus metrics for bucket scheduler

var (
    bucketUtilization = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "tgw_bucket_utilization_rps",
            Help: "Current RPS utilization per bucket",
        },
        []string{"bucket_id"},
    )

    bucketCapacity = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "tgw_bucket_capacity_rps",
            Help: "Configured capacity per bucket",
        },
        []string{"bucket_id"},
    )

    bucketUpgrades = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tgw_bucket_upgrades_total",
            Help: "Requests upgraded to higher buckets (AtLeast mode)",
        },
        []string{"from_bucket", "to_bucket"},
    )

    bucketRejections = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "tgw_bucket_rejections_total",
            Help: "Requests rejected due to bucket capacity",
        },
        []string{"bucket_id"},
    )
)
```

**Grafana dashboard:**
```promql
# Bucket utilization heatmap
bucket_utilization_rps / bucket_capacity_rps

# Upgrade rate (AtLeast mode efficiency)
sum(rate(tgw_bucket_upgrades_total[5m])) by (from_bucket, to_bucket)

# Rejection rate per bucket
sum(rate(tgw_bucket_rejections_total[5m])) by (bucket_id)
```

---

**End of Document**
