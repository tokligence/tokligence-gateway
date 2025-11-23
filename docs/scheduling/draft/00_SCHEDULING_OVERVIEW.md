# Tokligence Gateway Scheduling System - Overview

**Version:** 1.0
**Date:** 2025-02-01
**Status:** Design Documentation Index

---

## Document Structure

This directory contains the complete design documentation for Tokligence Gateway's request scheduling, routing, and capacity management systems.

### Core Design Documents

| Document | Title | Purpose | Status |
|----------|-------|---------|--------|
| **00_SCHEDULING_OVERVIEW.md** | Overview & Index | This document - navigation and integration guide | ✅ Current |
| **01_PRIORITY_BASED_SCHEDULING.md** | Priority-Based Scheduling | Traditional P0-P4 priority queues with fairness algorithms | ✅ Complete |
| **02_BUCKET_BASED_SCHEDULING.md** | Bucket-Based Capacity Scheduling | 100-bucket capacity allocation model analysis | ✅ Complete |
| **03_CONFIGURABLE_BUCKET_COUNT.md** | Configurable Bucket Count | Making bucket count configurable (2-100 buckets) | ✅ Complete |
| **04_TOKEN_BASED_ROUTING.md** | Token-Based Routing | Database-driven API token routing with header support | ✅ Complete |
| **05_ORTHOGONALITY_ANALYSIS.md** | Orthogonality & Integration | How all designs work together | ✅ Complete |

---

## Quick Navigation

### For New Readers: Start Here

**Read in this order:**
1. **This overview** (00_SCHEDULING_OVERVIEW.md) - Understand the landscape
2. **Priority-Based Scheduling** (01) - Traditional approach, well-understood
3. **Bucket-Based Scheduling** (02) - Novel capacity-centric model
4. **Orthogonality Analysis** (05) - How they integrate

### For Implementers

**Implementation-ready designs:**
1. **Token-Based Routing** (04) - Ready to implement first (foundational)
2. **Configurable Bucket Count** (03) - Implementation design with code examples
3. **Priority-Based Scheduling** (01) - Proven algorithms, ready to implement

### For Operators

**Choosing the right scheduling model:**
- Small deployment (< 100 RPS): Priority-Based (P0-P4) - see Section 01
- Medium deployment (100-1000 RPS): 10-Bucket Model - see Sections 02, 03
- Large enterprise (1000+ RPS): 20-30 Bucket Model - see Section 03
- Multi-tenant SaaS (many customers): Hybrid approach - see Section 05

---

## System Architecture - Big Picture

### Three Orthogonal Layers

```
┌─────────────────────────────────────────────────────────────────┐
│  Layer 1: REQUEST CLASSIFICATION                                │
│  ─────────────────────────────────────────────────────────────  │
│  How do we identify WHO this request is from and WHAT it needs? │
│                                                                   │
│  Inputs: HTTP Request                                            │
│  Mechanisms:                                                      │
│    • HTTP Headers (X-TGW-Source, X-TGW-Priority)                │
│    • API Token Lookup (database-driven)                         │
│    • Default Policy                                              │
│  Output: RoutingMetadata                                         │
│          {priority_tier, environment, priority_level}           │
│                                                                   │
│  📄 See: 04_TOKEN_BASED_ROUTING.md                              │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 2: CAPACITY ALLOCATION                                    │
│  ─────────────────────────────────────────────────────────────  │
│  Given classification, WHERE does this request get capacity?     │
│                                                                   │
│  Inputs: RoutingMetadata                                         │
│  Mechanisms (CHOOSE ONE):                                        │
│    A) Priority-Based: Map to priority queue (P0-P4)             │
│    B) Bucket-Based: Map to capacity bucket (0-99)               │
│  Output: Queue/Bucket Assignment                                 │
│                                                                   │
│  📄 See: 01_PRIORITY_BASED_SCHEDULING.md (Option A)             │
│  📄 See: 02_BUCKET_BASED_SCHEDULING.md (Option B)               │
│  📄 See: 03_CONFIGURABLE_BUCKET_COUNT.md (Option B config)      │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│  Layer 3: SCHEDULING & EXECUTION                                 │
│  ─────────────────────────────────────────────────────────────  │
│  Given queue/bucket, WHEN and HOW do we execute this request?   │
│                                                                   │
│  Inputs: Queue/Bucket + Current System State                     │
│  Mechanisms:                                                      │
│    • Scheduling Algorithm (Strict, WFQ, DRR, AtLeast)           │
│    • Capacity Guard (90% internal threshold, etc.)              │
│    • Time Window Adjustments (business hours vs night)          │
│    • Quota Management (token budgets)                           │
│    • Preemption (optional)                                       │
│  Output: Request Execution                                       │
│                                                                   │
│  📄 See: 01_PRIORITY_BASED_SCHEDULING.md Section 3              │
│  📄 See: 02_BUCKET_BASED_SCHEDULING.md Section 5                │
└─────────────────────────────────────────────────────────────────┘
```

### Key Insight: Orthogonality

**The three layers are ORTHOGONAL** - you can mix and match:

| Layer 1 (Classification) | Layer 2 (Allocation) | Layer 3 (Scheduling) |
|-------------------------|----------------------|----------------------|
| HTTP Header | → Priority-Based | → Strict Priority |
| HTTP Header | → Bucket-Based | → AtLeast Mode |
| API Token | → Priority-Based | → Weighted Fair Queuing |
| API Token | → Bucket-Based | → Hybrid Strict/AtLeast |

**All 8 combinations are valid and useful!**

---

## Design Decision: Priority vs Bucket

### When to Use Priority-Based Scheduling (01)

**✅ Use priority-based if:**
- You have 3-10 distinct service levels
- You want simple, well-understood semantics (P0 = critical, P4 = spot)
- You prefer abstract priority over concrete capacity
- Your team is familiar with Kubernetes pod priorities or similar systems

**Example use cases:**
- Internal workloads (prod/staging/dev)
- Small-medium deployments
- B2B SaaS with 3-5 customer tiers

**Pros:**
- Simpler mental model
- Less configuration
- Proven algorithms (Kubernetes, NGINX, etc. use this)

**Cons:**
- Abstract (what does "Priority 3" mean in RPS?)
- Less granular (only 5 levels)

### When to Use Bucket-Based Scheduling (02, 03)

**✅ Use bucket-based if:**
- You want concrete capacity guarantees (Bucket 5 = 17 RPS)
- You need fine-grained differentiation (10-100 tiers)
- You're selling LLM capacity and need precise SLAs
- You want to explain capacity to customers in concrete terms

**Example use cases:**
- Multi-tenant SaaS with many customer tiers
- Gateway-as-a-Service (selling capacity)
- Large enterprises with 50+ teams

**Pros:**
- Concrete, measurable (Bucket X = Y RPS)
- Very fine-grained (10-100 buckets)
- Easy to explain to customers ("You're in bucket 30 = 0.5 RPS guaranteed")

**Cons:**
- More configuration complexity
- Requires capacity benchmarking
- Newer concept (less battle-tested)

### Recommended Default: 10-Bucket Model

**Best of both worlds:**
- Fine enough for most use cases (10 tiers)
- Concrete capacity allocation
- Manageable configuration
- Can map to traditional priorities:
  ```
  P0 → Buckets 0-1
  P1 → Buckets 2-3
  P2 → Buckets 4-5
  P3 → Buckets 6-7
  P4 → Buckets 8-9
  ```

---

## Integration Patterns

### Pattern 1: Pure Priority-Based

```
HTTP Request
  ↓
[Token/Header Classification]
  ↓
priority_tier="internal", environment="production"
  ↓
[Priority Mapping]
  ↓
priority_level=0 (P0)
  ↓
[Priority Queue Scheduler]
  ↓
Execute
```

**Config:**
```ini
[scheduler]
type = priority
priority_levels = 5  # P0-P4
```

### Pattern 2: Pure Bucket-Based

```
HTTP Request
  ↓
[Token/Header Classification]
  ↓
priority_tier="internal", environment="production"
  ↓
[Bucket Mapping]
  ↓
bucket=0 (100 RPS capacity)
  ↓
[Bucket Scheduler]
  ↓
Execute
```

**Config:**
```ini
[scheduler]
type = bucket
bucket_count = 10
base_capacity_rps = 100
```

### Pattern 3: Hybrid (Recommended)

```
HTTP Request
  ↓
[Token/Header Classification]
  ↓
priority_tier="internal", environment="production", priority=0
  ↓
[Bucket Mapping with Priority Hint]
  ↓
bucket_range=[0, 1], priority_hint=0
  ↓
final_bucket = 0 + (priority / 4) * range_size = 0
  ↓
[Bucket Scheduler (AtLeast mode)]
  ↓
Execute
```

**Config:**
```ini
[scheduler]
type = bucket
bucket_count = 10

# Tier → bucket range mapping
[tier_mappings]
internal_production.bucket_range = [0, 1]
internal_production.mode = strict

external_standard.bucket_range = [6, 7]
external_standard.mode = atleast
```

---

## Time-Window Integration

**Time windows work with BOTH priority and bucket models:**

### With Priority-Based

```yaml
time_windows:
  business_hours:
    schedule: "0 9 * * 1-5"
    rules:
      - priority: 4  # Spot priority
        adjusted_priority: 4  # No change

  night_hours:
    schedule: "0 22 * * *"
    rules:
      - priority: 4  # Spot priority
        adjusted_priority: 2  # PROMOTED to P2 at night
```

### With Bucket-Based

```yaml
time_windows:
  business_hours:
    schedule: "0 9 * * 1-5"
    rules:
      - tier: external_standard
        bucket_shift: +2  # Move to lower buckets (less capacity)

  night_hours:
    schedule: "0 22 * * *"
    rules:
      - tier: external_standard
        bucket_shift: -2  # Move to higher buckets (more capacity)
```

**Orthogonal:** Time windows adjust priority/bucket, but scheduling logic remains the same.

---

## Capacity Guard Integration

**Capacity guard (90% internal threshold) works with BOTH models:**

### With Priority-Based

```go
func (pq *PriorityQueue) Enqueue(req *Request) error {
    // Check tier
    if req.PriorityTier == "external" {
        if !capacityGuard.CanAcceptExternal() {
            return ErrCapacityLimitReached
        }
    }

    // Enqueue to priority queue
    pq.queues[req.Priority].Push(req)
}
```

### With Bucket-Based

```go
func (bs *BucketScheduler) Schedule(req *Request) error {
    // Check tier
    if req.PriorityTier == "external" {
        if !capacityGuard.CanAcceptExternal() {
            return ErrCapacityLimitReached
        }
    }

    // Schedule to bucket
    bs.buckets[req.Bucket].Enqueue(req)
}
```

**Orthogonal:** Capacity guard is a gate BEFORE scheduling, works with any scheduler.

---

## Migration Path

### Phase 1: Start Simple (Priority-Based)

```ini
# config/gateway.ini
[scheduler]
type = priority
priority_levels = 5
```

**Pros:** Quick to implement, well-understood
**Timeline:** 2-3 weeks

### Phase 2: Add Token Routing

```ini
[token_routing]
enabled = true
```

**Pros:** Database-driven token management
**Timeline:** +2 weeks

### Phase 3: Evaluate Bucket Model

**After 3-6 months:**
- Analyze usage patterns
- Do operators need more granularity?
- Are 5 priorities sufficient?

**If yes → migrate to 10-bucket model:**

```bash
tokligence migrate-to-bucket-scheduler \
  --current-config config/gateway.ini \
  --bucket-count 10
```

### Phase 4: Scale to 20-50 Buckets (Optional)

**Only if:**
- Large multi-tenant deployment
- Many customer tiers
- Need fine-grained SLAs

---

## Performance Comparison

| Metric | Priority (P0-P4) | Bucket (10) | Bucket (100) |
|--------|-----------------|-------------|--------------|
| **Memory** | ~2 KB | ~640 KB | ~6.4 MB |
| **CPU (Strict)** | 0.5% | 0.5% | 0.5% |
| **CPU (WFQ/AtLeast)** | 1.0% | 1.0% | 3.0% |
| **Config Complexity** | Low | Low | High |
| **Granularity** | 5 levels | 10 levels | 100 levels |

**All are performant - choose based on use case, not performance.**

---

## Next Steps

### For Readers

1. Read 01_PRIORITY_BASED_SCHEDULING.md
2. Read 02_BUCKET_BASED_SCHEDULING.md
3. Read 05_ORTHOGONALITY_ANALYSIS.md
4. Decide which model fits your use case

### For Implementers

1. Start with 04_TOKEN_BASED_ROUTING.md (foundational)
2. Implement either:
   - Option A: 01_PRIORITY_BASED_SCHEDULING.md (simpler)
   - Option B: 02 + 03 BUCKET_BASED (more flexible)
3. Add time windows and capacity guard (works with both)

### For Operators

1. Run `tokligence benchmark` to measure capacity
2. Choose scheduler type based on use case
3. Configure tier mappings
4. Monitor and adjust

---

## Glossary

| Term | Definition |
|------|------------|
| **Priority Tier** | High-level classification: internal, external, premium, spot |
| **Environment** | Deployment stage: production, staging, dev |
| **Priority Level** | Numeric priority (0-4 in priority-based, or derived from bucket) |
| **Bucket** | Capacity allocation unit with concrete RPS guarantee |
| **Strict Mode** | Request can only use assigned bucket/priority |
| **AtLeast Mode** | Request can use assigned bucket or better (opportunistic) |
| **Capacity Guard** | 90% internal threshold protection for external requests |
| **Time Window** | Scheduled period with adjusted priorities/quotas |
| **WFQ** | Weighted Fair Queuing - fairness algorithm |
| **DRR** | Deficit Round Robin - another fairness algorithm |

---

## Related Documentation

- `../CLAUDE.md` - Project overview and development guidelines
- `../QUICK_START.md` - Getting started with Tokligence Gateway
- `../codex-to-anthropic.md` - Codex CLI integration
- `../claude_code-to-openai.md` - Claude Code integration
- `../../arc_design/01_comprehensive_system_architecture.md` - Overall system architecture

---

**End of Overview**
