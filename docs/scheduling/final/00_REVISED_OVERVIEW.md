# Tokligence Gateway Scheduling System - Revised Overview

**Version:** 2.0 (Revised based on review feedback)
**Date:** 2025-02-01
**Status:** Design Documentation - Production-Ready Path

---

## What Changed (v2.0)

Based on comprehensive review feedback, we've made major changes:

### ✅ Fixed Issues

1. **Marketplace Integration** - Now designed as optional plugin (not core requirement)
2. **Resource Measurement** - Changed from RPS to tokens/sec + context + model-specific
3. **LLM Protection** - Added context length protection with hooks
4. **Simplified Defaults** - 5-level priority as default (not 100 buckets)
5. **Availability** - Added fail-open/fail-close strategies
6. **Observability** - Constrained metrics cardinality

### ❌ Removed/Deprecated

1. **100-bucket model** - Moved to experimental, NOT recommended for production
2. **RPS-only capacity** - Now secondary to tokens/sec
3. **Marketplace-first design** - Marketplace is now a plugin, not core

---

## Quick Start (New Users)

### For Standalone Deployment (Most Users)

**Goal:** Run gateway with self-hosted LLM

**Read:**
1. This overview (10 min)
2. `01_PRIORITY_BASED_SCHEDULING.md` (30 min)
3. `06_MARKETPLACE_INTEGRATION.md` §3 LocalProvider (15 min)

**Deploy:**
```bash
# Install
go get github.com/tokligence/gateway

# Configure
cat > config/gateway.ini << EOF
[providers]
enabled_providers = local
default_provider = local

[provider.local]
endpoint = http://localhost:8000
models = gpt-4,claude-3-sonnet
max_tokens_per_sec = 10000  # From benchmarking
max_context_length = 128000

[scheduler]
type = priority  # 5-level priority (default)
EOF

# Benchmark your LLM
tokligence benchmark --profile long_context

# Start
tokligence gateway start
```

### For Marketplace Users (Optional)

**Goal:** Buy/sell capacity on marketplace

**Privacy Notice:** Marketplace is **disabled by default** and requires explicit opt-in. When enabled, only metadata (model name, region) is sent to marketplace API - not your request content.

**Read:**
1. Standalone setup (above)
2. `06_MARKETPLACE_INTEGRATION.md` §2.5 Privacy Policy
3. `06_MARKETPLACE_INTEGRATION.md` §4 MarketplaceProvider

**Deploy:**
```bash
# Enable marketplace plugin (OPT-IN)
[providers]
enabled_providers = local,marketplace

[provider.marketplace]
enabled = true  # Set to true to opt-in (disabled by default)
# Pay-as-you-go: 5% commission on marketplace transactions
# API key required for billing identity (no monthly fees, no limits)
api_key_env = TOKLIGENCE_MARKETPLACE_API_KEY

# IMPORTANT: Review privacy policy first
# https://marketplace.tokligence.com/privacy
```

---

## System Architecture (Revised)

### Three Independent Layers + Protection

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 0: LLM PROTECTION (NEW)                              │
│  ─────────────────────────────────────────────────────────  │
│  Validate and transform requests before scheduling           │
│                                                               │
│  Protections:                                                 │
│    • Context length limits (per-model)                       │
│    • Rate limiting (global + per-model)                      │
│    • Content filtering (optional)                            │
│    • Custom hooks (webhook/Lua)                              │
│                                                               │
│  Output: Protected Request                                   │
│                                                               │
│  📄 See: 06_MARKETPLACE_INTEGRATION.md §5                    │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: REQUEST CLASSIFICATION                             │
│  (unchanged from v1.0)                                       │
│                                                               │
│  📄 See: 04_TOKEN_BASED_ROUTING.md                           │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: CAPACITY ALLOCATION (REVISED)                      │
│  ─────────────────────────────────────────────────────────  │
│  CHANGED: Capacity now measured in tokens/sec, not RPS      │
│                                                               │
│  Mechanisms:                                                  │
│    A) Priority-Based: 5 levels (P0-P4) [DEFAULT]            │
│    B) Bucket-Based: 10 buckets (optional)                   │
│    C) Bucket-Based: 20-30 buckets (advanced)                │
│                                                               │
│  Capacity = (tokens/sec, context, model_family)              │
│                                                               │
│  📄 See: 01_PRIORITY_BASED_SCHEDULING.md (Option A)          │
│  📄 See: 02_BUCKET_BASED_SCHEDULING.md (Option B/C)          │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: PROVIDER SELECTION (NEW)                           │
│  ─────────────────────────────────────────────────────────  │
│  Where to route this request?                                │
│                                                               │
│  Providers:                                                   │
│    • LocalProvider: Self-hosted LLM (always available)      │
│    • MarketplaceProvider: Discover via marketplace (plugin) │
│    • HybridProvider: Local first, marketplace fallback      │
│                                                               │
│  📄 See: 06_MARKETPLACE_INTEGRATION.md                       │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 4: SCHEDULING & EXECUTION                             │
│  (unchanged from v1.0)                                       │
│                                                               │
│  📄 See: 01_PRIORITY_BASED_SCHEDULING.md §3                  │
└─────────────────────────────────────────────────────────────┘
```

---

## Recommended Configuration Path

### Phase 0: MVP (Weeks 1-4)

**Goal:** Production-ready standalone gateway

**Implement:**
- ✅ LocalProvider (self-hosted LLM)
- ✅ 5-level priority scheduling
- ✅ Token routing
- ✅ Context length protection
- ✅ Basic observability

**Configuration:**
```ini
[scheduler]
type = priority
priority_levels = 5

[protection.context_length]
enabled = true
truncation_strategy = truncate_oldest

[providers]
enabled_providers = local
```

**Deliverable:** Fully functional open-source gateway

### Phase 1: Marketplace Plugin (Weeks 5-8)

**Goal:** Enable marketplace integration (optional)

**Implement:**
- ✅ MarketplaceProvider plugin
- ✅ Supply discovery API
- ✅ Cross-gateway routing
- ✅ Usage reporting

**Configuration:**
```ini
[providers]
enabled_providers = local,marketplace  # Add marketplace

[provider.marketplace]
enabled = true
```

**Deliverable:** Commercial marketplace plugin

### Phase 2: Advanced Features (Weeks 9-12)

**Goal:** Advanced scheduling for power users

**Implement:**
- ✅ 10-bucket model (optional)
- ✅ Time windows
- ✅ AtLeast mode
- ✅ Advanced protection

**Configuration:**
```ini
[scheduler]
type = bucket
bucket_count = 10  # Optional upgrade
```

**Deliverable:** Feature-complete gateway

---

## Resource Measurement Model (REVISED)

### Old Model (v1.0) - DEPRECATED

```yaml
# ❌ Too simplistic
bucket_0:
  capacity_rps: 100  # What does 100 RPS mean for different models?
```

### New Model (v2.0) - RECOMMENDED

```go
type Capacity struct {
    // PRIMARY metric
    MaxTokensPerSec   int     // tokens/sec (MOST IMPORTANT)

    // SECONDARY metrics
    MaxRPS            int     // requests/sec
    MaxConcurrent     int     // concurrent requests
    MaxContextLength  int     // max context window

    // Model-specific
    ModelFamily       string  // "chat" | "embedding" | "completion"
    AvgContextLength  int     // average context used
    AvgCompletionTokens int   // average completion

    // Dynamic
    CurrentLoad       float64 // 0.0-1.0
    AvailableTokensPS int     // available right now
}
```

**Why this matters:**

```
Example: Same "100 RPS" bucket

Scenario 1: Short embeddings
  - 100 requests × 10 tokens each = 1,000 tokens/sec
  - GPU load: 5%

Scenario 2: Long context chat
  - 100 requests × 32K tokens each = 3.2M tokens/sec
  - GPU load: 200% (IMPOSSIBLE!)

Conclusion: RPS alone is meaningless
```

**Correct capacity allocation:**

```ini
[bucket_config]
# Chat models (long context)
bucket_0.model_family = chat
bucket_0.max_tokens_per_sec = 10000
bucket_0.max_context_length = 128000
bucket_0.expected_avg_context = 8000

# Embedding models (short, parallel)
bucket_1.model_family = embedding
bucket_1.max_tokens_per_sec = 100000
bucket_1.max_context_length = 8192
bucket_1.expected_avg_context = 512
```

---

## LLM Protection Layer (NEW)

### Context Length Protection

**Problem:** Different models have different context limits

**Solution:** Per-model context protection with multiple strategies

```ini
[protection.context_length]
enabled = true

# Model-specific limits
model_limits = {
    "gpt-4": 128000,
    "gpt-3.5-turbo": 16385,
    "claude-3-opus": 200000,
    "llama-3-70b": 8192
}

# What to do when exceeded?
truncation_strategy = truncate_oldest  # or: reject, truncate_middle, summarize, custom
```

**Strategies:**

1. **reject** - Return 400 error
   ```
   User asks with 200K tokens → claude-3-opus limit is 200K
   → Reject: "Context too long"
   ```

2. **truncate_oldest** - Remove old messages
   ```
   User asks with 200K tokens → limit 128K
   → Remove oldest 72K tokens worth of messages
   → Keep system message + recent conversation
   ```

3. **truncate_middle** - Keep first and last
   ```
   User asks with 200K tokens → limit 128K
   → Keep system message + last 10 messages
   → Remove middle conversation
   ```

4. **custom** - Webhook or Lua script
   ```ini
   truncation_strategy = custom
   custom_handler_url = http://localhost:9000/truncate
   ```

   Webhook receives:
   ```json
   {
     "request": {...},
     "limit": 128000,
     "actual": 200000,
     "model": "gpt-4"
   }
   ```

   Webhook returns truncated request.

**See:** `06_MARKETPLACE_INTEGRATION.md` §5 for full implementation

---

## Failure Modes & Degradation (NEW)

### Token Routing Availability

**Problem:** LRU → Redis → Postgres dependency chain

**Solution:** Layered degradation

```go
func (tc *TokenCache) GetTokenMetadata(hash string) (*TokenMetadata, error) {
    // Layer 1: LRU cache (always available)
    if meta, ok := tc.localCache.Get(hash); ok {
        return meta, nil  // < 1μs
    }

    // Layer 2: Redis (may fail)
    meta, err := tc.getFromRedis(hash)
    if err == nil {
        return meta, nil  // < 1ms
    }

    // Layer 2.5: Snapshot cache (read-only fallback)
    if tc.config.EnableSnapshotCache {
        if meta, ok := tc.snapshotCache.Get(hash); ok {
            log.Warn("Using stale cache (Redis down)")
            return meta, nil  // Degraded but working
        }
    }

    // Layer 3: PostgreSQL (may fail)
    meta, err = tc.getFromDB(hash)
    if err == nil {
        return meta, nil  // < 10ms
    }

    // All stores down - choose failure mode
    if tc.config.FailOpen {
        // Fail-open: Allow requests with strict limits
        log.Error("All token stores down, failing open")
        return &TokenMetadata{
            PriorityTier: "external",
            Priority:     4,  // Lowest priority
            QuotaLimit:   100,  // Strict limit
        }, nil
    } else {
        // Fail-close: Reject all requests
        return nil, ErrAllTokenStoresDown
    }
}
```

**Configuration:**

```ini
[token_routing]
enable_snapshot_cache = true
snapshot_refresh_interval = 1h

fail_mode = fail_open  # or: fail_close
fail_open_quota_limit = 100
```

---

## Observability Constraints (NEW)

### Metrics Cardinality Problem

**Problem:** 100 buckets × 5 envs × 10 tiers = 5000 time series → Prometheus爆炸

**Solution:** Adaptive metrics

```yaml
# When bucket_count <= 10: Full metrics
metrics:
  - tgw_bucket_utilization{bucket_id="0"}
  - tgw_bucket_utilization{bucket_id="1"}
  # ... 10 metrics

# When bucket_count > 20: Range metrics
metrics:
  - tgw_bucket_utilization{bucket_range="critical"}   # buckets 0-4
  - tgw_bucket_utilization{bucket_range="high"}       # buckets 5-14
  - tgw_bucket_utilization{bucket_range="medium"}     # buckets 15-29
  - tgw_bucket_utilization{bucket_range="low"}        # buckets 30-49
  - tgw_bucket_utilization{bucket_range="spot"}       # buckets 50-99
```

**Implementation:**

```go
func (bs *BucketScheduler) EmitMetrics() {
    if bs.config.BucketCount <= 20 {
        // Full granularity
        for i := 0; i < bs.config.BucketCount; i++ {
            metrics.BucketUtilization.WithLabelValues(
                fmt.Sprintf("%d", i),
            ).Set(bs.buckets[i].CurrentLoad)
        }
    } else {
        // Range aggregation
        ranges := map[string][2]int{
            "critical": {0, 4},
            "high":     {5, 14},
            "medium":   {15, 29},
            "low":      {30, 49},
            "spot":     {50, 99},
        }

        for rangeName, bounds := range ranges {
            avgLoad := bs.getAverageLoad(bounds[0], bounds[1])
            metrics.BucketUtilization.WithLabelValues(rangeName).Set(avgLoad)
        }
    }
}
```

---

## Migration from v1.0 to v2.0

### Breaking Changes

1. **Capacity measurement:** RPS → tokens/sec (config change needed)
2. **Default scheduler:** 100-bucket → 5-priority (simplified)
3. **Provider abstraction:** Direct LLM → Provider SPI (code change)

### Migration Steps

```bash
# Step 1: Benchmark your LLM with new tool
tokligence benchmark \
  --endpoint http://localhost:8000 \
  --model gpt-4 \
  --profile long_context \
  --output capacity.json

# Step 2: Update config (RPS → tokens/sec)
# OLD (v1.0):
[bucket_config]
base_capacity_rps = 100

# NEW (v2.0):
[provider.local]
max_tokens_per_sec = 10000  # From benchmark

# Step 3: Simplify scheduler (if using buckets)
# OLD (v1.0):
[scheduler]
type = bucket
bucket_count = 100  # Too many!

# NEW (v2.0):
[scheduler]
type = priority  # Start simple
priority_levels = 5

# Step 4: Enable protection
[protection.context_length]
enabled = true
truncation_strategy = truncate_oldest

# Step 5: Restart gateway
tokligence gateway restart
```

---

## Recommended Defaults (v2.0)

### Small Deployment (< 1000 RPS)

```ini
[scheduler]
type = priority
priority_levels = 5

[provider.local]
max_tokens_per_sec = 5000

[protection.context_length]
enabled = true
truncation_strategy = truncate_oldest
```

### Medium Deployment (1000-10K RPS)

```ini
[scheduler]
type = bucket
bucket_count = 10  # Fine-grained

[provider.local]
max_tokens_per_sec = 50000

[protection.context_length]
enabled = true
truncation_strategy = custom
custom_handler_url = http://localhost:9000/truncate
```

### Large Deployment (10K+ RPS) + Marketplace

```ini
[scheduler]
type = bucket
bucket_count = 20

[providers]
enabled_providers = local,marketplace

[provider.local]
max_tokens_per_sec = 200000

[provider.marketplace]
enabled = true
prefer_region = us-east-1
max_price_per_mtoken = 5.0
```

---

## Document Index (v2.1 - ALL UPDATED)

| # | Document | Status | Priority | Key Updates |
|---|----------|--------|----------|-------------|
| **00** | REVISED_OVERVIEW.md (this) | ✅ v2.0 | **READ FIRST** | Marketplace privacy notice |
| **01** | PRIORITY_BASED_SCHEDULING.md | ✅ v2.0 | High (default) | Provider SPI, tokens/sec, LLM protection |
| **02** | BUCKET_BASED_SCHEDULING.md | ✅ v2.0 | Medium (optional) | Tokens/sec, observability, fairness, 5-10 bucket rec |
| **03** | CONFIGURABLE_BUCKET_COUNT.md | ✅ v2.0 | Low (advanced) | Observability constraints, fairness limits |
| **04** | TOKEN_BASED_ROUTING.md | ✅ v2.0 | High | Degradation strategies, snapshot cache, fail-open/close |
| **05** | ORTHOGONALITY_ANALYSIS.md | ✅ v1.0 | Medium | (No changes needed) |
| **06** | MARKETPLACE_INTEGRATION.md | ✅ v2.1 | **High** | Privacy policy, disabled by default, degradation |
| **07** | PUBLICATION_ANALYSIS.md | ✅ v1.0 | Low (meta) | (No changes needed) |

**All critical documents (01-04, 06) are now v2.0/v2.1 and fully consistent.**

---

## Next Steps

### For New Implementers

1. ✅ Read this overview
2. ✅ Read `06_MARKETPLACE_INTEGRATION.md` §3 (LocalProvider)
3. ✅ Read `01_PRIORITY_BASED_SCHEDULING.md`
4. ✅ Implement Phase 0 MVP (4 weeks)
5. ⏭️ Decide: Marketplace plugin? (Phase 1)
6. ⏭️ Decide: Advanced scheduling? (Phase 2)

### For Existing v1.0 Users

1. ✅ Read this overview (understand breaking changes)
2. ✅ Benchmark LLM with new tool
3. ✅ Update config (RPS → tokens/sec)
4. ✅ Enable protection layer
5. ✅ Migrate to v2.0

### For Documentation Writers

1. ✅ All core docs updated to v2.0/v2.1 (01, 02, 03, 04, 06)
2. ⏳ Create `08_MIGRATION_GUIDE.md` (v1→v2) - future work
3. ⏳ Create `09_BENCHMARKING.md` (detailed benchmarking guide) - future work

---

**End of Revised Overview**

**Key Takeaway:** Start simple (5-priority + LocalProvider), add marketplace when needed, use buckets only if you really need fine-grained control.
