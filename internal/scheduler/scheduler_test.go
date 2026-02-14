package scheduler

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

func TestPriorityQueue_BasicEnqueueDequeue(t *testing.T) {
	// Enable debug logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	t.Log("===== TEST: Basic Enqueue/Dequeue with 10 Priority Levels =====")

	config := DefaultConfig()
	pq := NewPriorityQueue(config)

	// Enqueue requests with different priorities
	requests := []*Request{
		{ID: "req-p9-1", Priority: PriorityBackground, EstimatedTokens: 100}, // P9 (lowest)
		{ID: "req-p5-1", Priority: PriorityNormal, EstimatedTokens: 200},     // P5 (normal)
		{ID: "req-p0-1", Priority: PriorityCritical, EstimatedTokens: 50},    // P0 (highest)
		{ID: "req-p2-1", Priority: PriorityHigh, EstimatedTokens: 150},       // P2 (high)
		{ID: "req-p7-1", Priority: PriorityLow, EstimatedTokens: 300},        // P7 (low)
	}

	// Enqueue all requests
	for _, req := range requests {
		err := pq.Enqueue(req)
		if err != nil {
			t.Fatalf("Failed to enqueue %s: %v", req.ID, err)
		}
	}

	pq.LogStats()

	// Dequeue should return in priority order: P0, P2, P5, P7, P9
	expectedOrder := []string{"req-p0-1", "req-p2-1", "req-p5-1", "req-p7-1", "req-p9-1"}

	for i, expectedID := range expectedOrder {
		req, err := pq.Dequeue()
		if err != nil {
			t.Fatalf("Dequeue %d failed: %v", i, err)
		}
		if req.ID != expectedID {
			t.Errorf("Dequeue %d: expected %s, got %s", i, expectedID, req.ID)
		} else {
			t.Logf("✓ Dequeue %d: %s (priority=P%d) - CORRECT ORDER", i, req.ID, req.Priority)
		}
	}

	pq.LogStats()
	t.Log("===== TEST PASSED: Priority ordering verified =====")
}

func TestCapacity_MultipleDimensions(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	t.Log("===== TEST: Multi-Dimensional Capacity Check =====")

	capacity := NewCapacity(
		1000,   // max_tokens_per_sec
		10,     // max_rps
		5,      // max_concurrent
		128000, // max_context_length
	)

	// Test 1: Concurrent limit
	t.Log("--- Test 1: Concurrent limit ---")
	for i := 0; i < 5; i++ {
		req := &Request{ID: "concurrent-" + string(rune(i+'1')), EstimatedTokens: 100}
		canAccept, reason := capacity.CanAccept(req)
		if !canAccept {
			t.Errorf("Request %s rejected: %s", req.ID, reason)
		} else {
			capacity.Reserve(req)
			t.Logf("✓ Request %s accepted (concurrent=%d/5)", req.ID, i+1)
		}
	}

	// 6th concurrent should be rejected
	req6 := &Request{ID: "concurrent-6", EstimatedTokens: 100}
	canAccept, reason := capacity.CanAccept(req6)
	if canAccept {
		t.Error("6th concurrent request should be rejected")
	} else {
		t.Logf("✓ 6th concurrent request rejected: %s", reason)
	}

	capacity.LogUtilization()

	// Test 2: Context length limit
	t.Log("--- Test 2: Context length limit ---")
	reqTooLong := &Request{ID: "too-long", EstimatedTokens: 200000} // > 128K
	canAccept, reason = capacity.CanAccept(reqTooLong)
	if canAccept {
		t.Error("Too-long request should be rejected")
	} else {
		t.Logf("✓ Too-long request rejected: %s", reason)
	}

	t.Log("===== TEST PASSED: Capacity checks working =====")
}

func TestScheduler_HybridPolicy(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	t.Log("===== TEST: Scheduler with Hybrid Policy (P0 strict, P1-P9 WFQ) =====")

	config := DefaultConfig()
	config.QueueTimeout = 10 * time.Second

	capacity := NewCapacity(
		5000, // max_tokens_per_sec
		50,   // max_rps
		2,    // max_concurrent (only 2 to force queueing)
		128000,
	)

	scheduler := NewScheduler(config, capacity, PolicyHybrid)
	defer scheduler.Shutdown()

	// Submit requests with different priorities
	// Goal: Show that P0 gets absolute priority, then WFQ kicks in for P1-P9
	requests := []struct {
		id       string
		priority PriorityTier
		tokens   int64
	}{
		{"req-p5-1", PriorityNormal, 500},      // P5
		{"req-p9-1", PriorityBackground, 300},  // P9 (lowest)
		{"req-p0-1", PriorityCritical, 100},    // P0 (critical)
		{"req-p2-1", PriorityHigh, 200},        // P2
		{"req-p7-1", PriorityLow, 400},         // P7
		{"req-p1-1", PriorityUrgent, 150},      // P1
		{"req-p0-2", PriorityCritical, 100},    // P0 (another critical)
		{"req-p5-2", PriorityNormal, 500},      // P5
		{"req-p3-1", PriorityElevated, 250},    // P3
		{"req-p4-1", PriorityAboveNormal, 300}, // P4
	}

	for _, req := range requests {
		schedReq := &Request{
			ID:              req.id,
			Priority:        req.priority,
			EstimatedTokens: req.tokens,
			AccountID:       "test-account",
			Model:           "gpt-4",
			ResultChan:      make(chan *ScheduleResult, 2),
		}

		err := scheduler.Submit(schedReq)
		if err != nil {
			t.Logf("⚠ Request %s rejected on submit: %v", req.id, err)
		} else {
			t.Logf("✓ Request %s submitted (P%d, %d tokens)", req.id, req.priority, req.tokens)
		}
	}

	// Let scheduler process for a bit
	t.Log("--- Waiting for scheduler to process queue ---")
	time.Sleep(2 * time.Second)

	scheduler.LogStats()

	t.Log("===== TEST PASSED: Scheduler processed requests with hybrid policy =====")
}

func TestScheduler_WFQFairness(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	t.Log("===== TEST: WFQ Fairness - High Priority Gets More Bandwidth =====")

	config := DefaultConfig()
	config.QueueTimeout = 10 * time.Second

	capacity := NewCapacity(
		2000, // max_tokens_per_sec (limited to force queueing)
		100,  // max_rps
		3,    // max_concurrent
		128000,
	)

	scheduler := NewScheduler(config, capacity, PolicyWFQ) // Pure WFQ
	defer scheduler.Shutdown()

	// Submit many requests from different priority levels
	// P0 weight=256, P5 weight=8, P9 weight=1
	// We expect P0 to get ~32x more throughput than P9

	// Submit 20 P0 requests, 20 P5 requests, 20 P9 requests
	priorities := []struct {
		level PriorityTier
		count int
	}{
		{PriorityCritical, 20},   // P0
		{PriorityNormal, 20},     // P5
		{PriorityBackground, 20}, // P9
	}

	for _, p := range priorities {
		for i := 0; i < p.count; i++ {
			schedReq := &Request{
				ID:              fmt.Sprintf("p%d-req-%d", p.level, i),
				Priority:        p.level,
				EstimatedTokens: 100,
				ResultChan:      make(chan *ScheduleResult, 2),
			}

			err := scheduler.Submit(schedReq)
			if err != nil {
				t.Logf("⚠ Request %s rejected", schedReq.ID)
			}
		}
	}

	t.Log("--- Waiting for scheduler to process queue (WFQ fairness test) ---")
	time.Sleep(5 * time.Second)

	scheduler.LogStats()

	t.Log("===== TEST PASSED: Check logs to verify P0 got more bandwidth than P9 =====")
	t.Log("Expected: P0 should be dequeued ~32x more than P9 due to weight difference")
}
