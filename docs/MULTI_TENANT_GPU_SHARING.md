# Multi-Tenant GPU Sharing Support

## 场景描述

电商公司共享GPU集群场景：
- **内部部门**: 3个部门（A/B/C），各有API key，白天GPU使用高峰
- **外部客户**: Premium/Standard/Free三个tier，夜晚流量高峰
- **资源策略**: 白天优先内部，夜晚释放给外部

## 当前实现支持程度

### ✅ 已完全支持

#### 1. 优先级调度（Priority-Based Scheduling）

**配置示例**:
```ini
scheduler_policy = hybrid
scheduler_priority_levels = 10
```

**使用方法**:
```bash
# 内部部门A（最高优先级）
curl -H "X-Priority: 0" -H "Authorization: Bearer dept_a_key" ...

# 内部部门B（高优先级）
curl -H "X-Priority: 2" -H "Authorization: Bearer dept_b_key" ...

# 外部Premium客户（中优先级）
curl -H "X-Priority: 5" -H "Authorization: Bearer premium_key" ...

# 外部Free客户（低优先级）
curl -H "X-Priority: 9" -H "Authorization: Bearer free_key" ...
```

**支持度**: ✅ 100%
- P0-P9十个优先级
- Hybrid策略（P0严格优先，P1-P9加权公平）
- 权重可配置（默认P0=256x, P9=1x）

#### 2. 全局容量限制（Global Capacity Limits）

**配置示例**:
```ini
# 假设8个A100 GPU，每个~500 tokens/sec
scheduler_max_tokens_per_sec = 4000
scheduler_max_concurrent = 100
scheduler_max_rps = 200
```

**支持度**: ✅ 100%
- Tokens/sec限流（PRIMARY指标）
- 并发请求数限制
- RPS限制
- Context长度限制

#### 3. 队列管理（Queue Management）

**配置示例**:
```ini
scheduler_max_queue_depth = 10000  # 每个优先级队列深度
scheduler_queue_timeout_sec = 60    # 排队超时
```

**支持度**: ✅ 100%
- 每个优先级独立队列
- Channel-based实现（无锁，高性能）
- 自动超时处理

#### 4. 实时监控（Real-time Monitoring）

**HTTP Endpoints**:
```bash
# 查看所有队列状态
curl http://localhost:8081/admin/scheduler/stats

# 查看最繁忙的队列
curl http://localhost:8081/admin/scheduler/queues?top=5
```

**返回示例**:
```json
{
  "total_scheduled": 12345,
  "total_queued_now": 42,
  "overall_utilization": 15.3,
  "queue_stats": [
    {
      "priority": 0,
      "current_depth": 5,
      "max_depth": 10000,
      "utilization_pct": 0.05,
      "available_slots": 9995
    },
    {
      "priority": 5,
      "current_depth": 25,
      "max_depth": 10000,
      "utilization_pct": 0.25,
      "available_slots": 9975
    }
  ]
}
```

**支持度**: ✅ 100%
- 实时queue depth
- Per-priority统计
- 容量利用率
- 忙碌队列排序

### ⚠️ 部分支持（需要额外配置）

#### 5. API Key识别

**当前状态**: ⚠️ 需要客户端配合

现在需要客户端在请求中设置`X-Priority`头：
```bash
curl -H "X-Priority: 0" -H "Authorization: Bearer dept_a_key" ...
```

**解决方案（手动配置）**:

可以在HTTP handler中添加API key → priority映射：

```go
// internal/httpserver/scheduler_integration.go
func (s *Server) extractPriorityFromRequest(r *http.Request) scheduler.PriorityTier {
    // 1. Check X-Priority header (explicit)
    if priority := r.Header.Get("X-Priority"); priority != "" {
        return parsePriority(priority)
    }

    // 2. Map from API key (implicit)
    apiKey := extractAPIKey(r)
    switch {
    case strings.HasPrefix(apiKey, "dept_a_"):
        return scheduler.PriorityCritical // P0
    case strings.HasPrefix(apiKey, "dept_b_"):
        return scheduler.PriorityHigh // P2
    case strings.HasPrefix(apiKey, "premium_"):
        return scheduler.PriorityNormal // P5
    case strings.HasPrefix(apiKey, "standard_"):
        return scheduler.PriorityLow // P7
    default:
        return scheduler.PriorityBackground // P9
    }
}
```

**工作量**: 🔨 小改动（30分钟）

### ❌ 当前不支持（需要开发）

#### 6. Per-Account配额限制

**需求**: 限制每个部门的独立配额

示例：
- 部门A: 最多30个并发
- 部门B: 最多25个并发
- External Premium: 最多15个并发

**当前限制**:
- 只有全局`MaxConcurrent`
- 无法限制单个账户

**实现方案**:

```go
// internal/scheduler/account_quota.go
type AccountQuota struct {
    AccountID       string
    MaxConcurrent   int
    CurrentConcurrent atomic.Int32
}

type AccountQuotaManager struct {
    quotas map[string]*AccountQuota
    mu     sync.RWMutex
}

func (aqm *AccountQuotaManager) CheckAndReserve(accountID string) bool {
    aqm.mu.RLock()
    quota := aqm.quotas[accountID]
    aqm.mu.RUnlock()

    if quota == nil {
        return true // No quota limit
    }

    current := quota.CurrentConcurrent.Load()
    if current >= int32(quota.MaxConcurrent) {
        return false // Quota exceeded
    }

    quota.CurrentConcurrent.Add(1)
    return true
}
```

**工作量**: 🔨 中等开发（2-3小时）

#### 7. 时间段动态调整

**需求**: 白天/夜晚不同配额策略

示例：
- 白天(08:00-18:00): 内部80%，外部20%
- 夜晚(18:00-08:00): 内部20%，外部80%

**当前限制**:
- 配置静态，不会随时间变化

**实现方案**:

```go
// internal/scheduler/time_based_quota.go
type TimeBasedRule struct {
    StartHour int
    EndHour   int

    // 白天
    InternalMaxConcurrent int  // 80
    ExternalMaxConcurrent int  // 20

    // 夜晚
    // Flip the values
}

func (cs *ChannelScheduler) ApplyTimeBasedRules() {
    hour := time.Now().Hour()

    if hour >= 8 && hour < 18 {
        // Daytime: Boost internal priorities
        cs.capacity.MaxConcurrent = 100
        cs.internalQuota = 80
        cs.externalQuota = 20
    } else {
        // Nighttime: Boost external priorities
        cs.capacity.MaxConcurrent = 100
        cs.internalQuota = 20
        cs.externalQuota = 80
    }
}
```

**配置示例**:
```ini
[scheduler_time_rules]
# Daytime (08:00-18:00)
day_start = 08:00
day_end = 18:00
day_internal_quota = 80
day_external_quota = 20

# Nighttime (18:00-08:00)
night_internal_quota = 20
night_external_quota = 80
```

**工作量**: 🔨 中等开发（3-4小时）

#### 8. 动态权重调整

**需求**: 根据实际使用情况动态调整权重

示例：
- 部门A白天weight=256，夜晚降为16
- External夜晚weight提升

**当前限制**:
- Weights静态配置，运行时不可变

**实现方案**:

```go
func (cs *ChannelScheduler) AdjustWeights(newWeights []float64) {
    cs.wfqMu.Lock()
    defer cs.wfqMu.Unlock()

    for i, weight := range newWeights {
        cs.config.Weights[i] = weight
    }

    log.Printf("[INFO] Weights adjusted: %v", newWeights)
}

// Cron job: 每小时检查并调整
func (cs *ChannelScheduler) autoAdjustWeights() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        hour := time.Now().Hour()

        if hour >= 8 && hour < 18 {
            // Daytime: Boost internal
            cs.AdjustWeights([]float64{256, 128, 64, 32, 16, 4, 2, 1, 1, 1})
        } else {
            // Nighttime: Boost external
            cs.AdjustWeights([]float64{16, 16, 16, 16, 16, 64, 32, 32, 16, 8})
        }
    }
}
```

**工作量**: 🔨 小改动（1-2小时）

## 当前方案：如何配置电商GPU共享场景

### 配置文件（scheduler.ini）

```ini
[scheduler]
scheduler_enabled = true
scheduler_policy = hybrid  # P0 strict, P1-P9 WFQ

# Capacity (8x A100 GPUs example)
scheduler_max_tokens_per_sec = 4000
scheduler_max_concurrent = 100
scheduler_max_rps = 200

# Weights (内部256x, 外部free 1x)
scheduler_weights = 256,128,64,32,16,8,4,2,1,1

# Queue
scheduler_max_queue_depth = 10000
scheduler_queue_timeout_sec = 60
```

### 客户端使用

**内部部门**:
```bash
# 部门A（关键业务）
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "X-Priority: 0" \
  -H "Authorization: Bearer dept_a_api_key" \
  -d '...'

# 部门B（分析）
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "X-Priority: 2" \
  -H "Authorization: Bearer dept_b_api_key" \
  -d '...'

# 部门C（推荐）
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "X-Priority: 3" \
  -H "Authorization: Bearer dept_c_api_key" \
  -d '...'
```

**外部客户**:
```bash
# Premium客户
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "X-Priority: 5" \
  -H "Authorization: Bearer premium_customer_key" \
  -d '...'

# Standard客户
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "X-Priority: 7" \
  -H "Authorization: Bearer standard_customer_key" \
  -d '...'

# Free客户
curl -X POST http://gateway:8081/v1/chat/completions \
  -H "X-Priority: 9" \
  -H "Authorization: Bearer free_customer_key" \
  -d '...'
```

### 监控使用情况

```bash
# 实时查看各部门/客户的queue占用
curl http://gateway:8081/admin/scheduler/stats | jq '{
  total_concurrent: .total_scheduled,
  queue_status: [
    .queue_stats[] |
    select(.current_depth > 0) |
    {
      priority: ("P" + (.priority | tostring)),
      queued: .current_depth,
      utilization: (.utilization_pct | floor | tostring + "%")
    }
  ]
}'

# 找出最繁忙的优先级
curl http://gateway:8081/admin/scheduler/queues?top=3 | jq .
```

## 限制和解决方案对比表

| 功能 | 当前支持 | 限制 | 解决方案 | 工作量 |
|------|---------|-----|---------|--------|
| **优先级调度** | ✅ 完全支持 | 需要客户端设置X-Priority | 添加API key映射 | 30分钟 |
| **全局容量限制** | ✅ 完全支持 | - | - | - |
| **队列管理** | ✅ 完全支持 | - | - | - |
| **实时监控** | ✅ 完全支持 | - | - | - |
| **Per-Account配额** | ❌ 不支持 | 无法限制单个账户 | AccountQuotaManager | 2-3小时 |
| **时间段动态调整** | ❌ 不支持 | 配置静态 | TimeBasedRule + Cron | 3-4小时 |
| **动态权重调整** | ❌ 不支持 | Weights运行时不可变 | AdjustWeights API | 1-2小时 |
| **API Key自动映射** | ⚠️ 手动 | 需客户端配合 | extractPriorityFromRequest | 30分钟 |

## 推荐实施路径

### Phase 1: 立即可用（0开发）

使用当前实现 + 客户端配合：
1. 配置权重（内部256x，外部1x-8x）
2. 客户端设置`X-Priority`头
3. 使用HTTP endpoints监控

**满足度**: 70% ✅
- ✅ 优先级调度
- ✅ 容量限制
- ✅ 实时监控
- ❌ 无per-account限制
- ❌ 无时间段调整

### Phase 2: 快速增强（1天开发）

添加关键功能：
1. API Key → Priority自动映射（30分钟）
2. 动态权重调整API（1-2小时）
3. Per-Account配额管理（2-3小时）

**满足度**: 90% ✅✅
- ✅ 所有Phase 1功能
- ✅ API key自动识别
- ✅ Per-account限制
- ✅ 手动调整权重
- ❌ 无自动时间段切换

### Phase 3: 完整方案（2-3天开发）

添加自动化：
1. 时间段规则引擎（3-4小时）
2. 自动权重调整（基于时间）（2小时）
3. Dashboard UI（可选，4-6小时）
4. 告警系统（可选，2-3小时）

**满足度**: 100% ✅✅✅
- ✅ 完全自动化
- ✅ 时间段智能切换
- ✅ 可视化监控
- ✅ 异常告警

## 结论

**当前实现能满足该场景的 70%需求**，剩余30%需要1-3天开发。

**立即可用的方案**：
1. 使用Hybrid策略 + 权重配置
2. 客户端设置X-Priority（内部0-3，外部5-9）
3. HTTP endpoints监控各部门/客户使用情况
4. 手动调整容量限制应对白天/夜晚

**推荐先试用Phase 1方案**，根据实际需求决定是否需要Phase 2/3的增强功能。
