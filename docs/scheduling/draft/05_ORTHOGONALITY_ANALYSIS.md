# Scheduling System Orthogonality Analysis

**Version:** 1.0
**Date:** 2025-02-01
**Status:** Design Analysis
**Related Documents:**
- `01_PRIORITY_BASED_SCHEDULING.md` - Priority queue design
- `02_BUCKET_BASED_SCHEDULING.md` - Bucket capacity design
- `04_TOKEN_BASED_ROUTING.md` - Request classification

---

## Executive Summary

This document analyzes the **orthogonality** (independence) of different scheduling system components and demonstrates how they can be mixed and matched without conflicts.

**Key Finding: ✅ All components are ORTHOGONAL**

The scheduling system can be decomposed into three independent layers:
1. **Classification Layer** - WHO is this request from? (Token/Header routing)
2. **Allocation Layer** - WHERE does capacity come from? (Priority vs Bucket)
3. **Execution Layer** - WHEN/HOW to execute? (Scheduling algorithms)

Each layer can be swapped independently without affecting others.

---

## Table of Contents

1. [Orthogonality Principle](#1-orthogonality-principle)
2. [Layer Decomposition](#2-layer-decomposition)
3. [Interface Contracts](#3-interface-contracts)
4. [Mix-and-Match Combinations](#4-mix-and-match-combinations)
5. [Implementation Architecture](#5-implementation-architecture)
6. [Migration Scenarios](#6-migration-scenarios)
7. [Trade-off Analysis](#7-trade-off-analysis)

---

## 1. Orthogonality Principle

### 1.1 Definition

**Orthogonal design** means components can vary independently without affecting each other.

```
Component A ⊥ Component B

Changing A's implementation does NOT require changing B
```

### 1.2 Benefits

✅ **Flexibility** - Mix and match components
✅ **Testability** - Test components in isolation
✅ **Maintainability** - Change one without breaking others
✅ **Evolvability** - Easy to add new implementations

### 1.3 Visual Representation

```
┌─────────────────────────────────────────────────────────────┐
│  Orthogonal Axes (Independent Choices)                      │
└─────────────────────────────────────────────────────────────┘

Classification:
├─ HTTP Header-based
├─ Token-based
└─ Default policy

         ×  (orthogonal to)

Allocation:
├─ Priority-based (P0-P4)
└─ Bucket-based (0-99)

         ×  (orthogonal to)

Scheduling:
├─ Strict Priority
├─ Weighted Fair Queuing
├─ Deficit Round Robin
└─ AtLeast (opportunistic)

= 3 × 2 × 4 = 24 valid combinations
```

---

## 2. Layer Decomposition

### 2.1 Layer 1: Request Classification

**Responsibility:** Extract metadata from incoming request

**Input:** `http.Request`

**Output:** `RoutingMetadata`

```go
type RoutingMetadata struct {
    // Source identification
    PriorityTier string  // "internal" | "external" | "premium" | "spot"
    Environment  string  // "production" | "staging" | "dev"
    AccountID    string
    TokenID      string

    // Priority hints (may come from header or token)
    PriorityHint int     // 0-4 (P0-P4)
    WorkloadTag  string  // "realtime" | "batch"

    // Routing decision
    RoutedBy     string  // "header" | "token" | "default"
}
```

**Implementation Options:**

| Option | Mechanism | Document |
|--------|-----------|----------|
| **A** | HTTP Header (`X-TGW-Source`, etc.) | 04_TOKEN_BASED_ROUTING.md §1.3, §6.3 |
| **B** | API Token lookup (database) | 04_TOKEN_BASED_ROUTING.md §3 |
| **C** | Default policy | 04_TOKEN_BASED_ROUTING.md §6.3 |
| **D** | Hybrid (Header → Token → Default) | 04_TOKEN_BASED_ROUTING.md §1.3 |

**Orthogonal to:** Layers 2 and 3 (allocation and scheduling don't care how metadata was obtained)

### 2.2 Layer 2: Capacity Allocation

**Responsibility:** Map metadata to capacity source (queue/bucket)

**Input:** `RoutingMetadata`

**Output:** `AllocationDecision`

```go
type AllocationDecision struct {
    // Allocation target (one of these)
    PriorityLevel int     // For priority-based: 0-4
    BucketID      int     // For bucket-based: 0-99

    // Scheduling hints
    Mode          SchedulingMode  // Strict | AtLeast | WFQ
    Weight        float64         // For weighted scheduling
    QueueName     string          // Human-readable identifier

    // Capacity metadata
    GuaranteedRPS float64  // For bucket-based
    MaxQueueDepth int      // Queue limit
}
```

**Implementation Options:**

| Option | Model | Document |
|--------|-------|----------|
| **A** | Priority-based (P0-P4) | 01_PRIORITY_BASED_SCHEDULING.md §3.1 |
| **B** | Bucket-based (10 buckets) | 02_BUCKET_BASED_SCHEDULING.md §4 |
| **C** | Bucket-based (100 buckets) | 02_BUCKET_BASED_SCHEDULING.md §3 |
| **D** | Bucket-based (configurable) | 03_CONFIGURABLE_BUCKET_COUNT.md §3 |

**Orthogonal to:** Layers 1 and 3 (doesn't care how metadata was created, or how scheduling works)

### 2.3 Layer 3: Scheduling & Execution

**Responsibility:** Decide WHEN and HOW to execute queued requests

**Input:** `AllocationDecision` + Current System State

**Output:** Request Execution

**Implementation Options:**

| Algorithm | Characteristics | Starvation Risk | Document |
|-----------|----------------|-----------------|----------|
| **Strict Priority** | Always dequeue highest priority first | High (low priority starves) | 01 §3.3 |
| **Weighted Fair Queuing (WFQ)** | Allocate capacity proportional to weights | Low | 01 §3.3 |
| **Deficit Round Robin (DRR)** | Fair scheduling with quantum | Very low | 01 §3.3 |
| **Hybrid** | P0 strict, P1-P4 use WFQ | Medium | 01 §3.3 |
| **AtLeast (opportunistic)** | Try better buckets first | Low | 02 §5.2 |

**Orthogonal to:** Layers 1 and 2 (scheduling algorithm works regardless of how request was classified or allocated)

---

## 3. Interface Contracts

### 3.1 Layer 1 → Layer 2 Interface

```go
// Interface: Classifier
type Classifier interface {
    // Extract metadata from request
    Classify(r *http.Request) (*RoutingMetadata, error)
}

// Implementations:
type HeaderClassifier struct { ... }   // Read X-TGW-* headers
type TokenClassifier struct { ... }    // Lookup API token
type HybridClassifier struct { ... }   // Try header → token → default
```

**Contract:**
- Input: HTTP request
- Output: RoutingMetadata struct
- No assumptions about downstream allocation or scheduling

### 3.2 Layer 2 → Layer 3 Interface

```go
// Interface: Allocator
type Allocator interface {
    // Map metadata to capacity allocation
    Allocate(meta *RoutingMetadata) (*AllocationDecision, error)
}

// Implementations:
type PriorityAllocator struct { ... }  // Map to P0-P4
type BucketAllocator struct { ... }    // Map to bucket 0-99
```

**Contract:**
- Input: RoutingMetadata
- Output: AllocationDecision struct
- No assumptions about upstream classification or downstream scheduling

### 3.3 Complete Request Flow Interface

```go
// Top-level orchestrator
type RequestScheduler struct {
    classifier Classifier
    allocator  Allocator
    scheduler  Scheduler
}

func (rs *RequestScheduler) HandleRequest(r *http.Request) error {
    // Layer 1: Classify
    meta, err := rs.classifier.Classify(r)
    if err != nil {
        return err
    }

    // Layer 2: Allocate
    alloc, err := rs.allocator.Allocate(meta)
    if err != nil {
        return err
    }

    // Layer 3: Schedule
    return rs.scheduler.Schedule(r, alloc)
}
```

**Plug-and-play:** Swap any layer without affecting others.

---

## 4. Mix-and-Match Combinations

### 4.1 Combination Matrix

| ID | Classification | Allocation | Scheduling | Use Case |
|----|---------------|------------|------------|----------|
| **C1** | Header | Priority | Strict | Multi-gateway arch, simple priorities |
| **C2** | Header | Priority | WFQ | Multi-gateway arch, fair scheduling |
| **C3** | Header | Bucket(10) | Strict | Multi-gateway arch, capacity SLAs |
| **C4** | Header | Bucket(10) | AtLeast | Multi-gateway arch, opportunistic |
| **C5** | Token | Priority | Strict | Internal workloads, simple |
| **C6** | Token | Priority | WFQ | Internal workloads, fairness |
| **C7** | Token | Bucket(10) | Strict | SaaS, guaranteed capacity |
| **C8** | Token | Bucket(10) | AtLeast | SaaS, opportunistic capacity |
| **C9** | Token | Bucket(100) | Strict | Large SaaS, fine-grained SLAs |
| **C10** | Token | Bucket(100) | Hybrid | Large SaaS, mixed guarantees |
| **C11** | Hybrid | Priority | WFQ | Flexible, default choice |
| **C12** | Hybrid | Bucket(10) | AtLeast | Flexible, capacity-aware (RECOMMENDED) |

### 4.2 Recommended Configurations

#### Configuration 1: Simple Internal Deployment

```ini
# config/gateway.ini

[classifier]
type = token              # Use API token lookup

[allocator]
type = priority           # P0-P4 priorities
priority_levels = 5

[scheduler]
algorithm = weighted_fair_queuing
weights = 8,4,2,1,1       # P0:P1:P2:P3:P4 = 8:4:2:1:1
```

**Use case:** Small company, internal workloads only, simple priorities.

#### Configuration 2: Multi-Gateway Architecture

```ini
[classifier]
type = hybrid             # Header → Token → Default

[header_routing]
enabled = true
trusted_cidrs = 10.0.0.0/8

[allocator]
type = bucket
bucket_count = 10

[scheduler]
algorithm = atleast       # Opportunistic scheduling
```

**Use case:** E-commerce company with department gateway + external gateway (from earlier discussion).

#### Configuration 3: Large Multi-Tenant SaaS

```ini
[classifier]
type = token              # Token-based only (external customers)

[allocator]
type = bucket
bucket_count = 50         # Fine-grained customer tiers

[scheduler]
algorithm = hybrid
  strict_buckets = 0-19   # Paid tiers: strict SLAs
  atleast_buckets = 20-49 # Free tier: opportunistic
```

**Use case:** 1000+ external customers with many pricing tiers.

### 4.3 Example: Swapping Allocator

**Before: Priority-based**

```go
rs := &RequestScheduler{
    classifier: NewHybridClassifier(...),
    allocator:  NewPriorityAllocator(...),  // P0-P4
    scheduler:  NewWFQScheduler(...),
}
```

**After: Bucket-based**

```go
rs := &RequestScheduler{
    classifier: NewHybridClassifier(...),      // SAME
    allocator:  NewBucketAllocator(...),       // CHANGED
    scheduler:  NewWFQScheduler(...),          // SAME
}
```

**Changes required:**
- ✅ Swap allocator implementation
- ✅ Update config file
- ❌ NO changes to classifier
- ❌ NO changes to scheduler

**Result:** Seamless migration from priority to bucket model.

---

## 5. Implementation Architecture

### 5.1 Plugin Architecture

```go
// internal/scheduler/scheduler.go

package scheduler

import "net/http"

// ============================================================================
// Core Interfaces (contracts between layers)
// ============================================================================

type Classifier interface {
    Classify(r *http.Request) (*RoutingMetadata, error)
}

type Allocator interface {
    Allocate(meta *RoutingMetadata) (*AllocationDecision, error)
}

type Scheduler interface {
    Schedule(r *http.Request, alloc *AllocationDecision) error
}

// ============================================================================
// Orchestrator
// ============================================================================

type RequestScheduler struct {
    classifier Classifier
    allocator  Allocator
    scheduler  Scheduler
}

func NewRequestScheduler(config *Config) (*RequestScheduler, error) {
    // Factory pattern: create implementations based on config
    var classifier Classifier
    switch config.ClassifierType {
    case "header":
        classifier = NewHeaderClassifier(config.HeaderConfig)
    case "token":
        classifier = NewTokenClassifier(config.TokenConfig)
    case "hybrid":
        classifier = NewHybridClassifier(config.HeaderConfig, config.TokenConfig)
    default:
        return nil, fmt.Errorf("unknown classifier type: %s", config.ClassifierType)
    }

    var allocator Allocator
    switch config.AllocatorType {
    case "priority":
        allocator = NewPriorityAllocator(config.PriorityConfig)
    case "bucket":
        allocator = NewBucketAllocator(config.BucketConfig)
    default:
        return nil, fmt.Errorf("unknown allocator type: %s", config.AllocatorType)
    }

    var scheduler Scheduler
    switch config.SchedulerAlgorithm {
    case "strict":
        scheduler = NewStrictPriorityScheduler(config.SchedulerConfig)
    case "wfq":
        scheduler = NewWFQScheduler(config.SchedulerConfig)
    case "atleast":
        scheduler = NewAtLeastScheduler(config.SchedulerConfig)
    case "hybrid":
        scheduler = NewHybridScheduler(config.SchedulerConfig)
    default:
        return nil, fmt.Errorf("unknown scheduler algorithm: %s", config.SchedulerAlgorithm)
    }

    return &RequestScheduler{
        classifier: classifier,
        allocator:  allocator,
        scheduler:  scheduler,
    }, nil
}

func (rs *RequestScheduler) HandleRequest(r *http.Request) error {
    // Step 1: Classify
    meta, err := rs.classifier.Classify(r)
    if err != nil {
        return fmt.Errorf("classification failed: %w", err)
    }

    // Step 2: Allocate
    alloc, err := rs.allocator.Allocate(meta)
    if err != nil {
        return fmt.Errorf("allocation failed: %w", err)
    }

    // Step 3: Schedule
    if err := rs.scheduler.Schedule(r, alloc); err != nil {
        return fmt.Errorf("scheduling failed: %w", err)
    }

    return nil
}
```

### 5.2 Configuration-Driven Design

```ini
# config/gateway.ini

[request_scheduler]
# Layer 1: Classification
classifier = hybrid       # header | token | hybrid

# Layer 2: Allocation
allocator = bucket        # priority | bucket

# Layer 3: Scheduling
scheduler = atleast       # strict | wfq | drr | hybrid | atleast

# ============================================================================
# Classifier Config
# ============================================================================

[classifier.header]
enabled = true
trusted_cidrs = 10.0.0.0/8
require_token = true

[classifier.token]
enabled = true
cache_ttl = 300

# ============================================================================
# Allocator Config
# ============================================================================

[allocator.priority]
priority_levels = 5

[allocator.bucket]
bucket_count = 10
base_capacity_rps = 100
decay_ratio = 0.7

# Tier mappings (percentage-based, bucket-count-agnostic)
[allocator.bucket.tier_mappings]
internal_production.bucket_range_pct = [0, 0.20]
internal_production.mode = strict

external_standard.bucket_range_pct = [0.60, 0.80]
external_standard.mode = atleast

# ============================================================================
# Scheduler Config
# ============================================================================

[scheduler.wfq]
weights = 8,4,2,1,1       # P0:P1:P2:P3:P4 or Bucket weights

[scheduler.atleast]
# No additional config needed

[scheduler.hybrid]
strict_range = [0, 19]    # Buckets 0-19: strict
atleast_range = [20, 99]  # Buckets 20-99: atleast
```

### 5.3 Testing Orthogonality

```go
// Test: Swap allocator without changing classifier or scheduler

func TestOrthogonality_SwapAllocator(t *testing.T) {
    // Setup
    classifier := NewHybridClassifier(...)
    scheduler := NewWFQScheduler(...)

    // Test 1: Priority allocator
    rs1 := &RequestScheduler{
        classifier: classifier,
        allocator:  NewPriorityAllocator(...),
        scheduler:  scheduler,
    }
    testRequest(t, rs1)  // Should work

    // Test 2: Bucket allocator (swap allocator only)
    rs2 := &RequestScheduler{
        classifier: classifier,  // SAME instance
        allocator:  NewBucketAllocator(...),  // DIFFERENT
        scheduler:  scheduler,   // SAME instance
    }
    testRequest(t, rs2)  // Should still work

    // Assert: Both produce valid allocations
    assert.NotNil(t, rs1.classifier)
    assert.NotNil(t, rs2.classifier)
    assert.Same(t, rs1.classifier, rs2.classifier)  // Shared instance
}
```

---

## 6. Migration Scenarios

### 6.1 Scenario 1: Priority → Bucket Migration

**Initial state:** Priority-based with 5 levels (P0-P4)

**Goal:** Migrate to 10-bucket model for finer granularity

**Steps:**

1. **Add bucket allocator alongside priority allocator**

```ini
# config/gateway.ini

[allocator]
# Primary allocator (current)
type = priority
priority_levels = 5

# Secondary allocator (testing)
[allocator.bucket_experimental]
enabled = false
bucket_count = 10
```

2. **Test bucket allocator in shadow mode**

```go
// Dual allocation for testing
alloc1, _ := priorityAllocator.Allocate(meta)  // Production
alloc2, _ := bucketAllocator.Allocate(meta)    // Shadow testing

// Log differences
if alloc1.PriorityLevel != alloc2.BucketID {
    log.Info("Allocation mismatch (expected during migration)",
        "priority", alloc1.PriorityLevel,
        "bucket", alloc2.BucketID)
}

// Use priority allocator for now
return scheduler.Schedule(r, alloc1)
```

3. **Gradual cutover**

```ini
# Week 1: 10% traffic to bucket allocator
[allocator]
type = weighted_router
  priority_weight = 90
  bucket_weight = 10

# Week 2: 50%
  priority_weight = 50
  bucket_weight = 50

# Week 3: 90%
  priority_weight = 10
  bucket_weight = 90

# Week 4: 100% bucket
[allocator]
type = bucket
```

4. **Remove priority allocator**

No code changes needed! Just config update.

### 6.2 Scenario 2: Token → Hybrid Classifier

**Initial state:** Token-based classification only

**Goal:** Add header-based classification for multi-gateway architecture

**Steps:**

1. **Enable hybrid classifier**

```ini
[classifier]
type = hybrid  # Was: token

[classifier.header]
enabled = true
trusted_cidrs = 10.0.0.0/8
```

2. **No other changes needed**

- Allocator: UNCHANGED
- Scheduler: UNCHANGED

**Result:** Now supports both header and token classification.

### 6.3 Scenario 3: Strict → AtLeast Scheduling

**Initial state:** Strict priority scheduling

**Goal:** Enable opportunistic scheduling (AtLeast mode) for better utilization

**Steps:**

1. **Change scheduler algorithm**

```ini
[scheduler]
algorithm = atleast  # Was: strict
```

2. **No other changes needed**

- Classifier: UNCHANGED
- Allocator: UNCHANGED

**Result:** Requests can now opportunistically use better buckets.

---

## 7. Trade-off Analysis

### 7.1 Classification Layer Trade-offs

| Classifier | Pros | Cons | When to Use |
|-----------|------|------|-------------|
| **Header-only** | Fast (no DB lookup), multi-gateway friendly | No per-user tracking, requires trusted network | Multi-gateway architectures |
| **Token-only** | Fine-grained per-user control, billing-friendly | Slower (DB lookup), cache needed | SaaS, billing required |
| **Hybrid** | Best of both (fast header, fallback to token) | More complex | Most production use cases |

### 7.2 Allocation Layer Trade-offs

| Allocator | Pros | Cons | When to Use |
|----------|------|------|-------------|
| **Priority (P0-P4)** | Simple, well-understood, proven | Abstract (no concrete capacity), coarse (5 levels) | Internal workloads, small deployments |
| **Bucket (10)** | Concrete capacity, fine-grained | Requires benchmarking | Medium deployments, SaaS |
| **Bucket (50-100)** | Very fine-grained | Complex config | Large multi-tenant SaaS |

### 7.3 Scheduling Layer Trade-offs

| Scheduler | Pros | Cons | When to Use |
|----------|------|------|-------------|
| **Strict Priority** | Simple, predictable SLAs | Starvation risk | Paid SLA tiers (strict guarantees) |
| **WFQ** | Fair, no starvation | More complex, unpredictable latency | Mixed workloads, fairness important |
| **AtLeast** | Efficient (uses idle capacity) | Less predictable | Internal workloads, opportunistic |
| **Hybrid** | Combines best (strict for paid, atleast for free) | Most complex | Large SaaS with mixed tiers |

### 7.4 Recommendation Matrix

| Use Case | Classifier | Allocator | Scheduler |
|----------|-----------|-----------|-----------|
| **Small internal** | Token | Priority | WFQ |
| **Medium internal** | Token | Bucket(10) | AtLeast |
| **Multi-gateway** | Hybrid | Bucket(10) | AtLeast |
| **Small SaaS** | Token | Priority | Strict |
| **Medium SaaS** | Token | Bucket(10) | Hybrid |
| **Large SaaS** | Token | Bucket(50) | Hybrid |

---

## 8. Conclusion

### 8.1 Key Findings

✅ **All components are orthogonal** - can be mixed and matched freely

✅ **Interfaces are clean** - well-defined contracts between layers

✅ **Migration is easy** - swap components without breaking others

✅ **Testing is simple** - test layers in isolation

### 8.2 Design Principles Validated

1. **Separation of Concerns** - Each layer has single responsibility
2. **Open/Closed Principle** - Open for extension, closed for modification
3. **Dependency Inversion** - Depend on interfaces, not implementations
4. **Single Responsibility** - Each component does one thing well

### 8.3 Recommended Approach

**Start simple, grow as needed:**

```
Phase 1: Token + Priority + WFQ
  ↓ (if need more granularity)
Phase 2: Token + Bucket(10) + AtLeast
  ↓ (if need header routing)
Phase 3: Hybrid + Bucket(10) + AtLeast
  ↓ (if need fine-grained SLAs)
Phase 4: Hybrid + Bucket(50) + Hybrid
```

Each phase is a **config change only** - no code changes needed!

---

**End of Analysis**
