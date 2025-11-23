package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// PriorityTier represents the priority level (0 = highest)
type PriorityTier int

// Default priority tier constants for 10-level system
const (
	PriorityCritical    PriorityTier = 0 // P0: Internal critical services
	PriorityUrgent      PriorityTier = 1 // P1: High-value urgent requests
	PriorityHigh        PriorityTier = 2 // P2: Partner/premium users
	PriorityElevated    PriorityTier = 3 // P3: Elevated priority
	PriorityAboveNormal PriorityTier = 4 // P4: Above normal
	PriorityNormal      PriorityTier = 5 // P5: Standard users (default)
	PriorityBelowNormal PriorityTier = 6 // P6: Below normal
	PriorityLow         PriorityTier = 7 // P7: Low priority
	PriorityBulk        PriorityTier = 8 // P8: Bulk/batch processing
	PriorityBackground  PriorityTier = 9 // P9: Background jobs

	DefaultPriorityLevels = 10 // Default number of priority buckets
)

// Request represents a queued request
type Request struct {
	ID              string
	Priority        PriorityTier
	EstimatedTokens int64
	EnqueuedAt      time.Time
	Deadline        time.Time // EnqueuedAt + Timeout
	Environment     string
	AccountID       string
	Model           string

	// Response notification
	ResultChan chan *ScheduleResult
}

// ScheduleResult is the result of scheduling a request
type ScheduleResult struct {
	Accepted bool
	Reason   string
	QueuePos int
}

// Queue represents a single FIFO queue
type Queue struct {
	items  []*Request
	mu     sync.RWMutex
	maxLen int
}

// NewQueue creates a new FIFO queue
func NewQueue(maxLen int) *Queue {
	return &Queue{
		items:  make([]*Request, 0, maxLen),
		maxLen: maxLen,
	}
}

// Push adds a request to the queue
func (q *Queue) Push(req *Request) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) >= q.maxLen {
		return fmt.Errorf("queue full (max=%d)", q.maxLen)
	}

	q.items = append(q.items, req)
	log.Printf("[DEBUG] Queue.Push: Added request %s, queue depth now %d/%d", req.ID, len(q.items), q.maxLen)
	return nil
}

// Pop removes and returns the first request
func (q *Queue) Pop() *Request {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil
	}

	req := q.items[0]
	q.items = q.items[1:]
	log.Printf("[DEBUG] Queue.Pop: Removed request %s, queue depth now %d/%d", req.ID, len(q.items), q.maxLen)
	return req
}

// Len returns the current queue length
func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.items)
}

// IsEmpty checks if queue is empty
func (q *Queue) IsEmpty() bool {
	return q.Len() == 0
}

// PriorityQueue manages multiple priority levels
type PriorityQueue struct {
	queues []*Queue // Dynamic number of queues (configurable)
	config *Config

	// Statistics
	stats struct {
		totalEnqueued uint64
		totalDequeued uint64
		totalDropped  uint64
		totalExpired  uint64
		mu            sync.RWMutex
	}

	mu sync.RWMutex
}

// Config holds the priority queue configuration
type Config struct {
	NumPriorityLevels int          // Number of priority buckets (default: 10)
	DefaultPriority   PriorityTier // Default priority for requests (default: 5)
	MaxQueueDepth     int          // Max depth per queue (default: 1000)
	QueueTimeout      time.Duration // How long to wait before expiring (default: 30s)
	Weights           []float64    // WFQ weights per level (optional)
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		NumPriorityLevels: DefaultPriorityLevels,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     1000,
		QueueTimeout:      30 * time.Second,
		Weights: []float64{
			256, 128, 64, 32, 16, 8, 4, 2, 1, 1, // P0-P9 default weights
		},
	}
}

// NewPriorityQueue creates a new priority queue with configurable levels
func NewPriorityQueue(config *Config) *PriorityQueue {
	if config == nil {
		config = DefaultConfig()
	}

	log.Printf("[INFO] PriorityQueue: Initializing with %d priority levels (P0-P%d), max_depth=%d, timeout=%v",
		config.NumPriorityLevels, config.NumPriorityLevels-1, config.MaxQueueDepth, config.QueueTimeout)

	// Create dynamic number of queues
	queues := make([]*Queue, config.NumPriorityLevels)
	for i := 0; i < config.NumPriorityLevels; i++ {
		queues[i] = NewQueue(config.MaxQueueDepth)
		log.Printf("[DEBUG] PriorityQueue: Created queue P%d (max_depth=%d)", i, config.MaxQueueDepth)
	}

	// Ensure weights match priority levels
	if len(config.Weights) != config.NumPriorityLevels {
		log.Printf("[WARN] PriorityQueue: Weights length mismatch (%d != %d), using default exponential weights",
			len(config.Weights), config.NumPriorityLevels)
		config.Weights = make([]float64, config.NumPriorityLevels)
		for i := 0; i < config.NumPriorityLevels; i++ {
			// Use exponential weights: 2^(NumLevels-i-1)
			shift := uint(config.NumPriorityLevels - i - 1)
			config.Weights[i] = float64(int(1) << shift)
		}
	}

	log.Printf("[DEBUG] PriorityQueue: Weights=%v", config.Weights)

	return &PriorityQueue{
		queues: queues,
		config: config,
	}
}

// Enqueue adds a request to the appropriate priority queue
func (pq *PriorityQueue) Enqueue(req *Request) error {
	priority := req.Priority
	if priority < 0 {
		priority = pq.config.DefaultPriority
		log.Printf("[DEBUG] PriorityQueue.Enqueue: Request %s has negative priority, using default P%d", req.ID, priority)
	}

	// Validate priority range
	if int(priority) >= pq.config.NumPriorityLevels {
		return fmt.Errorf("invalid priority %d (max: P%d)", priority, pq.config.NumPriorityLevels-1)
	}

	// Set deadline if not set
	if req.Deadline.IsZero() {
		req.Deadline = time.Now().Add(pq.config.QueueTimeout)
	}

	// Set enqueued time
	if req.EnqueuedAt.IsZero() {
		req.EnqueuedAt = time.Now()
	}

	log.Printf("[DEBUG] PriorityQueue.Enqueue: Request %s → P%d queue (account=%s, model=%s, tokens=%d, deadline=%v)",
		req.ID, priority, req.AccountID, req.Model, req.EstimatedTokens, req.Deadline)

	// Enqueue to the appropriate priority queue
	err := pq.queues[priority].Push(req)
	if err != nil {
		pq.stats.mu.Lock()
		pq.stats.totalDropped++
		pq.stats.mu.Unlock()
		log.Printf("[WARN] PriorityQueue.Enqueue: Failed to enqueue request %s to P%d: %v (total_dropped=%d)",
			req.ID, priority, err, pq.stats.totalDropped)
		return err
	}

	pq.stats.mu.Lock()
	pq.stats.totalEnqueued++
	totalEnqueued := pq.stats.totalEnqueued
	pq.stats.mu.Unlock()

	log.Printf("[INFO] PriorityQueue.Enqueue: ✓ Request %s enqueued to P%d (total_enqueued=%d, queue_depth=%d)",
		req.ID, priority, totalEnqueued, pq.queues[priority].Len())

	return nil
}

// Dequeue removes a request from the highest priority non-empty queue (Strict Priority)
func (pq *PriorityQueue) Dequeue() (*Request, error) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Scan from highest priority (P0) to lowest
	for i := 0; i < pq.config.NumPriorityLevels; i++ {
		q := pq.queues[i]
		if q.IsEmpty() {
			continue
		}

		req := q.Pop()
		if req == nil {
			continue
		}

		// Check if request has expired
		if time.Now().After(req.Deadline) {
			pq.stats.mu.Lock()
			pq.stats.totalExpired++
			pq.stats.mu.Unlock()
			log.Printf("[WARN] PriorityQueue.Dequeue: Request %s from P%d expired (waited %v, deadline=%v, total_expired=%d)",
				req.ID, i, time.Since(req.EnqueuedAt), req.Deadline, pq.stats.totalExpired)
			continue
		}

		pq.stats.mu.Lock()
		pq.stats.totalDequeued++
		totalDequeued := pq.stats.totalDequeued
		pq.stats.mu.Unlock()

		waitTime := time.Since(req.EnqueuedAt)
		log.Printf("[INFO] PriorityQueue.Dequeue: ✓ Request %s dequeued from P%d (waited=%v, total_dequeued=%d)",
			req.ID, i, waitTime, totalDequeued)

		return req, nil
	}

	// All queues empty
	return nil, fmt.Errorf("no requests in queue")
}

// GetStats returns queue statistics
func (pq *PriorityQueue) GetStats() map[string]interface{} {
	pq.stats.mu.RLock()
	defer pq.stats.mu.RUnlock()

	depths := make([]int, pq.config.NumPriorityLevels)
	for i := 0; i < pq.config.NumPriorityLevels; i++ {
		depths[i] = pq.queues[i].Len()
	}

	return map[string]interface{}{
		"total_enqueued": pq.stats.totalEnqueued,
		"total_dequeued": pq.stats.totalDequeued,
		"total_dropped":  pq.stats.totalDropped,
		"total_expired":  pq.stats.totalExpired,
		"queue_depths":   depths,
		"priority_levels": pq.config.NumPriorityLevels,
	}
}

// LogStats logs current queue statistics
func (pq *PriorityQueue) LogStats() {
	stats := pq.GetStats()
	depths := stats["queue_depths"].([]int)

	log.Printf("[INFO] PriorityQueue Stats: enqueued=%d, dequeued=%d, dropped=%d, expired=%d",
		stats["total_enqueued"], stats["total_dequeued"], stats["total_dropped"], stats["total_expired"])

	for i, depth := range depths {
		if depth > 0 {
			log.Printf("[INFO]   P%d: %d requests waiting", i, depth)
		}
	}
}
