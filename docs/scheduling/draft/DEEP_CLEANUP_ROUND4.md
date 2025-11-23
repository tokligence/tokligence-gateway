# Deep Cleanup Round 4 - Transaction Commission Model

**Date:** 2025-11-23
**Status:** 🔄 IN PROGRESS
**Type:** Deep technical cleanup for pay-as-you-go commission model

---

## User Feedback Summary

用户审查后发现关键问题：

> "已查阅新增/更新的文档，核心问题是商业模式已切换为'纯 GMV 抽佣 5%（类似 Stripe/广告 DSP）'，但若干文档和配置仍保留'free tier / paid tier / API key 升级'等订阅式残留，且技术路线尚未充分体现'按交易计费、按报价/延迟/吞吐做动态路由'。"

### 关键问题

1. **订阅模式残留**: 大量 "free tier 100 req/day", "paid tiers", "upgrade prompts" 引用
2. **技术实现缺失**: 缺少多维度路由选择（价格/延迟/吞吐）的实现
3. **计费逻辑不清**: 5%佣金计算和对账流程未明确
4. **测试用例过时**: 测试计划仍针对订阅套餐而非交易佣金

---

## Round 4 Cleanup Scope

### Phase 1: Deep Clean 06_MARKETPLACE_INTEGRATION.md ✅ COMPLETED

#### Changes Made:

**1. Configuration Examples (Lines 280-291)**
```diff
Before:
- # Authentication (optional - free tier works without key)
- # Free tier: 100 requests/day without API key
- # Paid tiers require API key
- api_key_env = TOKLIGENCE_MARKETPLACE_API_KEY
- free_tier_limit = 100  # requests per day

After:
+ # Authentication (required for billing identity)
+ # API key identifies your account for transaction billing (5% commission)
+ # Get your API key from https://marketplace.tokligence.com/dashboard
+ api_key_env = TOKLIGENCE_MARKETPLACE_API_KEY
+
+ # Rate limiting (abuse prevention only, NOT billing tiers)
+ # Default: 100 RPS per account (adjustable based on usage patterns)
+ rate_limit_rps = 100  # requests per second (anti-abuse)
```

**2. Pricing Model Section (Lines 321-347)**
```diff
Before:
- ## 2.4 Freemium Pricing Model
- Free Tier (No API Key Required)
-   • 100 marketplace requests/day
-   • Community support
- Pro Tier ($49/month) - OBSOLETE
- Business Tier ($199/month) - OBSOLETE

After:
+ ## 2.4 Transaction Commission Pricing Model
+ Pay-as-you-go Transaction Commission
+   • NO monthly subscription fees
+   • NO request limits (unlimited usage)
+   • 5% commission on marketplace transactions only
+   • Direct provider use: 0% commission (free routing)
+
+ Pricing Example:
+   Supplier price: $100 for 1M tokens
+   User pays: $105 (supplier × 1.05)
+   Supplier gets: $100
+   Tokligence gets: $5 (5% commission)
+
+ Value Proposition:
+   vs OpenAI direct ($200): Save $95 (47.5% cheaper)
+   Commission cost: $5
+   Net savings: $90 (45% total savings)
+   ROI: 18x (save $90, pay $5)
```

**3. API Client Code (Lines 698-722)**
```diff
Before:
- // Add API key if available (paid tier)
- if mc.apiKey != "" {
-     httpReq.Header.Set("Authorization", "Bearer "+mc.apiKey)
- }
- // If no API key, marketplace API will enforce free tier limits (100 req/day)
-
- // Check for rate limit errors (free tier exceeded)
- if resp.StatusCode == 429 {
-     log.Warn("Free tier limit exceeded. Upgrade at: "+errResp.Upgrade)
-     return nil, fmt.Errorf("marketplace free tier limit exceeded (100 req/day). Upgrade at: %s", errResp.Upgrade)
- }

After:
+ // Add API key for billing identity (required for transaction commission)
+ if mc.apiKey != "" {
+     httpReq.Header.Set("Authorization", "Bearer "+mc.apiKey)
+ } else {
+     return nil, fmt.Errorf("marketplace API key required for billing (set TOKLIGENCE_MARKETPLACE_API_KEY)")
+ }
+
+ // Check for rate limit errors (anti-abuse, NOT billing tiers)
+ if resp.StatusCode == 429 {
+     log.Warn("Rate limit exceeded (anti-abuse). Retry after: %d seconds", errResp.RetryAfter)
+     return nil, fmt.Errorf("marketplace rate limit exceeded (anti-abuse): %s", errResp.Message)
+ }
```

**4. Added Multi-Dimensional Routing Implementation (Lines 740-841)**

NEW FUNCTION ADDED:
```go
// selectBestSupply implements multi-dimensional routing selection
// Similar to ad DSP allocation, but optimized for latency/throughput/cost instead of CTR
func (mp *MarketplaceProvider) selectBestSupply(supplies []*Supply, req *provider.Request) *Supply {
    // Multi-dimensional scoring:
    // Score = w1*PriceScore + w2*LatencyScore + w3*ThroughputScore + w4*AvailabilityScore
    //
    // Similar to ad DSP's eCPM = bid × pCTR, we use:
    // EffectiveCost = Price / (Throughput × Availability × (1 - NormalizedLatency))

    for _, s := range supplies {
        // Price score (lower is better): normalize to [0, 1]
        maxPrice := 30.0  // OpenAI gpt-4 price as baseline
        priceScore := 1.0 - (s.PricePerMToken / maxPrice)

        // Latency score (lower P99 is better): normalize to [0, 1]
        maxLatencyMs := 5000.0
        latencyScore := 1.0 - (float64(s.P99LatencyMs) / maxLatencyMs)

        // Throughput score (higher is better): normalize to [0, 1]
        maxThroughput := 50000.0
        throughputScore := float64(s.AvailableTokensPS) / maxThroughput

        // Availability score (higher is better): already in [0, 1]
        availabilityScore := s.Availability

        // Load score (lower is better): invert current load
        loadScore := 1.0 - s.CurrentLoad

        // Weighted sum (configurable weights)
        // Default: prioritize price (40%), latency (30%), availability (20%), throughput (10%)
        weights := struct {
            price        float64  // 0.40
            latency      float64  // 0.30
            throughput   float64  // 0.10
            availability float64  // 0.15
            load         float64  // 0.05
        }{...}

        finalScore := weights.price*priceScore +
            weights.latency*latencyScore +
            weights.throughput*throughputScore +
            weights.availability*availabilityScore +
            weights.load*loadScore

        scored = append(scored, scoredSupply{supply: s, score: finalScore})
    }

    // Sort by score descending (highest score = best supply)
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].score > scored[j].score
    })

    return scored[0].supply
}
```

**Key Features:**
- 5-dimensional scoring: price, latency, throughput, availability, load
- Configurable weights (default: 40% price, 30% latency, 15% availability, 10% throughput, 5% load)
- Normalized scores [0, 1] for fair comparison
- Inspired by ad DSP's eCPM model, adapted for LLM routing

**5. Added Transaction Billing Implementation (Lines 843-918)**

NEW FUNCTION ADDED:
```go
// reportUsage reports transaction usage for 5% commission billing
func (mp *MarketplaceProvider) reportUsage(supplyID string, req *provider.Request, resp *provider.Response) {
    // Calculate costs:
    // 1. Get supplier's base price
    supply := mp.cache.GetSupply(supplyID)

    // 2. Calculate supplier cost (their price × tokens used)
    tokensUsed := resp.Usage.TotalTokens
    supplierCost := (float64(tokensUsed) / 1_000_000.0) * supply.PricePerMToken

    // 3. Calculate user cost (supplier cost × 1.05)
    userCost := supplierCost * 1.05

    // 4. Calculate our commission (5%)
    commission := userCost - supplierCost

    // 5. Report to marketplace API for billing
    usage := &Usage{
        SupplyID:     supplyID,
        UserID:       req.UserID,
        TokensUsed:   tokensUsed,
        SupplierCost: supplierCost,  // What supplier gets
        UserCost:     userCost,      // What user pays
        Commission:   commission,     // Our 5% take
        Timestamp:    time.Now(),
        RequestID:    req.RequestID,
    }

    err := mp.client.ReportUsage(supplyID, usage)
    if err != nil {
        log.Error("Failed to report usage for billing: %v", err)
        // TODO: Retry with exponential backoff, or queue for later
    }

    log.Info("Transaction recorded: supply=%s, tokens=%d, supplier=$%.4f, user=$%.4f, commission=$%.4f (5%%)",
        supplyID, tokensUsed, supplierCost, userCost, commission)
}

// Updated ReportUsage API endpoint
func (mc *MarketplaceClient) ReportUsage(supplyID string, usage *Usage) error {
    // POST /v1/billing/transactions
    // {
    //   "supply_id": "supply_123",
    //   "user_id": "user_456",
    //   "tokens_used": 1500,
    //   "supplier_cost": 0.0075,    // $7.50 per Mtok × 0.0015M = $0.0075
    //   "user_cost": 0.007875,       // $0.0075 × 1.05 = $0.007875
    //   "commission": 0.000375,      // $0.007875 - $0.0075 = $0.000375 (5%)
    //   "timestamp": "2025-02-01T12:00:00Z",
    //   "request_id": "req_abc123"
    // }
    //
    // Marketplace API will:
    // 1. Credit supplier account: +$0.0075
    // 2. Debit user account: -$0.007875
    // 3. Credit Tokligence account: +$0.000375 (5% commission)
    return mc.client.Post(mc.endpoint+"/v1/billing/transactions", usage)
}
```

**Key Features:**
- Precise commission calculation (5% on user cost)
- Three-way accounting: user pays, supplier gets, we take commission
- Async reporting with error handling (TODO: retry queue)
- Detailed logging for audit trail

**6. Roadmap Phases (Lines 1637-1700)**
```diff
Before:
- ### 7.1 Phase 0: Open-Source Core + Free Tier MVP
- Week 7-8:   MarketplaceProvider (opt-in)
-             ├─ Supply discovery (100 req/day free)
-             ├─ Rate limit enforcement
-             ├─ Upgrade prompts
-             └─ Disabled by default (opt-in)
-
- Release:    v0.1.0 (Apache 2.0 + Opt-In Free Tier)
-
- ### 7.2 Phase 1: Paid Tier Features
- Week 11-14: Marketplace API (backend)
-             ├─ Supply discovery API
-             ├─ Health monitoring API
-             ├─ Usage reporting API
-             ├─ Billing/settlement API
-             └─ Rate limiting (free vs paid tiers)

After:
+ ### 7.1 Phase 0: Open-Source Core Gateway
+ Week 7-8:   MarketplaceProvider (opt-in)
+             ├─ Supply discovery API client
+             ├─ Multi-dimensional routing (price/latency/throughput)
+             ├─ Transaction billing integration
+             └─ Disabled by default (opt-in)
+
+ Release:    v0.1.0 (Apache 2.0 + Opt-In Marketplace)
+             - 5% commission on transactions (no subscription)
+
+ ### 7.2 Phase 1: Marketplace Backend + Transaction Billing
+ Week 11-14: Marketplace API (backend)
+             ├─ Supply discovery API
+             ├─ Health monitoring API
+             ├─ Supplier pricing/SLA API
+             ├─ Transaction billing API (5% commission)
+             └─ Rate limiting (anti-abuse only, NOT billing tiers)
```

**7. Marketing Message (Lines 1723-1733)**
```diff
Before:
- Works standalone with LocalProvider, or opt-in to Tokligence Marketplace (100 free requests/day) for dynamic capacity.
- ✅ Free tier (100 req/day) available when you opt-in

After:
+ Works standalone with LocalProvider, or opt-in to Tokligence Marketplace for 40-60% cost savings (pay-as-you-go, 5% commission).
+ ✅ Pay-as-you-go: 5% commission on transactions (no subscription, no limits)
```

**8. Model 2.5 Description (Lines 1776-1785)**
```diff
Before:
- **Model 2.5: Include Plugin, Disabled by Default (Free Tier Available)** ✅✅ **CHOSEN**
- Free tier works without API key (just set `enabled = true`)

After:
+ **Model 2.5: Include Plugin, Disabled by Default (Pay-as-you-go)** ✅✅ **CHOSEN**
+ API key required for billing identity (5% commission on transactions)
+ **Fair pricing:** Pay only for what you use (5% commission)
```

**9. Monetization Strategy (Lines 1813-1825)**
```diff
Before:
- 1. **Transaction commission** (pay-as-you-go):
-    - ❌ ~~Free: 100 req/day~~ (DELETED)
-    - ❌ ~~Pro: $49/month~~ (DELETED)
-    - ❌ ~~Business: $199/month~~ (DELETED)
-    - ✅ **5% commission on all marketplace transactions**
-
- 2. **Transaction fees** (marketplace purchases):
-    - ❌ ~~Pro tier: 3% fee~~ (DELETED)
-    - ❌ ~~Business tier: 5% fee~~ (DELETED)
-    - ✅ **Flat 5% commission for all users** (no tiers)

After:
+ 1. **Transaction commission** (pay-as-you-go, 5% on GMV):
+    - ✅ **Flat 5% commission on all marketplace transactions**
+    - Example: Supplier price $100 → User pays $105 → We get $5
+    - No monthly fees, no usage limits, no tiers
+    - Enterprise: Custom contracts with negotiable rates
+
+ 2. **Enterprise add-ons** (optional, not required):
+    - Private marketplace (one-time setup fee: $10K-$50K)
+    - White-label deployment (annual license: $50K/year)
+    - SLA guarantees 99.99% (commission rate increases to 7%)
+    - Dedicated support (annual retainer: $20K/year)
```

**10. Implementation Checklist (Lines 1917-1940)**
```diff
Before:
- [ ] Implement free tier rate limiting (100 req/day)
- [ ] Add upgrade prompts (when free tier exceeded)
- [ ] Set up Stripe for paid tiers
- [ ] Test upgrade flow (free → Pro → Business)
- [ ] Explain free vs paid tiers
- [ ] Write blog post: "Why we chose Apache 2.0 + freemium"

After:
+ [ ] Implement multi-dimensional routing (price/latency/throughput scoring)
+ [ ] Implement transaction billing API (5% commission calculation)
+ [ ] Set up Stripe for transaction billing (not subscriptions)
+ [ ] Test transaction flow (discovery → routing → billing → settlement)
+ [ ] Explain pay-as-you-go commission model (5%)
+ [ ] Document multi-dimensional routing algorithm
+ [ ] Write blog post: "Why we chose Apache 2.0 + pay-as-you-go commission"
+ [ ] Highlight cost savings vs OpenAI (40-60%)
```

**11. Added Transaction Flow Diagram (Lines 42-114)** ✅ NEW

Complete flow diagram showing:
1. User request → Marketplace discovery
2. Multi-supplier quotes received
3. Multi-dimensional scoring (price 40%, latency 30%, etc.)
4. Best supplier selected
5. Request routed to supplier
6. Response received with token usage
7. Commission calculated (5%)
8. Billing API called
9. Three-way settlement (user pays, supplier gets, we take commission)
10. Value proposition example (72% savings vs OpenAI)

---

## Summary of 06_MARKETPLACE_INTEGRATION.md Cleanup

### Files Changed: 1
### Lines Modified: ~200+ lines
### Sections Rewritten: 11

### Key Improvements:

**✅ Removed ALL subscription tier references**
- No more "free tier 100 req/day"
- No more "paid tiers ($49, $199/month)"
- No more "upgrade prompts"

**✅ Added multi-dimensional routing logic**
- 5-dimensional scoring algorithm
- Configurable weights
- Inspired by ad DSP eCPM model

**✅ Added transaction billing logic**
- 5% commission calculation
- Three-way accounting (user/supplier/us)
- Async reporting with error handling

**✅ Added comprehensive flow diagram**
- Step-by-step transaction flow
- Billing calculation example
- Value proposition (ROI calculation)

**✅ Updated all documentation sections**
- Configuration examples
- Code examples
- Roadmap phases
- Marketing messages
- Implementation checklist

---

## Phase 2: Remaining Work (TODO)

### 2. COMMERCIAL_STRATEGY_ANALYSIS.md
**Issues to fix:**
- Action items checklist still references "free tier / paid tier"
- Routing examples still have "Stripe for paid tiers"
- Key metrics table still tracks "free tier usage量"

**Required changes:**
- Update action items: remove "Implement free tier" → add "Implement commission billing"
- Update routing examples: remove tier checks → show direct routing
- Update metrics: GMV, commission rate, cost savings (not free tier usage)

### 3. 07_MVP_ITERATION_AND_TEST_PLAN.md
**Issues to fix:**
- Phase 3 has "free tier enforcement / upgrade prompts" test cases
- Missing tests for "transaction commission / GMV accounting"

**Required changes:**
- Delete: TC-P3-TIER-001 "Free tier limit enforcement"
- Delete: TC-P3-UPGRADE-001 "Upgrade prompt display"
- Add: TC-P3-BILLING-001 "Transaction commission calculation"
- Add: TC-P3-BILLING-002 "GMV accounting and settlement"
- Add: TC-P3-ROUTING-001 "Multi-dimensional supplier selection"

### 4. 00_REVISED_OVERVIEW.md
**Issues to fix:**
- May have references to "Marketplace free tier / paid tier"
- API key upgrade mentions

**Required changes:**
- Replace: "free tier / paid tier" → "pay-as-you-go (5% commission)"
- Replace: "API key for tier upgrade" → "API key for billing identity"

### 5. Consistency Documents
**Files to update:**
- CONSISTENCY_UPDATE_V2.1_COMPLETE.md
- BUSINESS_MODEL_FIX_SUMMARY.md
- BUSINESS_MODEL_VERIFICATION_REPORT.md

**Required changes:**
- Update to reflect deep technical cleanup
- Mark remaining "free tier" mentions as resolved
- Add verification of multi-dimensional routing implementation
- Add verification of transaction billing implementation

---

## Technical Design Completeness

### ✅ Implemented

1. **Multi-Dimensional Routing**
   - Price normalization (baseline: OpenAI $30/Mtok)
   - Latency normalization (max acceptable: 5000ms)
   - Throughput normalization (max: 50K tok/s)
   - Availability score (0-1 range)
   - Load score (inverted)
   - Weighted scoring (configurable)

2. **Transaction Billing**
   - Commission calculation (5%)
   - Three-way accounting
   - Usage struct with all required fields
   - Billing API endpoint design
   - Error handling and retry logic (TODO)

3. **Flow Diagrams**
   - Complete transaction flow (10 steps)
   - Billing calculation example
   - Value proposition demonstration

### ⚠️ Still Missing

1. **Region-Aware Routing**
   - Multi-region latency optimization
   - Geographic affinity scoring
   - Cross-region fallback

2. **Risk Controls (No Subscription Barriers)**
   - Credit-based limits (prepaid balance)
   - Transaction-based fraud detection
   - IP-based rate limiting (anti-abuse)
   - Blacklist/whitelist management

3. **Metrics & Monitoring**
   - GMV tracking
   - Commission revenue tracking
   - Cost savings metrics (vs OpenAI baseline)
   - Supplier performance metrics (price/latency/uptime)

4. **Test Cases**
   - Multi-supplier competitive bidding tests
   - Commission calculation correctness tests
   - GMV accounting tests
   - Routing optimization tests

---

## Next Steps

1. ✅ **Complete 06_MARKETPLACE_INTEGRATION.md cleanup** (DONE)
2. ⏳ **Clean COMMERCIAL_STRATEGY_ANALYSIS.md** (IN PROGRESS)
3. ⏳ **Update 07_MVP_ITERATION_AND_TEST_PLAN.md**
4. ⏳ **Clean 00_REVISED_OVERVIEW.md**
5. ⏳ **Update consistency documents**
6. ⏳ **Add missing sections:**
   - Region-aware routing design
   - Risk controls for pay-as-you-go
   - Metrics dashboard specification

---

## Verification Checklist

After all cleanup is complete, verify:

- [ ] Zero references to "free tier 100 req/day" (except OBSOLETE markers)
- [ ] Zero references to "paid tier / Pro tier / Business tier" (except OBSOLETE markers)
- [ ] Zero references to "upgrade prompts"
- [ ] All API key references clarify "for billing identity, not tier access"
- [ ] All rate limiting references clarify "anti-abuse, not billing tiers"
- [ ] Multi-dimensional routing algorithm documented
- [ ] Transaction billing flow documented
- [ ] Commission calculation (5%) documented
- [ ] GMV-based revenue metrics documented
- [ ] Test cases updated for transaction model

---

**Status:** Round 4 cleanup in progress. Phase 1 (06_MARKETPLACE_INTEGRATION.md) complete.

**Estimated remaining work:** 4-5 more documents to clean.

**Target:** Complete, production-ready documentation with no subscription remnants.
