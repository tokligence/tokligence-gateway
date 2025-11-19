package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestLimiter_AllowUser(t *testing.T) {
	cfg := Config{
		UserRequestsPerSecond: 10,
		UserBurstSize:         10,
	}
	limiter := NewLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()
	userID := int64(1)

	// Should allow first 10 requests
	for i := 0; i < 10; i++ {
		if !limiter.AllowUser(ctx, userID) {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 11th should be denied
	if limiter.AllowUser(ctx, userID) {
		t.Error("11th request should be denied")
	}

	// Different user should have separate limit
	userID2 := int64(2)
	if !limiter.AllowUser(ctx, userID2) {
		t.Error("different user should be allowed")
	}
}

func TestLimiter_AllowAPIKey(t *testing.T) {
	cfg := Config{
		APIKeyRequestsPerSecond: 5,
		APIKeyBurstSize:         5,
	}
	limiter := NewLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()
	apiKeyID := int64(100)

	// Should allow first 5 requests
	for i := 0; i < 5; i++ {
		if !limiter.AllowAPIKey(ctx, apiKeyID) {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 6th should be denied
	if limiter.AllowAPIKey(ctx, apiKeyID) {
		t.Error("6th request should be denied")
	}
}

func TestLimiter_Allow(t *testing.T) {
	cfg := Config{
		UserRequestsPerSecond:   10,
		UserBurstSize:           10,
		APIKeyRequestsPerSecond: 5,
		APIKeyBurstSize:         5,
	}
	limiter := NewLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()
	userID := int64(1)
	apiKeyID := int64(100)

	// Should allow first 5 (limited by API key)
	for i := 0; i < 5; i++ {
		if !limiter.Allow(ctx, userID, apiKeyID) {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 6th should be denied (API key limit)
	if limiter.Allow(ctx, userID, apiKeyID) {
		t.Error("6th request should be denied by API key limit")
	}

	// User should still have tokens available
	if remaining := limiter.GetUserRemaining(userID); remaining < 4 {
		t.Errorf("user should have ~5 tokens remaining, got %f", remaining)
	}
}

func TestLimiter_Reset(t *testing.T) {
	cfg := Config{
		UserRequestsPerSecond: 10,
		UserBurstSize:         10,
	}
	limiter := NewLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()
	userID := int64(1)

	// Consume all tokens
	for i := 0; i < 10; i++ {
		limiter.AllowUser(ctx, userID)
	}

	// Should be denied
	if limiter.AllowUser(ctx, userID) {
		t.Error("should be denied before reset")
	}

	// Reset
	limiter.ResetUser(userID)

	// Should be allowed again
	if !limiter.AllowUser(ctx, userID) {
		t.Error("should be allowed after reset")
	}
}

func TestLimiter_GetRemaining(t *testing.T) {
	cfg := Config{
		UserRequestsPerSecond: 100,
		UserBurstSize:         100,
	}
	limiter := NewLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()
	userID := int64(1)

	// Initial
	if remaining := limiter.GetUserRemaining(userID); remaining != 100 {
		t.Errorf("expected 100 remaining, got %f", remaining)
	}

	// After consuming 30
	for i := 0; i < 30; i++ {
		limiter.AllowUser(ctx, userID)
	}

	remaining := limiter.GetUserRemaining(userID)
	if remaining < 69.9 || remaining > 70.1 {
		t.Errorf("expected ~70 remaining, got %f", remaining)
	}
}

func TestLimiter_ZeroID(t *testing.T) {
	cfg := DefaultConfig()
	limiter := NewLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()

	// Zero IDs should always be allowed
	if !limiter.AllowUser(ctx, 0) {
		t.Error("user ID 0 should be allowed")
	}

	if !limiter.AllowAPIKey(ctx, 0) {
		t.Error("API key ID 0 should be allowed")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.UserRequestsPerSecond != 100 {
		t.Errorf("expected UserRequestsPerSecond=100, got %f", cfg.UserRequestsPerSecond)
	}

	if cfg.UserBurstSize != 200 {
		t.Errorf("expected UserBurstSize=200, got %f", cfg.UserBurstSize)
	}

	if cfg.APIKeyRequestsPerSecond != 50 {
		t.Errorf("expected APIKeyRequestsPerSecond=50, got %f", cfg.APIKeyRequestsPerSecond)
	}

	if cfg.APIKeyBurstSize != 100 {
		t.Errorf("expected APIKeyBurstSize=100, got %f", cfg.APIKeyBurstSize)
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStoreWithCleanup(100 * time.Millisecond)
	defer store.Close()

	ctx := context.Background()

	// Create buckets for 10 users
	for i := int64(1); i <= 10; i++ {
		store.AllowUser(ctx, i, 100, 100)
	}

	stats := store.GetStats()
	if stats.ActiveUserBuckets != 10 {
		t.Errorf("expected 10 active buckets, got %d", stats.ActiveUserBuckets)
	}

	// Wait for cleanup (buckets should be removed as they're full/inactive)
	time.Sleep(200 * time.Millisecond)

	stats = store.GetStats()
	if stats.ActiveUserBuckets != 0 {
		t.Errorf("expected 0 active buckets after cleanup, got %d", stats.ActiveUserBuckets)
	}
}
