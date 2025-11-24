package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// SchedulingPolicy defines the scheduling algorithm
type SchedulingPolicy string

const (
	PolicyStrictPriority SchedulingPolicy = "strict" // Always serve highest priority first
	PolicyWFQ            SchedulingPolicy = "wfq"    // Weighted Fair Queuing
	PolicyHybrid         SchedulingPolicy = "hybrid" // P0 strict, P1-P9 WFQ (recommended)
)

// Scheduler is the main request scheduler
type Scheduler struct {
	priorityQueue    *PriorityQueue
	capacityGuardian *CapacityGuardian
	policy           SchedulingPolicy

	// WFQ state (for WFQ and Hybrid policies)
	wfqDeficit map[int]float64
	wfqMu      sync.RWMutex

	// Background worker
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats struct {
		totalScheduled uint64
		totalRejected  uint64
		mu             sync.RWMutex
	}
}

// NewScheduler creates a new scheduler
func NewScheduler(config *Config, capacity *Capacity, policy SchedulingPolicy) *Scheduler {
	if policy == "" {
		policy = PolicyHybrid // Default to hybrid
	}

	log.Printf("[INFO] Scheduler: Initializing with policy=%s", policy)

	pq := NewPriorityQueue(config)
	cg := NewCapacityGuardian(capacity)

	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		priorityQueue:    pq,
		capacityGuardian: cg,
		policy:           policy,
		wfqDeficit:       make(map[int]float64),
		ctx:              ctx,
		cancel:           cancel,
	}

	// Initialize WFQ deficit counters
	for i := 0; i < config.NumPriorityLevels; i++ {
		s.wfqDeficit[i] = 0.0
	}

	// Start background worker
	s.wg.Add(1)
	go s.backgroundWorker()

	log.Printf("[INFO] Scheduler: ✓ Initialized and background worker started")

	return s
}

// Submit submits a request for scheduling
func (s *Scheduler) Submit(req *Request) error {
	log.Printf("[INFO] Scheduler.Submit: Request %s submitted (priority=P%d, tokens=%d, account=%s, model=%s)",
		req.ID, req.Priority, req.EstimatedTokens, req.AccountID, req.Model)

	// Check capacity first
	canAccept, fatal, reason := s.capacityGuardian.CheckAndReserve(req)
	if canAccept {
		// Capacity available - execute immediately
		s.stats.mu.Lock()
		s.stats.totalScheduled++
		totalScheduled := s.stats.totalScheduled
		s.stats.mu.Unlock()

		log.Printf("[INFO] Scheduler.Submit: ✓ Request %s accepted immediately (capacity available, total_scheduled=%d)",
			req.ID, totalScheduled)

		// Notify via result channel
		if req.ResultChan != nil {
			req.ResultChan <- &ScheduleResult{
				Accepted: true,
				Reason:   "capacity available",
				QueuePos: 0,
			}
		}

		return nil
	}

	// Fatal capacity failure (never schedulable) - reject immediately
	if fatal {
		s.stats.mu.Lock()
		s.stats.totalRejected++
		totalRejected := s.stats.totalRejected
		s.stats.mu.Unlock()

		log.Printf("[ERROR] Scheduler.Submit: ✗ Request %s rejected - unschedulable: %s (total_rejected=%d)",
			req.ID, reason, totalRejected)

		if req.ResultChan != nil {
			req.ResultChan <- &ScheduleResult{
				Accepted: false,
				Reason:   reason,
				QueuePos: -1,
			}
		}
		return fmt.Errorf("request exceeds capacity limits: %s", reason)
	}

	// No capacity - enqueue
	log.Printf("[INFO] Scheduler.Submit: No capacity for request %s, enqueueing to P%d (reason: %s)",
		req.ID, req.Priority, reason)

	err := s.priorityQueue.Enqueue(req)
	if err != nil {
		s.stats.mu.Lock()
		s.stats.totalRejected++
		totalRejected := s.stats.totalRejected
		s.stats.mu.Unlock()

		log.Printf("[ERROR] Scheduler.Submit: ✗ Request %s rejected - failed to enqueue: %v (total_rejected=%d)",
			req.ID, err, totalRejected)

		// Notify via result channel
		if req.ResultChan != nil {
			req.ResultChan <- &ScheduleResult{
				Accepted: false,
				Reason:   fmt.Sprintf("queue full: %v", err),
				QueuePos: -1,
			}
		}

		return err
	}

	log.Printf("[INFO] Scheduler.Submit: ✓ Request %s enqueued to P%d, waiting for capacity",
		req.ID, req.Priority)

	// Notify via result channel
	if req.ResultChan != nil {
		req.ResultChan <- &ScheduleResult{
			Accepted: true,
			Reason:   "queued",
			QueuePos: s.priorityQueue.queues[req.Priority].Len(),
		}
	}

	return nil
}

// backgroundWorker continuously dequeues and schedules requests
func (s *Scheduler) backgroundWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond) // Check every 100ms
	defer ticker.Stop()

	log.Printf("[INFO] Scheduler.backgroundWorker: Started (policy=%s)", s.policy)

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("[INFO] Scheduler.backgroundWorker: Shutting down")
			return

		case <-ticker.C:
			s.processQueue()
		}
	}
}

// processQueue processes queued requests based on scheduling policy
func (s *Scheduler) processQueue() {
	var req *Request
	var err error

	switch s.policy {
	case PolicyStrictPriority:
		req, err = s.dequeueStrictPriority()
	case PolicyWFQ:
		req, err = s.dequeueWFQ()
	case PolicyHybrid:
		req, err = s.dequeueHybrid()
	default:
		log.Printf("[ERROR] Scheduler.processQueue: Unknown policy %s", s.policy)
		return
	}

	if err != nil || req == nil {
		// No requests to process
		return
	}

	// Check capacity again before scheduling
	canAccept, fatal, reason := s.capacityGuardian.CheckAndReserve(req)
	if !canAccept {
		if fatal {
			s.stats.mu.Lock()
			s.stats.totalRejected++
			totalRejected := s.stats.totalRejected
			s.stats.mu.Unlock()

			log.Printf("[ERROR] Scheduler.processQueue: ✗ Dropping %s - unschedulable: %s (total_rejected=%d)",
				req.ID, reason, totalRejected)
			if req.ResultChan != nil {
				select {
				case req.ResultChan <- &ScheduleResult{
					Accepted: false,
					Reason:   reason,
					QueuePos: -1,
				}:
				default:
				}
			}
			return
		}
		// Put back in queue (re-enqueue)
		log.Printf("[WARN] Scheduler.processQueue: Request %s dequeued but capacity unavailable (%s), re-enqueueing",
			req.ID, reason)
		s.priorityQueue.Enqueue(req)
		return
	}

	s.stats.mu.Lock()
	s.stats.totalScheduled++
	totalScheduled := s.stats.totalScheduled
	s.stats.mu.Unlock()

	waitTime := time.Since(req.EnqueuedAt)
	log.Printf("[INFO] Scheduler.processQueue: ✓ Request %s scheduled (waited=%v, total_scheduled=%d)",
		req.ID, waitTime, totalScheduled)

	// Notify via result channel (if still listening)
	if req.ResultChan != nil {
		select {
		case req.ResultChan <- &ScheduleResult{
			Accepted: true,
			Reason:   "scheduled",
			QueuePos: 0,
		}:
		default:
			// Channel closed or not listening
		}
	}
}

// dequeueStrictPriority dequeues using strict priority (P0 always first)
func (s *Scheduler) dequeueStrictPriority() (*Request, error) {
	return s.priorityQueue.Dequeue()
}

// dequeueWFQ dequeues using Weighted Fair Queuing
func (s *Scheduler) dequeueWFQ() (*Request, error) {
	s.wfqMu.Lock()
	defer s.wfqMu.Unlock()

	// Step 1: Add quantum (proportional to weight)
	for i := 0; i < s.priorityQueue.config.NumPriorityLevels; i++ {
		weight := s.priorityQueue.config.Weights[i]
		s.wfqDeficit[i] += weight
		log.Printf("[DEBUG] Scheduler.dequeueWFQ: P%d deficit += %.1f → %.1f", i, weight, s.wfqDeficit[i])
	}

	// Step 2: Find queue with highest deficit that has requests
	var selectedQueue int = -1
	maxDeficit := 0.0

	for i := 0; i < s.priorityQueue.config.NumPriorityLevels; i++ {
		qLen := s.priorityQueue.queues[i].Len()
		deficit := s.wfqDeficit[i]

		if qLen > 0 && deficit > maxDeficit {
			maxDeficit = deficit
			selectedQueue = i
		}
	}

	if selectedQueue == -1 {
		// No requests
		return nil, fmt.Errorf("no requests in queue")
	}

	// Step 3: Dequeue from selected queue
	req := s.priorityQueue.queues[selectedQueue].Pop()
	if req == nil {
		return nil, fmt.Errorf("no requests in queue")
	}

	// Check expiration
	if time.Now().After(req.Deadline) {
		log.Printf("[WARN] Scheduler.dequeueWFQ: Request %s from P%d expired", req.ID, selectedQueue)
		return nil, fmt.Errorf("request expired")
	}

	// Step 4: Decrease deficit by request cost
	cost := float64(req.EstimatedTokens) / 1000.0
	if cost == 0 {
		cost = 1.0
	}
	s.wfqDeficit[selectedQueue] -= cost

	log.Printf("[INFO] Scheduler.dequeueWFQ: ✓ Selected P%d (deficit=%.1f, cost=%.1f, new_deficit=%.1f)",
		selectedQueue, maxDeficit, cost, s.wfqDeficit[selectedQueue])

	return req, nil
}

// dequeueHybrid dequeues using hybrid policy (P0 strict, P1-P9 WFQ)
func (s *Scheduler) dequeueHybrid() (*Request, error) {
	// Check P0 (critical) first - strict priority
	if !s.priorityQueue.queues[0].IsEmpty() {
		req := s.priorityQueue.queues[0].Pop()
		if req != nil && time.Now().Before(req.Deadline) {
			log.Printf("[INFO] Scheduler.dequeueHybrid: ✓ Selected P0 (critical) request %s", req.ID)
			return req, nil
		}
	}

	// Use WFQ for P1-P9
	s.wfqMu.Lock()
	defer s.wfqMu.Unlock()

	// Add quantum for P1-P9 only
	for i := 1; i < s.priorityQueue.config.NumPriorityLevels; i++ {
		weight := s.priorityQueue.config.Weights[i]
		s.wfqDeficit[i] += weight
	}

	// Find highest deficit queue (P1-P9)
	var selectedQueue int = -1
	maxDeficit := 0.0

	for i := 1; i < s.priorityQueue.config.NumPriorityLevels; i++ {
		qLen := s.priorityQueue.queues[i].Len()
		deficit := s.wfqDeficit[i]

		if qLen > 0 && deficit > maxDeficit {
			maxDeficit = deficit
			selectedQueue = i
		}
	}

	if selectedQueue == -1 {
		return nil, fmt.Errorf("no requests in queue")
	}

	req := s.priorityQueue.queues[selectedQueue].Pop()
	if req == nil {
		return nil, fmt.Errorf("no requests in queue")
	}

	if time.Now().After(req.Deadline) {
		log.Printf("[WARN] Scheduler.dequeueHybrid: Request %s from P%d expired", req.ID, selectedQueue)
		return nil, fmt.Errorf("request expired")
	}

	cost := float64(req.EstimatedTokens) / 1000.0
	if cost == 0 {
		cost = 1.0
	}
	s.wfqDeficit[selectedQueue] -= cost

	log.Printf("[INFO] Scheduler.dequeueHybrid: ✓ Selected P%d via WFQ (deficit=%.1f, cost=%.1f)",
		selectedQueue, maxDeficit, cost)

	return req, nil
}

// Release releases capacity for a completed request
func (s *Scheduler) Release(req *Request) {
	s.capacityGuardian.Release(req)
}

// GetStats returns scheduler statistics
func (s *Scheduler) GetStats() map[string]interface{} {
	s.stats.mu.RLock()
	totalScheduled := s.stats.totalScheduled
	totalRejected := s.stats.totalRejected
	s.stats.mu.RUnlock()

	queueStats := s.priorityQueue.GetStats()
	capacityUtil := s.capacityGuardian.GetUtilization()

	return map[string]interface{}{
		"total_scheduled":   totalScheduled,
		"total_rejected":    totalRejected,
		"queue_stats":       queueStats,
		"capacity_util":     capacityUtil,
		"scheduling_policy": s.policy,
	}
}

// LogStats logs scheduler statistics
func (s *Scheduler) LogStats() {
	stats := s.GetStats()

	log.Printf("[INFO] ===== Scheduler Statistics =====")
	log.Printf("[INFO] Scheduling Policy: %s", stats["scheduling_policy"])
	log.Printf("[INFO] Total Scheduled: %d", stats["total_scheduled"])
	log.Printf("[INFO] Total Rejected: %d", stats["total_rejected"])
	log.Printf("[INFO] ----- Queue Stats -----")
	s.priorityQueue.LogStats()
	log.Printf("[INFO] ----- Capacity Utilization -----")
	s.capacityGuardian.LogStats()
	log.Printf("[INFO] ================================")
}

// Shutdown gracefully shuts down the scheduler
func (s *Scheduler) Shutdown() {
	log.Printf("[INFO] Scheduler.Shutdown: Shutting down...")
	s.cancel()
	s.wg.Wait()
	log.Printf("[INFO] Scheduler.Shutdown: ✓ Shutdown complete")
}
