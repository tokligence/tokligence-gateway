package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CapacityDimension represents different capacity measurement dimensions
type CapacityDimension string

const (
	DimTokensPerSec  CapacityDimension = "tokens_per_sec" // PRIMARY metric
	DimRPS           CapacityDimension = "rps"            // SECONDARY metric
	DimConcurrent    CapacityDimension = "concurrent"     // SECONDARY metric
	DimContextLength CapacityDimension = "context_length" // SECONDARY metric
)

// Capacity defines resource limits for a provider
type Capacity struct {
	// PRIMARY metric (most important for LLM scheduling)
	MaxTokensPerSec int // tokens/sec capacity

	// SECONDARY metrics
	MaxRPS           int // requests/sec (fallback if tokens/sec unknown)
	MaxConcurrent    int // max concurrent requests
	MaxContextLength int // max context window

	// Dynamic tracking
	currentTokensPerSec int64
	currentRPS          int64
	currentConcurrent   int
	mu                  sync.RWMutex

	// Time window for rate tracking (1 second window)
	windowStart    time.Time
	windowRequests int64
	windowTokens   int64
}

// CapacityLimits represents configurable capacity ceilings
type CapacityLimits struct {
	MaxTokensPerSec  int
	MaxRPS           int
	MaxConcurrent    int
	MaxContextLength int
}

// NewCapacity creates a new capacity tracker
func NewCapacity(maxTokensPerSec, maxRPS, maxConcurrent, maxContextLength int) *Capacity {
	log.Printf("[INFO] Capacity: Initializing with max_tokens_per_sec=%d, max_rps=%d, max_concurrent=%d, max_context=%d",
		maxTokensPerSec, maxRPS, maxConcurrent, maxContextLength)

	return &Capacity{
		MaxTokensPerSec:  maxTokensPerSec,
		MaxRPS:           maxRPS,
		MaxConcurrent:    maxConcurrent,
		MaxContextLength: maxContextLength,
		windowStart:      time.Now(),
	}
}

// CanAccept checks if the request can be accepted given current capacity
func (c *Capacity) CanAccept(req *Request) (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset window if > 1 second elapsed
	if time.Since(c.windowStart) > time.Second {
		log.Printf("[DEBUG] Capacity.CanAccept: Resetting rate window (old: %d req, %d tokens)",
			c.windowRequests, c.windowTokens)
		c.windowStart = time.Now()
		c.windowRequests = 0
		c.windowTokens = 0
	}

	// Check 1: Concurrent limit
	if c.MaxConcurrent > 0 && c.currentConcurrent >= c.MaxConcurrent {
		log.Printf("[DEBUG] Capacity.CanAccept: ✗ Request %s rejected - concurrent limit reached (%d/%d)",
			req.ID, c.currentConcurrent, c.MaxConcurrent)
		return false, fmt.Sprintf("concurrent limit reached (%d/%d)", c.currentConcurrent, c.MaxConcurrent)
	}

	// Check 2: Context length limit
	if c.MaxContextLength > 0 && int(req.EstimatedTokens) > c.MaxContextLength {
		log.Printf("[DEBUG] Capacity.CanAccept: ✗ Request %s rejected - context too long (%d > %d)",
			req.ID, req.EstimatedTokens, c.MaxContextLength)
		return false, fmt.Sprintf("context too long (%d > %d)", req.EstimatedTokens, c.MaxContextLength)
	}

	// Check 3: Tokens/sec limit (PRIMARY)
	if c.MaxTokensPerSec > 0 {
		projectedTokens := c.windowTokens + req.EstimatedTokens
		if projectedTokens > int64(c.MaxTokensPerSec) {
			log.Printf("[DEBUG] Capacity.CanAccept: ✗ Request %s rejected - tokens/sec limit (%d + %d = %d > %d)",
				req.ID, c.windowTokens, req.EstimatedTokens, projectedTokens, c.MaxTokensPerSec)
			return false, fmt.Sprintf("tokens/sec limit (%d/%d)", c.windowTokens, c.MaxTokensPerSec)
		}
	}

	// Check 4: RPS limit (SECONDARY fallback)
	if c.MaxRPS > 0 {
		projectedRPS := c.windowRequests + 1
		if projectedRPS > int64(c.MaxRPS) {
			log.Printf("[DEBUG] Capacity.CanAccept: ✗ Request %s rejected - RPS limit (%d + 1 > %d)",
				req.ID, c.windowRequests, c.MaxRPS)
			return false, fmt.Sprintf("RPS limit (%d/%d)", c.windowRequests, c.MaxRPS)
		}
	}

	log.Printf("[DEBUG] Capacity.CanAccept: ✓ Request %s can be accepted (concurrent=%d/%d, tokens_window=%d/%d, rps_window=%d/%d)",
		req.ID, c.currentConcurrent, c.MaxConcurrent,
		c.windowTokens, c.MaxTokensPerSec,
		c.windowRequests, c.MaxRPS)

	return true, ""
}

// Reserve reserves capacity for a request
func (c *Capacity) Reserve(req *Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.currentConcurrent++
	c.windowRequests++
	c.windowTokens += req.EstimatedTokens

	log.Printf("[DEBUG] Capacity.Reserve: Request %s reserved (concurrent=%d/%d, tokens_window=%d/%d, rps_window=%d/%d)",
		req.ID, c.currentConcurrent, c.MaxConcurrent,
		c.windowTokens, c.MaxTokensPerSec,
		c.windowRequests, c.MaxRPS)
}

// Release releases capacity after request completes
func (c *Capacity) Release(req *Request) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentConcurrent > 0 {
		c.currentConcurrent--
	}

	log.Printf("[DEBUG] Capacity.Release: Request %s released (concurrent=%d/%d)",
		req.ID, c.currentConcurrent, c.MaxConcurrent)
}

// GetUtilization returns current capacity utilization (0.0-1.0)
func (c *Capacity) GetUtilization() map[CapacityDimension]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	util := make(map[CapacityDimension]float64)

	if c.MaxConcurrent > 0 {
		util[DimConcurrent] = float64(c.currentConcurrent) / float64(c.MaxConcurrent)
	}

	if c.MaxTokensPerSec > 0 {
		util[DimTokensPerSec] = float64(c.windowTokens) / float64(c.MaxTokensPerSec)
	}

	if c.MaxRPS > 0 {
		util[DimRPS] = float64(c.windowRequests) / float64(c.MaxRPS)
	}

	return util
}

// LogUtilization logs current capacity utilization
func (c *Capacity) LogUtilization() {
	util := c.GetUtilization()

	log.Printf("[INFO] Capacity Utilization:")
	for dim, val := range util {
		percentage := val * 100
		log.Printf("[INFO]   %s: %.1f%%", dim, percentage)
	}

	c.mu.RLock()
	log.Printf("[INFO]   Current: %d concurrent, %d tokens/window, %d req/window",
		c.currentConcurrent, c.windowTokens, c.windowRequests)
	c.mu.RUnlock()
}

// UpdateLimits safely updates capacity ceilings
func (c *Capacity) UpdateLimits(maxTokensPerSec *int64, maxRPS *int, maxConcurrent *int, maxContextLength *int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if maxTokensPerSec != nil {
		c.MaxTokensPerSec = int(*maxTokensPerSec)
	}
	if maxRPS != nil {
		c.MaxRPS = *maxRPS
	}
	if maxConcurrent != nil {
		c.MaxConcurrent = *maxConcurrent
	}
	if maxContextLength != nil {
		c.MaxContextLength = *maxContextLength
	}
}

// CurrentLimits returns a snapshot of configured limits
func (c *Capacity) CurrentLimits() CapacityLimits {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CapacityLimits{
		MaxTokensPerSec:  c.MaxTokensPerSec,
		MaxRPS:           c.MaxRPS,
		MaxConcurrent:    c.MaxConcurrent,
		MaxContextLength: c.MaxContextLength,
	}
}

// CapacityGuardian manages capacity checks and reservations
type CapacityGuardian struct {
	capacity *Capacity
	mu       sync.RWMutex
}

// NewCapacityGuardian creates a new capacity guardian
func NewCapacityGuardian(capacity *Capacity) *CapacityGuardian {
	log.Printf("[INFO] CapacityGuardian: Initialized")
	return &CapacityGuardian{
		capacity: capacity,
	}
}

// CheckAndReserve checks if request can be accepted and reserves capacity if yes
func (cg *CapacityGuardian) CheckAndReserve(req *Request) (bool, bool, string) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	canAccept, reason := cg.capacity.CanAccept(req)
	if !canAccept {
		log.Printf("[WARN] CapacityGuardian: Request %s rejected by capacity check: %s", req.ID, reason)
		return false, cg.isFatalReject(req), reason
	}

	cg.capacity.Reserve(req)
	log.Printf("[INFO] CapacityGuardian: ✓ Request %s capacity reserved", req.ID)
	return true, false, ""
}

// Release releases capacity for a completed request
func (cg *CapacityGuardian) Release(req *Request) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	cg.capacity.Release(req)
	log.Printf("[INFO] CapacityGuardian: Request %s capacity released", req.ID)
}

// GetUtilization returns current capacity utilization
func (cg *CapacityGuardian) GetUtilization() map[CapacityDimension]float64 {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.capacity.GetUtilization()
}

// LogStats logs capacity statistics
func (cg *CapacityGuardian) LogStats() {
	cg.capacity.LogUtilization()
}

// isFatalReject returns true when a request can never be admitted, even after backoff.
func (cg *CapacityGuardian) isFatalReject(req *Request) bool {
	if cg.capacity.MaxContextLength > 0 && int(req.EstimatedTokens) > cg.capacity.MaxContextLength {
		return true
	}
	if cg.capacity.MaxTokensPerSec > 0 && req.EstimatedTokens > int64(cg.capacity.MaxTokensPerSec) {
		return true
	}
	return false
}
