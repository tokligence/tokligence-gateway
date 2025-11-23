package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// CapacityRequest is sent to the capacity manager goroutine
type CapacityRequest struct {
	Req        *Request
	ResultChan chan *CapacityResult
}

// CapacityResult is the response from capacity manager
type CapacityResult struct {
	Accepted bool
	Reason   string
}

// CapacityRelease notifies capacity manager to release resources
type CapacityRelease struct {
	Req *Request
}

// ChannelScheduler is a lock-free, channel-based scheduler
type ChannelScheduler struct {
	// One channel per priority level (P0-P9)
	priorityChannels []chan *Request

	// Capacity management channels
	capacityCheckChan   chan *CapacityRequest
	capacityReleaseChan chan *CapacityRelease

	// Configuration
	config   *Config
	capacity *Capacity
	policy   SchedulingPolicy

	// WFQ state (only accessed by scheduler goroutine)
	wfqDeficit map[int]float64

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Statistics (atomic counters - no locks!)
	totalScheduled atomic.Uint64
	totalRejected  atomic.Uint64
	totalQueued    atomic.Uint64
}

// NewChannelScheduler creates a new channel-based scheduler
func NewChannelScheduler(config *Config, capacity *Capacity, policy SchedulingPolicy) *ChannelScheduler {
	if policy == "" {
		policy = PolicyHybrid
	}

	log.Printf("[INFO] ChannelScheduler: Initializing with policy=%s, priority_levels=%d",
		policy, config.NumPriorityLevels)

	ctx, cancel := context.WithCancel(context.Background())

	// Calculate buffer sizes for internal channels based on queue depth
	// Use larger buffers to handle burst traffic without blocking
	internalBufferSize := config.MaxQueueDepth
	if internalBufferSize == 0 {
		internalBufferSize = 5000 // Higher default for production
	}
	// Internal channels should be at least 10% of queue depth, minimum 500
	if internalBufferSize < 500 {
		internalBufferSize = 500
	}

	cs := &ChannelScheduler{
		priorityChannels:    make([]chan *Request, config.NumPriorityLevels),
		capacityCheckChan:   make(chan *CapacityRequest, internalBufferSize),   // Large buffer for burst
		capacityReleaseChan: make(chan *CapacityRelease, internalBufferSize),   // Large buffer for burst
		config:              config,
		capacity:            capacity,
		policy:              policy,
		wfqDeficit:          make(map[int]float64),
		ctx:                 ctx,
		cancel:              cancel,
	}

	log.Printf("[INFO] ChannelScheduler: Internal channel buffers set to %d", internalBufferSize)

	// Create one channel per priority level
	for i := 0; i < config.NumPriorityLevels; i++ {
		// Buffer size based on max queue depth
		bufferSize := config.MaxQueueDepth
		if bufferSize == 0 {
			bufferSize = 5000 // Higher default for production (was 1000)
		}
		cs.priorityChannels[i] = make(chan *Request, bufferSize)
		cs.wfqDeficit[i] = 0.0

		log.Printf("[INFO] ChannelScheduler: Created P%d channel (buffer=%d)", i, bufferSize)
	}

	// Start background goroutines
	go cs.capacityManagerLoop()
	go cs.schedulerLoop()
	go cs.statsMonitorLoop() // Periodic stats logging

	log.Printf("[INFO] ChannelScheduler: ✓ Initialized (lock-free, channel-based)")

	return cs
}

// Submit submits a request (non-blocking)
func (cs *ChannelScheduler) Submit(req *Request) error {
	log.Printf("[INFO] ChannelScheduler.Submit: Request %s (priority=P%d, tokens=%d)",
		req.ID, req.Priority, req.EstimatedTokens)

	// Validate priority
	if int(req.Priority) >= cs.config.NumPriorityLevels {
		cs.totalRejected.Add(1)
		return fmt.Errorf("invalid priority level P%d (max: P%d)",
			req.Priority, cs.config.NumPriorityLevels-1)
	}

	// Check capacity first (fast path)
	resultChan := make(chan *CapacityResult, 1)
	capReq := &CapacityRequest{
		Req:        req,
		ResultChan: resultChan,
	}

	select {
	case cs.capacityCheckChan <- capReq:
		// Request sent to capacity manager
	case <-cs.ctx.Done():
		return fmt.Errorf("scheduler shutting down")
	case <-time.After(5 * time.Second):
		cs.totalRejected.Add(1)
		return fmt.Errorf("capacity check timeout")
	}

	// Wait for capacity decision
	select {
	case result := <-resultChan:
		if result.Accepted {
			// Capacity available - accept immediately
			cs.totalScheduled.Add(1)
			log.Printf("[INFO] ChannelScheduler.Submit: ✓ Request %s accepted immediately (capacity available)",
				req.ID)

			if req.ResultChan != nil {
				req.ResultChan <- &ScheduleResult{
					Accepted: true,
					Reason:   "capacity available",
					QueuePos: 0,
				}
			}
			return nil
		}

		// No capacity - enqueue to priority channel
		log.Printf("[INFO] ChannelScheduler.Submit: No capacity, enqueueing %s to P%d (reason: %s)",
			req.ID, req.Priority, result.Reason)

		select {
		case cs.priorityChannels[req.Priority] <- req:
			cs.totalQueued.Add(1)
			log.Printf("[INFO] ChannelScheduler.Submit: ✓ Request %s enqueued to P%d",
				req.ID, req.Priority)

			if req.ResultChan != nil {
				req.ResultChan <- &ScheduleResult{
					Accepted: true,
					Reason:   "queued",
					QueuePos: len(cs.priorityChannels[req.Priority]),
				}
			}
			return nil

		default:
			// Queue full (channel buffer full)
			cs.totalRejected.Add(1)
			log.Printf("[ERROR] ChannelScheduler.Submit: ✗ Request %s rejected - P%d queue full",
				req.ID, req.Priority)

			if req.ResultChan != nil {
				req.ResultChan <- &ScheduleResult{
					Accepted: false,
					Reason:   "queue full",
					QueuePos: -1,
				}
			}
			return fmt.Errorf("queue full for priority P%d", req.Priority)
		}

	case <-cs.ctx.Done():
		return fmt.Errorf("scheduler shutting down")
	case <-time.After(5 * time.Second):
		cs.totalRejected.Add(1)
		return fmt.Errorf("capacity decision timeout")
	}
}

// capacityManagerLoop runs in a single goroutine - NO LOCKS NEEDED
func (cs *ChannelScheduler) capacityManagerLoop() {
	log.Printf("[INFO] ChannelScheduler.capacityManager: Started")

	// Local state (no locks needed - single goroutine)
	currentConcurrent := 0
	windowStart := time.Now()
	windowTokens := int64(0)
	windowRequests := int64(0)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cs.ctx.Done():
			log.Printf("[INFO] ChannelScheduler.capacityManager: Shutting down")
			return

		case <-ticker.C:
			// Reset rate window every second
			if time.Since(windowStart) > time.Second {
				log.Printf("[DEBUG] CapacityManager: Resetting window (tokens=%d, requests=%d, concurrent=%d)",
					windowTokens, windowRequests, currentConcurrent)
				windowStart = time.Now()
				windowTokens = 0
				windowRequests = 0
			}

		case capReq := <-cs.capacityCheckChan:
			req := capReq.Req

			// Reset window if needed
			if time.Since(windowStart) > time.Second {
				windowStart = time.Now()
				windowTokens = 0
				windowRequests = 0
			}

			// Check 1: Concurrent limit
			if cs.capacity.MaxConcurrent > 0 && currentConcurrent >= cs.capacity.MaxConcurrent {
				capReq.ResultChan <- &CapacityResult{
					Accepted: false,
					Reason:   fmt.Sprintf("concurrent limit (%d/%d)", currentConcurrent, cs.capacity.MaxConcurrent),
				}
				continue
			}

			// Check 2: Context length
			if cs.capacity.MaxContextLength > 0 && int(req.EstimatedTokens) > cs.capacity.MaxContextLength {
				capReq.ResultChan <- &CapacityResult{
					Accepted: false,
					Reason:   fmt.Sprintf("context too long (%d > %d)", req.EstimatedTokens, cs.capacity.MaxContextLength),
				}
				continue
			}

			// Check 3: Tokens/sec limit (PRIMARY)
			if cs.capacity.MaxTokensPerSec > 0 {
				projectedTokens := windowTokens + req.EstimatedTokens
				if projectedTokens > int64(cs.capacity.MaxTokensPerSec) {
					capReq.ResultChan <- &CapacityResult{
						Accepted: false,
						Reason:   fmt.Sprintf("tokens/sec limit (%d/%d)", windowTokens, cs.capacity.MaxTokensPerSec),
					}
					continue
				}
			}

			// Check 4: RPS limit (SECONDARY)
			if cs.capacity.MaxRPS > 0 {
				projectedRPS := windowRequests + 1
				if projectedRPS > int64(cs.capacity.MaxRPS) {
					capReq.ResultChan <- &CapacityResult{
						Accepted: false,
						Reason:   fmt.Sprintf("RPS limit (%d/%d)", windowRequests, cs.capacity.MaxRPS),
					}
					continue
				}
			}

			// All checks passed - reserve capacity
			currentConcurrent++
			windowTokens += req.EstimatedTokens
			windowRequests++

			log.Printf("[DEBUG] CapacityManager: ✓ Accepted %s (concurrent=%d/%d, tokens=%d/%d, rps=%d/%d)",
				req.ID, currentConcurrent, cs.capacity.MaxConcurrent,
				windowTokens, cs.capacity.MaxTokensPerSec,
				windowRequests, cs.capacity.MaxRPS)

			capReq.ResultChan <- &CapacityResult{
				Accepted: true,
				Reason:   "capacity available",
			}

		case rel := <-cs.capacityReleaseChan:
			// Release capacity
			if currentConcurrent > 0 {
				currentConcurrent--
			}
			log.Printf("[DEBUG] CapacityManager: Released %s (concurrent=%d/%d)",
				rel.Req.ID, currentConcurrent, cs.capacity.MaxConcurrent)
		}
	}
}

// schedulerLoop dequeues requests based on policy
func (cs *ChannelScheduler) schedulerLoop() {
	log.Printf("[INFO] ChannelScheduler.schedulerLoop: Started (policy=%s)", cs.policy)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cs.ctx.Done():
			log.Printf("[INFO] ChannelScheduler.schedulerLoop: Shutting down")
			return

		case <-ticker.C:
			cs.processQueues()
		}
	}
}

// processQueues processes queued requests using select + policy
func (cs *ChannelScheduler) processQueues() {
	var req *Request

	switch cs.policy {
	case PolicyStrictPriority:
		req = cs.dequeueStrictPriority()
	case PolicyWFQ:
		req = cs.dequeueWFQ()
	case PolicyHybrid:
		req = cs.dequeueHybrid()
	default:
		return
	}

	if req == nil {
		return
	}

	// Check if expired
	if time.Now().After(req.Deadline) {
		log.Printf("[WARN] ChannelScheduler: Request %s expired, dropping", req.ID)
		return
	}

	// Check capacity again before scheduling
	resultChan := make(chan *CapacityResult, 1)
	capReq := &CapacityRequest{
		Req:        req,
		ResultChan: resultChan,
	}

	select {
	case cs.capacityCheckChan <- capReq:
		// Sent to capacity manager
	case <-cs.ctx.Done():
		return
	case <-time.After(time.Second):
		log.Printf("[WARN] ChannelScheduler: Capacity check timeout for %s", req.ID)
		return
	}

	// Wait for capacity decision
	select {
	case result := <-resultChan:
		if !result.Accepted {
			// Put back in queue (re-enqueue)
			log.Printf("[WARN] ChannelScheduler: Request %s dequeued but capacity unavailable, re-enqueueing",
				req.ID)
			select {
			case cs.priorityChannels[req.Priority] <- req:
				// Re-enqueued
			default:
				// Queue full, drop request
				log.Printf("[ERROR] ChannelScheduler: Failed to re-enqueue %s, dropping", req.ID)
			}
			return
		}

		// Capacity reserved - schedule request
		cs.totalScheduled.Add(1)
		waitTime := time.Since(req.EnqueuedAt)
		log.Printf("[INFO] ChannelScheduler: ✓ Request %s scheduled (waited=%v)", req.ID, waitTime)

		if req.ResultChan != nil {
			select {
			case req.ResultChan <- &ScheduleResult{
				Accepted: true,
				Reason:   "scheduled",
				QueuePos: 0,
			}:
			default:
				// Channel closed
			}
		}

	case <-cs.ctx.Done():
		return
	case <-time.After(time.Second):
		log.Printf("[WARN] ChannelScheduler: Capacity decision timeout for %s", req.ID)
		return
	}
}

// dequeueStrictPriority uses select with priority (P0 first)
func (cs *ChannelScheduler) dequeueStrictPriority() *Request {
	// Try P0-P9 in strict order
	for i := 0; i < cs.config.NumPriorityLevels; i++ {
		select {
		case req := <-cs.priorityChannels[i]:
			log.Printf("[DEBUG] ChannelScheduler: Dequeued %s from P%d (strict)", req.ID, i)
			return req
		default:
			// Queue empty, try next priority
		}
	}
	return nil
}

// dequeueWFQ uses weighted selection across all priority levels
func (cs *ChannelScheduler) dequeueWFQ() *Request {
	// Add quantum to all queues
	for i := 0; i < cs.config.NumPriorityLevels; i++ {
		cs.wfqDeficit[i] += cs.config.Weights[i]
	}

	// Find queue with highest deficit that has requests
	var selectedQueue int = -1
	maxDeficit := 0.0

	for i := 0; i < cs.config.NumPriorityLevels; i++ {
		qLen := len(cs.priorityChannels[i])
		if qLen > 0 && cs.wfqDeficit[i] > maxDeficit {
			maxDeficit = cs.wfqDeficit[i]
			selectedQueue = i
		}
	}

	if selectedQueue == -1 {
		return nil
	}

	// Dequeue from selected queue
	select {
	case req := <-cs.priorityChannels[selectedQueue]:
		cost := float64(req.EstimatedTokens) / 1000.0
		if cost == 0 {
			cost = 1.0
		}
		cs.wfqDeficit[selectedQueue] -= cost

		log.Printf("[DEBUG] ChannelScheduler: Dequeued %s from P%d (WFQ, deficit=%.1f, cost=%.1f)",
			req.ID, selectedQueue, maxDeficit, cost)
		return req
	default:
		return nil
	}
}

// dequeueHybrid uses P0 strict, P1-P9 WFQ
func (cs *ChannelScheduler) dequeueHybrid() *Request {
	// Check P0 first (strict priority)
	select {
	case req := <-cs.priorityChannels[0]:
		log.Printf("[DEBUG] ChannelScheduler: Dequeued %s from P0 (hybrid-strict)", req.ID)
		return req
	default:
		// P0 empty, use WFQ for P1-P9
	}

	// WFQ for P1-P9
	for i := 1; i < cs.config.NumPriorityLevels; i++ {
		cs.wfqDeficit[i] += cs.config.Weights[i]
	}

	var selectedQueue int = -1
	maxDeficit := 0.0

	for i := 1; i < cs.config.NumPriorityLevels; i++ {
		qLen := len(cs.priorityChannels[i])
		if qLen > 0 && cs.wfqDeficit[i] > maxDeficit {
			maxDeficit = cs.wfqDeficit[i]
			selectedQueue = i
		}
	}

	if selectedQueue == -1 {
		return nil
	}

	select {
	case req := <-cs.priorityChannels[selectedQueue]:
		cost := float64(req.EstimatedTokens) / 1000.0
		if cost == 0 {
			cost = 1.0
		}
		cs.wfqDeficit[selectedQueue] -= cost

		log.Printf("[DEBUG] ChannelScheduler: Dequeued %s from P%d (hybrid-WFQ, deficit=%.1f)",
			req.ID, selectedQueue, maxDeficit)
		return req
	default:
		return nil
	}
}

// Release releases capacity after request completes
func (cs *ChannelScheduler) Release(req *Request) {
	select {
	case cs.capacityReleaseChan <- &CapacityRelease{Req: req}:
		log.Printf("[DEBUG] ChannelScheduler.Release: Sent release for %s", req.ID)
	case <-cs.ctx.Done():
		return
	case <-time.After(time.Second):
		log.Printf("[WARN] ChannelScheduler.Release: Timeout releasing %s", req.ID)
	}
}

// QueueStats represents detailed statistics for a single priority queue
type QueueStats struct {
	Priority         int     `json:"priority"`
	CurrentDepth     int     `json:"current_depth"`      // Current number of queued requests
	MaxDepth         int     `json:"max_depth"`          // Maximum capacity
	Utilization      float64 `json:"utilization"`        // 0.0-1.0 (current/max)
	UtilizationPct   float64 `json:"utilization_pct"`    // 0-100%
	IsFull           bool    `json:"is_full"`            // Is queue at capacity?
	AvailableSlots   int     `json:"available_slots"`    // Remaining capacity
}

// ChannelStats represents internal channel statistics
type ChannelStats struct {
	CapacityCheckQueue   int     `json:"capacity_check_queue"`    // Pending capacity checks
	CapacityReleaseQueue int     `json:"capacity_release_queue"`  // Pending releases
	InternalBufferSize   int     `json:"internal_buffer_size"`    // Buffer capacity
	CheckUtilization     float64 `json:"check_utilization_pct"`   // % of check buffer used
	ReleaseUtilization   float64 `json:"release_utilization_pct"` // % of release buffer used
}

// GetStats returns statistics (atomic reads - no locks!)
func (cs *ChannelScheduler) GetStats() map[string]interface{} {
	queueLengths := make(map[string]int)
	for i := 0; i < cs.config.NumPriorityLevels; i++ {
		queueLengths[fmt.Sprintf("P%d", i)] = len(cs.priorityChannels[i])
	}

	return map[string]interface{}{
		"total_scheduled":    cs.totalScheduled.Load(),
		"total_rejected":     cs.totalRejected.Load(),
		"total_queued":       cs.totalQueued.Load(),
		"queue_lengths":      queueLengths,
		"scheduling_policy":  cs.policy,
	}
}

// GetDetailedStats returns comprehensive queue occupancy statistics
func (cs *ChannelScheduler) GetDetailedStats() map[string]interface{} {
	// Per-priority queue stats
	queueStats := make([]QueueStats, cs.config.NumPriorityLevels)
	totalQueued := 0
	totalCapacity := 0

	for i := 0; i < cs.config.NumPriorityLevels; i++ {
		currentDepth := len(cs.priorityChannels[i])
		maxDepth := cap(cs.priorityChannels[i])
		utilization := 0.0
		if maxDepth > 0 {
			utilization = float64(currentDepth) / float64(maxDepth)
		}

		queueStats[i] = QueueStats{
			Priority:       i,
			CurrentDepth:   currentDepth,
			MaxDepth:       maxDepth,
			Utilization:    utilization,
			UtilizationPct: utilization * 100,
			IsFull:         currentDepth >= maxDepth,
			AvailableSlots: maxDepth - currentDepth,
		}

		totalQueued += currentDepth
		totalCapacity += maxDepth
	}

	// Internal channel stats
	checkQueueDepth := len(cs.capacityCheckChan)
	releaseQueueDepth := len(cs.capacityReleaseChan)
	internalBufferSize := cap(cs.capacityCheckChan)

	channelStats := ChannelStats{
		CapacityCheckQueue:   checkQueueDepth,
		CapacityReleaseQueue: releaseQueueDepth,
		InternalBufferSize:   internalBufferSize,
		CheckUtilization:     float64(checkQueueDepth) / float64(internalBufferSize) * 100,
		ReleaseUtilization:   float64(releaseQueueDepth) / float64(internalBufferSize) * 100,
	}

	// Overall stats
	overallUtilization := 0.0
	if totalCapacity > 0 {
		overallUtilization = float64(totalQueued) / float64(totalCapacity)
	}

	return map[string]interface{}{
		// Counters
		"total_scheduled": cs.totalScheduled.Load(),
		"total_rejected":  cs.totalRejected.Load(),
		"total_queued":    cs.totalQueued.Load(),

		// Queue occupancy
		"queue_stats":          queueStats,
		"total_queued_now":     totalQueued,
		"total_capacity":       totalCapacity,
		"overall_utilization":  overallUtilization * 100,

		// Internal channels
		"channel_stats": channelStats,

		// Config
		"scheduling_policy":    cs.policy,
		"num_priority_levels":  cs.config.NumPriorityLevels,
	}
}

// LogDetailedStats logs comprehensive statistics for debugging
func (cs *ChannelScheduler) LogDetailedStats() {
	stats := cs.GetDetailedStats()

	log.Printf("[INFO] ===== Channel Scheduler Statistics =====")
	log.Printf("[INFO] Policy: %s", stats["scheduling_policy"])
	log.Printf("[INFO] Total Scheduled: %d", stats["total_scheduled"])
	log.Printf("[INFO] Total Rejected: %d", stats["total_rejected"])
	log.Printf("[INFO] Total Queued (lifetime): %d", stats["total_queued"])
	log.Printf("[INFO] Overall Queue Utilization: %.1f%%", stats["overall_utilization"])
	log.Printf("[INFO]")
	log.Printf("[INFO] ----- Priority Queue Occupancy -----")

	queueStats := stats["queue_stats"].([]QueueStats)
	for _, qs := range queueStats {
		busyIndicator := "  "
		if qs.UtilizationPct > 80 {
			busyIndicator = "🔥" // Hot queue
		} else if qs.UtilizationPct > 50 {
			busyIndicator = "⚠️ " // Warning
		} else if qs.CurrentDepth > 0 {
			busyIndicator = "✓ " // Active
		}

		log.Printf("[INFO] %s P%d: %d/%d (%.1f%%) - %d slots available",
			busyIndicator, qs.Priority, qs.CurrentDepth, qs.MaxDepth,
			qs.UtilizationPct, qs.AvailableSlots)
	}

	log.Printf("[INFO]")
	log.Printf("[INFO] ----- Internal Channel Stats -----")
	channelStats := stats["channel_stats"].(ChannelStats)
	log.Printf("[INFO] Capacity Check Queue: %d/%d (%.1f%%)",
		channelStats.CapacityCheckQueue, channelStats.InternalBufferSize,
		channelStats.CheckUtilization)
	log.Printf("[INFO] Capacity Release Queue: %d/%d (%.1f%%)",
		channelStats.CapacityReleaseQueue, channelStats.InternalBufferSize,
		channelStats.ReleaseUtilization)
	log.Printf("[INFO] ======================================")
}

// GetBusiestQueues returns the top N busiest priority queues
func (cs *ChannelScheduler) GetBusiestQueues(topN int) []QueueStats {
	stats := cs.GetDetailedStats()
	queueStats := stats["queue_stats"].([]QueueStats)

	// Sort by utilization (descending)
	sorted := make([]QueueStats, len(queueStats))
	copy(sorted, queueStats)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Utilization > sorted[i].Utilization {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if topN > len(sorted) {
		topN = len(sorted)
	}

	return sorted[:topN]
}

// statsMonitorLoop periodically logs queue occupancy statistics
func (cs *ChannelScheduler) statsMonitorLoop() {
	// Check if stats logging is disabled
	if cs.config.StatsIntervalSec <= 0 {
		log.Printf("[INFO] ChannelScheduler.statsMonitor: Disabled (scheduler_stats_interval_sec=%d)", cs.config.StatsIntervalSec)
		return
	}

	interval := time.Duration(cs.config.StatsIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[INFO] ChannelScheduler.statsMonitor: Started (interval=%v)", interval)

	for {
		select {
		case <-cs.ctx.Done():
			log.Printf("[INFO] ChannelScheduler.statsMonitor: Shutting down")
			return

		case <-ticker.C:
			// Only log if there's activity
			stats := cs.GetDetailedStats()
			totalQueued := stats["total_queued_now"].(int)
			totalScheduled := stats["total_scheduled"].(uint64)

			if totalQueued > 0 || totalScheduled > 0 {
				cs.LogDetailedStats()

				// Log busiest queues
				busiest := cs.GetBusiestQueues(3)
				if len(busiest) > 0 && busiest[0].CurrentDepth > 0 {
					log.Printf("[INFO] Top 3 Busiest Queues:")
					for i, qs := range busiest {
						if qs.CurrentDepth == 0 {
							break
						}
						log.Printf("[INFO]   %d. P%d: %d requests (%.1f%% full)",
							i+1, qs.Priority, qs.CurrentDepth, qs.UtilizationPct)
					}
				}
			}
		}
	}
}

// Shutdown gracefully shuts down the scheduler
func (cs *ChannelScheduler) Shutdown() {
	log.Printf("[INFO] ChannelScheduler.Shutdown: Shutting down...")
	cs.cancel()
	time.Sleep(200 * time.Millisecond) // Allow goroutines to finish
	log.Printf("[INFO] ChannelScheduler.Shutdown: ✓ Complete")
}
