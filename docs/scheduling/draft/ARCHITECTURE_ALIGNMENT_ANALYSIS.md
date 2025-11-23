# Scheduling Design vs Overall Architecture Alignment Analysis

**Date:** 2025-11-23
**Status:** ✅ 高度对齐，少量术语/接口需统一
**Conclusion:** Scheduling 设计基本符合总体架构，可直接进入实施，建议先进行少量术语统一

---

## Executive Summary

对比 `docs/scheduling/final/` 与 `/home/alejandroseaah/tokligence/arc_design/v20251123/` 后发现：

**✅ 高度一致的核心设计：**
- 商业模式：5% 交易佣金，无订阅/套餐（100% 对齐）
- 多维度路由：价格/延迟/吞吐/可用性/地域打分（算法一致）
- 计费模型：GMV + Commission 拆账（逻辑一致）
- Provider 抽象：LocalProvider + MarketplaceProvider（架构一致）

**⚠️ 需要统一的术语/接口：**
1. **API 字段命名**：scheduling 用 `selectBestSupply()`，总体架构用 Quote API 返回 `supply` 对象
2. **执行窗口概念**：总体架构明确区分"启动窗口（expires_at）"和"执行窗口（exec_timeout_sec）"
3. **验签机制**：总体架构详细定义了 JWT/JWKS/mTLS，scheduling 文档未详述
4. **计量可信度**：总体架构强调 `source` 字段（consumer/provider/proxy），scheduling 未提及

**🔧 推荐调整（非阻塞）：**
- 将 scheduling 文档中的函数签名对齐到 `04_API_CONTRACTS.md`
- 补充执行窗口与启动窗口的区分
- 明确验签流程（可后续迭代）

**✅ 可立即开始实施：**
- 核心逻辑无冲突，术语差异不影响实现
- 建议按总体架构 API 合同（`04_API_CONTRACTS.md`）为准
- Scheduling 的调度/保护层设计完全兼容

---

## 1. 商业模式对齐度：100%

### Scheduling Design
```
Revenue: 5% commission on GMV
NO monthly fees, NO usage limits, NO subscription tiers

Example:
  Supplier price: $100/Mtok
  User pays: $105/Mtok
  Commission: $5 (5%)
```
**Source:** `docs/scheduling/final/CORRECT_BUSINESS_MODEL.md`

### Overall Architecture
```
商业模式: Pay-as-you-go，交易抽佣 5%（无订阅/套餐/免费额度）

GMV = tokens × supplier_price
Commission = GMV × commission_rate (默认 5%)
用户价 = GMV × (1 + commission_rate)
```
**Source:** `arc_design/v20251123/01_SYSTEM_OVERVIEW.md`

**✅ 完全一致** - 无需调整

---

## 2. 多维度路由算法对齐度：95%

### Scheduling Design
```go
// 06_MARKETPLACE_INTEGRATION.md:740-841
Score = 0.4×Price + 0.3×Latency + 0.15×Availability + 0.1×Throughput + 0.05×Load

Normalization:
- Price: Lower is better (vs OpenAI $30/Mtok baseline)
- Latency: Lower P99 is better (max 5000ms)
- Availability: Higher is better (0-1 range)
- Throughput: Higher tokens/sec is better (max 50K tok/s)
- Load: Lower current load is better
```

### Overall Architecture
```
// 03_MARKETPLACE_DESIGN.md:23-36
score = w_price*price_score
      + w_latency*latency_score
      + w_throughput*throughput_score
      + w_availability*availability_score
      + w_region*region_score

price_score       = 1 - (price / price_max)
latency_score     = 1 - (p99 / max_latency_ms)
throughput_score  = min(1, available_tps / req_tps)
availability_score= availability (0-1)
region_score      = 1 if region==preferred else decay_by_distance
```

**对比分析：**

| 维度 | Scheduling | Overall Arch | 对齐度 |
|------|------------|--------------|--------|
| Price | ✅ 40% weight | ✅ w_price | 100% |
| Latency | ✅ 30% weight (P99) | ✅ w_latency (p99) | 100% |
| Availability | ✅ 15% weight | ✅ w_availability | 100% |
| Throughput | ✅ 10% weight | ✅ w_throughput | 100% |
| Load/Region | ⚠️ 5% Load | ⚠️ Region score | 95% |

**差异点：**
- Scheduling 用 "Load"（当前负载）
- Overall Arch 用 "Region"（地域亲和）

**建议：** 两者可共存，总权重调整为：
```
Score = 0.35×Price + 0.25×Latency + 0.15×Availability
      + 0.10×Throughput + 0.10×Region + 0.05×Load
```

---

## 3. API 接口对齐度：85%

### Scheduling Design (MarketplaceProvider)

**函数签名：**
```go
// 06_MARKETPLACE_INTEGRATION.md:740-918
func selectBestSupply(supplies []*Supply, req *Request) *Supply
func reportUsage(supplyID string, req *Request, resp *Response)

type Supply struct {
    ID               string
    PricePerMToken   float64
    P99LatencyMs     int
    ThroughputTPS    int
    Availability     float64
    Region           string
}
```

### Overall Architecture (Quote API)

**API Contract:**
```json
// 04_API_CONTRACTS.md:48-86
POST /v1/marketplace/quote
Request:
{
  "model": "qwen-3-70b-instruct",
  "estimated_tokens": 12000,
  "region_hint": "ap-southeast-1",
  "sla_target": "standard"
}

Response:
{
  "quote_id": "q-abc123",
  "supply": {
    "supply_id": "sup-1",
    "endpoint": "https://...",
    "signed_token": "eyJ...",
    "price_per_mtoken": 8.00,
    "supplier_price_per_mtoken": 7.62,
    "commission_rate": 0.05,
    "region": "ap-southeast-1",
    "p99_latency_ms": 450,
    "throughput_tps": 12000,
    "availability": 0.999
  },
  "expires_at": "2025-11-24T00:00:00Z",
  "exec_timeout_sec": 1800
}
```

**对比分析：**

| 字段 | Scheduling | Overall Arch | 对齐度 |
|------|------------|--------------|--------|
| 供给选择 | `selectBestSupply()` | `POST /quote` | ⚠️ 需映射 |
| 供给ID | `Supply.ID` | `supply.supply_id` | ✅ 相同 |
| 价格 | `PricePerMToken` | `price_per_mtoken` | ✅ 相同 |
| 供应商价格 | ❌ 缺失 | `supplier_price_per_mtoken` | ⚠️ 需补充 |
| 佣金率 | ❌ 缺失 | `commission_rate` | ⚠️ 需补充 |
| Endpoint | ❌ 缺失 | `endpoint` | ⚠️ 需补充 |
| Signed Token | ❌ 缺失 | `signed_token` | ⚠️ 需补充 |
| Quote ID | ❌ 缺失 | `quote_id` | ⚠️ 需补充 |
| 执行窗口 | ❌ 缺失 | `exec_timeout_sec` | ⚠️ 需补充 |

**建议调整：**

将 scheduling 的 `Supply` 结构对齐到总体架构：

```go
// 更新后的 Supply 结构（兼容 04_API_CONTRACTS.md）
type SupplyQuote struct {
    QuoteID              string    // 新增
    SupplyID             string    // 原 ID
    Endpoint             string    // 新增
    SignedToken          string    // 新增
    PricePerMToken       float64   // 用户支付单价（含佣金）
    SupplierPricePerMToken float64 // 新增：供应商单价
    CommissionRate       float64   // 新增：佣金率（0.05）
    Region               string
    P99LatencyMs         int
    ThroughputTPS        int
    Availability         float64
    ExpiresAt            time.Time // 新增：启动窗口
    ExecTimeoutSec       int       // 新增：执行窗口
}

// 客户端调用改为：
func (mp *MarketplaceProvider) GetQuote(ctx context.Context, req *QuoteRequest) (*SupplyQuote, error)
func (mp *MarketplaceProvider) ReportUsage(ctx context.Context, usage *UsageReport) error
```

---

## 4. 计费模型对齐度：90%

### Scheduling Design
```go
// 06_MARKETPLACE_INTEGRATION.md:843-918
func reportUsage(supplyID string, req *Request, resp *Response) {
    tokensUsed := resp.Usage.TotalTokens
    supplierCost := (float64(tokensUsed) / 1_000_000.0) * supply.PricePerMToken
    userCost := supplierCost * 1.05  // 5% markup
    commission := userCost - supplierCost

    usage := &Usage{
        SupplyID:     supplyID,
        UserID:       req.UserID,
        TokensUsed:   tokensUsed,
        SupplierCost: supplierCost,
        UserCost:     userCost,
        Commission:   commission,
    }

    mp.client.ReportUsage(supplyID, usage)
}
```

### Overall Architecture
```json
// 04_API_CONTRACTS.md:98-128
POST /v1/marketplace/usage
Request:
{
  "request_id": "uuid",
  "quote_id": "q-abc123",
  "supply_id": "sup-1",
  "model": "qwen-3-70b-instruct",
  "prompt_tokens": 1000,
  "completion_tokens": 2000,
  "latency_ms": 780,
  "status": "ok",
  "user_id": "acct_xxx",
  "source": "consumer"
}

Response:
{
  "transaction_id": "tx-789",
  "gmv_usd": 0.02286,
  "commission_usd": 0.00114,
  "commission_rate": 0.05,
  "supplier_payout_usd": 0.02172,
  "user_charge_usd": 0.02400
}
```

**对比分析：**

| 字段 | Scheduling | Overall Arch | 对齐度 |
|------|------------|--------------|--------|
| Quote ID | ❌ 缺失 | ✅ `quote_id` | ⚠️ 需补充 |
| Prompt/Completion tokens | ⚠️ 合并为 TotalTokens | ✅ 分开 | ⚠️ 需拆分 |
| Source | ❌ 缺失 | ✅ consumer/provider/proxy | ⚠️ 需补充 |
| GMV | ✅ `SupplierCost` | ✅ `gmv_usd` | 100% |
| Commission | ✅ 计算逻辑相同 | ✅ `commission_usd` | 100% |
| Commission Rate | ⚠️ 硬编码 1.05 | ✅ 字段返回 | ⚠️ 需动态 |

**建议调整：**

```go
// 更新后的 UsageReport（兼容 04_API_CONTRACTS.md）
type UsageReport struct {
    RequestID        string    // 新增
    QuoteID          string    // 新增：关联 Quote
    SupplyID         string
    Model            string
    PromptTokens     int       // 拆分：原 TotalTokens
    CompletionTokens int       // 拆分：原 TotalTokens
    LatencyMs        int
    Status           string    // ok | timeout | error
    ErrorCode        string    // 新增
    UserID           string
    Source           string    // 新增：consumer | provider | proxy
    Timestamp        time.Time
}

func (mp *MarketplaceProvider) ReportUsage(ctx context.Context, report *UsageReport) (*TransactionResult, error) {
    // POST /v1/marketplace/usage
    // 返回 GMV/Commission/TransactionID
}
```

---

## 5. 执行窗口与启动窗口对齐度：70%

### Scheduling Design
```
Quote 过期时间：未明确区分
执行超时：未明确定义
```

### Overall Architecture
```
// overall_workflow.md:89-103
启动窗口（expires_at）：30-120秒，限制"开始执行"的时间
执行窗口（exec_timeout_sec）：15-30分钟，限制"单次执行"的最长时间

关键设计：
- Quote 过期不影响已启动的执行
- 执行令牌包含 exec_deadline，与 Quote TTL 分离
- 长推理场景（如 o1）需要独立的执行窗口保障
```

**差异点：**
Scheduling 文档未区分这两个概念，可能导致长耗时推理被中断。

**建议补充：**

在 `06_MARKETPLACE_INTEGRATION.md` 中添加：

```markdown
## 执行窗口设计

### 启动窗口（Quote Expiration）
- **用途**: 限制开始执行的时间窗口
- **时长**: 30-120秒（可配置）
- **过期行为**: 必须重新 Quote，但不影响已启动的执行

### 执行窗口（Execution Timeout）
- **用途**: 单次推理允许的最长执行时间
- **时长**: 15-30分钟（按模型配置，o1 等长推理模型可更长）
- **实现**: 执行令牌包含 `exec_deadline` 字段
- **验签**: Provider 只检查执行截止时间，不检查 Quote TTL

### 代码示例
```go
type SupplyQuote struct {
    ExpiresAt      time.Time // 启动窗口：必须在此之前开始执行
    ExecTimeoutSec int       // 执行窗口：单次执行最长时间（秒）
}

// Gateway 执行逻辑
func (mp *MarketplaceProvider) Execute(quote *SupplyQuote, req *Request) (*Response, error) {
    // 检查启动窗口
    if time.Now().After(quote.ExpiresAt) {
        return nil, errors.New("quote expired, need re-quote")
    }

    // 设置执行截止时间（与 Quote 过期分离）
    execDeadline := time.Now().Add(time.Duration(quote.ExecTimeoutSec) * time.Second)
    ctx, cancel := context.WithDeadline(context.Background(), execDeadline)
    defer cancel()

    // 执行请求（长推理不受 Quote TTL 影响）
    return mp.executeWithToken(ctx, quote.Endpoint, quote.SignedToken, req)
}
```

---

## 6. 验签机制对齐度：60%

### Scheduling Design
```
未详细定义验签流程
仅提到 signed_token 概念
```

### Overall Architecture
```
// 04_API_CONTRACTS.md:19-44
JWT 格式，Header 含 kid
Payload: iss/aud/supply_id/quote_id/request_id/exp/nbf/exec_deadline/max_tokens_cap/commission_rate/supplier_price_per_mtoken

验证流程（Provider 侧）:
1. 校验签名（按 kid 找 JWKS 公钥或对称密钥）
2. 校验 iss/aud/supply_id 匹配
3. 校验 nbf/exp/exec_deadline 未过期
4. 可选：重放保护（记录 request_id/nonce）
5. 可选：扣减 max_tokens_cap

公钥分发：JWKS URL + kid 轮换
信道加固：mTLS（双向证书）+ 签名
```

**差异点：**
Scheduling 文档完全未涉及验签细节，这在实施 Provider Gateway 时是必需的。

**建议：**
不阻塞当前 Gateway 实施，但需在后续 Provider 集成时补充：

1. 在 `06_MARKETPLACE_INTEGRATION.md` 添加"验签流程"章节
2. 参考 `04_API_CONTRACTS.md` 定义 JWT payload 字段
3. Gateway 实施时可先用简单的 HMAC 验签，后续升级为 JWKS

---

## 7. 计量可信度（Source 字段）对齐度：50%

### Scheduling Design
```
未提及计量来源（consumer vs provider vs proxy）
默认假设 Gateway 上报是可信的
```

### Overall Architecture
```
// 04_API_CONTRACTS.md:113-137
source 字段：consumer | provider | proxy

计量可信度优先级：
1. provider 上报（最可信）
2. proxy 上报（Marketplace 侧车计量）
3. consumer 上报（需限额/押金/信用控制）

对账策略：
- 双侧上报比对，差异报警
- consumer-only 场景需风控保护
- 采样审计
```

**差异点：**
Scheduling 未考虑供应商虚报风险和双侧计量对账。

**建议：**
在 MVP 阶段可先实现 consumer 上报（Gateway → Marketplace），后续迭代加入：
1. Provider 上报支持（供应商侧 Gateway 也上报 Usage）
2. 双侧对账逻辑
3. Proxy/侧车计量（可选）

**短期调整（非阻塞）：**
在 `UsageReport` 中添加 `Source` 字段，默认值 "consumer"：

```go
type UsageReport struct {
    Source string // "consumer" | "provider" | "proxy"
    // ... 其他字段
}
```

---

## 8. 数据模型对齐度：90%

### Scheduling Design
```
未明确定义数据库 schema
仅在测试计划中提到 token_metadata 表
```

### Overall Architecture
```sql
// 05_DATA_MODEL.md
accounts (买家/卖家)
supplies (供给注册)
quotes (请求级选型记录)
usage_events (原始用量)
transactions (GMV + Commission)
health_metrics (供应商健康度)
```

**对齐建议：**
Scheduling 实施时直接采用总体架构的数据模型，无需重新设计。

---

## 9. 测试用例对齐度：100%

### Scheduling Design
```
TC-P3-BILLING-001: 交易佣金计算（5%）
TC-P3-BILLING-002: GMV 对账与结算
TC-P3-BILLING-003: 无 API key 拒绝计费

TC-P3-ROUTING-001: 价格优化选择
TC-P3-ROUTING-002: 延迟 vs 价格权衡
TC-P3-ROUTING-003: 多区域选择
TC-P3-ROUTING-004: 吞吐优化（负载）
```

### Overall Architecture
```
无具体测试用例定义
```

**✅ Scheduling 测试覆盖更完善** - 可直接使用，无需调整

---

## 10. 优先级调度对齐度：100%

### Scheduling Design
```
5级优先队列：Critical/High/Medium/Low/Background
容量守护：internal >= 90% 时阻断外部
LLM 保护：上下文长度/速率限制/内容过滤
```

### Overall Architecture
```
// 02_GATEWAY_DESIGN.md:76-81
优先级调度：5级队列，支持 WFQ/AtLeast
容量守护：internal >= 90% 时阻断外部
LLM 保护：上下文长度限制、速率限制（防滥用）
```

**✅ 完全一致** - 无需调整

---

## 总结：需要调整的地方

### 🔴 高优先级（建议立即调整，阻塞实施）

**无** - 核心逻辑无冲突，可立即开始实施

### 🟡 中优先级（建议在实施前统一，不阻塞启动）

1. **API 接口对齐**（3-5 小时工作量）
   - 将 `selectBestSupply()` 改为调用 `POST /v1/marketplace/quote`
   - 更新 `Supply` 结构为 `SupplyQuote`，补充字段：
     - `quote_id`
     - `endpoint`
     - `signed_token`
     - `supplier_price_per_mtoken`
     - `commission_rate`
     - `expires_at`
     - `exec_timeout_sec`

   **位置**: `docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md:740-918`

2. **计费接口对齐**（2-3 小时工作量）
   - `reportUsage()` 改为调用 `POST /v1/marketplace/usage`
   - 更新 `Usage` 结构为 `UsageReport`，补充字段：
     - `request_id`
     - `quote_id`
     - `prompt_tokens` / `completion_tokens`（拆分 TotalTokens）
     - `source`（默认 "consumer"）
     - `error_code`

   **位置**: `docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md:843-918`

3. **执行窗口文档补充**（1-2 小时工作量）
   - 添加"启动窗口 vs 执行窗口"说明
   - 更新代码示例，展示长推理保障

   **位置**: `docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md` 新增章节

### 🟢 低优先级（后续迭代，不影响 MVP）

4. **验签机制补充**（后续 Provider 集成时）
   - 补充 JWT/JWKS 验签流程文档
   - 定义 signed_token payload 字段

   **位置**: `docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md` 或独立文档

5. **计量可信度（Source）**（后续迭代）
   - 补充双侧上报对账逻辑
   - 定义 provider/proxy 上报流程

   **位置**: 新增 `USAGE_RECONCILIATION.md`

6. **Region vs Load 评分融合**（后续优化）
   - 将 Load (5%) 和 Region 评分合并
   - 调整权重分配

   **位置**: `docs/scheduling/final/06_MARKETPLACE_INTEGRATION.md:740-841`

---

## 实施建议

### 方案 A：先统一术语，再实施（推荐）

**优点**: 代码与文档完全一致，减少后期重构
**工作量**: 6-10 小时（更新文档 + 测试用例）
**时间线**: 1-2 天

**步骤**:
1. 更新 `06_MARKETPLACE_INTEGRATION.md` API 接口（3h）
2. 更新测试用例字段（2h）
3. 补充执行窗口文档（1h）
4. 验证一致性（1h）
5. 开新分支实施

### 方案 B：直接实施，边改边对齐

**优点**: 立即开始，快速迭代
**缺点**: 可能需要后期重构部分代码
**适用**: 急需验证核心逻辑

**步骤**:
1. 按现有 scheduling 文档开发 MarketplaceProvider
2. 实施时直接对接 `04_API_CONTRACTS.md` 的 Quote/Usage API
3. 遇到差异时以总体架构为准，同步更新文档

---

## 推荐方案

**建议采用方案 A**，原因：
1. 工作量不大（1-2 天）
2. 避免后期返工
3. 确保团队理解一致
4. 文档作为单一可信源（Single Source of Truth）

**具体执行**:
1. ✅ 今天：我更新 `06_MARKETPLACE_INTEGRATION.md` 对齐 API 接口
2. ✅ 今天：补充执行窗口说明
3. ✅ 明天：验证一致性，生成最终 checklist
4. ✅ 明天下午：开新分支，开始实施 scheduling 功能

---

## 最终结论

**✅ Scheduling 设计与总体架构高度对齐（90%+）**

**核心逻辑完全一致**：
- 商业模式：5% 佣金 ✅
- 路由算法：多维度打分 ✅
- 计费拆账：GMV + Commission ✅
- 调度保护：5级队列 + 容量守护 ✅

**术语差异可快速统一**（1-2 天）：
- API 字段命名
- 执行窗口概念
- 计量来源标识

**建议**：
**先花 1-2 天统一术语和接口定义，然后开新分支实施。这样可以确保代码与文档 100% 一致，避免后期返工。**

---

**Next Steps:**
1. 决定采用方案 A 或 方案 B
2. 如果方案 A，我立即开始更新文档
3. 如果方案 B，直接开新分支，边实施边对齐

**你的决定？**
