# Phase 3: Time-Based Dynamic Rules

## Overview

**Objective**: Enable automatic adjustment of scheduler behavior (quotas, weights, priorities) based on time of day to support multi-tenant scenarios where resource allocation needs differ during business hours vs off-hours.

**Use Case**: E-commerce company with GPU cluster:
- **Daytime (08:00-18:00)**: Internal departments prioritized (80% capacity), external customers limited (20%)
- **Nighttime (18:00-08:00)**: External customers prioritized (80% capacity), internal departments best-effort (20%)

**Dependencies**:
- Phase 1: API Key Priority Mapping (completed)
- Phase 2: Per-Account Quota Management (completed)

**Estimated Effort**: 8-10 hours

---

## Design Principles

1. **Non-Invasive**: Time-based rules modify existing scheduler/quota parameters without changing core architecture
2. **Declarative Configuration**: Rules defined in configuration file, not hardcoded
3. **Smooth Transitions**: Avoid abrupt changes that could disrupt in-flight requests
4. **Observable**: Log all rule activations and parameter changes
5. **Testable**: Time can be mocked for testing different schedules

---

## Architecture

### Component Structure

```
┌─────────────────────────────────────────────────────────────┐
│                    TimeBasedRuleEngine                       │
│  - Load rules from config                                    │
│  - Evaluate current time against rule schedules              │
│  - Apply parameter changes to scheduler/quota manager        │
└──────────────────┬──────────────────────────────────────────┘
                   │
      ┌────────────┼────────────┐
      │            │            │
      ▼            ▼            ▼
┌──────────┐  ┌──────────┐  ┌──────────────────┐
│Scheduler │  │  Quota   │  │  API Key Mapper  │
│ Weights  │  │ Manager  │  │   (Phase 1)      │
│(Phase 0) │  │(Phase 2) │  └──────────────────┘
└──────────┘  └──────────┘
```

### Key Components

#### 1. TimeBasedRuleEngine

**Responsibilities**:
- Parse time-based rules from configuration
- Evaluate current time against rule schedules (cron-like)
- Apply parameter changes to target components
- Log all rule activations

**Interface**:
```go
type RuleEngine interface {
    Start(ctx context.Context) error
    Stop() error
    ApplyRulesNow() error  // Manual trigger for testing
    GetActiveRules() []RuleStatus
}
```

#### 2. TimeWindow

**Purpose**: Define when a rule is active

**Structure**:
```go
type TimeWindow struct {
    // Time range (24-hour format)
    StartHour   int  // 0-23
    StartMinute int  // 0-59
    EndHour     int  // 0-23
    EndMinute   int  // 0-59

    // Days of week (optional, default: all days)
    DaysOfWeek  []time.Weekday  // Monday=1, Sunday=7

    // Timezone (default: system timezone)
    Timezone    string  // "America/New_York", "Asia/Singapore"
}

func (tw *TimeWindow) IsActive(t time.Time) bool
```

#### 3. Rule Types

**A. Weight Adjustment Rule**

Dynamically adjusts priority weights for WFQ/Hybrid scheduling.

```go
type WeightAdjustmentRule struct {
    Name        string
    Window      TimeWindow
    Weights     []float64  // New weights for P0-P9
    Description string
}
```

**Example**:
- **Daytime**: Boost internal priorities → `[256, 128, 64, 32, 16, 8, 4, 2, 1, 1]`
- **Nighttime**: Flatten priorities → `[32, 32, 32, 32, 32, 64, 64, 64, 32, 16]`

**B. Quota Adjustment Rule**

Adjusts per-account quotas (Phase 2 integration).

```go
type QuotaAdjustmentRule struct {
    Name        string
    Window      TimeWindow
    Adjustments []QuotaAdjustment
    Description string
}

type QuotaAdjustment struct {
    AccountPattern string  // "dept-a-*", "premium-*"
    MaxConcurrent  *int    // nil = no change
    MaxRPS         *int
    MaxTokensPerSec *int64
}
```

**Example**:
- **Daytime**:
  - Internal dept-a: MaxConcurrent=50
  - External premium: MaxConcurrent=10
- **Nighttime**:
  - Internal dept-a: MaxConcurrent=15
  - External premium: MaxConcurrent=40

**C. Global Capacity Rule**

Adjusts scheduler's global capacity limits.

```go
type CapacityAdjustmentRule struct {
    Name             string
    Window           TimeWindow
    MaxConcurrent    *int
    MaxRPS           *int
    MaxTokensPerSec  *int64
    Description      string
}
```

**Example**:
- **Peak hours (12:00-14:00)**: Increase MaxConcurrent=200
- **Off-peak (02:00-06:00)**: Reduce MaxConcurrent=50 to save resources

---

## Configuration Format

### File: `config/scheduler_time_rules.ini`

```ini
[time_rules]
# Enable time-based rule engine
enabled = true

# Evaluation interval (seconds) - how often to check if rules should apply
check_interval_sec = 60

# Timezone for all time windows (default: system timezone)
default_timezone = Asia/Singapore

# ============================================================================
# Weight Adjustment Rules
# ============================================================================

[rule.weights.daytime]
type = weight_adjustment
name = Internal Priority (Daytime)
description = Boost internal department priorities during business hours

# Time window
start_time = 08:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri

# Weights for P0-P9 (internal=P0-P3, external=P5-P9)
weights = 256,128,64,32,16,8,4,2,1,1

[rule.weights.nighttime]
type = weight_adjustment
name = External Priority (Nighttime)
description = Flatten priorities to favor external customers at night

start_time = 18:00
end_time = 08:00  # Wraps to next day

# Flatten weights to give external customers more share
weights = 32,32,32,32,32,64,64,64,32,16

# ============================================================================
# Quota Adjustment Rules
# ============================================================================

[rule.quota.daytime]
type = quota_adjustment
name = Daytime Quotas
description = Reserve most capacity for internal departments

start_time = 08:00
end_time = 18:00
days_of_week = Mon,Tue,Wed,Thu,Fri

# Internal departments (generous quotas)
quota.dept-a-* = concurrent:50,rps:100,tokens_per_sec:2000
quota.dept-b-* = concurrent:40,rps:80,tokens_per_sec:1500
quota.dept-c-* = concurrent:30,rps:60,tokens_per_sec:1000

# External customers (limited quotas)
quota.premium-* = concurrent:10,rps:20,tokens_per_sec:300
quota.standard-* = concurrent:5,rps:10,tokens_per_sec:150
quota.free-* = concurrent:2,rps:5,tokens_per_sec:50

[rule.quota.nighttime]
type = quota_adjustment
name = Nighttime Quotas
description = Release capacity to external customers

start_time = 18:00
end_time = 08:00

# Internal departments (reduced quotas)
quota.dept-a-* = concurrent:15,rps:30,tokens_per_sec:500
quota.dept-b-* = concurrent:10,rps:20,tokens_per_sec:300
quota.dept-c-* = concurrent:10,rps:20,tokens_per_sec:300

# External customers (expanded quotas)
quota.premium-* = concurrent:40,rps:80,tokens_per_sec:1500
quota.standard-* = concurrent:25,rps:50,tokens_per_sec:800
quota.free-* = concurrent:10,rps:20,tokens_per_sec:200

# ============================================================================
# Global Capacity Rules
# ============================================================================

[rule.capacity.peak_hours]
type = capacity_adjustment
name = Lunchtime Peak Capacity
description = Increase capacity during peak lunch hours

start_time = 12:00
end_time = 14:00
days_of_week = Mon,Tue,Wed,Thu,Fri

max_concurrent = 200
max_rps = 400
max_tokens_per_sec = 8000

[rule.capacity.maintenance_window]
type = capacity_adjustment
name = Maintenance Window
description = Reduce capacity during maintenance window

start_time = 02:00
end_time = 04:00
days_of_week = Sun

max_concurrent = 20
max_rps = 40
max_tokens_per_sec = 1000
```

---

## Implementation Details

### 1. TimeBasedRuleEngine

**Location**: `internal/scheduler/time_rules.go`

```go
package scheduler

import (
    "context"
    "sync"
    "time"
)

type RuleEngine struct {
    config          *TimeRulesConfig
    scheduler       *ChannelScheduler
    quotaManager    *AccountQuotaManager  // Phase 2

    // Parsed rules
    weightRules     []WeightAdjustmentRule
    quotaRules      []QuotaAdjustmentRule
    capacityRules   []CapacityAdjustmentRule

    // State
    activeRules     map[string]bool
    mu              sync.RWMutex

    // Control
    ticker          *time.Ticker
    stopChan        chan struct{}

    // Time override (for testing)
    timeOverride    *time.Time
}

func NewRuleEngine(
    config *TimeRulesConfig,
    scheduler *ChannelScheduler,
    quotaManager *AccountQuotaManager,
) (*RuleEngine, error) {
    re := &RuleEngine{
        config:       config,
        scheduler:    scheduler,
        quotaManager: quotaManager,
        activeRules:  make(map[string]bool),
        stopChan:     make(chan struct{}),
    }

    if err := re.loadRules(); err != nil {
        return nil, fmt.Errorf("failed to load rules: %w", err)
    }

    return re, nil
}

func (re *RuleEngine) Start(ctx context.Context) error {
    if !re.config.Enabled {
        log.Printf("[INFO] TimeBasedRuleEngine: Disabled")
        return nil
    }

    interval := time.Duration(re.config.CheckIntervalSec) * time.Second
    re.ticker = time.NewTicker(interval)

    log.Printf("[INFO] TimeBasedRuleEngine: Started (check_interval=%s, timezone=%s)",
        interval, re.config.DefaultTimezone)

    // Apply rules immediately on startup
    if err := re.ApplyRulesNow(); err != nil {
        log.Printf("[WARN] TimeBasedRuleEngine: Initial rule application failed: %v", err)
    }

    go re.evaluationLoop(ctx)
    return nil
}

func (re *RuleEngine) evaluationLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-re.stopChan:
            return
        case <-re.ticker.C:
            if err := re.ApplyRulesNow(); err != nil {
                log.Printf("[ERROR] TimeBasedRuleEngine: Rule application failed: %v", err)
            }
        }
    }
}

func (re *RuleEngine) ApplyRulesNow() error {
    now := re.getCurrentTime()

    log.Printf("[DEBUG] TimeBasedRuleEngine: Evaluating rules at %s", now.Format("15:04:05"))

    // Apply weight adjustment rules
    for _, rule := range re.weightRules {
        if rule.Window.IsActive(now) {
            if !re.isRuleActive(rule.Name) {
                log.Printf("[INFO] TimeBasedRuleEngine: Activating rule '%s'", rule.Name)
                re.applyWeightAdjustment(&rule)
                re.markRuleActive(rule.Name, true)
            }
        } else {
            if re.isRuleActive(rule.Name) {
                log.Printf("[INFO] TimeBasedRuleEngine: Deactivating rule '%s'", rule.Name)
                re.markRuleActive(rule.Name, false)
            }
        }
    }

    // Apply quota adjustment rules
    for _, rule := range re.quotaRules {
        if rule.Window.IsActive(now) {
            if !re.isRuleActive(rule.Name) {
                log.Printf("[INFO] TimeBasedRuleEngine: Activating rule '%s'", rule.Name)
                re.applyQuotaAdjustment(&rule)
                re.markRuleActive(rule.Name, true)
            }
        } else {
            if re.isRuleActive(rule.Name) {
                re.markRuleActive(rule.Name, false)
            }
        }
    }

    // Apply capacity adjustment rules
    for _, rule := range re.capacityRules {
        if rule.Window.IsActive(now) {
            if !re.isRuleActive(rule.Name) {
                log.Printf("[INFO] TimeBasedRuleEngine: Activating rule '%s'", rule.Name)
                re.applyCapacityAdjustment(&rule)
                re.markRuleActive(rule.Name, true)
            }
        } else {
            if re.isRuleActive(rule.Name) {
                re.markRuleActive(rule.Name, false)
            }
        }
    }

    return nil
}

func (re *RuleEngine) applyWeightAdjustment(rule *WeightAdjustmentRule) {
    re.scheduler.AdjustWeights(rule.Weights)
    log.Printf("[INFO] TimeBasedRuleEngine: Applied weight adjustment '%s': %v",
        rule.Name, rule.Weights)
}

func (re *RuleEngine) applyQuotaAdjustment(rule *QuotaAdjustmentRule) {
    for _, adj := range rule.Adjustments {
        accounts := re.quotaManager.FindAccountsByPattern(adj.AccountPattern)
        for _, accountID := range accounts {
            if adj.MaxConcurrent != nil {
                re.quotaManager.UpdateQuota(accountID, "max_concurrent", *adj.MaxConcurrent)
            }
            if adj.MaxRPS != nil {
                re.quotaManager.UpdateQuota(accountID, "max_rps", *adj.MaxRPS)
            }
            if adj.MaxTokensPerSec != nil {
                re.quotaManager.UpdateQuota(accountID, "max_tokens_per_sec", *adj.MaxTokensPerSec)
            }
        }
        log.Printf("[INFO] TimeBasedRuleEngine: Applied quota adjustment for pattern '%s': %d accounts",
            adj.AccountPattern, len(accounts))
    }
}

func (re *RuleEngine) applyCapacityAdjustment(rule *CapacityAdjustmentRule) {
    if rule.MaxConcurrent != nil {
        re.scheduler.AdjustGlobalCapacity("max_concurrent", *rule.MaxConcurrent)
    }
    if rule.MaxRPS != nil {
        re.scheduler.AdjustGlobalCapacity("max_rps", *rule.MaxRPS)
    }
    if rule.MaxTokensPerSec != nil {
        re.scheduler.AdjustGlobalCapacity("max_tokens_per_sec", *rule.MaxTokensPerSec)
    }
    log.Printf("[INFO] TimeBasedRuleEngine: Applied capacity adjustment '%s'", rule.Name)
}

func (re *RuleEngine) getCurrentTime() time.Time {
    if re.timeOverride != nil {
        return *re.timeOverride
    }
    return time.Now()
}

// For testing
func (re *RuleEngine) SetTimeOverride(t time.Time) {
    re.timeOverride = &t
}

func (re *RuleEngine) ClearTimeOverride() {
    re.timeOverride = nil
}
```

### 2. ChannelScheduler Enhancements

**Location**: `internal/scheduler/scheduler_channel.go`

Add dynamic adjustment methods:

```go
func (cs *ChannelScheduler) AdjustWeights(newWeights []float64) error {
    cs.wfqMu.Lock()
    defer cs.wfqMu.Unlock()

    if len(newWeights) != cs.config.NumPriorityLevels {
        return fmt.Errorf("weight count mismatch: expected %d, got %d",
            cs.config.NumPriorityLevels, len(newWeights))
    }

    oldWeights := cs.config.Weights
    cs.config.Weights = newWeights

    log.Printf("[INFO] ChannelScheduler: Weights adjusted: %v → %v", oldWeights, newWeights)
    return nil
}

func (cs *ChannelScheduler) AdjustGlobalCapacity(param string, value interface{}) error {
    cs.capacityMu.Lock()
    defer cs.capacityMu.Unlock()

    switch param {
    case "max_concurrent":
        oldVal := cs.capacity.MaxConcurrent
        cs.capacity.MaxConcurrent = value.(int)
        log.Printf("[INFO] ChannelScheduler: MaxConcurrent adjusted: %d → %d",
            oldVal, cs.capacity.MaxConcurrent)
    case "max_rps":
        oldVal := cs.capacity.MaxRPS
        cs.capacity.MaxRPS = int64(value.(int))
        log.Printf("[INFO] ChannelScheduler: MaxRPS adjusted: %d → %d",
            oldVal, cs.capacity.MaxRPS)
    case "max_tokens_per_sec":
        oldVal := cs.capacity.MaxTokensPerSec
        cs.capacity.MaxTokensPerSec = value.(int64)
        log.Printf("[INFO] ChannelScheduler: MaxTokensPerSec adjusted: %d → %d",
            oldVal, cs.capacity.MaxTokensPerSec)
    default:
        return fmt.Errorf("unknown capacity parameter: %s", param)
    }

    return nil
}
```

### 3. TimeWindow Implementation

**Location**: `internal/scheduler/time_window.go`

```go
package scheduler

import (
    "fmt"
    "time"
)

type TimeWindow struct {
    StartHour   int
    StartMinute int
    EndHour     int
    EndMinute   int
    DaysOfWeek  []time.Weekday
    Timezone    *time.Location
}

func (tw *TimeWindow) IsActive(t time.Time) bool {
    // Convert to window's timezone
    t = t.In(tw.Timezone)

    // Check day of week
    if len(tw.DaysOfWeek) > 0 {
        dayMatch := false
        for _, day := range tw.DaysOfWeek {
            if t.Weekday() == day {
                dayMatch = true
                break
            }
        }
        if !dayMatch {
            return false
        }
    }

    // Check time range
    currentMinutes := t.Hour()*60 + t.Minute()
    startMinutes := tw.StartHour*60 + tw.StartMinute
    endMinutes := tw.EndHour*60 + tw.EndMinute

    // Handle wrap-around (e.g., 22:00 - 06:00)
    if endMinutes < startMinutes {
        // Active if current time is after start OR before end
        return currentMinutes >= startMinutes || currentMinutes < endMinutes
    }

    // Normal case (e.g., 08:00 - 18:00)
    return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

func (tw *TimeWindow) String() string {
    daysStr := "all days"
    if len(tw.DaysOfWeek) > 0 {
        daysStr = fmt.Sprintf("%v", tw.DaysOfWeek)
    }

    return fmt.Sprintf("%02d:%02d-%02d:%02d (%s, %s)",
        tw.StartHour, tw.StartMinute,
        tw.EndHour, tw.EndMinute,
        daysStr, tw.Timezone)
}
```

---

## Integration with Existing Phases

### Integration with Phase 1 (API Key Priority Mapping)

Time-based rules can dynamically adjust **which accounts get which priorities**:

```ini
[rule.priority_mapping.daytime]
type = priority_mapping_override
name = Daytime Priority Overrides

start_time = 08:00
end_time = 18:00

# During daytime, boost internal departments even further
priority_override.dept-a-* = 0  # Force to P0
priority_override.dept-b-* = 1  # Force to P1
priority_override.external-* = 8  # Demote external to P8
```

### Integration with Phase 2 (Per-Account Quota)

Time-based rules adjust quota values as shown in configuration examples above.

**Key Point**: Rule engine calls `AccountQuotaManager.UpdateQuota()` to modify quotas.

### Integration with Phase 0 (Scheduler)

Time-based rules adjust:
- Scheduler weights (WFQ)
- Global capacity limits
- Priority levels (indirectly via Phase 1 integration)

---

## HTTP API Extensions

### 1. Get Active Rules

**Endpoint**: `GET /admin/scheduler/time-rules/active`

**Response**:
```json
{
  "engine_enabled": true,
  "check_interval_sec": 60,
  "timezone": "Asia/Singapore",
  "current_time": "2025-01-23T14:30:00+08:00",
  "active_rules": [
    {
      "name": "Internal Priority (Daytime)",
      "type": "weight_adjustment",
      "description": "Boost internal department priorities during business hours",
      "window": {
        "start_time": "08:00",
        "end_time": "18:00",
        "days_of_week": ["Mon", "Tue", "Wed", "Thu", "Fri"]
      },
      "active_since": "2025-01-23T08:00:00+08:00"
    },
    {
      "name": "Daytime Quotas",
      "type": "quota_adjustment",
      "active_since": "2025-01-23T08:00:00+08:00"
    }
  ],
  "inactive_rules": [
    {
      "name": "External Priority (Nighttime)",
      "type": "weight_adjustment",
      "next_activation": "2025-01-23T18:00:00+08:00"
    }
  ]
}
```

### 2. Manual Rule Trigger

**Endpoint**: `POST /admin/scheduler/time-rules/apply`

**Purpose**: Manually trigger rule evaluation (for testing or emergency adjustments)

**Response**:
```json
{
  "success": true,
  "rules_applied": 2,
  "timestamp": "2025-01-23T14:30:00+08:00"
}
```

### 3. Rule Override

**Endpoint**: `POST /admin/scheduler/time-rules/override`

**Body**:
```json
{
  "override_time": "2025-01-23T22:00:00+08:00",
  "duration_minutes": 60
}
```

**Purpose**: Temporarily override system time for testing (e.g., simulate nighttime rules during daytime)

---

## Testing Strategy

### 1. Unit Tests

**Location**: `internal/scheduler/time_rules_test.go`

```go
func TestTimeWindow_IsActive(t *testing.T) {
    tests := []struct {
        name     string
        window   TimeWindow
        testTime time.Time
        expected bool
    }{
        {
            name: "daytime_active",
            window: TimeWindow{StartHour: 8, EndHour: 18},
            testTime: time.Date(2025, 1, 23, 14, 0, 0, 0, time.Local),
            expected: true,
        },
        {
            name: "nighttime_inactive",
            window: TimeWindow{StartHour: 8, EndHour: 18},
            testTime: time.Date(2025, 1, 23, 22, 0, 0, 0, time.Local),
            expected: false,
        },
        {
            name: "wrap_around_active",
            window: TimeWindow{StartHour: 22, EndHour: 6},
            testTime: time.Date(2025, 1, 23, 2, 0, 0, 0, time.Local),
            expected: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            active := tt.window.IsActive(tt.testTime)
            if active != tt.expected {
                t.Errorf("Expected %v, got %v", tt.expected, active)
            }
        })
    }
}

func TestRuleEngine_WeightAdjustment(t *testing.T) {
    scheduler := NewChannelScheduler(...)
    engine := NewRuleEngine(config, scheduler, nil)

    // Override time to daytime
    engine.SetTimeOverride(time.Date(2025, 1, 23, 14, 0, 0, 0, time.Local))

    // Apply rules
    err := engine.ApplyRulesNow()
    if err != nil {
        t.Fatalf("ApplyRulesNow failed: %v", err)
    }

    // Verify weights changed
    weights := scheduler.GetCurrentWeights()
    expectedDaytimeWeights := []float64{256, 128, 64, 32, 16, 8, 4, 2, 1, 1}
    if !reflect.DeepEqual(weights, expectedDaytimeWeights) {
        t.Errorf("Expected weights %v, got %v", expectedDaytimeWeights, weights)
    }

    // Override time to nighttime
    engine.SetTimeOverride(time.Date(2025, 1, 23, 22, 0, 0, 0, time.Local))

    // Apply rules again
    err = engine.ApplyRulesNow()
    if err != nil {
        t.Fatalf("ApplyRulesNow failed: %v", err)
    }

    // Verify weights changed to nighttime values
    weights = scheduler.GetCurrentWeights()
    expectedNighttimeWeights := []float64{32, 32, 32, 32, 32, 64, 64, 64, 32, 16}
    if !reflect.DeepEqual(weights, expectedNighttimeWeights) {
        t.Errorf("Expected weights %v, got %v", expectedNighttimeWeights, weights)
    }
}
```

### 2. Integration Tests

**Location**: `tests/integration/scheduler/test_time_based_rules.sh`

```bash
#!/bin/bash

# Test daytime rule activation
curl -X POST http://localhost:8081/admin/scheduler/time-rules/override \
  -d '{"override_time": "2025-01-23T14:00:00+08:00", "duration_minutes": 5}'

# Check active rules
curl -s http://localhost:8081/admin/scheduler/time-rules/active | jq '
  .active_rules[] | select(.name == "Internal Priority (Daytime)")
'

# Submit internal department request (should get P0)
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-dept-a-test" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}]}'

# Check scheduler stats (P0 should be used)
curl -s http://localhost:8081/admin/scheduler/stats | jq '
  .queue_stats[] | select(.priority == 0) | .current_depth
'

# Test nighttime rule activation
curl -X POST http://localhost:8081/admin/scheduler/time-rules/override \
  -d '{"override_time": "2025-01-23T22:00:00+08:00", "duration_minutes": 5}'

# Check active rules (should show nighttime rules)
curl -s http://localhost:8081/admin/scheduler/time-rules/active | jq '
  .active_rules[] | select(.name == "External Priority (Nighttime)")
'

# Submit external customer request (should get better quota)
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-premium-test" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "Test"}]}'

# Check quota status
curl -s http://localhost:8081/admin/scheduler/quota/sk-premium-test
```

### 3. Manual Testing

**Test Scenario**: Day-Night Transition

1. Start gateway with time rules enabled
2. Monitor logs for rule activations:
   ```bash
   tail -f logs/gateway.log | grep "TimeBasedRuleEngine"
   ```
3. Use time override to simulate time transitions
4. Verify weights/quotas change as expected
5. Submit requests and verify priority/quota behavior

---

## Performance Considerations

### 1. Evaluation Frequency

- Default check interval: **60 seconds**
- Trade-off: More frequent = more responsive, but more CPU overhead
- Recommendation: 60s is sufficient for most use cases (time-of-day changes are gradual)

### 2. Rule Application Overhead

- Weight adjustment: **~100μs** (atomic pointer swap)
- Quota adjustment: **~1ms per account** (atomic int updates)
- Capacity adjustment: **~100μs** (atomic int updates)

**Total overhead for typical setup (50 accounts, 3 rules)**: ~60ms every 60s = **0.1% CPU impact**

### 3. Atomic Updates

All parameter changes use atomic operations or mutex-protected updates to avoid race conditions:

```go
// Example: Weight adjustment
cs.wfqMu.Lock()
cs.config.Weights = newWeights  // Protected by mutex
cs.wfqMu.Unlock()

// Example: Quota adjustment
quota.currentConcurrent.Store(newValue)  // Atomic operation
```

### 4. No Request Interruption

Rule changes only affect **new requests**. In-flight requests continue with old parameters.

---

## Migration Path

### From Phase 0 (Current Scheduler)

**No breaking changes** - time rules are optional:

```ini
[time_rules]
enabled = false  # Disable time rules, scheduler works as before
```

### From Phase 1 + Phase 2

Time rules enhance Phase 1 and Phase 2 without replacing them:

- **Phase 1 (API Key Mapping)**: Static mapping still works, time rules can override
- **Phase 2 (Account Quotas)**: Static quotas still enforced, time rules can adjust

---

## Error Handling

### 1. Configuration Errors

**Scenario**: Invalid time window (e.g., `start_time = 25:00`)

**Handling**:
```go
if tw.StartHour < 0 || tw.StartHour > 23 {
    return fmt.Errorf("invalid start_hour: %d (must be 0-23)", tw.StartHour)
}
```

**Behavior**: Gateway startup fails with clear error message

### 2. Rule Application Errors

**Scenario**: Rule tries to adjust non-existent account quota

**Handling**:
```go
if quota == nil {
    log.Printf("[WARN] TimeBasedRuleEngine: Account '%s' not found, skipping quota adjustment", accountID)
    continue  // Skip this account, continue with others
}
```

**Behavior**: Log warning, continue applying other rules

### 3. Conflicting Rules

**Scenario**: Two rules active at same time, both try to adjust same parameter

**Handling**: **Last rule wins** (rules evaluated in configuration order)

**Alternative** (future enhancement): Rule priority system

---

## Monitoring and Observability

### 1. Logs

**Rule Activation**:
```
[INFO] TimeBasedRuleEngine: Activating rule 'Internal Priority (Daytime)'
[INFO] TimeBasedRuleEngine: Applied weight adjustment 'Internal Priority (Daytime)': [256 128 64 32 16 8 4 2 1 1]
```

**Rule Deactivation**:
```
[INFO] TimeBasedRuleEngine: Deactivating rule 'Internal Priority (Daytime)'
```

**Parameter Changes**:
```
[INFO] ChannelScheduler: Weights adjusted: [256 128 64 32 16 8 4 2 1 1] → [32 32 32 32 32 64 64 64 32 16]
[INFO] AccountQuotaManager: Quota updated for 'dept-a-*': max_concurrent 50 → 15
```

### 2. Metrics (Future)

Potential Prometheus metrics:
- `time_rule_activations_total{rule_name}`
- `time_rule_evaluation_duration_seconds`
- `active_time_rules_count`
- `weight_adjustment_count`
- `quota_adjustment_count`

### 3. HTTP API

Real-time visibility via `/admin/scheduler/time-rules/active` endpoint

---

## Security Considerations

### 1. Configuration File Access

**Risk**: Unauthorized modification of time rules could disrupt service

**Mitigation**:
- Config file should be read-only for gateway process
- Only admins should have write access
- Changes require gateway restart (no hot-reload initially)

### 2. Time Override API

**Risk**: Time override endpoint could be abused to trigger unintended rule activations

**Mitigation**:
- Require admin authentication (Phase 4: Auth enhancement)
- Log all time override requests
- Auto-expire time overrides after duration
- Initially: Only available in dev/test mode

### 3. Rule Validation

**Risk**: Malicious rules could set quotas to 0, causing DoS

**Mitigation**:
- Validate rule parameters at load time:
  ```go
  if adj.MaxConcurrent != nil && *adj.MaxConcurrent < 1 {
      return fmt.Errorf("invalid max_concurrent: %d (must be >= 1)", *adj.MaxConcurrent)
  }
  ```

---

## Implementation Steps

### Step 1: Core Time Window (2 hours)

1. Create `internal/scheduler/time_window.go`
2. Implement `TimeWindow` struct with `IsActive()` method
3. Add unit tests for time range logic (including wrap-around)
4. Add timezone support

### Step 2: Rule Engine Structure (2 hours)

1. Create `internal/scheduler/time_rules.go`
2. Implement `RuleEngine` struct with evaluation loop
3. Add rule loading from configuration
4. Implement time override for testing

### Step 3: Scheduler Integration (1.5 hours)

1. Add `AdjustWeights()` method to `ChannelScheduler`
2. Add `AdjustGlobalCapacity()` method to `ChannelScheduler`
3. Add `GetCurrentWeights()` method for testing
4. Add thread-safety for parameter updates

### Step 4: Quota Manager Integration (1.5 hours)

1. Add `FindAccountsByPattern()` to `AccountQuotaManager` (Phase 2)
2. Add `UpdateQuota()` method to `AccountQuotaManager`
3. Ensure atomic quota updates
4. Add logging for quota changes

### Step 5: Configuration Parser (1.5 hours)

1. Update `internal/config/config.go` to parse time rules
2. Implement INI parsing for time windows
3. Implement INI parsing for rule types
4. Add configuration validation

### Step 6: HTTP API (1 hour)

1. Add `/admin/scheduler/time-rules/active` endpoint
2. Add `/admin/scheduler/time-rules/apply` endpoint
3. Add `/admin/scheduler/time-rules/override` endpoint (dev mode only)
4. Add JSON response formatting

### Step 7: Testing (1.5 hours)

1. Unit tests for `TimeWindow`
2. Unit tests for `RuleEngine`
3. Integration tests with time override
4. End-to-end test with live gateway

### Step 8: Documentation (1 hour)

1. Update `config/scheduler_time_rules.ini` with examples
2. Update testing guide with time rule scenarios
3. Add troubleshooting guide
4. Update main README

---

## Estimated Timeline

| Task | Estimated Time |
|------|---------------|
| Core Time Window | 2 hours |
| Rule Engine Structure | 2 hours |
| Scheduler Integration | 1.5 hours |
| Quota Manager Integration | 1.5 hours |
| Configuration Parser | 1.5 hours |
| HTTP API | 1 hour |
| Testing | 1.5 hours |
| Documentation | 1 hour |
| **Total** | **~12 hours** |

**Note**: Revised from initial 8-10 hour estimate after detailed design; 12 hours is more realistic.

---

## Success Criteria

1. ✅ **Functional**:
   - Weight adjustments take effect within check interval
   - Quota adjustments apply to correct accounts
   - Capacity adjustments affect global limits
   - Time window logic handles wrap-around correctly

2. ✅ **Performance**:
   - Rule evaluation overhead < 0.2% CPU
   - No impact on request latency
   - Atomic updates prevent race conditions

3. ✅ **Observable**:
   - All rule activations logged
   - HTTP API shows active rules
   - Parameter changes visible in logs

4. ✅ **Testable**:
   - Time can be overridden for testing
   - Rules can be manually triggered
   - All rule types covered by unit tests

5. ✅ **Production-Ready**:
   - Configuration validation prevents invalid rules
   - Errors don't crash gateway
   - Rules can be disabled without code changes

---

## Future Enhancements (Phase 4+)

1. **Rule Priority System**: Handle conflicting rules explicitly
2. **Gradual Transitions**: Smooth weight/quota changes over time instead of instant
3. **Dynamic Rule Loading**: Hot-reload rules without gateway restart
4. **Rule Templates**: Pre-defined rule sets for common scenarios
5. **Metrics Dashboard**: Grafana dashboard showing rule activations and effects
6. **Alert Integration**: Notify when rules activate/deactivate
7. **Machine Learning**: Auto-adjust weights based on historical traffic patterns

---

## Summary

Phase 3 adds **time-based dynamic rules** to automatically adjust scheduler behavior throughout the day:

**Key Features**:
- Weight adjustment rules (boost/flatten priorities)
- Quota adjustment rules (expand/contract account limits)
- Capacity adjustment rules (scale global limits)
- Declarative INI configuration
- HTTP API for real-time visibility
- Time override for testing

**Integration**:
- Builds on Phase 1 (API key mapping)
- Builds on Phase 2 (account quotas)
- Non-invasive to Phase 0 (scheduler core)

**Effort**: ~12 hours implementation + testing

**Value**: Enables multi-tenant GPU sharing scenarios with automatic day/night resource allocation without manual intervention.
