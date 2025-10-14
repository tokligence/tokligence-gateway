# API Key Model Management Improvement Proposal

## Current State

The current Tokligence Gateway API key structure only supports:
- Basic authentication (key hash, prefix)
- Generic scopes array
- Expiration timestamp

## Proposed Improvements (Based on LiteLLM Analysis)

### 1. Model Access Control

**Current:** No model-level restrictions
**Proposed:** Add explicit model access control

```go
type APIKey struct {
    // Existing fields...

    // New fields for model management
    AllowedModels  []string          `json:"allowed_models"`  // Empty = all models allowed
    BlockedModels  []string          `json:"blocked_models"`  // Explicit deny list
    ModelAliases   map[string]string `json:"model_aliases"`   // Key-specific model remapping
}
```

**Benefits:**
- Fine-grained control over which models each API key can access
- Support for model upgrade/downgrade scenarios via aliases
- Explicit deny list for sensitive models

### 2. Rate Limiting Per Key

**Current:** No rate limiting at API key level
**Proposed:** Add rate limit fields

```go
type APIKey struct {
    // ...

    // Rate limiting
    RPMLimit int64 `json:"rpm_limit"` // Requests per minute
    TPMLimit int64 `json:"tpm_limit"` // Tokens per minute
    MaxParallelRequests int `json:"max_parallel_requests"`
}
```

**Benefits:**
- Prevent abuse at the key level
- Different rate limits for different use cases
- Better resource management

### 3. Budget Management

**Current:** No budget tracking
**Proposed:** Add budget controls

```go
type APIKey struct {
    // ...

    // Budget management
    MaxBudget       float64              `json:"max_budget"`
    BudgetDuration  string               `json:"budget_duration"` // "30d", "1h", etc.
    BudgetResetAt   *time.Time           `json:"budget_reset_at"`
    CurrentSpend    float64              `json:"current_spend"`
    ModelBudgets    map[string]BudgetConfig `json:"model_budgets"` // Per-model budgets
}

type BudgetConfig struct {
    BudgetLimit float64 `json:"budget_limit"`
    TimePeriod  string  `json:"time_period"`
}
```

**Benefits:**
- Cost control at key level
- Automatic budget reset cycles
- Model-specific budget limits

### 4. Enhanced Metadata

**Current:** No metadata support
**Proposed:** Add structured metadata

```go
type APIKey struct {
    // ...

    // Enhanced metadata
    Name        string                 `json:"name"`        // Human-readable name
    Description string                 `json:"description"` // Purpose of the key
    Tags        []string               `json:"tags"`        // For organization
    Metadata    map[string]interface{} `json:"metadata"`    // Flexible key-value pairs
    LastUsedAt  *time.Time            `json:"last_used_at"`
}
```

**Benefits:**
- Better key organization and management
- Track key usage patterns
- Store application-specific data

### 5. Implementation in SQLite/PostgreSQL

#### Schema Updates

```sql
-- Add to api_keys table
ALTER TABLE api_keys ADD COLUMN name TEXT;
ALTER TABLE api_keys ADD COLUMN description TEXT;
ALTER TABLE api_keys ADD COLUMN allowed_models TEXT; -- JSON array
ALTER TABLE api_keys ADD COLUMN blocked_models TEXT; -- JSON array
ALTER TABLE api_keys ADD COLUMN model_aliases TEXT; -- JSON object
ALTER TABLE api_keys ADD COLUMN rpm_limit INTEGER;
ALTER TABLE api_keys ADD COLUMN tpm_limit INTEGER;
ALTER TABLE api_keys ADD COLUMN max_parallel_requests INTEGER;
ALTER TABLE api_keys ADD COLUMN max_budget DECIMAL(10,2);
ALTER TABLE api_keys ADD COLUMN budget_duration TEXT;
ALTER TABLE api_keys ADD COLUMN budget_reset_at TIMESTAMP;
ALTER TABLE api_keys ADD COLUMN current_spend DECIMAL(10,2) DEFAULT 0;
ALTER TABLE api_keys ADD COLUMN model_budgets TEXT; -- JSON object
ALTER TABLE api_keys ADD COLUMN tags TEXT; -- JSON array
ALTER TABLE api_keys ADD COLUMN metadata TEXT; -- JSON object
ALTER TABLE api_keys ADD COLUMN last_used_at TIMESTAMP;

-- Add indexes for performance
CREATE INDEX idx_api_keys_name ON api_keys(name);
CREATE INDEX idx_api_keys_last_used ON api_keys(last_used_at);
```

### 6. Model Access Validation

```go
func (k *APIKey) CanAccessModel(model string) bool {
    // If blocked models are defined, check deny list first
    if len(k.BlockedModels) > 0 {
        for _, blocked := range k.BlockedModels {
            if blocked == model || matchesWildcard(blocked, model) {
                return false
            }
        }
    }

    // If allowed models are defined, check allow list
    if len(k.AllowedModels) > 0 {
        for _, allowed := range k.AllowedModels {
            if allowed == model || matchesWildcard(allowed, model) {
                return true
            }
        }
        return false // Not in allow list
    }

    // No restrictions defined, allow access
    return true
}

func (k *APIKey) GetModelAlias(model string) string {
    if alias, exists := k.ModelAliases[model]; exists {
        return alias
    }
    return model
}
```

### 7. Rate Limit Enforcement

```go
type RateLimiter struct {
    requestCounts map[string]*sliding.Window
    tokenCounts   map[string]*sliding.Window
    parallelReqs  map[string]int64
}

func (r *RateLimiter) CheckAndIncrement(keyID string, key *APIKey, tokens int) error {
    // Check RPM limit
    if key.RPMLimit > 0 {
        if r.requestCounts[keyID].Count() >= key.RPMLimit {
            return ErrRateLimitExceeded
        }
    }

    // Check TPM limit
    if key.TPMLimit > 0 {
        if r.tokenCounts[keyID].Count() + tokens > key.TPMLimit {
            return ErrTokenLimitExceeded
        }
    }

    // Check parallel requests
    if key.MaxParallelRequests > 0 {
        if atomic.LoadInt64(&r.parallelReqs[keyID]) >= int64(key.MaxParallelRequests) {
            return ErrTooManyParallelRequests
        }
    }

    // Increment counters
    r.requestCounts[keyID].Add(1)
    r.tokenCounts[keyID].Add(tokens)
    atomic.AddInt64(&r.parallelReqs[keyID], 1)

    return nil
}
```

### 8. CLI Updates

```bash
# Create key with model restrictions
gateway admin api-keys create \
  --user 1 \
  --name "Production API Key" \
  --allowed-models "gpt-4,gpt-3.5-turbo" \
  --rpm-limit 100 \
  --tpm-limit 100000 \
  --max-budget 50.00 \
  --budget-duration "30d"

# Update key model access
gateway admin api-keys update \
  --id 123 \
  --allowed-models "gpt-4,claude-3-opus" \
  --blocked-models "gpt-4-32k"

# List keys with details
gateway admin api-keys list --verbose
```

## Migration Strategy

1. **Phase 1**: Add new columns with NULL defaults
2. **Phase 2**: Update API to support new fields
3. **Phase 3**: Migrate existing keys with sensible defaults
4. **Phase 4**: Enable enforcement of new features

## Benefits

1. **Security**: Fine-grained access control per API key
2. **Cost Control**: Budget limits prevent unexpected charges
3. **Performance**: Rate limiting prevents abuse
4. **Flexibility**: Model aliases support migration scenarios
5. **Observability**: Better tracking of key usage patterns
6. **Multi-tenancy**: Keys can be scoped to specific resources

## Compatibility

- Backward compatible (new fields are optional)
- Forward compatible (unknown fields ignored)
- Works with both SQLite and PostgreSQL
- Enterprise edition can extend with additional fields

## Marketplace Integration Layer

Since Tokligence Gateway can connect to the marketplace to both consume and supply tokens, we need additional fields beyond LiteLLM's design:

### 9. Marketplace-Specific Fields

```go
type APIKey struct {
    // ... existing fields ...

    // Marketplace integration
    MarketplaceRole     string   `json:"marketplace_role"`     // "consumer", "provider", "both"
    MarketplaceEnabled  bool     `json:"marketplace_enabled"`   // Can use marketplace

    // Provider capabilities (when selling tokens)
    ProvidedModels      []ModelOffering `json:"provided_models"`  // Models this key can offer
    ServiceLevel        string          `json:"service_level"`    // "bronze", "silver", "gold"
    MinPrice           float64          `json:"min_price"`        // Minimum price per 1k tokens
    MaxCapacityTPM     int64            `json:"max_capacity_tpm"` // Max tokens per minute to provide
    AvailabilityWindow  []TimeWindow    `json:"availability"`     // When provider is available

    // Consumer preferences (when buying tokens)
    PreferredProviders  []string        `json:"preferred_providers"` // Preferred provider list
    MaxPricePerModel   map[string]float64 `json:"max_price_per_model"` // Price limits
    QualityPreference  string          `json:"quality_preference"`  // "cost", "quality", "balanced"
    FallbackBehavior   string          `json:"fallback_behavior"`   // "local", "error", "queue"
}

type ModelOffering struct {
    ModelName       string  `json:"model_name"`
    ModelFamily     string  `json:"model_family"`
    PricePerKTokens float64 `json:"price_per_k_tokens"`
    MaxTPM         int64    `json:"max_tpm"`
    Capabilities   []string `json:"capabilities"` // ["chat", "completion", "embedding"]
    SLA            SLAConfig `json:"sla"`
}

type SLAConfig struct {
    AvailabilityPercent float64 `json:"availability_percent"` // 99.9
    MaxLatencyMs       int     `json:"max_latency_ms"`
    SupportTier        string  `json:"support_tier"`        // "basic", "priority"
}

type TimeWindow struct {
    DayOfWeek  string `json:"day_of_week"`  // "monday", "tuesday", etc.
    StartTime  string `json:"start_time"`   // "09:00"
    EndTime    string `json:"end_time"`     // "17:00"
    Timezone   string `json:"timezone"`     // "UTC", "PST", etc.
}
```

### 10. Marketplace Schema Extensions

```sql
-- Additional marketplace-specific columns
ALTER TABLE api_keys ADD COLUMN marketplace_role TEXT DEFAULT 'consumer';
ALTER TABLE api_keys ADD COLUMN marketplace_enabled BOOLEAN DEFAULT true;
ALTER TABLE api_keys ADD COLUMN provided_models TEXT; -- JSON array
ALTER TABLE api_keys ADD COLUMN service_level TEXT DEFAULT 'bronze';
ALTER TABLE api_keys ADD COLUMN min_price DECIMAL(10,4);
ALTER TABLE api_keys ADD COLUMN max_capacity_tpm BIGINT;
ALTER TABLE api_keys ADD COLUMN availability_window TEXT; -- JSON array
ALTER TABLE api_keys ADD COLUMN preferred_providers TEXT; -- JSON array
ALTER TABLE api_keys ADD COLUMN max_price_per_model TEXT; -- JSON object
ALTER TABLE api_keys ADD COLUMN quality_preference TEXT DEFAULT 'balanced';
ALTER TABLE api_keys ADD COLUMN fallback_behavior TEXT DEFAULT 'local';

-- Marketplace service registry (local cache of marketplace offerings)
CREATE TABLE marketplace_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_key_id INTEGER REFERENCES api_keys(id),
    service_id TEXT NOT NULL, -- Marketplace service ID
    model_name TEXT NOT NULL,
    model_family TEXT NOT NULL,
    price_per_k_tokens DECIMAL(10,4),
    status TEXT DEFAULT 'active', -- active, paused, inactive
    last_heartbeat TIMESTAMP,
    total_tokens_served BIGINT DEFAULT 0,
    total_revenue DECIMAL(10,2) DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_marketplace_services_api_key ON marketplace_services(api_key_id);
CREATE INDEX idx_marketplace_services_status ON marketplace_services(status);
```

### 11. Marketplace Routing Logic

```go
type MarketplaceRouter struct {
    localModels     map[string]ModelConfig
    marketplaceAPI  MarketplaceClient
    rateLimiter     *RateLimiter
}

func (r *MarketplaceRouter) RouteRequest(apiKey *APIKey, model string) (*RouteDecision, error) {
    // Check if key can access the model
    if !apiKey.CanAccessModel(model) {
        return nil, ErrModelNotAllowed
    }

    // Apply model alias if configured
    actualModel := apiKey.GetModelAlias(model)

    // Decision tree for routing
    switch apiKey.MarketplaceRole {
    case "provider":
        // This key only provides models, cannot consume
        return nil, ErrKeyIsProviderOnly

    case "consumer", "both":
        // Try local models first if available
        if local, exists := r.localModels[actualModel]; exists {
            if apiKey.FallbackBehavior != "marketplace_only" {
                return &RouteDecision{
                    Type: "local",
                    Model: local,
                }, nil
            }
        }

        // Check marketplace if enabled
        if apiKey.MarketplaceEnabled {
            providers := r.marketplaceAPI.FindProviders(
                actualModel,
                apiKey.PreferredProviders,
                apiKey.MaxPricePerModel[actualModel],
                apiKey.QualityPreference,
            )

            if len(providers) > 0 {
                return &RouteDecision{
                    Type: "marketplace",
                    Provider: providers[0],
                    Model: actualModel,
                    Price: providers[0].Price,
                }, nil
            }
        }

        // Fallback behavior
        switch apiKey.FallbackBehavior {
        case "error":
            return nil, ErrNoProvidersAvailable
        case "queue":
            return &RouteDecision{Type: "queue"}, nil
        default:
            return &RouteDecision{Type: "local_fallback"}, nil
        }
    }

    return nil, ErrInvalidConfiguration
}
```

### 12. Provider Registration

```go
func (g *Gateway) RegisterAsProvider(ctx context.Context, apiKey *APIKey) error {
    if apiKey.MarketplaceRole != "provider" && apiKey.MarketplaceRole != "both" {
        return ErrNotProviderKey
    }

    for _, offering := range apiKey.ProvidedModels {
        service := MarketplaceService{
            APIKeyID: apiKey.ID,
            Model: offering.ModelName,
            Family: offering.ModelFamily,
            PricePerKTokens: offering.PricePerKTokens,
            MaxTPM: offering.MaxTPM,
            Capabilities: offering.Capabilities,
            SLA: offering.SLA,
            AvailabilityWindow: apiKey.AvailabilityWindow,
        }

        serviceID, err := g.marketplaceClient.PublishService(ctx, service)
        if err != nil {
            return fmt.Errorf("failed to publish %s: %w", offering.ModelName, err)
        }

        // Store service registration locally
        g.ledger.RecordServiceRegistration(apiKey.ID, serviceID, offering)
    }

    return nil
}
```

### 13. Usage Tracking for Marketplace

```sql
-- Extended usage_entries for marketplace transactions
ALTER TABLE usage_entries ADD COLUMN transaction_type TEXT; -- 'consume', 'provide'
ALTER TABLE usage_entries ADD COLUMN marketplace_service_id TEXT;
ALTER TABLE usage_entries ADD COLUMN provider_id TEXT;
ALTER TABLE usage_entries ADD COLUMN consumer_id TEXT;
ALTER TABLE usage_entries ADD COLUMN marketplace_price DECIMAL(10,4);
ALTER TABLE usage_entries ADD COLUMN marketplace_fee DECIMAL(10,4);
ALTER TABLE usage_entries ADD COLUMN net_revenue DECIMAL(10,4);
```

## Benefits of Marketplace Layer

1. **Dual Role Support**: Keys can be consumers, providers, or both
2. **Dynamic Pricing**: Market-based token pricing
3. **Service Discovery**: Automatic provider selection
4. **Quality Control**: SLA enforcement and monitoring
5. **Revenue Generation**: Sell excess capacity
6. **Cost Optimization**: Buy tokens at best prices
7. **Resilience**: Fallback to marketplace when local unavailable

## Next Steps

1. Review and approve proposal
2. Implement schema changes
3. Update API key CRUD operations
4. Add marketplace routing logic
5. Implement provider registration
6. Add marketplace service discovery
7. Update CLI and documentation
8. Add tests for marketplace features