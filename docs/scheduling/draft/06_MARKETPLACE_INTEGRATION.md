# Marketplace Integration Design (Optional Plugin)

**Version:** 2.2
**Date:** 2025-11-23
**Status:** ✅ CORRECTED - Pay-as-you-go model (5% commission)
**License Model:** Apache 2.0 (Core + Plugin) with Pay-as-you-go SaaS Service

---

## ⚠️ BUSINESS MODEL CORRECTION

**Subscription models in this document are OBSOLETE.**
- ❌ Pro tier ($49/month) - DELETED
- ❌ Business tier ($199/month) - DELETED
- ❌ Free tier (100 req/day) - DELETED
- ✅ Pay-as-you-go (5% commission) - CORRECT

**See `CORRECT_BUSINESS_MODEL.md` for authoritative pricing.**

---

## Executive Summary

This document designs **Marketplace integration as an optional plugin** for Tokligence Gateway. Both the gateway core and marketplace plugin are **Apache 2.0 licensed**. The marketplace API service uses a **pay-as-you-go model (5% transaction commission)** for monetization, and is **disabled by default** to respect privacy and offline use cases.

**Key Principle: Plugin, Not Core (Disabled by Default, Opt-In)**
- ✅ Gateway works perfectly standalone (marketplace **disabled by default**)
- ✅ Marketplace plugin is **included but requires explicit opt-in**
- ✅ First run is **completely offline** (no network calls)
- ✅ Clean plugin interface (Provider SPI)
- ✅ Apache 2.0 code + Optional pay-as-you-go SaaS service (5% commission)
- ✅ Privacy-first: No data sent without user consent

**Critical Design Decision:** After review, we chose **Model 2.5 (Disabled by Default)** instead of Model 3, because:
- Open-source users expect "no dial-home" by default
- Enterprise/compliance environments require offline operation
- Privacy regulations (GDPR, etc.) require explicit consent
- First-time experience should work in air-gapped environments

---

## Transaction Commission Flow (Pay-as-you-go)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  User Request Flow (with 5% Commission)                                 │
└─────────────────────────────────────────────────────────────────────────┘

1. User sends LLM request
   ↓
2. Gateway: MarketplaceProvider.RouteRequest()
   ↓
3. Marketplace API: GET /v1/supplies?model=gpt-4&region=us-east-1
   ← Returns supplier quotes:
     [
       {id: "sup-1", price: $8/Mtok, p99: 500ms, throughput: 10Ktok/s},
       {id: "sup-2", price: $10/Mtok, p99: 300ms, throughput: 15Ktok/s},
       {id: "sup-3", price: $12/Mtok, p99: 200ms, throughput: 20Ktok/s}
     ]
   ↓
4. Gateway: selectBestSupply() - Multi-dimensional scoring
   Score = 0.4×PriceScore + 0.3×LatencyScore + 0.1×ThroughputScore + ...
   ↓
   Best choice: sup-1 (price=$8/Mtok, score=0.85)
   ↓
5. Gateway → Supplier Gateway: POST /v1/chat/completions
   ↓
6. Supplier processes request, returns response (e.g., 1500 tokens used)
   ↓
7. Gateway: reportUsage() - Calculate commission
   ┌──────────────────────────────────────┐
   │ Billing Calculation:                 │
   │ ───────────────────────────────────  │
   │ Tokens used: 1,500                   │
   │ Supplier price: $8/Mtok              │
   │ Supplier cost: 0.0015M × $8 = $0.012 │
   │ User cost: $0.012 × 1.05 = $0.0126   │
   │ Commission: $0.0126 - $0.012 = $0.0006 (5%) │
   └──────────────────────────────────────┘
   ↓
8. Gateway → Marketplace API: POST /v1/billing/transactions
   {
     "supply_id": "sup-1",
     "user_id": "user-456",
     "tokens_used": 1500,
     "supplier_cost": 0.012,   // What supplier gets
     "user_cost": 0.0126,      // What user pays
     "commission": 0.0006      // Our 5% take (GMV)
   }
   ↓
9. Marketplace API updates accounts:
   • Debit user account: -$0.0126
   • Credit supplier account: +$0.012
   • Credit Tokligence account: +$0.0006 (5% commission)
   ↓
10. Response returned to user

┌─────────────────────────────────────────────────────────────────────────┐
│  Value Proposition for User                                             │
└─────────────────────────────────────────────────────────────────────────┘

If user went directly to OpenAI:
  • OpenAI gpt-4 price: $30/Mtok
  • Cost for 1500 tokens: 0.0015M × $30 = $0.045

Through Marketplace:
  • Supplier price: $8/Mtok
  • Cost for 1500 tokens: $0.0126 (including 5% commission)

Savings:
  • Absolute: $0.045 - $0.0126 = $0.0324
  • Percentage: 72% cheaper
  • ROI on commission: User saves $0.0324, pays $0.0006 commission = 54× ROI
```

---

## Table of Contents

1. [Architecture Philosophy](#1-architecture-philosophy)
2. [Provider SPI (Plugin Interface)](#2-provider-spi-plugin-interface)
   - 2.4 [Transaction Commission Pricing Model](#24-transaction-commission-pricing-model)
   - 2.5 [Privacy and Data Policy](#25-privacy-and-data-policy)
3. [Standalone Mode (Open-Source)](#3-standalone-mode-open-source)
4. [Marketplace Mode (Opt-In)](#4-marketplace-mode-opt-in)
   - 4.3 [Degradation and Fallback Strategies](#43-degradation-and-fallback-strategies)
5. [LLM Protection Layer](#5-llm-protection-layer)
6. [Resource Measurement Model](#6-resource-measurement-model)
7. [Migration Path](#7-migration-path)
8. [Open-Source Positioning](#8-open-source-positioning)
9. [Distribution Model Decision (Model 2.5)](#9-distribution-model-decision-model-25)

---

## 1. Architecture Philosophy

### 1.1 Design Principles

**Principle 1: Gateway is vendor-neutral**
```
Anyone can use Tokligence Gateway:
  ├─ Self-hosted LLM operators (local deployment)
  ├─ Companies with multi-tenant workloads
  ├─ Developers building LLM apps
  └─ Marketplace users (optional)
```

**Principle 2: Plugin-based architecture**
```
Core Gateway (Open-Source)
  ├─ Request scheduling
  ├─ Token routing
  ├─ LLM protection
  └─ Provider SPI (plugin interface)

Marketplace Plugin (Optional)
  ├─ Supply discovery
  ├─ Cross-gateway routing
  ├─ Dynamic pricing
  └─ Health monitoring
```

**Principle 3: Zero lock-in**
- Users can switch from standalone → marketplace
- Users can switch from marketplace → standalone
- No vendor lock-in

### 1.2 Deployment Modes

```
┌─────────────────────────────────────────────────────────────┐
│  Mode 1: Standalone (Open-Source)                           │
│  ──────────────────────────────────────────────────────────  │
│  Client → Tokligence Gateway → Self-hosted LLM              │
│                                                               │
│  Use case: Companies with their own LLMs                     │
│  License: MIT/Apache 2.0                                     │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  Mode 2: Marketplace-Connected (Optional Plugin)             │
│  ──────────────────────────────────────────────────────────  │
│  Client → Gateway (with marketplace plugin)                  │
│            ↓                                                  │
│       Marketplace API (discover providers)                   │
│            ↓                                                  │
│       Provider A, Provider B, Provider C                     │
│                                                               │
│  Use case: Buy/sell LLM capacity on marketplace             │
│  Code License: Apache 2.0 (plugin is open-source)            │
│  SaaS Service: Freemium (100 req/day free, paid tiers)       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  Mode 3: Hybrid                                              │
│  ──────────────────────────────────────────────────────────  │
│  Client → Gateway                                             │
│            ↓                                                  │
│       ┌─────────┬──────────────┐                             │
│       │ Primary │   Fallback    │                             │
│       │ (self)  │ (marketplace) │                             │
│       └─────────┴──────────────┘                             │
│                                                               │
│  Use case: Self-hosted with marketplace overflow             │
│  Code License: Apache 2.0 (all code is open-source)          │
│  SaaS Service: Freemium (100 req/day free, paid tiers)       │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Provider SPI (Plugin Interface)

### 2.1 Core Abstraction

**Provider = source of LLM capacity**

```go
// internal/provider/spi.go

package provider

import (
    "context"
    "time"
)

// Provider is the core abstraction for LLM capacity sources
// Implementations:
//   - LocalProvider: Direct connection to self-hosted LLM
//   - MarketplaceProvider: Discover and route via marketplace
//   - HybridProvider: Try local first, fallback to marketplace
type Provider interface {
    // GetCapacity returns available capacity for a model
    GetCapacity(ctx context.Context, model string) (*Capacity, error)

    // RouteRequest routes a request to an LLM instance
    RouteRequest(ctx context.Context, req *Request) (*Response, error)

    // HealthCheck checks if provider is healthy
    HealthCheck(ctx context.Context) (*Health, error)

    // GetMetadata returns provider metadata
    GetMetadata() *ProviderMetadata
}

// Capacity represents available LLM capacity
type Capacity struct {
    // Model identification
    ModelName   string
    ModelFamily string  // "chat" | "embedding" | "completion"

    // Capacity metrics (NOT just RPS!)
    MaxTokensPerSec   int     // tokens/sec (primary metric)
    MaxRPS            int     // requests/sec (secondary)
    MaxConcurrent     int     // max concurrent requests
    MaxContextLength  int     // max context window

    // Current load
    CurrentLoad       float64 // 0.0-1.0
    AvailableTokensPS int     // available tokens/sec right now

    // Cost (optional, for marketplace)
    PricePerMToken    float64 // USD per million tokens

    // SLA (optional)
    P99LatencyMs      int
    Availability      float64 // 0.0-1.0
}

// ProviderMetadata describes the provider
type ProviderMetadata struct {
    Name        string
    Type        string  // "local" | "marketplace" | "hybrid"
    Region      string  // "us-east-1" | "eu-west-1" | ...
    Models      []string
    SupportedAPIs []string // "openai" | "anthropic" | ...
}

// Health represents provider health
type Health struct {
    Status      string  // "healthy" | "degraded" | "down"
    Latency     time.Duration
    ErrorRate   float64
    LastChecked time.Time
}
```

### 2.2 Provider Registry

```go
// internal/provider/registry.go

package provider

// Registry manages multiple providers
type Registry struct {
    providers map[string]Provider
    router    *ProviderRouter
}

func NewRegistry() *Registry {
    return &Registry{
        providers: make(map[string]Provider),
        router:    NewProviderRouter(),
    }
}

// Register a provider
func (r *Registry) Register(name string, provider Provider) {
    r.providers[name] = provider
}

// Select best provider for a request
func (r *Registry) SelectProvider(req *Request) (Provider, error) {
    return r.router.Route(req, r.providers)
}
```

### 2.3 Configuration

```ini
# config/gateway.ini

[providers]
# Enable multiple providers
enabled_providers = local,marketplace

# Default provider
default_provider = local

# Failover strategy
failover_enabled = true
failover_order = local,marketplace

# ============================================================================
# Local Provider (always available in open-source)
# ============================================================================
[provider.local]
type = local
endpoint = http://localhost:8000
models = gpt-4,claude-3-sonnet

# Capacity limits (from benchmarking)
max_tokens_per_sec = 10000
max_rps = 100
max_concurrent = 50
max_context_length = 128000

# ============================================================================
# Marketplace Provider (included, DISABLED by default - OPT-IN)
# ============================================================================
[provider.marketplace]
type = marketplace
enabled = false  # DISABLED by default (privacy-first, opt-in only)

# Marketplace API endpoint
api_endpoint = https://marketplace.tokligence.com

# Authentication (required for billing identity)
# API key identifies your account for transaction billing (5% commission)
# Get your API key from https://marketplace.tokligence.com/dashboard
api_key_env = TOKLIGENCE_MARKETPLACE_API_KEY

# Rate limiting (abuse prevention only, NOT billing tiers)
# Default: 100 RPS per account (adjustable based on usage patterns)
rate_limit_rps = 100  # requests per second (anti-abuse)

# Privacy: What data is sent when enabled?
# - Model name (e.g., "gpt-4")
# - Preferred region (e.g., "us-east-1")
# - Max price preference
# - NO request content, NO user data, NO PII
# Full privacy policy: https://marketplace.tokligence.com/privacy

# Offline mode (completely disable network calls)
offline_mode = false  # Set to true for air-gapped environments

# Degradation behavior when marketplace API is unavailable
# Options: "fail_closed" (reject requests) | "fail_open" (use local only)
degradation_mode = fail_open

# Discovery preferences
prefer_region = us-east-1
max_price_per_mtoken = 5.0
min_availability = 0.99

# Health check and retry settings
health_check_interval = 60s  # Check marketplace API health every 60s
health_check_timeout = 5s
max_retries = 3
retry_backoff = exponential  # exponential | linear | fixed
```

---

## 2.4 Transaction Commission Pricing Model

**Strategy:** Pay-as-you-go commission on GMV (like Stripe for payments)

```
┌─────────────────────────────────────────────────────────────┐
│  Pay-as-you-go Transaction Commission                       │
│  ──────────────────────────────────────────────────────────  │
│  • NO monthly subscription fees                              │
│  • NO request limits (unlimited usage)                       │
│  • 5% commission on marketplace transactions only            │
│  • Direct provider use: 0% commission (free routing)         │
│                                                              │
│  Pricing Example:                                            │
│    Supplier price: $100 for 1M tokens                        │
│    User pays: $105 (supplier × 1.05)                         │
│    Supplier gets: $100                                       │
│    Tokligence gets: $5 (5% commission)                       │
│                                                              │
│  Value Proposition:                                          │
│    vs OpenAI direct ($200): Save $95 (47.5% cheaper)         │
│    Commission cost: $5                                       │
│    Net savings: $90 (45% total savings)                      │
│    ROI: 18x (save $90, pay $5)                               │
│                                                              │
│  Suitable for: Everyone (fair, scales with usage)            │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│  Enterprise Tier (Custom)                                    │
│  ──────────────────────────────────────────────────────────  │
│  • Dedicated account manager                                 │
│  • Custom SLA                                                │
│  • On-premise marketplace deployment                         │
│  • White-label options                                       │
│  • Custom transaction fee negotiation                        │
│  • Suitable for: Large enterprises                           │
└─────────────────────────────────────────────────────────────┘
```

**Key Insight:** Under Apache 2.0, anyone can fork the code. We monetize the **marketplace network and service**, not the code itself. This is similar to:
- Confluent (Kafka): Open-source code, paid managed service
- Databricks (Spark): Open-source code, paid platform
- MongoDB: Open-source code, paid Atlas service

**Competitive Moat:** Network effects (more suppliers = more buyers = more suppliers), not code ownership.

---

## 2.5 Privacy and Data Policy

### What Data is Sent to Marketplace? (Only When Enabled)

**IMPORTANT:** Marketplace is **disabled by default**. No data is sent unless you explicitly set `enabled = true`.

When marketplace is enabled, the following **metadata only** is sent:

```yaml
# Data sent to marketplace.tokligence.com
Discovery Request:
  model_name: "gpt-4"              # Which model you need
  preferred_region: "us-east-1"    # Region preference
  max_price: 5.0                   # Price limit (USD per million tokens)
  min_availability: 0.99           # Minimum SLA requirement

# Data NOT sent (stays local):
- ❌ Request content (prompts, messages)
- ❌ Response content (completions)
- ❌ User data / PII
- ❌ API keys / secrets
- ❌ Internal IP addresses
- ❌ Request payloads
```

### Privacy Guarantees

1. **No Content Leakage:**
   - Gateway processes all requests locally
   - Only routing metadata sent to marketplace
   - LLM requests go directly to selected provider (not through marketplace)

2. **Opt-In Only:**
   - First run: marketplace disabled, fully offline
   - Must explicitly enable in config
   - Can disable anytime, no lock-in

3. **Offline Mode:**
   - Set `offline_mode = true` for air-gapped environments
   - Gateway will never attempt network calls
   - All requests use local provider only

4. **GDPR Compliance:**
   - No PII collected without consent
   - Data minimization (only necessary metadata)
   - Right to be forgotten (delete account deletes all data)
   - Data residency options (EU/US marketplace endpoints)

### Legal Notice Template

When marketplace is enabled, gateway should show (first time only):

```
┌─────────────────────────────────────────────────────────────┐
│ Marketplace Provider Enabled                                 │
│ ─────────────────────────────────────────────────────────── │
│                                                               │
│ Tokligence Gateway will send the following data to           │
│ marketplace.tokligence.com:                                  │
│   • Model name and region preferences                        │
│   • Capacity and pricing requirements                        │
│                                                               │
│ Your request content and responses are NOT sent.             │
│                                                               │
│ Privacy Policy: https://tokligence.com/privacy               │
│ Disable anytime: Set enabled=false in config                 │
│                                                               │
│ Continue? [Y/n]                                               │
└─────────────────────────────────────────────────────────────┘
```

### Compliance Checklist

**For enterprise/regulated environments:**

- [ ] Review marketplace privacy policy
- [ ] Verify no PII sent (audit network traffic)
- [ ] Test offline mode (`offline_mode = true`)
- [ ] Configure degradation mode (`fail_open` vs `fail_closed`)
- [ ] Set up internal audit logging
- [ ] Review data residency requirements
- [ ] Get legal/security approval
- [ ] Document what data crosses boundary

---

## 3. Standalone Mode (Open-Source)

### 3.1 LocalProvider Implementation

```go
// internal/provider/local/provider.go

package local

import (
    "context"
    "net/http"
    "tokligence/internal/provider"
)

// LocalProvider connects directly to self-hosted LLM
type LocalProvider struct {
    config   *Config
    client   *http.Client
    capacity *provider.Capacity
}

type Config struct {
    Endpoint         string
    Models           []string
    MaxTokensPerSec  int
    MaxRPS           int
    MaxConcurrent    int
    MaxContextLength int
}

func NewLocalProvider(config *Config) *LocalProvider {
    return &LocalProvider{
        config: config,
        client: &http.Client{Timeout: 30 * time.Second},
        capacity: &provider.Capacity{
            ModelName:         config.Models[0],
            MaxTokensPerSec:   config.MaxTokensPerSec,
            MaxRPS:            config.MaxRPS,
            MaxConcurrent:     config.MaxConcurrent,
            MaxContextLength:  config.MaxContextLength,
            CurrentLoad:       0.0,
        },
    }
}

func (lp *LocalProvider) GetCapacity(ctx context.Context, model string) (*provider.Capacity, error) {
    // Return static capacity (user-configured)
    return lp.capacity, nil
}

func (lp *LocalProvider) RouteRequest(ctx context.Context, req *provider.Request) (*provider.Response, error) {
    // Forward to local LLM
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", lp.config.Endpoint, req.Body)
    resp, err := lp.client.Do(httpReq)
    // ... handle response
    return provider.NewResponse(resp), nil
}

func (lp *LocalProvider) HealthCheck(ctx context.Context) (*provider.Health, error) {
    start := time.Now()
    resp, err := lp.client.Get(lp.config.Endpoint + "/health")
    latency := time.Since(start)

    if err != nil {
        return &provider.Health{Status: "down"}, err
    }

    return &provider.Health{
        Status:      "healthy",
        Latency:     latency,
        LastChecked: time.Now(),
    }, nil
}

func (lp *LocalProvider) GetMetadata() *provider.ProviderMetadata {
    return &provider.ProviderMetadata{
        Name:   "local",
        Type:   "local",
        Models: lp.config.Models,
    }
}
```

**This is ALL you need for standalone mode - no marketplace dependency!**

---

## 4. Marketplace Mode (Opt-In)

### 4.1 MarketplaceProvider (Plugin)

```go
// plugins/marketplace/provider.go (separate module, not in core)

package marketplace

import (
    "context"
    "tokligence/internal/provider"
)

// MarketplaceProvider discovers and routes via marketplace API
type MarketplaceProvider struct {
    client *MarketplaceClient
    cache  *SupplyCache
    config *Config
}

type Config struct {
    APIEndpoint string
    APIKey      string

    // Discovery preferences
    PreferRegion     string
    MaxPricePerToken float64
    MinAvailability  float64
}

func NewMarketplaceProvider(config *Config) *MarketplaceProvider {
    mp := &MarketplaceProvider{
        client: NewMarketplaceClient(config.APIEndpoint, config.APIKey),
        cache:  NewSupplyCache(5 * time.Minute), // Cache supplies for 5min
        config: config,
    }

    // If no API key provided, use free tier
    if config.APIKey == "" {
        log.Info("Marketplace provider using FREE TIER (100 requests/day). Upgrade at https://marketplace.tokligence.com/pricing")
    }

    return mp
}

func (mp *MarketplaceProvider) GetCapacity(ctx context.Context, model string) (*provider.Capacity, error) {
    // Query marketplace for available supplies
    supplies, err := mp.client.DiscoverSupplies(ctx, &DiscoveryRequest{
        Model:          model,
        Region:         mp.config.PreferRegion,
        MaxPrice:       mp.config.MaxPricePerToken,
        MinAvailability: mp.config.MinAvailability,
    })

    if err != nil {
        return nil, err
    }

    // Aggregate capacity from all supplies
    totalCapacity := mp.aggregateCapacity(supplies)

    // Cache supplies for routing
    mp.cache.Set(model, supplies)

    return totalCapacity, nil
}

func (mp *MarketplaceProvider) RouteRequest(ctx context.Context, req *provider.Request) (*provider.Response, error) {
    // Get cached supplies for this model
    supplies := mp.cache.Get(req.Model)

    if len(supplies) == 0 {
        // Re-discover if cache expired
        newSupplies, _ := mp.client.DiscoverSupplies(ctx, &DiscoveryRequest{
            Model: req.Model,
        })
        supplies = newSupplies
    }

    // Select best supply based on:
    // - Current load
    // - Price
    // - Latency
    // - Availability
    supply := mp.selectBestSupply(supplies, req)

    // Route request to selected supply
    resp, err := mp.client.RouteToSupply(ctx, supply.GatewayURL, req)

    // Report usage to marketplace (for billing)
    go mp.reportUsage(supply.ID, req, resp)

    return resp, err
}

func (mp *MarketplaceProvider) HealthCheck(ctx context.Context) (*provider.Health, error) {
    // Check marketplace API health
    return mp.client.HealthCheck(ctx)
}

func (mp *MarketplaceProvider) GetMetadata() *provider.ProviderMetadata {
    return &provider.ProviderMetadata{
        Name: "marketplace",
        Type: "marketplace",
    }
}

// ============================================================================
// Marketplace API Client
// ============================================================================

type MarketplaceClient struct {
    endpoint string
    apiKey   string
    client   *http.Client
}

type DiscoveryRequest struct {
    Model           string
    Region          string
    MaxPrice        float64
    MinAvailability float64
}

type Supply struct {
    ID              string
    ProviderName    string
    GatewayURL      string
    Model           string
    Region          string

    // Capacity
    AvailableTokensPS int
    MaxContextLength  int

    // Pricing
    PricePerMToken    float64

    // SLA
    P99LatencyMs      int
    Availability      float64

    // Health
    CurrentLoad       float64
    LastHealthCheck   time.Time
}

func (mc *MarketplaceClient) DiscoverSupplies(ctx context.Context, req *DiscoveryRequest) ([]*Supply, error) {
    // GET /v1/supplies?model=gpt-4&region=us-east-1&max_price=5.0
    httpReq, _ := http.NewRequestWithContext(ctx, "GET",
        fmt.Sprintf("%s/v1/supplies?model=%s&region=%s&max_price=%.2f",
            mc.endpoint, req.Model, req.Region, req.MaxPrice), nil)

    // Add API key for billing identity (required for transaction commission)
    if mc.apiKey != "" {
        httpReq.Header.Set("Authorization", "Bearer "+mc.apiKey)
    } else {
        return nil, fmt.Errorf("marketplace API key required for billing (set TOKLIGENCE_MARKETPLACE_API_KEY)")
    }

    resp, err := mc.client.Do(httpReq)
    if err != nil {
        return nil, err
    }

    // Check for rate limit errors (anti-abuse, NOT billing tiers)
    if resp.StatusCode == 429 {
        var errResp struct {
            Error   string `json:"error"`
            Message string `json:"message"`
            RetryAfter int `json:"retry_after_seconds"`
        }
        json.NewDecoder(resp.Body).Decode(&errResp)

        // Rate limit is for abuse prevention, not billing tiers
        log.Warn("Rate limit exceeded (anti-abuse). Retry after: %d seconds", errResp.RetryAfter)
        return nil, fmt.Errorf("marketplace rate limit exceeded (anti-abuse): %s", errResp.Message)
    }

    // Parse response
    var supplies []*Supply
    json.NewDecoder(resp.Body).Decode(&supplies)

    return supplies, nil
}

func (mc *MarketplaceClient) RouteToSupply(ctx context.Context, gatewayURL string, req *provider.Request) (*provider.Response, error) {
    // Forward request to supply's gateway
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", gatewayURL+"/v1/chat/completions", req.Body)
    httpReq.Header.Set("Authorization", "Bearer "+mc.apiKey)

    resp, err := mc.client.Do(httpReq)
    return provider.NewResponse(resp), err
}

// selectBestSupply implements multi-dimensional routing selection
// Similar to ad DSP allocation, but optimized for latency/throughput/cost instead of CTR
func (mp *MarketplaceProvider) selectBestSupply(supplies []*Supply, req *provider.Request) *Supply {
    if len(supplies) == 0 {
        return nil
    }
    if len(supplies) == 1 {
        return supplies[0]
    }

    // Multi-dimensional scoring:
    // Score = w1*PriceScore + w2*LatencyScore + w3*ThroughputScore + w4*AvailabilityScore
    //
    // Similar to ad DSP's eCPM = bid × pCTR, we use:
    // EffectiveCost = Price / (Throughput × Availability × (1 - NormalizedLatency))

    type scoredSupply struct {
        supply *Supply
        score  float64
    }

    var scored []scoredSupply

    for _, s := range supplies {
        // Price score (lower is better): normalize to [0, 1]
        // If price = $5/Mtok, max = $30/Mtok → score = 1 - (5/30) = 0.83
        maxPrice := 30.0  // OpenAI gpt-4 price as baseline
        priceScore := 1.0 - (s.PricePerMToken / maxPrice)
        if priceScore < 0 {
            priceScore = 0
        }

        // Latency score (lower P99 is better): normalize to [0, 1]
        // If P99 = 500ms, max acceptable = 5000ms → score = 1 - (500/5000) = 0.9
        maxLatencyMs := 5000.0
        latencyScore := 1.0 - (float64(s.P99LatencyMs) / maxLatencyMs)
        if latencyScore < 0 {
            latencyScore = 0
        }

        // Throughput score (higher is better): normalize to [0, 1]
        // If throughput = 10K tok/s, max = 50K tok/s → score = 10K/50K = 0.2
        maxThroughput := 50000.0
        throughputScore := float64(s.AvailableTokensPS) / maxThroughput
        if throughputScore > 1.0 {
            throughputScore = 1.0
        }

        // Availability score (higher is better): already in [0, 1]
        // If availability = 0.999 → score = 0.999
        availabilityScore := s.Availability

        // Load score (lower is better): invert current load
        // If load = 0.2 → score = 1 - 0.2 = 0.8
        loadScore := 1.0 - s.CurrentLoad
        if loadScore < 0 {
            loadScore = 0
        }

        // Weighted sum (configurable weights)
        // Default: prioritize price (40%), latency (30%), availability (20%), throughput (10%)
        weights := struct {
            price        float64
            latency      float64
            throughput   float64
            availability float64
            load         float64
        }{
            price:        0.40,
            latency:      0.30,
            throughput:   0.10,
            availability: 0.15,
            load:         0.05,
        }

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

    // Return best supply
    bestSupply := scored[0].supply

    log.Info("Selected supply: %s (price=$%.2f/Mtok, P99=%dms, throughput=%dtok/s, availability=%.3f, score=%.3f)",
        bestSupply.ProviderName,
        bestSupply.PricePerMToken,
        bestSupply.P99LatencyMs,
        bestSupply.AvailableTokensPS,
        bestSupply.Availability,
        scored[0].score)

    return bestSupply
}

// reportUsage reports transaction usage for 5% commission billing
func (mp *MarketplaceProvider) reportUsage(supplyID string, req *provider.Request, resp *provider.Response) {
    if resp == nil || resp.Usage == nil {
        return
    }

    // Calculate costs:
    // 1. Get supplier's base price
    supply := mp.cache.GetSupply(supplyID)
    if supply == nil {
        log.Error("Supply not found for billing: %s", supplyID)
        return
    }

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
        SupplierCost: supplierCost,
        UserCost:     userCost,
        Commission:   commission,
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

type Usage struct {
    SupplyID     string    `json:"supply_id"`
    UserID       string    `json:"user_id"`
    TokensUsed   int       `json:"tokens_used"`
    SupplierCost float64   `json:"supplier_cost"`  // What supplier gets
    UserCost     float64   `json:"user_cost"`      // What user pays
    Commission   float64   `json:"commission"`     // Our 5% take
    Timestamp    time.Time `json:"timestamp"`
    RequestID    string    `json:"request_id"`
}

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

### 4.2 Hybrid Provider (Best of Both Worlds)

```go
// internal/provider/hybrid/provider.go

package hybrid

import (
    "context"
    "tokligence/internal/provider"
    "tokligence/internal/provider/local"
    "tokligence/plugins/marketplace"  // Optional import
)

// HybridProvider tries local first, fallback to marketplace
type HybridProvider struct {
    local       provider.Provider
    marketplace provider.Provider
    config      *Config
}

type Config struct {
    // Failover thresholds
    LocalLoadThreshold float64  // Switch to marketplace if local load > this

    // Cost optimization
    PreferLocal bool  // Always prefer local (even if marketplace is cheaper)
}

func NewHybridProvider(local provider.Provider, marketplace provider.Provider, config *Config) *HybridProvider {
    return &HybridProvider{
        local:       local,
        marketplace: marketplace,
        config:      config,
    }
}

func (hp *HybridProvider) RouteRequest(ctx context.Context, req *provider.Request) (*provider.Response, error) {
    // Check local capacity
    localCap, err := hp.local.GetCapacity(ctx, req.Model)

    if err == nil && localCap.CurrentLoad < hp.config.LocalLoadThreshold {
        // Local has capacity, use it
        return hp.local.RouteRequest(ctx, req)
    }

    // Local is overloaded, try marketplace
    log.Info("Local overloaded, routing to marketplace",
        "local_load", localCap.CurrentLoad,
        "threshold", hp.config.LocalLoadThreshold)

    return hp.marketplace.RouteRequest(ctx, req)
}
```

### 4.3 Degradation and Fallback Strategies

**Critical for production:** Marketplace API can fail or be rate-limited. Gateway must handle these gracefully.

#### 4.3.1 Degradation Modes

```go
// internal/provider/marketplace/degradation.go

package marketplace

type DegradationMode string

const (
    // Fail-Open: Continue with local provider only when marketplace unavailable
    DegradationModeFailOpen DegradationMode = "fail_open"

    // Fail-Closed: Reject requests when marketplace unavailable
    DegradationModeFailClosed DegradationMode = "fail_closed"

    // Cached: Use cached supplier list when marketplace unavailable
    DegradationModeCached DegradationMode = "cached"
)

type DegradationConfig struct {
    Mode              DegradationMode
    CacheTTL          time.Duration  // How long to cache supplier lists
    HealthCheckInterval time.Duration  // How often to check marketplace health
    CircuitBreakerThreshold int       // Consecutive failures before opening circuit
    CircuitBreakerTimeout   time.Duration  // How long to keep circuit open
}
```

#### 4.3.2 Implementation with Circuit Breaker

```go
// MarketplaceProvider with degradation handling

type MarketplaceProvider struct {
    client         *MarketplaceClient
    cache          *SupplyCache
    config         *Config
    degradation    *DegradationConfig
    circuitBreaker *CircuitBreaker
    healthStatus   *HealthStatus
}

type HealthStatus struct {
    mu              sync.RWMutex
    isHealthy       bool
    lastCheck       time.Time
    consecutiveFails int
}

func (mp *MarketplaceProvider) GetCapacity(ctx context.Context, model string) (*provider.Capacity, error) {
    // Check circuit breaker
    if mp.circuitBreaker.IsOpen() {
        return mp.handleDegradation(ctx, model)
    }

    // Try marketplace
    supplies, err := mp.client.DiscoverSupplies(ctx, &DiscoveryRequest{
        Model:  model,
        Region: mp.config.PreferRegion,
    })

    if err != nil {
        // Record failure
        mp.circuitBreaker.RecordFailure()
        mp.healthStatus.RecordFailure()

        log.Warn("Marketplace discovery failed, applying degradation",
            "error", err,
            "mode", mp.degradation.Mode)

        return mp.handleDegradation(ctx, model)
    }

    // Success - reset circuit breaker
    mp.circuitBreaker.RecordSuccess()
    mp.healthStatus.RecordSuccess()

    // Cache supplies for degradation scenarios
    mp.cache.Set(model, supplies, mp.degradation.CacheTTL)

    return mp.aggregateCapacity(supplies), nil
}

func (mp *MarketplaceProvider) handleDegradation(ctx context.Context, model string) (*provider.Capacity, error) {
    switch mp.degradation.Mode {
    case DegradationModeFailOpen:
        // Return zero capacity - caller will use local provider
        log.Info("Marketplace unavailable (fail-open), returning zero capacity")
        return &provider.Capacity{
            ModelName:         model,
            MaxTokensPerSec:   0,  // No marketplace capacity
            AvailableTokensPS: 0,
        }, nil

    case DegradationModeFailClosed:
        // Reject requests
        return nil, fmt.Errorf("marketplace unavailable (fail-closed mode)")

    case DegradationModeCached:
        // Try cached supplies
        if cached := mp.cache.Get(model); cached != nil {
            log.Warn("Using cached supplier list (marketplace unavailable)",
                "cache_age", time.Since(cached.CachedAt))
            return mp.aggregateCapacity(cached.Supplies), nil
        }

        // No cache - fall back based on config
        if mp.degradation.FallbackToFailOpen {
            return mp.handleDegradation(ctx, model) // Recursive call with fail-open
        }
        return nil, fmt.Errorf("marketplace unavailable and no cached suppliers")

    default:
        return nil, fmt.Errorf("unknown degradation mode: %s", mp.degradation.Mode)
    }
}
```

#### 4.3.3 Circuit Breaker Implementation

```go
// internal/provider/marketplace/circuitbreaker.go

type CircuitBreaker struct {
    mu             sync.RWMutex
    threshold      int           // Open circuit after N failures
    timeout        time.Duration // Keep circuit open for this duration
    consecutiveFails int
    state          CircuitState
    openedAt       time.Time
}

type CircuitState int

const (
    CircuitClosed CircuitState = iota  // Normal operation
    CircuitOpen                         // Stop calling marketplace
    CircuitHalfOpen                     // Try one request to test
)

func (cb *CircuitBreaker) IsOpen() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()

    if cb.state == CircuitClosed {
        return false
    }

    if cb.state == CircuitOpen {
        // Check if timeout expired
        if time.Since(cb.openedAt) > cb.timeout {
            cb.state = CircuitHalfOpen
            log.Info("Circuit breaker: transitioning to half-open")
            return false  // Allow one request through
        }
        return true
    }

    // HalfOpen - allow request
    return false
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.consecutiveFails++

    if cb.consecutiveFails >= cb.threshold {
        cb.state = CircuitOpen
        cb.openedAt = time.Now()
        log.Warn("Circuit breaker: OPENED",
            "consecutive_fails", cb.consecutiveFails,
            "threshold", cb.threshold)
    }
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == CircuitHalfOpen {
        // Success in half-open - close circuit
        cb.state = CircuitClosed
        log.Info("Circuit breaker: CLOSED (recovered)")
    }

    cb.consecutiveFails = 0
}
```

#### 4.3.4 Configuration Example

```ini
[provider.marketplace.degradation]
# Degradation mode: fail_open | fail_closed | cached
mode = fail_open

# Cache settings
cache_ttl = 5m                    # Cache supplier lists for 5 minutes
cache_refresh_interval = 1m       # Background refresh every minute

# Circuit breaker settings
circuit_breaker_threshold = 3     # Open after 3 consecutive failures
circuit_breaker_timeout = 30s     # Keep open for 30 seconds
circuit_breaker_half_open_max = 1 # Allow 1 request in half-open state

# Health check
health_check_interval = 10s       # Check marketplace health every 10s
health_check_timeout = 3s         # Health check timeout

# Retry settings
max_retries = 3
retry_backoff = exponential       # exponential | linear | fixed
retry_initial_delay = 100ms
retry_max_delay = 5s

# Fallback behavior
fallback_to_local = true          # Use local provider when marketplace fails
fallback_show_warning = true      # Log warning when using fallback
```

#### 4.3.5 Observability

**Metrics to track:**

```go
// Prometheus metrics for degradation monitoring

marketplace_circuit_breaker_state{state="closed|open|halfopen"}
marketplace_consecutive_failures{provider="marketplace"}
marketplace_cache_hits{model="gpt-4"}
marketplace_cache_misses{model="gpt-4"}
marketplace_degradation_mode{mode="fail_open|fail_closed|cached"}
marketplace_fallback_to_local_total{reason="circuit_open|rate_limit|timeout"}
```

**Alerts:**

```yaml
# Alert when circuit breaker opens
- alert: MarketplaceCircuitBreakerOpen
  expr: marketplace_circuit_breaker_state{state="open"} == 1
  for: 1m
  annotations:
    summary: "Marketplace circuit breaker is OPEN"
    description: "Gateway is in degraded mode ({{ $labels.degradation_mode }})"

# Alert when cache is stale
- alert: MarketplaceCacheStale
  expr: time() - marketplace_cache_last_update > 600
  for: 5m
  annotations:
    summary: "Marketplace cache is stale (>10 minutes)"
```

---

## 5. LLM Protection Layer

### 5.1 Protection Interface (Hooks)

```go
// internal/protection/protection.go

package protection

import (
    "context"
    "fmt"
)

// Protector validates and transforms requests before sending to LLM
type Protector interface {
    // Validate checks if request is safe/valid
    Validate(ctx context.Context, req *Request) error

    // Transform modifies request if needed (e.g., truncate context)
    Transform(ctx context.Context, req *Request) (*Request, error)
}

// ProtectionChain runs multiple protectors in sequence
type ProtectionChain struct {
    protectors []Protector
}

func NewProtectionChain(protectors ...Protector) *ProtectionChain {
    return &ProtectionChain{protectors: protectors}
}

func (pc *ProtectionChain) Protect(ctx context.Context, req *Request) (*Request, error) {
    // Run validators
    for _, p := range pc.protectors {
        if err := p.Validate(ctx, req); err != nil {
            return nil, err
        }
    }

    // Run transformers
    transformedReq := req
    for _, p := range pc.protectors {
        newReq, err := p.Transform(ctx, transformedReq)
        if err != nil {
            return nil, err
        }
        transformedReq = newReq
    }

    return transformedReq, nil
}
```

### 5.2 Context Length Protection

```go
// internal/protection/context_length.go

package protection

import (
    "context"
    "fmt"
)

// ContextLengthProtector validates and truncates context if needed
type ContextLengthProtector struct {
    config *ContextLengthConfig
}

type ContextLengthConfig struct {
    // Model-specific limits
    ModelLimits map[string]int  // model -> max_context_length

    // Default limit
    DefaultLimit int

    // Truncation strategy
    TruncationStrategy string  // "reject" | "truncate_oldest" | "truncate_middle" | "summarize"

    // Hook for custom logic
    CustomHandler ContextLengthHandler
}

type ContextLengthHandler func(ctx context.Context, req *Request, limit int, actual int) (*Request, error)

func NewContextLengthProtector(config *ContextLengthConfig) *ContextLengthProtector {
    return &ContextLengthProtector{config: config}
}

func (clp *ContextLengthProtector) Validate(ctx context.Context, req *Request) error {
    limit := clp.getLimit(req.Model)
    actual := clp.estimateTokens(req)

    if actual > limit {
        log.Warn("Context length exceeded",
            "model", req.Model,
            "limit", limit,
            "actual", actual,
            "strategy", clp.config.TruncationStrategy)

        // Will be handled in Transform()
    }

    return nil
}

func (clp *ContextLengthProtector) Transform(ctx context.Context, req *Request) (*Request, error) {
    limit := clp.getLimit(req.Model)
    actual := clp.estimateTokens(req)

    if actual <= limit {
        // No transformation needed
        return req, nil
    }

    // Context exceeds limit, apply strategy
    switch clp.config.TruncationStrategy {
    case "reject":
        return nil, fmt.Errorf("context length %d exceeds limit %d for model %s",
            actual, limit, req.Model)

    case "truncate_oldest":
        return clp.truncateOldest(req, limit), nil

    case "truncate_middle":
        return clp.truncateMiddle(req, limit), nil

    case "summarize":
        return clp.summarize(ctx, req, limit)

    case "custom":
        if clp.config.CustomHandler != nil {
            return clp.config.CustomHandler(ctx, req, limit, actual)
        }
        return nil, fmt.Errorf("custom handler not configured")

    default:
        return nil, fmt.Errorf("unknown truncation strategy: %s", clp.config.TruncationStrategy)
    }
}

func (clp *ContextLengthProtector) getLimit(model string) int {
    if limit, ok := clp.config.ModelLimits[model]; ok {
        return limit
    }
    return clp.config.DefaultLimit
}

func (clp *ContextLengthProtector) estimateTokens(req *Request) int {
    total := 0
    for _, msg := range req.Messages {
        // Simple estimation: ~4 chars per token
        total += len(msg.Content) / 4
    }
    return total
}

func (clp *ContextLengthProtector) truncateOldest(req *Request, limit int) *Request {
    // Remove oldest messages until within limit
    newReq := *req
    newReq.Messages = make([]Message, len(req.Messages))
    copy(newReq.Messages, req.Messages)

    for clp.estimateTokens(&newReq) > limit && len(newReq.Messages) > 1 {
        // Remove oldest message (keep system message if exists)
        if newReq.Messages[0].Role == "system" {
            newReq.Messages = append(newReq.Messages[:1], newReq.Messages[2:]...)
        } else {
            newReq.Messages = newReq.Messages[1:]
        }
    }

    return &newReq
}

func (clp *ContextLengthProtector) truncateMiddle(req *Request, limit int) *Request {
    // Keep first and last N messages, remove middle
    // Common pattern for chat: keep system + recent messages
    newReq := *req

    if len(req.Messages) <= 2 {
        return &newReq
    }

    // Keep system message + last 5 messages
    systemMsg := []Message{}
    if req.Messages[0].Role == "system" {
        systemMsg = []Message{req.Messages[0]}
    }

    recentMsgs := req.Messages[max(0, len(req.Messages)-5):]
    newReq.Messages = append(systemMsg, recentMsgs...)

    return &newReq
}

func (clp *ContextLengthProtector) summarize(ctx context.Context, req *Request, limit int) (*Request, error) {
    // Call LLM to summarize old messages
    // This is advanced - requires another LLM call
    // For now, return error
    return nil, fmt.Errorf("summarization not implemented yet")
}
```

### 5.3 Configuration

```ini
# config/protection.ini

[protection]
enabled = true

# Enable specific protectors
enabled_protectors = context_length,rate_limit,content_filter

# ============================================================================
# Context Length Protection
# ============================================================================
[protection.context_length]
enabled = true

# Model-specific limits
model_limits = {
    "gpt-4": 128000,
    "gpt-4-turbo": 128000,
    "gpt-3.5-turbo": 16385,
    "claude-3-opus": 200000,
    "claude-3-sonnet": 200000,
    "claude-3-haiku": 200000,
    "llama-3-70b": 8192
}

# Default limit (if model not in map)
default_limit = 8192

# Truncation strategy: reject | truncate_oldest | truncate_middle | summarize | custom
truncation_strategy = truncate_oldest

# Custom handler (Lua script or webhook)
custom_handler_type = webhook
custom_handler_url = http://localhost:9000/truncate

# ============================================================================
# Rate Limiting Protection
# ============================================================================
[protection.rate_limit]
enabled = true

# Global rate limit
global_rps = 1000
global_tokens_per_sec = 100000

# Per-model rate limits
model_rate_limits = {
    "gpt-4": {"rps": 100, "tokens_per_sec": 10000},
    "gpt-3.5-turbo": {"rps": 500, "tokens_per_sec": 50000}
}

# ============================================================================
# Content Filter Protection
# ============================================================================
[protection.content_filter]
enabled = false  # Disabled by default

# Content safety API
safety_api_endpoint = https://api.openai.com/v1/moderations
safety_api_key_env = OPENAI_API_KEY

# Action on unsafe content: reject | warn | redact
unsafe_content_action = reject
```

### 5.4 Custom Protection Hook Example

```go
// Example: Custom context length handler via webhook

package protection

func WebhookContextLengthHandler(webhookURL string) ContextLengthHandler {
    return func(ctx context.Context, req *Request, limit int, actual int) (*Request, error) {
        // Call webhook with request
        webhookReq := map[string]interface{}{
            "request":      req,
            "limit":        limit,
            "actual":       actual,
            "model":        req.Model,
        }

        resp, err := http.Post(webhookURL, "application/json", toJSON(webhookReq))
        if err != nil {
            return nil, err
        }

        // Webhook should return truncated request
        var truncatedReq Request
        json.NewDecoder(resp.Body).Decode(&truncatedReq)

        return &truncatedReq, nil
    }
}
```

---

## 6. Resource Measurement Model

### 6.1 Multi-Dimensional Capacity

```go
// internal/capacity/measurement.go

package capacity

// CapacityMeasurement tracks multi-dimensional capacity
type CapacityMeasurement struct {
    // Primary metrics
    TokensPerSec      int     // tokens/sec (MOST IMPORTANT)
    RequestsPerSec    int     // requests/sec (secondary)
    ConcurrentRequests int    // concurrent requests

    // GPU metrics
    GPUMemoryUsed     int64   // bytes
    GPUMemoryTotal    int64   // bytes
    GPUUtilization    float64 // 0.0-1.0

    // Model-specific
    ModelFamily       string  // "chat" | "embedding" | "completion"
    AvgContextLength  int     // average context length
    AvgCompletionTokens int   // average completion tokens

    // Latency
    P50LatencyMs      int
    P95LatencyMs      int
    P99LatencyMs      int

    // Load
    CurrentLoad       float64 // 0.0-1.0

    // Timestamp
    MeasuredAt        time.Time
}

// BenchmarkResult represents benchmark output
type BenchmarkResult struct {
    Model            string
    Duration         time.Duration

    // Workload profile
    WorkloadProfile  string  // "short_chat" | "long_context" | "embedding"

    // Results
    MaxTokensPerSec  int
    MaxRPS           int
    MaxConcurrent    int
    P99LatencyMs     int

    // Recommended safe limits (80% of max)
    SafeTokensPerSec int
    SafeRPS          int
    SafeConcurrent   int
}
```

### 6.2 Benchmarking Tool

```bash
# tokligence benchmark command

tokligence benchmark \
  --endpoint http://localhost:8000 \
  --model gpt-4 \
  --profile long_context \
  --duration 300s \
  --output benchmark.json

# Output:
{
  "model": "gpt-4",
  "profile": "long_context",
  "max_tokens_per_sec": 8500,
  "max_rps": 45,
  "max_concurrent": 20,
  "p99_latency_ms": 2100,
  "recommended": {
    "safe_tokens_per_sec": 6800,  // 80% of max
    "safe_rps": 36,
    "safe_concurrent": 16
  }
}
```

---

## 7. Migration Path

### 7.1 Phase 0: Open-Source Core Gateway

```
Week 1-4:   Core gateway
            ├─ LocalProvider implementation
            ├─ Basic scheduling (5-level priority)
            ├─ Token routing
            └─ LLM protection layer

Week 5-6:   Benchmarking tool
            ├─ Multi-profile benchmarks
            ├─ Capacity measurement
            └─ Auto-configuration

Week 7-8:   MarketplaceProvider (opt-in)
            ├─ Supply discovery API client
            ├─ Multi-dimensional routing (price/latency/throughput)
            ├─ Transaction billing integration
            └─ Disabled by default (opt-in)

Week 9-10:  Documentation
            ├─ Quickstart
            ├─ Configuration guide
            ├─ Deployment examples
            └─ Opt-in guide and commission model explanation

Release:    v0.1.0 (Apache 2.0 + Opt-In Marketplace)
            - Full-featured standalone gateway
            - Marketplace disabled by default (opt-in)
            - 5% commission on transactions (no subscription)
```

### 7.2 Phase 1: Marketplace Backend + Transaction Billing

```
Week 11-14: Marketplace API (backend)
            ├─ Supply discovery API
            ├─ Health monitoring API
            ├─ Supplier pricing/SLA API
            ├─ Transaction billing API (5% commission)
            └─ Rate limiting (anti-abuse only, NOT billing tiers)

Week 15-18: Advanced marketplace features
            ├─ Multi-region routing
            ├─ Price optimization
            ├─ Custom contracts
            └─ Advanced analytics

Week 19-20: HybridProvider
            ├─ Local + marketplace failover
            ├─ Cost optimization
            └─ Load balancing

Week 21-22: Billing & payments
            ├─ Stripe integration
            ├─ ❌ ~~Subscription management~~ (REMOVED)
            ├─ Transaction commission processing (5%)
            └─ Usage tracking

Release:    v0.2.0 (Pay-as-you-go)
            - ❌ ~~Pro tier~~ (DELETED)
            - ❌ ~~Business tier~~ (DELETED)
            - ✅ 5% commission on all transactions
            - Hybrid mode supported
```

### 7.3 Phase 2: Advanced Features

```
Week 19-22: Advanced scheduling
            ├─ 10-bucket model (optional)
            ├─ Time windows
            └─ AtLeast mode

Week 23-24: Advanced protection
            ├─ Content filtering
            ├─ PII detection
            └─ Custom hooks

Release:    v0.3.0 (Feature Complete)
```

---

## 8. Open-Source Positioning

### 8.1 Marketing Message

**For open-source users:**
> "Tokligence Gateway: Production-ready LLM gateway with advanced scheduling, multi-tenant support, and LLM protection. Works standalone with LocalProvider, or opt-in to Tokligence Marketplace for 40-60% cost savings (pay-as-you-go, 5% commission)."

**Key points:**
- ✅ Fully functional without marketplace (standalone by default)
- ✅ Apache 2.0 license (core + marketplace plugin)
- ✅ Marketplace disabled by default (opt-in for privacy/compliance)
- ✅ No vendor lock-in (enable marketplace only when needed)
- ✅ Pay-as-you-go: 5% commission on transactions (no subscription, no limits)

**Why opt-in marketplace?**
1. **Privacy-first:** No external calls without explicit consent
2. **Compliance-friendly:** GDPR/enterprise default, enable only if needed
3. **Network effects:** Users who opt-in get better marketplace experience
4. **Still open-source:** Apache 2.0 allows anyone to fork/modify/sell

### 8.2 Comparison with Competitors

| Feature | Tokligence (OSS) | Kong | NGINX | LiteLLM |
|---------|-----------------|------|-------|---------|
| **Multi-tenant scheduling** | ✅ | ❌ | ❌ | ⚠️ Basic |
| **LLM protection** | ✅ | ❌ | ❌ | ❌ |
| **Token routing** | ✅ | ⚠️ (via plugins) | ❌ | ❌ |
| **Capacity measurement** | ✅ | ❌ | ❌ | ❌ |
| **Marketplace integration** | ✅ (plugin) | ❌ | ❌ | ❌ |
| **License** | MIT/Apache | Enterprise | Open-source | Apache |

**Positioning:**
- More advanced than NGINX/Kong for LLM workloads
- More production-ready than LiteLLM
- Freemium marketplace = unique differentiator

---

## 9. Distribution Model Decision (Model 2.5)

### 9.1 Why Model 2.5 (Included but Disabled by Default)?

After analyzing three distribution models **and receiving critical feedback on privacy/compliance**, we chose **Model 2.5: Include Plugin, Disabled by Default (Opt-In)**:

**Model 1: Separate Commercial Plugin** ❌
- Gateway (Apache 2.0) + Plugin (Proprietary)
- **Problem:** Apache 2.0 allows anyone to fork and sell the code
- Cannot prevent competitors from including marketplace plugin
- Ineffective for protecting commercial value

**Model 2: Include Plugin, Disabled by Default (Requires API Key)** ⚠️
- Plugin included but requires API key to enable
- **Problem:** ❌ (OBSOLETE - no tiers in pay-as-you-go model)
- Extra step hurts adoption

**Model 2.5: Include Plugin, Disabled by Default (Pay-as-you-go)** ✅✅ **CHOSEN**
- Plugin included in code but `enabled = false` by default
- API key required for billing identity (5% commission on transactions)
- **Benefits:**
  - **Privacy-first:** No network calls on first run
  - **Compliance-friendly:** Works in air-gapped environments
  - **Easy opt-in:** Set `enabled = true` + add API key
  - **Still discoverable:** Code is there, docs explain it
  - **Open-source friendly:** No "dial-home" by default
  - **Fair pricing:** Pay only for what you use (5% commission)

**Model 3: Include Plugin, Enabled by Default** ❌ **REJECTED**
- **Problems identified in review:**
  - Violates open-source "no dial-home" expectation
  - GDPR/compliance risk (data sent without explicit consent)
  - Breaks in offline/air-gapped environments
  - Enterprise security teams would block/reject
  - Community backlash risk

### 9.2 Apache 2.0 Implications

**Q: Can anyone fork and sell our code?**
A: Yes, Apache 2.0 explicitly allows commercial use.

**Q: How do we protect commercial value?**
A: We don't protect the code, we protect the **marketplace network and service**.

**Examples of this strategy:**
- **Confluent** ($10B valuation): Kafka is Apache 2.0, monetize managed service
- **Databricks** ($38B valuation): Spark is Apache 2.0, monetize platform
- **MongoDB** ($26B valuation): MongoDB is open-source, monetize Atlas
- **Elastic** ($9B valuation): Elasticsearch is Apache 2.0, monetize cloud

**Key insight:** Competitive moat is network effects, not code ownership.

### 9.3 Monetization Strategy

**What we monetize:**
1. **Transaction commission** (pay-as-you-go, 5% on GMV):
   - ✅ **Flat 5% commission on all marketplace transactions**
   - Example: Supplier price $100 → User pays $105 → We get $5
   - No monthly fees, no usage limits, no tiers
   - Enterprise: Custom contracts with negotiable rates

2. **Enterprise add-ons** (optional, not required):
   - Private marketplace (one-time setup fee: $10K-$50K)
   - White-label deployment (annual license: $50K/year)
   - SLA guarantees 99.99% (commission rate increases to 7%)
   - Dedicated support (annual retainer: $20K/year)

3. **Network effects:**
   - More suppliers → better prices for buyers
   - More buyers → more revenue for suppliers
   - Virtuous cycle creates switching costs

**What we do NOT monetize:**
- ❌ Code licensing (can't prevent under Apache 2.0)
- ❌ Proprietary plugins (can be forked)
- ❌ Vendor lock-in (against open-source principles)

### 9.4 Competitive Advantage

**Why competitors can't easily copy:**

1. **Network effects** (hard to replicate):
   - Marketplace value = suppliers × buyers
   - Need critical mass to be useful
   - First-mover advantage

2. **Trust and brand** (time-based moat):
   - Reputation takes years to build
   - Suppliers trust established marketplace
   - Security and compliance certifications

3. **Data and optimization** (compounding advantage):
   - Historical pricing data
   - Supply reliability scores
   - Demand prediction models

4. **Integration ecosystem** (stickiness):
   - Monitoring integrations (Prometheus, Grafana)
   - CI/CD integrations (GitHub Actions, GitLab)
   - Marketplace API integrations

**Competitors can fork the code, but cannot fork:**
- Our supplier network
- Our buyer base
- Our marketplace liquidity
- Our historical data
- Our brand reputation

### 9.5 Risk Mitigation

**Risk: Large company forks and outcompetes us**

**Mitigations:**
1. Move fast on marketplace launch (first-mover advantage)
2. Focus on developer experience (ease of use)
3. Build community (open-source contributors)
4. Offer best pricing (competitive transaction fees)
5. Provide superior support (SLA, documentation)

**Risk: Users disable marketplace to avoid fees**

**Mitigations:**
1. Make free tier generous (100 req/day)
2. Show clear value proposition (save 30% on LLM costs)
3. Optional feature (can disable without breaking)
4. Transparent pricing (no hidden fees)

**Risk: Marketplace doesn't get enough suppliers**

**Mitigations:**
1. Launch with 5-10 initial suppliers (seed network)
2. Offer supplier incentives (first 3 months free)
3. Make supplier onboarding easy (1-click setup)
4. Provide supplier marketing (featured listings)

### 9.6 Legal and Licensing

**Apache 2.0 License:**
- ✅ Allows commercial use
- ✅ Allows modification
- ✅ Allows distribution
- ✅ Allows private use
- ⚠️ Requires license and copyright notice
- ⚠️ Provides patent grant

**Marketplace Terms of Service (separate from code license):**
- Free tier: Accept ToS only
- Paid tier: Accept ToS + payment agreement
- Suppliers: Accept supplier agreement (revenue share, SLA, etc.)

**Data and Privacy:**
- Gateway: Processes requests locally (no data sent to marketplace)
- Marketplace: Only receives metadata (model, region, capacity, pricing)
- GDPR compliant: No PII in marketplace API calls
- Optional telemetry: Users can disable (respect privacy)

### 9.7 Implementation Checklist

**Before v0.1.0 launch:**
- [ ] Finalize marketplace API design (supply discovery, pricing, billing)
- [ ] Implement multi-dimensional routing (price/latency/throughput scoring)
- [ ] Implement transaction billing API (5% commission calculation)
- [ ] Write ToS and privacy policy
- [ ] Set up Stripe for transaction billing (not subscriptions)
- [ ] Create commission explanation page (https://marketplace.tokligence.com/pricing)
- [ ] Seed marketplace with 5-10 suppliers
- [ ] Test transaction flow (discovery → routing → billing → settlement)

**Documentation:**
- [ ] Add marketplace quickstart guide
- [ ] Explain pay-as-you-go commission model (5%)
- [ ] Document how to disable marketplace (opt-out)
- [ ] Provide supplier onboarding guide
- [ ] Create marketplace API reference
- [ ] Document multi-dimensional routing algorithm

**Marketing:**
- [ ] Announce on Hacker News, Reddit, Twitter
- [ ] Write blog post: "Why we chose Apache 2.0 + pay-as-you-go commission"
- [ ] Create demo video (5 minutes)
- [ ] Prepare case studies (early suppliers)
- [ ] Highlight cost savings vs OpenAI (40-60%)

---

**End of Document**
