# Priority Levels Configuration

**Date:** 2025-11-23
**Status:** ✅ Design Complete
**Default:** 10 priority levels (P0-P9)

---

## Overview

Gateway 的优先级队列系统支持**可配置的优先级级别数**，默认为 **10 个桶（P0-P9）**。

### Why Configurable?

- **灵活性**：不同场景需要不同粒度的优先级划分
- **性能优化**：桶数量影响调度开销
- **业务适配**：可根据用户分层数量调整

---

## Default: 10 Priority Levels

### P0-P9 Mapping

| Level | Constant | Use Case | Example |
|-------|----------|----------|---------|
| **P0** | `PriorityCritical` | Internal critical services | Health checks, metrics |
| **P1** | `PriorityUrgent` | High-value urgent requests | VIP user emergencies |
| **P2** | `PriorityHigh` | Partner/premium users | Enterprise tier |
| **P3** | `PriorityElevated` | Elevated priority | Business tier |
| **P4** | `PriorityAboveNormal` | Above normal | Pro tier |
| **P5** | `PriorityNormal` | Standard users (default) | Free tier |
| **P6** | `PriorityBelowNormal` | Below normal | Rate-limited users |
| **P7** | `PriorityLow` | Low priority | Batch API |
| **P8** | `PriorityBulk` | Bulk/batch processing | Data exports |
| **P9** | `PriorityBackground` | Background jobs | Cleanup, analytics |

### Configuration

```ini
# config/gateway.ini
[scheduler]
# Number of priority buckets (default: 10)
priority_levels = 10

# Default priority for requests without explicit priority (default: 5 = P5)
default_priority = 5

# Weights for WFQ (Weighted Fair Queuing)
# Higher weight = more bandwidth share
priority_weights = 256,128,64,32,16,8,4,2,1,1  # P0-P9
```

**Environment Variable:**
```bash
export TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS=10
export TOKLIGENCE_SCHEDULER_DEFAULT_PRIORITY=5
```

---

## Alternative: 5 Priority Levels

如果你的场景更简单，可以配置为 **5 个桶（P0-P4）**：

```ini
[scheduler]
priority_levels = 5
default_priority = 2  # P2 = Normal
priority_weights = 16,8,4,2,1  # P0-P4
```

### P0-P4 Mapping (Simplified)

| Level | Use Case |
|-------|----------|
| **P0** | Critical (internal) |
| **P1** | High (premium) |
| **P2** | Normal (default) |
| **P3** | Low (bulk) |
| **P4** | Background |

---

## Alternative: 20 Priority Levels

对于非常细粒度的场景（如多租户 SaaS），可以配置为 **20 个桶**：

```ini
[scheduler]
priority_levels = 20
default_priority = 10  # P10 = Normal
```

**Use Case:**
- P0-P4: Internal/critical
- P5-P9: Enterprise tiers (5 sub-levels)
- P10-P14: Standard tiers (5 sub-levels)
- P15-P19: Free/background tiers (5 sub-levels)

---

## Implementation

### Code Example

```go
// internal/scheduler/priority_queue.go

type SchedulerConfig struct {
    NumPriorityLevels int          // Number of priority buckets
    DefaultPriority   PriorityTier // Default priority
    Weights           []int        // WFQ weights per level
}

func NewScheduler(config *SchedulerConfig) *Scheduler {
    if config == nil {
        config = &SchedulerConfig{
            NumPriorityLevels: 10,
            DefaultPriority:   5,
            Weights:           []int{256,128,64,32,16,8,4,2,1,1},
        }
    }

    // Create dynamic number of queues
    queues := make([]*Queue, config.NumPriorityLevels)
    for i := 0; i < config.NumPriorityLevels; i++ {
        queues[i] = NewQueue()
    }

    return &Scheduler{
        queues: queues,
        config: config,
    }
}

func (s *Scheduler) Enqueue(req *Request) error {
    priority := req.Priority
    if priority < 0 {
        priority = s.config.DefaultPriority
    }

    // Validate priority range
    if priority >= PriorityTier(s.config.NumPriorityLevels) {
        return fmt.Errorf("invalid priority %d (max: %d)",
            priority, s.config.NumPriorityLevels-1)
    }

    s.queues[priority].Enqueue(req)
    return nil
}
```

### Config Parsing

```go
// internal/config/scheduler.go

func LoadSchedulerConfig(cfg *ini.File) (*SchedulerConfig, error) {
    section := cfg.Section("scheduler")

    numLevels := section.Key("priority_levels").MustInt(10)
    defaultPriority := section.Key("default_priority").MustInt(5)

    // Parse weights
    weightsStr := section.Key("priority_weights").String()
    weights := parseWeights(weightsStr, numLevels)

    return &SchedulerConfig{
        NumPriorityLevels: numLevels,
        DefaultPriority:   PriorityTier(defaultPriority),
        Weights:           weights,
    }, nil
}
```

---

## Performance Considerations

### Tradeoffs

| Levels | Pros | Cons |
|--------|------|------|
| **5** | Simple, fast scheduling | Coarse granularity |
| **10** | Good balance (default) | Balanced |
| **20** | Fine-grained control | Higher overhead |
| **50+** | ❌ Not recommended | Too much complexity |

### Recommendation

- **Start with 10** (default)
- **Monitor queue depths** per level
- **Adjust if needed**:
  - Most queues empty → Reduce levels
  - Many requests in same queue → Increase levels

---

## Migration Path

### From 5 to 10 Levels

If you have existing priority mappings:

```go
// Old (5 levels)
const (
    PriorityCritical   = 0
    PriorityHigh       = 1
    PriorityNormal     = 2
    PriorityLow        = 3
    PriorityBackground = 4
)

// New (10 levels) - backward compatible
const (
    PriorityCritical   = 0  // Still P0
    PriorityHigh       = 2  // P1 -> P2 (more granular)
    PriorityNormal     = 5  // P2 -> P5 (middle)
    PriorityLow        = 7  // P3 -> P7
    PriorityBackground = 9  // P4 -> P9
)

// Auto-migration
func migrateOldPriority(oldPriority int, oldLevels, newLevels int) int {
    // Scale: oldPriority * (newLevels / oldLevels)
    return (oldPriority * newLevels) / oldLevels
}

// Example: P2 (Normal) in 5-level -> P4 in 10-level
// 2 * 10 / 5 = 4
```

---

## Monitoring

### Metrics to Track

```
# Queue depth per level
tokligence_scheduler_queue_depth{priority="0"} 5
tokligence_scheduler_queue_depth{priority="1"} 12
tokligence_scheduler_queue_depth{priority="5"} 234  # Most traffic
tokligence_scheduler_queue_depth{priority="9"} 3

# Wait time percentiles per level
tokligence_scheduler_wait_p50_ms{priority="0"} 2
tokligence_scheduler_wait_p50_ms{priority="5"} 150
tokligence_scheduler_wait_p99_ms{priority="9"} 5000

# Utilization per level
tokligence_scheduler_utilization{priority="0"} 0.05
tokligence_scheduler_utilization{priority="5"} 0.78  # High utilization
tokligence_scheduler_utilization{priority="9"} 0.12
```

### Dashboard Example

```
Priority Queue Health:
├─ P0 (Critical):   ▓░░░░░░░░░  5%  (2ms P50, 10ms P99)
├─ P1 (Urgent):     ▓▓░░░░░░░░ 12%  (8ms P50, 25ms P99)
├─ P2 (High):       ▓▓▓░░░░░░░ 18%  (15ms P50, 50ms P99)
├─ P5 (Normal):     ▓▓▓▓▓▓▓▓░░ 78%  (150ms P50, 500ms P99) ⚠️ High
└─ P9 (Background): ▓░░░░░░░░░ 12%  (2s P50, 10s P99)

Recommendation: Consider adding P4 or P6 to split P5 traffic
```

---

## Summary

✅ **Default: 10 priority levels (P0-P9)**
✅ **Configurable via**: `TOKLIGENCE_SCHEDULER_PRIORITY_LEVELS`
✅ **Recommended range**: 5-20 levels
✅ **Migration**: Backward compatible with scaling formula

**Next Steps:**
1. Implement dynamic queue allocation
2. Add config parsing
3. Add priority validation
4. Add monitoring metrics
