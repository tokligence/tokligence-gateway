package ratelimit

import (
	"context"
	"sync"
	"time"
)

// MemoryStore implements an in-memory rate limit store using token buckets.
// Suitable for single-instance deployments. For distributed deployments, use RedisStore.
type MemoryStore struct {
	userBuckets   map[int64]*TokenBucket
	apiKeyBuckets map[int64]*TokenBucket
	mu            sync.RWMutex

	// Cleanup
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewMemoryStore creates a new in-memory rate limit store.
func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithCleanup(5 * time.Minute)
}

// NewMemoryStoreWithCleanup creates a new in-memory store with custom cleanup interval.
func NewMemoryStoreWithCleanup(cleanupInterval time.Duration) *MemoryStore {
	s := &MemoryStore{
		userBuckets:     make(map[int64]*TokenBucket),
		apiKeyBuckets:   make(map[int64]*TokenBucket),
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup
	go s.cleanupLoop()

	return s
}

// AllowUser checks if a request from the user should be allowed.
func (s *MemoryStore) AllowUser(ctx context.Context, userID int64, capacity, refillRate float64) (bool, float64, error) {
	bucket := s.getUserBucket(userID, capacity, refillRate)
	allowed := bucket.Allow()
	remaining := bucket.Remaining()
	return allowed, remaining, nil
}

// AllowAPIKey checks if a request from the API key should be allowed.
func (s *MemoryStore) AllowAPIKey(ctx context.Context, apiKeyID int64, capacity, refillRate float64) (bool, float64, error) {
	bucket := s.getAPIKeyBucket(apiKeyID, capacity, refillRate)
	allowed := bucket.Allow()
	remaining := bucket.Remaining()
	return allowed, remaining, nil
}

// ResetUser resets the rate limit for a user.
func (s *MemoryStore) ResetUser(ctx context.Context, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bucket, exists := s.userBuckets[userID]; exists {
		bucket.Reset()
	}
	return nil
}

// ResetAPIKey resets the rate limit for an API key.
func (s *MemoryStore) ResetAPIKey(ctx context.Context, apiKeyID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if bucket, exists := s.apiKeyBuckets[apiKeyID]; exists {
		bucket.Reset()
	}
	return nil
}

// GetUserRemaining returns remaining tokens for a user.
func (s *MemoryStore) GetUserRemaining(ctx context.Context, userID int64, capacity, refillRate float64) (float64, error) {
	bucket := s.getUserBucket(userID, capacity, refillRate)
	return bucket.Remaining(), nil
}

// GetAPIKeyRemaining returns remaining tokens for an API key.
func (s *MemoryStore) GetAPIKeyRemaining(ctx context.Context, apiKeyID int64, capacity, refillRate float64) (float64, error) {
	bucket := s.getAPIKeyBucket(apiKeyID, capacity, refillRate)
	return bucket.Remaining(), nil
}

// Close stops background cleanup.
func (s *MemoryStore) Close() error {
	close(s.stopCleanup)
	return nil
}

// getUserBucket gets or creates a token bucket for the user.
func (s *MemoryStore) getUserBucket(userID int64, capacity, refillRate float64) *TokenBucket {
	s.mu.RLock()
	bucket, exists := s.userBuckets[userID]
	s.mu.RUnlock()

	if exists {
		return bucket
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists = s.userBuckets[userID]; exists {
		return bucket
	}

	bucket = NewTokenBucket(capacity, refillRate)
	s.userBuckets[userID] = bucket
	return bucket
}

// getAPIKeyBucket gets or creates a token bucket for the API key.
func (s *MemoryStore) getAPIKeyBucket(apiKeyID int64, capacity, refillRate float64) *TokenBucket {
	s.mu.RLock()
	bucket, exists := s.apiKeyBuckets[apiKeyID]
	s.mu.RUnlock()

	if exists {
		return bucket
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists = s.apiKeyBuckets[apiKeyID]; exists {
		return bucket
	}

	bucket = NewTokenBucket(capacity, refillRate)
	s.apiKeyBuckets[apiKeyID] = bucket
	return bucket
}

// cleanupLoop periodically removes buckets that are full (inactive).
func (s *MemoryStore) cleanupLoop() {
	if s.cleanupInterval <= 0 {
		return
	}

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCleanup:
			return
		}
	}
}

// cleanup removes inactive buckets to prevent memory leaks.
func (s *MemoryStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove buckets that are close to full capacity (inactive for a while)
	// We use 95% threshold to account for recent refills
	for userID, bucket := range s.userBuckets {
		remaining := bucket.Remaining()
		capacity := bucket.capacity
		if remaining >= capacity*0.95 {
			delete(s.userBuckets, userID)
		}
	}

	for apiKeyID, bucket := range s.apiKeyBuckets {
		remaining := bucket.Remaining()
		capacity := bucket.capacity
		if remaining >= capacity*0.95 {
			delete(s.apiKeyBuckets, apiKeyID)
		}
	}
}

// GetStats returns current statistics about the store.
type StoreStats struct {
	ActiveUserBuckets   int
	ActiveAPIKeyBuckets int
}

// GetStats returns current statistics.
func (s *MemoryStore) GetStats() StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return StoreStats{
		ActiveUserBuckets:   len(s.userBuckets),
		ActiveAPIKeyBuckets: len(s.apiKeyBuckets),
	}
}
