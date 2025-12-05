package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a thread-safe token bucket algorithm for rate limiting.
// The bucket is refilled at a constant rate and allows bursts up to the bucket capacity.
type TokenBucket struct {
	capacity   float64    // Maximum tokens in bucket
	refillRate float64    // Tokens added per second
	tokens     float64    // Current tokens available
	lastRefill time.Time  // Last time bucket was refilled
	mu         sync.Mutex // Thread safety
}

// NewTokenBucket creates a new token bucket rate limiter.
//   - capacity: maximum number of tokens (burst size)
//   - refillRate: tokens added per second (sustained rate)
//
// Example:
//   - capacity=100, refillRate=10 allows 100 requests immediately, then 10/sec sustained
func NewTokenBucket(capacity, refillRate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		refillRate: refillRate,
		tokens:     capacity, // Start with full bucket
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed based on available tokens.
// Returns true if allowed, false if rate limited.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN checks if N tokens are available and consumes them if so.
// Useful for operations with different costs (e.g., batch requests).
func (tb *TokenBucket) AllowN(n float64) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// Remaining returns the number of tokens currently available.
func (tb *TokenBucket) Remaining() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return tb.tokens
}

// Reset resets the bucket to full capacity.
func (tb *TokenBucket) Reset() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.tokens = tb.capacity
	tb.lastRefill = time.Now()
}

// refill adds tokens based on elapsed time since last refill.
// Must be called with lock held.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// Calculate tokens to add
	tokensToAdd := elapsed * tb.refillRate
	tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
	tb.lastRefill = now
}

// WaitTime returns the duration until a token will be available.
// Returns 0 if tokens are currently available.
func (tb *TokenBucket) WaitTime() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		return 0
	}

	// Calculate time needed to refill 1 token
	tokensNeeded := 1 - tb.tokens
	secondsNeeded := tokensNeeded / tb.refillRate
	return time.Duration(secondsNeeded * float64(time.Second))
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
