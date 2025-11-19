package ratelimit

import (
	"context"
)

// Store defines the interface for rate limit storage backends.
// Implementations can be in-memory (for single instance) or distributed (Redis, etc.).
type Store interface {
	// AllowUser checks if a request from the user should be allowed.
	AllowUser(ctx context.Context, userID int64, capacity, refillRate float64) (allowed bool, remaining float64, err error)

	// AllowAPIKey checks if a request from the API key should be allowed.
	AllowAPIKey(ctx context.Context, apiKeyID int64, capacity, refillRate float64) (allowed bool, remaining float64, err error)

	// ResetUser resets the rate limit for a user.
	ResetUser(ctx context.Context, userID int64) error

	// ResetAPIKey resets the rate limit for an API key.
	ResetAPIKey(ctx context.Context, apiKeyID int64) error

	// GetUserRemaining returns remaining tokens for a user.
	GetUserRemaining(ctx context.Context, userID int64, capacity, refillRate float64) (float64, error)

	// GetAPIKeyRemaining returns remaining tokens for an API key.
	GetAPIKeyRemaining(ctx context.Context, apiKeyID int64, capacity, refillRate float64) (float64, error)

	// Close releases resources.
	Close() error
}

// Limiter manages rate limits for users and API keys using a pluggable storage backend.
// For single-instance deployments, use MemoryStore (default).
// For distributed/clustered deployments, use RedisStore or other distributed implementations.
type Limiter struct {
	store Store

	// Default limits
	userCapacity     float64
	userRefillRate   float64
	apiKeyCapacity   float64
	apiKeyRefillRate float64
}

// Config holds configuration for the rate limiter.
type Config struct {
	// Storage backend (optional, defaults to MemoryStore)
	Store Store

	// User limits (per user_id)
	UserRequestsPerSecond float64 // Sustained rate
	UserBurstSize         float64 // Burst capacity

	// API key limits (per api_key_id)
	APIKeyRequestsPerSecond float64
	APIKeyBurstSize         float64
}

// DefaultConfig returns sensible production defaults.
func DefaultConfig() Config {
	return Config{
		// User: 100 req/sec sustained, 200 burst
		UserRequestsPerSecond: 100,
		UserBurstSize:         200,

		// API key: 50 req/sec sustained, 100 burst
		APIKeyRequestsPerSecond: 50,
		APIKeyBurstSize:         100,
	}
}

// NewLimiter creates a new rate limiter with the given configuration.
func NewLimiter(cfg Config) *Limiter {
	// Apply defaults
	if cfg.UserRequestsPerSecond <= 0 {
		cfg.UserRequestsPerSecond = 100
	}
	if cfg.UserBurstSize <= 0 {
		cfg.UserBurstSize = 200
	}
	if cfg.APIKeyRequestsPerSecond <= 0 {
		cfg.APIKeyRequestsPerSecond = 50
	}
	if cfg.APIKeyBurstSize <= 0 {
		cfg.APIKeyBurstSize = 100
	}

	// Default to MemoryStore if no store provided
	store := cfg.Store
	if store == nil {
		store = NewMemoryStore()
	}

	return &Limiter{
		store:            store,
		userCapacity:     cfg.UserBurstSize,
		userRefillRate:   cfg.UserRequestsPerSecond,
		apiKeyCapacity:   cfg.APIKeyBurstSize,
		apiKeyRefillRate: cfg.APIKeyRequestsPerSecond,
	}
}

// AllowUser checks if a request from the given user should be allowed.
func (l *Limiter) AllowUser(ctx context.Context, userID int64) bool {
	if userID == 0 {
		return true // No user ID, allow by default
	}

	allowed, _, err := l.store.AllowUser(ctx, userID, l.userCapacity, l.userRefillRate)
	if err != nil {
		// On error, allow the request (fail open)
		return true
	}
	return allowed
}

// AllowAPIKey checks if a request from the given API key should be allowed.
func (l *Limiter) AllowAPIKey(ctx context.Context, apiKeyID int64) bool {
	if apiKeyID == 0 {
		return true // No API key ID, allow by default
	}

	allowed, _, err := l.store.AllowAPIKey(ctx, apiKeyID, l.apiKeyCapacity, l.apiKeyRefillRate)
	if err != nil {
		// On error, allow the request (fail open)
		return true
	}
	return allowed
}

// Allow checks both user and API key limits.
// Returns true only if both limits allow the request.
func (l *Limiter) Allow(ctx context.Context, userID, apiKeyID int64) bool {
	userAllowed := l.AllowUser(ctx, userID)
	apiKeyAllowed := l.AllowAPIKey(ctx, apiKeyID)

	return userAllowed && apiKeyAllowed
}

// GetUserRemaining returns the number of tokens remaining for the user.
func (l *Limiter) GetUserRemaining(userID int64) float64 {
	if userID == 0 {
		return l.userCapacity
	}

	remaining, err := l.store.GetUserRemaining(context.Background(), userID, l.userCapacity, l.userRefillRate)
	if err != nil {
		return l.userCapacity // On error, return full capacity
	}
	return remaining
}

// GetAPIKeyRemaining returns the number of tokens remaining for the API key.
func (l *Limiter) GetAPIKeyRemaining(apiKeyID int64) float64 {
	if apiKeyID == 0 {
		return l.apiKeyCapacity
	}

	remaining, err := l.store.GetAPIKeyRemaining(context.Background(), apiKeyID, l.apiKeyCapacity, l.apiKeyRefillRate)
	if err != nil {
		return l.apiKeyCapacity // On error, return full capacity
	}
	return remaining
}

// ResetUser resets the rate limit for a specific user.
func (l *Limiter) ResetUser(userID int64) error {
	return l.store.ResetUser(context.Background(), userID)
}

// ResetAPIKey resets the rate limit for a specific API key.
func (l *Limiter) ResetAPIKey(apiKeyID int64) error {
	return l.store.ResetAPIKey(context.Background(), apiKeyID)
}

// Close stops the limiter and releases resources.
func (l *Limiter) Close() error {
	return l.store.Close()
}
