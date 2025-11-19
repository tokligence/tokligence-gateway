package metrics

import (
	"sync"
	"time"
)

// Collector collects and exports metrics for Prometheus.
// This implementation uses manual metric tracking without external dependencies.
// For production, consider integrating prometheus/client_golang.
type Collector struct {
	mu sync.RWMutex

	// Request metrics
	totalRequests      map[string]int64  // by endpoint
	totalRequestsDur   map[string]int64  // total duration in ms
	requestErrors      map[string]int64  // by endpoint
	requestsInProgress map[string]int64  // current in-flight requests

	// Rate limit metrics
	rateLimitHits  int64 // total rate limit rejections
	rateLimitByKey map[string]int64 // rate limits by user/apikey

	// Token usage metrics
	totalPromptTokens     int64
	totalCompletionTokens int64
	tokensByModel         map[string]int64 // total tokens by model
	tokensByUser          map[string]int64 // total tokens by user

	// Adapter metrics
	adapterRequests map[string]int64 // requests by adapter (openai, anthropic, etc.)
	adapterErrors   map[string]int64 // errors by adapter
	adapterLatency  map[string]int64 // total latency in ms by adapter

	// System metrics
	startTime time.Time
}

// NewCollector creates a new metrics collector.
func NewCollector() *Collector {
	return &Collector{
		totalRequests:      make(map[string]int64),
		totalRequestsDur:   make(map[string]int64),
		requestErrors:      make(map[string]int64),
		requestsInProgress: make(map[string]int64),
		rateLimitByKey:     make(map[string]int64),
		tokensByModel:      make(map[string]int64),
		tokensByUser:       make(map[string]int64),
		adapterRequests:    make(map[string]int64),
		adapterErrors:      make(map[string]int64),
		adapterLatency:     make(map[string]int64),
		startTime:          time.Now(),
	}
}

// RecordRequest records a request to an endpoint.
func (c *Collector) RecordRequest(endpoint string, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests[endpoint]++
	c.totalRequestsDur[endpoint] += duration.Milliseconds()
}

// RecordError records an error for an endpoint.
func (c *Collector) RecordError(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestErrors[endpoint]++
}

// RecordRequestStart increments in-progress requests.
func (c *Collector) RecordRequestStart(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestsInProgress[endpoint]++
}

// RecordRequestEnd decrements in-progress requests.
func (c *Collector) RecordRequestEnd(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.requestsInProgress[endpoint]--
}

// RecordRateLimitHit records a rate limit rejection.
func (c *Collector) RecordRateLimitHit(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rateLimitHits++
	c.rateLimitByKey[key]++
}

// RecordTokenUsage records token usage.
func (c *Collector) RecordTokenUsage(model, userID string, promptTokens, completionTokens int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalPromptTokens += promptTokens
	c.totalCompletionTokens += completionTokens

	if model != "" {
		c.tokensByModel[model] += (promptTokens + completionTokens)
	}
	if userID != "" {
		c.tokensByUser[userID] += (promptTokens + completionTokens)
	}
}

// RecordAdapterRequest records a request to an adapter.
func (c *Collector) RecordAdapterRequest(adapter string, duration time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.adapterRequests[adapter]++
	c.adapterLatency[adapter] += duration.Milliseconds()

	if err != nil {
		c.adapterErrors[adapter]++
	}
}

// Snapshot returns a point-in-time snapshot of all metrics.
type Snapshot struct {
	Uptime               int64
	TotalRequests        map[string]int64
	TotalRequestsDur     map[string]int64
	RequestErrors        map[string]int64
	RequestsInProgress   map[string]int64
	RateLimitHits        int64
	RateLimitByKey       map[string]int64
	TotalPromptTokens    int64
	TotalCompletionTokens int64
	TokensByModel        map[string]int64
	TokensByUser         map[string]int64
	AdapterRequests      map[string]int64
	AdapterErrors        map[string]int64
	AdapterLatency       map[string]int64
}

// GetSnapshot returns a snapshot of current metrics.
func (c *Collector) GetSnapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Snapshot{
		Uptime:                int64(time.Since(c.startTime).Seconds()),
		TotalRequests:         copyMap(c.totalRequests),
		TotalRequestsDur:      copyMap(c.totalRequestsDur),
		RequestErrors:         copyMap(c.requestErrors),
		RequestsInProgress:    copyMap(c.requestsInProgress),
		RateLimitHits:         c.rateLimitHits,
		RateLimitByKey:        copyMap(c.rateLimitByKey),
		TotalPromptTokens:     c.totalPromptTokens,
		TotalCompletionTokens: c.totalCompletionTokens,
		TokensByModel:         copyMap(c.tokensByModel),
		TokensByUser:          copyMap(c.tokensByUser),
		AdapterRequests:       copyMap(c.adapterRequests),
		AdapterErrors:         copyMap(c.adapterErrors),
		AdapterLatency:        copyMap(c.adapterLatency),
	}
}

func copyMap(m map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
