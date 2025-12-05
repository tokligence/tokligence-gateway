# Phase 2: Per-Account Quota Management Design

**Branch**: `feat/scheduler-phase2-account-quota`
**Base**: `feat/scheduler-phase1-api-key-mapping`
**估算工作量**: 6-8小时
**依赖**: Phase 1 (API Key Mapping)

## 目标

**核心需求**: 基于Account/API Key的独立配额管理，限制每个部门/客户的资源使用。

**使用场景**:
- 限制每个部门的最大并发请求数
- 限制每个客户的QPS (Queries Per Second)
- 限制每个账户的tokens/sec
- 防止单个账户耗尽所有资源

**与Priority Queue的关系**: ✅ **正交设计**
- Priority Queue: 决定**处理顺序**（谁先执行）
- Account Quota: 决定**资源上限**（谁能用多少）
- 两者独立工作，互不干扰

## 架构设计

### 1. 核心概念

```
                   Global Capacity (100 concurrent)
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
    Dept A Quota      Dept B Quota     External Quota
    (Max 30)          (Max 25)         (Max 45)
         │                 │                 │
    ┌────┴────┐       ┌────┴────┐      ┌────┴────┐
   P0   P1   P2      P2   P3      P5   P7   P9
  (优先级)          (优先级)      (优先级)

Account Quota (资源上限) + Priority (处理顺序) = 完整的多租户控制
```

### 2. 数据结构

#### 2.1 Account Quota Definition

```go
// internal/scheduler/account_quota.go

package scheduler

import (
    "sync"
    "sync/atomic"
    "time"
)

// QuotaDimension represents different quota measurement dimensions
type QuotaDimension string

const (
    QuotaDimConcurrent   QuotaDimension = "concurrent"    // Max concurrent requests
    QuotaDimRPS          QuotaDimension = "rps"           // Requests per second
    QuotaDimTokensPerSec QuotaDimension = "tokens_per_sec" // Tokens per second
    QuotaDimRequestsPerDay QuotaDimension = "requests_per_day" // Daily request limit
)

// AccountQuota defines resource limits for a specific account
type AccountQuota struct {
    AccountID string // Account/API key identifier

    // Concurrent request limit
    MaxConcurrent int
    currentConcurrent atomic.Int32

    // Rate limits (per-second window)
    MaxRPS          int
    MaxTokensPerSec int64

    // Window tracking
    windowStart     atomic.Int64 // Unix timestamp
    windowRequests  atomic.Int32
    windowTokens    atomic.Int64

    // Daily limits (optional)
    MaxRequestsPerDay int
    dailyStart        atomic.Int64 // Unix timestamp of day start
    dailyRequests     atomic.Int32

    // Statistics
    totalRequests    atomic.Uint64
    totalTokens      atomic.Uint64
    totalRejections  atomic.Uint64

    // Metadata
    Priority     PriorityTier // Associated priority (from Phase 1)
    Description  string       // Human-readable description
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// NewAccountQuota creates a new account quota
func NewAccountQuota(accountID string, maxConcurrent, maxRPS int, maxTokensPerSec int64) *AccountQuota {
    now := time.Now()
    aq := &AccountQuota{
        AccountID:       accountID,
        MaxConcurrent:   maxConcurrent,
        MaxRPS:          maxRPS,
        MaxTokensPerSec: maxTokensPerSec,
        CreatedAt:       now,
        UpdatedAt:       now,
    }

    aq.windowStart.Store(now.Unix())
    aq.dailyStart.Store(now.Truncate(24 * time.Hour).Unix())

    return aq
}

// CheckAndReserve checks if request can be accepted and reserves resources
func (aq *AccountQuota) CheckAndReserve(estimatedTokens int64) (bool, string) {
    now := time.Now()

    // Reset windows if necessary
    aq.resetWindows(now)

    // Check 1: Concurrent limit
    if aq.MaxConcurrent > 0 {
        current := aq.currentConcurrent.Load()
        if current >= int32(aq.MaxConcurrent) {
            aq.totalRejections.Add(1)
            return false, fmt.Sprintf("account %s concurrent limit exceeded (%d/%d)",
                aq.AccountID, current, aq.MaxConcurrent)
        }
    }

    // Check 2: RPS limit
    if aq.MaxRPS > 0 {
        requests := aq.windowRequests.Load()
        if requests >= int32(aq.MaxRPS) {
            aq.totalRejections.Add(1)
            return false, fmt.Sprintf("account %s RPS limit exceeded (%d/%d)",
                aq.AccountID, requests, aq.MaxRPS)
        }
    }

    // Check 3: Tokens/sec limit
    if aq.MaxTokensPerSec > 0 {
        tokens := aq.windowTokens.Load()
        if tokens+estimatedTokens > aq.MaxTokensPerSec {
            aq.totalRejections.Add(1)
            return false, fmt.Sprintf("account %s tokens/sec limit exceeded (%d+%d > %d)",
                aq.AccountID, tokens, estimatedTokens, aq.MaxTokensPerSec)
        }
    }

    // Check 4: Daily request limit
    if aq.MaxRequestsPerDay > 0 {
        daily := aq.dailyRequests.Load()
        if daily >= int32(aq.MaxRequestsPerDay) {
            aq.totalRejections.Add(1)
            return false, fmt.Sprintf("account %s daily limit exceeded (%d/%d)",
                aq.AccountID, daily, aq.MaxRequestsPerDay)
        }
    }

    // All checks passed - reserve resources
    aq.currentConcurrent.Add(1)
    aq.windowRequests.Add(1)
    aq.windowTokens.Add(estimatedTokens)
    aq.dailyRequests.Add(1)
    aq.totalRequests.Add(1)
    aq.totalTokens.Add(uint64(estimatedTokens))

    return true, ""
}

// Release releases reserved resources after request completes
func (aq *AccountQuota) Release() {
    aq.currentConcurrent.Add(-1)
}

// resetWindows resets rate limiting windows if they've expired
func (aq *AccountQuota) resetWindows(now time.Time) {
    nowUnix := now.Unix()

    // Reset per-second window
    windowStart := aq.windowStart.Load()
    if nowUnix > windowStart {
        if aq.windowStart.CompareAndSwap(windowStart, nowUnix) {
            aq.windowRequests.Store(0)
            aq.windowTokens.Store(0)
        }
    }

    // Reset daily window
    dayStart := time.Unix(aq.dailyStart.Load(), 0)
    if now.Truncate(24 * time.Hour).After(dayStart) {
        newDayStart := now.Truncate(24 * time.Hour).Unix()
        if aq.dailyStart.CompareAndSwap(dayStart.Unix(), newDayStart) {
            aq.dailyRequests.Store(0)
        }
    }
}

// GetStats returns current quota statistics
func (aq *AccountQuota) GetStats() *AccountQuotaStats {
    return &AccountQuotaStats{
        AccountID:         aq.AccountID,
        CurrentConcurrent: int(aq.currentConcurrent.Load()),
        MaxConcurrent:     aq.MaxConcurrent,
        CurrentRPS:        int(aq.windowRequests.Load()),
        MaxRPS:            aq.MaxRPS,
        CurrentTokensPS:   aq.windowTokens.Load(),
        MaxTokensPS:       aq.MaxTokensPerSec,
        DailyRequests:     int(aq.dailyRequests.Load()),
        MaxDailyRequests:  aq.MaxRequestsPerDay,
        TotalRequests:     aq.totalRequests.Load(),
        TotalTokens:       aq.totalTokens.Load(),
        TotalRejections:   aq.totalRejections.Load(),
        Priority:          aq.Priority,
        Description:       aq.Description,
    }
}

type AccountQuotaStats struct {
    AccountID         string
    CurrentConcurrent int
    MaxConcurrent     int
    CurrentRPS        int
    MaxRPS            int
    CurrentTokensPS   int64
    MaxTokensPS       int64
    DailyRequests     int
    MaxDailyRequests  int
    TotalRequests     uint64
    TotalTokens       uint64
    TotalRejections   uint64
    Priority          PriorityTier
    Description       string
}
```

#### 2.2 Account Quota Manager

```go
// internal/scheduler/account_quota_manager.go

package scheduler

import (
    "sync"
)

// AccountQuotaManager manages quotas for multiple accounts
type AccountQuotaManager struct {
    quotas map[string]*AccountQuota // accountID -> quota
    mu     sync.RWMutex

    // Default quota for accounts without specific config
    defaultQuota *AccountQuota
    enabled      bool
}

// NewAccountQuotaManager creates a new account quota manager
func NewAccountQuotaManager(enabled bool) *AccountQuotaManager {
    return &AccountQuotaManager{
        quotas:  make(map[string]*AccountQuota),
        enabled: enabled,
    }
}

// SetQuota sets or updates quota for an account
func (aqm *AccountQuotaManager) SetQuota(quota *AccountQuota) {
    aqm.mu.Lock()
    defer aqm.mu.Unlock()

    aqm.quotas[quota.AccountID] = quota
    log.Printf("[INFO] AccountQuotaManager: Set quota for %s (concurrent=%d, rps=%d)",
        quota.AccountID, quota.MaxConcurrent, quota.MaxRPS)
}

// SetDefaultQuota sets the default quota for unmapped accounts
func (aqm *AccountQuotaManager) SetDefaultQuota(quota *AccountQuota) {
    aqm.mu.Lock()
    defer aqm.mu.Unlock()

    aqm.defaultQuota = quota
    log.Printf("[INFO] AccountQuotaManager: Set default quota (concurrent=%d, rps=%d)",
        quota.MaxConcurrent, quota.MaxRPS)
}

// GetQuota returns quota for an account (or default if not found)
func (aqm *AccountQuotaManager) GetQuota(accountID string) *AccountQuota {
    if !aqm.enabled {
        return nil // Quotas disabled
    }

    aqm.mu.RLock()
    defer aqm.mu.RUnlock()

    // Try specific quota first
    if quota, exists := aqm.quotas[accountID]; exists {
        return quota
    }

    // Fallback to default
    return aqm.defaultQuota
}

// CheckAndReserve checks quota and reserves resources
func (aqm *AccountQuotaManager) CheckAndReserve(accountID string, estimatedTokens int64) (bool, string) {
    quota := aqm.GetQuota(accountID)
    if quota == nil {
        return true, "" // No quota restrictions
    }

    return quota.CheckAndReserve(estimatedTokens)
}

// Release releases resources for an account
func (aqm *AccountQuotaManager) Release(accountID string) {
    quota := aqm.GetQuota(accountID)
    if quota != nil {
        quota.Release()
    }
}

// GetAllStats returns statistics for all accounts
func (aqm *AccountQuotaManager) GetAllStats() []*AccountQuotaStats {
    aqm.mu.RLock()
    defer aqm.mu.RUnlock()

    stats := make([]*AccountQuotaStats, 0, len(aqm.quotas))
    for _, quota := range aqm.quotas {
        stats = append(stats, quota.GetStats())
    }

    return stats
}

// Reload reloads quotas from configuration
func (aqm *AccountQuotaManager) Reload(quotas []*AccountQuota) {
    aqm.mu.Lock()
    defer aqm.mu.Unlock()

    aqm.quotas = make(map[string]*AccountQuota)
    for _, quota := range quotas {
        aqm.quotas[quota.AccountID] = quota
    }

    log.Printf("[INFO] AccountQuotaManager: Reloaded %d account quotas", len(quotas))
}
```

### 3. 配置格式

#### 3.1 配置文件

```ini
# config/account_quotas.ini

# ============================================================================
# Per-Account Quota Configuration
# ============================================================================
# Define resource limits for each account/department/customer
#
# Quota dimensions:
#   max_concurrent:     Maximum concurrent requests
#   max_rps:            Maximum requests per second
#   max_tokens_per_sec: Maximum tokens per second
#   max_requests_per_day: Daily request limit (optional)
# ============================================================================

[account_quota_settings]
enabled = true
enforce_quotas = true  # Set to false for monitoring-only mode

# Default quota for accounts without specific configuration
[default_quota]
max_concurrent = 10
max_rps = 20
max_tokens_per_sec = 1000
max_requests_per_day = 10000
description = "Default quota for unmapped accounts"

# ============================================================================
# Internal Departments
# ============================================================================

[account:dept-a]
account_id = dept-a
max_concurrent = 30
max_rps = 100
max_tokens_per_sec = 1500
max_requests_per_day = 50000
priority = 0  # Link to priority from Phase 1
description = "Department A - ML Team (Critical)"

[account:dept-b]
account_id = dept-b
max_concurrent = 25
max_rps = 80
max_tokens_per_sec = 1200
max_requests_per_day = 40000
priority = 2
description = "Department B - Analytics"

[account:dept-c]
account_id = dept-c
max_concurrent = 25
max_rps = 80
max_tokens_per_sec = 1200
max_requests_per_day = 40000
priority = 3
description = "Department C - Recommendations"

# ============================================================================
# External Customers
# ============================================================================

[account:external-enterprise]
account_id = external-enterprise
max_concurrent = 20
max_rps = 50
max_tokens_per_sec = 800
max_requests_per_day = 20000
priority = 5
description = "Enterprise tier customers"

[account:external-premium]
account_id = external-premium
max_concurrent = 15
max_rps = 30
max_tokens_per_sec = 500
max_requests_per_day = 10000
priority = 6
description = "Premium tier customers"

[account:external-standard]
account_id = external-standard
max_concurrent = 10
max_rps = 20
max_tokens_per_sec = 300
max_requests_per_day = 5000
priority = 7
description = "Standard tier customers"

[account:external-free]
account_id = external-free
max_concurrent = 5
max_rps = 10
max_tokens_per_sec = 100
max_requests_per_day = 1000
priority = 9
description = "Free tier customers"
```

### 4. 集成到Scheduler

#### 4.1 ChannelScheduler集成

```go
// internal/scheduler/scheduler_channel.go

type ChannelScheduler struct {
    // ... existing fields ...

    // Account quota manager (Phase 2)
    accountQuotaManager *AccountQuotaManager
}

// Submit with account quota check
func (cs *ChannelScheduler) Submit(req *Request) error {
    // Phase 2: Check account quota BEFORE global capacity
    if cs.accountQuotaManager != nil {
        accepted, reason := cs.accountQuotaManager.CheckAndReserve(
            req.AccountID,
            req.EstimatedTokens,
        )

        if !accepted {
            cs.totalRejected.Add(1)
            log.Printf("[WARN] ChannelScheduler: Request %s rejected by account quota: %s",
                req.ID, reason)

            if req.ResultChan != nil {
                req.ResultChan <- &ScheduleResult{
                    Accepted: false,
                    Reason:   reason,
                    QueuePos: -1,
                }
            }

            return fmt.Errorf("account quota exceeded: %s", reason)
        }

        // Ensure quota is released when request completes
        // (in addition to global capacity release)
        defer func() {
            if !accepted {
                cs.accountQuotaManager.Release(req.AccountID)
            }
        }()
    }

    // Continue with existing logic (global capacity check, priority queue, etc.)
    // ...
}

// Release with account quota release
func (cs *ChannelScheduler) Release(req *Request) {
    // Release global capacity
    select {
    case cs.capacityReleaseChan <- &CapacityRelease{Req: req}:
    case <-time.After(time.Second):
        log.Printf("[WARN] ChannelScheduler.Release: Timeout releasing %s", req.ID)
    }

    // Release account quota (Phase 2)
    if cs.accountQuotaManager != nil {
        cs.accountQuotaManager.Release(req.AccountID)
    }
}
```

### 5. HTTP Endpoints

#### 5.1 查询Account Quotas

```go
// internal/httpserver/endpoint_account_quotas.go

// GET /admin/scheduler/account-quotas
func (s *Server) HandleAccountQuotas(w http.ResponseWriter, r *http.Request) {
    if s.accountQuotaManager == nil {
        s.respondJSON(w, http.StatusOK, map[string]interface{}{
            "enabled": false,
            "message": "Account quotas not configured",
        })
        return
    }

    stats := s.accountQuotaManager.GetAllStats()

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "enabled":        true,
        "total_accounts": len(stats),
        "quotas":         stats,
    })
}

// GET /admin/scheduler/account-quotas/:accountID
func (s *Server) HandleAccountQuotaDetail(w http.ResponseWriter, r *http.Request) {
    accountID := chi.URLParam(r, "accountID")

    if s.accountQuotaManager == nil {
        s.respondJSON(w, http.StatusOK, map[string]interface{}{
            "enabled": false,
        })
        return
    }

    quota := s.accountQuotaManager.GetQuota(accountID)
    if quota == nil {
        s.respondJSON(w, http.StatusNotFound, map[string]interface{}{
            "error": "Account not found",
        })
        return
    }

    s.respondJSON(w, http.StatusOK, quota.GetStats())
}
```

### 6. 监控和可观测性

#### 6.1 Prometheus Metrics (可选)

```go
// internal/scheduler/metrics.go

var (
    accountQuotaRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "scheduler_account_quota_requests_total",
            Help: "Total requests per account",
        },
        []string{"account_id", "result"}, // result: accepted, rejected
    )

    accountQuotaConcurrent = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "scheduler_account_quota_concurrent",
            Help: "Current concurrent requests per account",
        },
        []string{"account_id"},
    )
)
```

#### 6.2 日志格式

```
[WARN] ChannelScheduler: Request req-123 rejected by account quota: account dept-a concurrent limit exceeded (30/30)
[INFO] AccountQuotaManager: Account dept-a stats: concurrent=28/30, rps=95/100, daily=45123/50000
```

### 7. 测试策略

#### 7.1 单元测试

```go
func TestAccountQuota_ConcurrentLimit(t *testing.T) {
    quota := NewAccountQuota("test-account", 5, 100, 1000)

    // Accept 5 requests
    for i := 0; i < 5; i++ {
        accepted, _ := quota.CheckAndReserve(100)
        assert.True(t, accepted)
    }

    // 6th request should be rejected
    accepted, reason := quota.CheckAndReserve(100)
    assert.False(t, accepted)
    assert.Contains(t, reason, "concurrent limit exceeded")

    // Release one
    quota.Release()

    // Now should accept again
    accepted, _ = quota.CheckAndReserve(100)
    assert.True(t, accepted)
}

func TestAccountQuota_RPSLimit(t *testing.T) {
    quota := NewAccountQuota("test-account", 100, 10, 10000)

    // Accept 10 requests (within RPS limit)
    for i := 0; i < 10; i++ {
        accepted, _ := quota.CheckAndReserve(100)
        assert.True(t, accepted)
        quota.Release() // Release immediately (doesn't affect RPS)
    }

    // 11th request should be rejected (RPS limit)
    accepted, reason := quota.CheckAndReserve(100)
    assert.False(t, accepted)
    assert.Contains(t, reason, "RPS limit exceeded")

    // Wait for window reset
    time.Sleep(1 * time.Second)

    // Should accept again
    accepted, _ = quota.CheckAndReserve(100)
    assert.True(t, accepted)
}
```

#### 7.2 集成测试

```bash
# Submit requests from dept-a (limit: 30 concurrent)
for i in {1..35}; do
  curl -X POST http://localhost:8081/v1/chat/completions \
    -H "Authorization: Bearer sk-dept-a-key" \
    -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}' &
done

# Check stats - should see ~30 accepted, 5 rejected
curl http://localhost:8081/admin/scheduler/account-quotas/dept-a | jq .

# Expected output:
# {
#   "account_id": "dept-a",
#   "current_concurrent": 30,
#   "max_concurrent": 30,
#   "total_rejections": 5
# }
```

### 8. 与Priority Queue的正交性

```
Request Flow with Both Systems:

1. Extract Account ID from API key (Phase 1)
   ↓
2. Check Account Quota (Phase 2) ← 第一道门槛：资源上限
   ├─ Rejected? → 429 Too Many Requests
   └─ Accepted? → Continue
       ↓
3. Check Global Capacity (Existing)
   ├─ Available? → Execute immediately
   └─ Full? → Enqueue to Priority Queue ← 第二道门槛：处理顺序
       ↓
4. Priority Queue Scheduling (Existing)
   - Dequeue based on priority (P0-P9)
   - WFQ/Hybrid policy
   ↓
5. Execute Request
   ↓
6. Release Resources (Both account quota AND global capacity)
```

**关键点**:
- Account Quota限制"能否进入系统"（资源上限）
- Priority Queue决定"何时被处理"（处理顺序）
- 两者独立工作，可单独启用/禁用

### 9. 配置示例：电商GPU共享

```ini
# 白天配置（内部优先）
[account:dept-a]
max_concurrent = 30  # 内部部门A
[account:dept-b]
max_concurrent = 25  # 内部部门B
[account:external-all]
max_concurrent = 20  # 外部所有客户共享

# 夜晚配置（外部优先，Phase 3动态调整）
[account:dept-a]
max_concurrent = 15  # 内部降低
[account:external-all]
max_concurrent = 60  # 外部提升
```

### 10. 实施步骤

1. **核心Quota实现** (3小时)
2. **Quota Manager** (2小时)
3. **Scheduler集成** (1.5小时)
4. **HTTP Endpoints** (1小时)
5. **测试和文档** (1.5小时)

### 11. 验收标准

- [ ] Per-account concurrent limit
- [ ] Per-account RPS limit
- [ ] Per-account tokens/sec limit
- [ ] Per-account daily limit (optional)
- [ ] 与priority queue正交工作
- [ ] HTTP endpoints查询quota状态
- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试通过
- [ ] 向后兼容（可选启用）

## 下一步

Phase 2完成后，进入**Phase 3: Time-Based Dynamic Rules**，实现白天/夜晚自动切换配额策略。
