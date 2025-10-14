# Comprehensive Billing and Provider Management Design

## Overview

This document outlines a unified approach to:
1. Multiple API provider management (OpenAI, Anthropic, etc.)
2. Billing aggregation at different organizational levels
3. Marketplace integration for token trading
4. Cost tracking and optimization across all layers

## Architecture Layers

```
┌─────────────────────────────────────────────┐
│          Organization (Enterprise)           │
│  ┌─────────────────────────────────────┐   │
│  │            Teams                     │   │
│  │  ┌─────────────────────────────┐    │   │
│  │  │         Users                │    │   │
│  │  │  ┌───────────────────────┐   │    │   │
│  │  │  │     API Keys           │   │    │   │
│  │  │  └───────────────────────┘   │    │   │
│  │  └─────────────────────────────┘    │   │
│  └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
                    │
                    ▼
     ┌──────────────────────────────┐
     │    Gateway Router             │
     └──────────────────────────────┘
                    │
     ┌──────────────┴──────────────┐
     ▼                             ▼
┌─────────────┐           ┌─────────────────┐
│Local Providers│         │   Marketplace    │
│- OpenAI      │          │   Providers      │
│- Anthropic   │          └─────────────────┘
│- Azure       │
│- Custom      │
└─────────────┘
```

## 1. Provider Management Layer

### Provider Configuration

```go
type Provider struct {
    ID              string                 `json:"id"`
    Name            string                 `json:"name"`
    Type            string                 `json:"type"` // "openai", "anthropic", "azure", "custom"
    Endpoint        string                 `json:"endpoint"`
    APIKey          string                 `json:"api_key"` // Encrypted
    Models          []ModelConfig          `json:"models"`
    Priority        int                    `json:"priority"`
    Weight          float64                `json:"weight"` // For load balancing
    Status          string                 `json:"status"` // "active", "degraded", "offline"
    CostMultiplier  float64                `json:"cost_multiplier"` // Provider-specific markup
    RateLimit       RateLimitConfig        `json:"rate_limit"`
    HealthCheck     HealthCheckConfig      `json:"health_check"`
    Metadata        map[string]interface{} `json:"metadata"`
}

type ModelConfig struct {
    Name            string  `json:"name"`
    DisplayName     string  `json:"display_name"`
    InputCostPer1K  float64 `json:"input_cost_per_1k"`
    OutputCostPer1K float64 `json:"output_cost_per_1k"`
    MaxTokens       int     `json:"max_tokens"`
    Capabilities    []string `json:"capabilities"`
    Deprecated      bool    `json:"deprecated"`
    ReplacementModel string `json:"replacement_model"`
}

type RateLimitConfig struct {
    RPM int `json:"rpm"` // Requests per minute
    TPM int `json:"tpm"` // Tokens per minute
    RPD int `json:"rpd"` // Requests per day
}
```

### Provider Database Schema

```sql
CREATE TABLE providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    models TEXT NOT NULL, -- JSON
    priority INTEGER DEFAULT 0,
    weight DECIMAL(3,2) DEFAULT 1.0,
    status TEXT DEFAULT 'active',
    cost_multiplier DECIMAL(5,2) DEFAULT 1.0,
    rate_limit TEXT, -- JSON
    health_check TEXT, -- JSON
    metadata TEXT, -- JSON
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_providers_type ON providers(type);
CREATE INDEX idx_providers_status ON providers(status);
CREATE INDEX idx_providers_priority ON providers(priority DESC);
```

## 2. Billing Aggregation Layers

### Cost Tracking Schema

```sql
-- Enhanced usage_entries with multi-level tracking
CREATE TABLE usage_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE DEFAULT (uuid_generate_v4()),

    -- Organizational hierarchy (NULL for OSS)
    organization_id INTEGER REFERENCES organizations(id),
    team_id INTEGER REFERENCES teams(id),
    user_id INTEGER NOT NULL,
    api_key_id INTEGER NOT NULL,

    -- Provider information
    provider_id TEXT REFERENCES providers(id),
    provider_type TEXT NOT NULL, -- 'local', 'marketplace'
    model_name TEXT NOT NULL,

    -- Token counts
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    total_tokens INTEGER GENERATED ALWAYS AS (prompt_tokens + completion_tokens),

    -- Cost breakdown
    input_cost DECIMAL(10,6),
    output_cost DECIMAL(10,6),
    provider_cost DECIMAL(10,6), -- What we pay to provider
    markup_cost DECIMAL(10,6),    -- Our markup
    total_cost DECIMAL(10,6),     -- What user pays

    -- Marketplace specific
    marketplace_provider_id TEXT,
    marketplace_transaction_id TEXT,
    marketplace_fee DECIMAL(10,6),

    -- Request metadata
    request_id TEXT NOT NULL UNIQUE,
    latency_ms INTEGER,
    status_code INTEGER,
    error_message TEXT,

    -- Billing cycle
    billing_period TEXT, -- '2024-01'

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- Indexes for efficient aggregation
CREATE INDEX idx_usage_org_period ON usage_entries(organization_id, billing_period);
CREATE INDEX idx_usage_team_period ON usage_entries(team_id, billing_period);
CREATE INDEX idx_usage_user_period ON usage_entries(user_id, billing_period);
CREATE INDEX idx_usage_provider_period ON usage_entries(provider_id, billing_period);
CREATE INDEX idx_usage_model_period ON usage_entries(model_name, billing_period);
```

### Aggregation Views

```sql
-- Organization-level aggregation
CREATE VIEW organization_usage AS
SELECT
    organization_id,
    billing_period,
    COUNT(DISTINCT team_id) as team_count,
    COUNT(DISTINCT user_id) as user_count,
    COUNT(DISTINCT api_key_id) as key_count,
    SUM(prompt_tokens) as total_prompt_tokens,
    SUM(completion_tokens) as total_completion_tokens,
    SUM(total_cost) as total_cost,
    SUM(provider_cost) as total_provider_cost,
    SUM(total_cost - provider_cost) as total_profit,
    COUNT(*) as request_count,
    AVG(latency_ms) as avg_latency
FROM usage_entries
WHERE deleted_at IS NULL
GROUP BY organization_id, billing_period;

-- Team-level aggregation
CREATE VIEW team_usage AS
SELECT
    team_id,
    organization_id,
    billing_period,
    COUNT(DISTINCT user_id) as user_count,
    COUNT(DISTINCT api_key_id) as key_count,
    SUM(prompt_tokens) as total_prompt_tokens,
    SUM(completion_tokens) as total_completion_tokens,
    SUM(total_cost) as total_cost,
    COUNT(*) as request_count
FROM usage_entries
WHERE deleted_at IS NULL
GROUP BY team_id, organization_id, billing_period;

-- Provider cost analysis
CREATE VIEW provider_cost_analysis AS
SELECT
    provider_id,
    provider_type,
    model_name,
    billing_period,
    COUNT(*) as request_count,
    SUM(total_tokens) as total_tokens,
    SUM(provider_cost) as total_provider_cost,
    SUM(total_cost) as total_revenue,
    SUM(total_cost - provider_cost) as gross_profit,
    (SUM(total_cost - provider_cost) / SUM(total_cost)) * 100 as profit_margin_percent
FROM usage_entries
WHERE deleted_at IS NULL
GROUP BY provider_id, provider_type, model_name, billing_period;

-- User-level cost breakdown by model
CREATE VIEW user_model_usage AS
SELECT
    user_id,
    model_name,
    provider_type,
    billing_period,
    COUNT(*) as request_count,
    SUM(prompt_tokens) as prompt_tokens,
    SUM(completion_tokens) as completion_tokens,
    SUM(total_cost) as total_cost,
    AVG(latency_ms) as avg_latency
FROM usage_entries
WHERE deleted_at IS NULL
GROUP BY user_id, model_name, provider_type, billing_period;
```

## 3. Unified Router with Cost Optimization

```go
type UnifiedRouter struct {
    providers       map[string]*Provider
    marketplace     *MarketplaceClient
    costOptimizer   *CostOptimizer
    loadBalancer    *LoadBalancer
    circuitBreakers map[string]*CircuitBreaker
}

func (r *UnifiedRouter) Route(ctx context.Context, req *Request) (*Response, error) {
    // 1. Validate API key and get configuration
    apiKey, err := r.validateAPIKey(ctx, req.APIKey)
    if err != nil {
        return nil, err
    }

    // 2. Check model access permissions
    if !apiKey.CanAccessModel(req.Model) {
        return nil, ErrModelNotAllowed
    }

    // 3. Get routing strategy based on key configuration
    strategy := r.getRoutingStrategy(apiKey)

    // 4. Find available providers (local + marketplace)
    candidates := r.findProviders(req.Model, apiKey, strategy)

    // 5. Apply cost optimization
    optimal := r.costOptimizer.SelectProvider(candidates, CostOptimizationParams{
        Budget: apiKey.GetRemainingBudget(),
        QualityPreference: apiKey.QualityPreference,
        LatencyRequirement: req.MaxLatency,
    })

    // 6. Execute request
    resp, providerCost, err := r.executeRequest(ctx, optimal, req)
    if err != nil {
        return nil, err
    }

    // 7. Calculate billing
    billing := r.calculateBilling(apiKey, optimal, req, resp, providerCost)

    // 8. Record usage at all levels
    r.recordUsage(ctx, UsageEntry{
        OrganizationID: apiKey.OrganizationID,
        TeamID: apiKey.TeamID,
        UserID: apiKey.UserID,
        APIKeyID: apiKey.ID,
        ProviderID: optimal.ID,
        ProviderType: optimal.Type,
        ModelName: req.Model,
        PromptTokens: resp.Usage.PromptTokens,
        CompletionTokens: resp.Usage.CompletionTokens,
        ProviderCost: providerCost,
        TotalCost: billing.TotalCost,
        RequestID: req.ID,
        LatencyMs: resp.LatencyMs,
    })

    return resp, nil
}

type CostOptimizer struct {
    historicalData *UsageHistory
    pricingEngine  *PricingEngine
}

func (co *CostOptimizer) SelectProvider(candidates []ProviderOption, params CostOptimizationParams) *ProviderOption {
    scores := make(map[string]float64)

    for _, candidate := range candidates {
        score := 0.0

        // Cost factor
        costScore := 1.0 / (candidate.CostPer1K + 0.001)
        score += costScore * params.CostWeight

        // Quality factor (based on historical performance)
        qualityScore := co.historicalData.GetQualityScore(candidate.Provider)
        score += qualityScore * params.QualityWeight

        // Latency factor
        latencyScore := 1.0 / (float64(candidate.ExpectedLatency) + 1.0)
        score += latencyScore * params.LatencyWeight

        // Availability factor
        availabilityScore := candidate.HealthScore
        score += availabilityScore * 0.1

        scores[candidate.ID] = score
    }

    // Select highest scoring provider
    return selectHighestScore(candidates, scores)
}
```

## 4. Billing Aggregation Functions

```go
// Organization-level billing
func (b *BillingService) GetOrganizationBilling(ctx context.Context, orgID int64, period string) (*OrganizationBilling, error) {
    return b.db.Query(`
        SELECT
            organization_id,
            billing_period,
            SUM(total_cost) as total_cost,
            SUM(provider_cost) as total_provider_cost,
            SUM(marketplace_fee) as total_marketplace_fee,
            SUM(total_tokens) as total_tokens,
            COUNT(DISTINCT team_id) as team_count,
            COUNT(DISTINCT user_id) as user_count,
            COUNT(*) as request_count,
            GROUP_CONCAT(DISTINCT provider_id) as providers_used,
            GROUP_CONCAT(DISTINCT model_name) as models_used
        FROM usage_entries
        WHERE organization_id = ?
        AND billing_period = ?
        AND deleted_at IS NULL
        GROUP BY organization_id, billing_period
    `, orgID, period)
}

// Team-level billing with drill-down
func (b *BillingService) GetTeamBilling(ctx context.Context, teamID int64, period string) (*TeamBilling, error) {
    billing := &TeamBilling{}

    // Get team totals
    b.db.Query(`
        SELECT ... FROM usage_entries
        WHERE team_id = ? AND billing_period = ?
        GROUP BY team_id
    `, teamID, period)

    // Get per-user breakdown
    billing.UserBreakdown = b.db.Query(`
        SELECT user_id, SUM(total_cost) as cost, SUM(total_tokens) as tokens
        FROM usage_entries
        WHERE team_id = ? AND billing_period = ?
        GROUP BY user_id
    `, teamID, period)

    // Get per-model breakdown
    billing.ModelBreakdown = b.db.Query(`
        SELECT model_name, provider_type, COUNT(*) as requests, SUM(total_cost) as cost
        FROM usage_entries
        WHERE team_id = ? AND billing_period = ?
        GROUP BY model_name, provider_type
    `, teamID, period)

    return billing, nil
}

// Cost allocation for shared resources
func (b *BillingService) AllocateSharedCosts(ctx context.Context, sharedCost float64, period string) error {
    // Get usage proportions by organization
    proportions := b.db.Query(`
        SELECT
            organization_id,
            SUM(total_tokens) as tokens,
            SUM(total_tokens) * 1.0 / (SELECT SUM(total_tokens) FROM usage_entries WHERE billing_period = ?) as proportion
        FROM usage_entries
        WHERE billing_period = ?
        GROUP BY organization_id
    `, period, period)

    // Allocate shared costs proportionally
    for _, p := range proportions {
        allocatedCost := sharedCost * p.Proportion
        b.db.Exec(`
            INSERT INTO cost_allocations (organization_id, period, allocated_cost, allocation_type)
            VALUES (?, ?, ?, 'shared_infrastructure')
        `, p.OrganizationID, period, allocatedCost)
    }

    return nil
}
```

## 5. Multi-Provider Load Balancing

```go
type LoadBalancer struct {
    strategy LoadBalanceStrategy
    providers []*Provider
    weights map[string]float64
    healthScores map[string]float64
}

type LoadBalanceStrategy interface {
    SelectProvider(providers []*Provider, request *Request) *Provider
}

// Weighted round-robin with health awareness
type WeightedRoundRobin struct {
    current int
    weights []float64
}

func (w *WeightedRoundRobin) SelectProvider(providers []*Provider, request *Request) *Provider {
    totalWeight := 0.0
    for _, p := range providers {
        if p.Status == "active" && p.CanHandleModel(request.Model) {
            totalWeight += p.Weight * p.HealthScore
        }
    }

    random := rand.Float64() * totalWeight
    cumulative := 0.0

    for _, p := range providers {
        if p.Status == "active" && p.CanHandleModel(request.Model) {
            cumulative += p.Weight * p.HealthScore
            if random <= cumulative {
                return p
            }
        }
    }

    return providers[0] // Fallback
}

// Least-cost routing
type LeastCostRouting struct {
    costCache map[string]float64
}

func (l *LeastCostRouting) SelectProvider(providers []*Provider, request *Request) *Provider {
    minCost := math.MaxFloat64
    var selected *Provider

    for _, p := range providers {
        if p.Status == "active" && p.CanHandleModel(request.Model) {
            cost := p.GetModelCost(request.Model, request.EstimatedTokens)
            if cost < minCost {
                minCost = cost
                selected = p
            }
        }
    }

    return selected
}
```

## 6. Provider Health Monitoring

```go
type HealthMonitor struct {
    providers map[string]*Provider
    metrics   map[string]*ProviderMetrics
    alerter   *Alerter
}

type ProviderMetrics struct {
    SuccessRate   float64
    AvgLatency    time.Duration
    ErrorRate     float64
    LastError     time.Time
    ConsecutiveErrors int
}

func (h *HealthMonitor) CheckHealth(ctx context.Context) {
    for id, provider := range h.providers {
        go func(p *Provider) {
            start := time.Now()
            err := p.Ping(ctx)
            latency := time.Since(start)

            metrics := h.metrics[p.ID]
            metrics.AvgLatency = (metrics.AvgLatency + latency) / 2

            if err != nil {
                metrics.ConsecutiveErrors++
                metrics.LastError = time.Now()

                if metrics.ConsecutiveErrors > 3 {
                    p.Status = "degraded"
                }
                if metrics.ConsecutiveErrors > 10 {
                    p.Status = "offline"
                    h.alerter.Send(Alert{
                        Level: "critical",
                        Message: fmt.Sprintf("Provider %s is offline", p.Name),
                    })
                }
            } else {
                metrics.ConsecutiveErrors = 0
                p.Status = "active"
            }

            // Calculate health score (0-1)
            p.HealthScore = calculateHealthScore(metrics)
        }(provider)
    }
}
```

## 7. CLI Commands

```bash
# Provider management
gateway provider add --name "openai-primary" --type openai --endpoint "https://api.openai.com" --api-key "sk-..."
gateway provider list
gateway provider update --id "openai-primary" --priority 1 --weight 0.7
gateway provider health --id "openai-primary"

# Billing queries
gateway billing org --org-id 1 --period "2024-01"
gateway billing team --team-id 5 --period "2024-01" --breakdown
gateway billing user --user-id 10 --period "2024-01" --by-model

# Cost optimization
gateway optimize suggest --period "2024-01"  # Suggests provider changes
gateway optimize simulate --config new-providers.yaml  # Simulates cost with new config

# Multi-level API key creation
gateway admin api-keys create \
  --user 1 \
  --team 5 \
  --org 1 \
  --allowed-models "gpt-4,claude-3" \
  --preferred-providers "openai-primary,anthropic-backup" \
  --budget 100.00 \
  --budget-level "team"  # Budget applies at team level
```

## 8. Benefits

1. **Cost Transparency**: Full visibility from provider cost to end-user billing
2. **Multi-level Aggregation**: Billing at user, team, and organization levels
3. **Provider Flexibility**: Easy to add/remove/switch providers
4. **Optimization**: Automatic routing to lowest-cost or best-performing providers
5. **Marketplace Integration**: Seamless failover between local and marketplace providers
6. **Enterprise Ready**: Supports complex organizational hierarchies
7. **Audit Trail**: Complete tracking of all token usage and costs

## 9. Migration Path

### Phase 1: Provider Management
- Implement provider CRUD operations
- Add health monitoring
- Basic load balancing

### Phase 2: Enhanced Billing
- Add cost tracking fields
- Implement aggregation views
- Build billing APIs

### Phase 3: Optimization
- Add cost optimizer
- Implement intelligent routing
- Add predictive budgeting

### Phase 4: Full Integration
- Marketplace integration
- Advanced analytics
- Real-time cost alerts