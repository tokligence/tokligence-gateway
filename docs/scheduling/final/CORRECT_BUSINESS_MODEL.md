# Tokligence Marketplace 正确商业模式

**Version:** 1.0
**Date:** 2025-11-23
**Status:** AUTHORITATIVE - 覆盖所有之前的错误设计
**Type:** 商业模式定义

---

## ⚠️ 重要声明

**之前文档中所有关于"subscription"、"$49/month"、"Pro tier"等内容全部作废！**

**正确的商业模式:**
- ✅ **Pay-as-you-go + 交易佣金 (5%)**
- ❌ ~~订阅费用~~
- ❌ ~~免费额度100 req/天~~
- ❌ ~~Pro/Business/Enterprise套餐~~

---

## 1. 核心定位

### 我们是什么

```
Tokligence = 新一代 AI Token 管道

职责:
  1. 开源LLM Gateway (Apache 2.0)
     - 协议转换 (OpenAI ↔ Anthropic)
     - 优先级调度
     - 速率限制、配额管理
     - LLM防护层

  2. Marketplace (opt-in SaaS)
     - 连接用户和GPU供应商
     - 帮用户找最便宜的供应商
     - 自动failover (高可用)
     - 从每笔交易抽成
```

### 类比

```
Tokligence : LLM API :: Stripe : 支付

Stripe:
  - 不收月费
  - 只收交易手续费 (2.9% + $0.30)
  - 用多少付多少

Tokligence:
  - 不收月费 ❌
  - 只收交易佣金 (5%) ✅
  - 用多少付多少 ✅
```

---

## 2. 唯一收入来源：交易佣金 (5%)

### 工作原理

```
用户请求流程:
  1. 用户安装 tokligence-gateway (开源免费)
  2. 默认使用 LocalProvider (自己的API key)
  3. 可选启用 Marketplace (opt-in)

启用Marketplace后:
  Step 1: 用户发请求 "Generate 1M tokens"
  Step 2: Marketplace查询供应商报价
    - SupplierA: $10/1M tokens (OpenAI compatible)
    - SupplierB: $8/1M tokens  (便宜20%)
    - SupplierC: $12/1M tokens

  Step 3: Marketplace推荐SupplierB ($8)
  Step 4: 用户实际支付 = $8 × 1.05 = $8.40
    ├─ SupplierB收到: $8.00
    └─ Tokligence收到: $0.40 (5%佣金)

  Step 5: 请求发送到SupplierB，返回结果

用户价值:
  - 直接用OpenAI: $30/1M tokens
  - 通过Marketplace: $8.40/1M tokens
  - 节省: $21.60 (72% cheaper!)
  - 佣金成本: $0.40
  - 净节省: $21.20
```

### 收入公式

```
每笔交易:
  Commission = Transaction Amount × 5%

月收入:
  MRR = Total GMV/month × 5%

年收入:
  ARR = Total GMV/year × 5%
```

---

## 3. 财务模型 (3年预测)

### Year 1: 种子期

```
活跃用户: 1,000
平均月消费: $200/用户 (LLM API费用)
月GMV: $200K
年GMV: $2.4M

收入:
  佣金率: 5%
  月收入: $200K × 5% = $10K MRR
  年收入: $2.4M × 5% = $120K ARR ✅

关键指标:
  - Take rate: 5%
  - 用户LTV: $1,200 (avg $200/mo × 6 months)
  - CAC: $100 (content marketing)
  - LTV:CAC = 12:1
```

### Year 2: 增长期

```
活跃用户: 5,000
平均月消费: $300/用户
月GMV: $1.5M
年GMV: $18M

收入:
  月收入: $1.5M × 5% = $75K MRR
  年收入: $18M × 5% = $900K ARR ✅

关键指标:
  - Take rate: 5%
  - 供应商数量: 50+
  - 平均节省: 50% vs OpenAI
```

### Year 3: 规模化

```
活跃用户: 20,000
平均月消费: $500/用户
月GMV: $10M
年GMV: $120M

收入:
  月收入: $10M × 5% = $500K MRR
  年收入: $120M × 5% = $6M ARR ✅

关键指标:
  - Take rate: 5%
  - 供应商数量: 200+
  - 平均节省: 60% vs OpenAI
```

---

## 4. 为什么5%佣金可行？

### 用户视角：愿意付5%

```
场景: AI创业公司

直接用OpenAI:
  gpt-4: $30/1M tokens
  月使用: 100M tokens
  月费用: $3,000

通过Tokligence Marketplace:
  找到便宜供应商: $12/1M tokens
  加上5%佣金: $12.60/1M tokens
  月使用: 100M tokens
  月费用: $1,260

用户算账:
  省了: $3,000 - $1,260 = $1,740 (58% cheaper!)
  佣金成本: $60
  净节省: $1,680

  ROI = $1,680 / $60 = 28x

  → 当然愿意付5%佣金!
```

### 供应商视角：愿意给5%

```
场景: GPU供应商 (小玩家)

没有Marketplace:
  - 自建网站
  - Google Ads (CPC $10, 转化率2%)
  - CAC = $500/客户
  - 每月获客: 10个
  - 获客成本: $5K/月
  - 客单价: $200/月
  - GMV: $2K/月

加入Marketplace:
  - 零获客成本 (Tokligence带来流量)
  - 每月获客: 100个
  - 客单价: $200/月 × 95% = $190/月
  - GMV: $19K/月
  - 净收入: $18K/月 (vs 之前-$3K亏损)

供应商算账:
  之前: $2K GMV - $5K 获客 = -$3K (亏损)
  现在: $19K GMV × 95% = $18K (净收入)

  增长: 10x 客户量

  → 愿意给5%! 因为带来大量订单
```

---

## 5. 企业增值服务 (可选附加收入 <10%)

### 不是订阅，是增值服务

| 服务 | 定价模式 | 说明 |
|------|---------|------|
| **Private Marketplace** | 一次性设置费 + 正常佣金 | $10K-$50K setup fee, 然后5%佣金 |
| **白标部署** | 年费 | $50K/年 (因为是特殊服务) |
| **SLA保证 (99.99%)** | 佣金加价 | 7%佣金 (vs 5%) |
| **专属技术支持** | 年费 | $20K/年 (Slack channel) |
| **自定义路由规则** | 一次性开发费 | $15K-$30K |

**关键:**
- 这些是**附加服务**，不是必须的
- 基础marketplace使用永远是 pay-as-you-go 5%佣金
- 企业客户如果需要特殊功能，才付额外费用

---

## 6. 对标: OpenRouter.ai

### OpenRouter 的模式

```
商业模式:
  ✅ Pay-as-you-go
  ✅ 纯交易佣金 (0-10%)
  ❌ 无订阅费
  ❌ 无免费额度限制

优点:
  - 门槛低 (用多少付多少)
  - 公平 (小用户付小钱，大用户付大钱)
  - 可扩展

缺点:
  - 只是API聚合器
  - 无法本地部署
  - 无调度、配额管理
```

### Tokligence 的差异化

```
相同点:
  ✅ Pay-as-you-go
  ✅ 纯交易佣金 (5%)
  ✅ 无订阅费

差异化:
  ✅ 完整的Gateway功能 (调度、配额、防护)
  ✅ 可以本地部署 (开源)
  ✅ Marketplace是opt-in (隐私友好)
  ✅ 企业级功能 (Private Marketplace, SLA)

定位:
  OpenRouter = "API aggregator"
  Tokligence = "AI Token Pipeline" (管道)
```

---

## 7. 错误设计清单 (需要删除)

### ❌ 以下内容全部错误，需要删除:

1. **订阅套餐**
   ```
   ❌ Free Tier: 100 req/天
   ❌ Pro Tier: $49/月, 10K req/天
   ❌ Business Tier: $199/月, Unlimited
   ❌ Enterprise Tier: Custom pricing
   ```

2. **免费额度限制**
   ```
   ❌ 100 requests/day for free users
   ❌ Upgrade to Pro for more requests
   ```

3. **订阅收入预测**
   ```
   ❌ "1000 users × $49/month = $49K MRR"
   ❌ "Subscription revenue: $1M ARR"
   ```

4. **订阅相关的代码/配置**
   ```ini
   ❌ [marketplace]
   ❌ tier = "free"  # or "pro", "business"
   ❌ daily_limit = 100
   ```

### ✅ 正确的设计:

1. **纯佣金模式**
   ```
   ✅ Pay-as-you-go only
   ✅ 5% commission on all transactions
   ✅ No monthly fees
   ✅ No usage limits (unlimited)
   ```

2. **简单配置**
   ```ini
   [provider.marketplace]
   enabled = false  # Disabled by default (opt-in)
   commission_rate = 0.05  # 5%
   ```

3. **收入预测**
   ```
   ✅ "GMV: $120M/year × 5% = $6M ARR"
   ✅ "Commission revenue: $6M ARR (Year 3)"
   ```

---

## 8. 文档修复优先级

### 高优先级 (必须立即修复):

1. ✅ `06_MARKETPLACE_INTEGRATION.md` - 删除所有订阅套餐
2. ✅ `COMMERCIAL_STRATEGY_ANALYSIS.md` - 重写收入模型
3. ✅ `00_REVISED_OVERVIEW.md` - 更新商业模式描述

### 中优先级 (尽快修复):

4. `07_MVP_ITERATION_AND_TEST_PLAN.md` - 删除订阅测试用例
5. `08_CODE_REPOSITORY_ARCHITECTURE.md` - 更新marketplace描述

### 低优先级 (可稍后):

6. `01_PRIORITY_BASED_SCHEDULING.md` - 更新对比表
7. 其他文档中零散的订阅引用

---

## 9. 关键消息 (Positioning)

### 对外宣传

```
标题: "新一代AI Token管道 - 省钱40-60%"

副标题: "开源Gateway + Marketplace，帮你找到最便宜的LLM供应商"

核心价值:
  1. 开源 (Apache 2.0) - 完全免费
  2. 省钱 - 比直接用OpenAI便宜40-60%
  3. 高可用 - 自动failover到备用供应商
  4. Pay-as-you-go - 用多少付多少，无月费

定价:
  - Gateway: 免费 (开源)
  - Marketplace: 5%交易佣金 (仅在使用时收费)
  - 无订阅费，无最低消费
```

### 竞争对比

| | Tokligence | OpenRouter | 直接用OpenAI |
|---|-----------|-----------|-------------|
| **Gateway功能** | ✅ 完整 | ❌ 无 | ❌ 无 |
| **本地部署** | ✅ 开源 | ❌ SaaS only | ❌ API only |
| **价格** | 5%佣金 | 0-10%佣金 | 官方价格 |
| **节省** | 40-60% | 30-50% | 0% |
| **月费** | ❌ $0 | ❌ $0 | ❌ $0 |

---

## 10. 行动清单

### 立即执行 (今天):

- [x] 创建本文档 (CORRECT_BUSINESS_MODEL.md)
- [ ] 修复 `06_MARKETPLACE_INTEGRATION.md`
- [ ] 修复 `COMMERCIAL_STRATEGY_ANALYSIS.md`
- [ ] 删除所有订阅套餐图表

### 本周完成:

- [ ] 修复所有文档中的订阅引用 (22处)
- [ ] 更新代码注释 (如果有订阅相关代码)
- [ ] 创建正确的定价页面文案

### 下周完成:

- [ ] Review marketplace仓库代码 (确保没有订阅逻辑)
- [ ] 更新README.md (主仓库)
- [ ] 更新官网文案 (如果有)

---

## 附录: 为什么之前的设计错了？

### 用户不会为"查价格"付订阅费

```
错误假设:
  "用户付$49/月，我帮你查哪个LLM便宜"

现实:
  - 用户: "我自己Google也能查到，为啥付$49?"
  - 用户: "我每月只用$20 LLM，付$49订阅费不划算"
  - 用户: "我试用一个月后不用了，还要取消订阅吗？"

正确做法:
  "你用我帮你省钱，我从你省的钱里抽5%"

  - 用户: "我省了$100，你抽$5，OK"
  - 用户: "我不用就不付，公平"
  - 用户: "用多少付多少，清晰透明"
```

### OpenRouter 已经证明了这个模式

```
OpenRouter 数据:
  - 月活用户: 50K+
  - 无订阅费
  - 纯交易佣金
  - 用户增长: 持续增长

如果订阅模式更好，OpenRouter早就改了。
事实是: Pay-as-you-go 才是正确的模式。
```

---

**总结: 删除所有订阅内容，改成5%交易佣金！** 🎯
