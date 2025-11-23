# Terminology Alignment Complete - Ready for Implementation

**Date:** 2025-11-23
**Status:** ✅ 完成 - 术语已对齐，架构边界已澄清
**Summary:** Scheduling 文档已与总体架构完全对齐，可以开始实施

---

## 执行摘要

已成功完成以下工作：

1. ✅ **术语对齐** - API 接口、数据结构与 `arc_design/v20251123/` 100% 一致
2. ✅ **架构边界澄清** - 明确分离 Gateway Scheduling（内部调度）vs Marketplace Supply Selection（供给选择）
3. ✅ **职责分离** - selectBestSupply() 从 Gateway 移除，确认为 Marketplace 职责
4. ✅ **执行窗口文档化** - 补充详细的启动窗口 vs 执行窗口说明
5. ✅ **接口完整性** - 所有 API 字段、请求/响应结构已对齐

---

## 主要变更记录

### 1. 明确架构边界（最重要！）

**新增章节**：`06_MARKETPLACE_INTEGRATION.md` 第161-230行

添加了 "⚠️ IMPORTANT: Scheduling vs Supply Selection (Architecture Boundary)" 章节，澄清：

- **Gateway Scheduling**：内部调度，5级优先队列，纳秒-微秒级延迟
- **Marketplace Supply Selection**：远程供给选择，DSP式打分，毫秒级延迟（网络调用）
- 两者是**独立模块**，解决不同层次的问题

**关键对比表**：

| Aspect | Gateway Scheduling | Marketplace Supply Selection |
|--------|-------------------|------------------------------|
| **Location** | Gateway process (local) | Marketplace service (remote) |
| **Repo** | tokligence-gateway | tokligence-marketplace |
| **Input** | Requests already received | Model + region requirements |
| **Output** | Execute / queue / reject | Supplier + price + token |
| **Delay** | Nanoseconds - microseconds | Milliseconds (network call) |
| **Algorithm** | Priority queue + WFQ | DSP-style scoring |

### 2. API 接口对齐

**2.1 Quote API（第51-90行）**

更新为：
```go
// 旧：selectBestSupply()  - Gateway端选择（错误！）
// 新：GetQuote() - 调用Marketplace API

POST /v1/marketplace/quote
Request: {
  "model": "gpt-4",
  "estimated_tokens": 1500,
  "region_hint": "us-east-1",
  "sla_target": "standard"
}

Response: {
  "quote_id": "q-abc123",
  "supply": {
    "supply_id": "sup-1",
    "endpoint": "https://...",
    "signed_token": "eyJ...",
    "price_per_mtoken": 8.40,           // 用户支付（含佣金）
    "supplier_price_per_mtoken": 8.00, // 供应商收入
    "commission_rate": 0.05,
    "region": "us-east-1",
    "p99_latency_ms": 500,
    "throughput_tps": 10000,
    "availability": 0.999
  },
  "expires_at": "2025-11-23T12:35:00Z",  // 启动窗口
  "exec_timeout_sec": 1800                // 执行窗口
}
```

**2.2 Usage API（第97-137行）**

更新为：
```go
// 旧：reportUsage() - Gateway计算计费（错误！）
// 新：ReportUsage() - 上报实际用量，Marketplace计算计费

POST /v1/marketplace/usage
Request: {
  "request_id": "req-xyz789",
  "quote_id": "q-abc123",      // 新增：关联Quote
  "supply_id": "sup-1",
  "model": "gpt-4",
  "prompt_tokens": 500,         // 拆分：原TotalTokens
  "completion_tokens": 1000,    // 拆分：原TotalTokens
  "latency_ms": 450,
  "status": "ok",               // ok | timeout | error
  "user_id": "user-456",
  "source": "consumer"          // 新增：计量来源
}

Response: {
  "transaction_id": "tx-def456",
  "gmv_usd": 0.012,            // Marketplace计算
  "commission_usd": 0.0006,    // Marketplace计算
  "commission_rate": 0.05,
  "supplier_payout_usd": 0.012,
  "user_charge_usd": 0.0126
}
```

### 3. 数据结构对齐

**3.1 新增结构体（第787-842行）**

```go
// QuoteRequest - 请求Quote
type QuoteRequest struct {
    RequestID       string
    Model           string
    EstimatedTokens int
    RegionHint      string
    SLATarget       string
    MaxLatencyMs    int
    UserID          string
}

// SupplyQuote - Quote响应
type SupplyQuote struct {
    QuoteID        string
    Supply         SupplyInfo
    ExpiresAt      time.Time    // 启动窗口
    ExecTimeoutSec int          // 执行窗口
}

// SupplyInfo - 选中的供给信息
type SupplyInfo struct {
    SupplyID               string
    Endpoint               string
    SignedToken            string
    PricePerMToken         float64  // 用户支付（含佣金）
    SupplierPricePerMToken float64  // 供应商收入
    CommissionRate         float64  // 佣金率
    Region                 string
    P99LatencyMs           int
    ThroughputTPS          int
    Availability           float64
}
```

**3.2 新增结构体（第1018-1042行）**

```go
// UsageReport - 使用上报
type UsageReport struct {
    RequestID        string
    QuoteID          string    // 关联Quote
    SupplyID         string
    Model            string
    PromptTokens     int       // 拆分tokens
    CompletionTokens int
    LatencyMs        int
    Status           string    // ok | timeout | error
    ErrorCode        string
    UserID           string
    Source           string    // consumer | provider | proxy
    Timestamp        time.Time
}

// TransactionResult - 计费结果
type TransactionResult struct {
    TransactionID    string
    GMV              float64
    Commission       float64
    CommissionRate   float64
    SupplierPayout   float64
    UserCharge       float64
}
```

### 4. Gateway 实现更新

**4.1 RouteRequest() - 第731-763行**

```go
func (mp *MarketplaceProvider) RouteRequest(ctx context.Context, req *Request) (*Response, error) {
    // Step 1: Get Quote from Marketplace（供给选择在Marketplace端）
    quote, err := mp.client.GetQuote(ctx, &QuoteRequest{
        Model:           req.Model,
        EstimatedTokens: req.EstimatedTokens,
        RegionHint:      mp.config.PreferRegion,
        SLATarget:       "standard",
        UserID:          req.UserID,
    })

    // Step 2: Check startup window（启动窗口检查）
    if time.Now().After(quote.ExpiresAt) {
        return nil, fmt.Errorf("quote expired, need to re-quote")
    }

    // Step 3: Execute with execution deadline（执行窗口由exec_deadline控制）
    execCtx, cancel := context.WithTimeout(ctx, time.Duration(quote.ExecTimeoutSec)*time.Second)
    defer cancel()

    resp, err := mp.client.ExecuteWithToken(execCtx, quote.Supply.Endpoint, quote.Supply.SignedToken, req)

    // Step 4: Report usage（上报实际用量，计费由Marketplace完成）
    go mp.reportUsage(quote.QuoteID, quote.Supply.SupplyID, req, resp)

    return resp, err
}
```

**4.2 reportUsage() - 第978-1016行**

```go
func (mp *MarketplaceProvider) reportUsage(quoteID, supplyID string, req *Request, resp *Response) {
    // Gateway只上报实际用量，不计算计费
    usage := &UsageReport{
        RequestID:        req.RequestID,
        QuoteID:          quoteID,           // 关联Quote
        SupplyID:         supplyID,
        Model:            req.Model,
        PromptTokens:     resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        LatencyMs:        int(resp.LatencyMs),
        Status:           "ok",
        UserID:           req.UserID,
        Source:           "consumer",        // 计量来源
        Timestamp:        time.Now(),
    }

    // Marketplace返回计算后的计费
    transaction, err := mp.client.ReportUsage(usage)

    log.Info("Transaction: tx=%s, GMV=$%.4f, commission=$%.4f",
        transaction.TransactionID,
        transaction.GMV,
        transaction.Commission)
}
```

### 5. 删除错误实现

**5.1 移除 selectBestSupply()（第931-976行）**

原来错误地将供给选择算法放在 Gateway 端，现已改为注释说明：

```go
// ============================================================================
// NOTE: Multi-dimensional routing (selectBestSupply) is Marketplace's responsibility
// ============================================================================
//
// Gateway does NOT select suppliers. Marketplace does it via Quote API.
//
// The following algorithm is implemented on **MARKETPLACE side** (tokligence-marketplace repo):
//
// func (ms *MarketplaceService) selectBestSupply(supplies []*Supply, req *QuoteRequest) *Supply {
//     // Multi-dimensional scoring:
//     // Score = 0.40×Price + 0.30×Latency + 0.15×Availability + 0.10×Throughput + 0.05×Region
//     ...
// }
//
// Gateway just calls Quote API and receives the selected supply.
// See: /home/alejandroseaah/tokligence/arc_design/v20251123/03_MARKETPLACE_DESIGN.md
```

### 6. 新增执行窗口文档（第251-381行）

完整章节："1. Execution Window vs Startup Window"

**关键内容：**

- **两个独立时间窗口**：启动窗口（30-120s）vs 执行窗口（15-30分钟）
- **为什么分离**：长推理模型（o1）不会被 Quote 过期中断
- **实现细节**：JWT payload 包含 `exec_deadline` claim
- **按模型配置**：gpt-3.5 (15min), gpt-4 (30min), o1 (60min)
- **错误处理场景**：Quote 过期 / 执行超时 / 正常长执行

---

## 与总体架构的对齐度

### 对齐验证清单

- [x] **API 契约** - 100% 符合 `04_API_CONTRACTS.md`
  - Quote API: ✅ 字段完全一致
  - Usage API: ✅ 字段完全一致

- [x] **数据模型** - 100% 符合 `05_DATA_MODEL.md`
  - QuoteRequest/SupplyQuote: ✅
  - UsageReport/TransactionResult: ✅

- [x] **职责划分** - 100% 符合 `02_GATEWAY_DESIGN.md` + `03_MARKETPLACE_DESIGN.md`
  - Gateway: Quote调用者，Usage上报者 ✅
  - Marketplace: Quote提供者，计费计算者 ✅

- [x] **执行窗口** - 100% 符合 `overall_workflow.md`
  - expires_at vs exec_timeout_sec: ✅
  - JWT exec_deadline claim: ✅

- [x] **计量可信度** - 100% 符合 `04_API_CONTRACTS.md`
  - Source字段: consumer/provider/proxy ✅

- [x] **商业模式** - 100% 符合
  - 5% 交易佣金: ✅
  - GMV + Commission 拆账: ✅
  - 无订阅/套餐: ✅

---

## 文件变更清单

### 修改的文件

1. **`docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md`**
   - 新增：架构边界说明章节（第161-230行）
   - 新增：执行窗口详细文档（第251-381行）
   - 更新：Transaction Flow（第51-140行）
   - 更新：QuoteRequest/SupplyQuote 结构（第787-818行）
   - 更新：UsageReport/TransactionResult 结构（第1018-1042行）
   - 更新：RouteRequest() 实现（第731-763行）
   - 更新：reportUsage() 实现（第978-1016行）
   - 更新：GetQuote() 客户端（第890-914行）
   - 更新：ExecuteWithToken() 客户端（第916-929行）
   - 更新：ReportUsage() 客户端（第1044-1096行）
   - 移除：selectBestSupply() 改为注释（第931-976行）

### 新建的文件

2. **`docs/scheduling/ARCHITECTURE_ALIGNMENT_ANALYSIS.md`**
   - 详细对比分析 scheduling vs arc_design
   - 9个维度的对齐度评估
   - 需要调整的地方清单（已完成）

3. **`docs/scheduling/TERMINOLOGY_ALIGNMENT_COMPLETE.md`** (本文档)
   - 完整变更记录
   - 对齐验证清单
   - 实施准备确认

---

## 实施准备确认

### ✅ 可以立即开始实施

**理由：**
1. API 接口 100% 对齐总体架构
2. 架构边界清晰，无混淆
3. 数据结构完整，字段齐全
4. 执行窗口机制已文档化
5. 商业模型一致（5% 佣金）

### 推荐实施顺序

**Phase 1: Gateway MarketplaceProvider（本 repo）**
```
Week 1-2:
├─ internal/provider/marketplace/
│  ├─ client.go           - GetQuote() / ExecuteWithToken() / ReportUsage()
│  ├─ provider.go         - RouteRequest() / reportUsage()
│  └─ types.go            - QuoteRequest / SupplyQuote / UsageReport
│
├─ Tests:
│  ├─ TC-P3-BILLING-001   - 交易佣金计算
│  ├─ TC-P3-BILLING-002   - GMV 对账
│  └─ TC-P3-BILLING-003   - API key 要求
│
└─ Integration:
   └─ Mock Marketplace API（返回固定Quote用于测试）
```

**Phase 2: Marketplace Service（独立 repo）**
```
Week 3-4:
├─ Marketplace API 实现
│  ├─ POST /v1/marketplace/quote   - DSP式供给选择
│  ├─ POST /v1/marketplace/usage   - 计费计算
│  └─ Database: supplies / quotes / usage_events / transactions
│
├─ Tests:
│  ├─ TC-P3-ROUTING-001   - 价格优化选择
│  ├─ TC-P3-ROUTING-002   - 延迟 vs 价格权衡
│  ├─ TC-P3-ROUTING-003   - 多区域选择
│  └─ TC-P3-ROUTING-004   - 吞吐优化
│
└─ End-to-End测试（Gateway + Marketplace集成）
```

### 开分支命令

```bash
cd /home/alejandroseaah/tokligence/sell_dev/tokligence-gateway

# 创建实施分支
git checkout -b feat/marketplace-integration

# 确认文档对齐
git add docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md
git add docs/scheduling/ARCHITECTURE_ALIGNMENT_ANALYSIS.md
git add docs/scheduling/TERMINOLOGY_ALIGNMENT_COMPLETE.md

git commit -m "docs: align scheduling design with overall architecture

- Clarify Gateway Scheduling vs Marketplace Supply Selection boundary
- Update API interfaces to match arc_design/v20251123
- Add execution window documentation (startup vs execution)
- Remove selectBestSupply() from Gateway (Marketplace's responsibility)
- Align data structures: QuoteRequest/SupplyQuote/UsageReport/TransactionResult
- Update RouteRequest() to call Quote API instead of local selection
- Update reportUsage() to report usage (Marketplace calculates billing)

Refs: arc_design/v20251123/04_API_CONTRACTS.md
"

# 准备开始实施
echo "✅ Ready to implement MarketplaceProvider"
```

---

## 后续可选优化（不阻塞实施）

### 低优先级增强

1. **验签机制详细文档**（Phase 2-3）
   - JWT/JWKS 验签流程
   - mTLS 信道加固
   - 密钥轮换策略

2. **计量可信度增强**（Phase 3-4）
   - Provider 上报对账
   - Marketplace 侧车/中继
   - 双侧比对审计

3. **Region vs Load 评分融合**（Phase 4+）
   - 合并 Load 和 Region 评分
   - 调整权重分配

---

## 联系与反馈

**文档位置**:
- Scheduling 设计: `/home/alejandroseaah/tokligence/sell_dev/tokligence-gateway/docs/scheduling/final/`
- 总体架构: `/home/alejandroseaah/tokligence/arc_design/v20251123/`

**对齐报告**:
- 详细分析: `docs/scheduling/ARCHITECTURE_ALIGNMENT_ANALYSIS.md`
- 变更总结: `docs/scheduling/TERMINOLOGY_ALIGNMENT_COMPLETE.md` (本文档)

**状态**: ✅ 术语对齐完成，可以开始实施

---

**Last Updated**: 2025-11-23
**Ready for**: feat/marketplace-integration branch
