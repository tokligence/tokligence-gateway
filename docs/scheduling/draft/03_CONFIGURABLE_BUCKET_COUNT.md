# Configurable Bucket Count Design

**Version:** 2.0 (With Observability Constraints & Fairness Limits)
**Date:** 2025-02-01
**Status:** Design Analysis - Updated with v2.0 Production Concerns
**Related Documents:**
- `00_REVISED_OVERVIEW.md` - Overall scheduling architecture (recommends 5-10 buckets)
- `02_BUCKET_BASED_SCHEDULING.md` - Core bucket scheduling design (see §8.5 for observability & fairness)
- `01_PRIORITY_BASED_SCHEDULING.md` - Priority queuing (recommended default)
- `06_MARKETPLACE_INTEGRATION.md` - Provider SPI and capacity model

---

## Executive Summary

This document analyzes making **bucket count configurable** instead of hardcoding to 10 or 100. The proposal:

```ini
# config/gateway.ini
[bucket_scheduler]
bucket_count = 10  # Default: 10, Range: 2-100
```

**Verdict: ✅ Highly Recommended (With Constraints)**

**v2.0 Updates:**
- ✅ Configurable bucket count (2-100 range) - still recommended
- ⚠️ **NEW: Observability constraints** - Adaptive metrics for >20 buckets (see §2.3)
- ⚠️ **NEW: AtLeast fairness limits** - Upgrade quotas to prevent starvation (see §2.4)
- ⚠️ **Default changed**: 10 buckets (was considering 100)
- ⚠️ **Hard recommendation**: Do NOT use >50 buckets without strong justification

**Key Benefits:**
- Flexibility for different use cases (small startups vs large enterprises)
- Future-proof (can grow as needed)
- Simple to implement with proper abstraction
- Observability scales with adaptive metrics

**Costs (v2.0 - UPDATED):**
- Minimal implementation complexity
- Slight memory overhead (linear with bucket count)
- Configuration validation needed
- **NEW: Observability overhead** - Prometheus cardinality scales with bucket count
- **NEW: AtLeast complexity** - Need fairness constraints for >10 buckets

---

## Table of Contents

1. [Feasibility Analysis](#1-feasibility-analysis)
2. [Cost Analysis](#2-cost-analysis)
   - 2.3 [Observability Constraints (v2.0)](#23-observability-constraints-v20)
   - 2.4 [AtLeast Fairness Constraints (v2.0)](#24-atleast-fairness-constraints-v20)
3. [Implementation Design](#3-implementation-design)
4. [Migration & Backward Compatibility](#4-migration--backward-compatibility)
5. [Validation & Constraints](#5-validation--constraints)
6. [Edge Cases & Failure Modes](#6-edge-cases--failure-modes)
7. [Best Practices & Recommendations](#7-best-practices--recommendations)
8. [Final Recommendation](#8-final-recommendation)

---

## 1. Feasibility Analysis

### 1.1 Technical Feasibility: ✅ Fully Feasible

**Core principle:** Bucket scheduler should be bucket-count-agnostic.

```go
// BEFORE: Hardcoded 100 buckets
type BucketScheduler struct {
    buckets [100]*Bucket  // ❌ Hardcoded array
}

// AFTER: Dynamic bucket count
type BucketScheduler struct {
    buckets []*Bucket     // ✅ Slice (dynamic)
    config  *BucketConfig
}

type BucketConfig struct {
    BucketCount int  // Configurable: 2-100
    // ... other config
}
```

**No fundamental technical barriers.**

### 1.2 Algorithm Compatibility

**All core algorithms work regardless of bucket count:**

| Algorithm | Bucket Count Dependency | Compatibility |
|-----------|-------------------------|---------------|
| Exponential decay | None (formula adapts) | ✅ Fully compatible |
| Hybrid distribution | Needs tier boundaries adjustment | ✅ Compatible with dynamic tiers |
| Strict scheduling | None (O(1) lookup) | ✅ Fully compatible |
| AtLeast scheduling | O(N) scan where N=bucket count | ✅ Compatible (performance scales linearly) |
| Bucket mapping | Needs range adjustment | ✅ Compatible with dynamic ranges |

### 1.3 Use Case Coverage

**Different bucket counts serve different needs:**

| Use Case | Recommended Bucket Count | Rationale |
|----------|--------------------------|-----------|
| **Small startup** | 3-5 buckets | Internal (prod/staging/dev) + external + spot |
| **Medium company** | 10 buckets (default) | Good balance, covers 95% of needs |
| **Large enterprise** | 20-30 buckets | Many internal teams + external tiers |
| **Multi-tenant SaaS** | 50-100 buckets | Fine-grained customer SLAs |
| **Research/academic** | 100+ buckets | Experimental, fine control |

**Verdict:** Configurable bucket count enables all use cases.

---

## 2. Cost Analysis

### 2.1 Memory Overhead

**Per-bucket memory footprint:**

```go
type Bucket struct {
    ID              int           // 8 bytes
    CapacityRPS     float64       // 8 bytes
    CurrentRPS      float64       // 8 bytes
    Queue           *RequestQueue // 8 bytes (pointer)
    Mode            SchedulingMode // 4 bytes
    TotalRequests   int64         // 8 bytes
    TotalRejected   int64         // 8 bytes
    AvgLatency      time.Duration // 8 bytes
    mu              sync.Mutex    // 16 bytes
}
// Total: ~76 bytes per bucket
```

**Memory cost by bucket count:**

| Bucket Count | Bucket Metadata | Queue Memory (1K requests/bucket) | Total |
|--------------|-----------------|-----------------------------------|-------|
| 5            | 380 bytes       | 320 KB                            | ~320 KB |
| 10 (default) | 760 bytes       | 640 KB                            | ~640 KB |
| 20           | 1.5 KB          | 1.28 MB                           | ~1.3 MB |
| 50           | 3.8 KB          | 3.2 MB                            | ~3.2 MB |
| 100          | 7.6 KB          | 6.4 MB                            | ~6.4 MB |

**Conclusion:** Even at 100 buckets, memory overhead is < 10 MB (negligible).

### 2.2 CPU Overhead

**Strict mode (O(1)):**
- Bucket count has NO impact
- Always single bucket lookup

**AtLeast mode (O(N)):**

| Bucket Count | Avg Bucket Scan | CPU Cost (10K RPS) | Impact |
|--------------|-----------------|---------------------|--------|
| 5            | 2.5 buckets     | 0.5% CPU            | Negligible |
| 10           | 5 buckets       | 1.0% CPU            | Negligible |
| 20           | 10 buckets      | 2.0% CPU            | Negligible |
| 50           | 25 buckets      | 5.0% CPU            | Minor |
| 100          | 50 buckets      | 10.0% CPU           | Noticeable |

**Mitigation:** Use bitmap optimization for AtLeast mode.

```go
// With bitmap (Roaring Bitmap)
type BucketScheduler struct {
    availableBuckets *roaring.Bitmap  // O(log N) lookup
}

// CPU cost with bitmap:
// 100 buckets @ 10K RPS = 2-3% CPU (acceptable)
```

**Conclusion:** CPU overhead scales linearly but remains acceptable with bitmap optimization.

### 2.3 Configuration Complexity

**Configuration file size:**

```yaml
# 5 buckets - Simple
tier_mappings:
  internal_production:  {bucket_range: [0, 0]}
  internal_staging:     {bucket_range: [1, 1]}
  external_premium:     {bucket_range: [2, 2]}
  external_standard:    {bucket_range: [3, 3]}
  spot:                 {bucket_range: [4, 4]}

# 10 buckets - Manageable
tier_mappings:
  internal_production:  {bucket_range: [0, 1]}
  internal_staging:     {bucket_range: [2, 3]}
  internal_dev:         {bucket_range: [4, 4]}
  external_premium:     {bucket_range: [5, 6]}
  external_standard:    {bucket_range: [7, 8]}
  spot:                 {bucket_range: [9, 9]}

# 100 buckets - Complex
tier_mappings:
  internal_production:  {bucket_range: [0, 9]}
  internal_staging:     {bucket_range: [10, 19]}
  # ... many more lines
```

**Cost:** Higher bucket counts increase config verbosity (but manageable with ranges).

### 2.4 Monitoring & Observability

**Metrics cardinality:**

```go
// Prometheus metric with bucket_id label
bucketUtilization = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "tgw_bucket_utilization_rps",
    },
    []string{"bucket_id"},  // Cardinality = bucket_count
)
```

**Cardinality impact:**

| Bucket Count | Metric Series (per metric) | Total (10 metrics) | Prometheus Impact |
|--------------|---------------------------|---------------------|-------------------|
| 5            | 5                         | 50                  | Negligible |
| 10           | 10                        | 100                 | Negligible |
| 50           | 50                        | 500                 | Minor |
| 100          | 100                       | 1,000               | Moderate (acceptable) |

**Conclusion:** Prometheus can handle 1K series easily. Only becomes problematic at 1000+ buckets.

### 2.5 Total Cost Summary

| Cost Category | 5 Buckets | 10 Buckets | 50 Buckets | 100 Buckets |
|---------------|-----------|------------|------------|-------------|
| **Memory**    | ~320 KB   | ~640 KB    | ~3.2 MB    | ~6.4 MB     |
| **CPU (strict)** | 0.5%   | 0.5%       | 0.5%       | 0.5%        |
| **CPU (atleast)** | 0.5%  | 1.0%       | 2.0%       | 3.0% (with bitmap) |
| **Config complexity** | Low | Low       | Medium     | High        |
| **Metrics cardinality** | 50 | 100      | 500        | 1,000       |

**Overall Cost: ✅ Acceptable across all reasonable bucket counts (5-100)**

### 2.3 Observability Constraints (v2.0)

**NEW CONCERN:** Prometheus metric cardinality explodes with high bucket counts.

See `02_BUCKET_BASED_SCHEDULING.md` §8.5.1 for full details.

**Problem:**
```prometheus
# With 100 buckets × 10 models × 5 environments × 3 regions:
bucket_depth{bucket="0",model="gpt-4",env="prod",region="us"}

# = 100 × 10 × 5 × 3 × 10 metrics = 150,000 time series!
```

**Solution: Adaptive Metrics**

```ini
[observability]
# Auto-adapt based on bucket count
max_exposed_buckets = 20          # Hard cap
aggregate_inactive_threshold = 5m # Aggregate inactive buckets
bucket_range_aggregation = true   # Use ranges for buckets >20

# Behavior:
# - bucket_count <= 20: Expose all buckets individually
# - bucket_count > 20:  Expose top 20 + aggregated ranges for rest
```

**Implementation:**
```go
func (bm *BucketMetrics) ShouldExpose(bucketID int) bool {
    if bm.config.BucketCount <= 20 {
        return true  // Small count - expose all
    }

    // Large count - selective exposure
    if bucketID < 10 {
        return true  // Always expose top 10
    }

    if bm.recentActivity[bucketID] > 0 {
        return true  // Expose active buckets
    }

    return false  // Suppress inactive
}
```

**Observability Cost by Bucket Count:**

| Bucket Count | Exposed Metrics | Prometheus Series | Verdict |
|--------------|-----------------|-------------------|---------|
| 5            | All 5 buckets   | ~500              | ✅ Excellent |
| 10           | All 10 buckets  | ~1,000            | ✅ Great |
| 20           | All 20 buckets  | ~2,000            | ✅ Good |
| 50           | Top 20 + ranges | ~2,500            | ⚠️ Acceptable with adaptive |
| 100          | Top 20 + ranges | ~3,000            | ⚠️ Acceptable with adaptive |

**Recommendation:**
- ✅ 10-20 buckets: No constraints needed
- ⚠️ 20-50 buckets: Enable adaptive metrics
- ❌ >50 buckets: Not recommended (complexity outweighs benefits)

### 2.4 AtLeast Fairness Constraints (v2.0)

**NEW CONCERN:** AtLeast mode allows low-priority to occupy high-priority buckets, creating unfairness.

See `02_BUCKET_BASED_SCHEDULING.md` §8.5.2 for full details.

**Problem:**
```
Time 0: Bucket 0 (high-priority) is idle
Time 1: Low-priority request (bucket 99) upgrades to bucket 0
Time 2: Runs for 60 seconds
Time 3: High-priority arrives → must wait! (unfair)
```

**Solution: Upgrade Limits**

```ini
[scheduler.atleast]
# Limit how far requests can upgrade
max_upgrade_distance = 10  # Priority 50 can use buckets 50-40 (not bucket 0)

# Per-bucket upgrade quotas
upgrade_quota_bucket_0 = 2    # Max 2 upgraded requests
upgrade_quota_bucket_1 = 5
upgrade_quota_default = 20    # For lower-priority buckets
```

**Implementation:**
```go
func (s *BucketScheduler) CanUpgrade(req *Request, targetBucket int) bool {
    upgradeDistance := req.AssignedBucket - targetBucket

    // Check distance limit
    if upgradeDistance > s.config.MaxUpgradeDistance {
        return false
    }

    // Check quota
    if s.upgradedCount[targetBucket] >= s.config.UpgradeQuota[targetBucket] {
        return false
    }

    return true
}
```

**Fairness Cost by Bucket Count:**

| Bucket Count | Upgrade Complexity | Fairness Risk | Mitigation Required |
|--------------|-------------------|---------------|---------------------|
| 5            | Low (5 levels)    | Low           | ✅ None |
| 10           | Medium (10 levels)| Medium        | ✅ Upgrade limits recommended |
| 20+          | High (many levels)| High          | ⚠️ Upgrade limits + soft preemption |

**Recommendation:**
- ✅ 5 buckets: No fairness constraints needed
- ⚠️ 10 buckets: Add upgrade limits
- ⚠️ 20+ buckets: Add upgrade limits + soft preemption + time-based degradation

---

## 3. Implementation Design

### 3.1 Configuration Schema

```ini
# config/gateway.ini

[bucket_scheduler]
# Enable bucket-based scheduling
enabled = true

# Number of buckets (2-100)
# Recommended:
#   - Small deployments: 5-10
#   - Medium deployments: 10-20
#   - Large enterprises: 20-50
#   - Multi-tenant SaaS: 50-100
bucket_count = 10

# Capacity distribution model
distribution_model = exponential  # exponential | hybrid | linear | custom

# Exponential decay ratio (applied when distribution_model = exponential)
# Each bucket has capacity = previous_bucket * decay_ratio
# Recommended: 0.6-0.8 (0.7 is good default)
decay_ratio = 0.7

# Base capacity (bucket 0) in RPS
# Get this from benchmarking tool: tokligence benchmark
base_capacity_rps = 100
```

**Environment variable overrides:**

```bash
export TOKLIGENCE_BUCKET_COUNT=20
export TOKLIGENCE_BUCKET_DECAY_RATIO=0.75
export TOKLIGENCE_BUCKET_BASE_CAPACITY_RPS=150
```

### 3.2 Dynamic Bucket Initialization

```go
// internal/scheduler/bucket_scheduler.go

package scheduler

import (
    "fmt"
    "math"
)

type BucketScheduler struct {
    buckets []*Bucket  // Dynamic slice
    config  *BucketConfig
    metrics *BucketMetrics
}

type BucketConfig struct {
    BucketCount       int     // Configurable: 2-100
    BaseCapacityRPS   float64
    DecayRatio        float64
    DistributionModel string
}

func NewBucketScheduler(config *BucketConfig) (*BucketScheduler, error) {
    // Validate bucket count
    if config.BucketCount < 2 || config.BucketCount > 100 {
        return nil, fmt.Errorf("bucket_count must be 2-100, got %d", config.BucketCount)
    }

    // Validate decay ratio
    if config.DecayRatio <= 0 || config.DecayRatio >= 1 {
        return nil, fmt.Errorf("decay_ratio must be 0 < ratio < 1, got %f", config.DecayRatio)
    }

    bs := &BucketScheduler{
        buckets: make([]*Bucket, config.BucketCount),
        config:  config,
        metrics: NewBucketMetrics(config.BucketCount),
    }

    // Initialize buckets
    if err := bs.initializeBuckets(); err != nil {
        return nil, err
    }

    return bs, nil
}

func (bs *BucketScheduler) initializeBuckets() error {
    switch bs.config.DistributionModel {
    case "exponential":
        return bs.initExponential()
    case "hybrid":
        return bs.initHybrid()
    case "linear":
        return bs.initLinear()
    case "custom":
        return bs.loadCustomDistribution()
    default:
        return fmt.Errorf("unknown distribution model: %s", bs.config.DistributionModel)
    }
}

func (bs *BucketScheduler) initExponential() error {
    for i := 0; i < bs.config.BucketCount; i++ {
        capacity := bs.config.BaseCapacityRPS * math.Pow(bs.config.DecayRatio, float64(i))

        bs.buckets[i] = &Bucket{
            ID:          i,
            CapacityRPS: capacity,
            Queue:       NewRequestQueue(),
            Mode:        SchedulingModeStrict, // Default
        }
    }

    log.Info("Initialized bucket scheduler",
        "bucket_count", bs.config.BucketCount,
        "distribution", "exponential",
        "decay_ratio", bs.config.DecayRatio,
        "total_capacity", bs.getTotalCapacity())

    return nil
}

func (bs *BucketScheduler) getTotalCapacity() float64 {
    total := 0.0
    for _, bucket := range bs.buckets {
        total += bucket.CapacityRPS
    }
    return total
}
```

### 3.3 Dynamic Bucket Range Mapping

**Challenge:** Tier mappings must adapt to bucket count.

**Example:**
```
With 10 buckets:
  internal_production: [0, 1]   (20% of buckets)
  external_standard:   [7, 8]   (20% of buckets)

With 100 buckets:
  internal_production: [0, 19]  (20% of buckets)
  external_standard:   [70, 89] (20% of buckets)
```

**Solution: Percentage-based ranges**

```yaml
# config/tier_mappings.yaml

tier_mappings:
  internal_production:
    # Use first 20% of buckets
    bucket_range_pct: [0, 0.20]
    default_bucket_pct: 0.0  # First bucket
    mode: strict

  internal_staging:
    bucket_range_pct: [0.20, 0.40]
    default_bucket_pct: 0.25
    mode: atleast

  external_premium:
    bucket_range_pct: [0.40, 0.60]
    default_bucket_pct: 0.45
    mode: strict

  external_standard:
    bucket_range_pct: [0.60, 0.80]
    default_bucket_pct: 0.70
    mode: atleast

  spot:
    bucket_range_pct: [0.80, 1.0]
    default_bucket_pct: 0.90
    mode: atleast
```

**Implementation:**

```go
type TierMapping struct {
    BucketRangePct   [2]float64  // [start%, end%]
    DefaultBucketPct float64
    Mode             SchedulingMode
}

func (bs *BucketScheduler) loadTierMappings(config map[string]*TierMapping) error {
    for tierName, mapping := range config {
        // Convert percentages to absolute bucket numbers
        startBucket := int(mapping.BucketRangePct[0] * float64(bs.config.BucketCount))
        endBucket := int(mapping.BucketRangePct[1] * float64(bs.config.BucketCount))
        defaultBucket := int(mapping.DefaultBucketPct * float64(bs.config.BucketCount))

        // Clamp to valid range
        if endBucket >= bs.config.BucketCount {
            endBucket = bs.config.BucketCount - 1
        }

        bs.tierMappings[tierName] = &BucketRange{
            Min:           startBucket,
            Max:           endBucket,
            DefaultBucket: defaultBucket,
            Mode:          mapping.Mode,
        }

        log.Debug("Loaded tier mapping",
            "tier", tierName,
            "bucket_range", fmt.Sprintf("[%d, %d]", startBucket, endBucket),
            "default_bucket", defaultBucket)
    }

    return nil
}

// Example with bucket_count=10:
//   internal_production: [0, 0.20] → buckets [0, 2)   → [0, 1]
//   external_standard:   [0.60, 0.80] → buckets [6, 8) → [6, 7]

// Example with bucket_count=100:
//   internal_production: [0, 0.20] → buckets [0, 20)  → [0, 19]
//   external_standard:   [0.60, 0.80] → buckets [60, 80) → [60, 79]
```

**Benefits:**
- ✅ Configuration is bucket-count-agnostic
- ✅ Same config works for 10 or 100 buckets
- ✅ Scales automatically

### 3.4 Validation on Startup

```go
func (bs *BucketScheduler) Validate() error {
    // 1. Check bucket count range
    if bs.config.BucketCount < 2 {
        return fmt.Errorf("bucket_count too low: %d (minimum: 2)", bs.config.BucketCount)
    }
    if bs.config.BucketCount > 100 {
        log.Warn("bucket_count is high, consider performance impact",
            "bucket_count", bs.config.BucketCount,
            "recommendation", "use 10-50 for most use cases")
    }

    // 2. Check total capacity allocation
    totalCapacity := bs.getTotalCapacity()
    if totalCapacity < 1.0 {
        return fmt.Errorf("total capacity too low: %.2f RPS", totalCapacity)
    }

    // 3. Check tier mappings cover all buckets
    coveredBuckets := make([]bool, bs.config.BucketCount)
    for tierName, bucketRange := range bs.tierMappings {
        if bucketRange.Max >= bs.config.BucketCount {
            return fmt.Errorf("tier %s: bucket range [%d, %d] exceeds bucket_count %d",
                tierName, bucketRange.Min, bucketRange.Max, bs.config.BucketCount)
        }

        for i := bucketRange.Min; i <= bucketRange.Max; i++ {
            coveredBuckets[i] = true
        }
    }

    // Warn about uncovered buckets
    uncovered := []int{}
    for i, covered := range coveredBuckets {
        if !covered {
            uncovered = append(uncovered, i)
        }
    }
    if len(uncovered) > 0 {
        log.Warn("Some buckets are not mapped to any tier",
            "uncovered_buckets", uncovered,
            "recommendation", "add tier mappings or adjust bucket_count")
    }

    // 4. Check decay ratio sanity
    if bs.config.DecayRatio < 0.5 || bs.config.DecayRatio > 0.9 {
        log.Warn("decay_ratio may be too extreme",
            "decay_ratio", bs.config.DecayRatio,
            "recommendation", "use 0.6-0.8 for most use cases")
    }

    return nil
}
```

---

## 4. Migration & Backward Compatibility

### 4.1 Default Behavior

**For new installations:**
```ini
# config/gateway.ini (default)
[bucket_scheduler]
enabled = true
bucket_count = 10  # Safe default
```

**For existing installations (pre-bucket-scheduler):**
```ini
# config/gateway.ini (legacy)
[scheduler]
# Old priority-based scheduler (P0-P4)
enabled = true
```

**Migration path:**
```
1. Keep old scheduler as default for backward compatibility
2. Bucket scheduler is opt-in via bucket_scheduler.enabled = true
3. Provide migration tool:
   tokligence migrate-to-bucket-scheduler --dry-run
```

### 4.2 Configuration Migration

```bash
# Migrate from old priority config to bucket config
tokligence migrate-to-bucket-scheduler \
  --old-config config/old_gateway.ini \
  --new-config config/gateway.ini \
  --bucket-count 10

# Output:
# ✓ Detected 5 priority levels (P0-P4) in old config
# ✓ Recommended bucket_count: 10
# ✓ Generated tier mappings:
#   - P0 (internal/production) → buckets [0, 1]
#   - P1 (internal/staging)    → buckets [2, 3]
#   - P2 (external/premium)    → buckets [4, 5]
#   - P3 (external/standard)   → buckets [6, 8]
#   - P4 (spot)                → buckets [9, 9]
#
# ✓ Migration config written to: config/gateway.ini
```

---

## 5. Validation & Constraints

### 5.1 Bucket Count Constraints

```yaml
bucket_count:
  minimum: 2              # At least 2 buckets (high priority + low priority)
  maximum: 100            # Hard limit to prevent runaway config
  recommended_min: 5      # Practical minimum for real use
  recommended_max: 50     # Practical maximum for most use cases
  default: 10             # Safe default

  warnings:
    - bucket_count < 5:   "Too few buckets, consider at least 5"
    - bucket_count > 50:  "High bucket count may complicate config"
    - bucket_count > 100: "ERROR: exceeds maximum"
```

### 5.2 Capacity Distribution Validation

**Problem:** With high decay ratios, later buckets have near-zero capacity.

```
bucket_count = 100
decay_ratio = 0.5

Bucket 0:  100.000 RPS
Bucket 10:   0.098 RPS
Bucket 20:   0.000095 RPS  (effectively zero)
Bucket 50:   ~0 RPS
```

**Validation:**

```go
func (bs *BucketScheduler) validateDistribution() error {
    minMeaningfulCapacity := 0.01  // 0.01 RPS = 36 requests/hour

    zeroCapacityBuckets := 0
    for i, bucket := range bs.buckets {
        if bucket.CapacityRPS < minMeaningfulCapacity {
            zeroCapacityBuckets++
        }
    }

    zeroCapacityPct := float64(zeroCapacityBuckets) / float64(bs.config.BucketCount) * 100

    if zeroCapacityPct > 30 {
        log.Warn("Many buckets have near-zero capacity",
            "bucket_count", bs.config.BucketCount,
            "zero_capacity_buckets", zeroCapacityBuckets,
            "percentage", fmt.Sprintf("%.1f%%", zeroCapacityPct),
            "recommendation", "reduce bucket_count or increase decay_ratio")
    }

    return nil
}
```

**Automatic adjustment:**

```go
func (bs *BucketScheduler) suggestOptimalConfig() *BucketConfig {
    // Find bucket where capacity drops below threshold
    minCapacity := 0.01  // 0.01 RPS
    optimalBucketCount := bs.config.BucketCount

    for i, bucket := range bs.buckets {
        if bucket.CapacityRPS < minCapacity {
            optimalBucketCount = i
            break
        }
    }

    if optimalBucketCount < bs.config.BucketCount {
        log.Info("Suggested optimization",
            "current_bucket_count", bs.config.BucketCount,
            "suggested_bucket_count", optimalBucketCount,
            "reason", "later buckets have near-zero capacity")
    }

    return &BucketConfig{
        BucketCount: optimalBucketCount,
        // ... copy other config
    }
}
```

### 5.3 Runtime Validation

```go
// Validate on config reload (hot-reload support)
func (bs *BucketScheduler) ReloadConfig(newConfig *BucketConfig) error {
    // Bucket count changes require restart
    if newConfig.BucketCount != bs.config.BucketCount {
        return fmt.Errorf("bucket_count change requires gateway restart (current: %d, new: %d)",
            bs.config.BucketCount, newConfig.BucketCount)
    }

    // Other config can be hot-reloaded
    bs.mu.Lock()
    defer bs.mu.Unlock()

    // Update capacities
    bs.config.BaseCapacityRPS = newConfig.BaseCapacityRPS
    bs.config.DecayRatio = newConfig.DecayRatio

    // Recalculate bucket capacities
    if err := bs.recalculateCapacities(); err != nil {
        return err
    }

    log.Info("Configuration reloaded",
        "base_capacity_rps", bs.config.BaseCapacityRPS,
        "decay_ratio", bs.config.DecayRatio)

    return nil
}
```

---

## 6. Edge Cases & Failure Modes

### 6.1 Edge Case: bucket_count = 2

**Minimal configuration:**

```ini
[bucket_scheduler]
bucket_count = 2
base_capacity_rps = 100
decay_ratio = 0.5

# Result:
# Bucket 0: 100 RPS (high priority)
# Bucket 1: 50 RPS  (low priority)
```

**Use case:** Very simple deployment (internal vs external).

**Verdict:** ✅ Works, but users should be warned this is minimal.

### 6.2 Edge Case: bucket_count = 100

**Maximum configuration:**

```ini
[bucket_scheduler]
bucket_count = 100
base_capacity_rps = 100
decay_ratio = 0.7

# Result:
# Bucket 0:  100.000 RPS
# Bucket 10:   2.824 RPS
# Bucket 20:   0.080 RPS
# Bucket 50:   0.000002 RPS (effectively zero)
```

**Problems:**
- Buckets 50-99 have negligible capacity
- Config complexity high
- Monitoring cardinality high (1000 metric series)

**Mitigation:**
- Warn user on startup
- Suggest optimal bucket count based on capacity distribution
- Allow but don't recommend

**Verdict:** ✅ Allow, but warn and suggest optimization.

### 6.3 Failure Mode: Invalid Tier Mappings

**Problem:**

```yaml
# bucket_count = 10
tier_mappings:
  internal_production:
    bucket_range: [0, 15]  # ❌ Exceeds bucket_count
```

**Detection:**

```go
func (bs *BucketScheduler) validateTierMappings() error {
    for tierName, bucketRange := range bs.tierMappings {
        if bucketRange.Min < 0 {
            return fmt.Errorf("tier %s: min bucket %d is negative", tierName, bucketRange.Min)
        }
        if bucketRange.Max >= bs.config.BucketCount {
            return fmt.Errorf("tier %s: max bucket %d exceeds bucket_count %d",
                tierName, bucketRange.Max, bs.config.BucketCount)
        }
        if bucketRange.Min > bucketRange.Max {
            return fmt.Errorf("tier %s: invalid range [%d, %d]",
                tierName, bucketRange.Min, bucketRange.Max)
        }
    }
    return nil
}
```

**Handling:** Fail fast on startup (don't allow invalid config).

### 6.4 Failure Mode: bucket_count Change Without Restart

**Problem:** User changes bucket_count from 10 → 20 and reloads config (SIGHUP).

**Impact:**
- Existing buckets [0-9] still in memory
- Config references buckets [0-19]
- Mismatch causes crashes

**Prevention:**

```go
func (bs *BucketScheduler) ReloadConfig(newConfig *BucketConfig) error {
    if newConfig.BucketCount != bs.config.BucketCount {
        log.Error("bucket_count change requires restart",
            "current", bs.config.BucketCount,
            "new", newConfig.BucketCount)
        return fmt.Errorf("bucket_count change not allowed during hot-reload (requires restart)")
    }
    // ... proceed with other config changes
}
```

**Alternative:** Support dynamic bucket count changes (complex).

```go
func (bs *BucketScheduler) ResizeBuckets(newCount int) error {
    bs.mu.Lock()
    defer bs.mu.Unlock()

    if newCount > bs.config.BucketCount {
        // Add new buckets
        for i := bs.config.BucketCount; i < newCount; i++ {
            bs.buckets = append(bs.buckets, &Bucket{
                ID:          i,
                CapacityRPS: bs.calculateCapacity(i),
                Queue:       NewRequestQueue(),
            })
        }
    } else if newCount < bs.config.BucketCount {
        // Remove buckets (drain queues first)
        for i := newCount; i < bs.config.BucketCount; i++ {
            if bs.buckets[i].Queue.Len() > 0 {
                return fmt.Errorf("cannot remove bucket %d: queue not empty", i)
            }
        }
        bs.buckets = bs.buckets[:newCount]
    }

    bs.config.BucketCount = newCount
    return nil
}
```

**Recommendation:** Require restart for bucket_count changes (simpler, safer).

---

## 7. Best Practices & Recommendations

### 7.1 Choosing Bucket Count

**Decision tree:**

```
Start here: What's your deployment size?

├─ Small (1-10 services, < 100 RPS)
│  └─ Use bucket_count = 5
│     Buckets: [internal-prod, internal-stage, internal-dev, external, spot]

├─ Medium (10-100 services, 100-1000 RPS)
│  └─ Use bucket_count = 10 (DEFAULT)
│     Buckets: [internal-prod-critical, internal-prod, internal-stage, internal-dev,
│                external-premium, external-standard, external-basic, batch-high, batch-low, spot]

├─ Large (100+ services, 1000+ RPS)
│  └─ Use bucket_count = 20-30
│     Fine-grained separation of internal teams + external customer tiers

└─ Multi-tenant SaaS (1000+ customers)
   └─ Use bucket_count = 50-100
      Each customer tier gets dedicated buckets
```

### 7.2 Recommended Configurations

**config/presets/small.ini:**

```ini
[bucket_scheduler]
bucket_count = 5
base_capacity_rps = 50
decay_ratio = 0.7
distribution_model = exponential
```

**config/presets/medium.ini (DEFAULT):**

```ini
[bucket_scheduler]
bucket_count = 10
base_capacity_rps = 100
decay_ratio = 0.7
distribution_model = exponential
```

**config/presets/large.ini:**

```ini
[bucket_scheduler]
bucket_count = 30
base_capacity_rps = 500
decay_ratio = 0.75
distribution_model = hybrid
```

**config/presets/saas.ini:**

```ini
[bucket_scheduler]
bucket_count = 100
base_capacity_rps = 1000
decay_ratio = 0.8
distribution_model = hybrid
```

### 7.3 Configuration Wizard

```bash
# Interactive configuration
tokligence bucket-scheduler configure

# Output:
? How many services/teams will use this gateway? (1-10, 10-100, 100+, 1000+)
> 10-100

? What's your total LLM capacity? (Run 'tokligence benchmark' to find out)
> 150 RPS

? Do you need fine-grained customer SLAs? (y/n)
> n

✓ Recommended configuration:
  bucket_count: 10
  base_capacity_rps: 150
  decay_ratio: 0.7
  distribution_model: exponential

✓ Configuration written to: config/gateway.ini

Next steps:
  1. Review tier mappings in config/tier_mappings.yaml
  2. Run: tokligence validate-config
  3. Start gateway: make gds
```

---

## 8. Final Recommendation

### 8.1 Verdict: ✅ **Make bucket_count Configurable**

**Rationale:**
1. ✅ Technically feasible with minimal complexity
2. ✅ Memory/CPU costs are negligible (< 10 MB, < 3% CPU even at 100 buckets)
3. ✅ Enables flexibility for different use cases
4. ✅ Future-proof (can grow as needed)
5. ✅ Easy to implement with proper abstraction

### 8.2 Implementation Checklist

**Phase 1: Core Implementation**
- [ ] Make `BucketScheduler` use dynamic slice instead of fixed array
- [ ] Add `bucket_count` to config schema
- [ ] Implement percentage-based tier mappings
- [ ] Add validation for bucket count (2-100 range)
- [ ] Add startup warnings for extreme values

**Phase 2: Safety & Validation**
- [ ] Validate tier mappings don't exceed bucket count
- [ ] Warn if many buckets have near-zero capacity
- [ ] Prevent bucket_count changes during hot-reload (require restart)
- [ ] Add config migration tool for legacy systems

**Phase 3: User Experience**
- [ ] Provide preset configs (small/medium/large/saas)
- [ ] Add configuration wizard (`tokligence bucket-scheduler configure`)
- [ ] Add capacity distribution visualization (`tokligence bucket-scheduler visualize`)
- [ ] Update documentation with bucket count selection guide

**Phase 4: Monitoring**
- [ ] Add metrics for bucket count
- [ ] Add dashboard templates for different bucket counts
- [ ] Add alerts for uncovered buckets
- [ ] Add capacity utilization heatmaps

### 8.3 Configuration Template

```ini
# config/gateway.ini

[bucket_scheduler]
# Enable bucket-based scheduling
enabled = true

# Number of capacity buckets (2-100)
# Recommended by deployment size:
#   Small (< 100 RPS):        5 buckets
#   Medium (100-1000 RPS):    10 buckets (DEFAULT)
#   Large (1000+ RPS):        20-30 buckets
#   Multi-tenant SaaS:        50-100 buckets
bucket_count = 10

# Capacity distribution model
distribution_model = exponential  # exponential | hybrid | linear | custom

# Exponential decay ratio (0 < ratio < 1)
# Each bucket has capacity = previous_bucket * decay_ratio
# Recommended: 0.6-0.8 (0.7 is good balance)
decay_ratio = 0.7

# Base capacity (bucket 0) in RPS
# Run 'tokligence benchmark' to determine this value
# Use 70-85% of benchmark result for safety margin
base_capacity_rps = 100

# Scheduling mode per tier
# See config/tier_mappings.yaml for tier-to-bucket mappings
```

### 8.4 Documentation Requirements

**User-facing docs:**
1. "Choosing the Right Bucket Count" guide
2. Bucket count migration guide (5 → 10 → 20 → 50)
3. Troubleshooting guide for common config errors
4. Performance impact table (bucket count vs CPU/memory)

**Operator runbook:**
1. How to benchmark capacity (`tokligence benchmark`)
2. How to validate config (`tokligence validate-config`)
3. How to visualize distribution (`tokligence bucket-scheduler visualize`)
4. How to monitor bucket utilization (Grafana dashboards)

---

## Conclusion

**Making bucket_count configurable is highly recommended.**

**Key Points:**
- ✅ Default to 10 buckets (good for 95% of users)
- ✅ Allow 2-100 bucket range
- ✅ Use percentage-based tier mappings (bucket-count-agnostic config)
- ✅ Validate on startup, fail fast on invalid config
- ✅ Require restart for bucket_count changes (don't support hot-reload)
- ✅ Provide presets and configuration wizard
- ⚠️ Warn users when bucket_count is extreme (< 5 or > 50)
- ⚠️ Suggest optimal bucket_count based on capacity distribution

**副作用 (Side Effects):**
- 轻微增加内存（线性增长，可忽略）
- AtLeast模式下CPU开销线性增长（但bitmap优化后可接受）
- 配置复杂度随桶数增加（但百分比映射缓解此问题）
- 监控指标基数增加（100桶 = 1000个时间序列，Prometheus可处理）

**总体评估：收益 >> 成本**
