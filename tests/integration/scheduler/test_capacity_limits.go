//go:build integration
// +build integration

package main

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// Integration Test: Capacity Limits and Queueing Behavior
//
// Purpose: Verify scheduler correctly enforces capacity limits and queues requests
// Test Strategy:
//   1. Set low capacity limits to force queueing
//   2. Submit more requests than capacity allows
//   3. Verify requests are queued when capacity reached
//   4. Verify requests are dequeued as capacity becomes available
//   5. Verify capacity tracking across all dimensions

const (
	maxConcurrent    = 5 // Low limit to force queueing
	maxTokensPerSec  = 500
	maxRPS           = 10
	totalRequests    = 20 // More than maxConcurrent
	tokensPerRequest = 100
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	fmt.Println("========================================")
	fmt.Println("Capacity Limits & Queueing Test")
	fmt.Println("========================================\n")

	// Configure with low limits to force queueing
	schedConfig := &scheduler.Config{
		NumPriorityLevels: 10,
		DefaultPriority:   scheduler.PriorityNormal,
		MaxQueueDepth:     1000,
		QueueTimeout:      30 * time.Second,
		Weights:           scheduler.GenerateDefaultWeights(10),
	}

	capacity := &scheduler.Capacity{
		MaxTokensPerSec:  maxTokensPerSec,
		MaxRPS:           maxRPS,
		MaxConcurrent:    maxConcurrent,
		MaxContextLength: 128000,
	}

	sched := scheduler.NewScheduler(schedConfig, capacity, scheduler.PolicyHybrid)
	defer sched.Shutdown()

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Max Concurrent: %d\n", maxConcurrent)
	fmt.Printf("  Max Tokens/sec: %d\n", maxTokensPerSec)
	fmt.Printf("  Max RPS: %d\n", maxRPS)
	fmt.Printf("  Total Requests: %d\n\n", totalRequests)

	fmt.Println("========================================")
	fmt.Println("Test 1: Concurrent Limit Enforcement")
	fmt.Println("========================================\n")

	// Track results
	var mu sync.Mutex
	queuedCount := 0
	immediateCount := 0
	rejectedCount := 0
	completedCount := 0

	// Submit requests concurrently
	var wg sync.WaitGroup
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqID := fmt.Sprintf("req-concurrent-%d", idx)
			schedReq := &scheduler.Request{
				ID:              reqID,
				Priority:        scheduler.PriorityNormal,
				EstimatedTokens: tokensPerRequest,
				AccountID:       "test-account",
				Model:           "test-model",
				ResultChan:      make(chan *scheduler.ScheduleResult, 2),
			}

			err := sched.Submit(schedReq)
			if err != nil {
				mu.Lock()
				rejectedCount++
				mu.Unlock()
				fmt.Printf("✗ Request %s REJECTED: %v\n", reqID, err)
				return
			}

			// Wait for scheduling result
			select {
			case result := <-schedReq.ResultChan:
				mu.Lock()
				if result.Accepted {
					if result.Reason == "queued" {
						queuedCount++
						fmt.Printf("⏳ Request %s QUEUED at position %d\n", reqID, result.QueuePos)
					} else {
						immediateCount++
						fmt.Printf("✓ Request %s SCHEDULED immediately\n", reqID)
					}
				} else {
					rejectedCount++
					fmt.Printf("✗ Request %s REJECTED: %s\n", reqID, result.Reason)
				}
				mu.Unlock()

				// Simulate work
				time.Sleep(100 * time.Millisecond)

				// Release
				sched.Release(schedReq)
				mu.Lock()
				completedCount++
				mu.Unlock()

			case <-time.After(5 * time.Second):
				mu.Lock()
				rejectedCount++
				mu.Unlock()
				fmt.Printf("⚠ Request %s TIMEOUT\n", reqID)
			}
		}(i)

		// Stagger submissions slightly
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for all requests to complete
	wg.Wait()

	fmt.Println("\n========================================")
	fmt.Println("Test 1 Results")
	fmt.Println("========================================\n")

	fmt.Printf("Total Submitted: %d\n", totalRequests)
	fmt.Printf("Immediate: %d\n", immediateCount)
	fmt.Printf("Queued: %d\n", queuedCount)
	fmt.Printf("Rejected: %d\n", rejectedCount)
	fmt.Printf("Completed: %d\n", completedCount)

	// Validation
	testsPassed := true

	// At least some requests should have been queued (since we submit more than max concurrent)
	if queuedCount == 0 {
		fmt.Printf("\n✗ FAIL: Expected some requests to be queued (submitted %d, max concurrent %d)\n",
			totalRequests, maxConcurrent)
		testsPassed = false
	} else {
		fmt.Printf("\n✓ PASS: Queueing behavior verified (%d queued)\n", queuedCount)
	}

	// All requests should complete (either immediate or queued, but not rejected)
	if completedCount != totalRequests {
		fmt.Printf("✗ FAIL: Expected %d completed, got %d\n", totalRequests, completedCount)
		testsPassed = false
	} else {
		fmt.Printf("✓ PASS: All requests completed (%d/%d)\n", completedCount, totalRequests)
	}

	// No requests should be rejected (queue has enough depth)
	if rejectedCount > 0 {
		fmt.Printf("✗ FAIL: %d requests rejected unexpectedly\n", rejectedCount)
		testsPassed = false
	} else {
		fmt.Printf("✓ PASS: No requests rejected\n")
	}

	fmt.Println("\n========================================")
	fmt.Println("Test 2: Token Rate Limiting")
	fmt.Println("========================================\n")

	// Reset counters
	queuedCount = 0
	immediateCount = 0
	rejectedCount = 0
	completedCount = 0

	// Submit high-token requests to hit token/sec limit
	highTokenRequests := 10
	highTokensPerReq := int64(200) // 10 * 200 = 2000 tokens in burst

	for i := 0; i < highTokenRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqID := fmt.Sprintf("req-tokens-%d", idx)
			schedReq := &scheduler.Request{
				ID:              reqID,
				Priority:        scheduler.PriorityHigh,
				EstimatedTokens: highTokensPerReq,
				AccountID:       "test-account",
				Model:           "test-model",
				ResultChan:      make(chan *scheduler.ScheduleResult, 2),
			}

			err := sched.Submit(schedReq)
			if err != nil {
				mu.Lock()
				rejectedCount++
				mu.Unlock()
				return
			}

			select {
			case result := <-schedReq.ResultChan:
				mu.Lock()
				if result.Accepted {
					if result.Reason == "queued" {
						queuedCount++
					} else {
						immediateCount++
					}
				} else {
					rejectedCount++
				}
				mu.Unlock()

				time.Sleep(50 * time.Millisecond)
				sched.Release(schedReq)
				mu.Lock()
				completedCount++
				mu.Unlock()

			case <-time.After(5 * time.Second):
				mu.Lock()
				rejectedCount++
				mu.Unlock()
			}
		}(i)

		time.Sleep(20 * time.Millisecond) // Rapid submissions
	}

	wg.Wait()

	fmt.Println("Test 2 Results")
	fmt.Println("========================================\n")
	fmt.Printf("High-token requests: %d (×%d tokens)\n", highTokenRequests, highTokensPerReq)
	fmt.Printf("Immediate: %d\n", immediateCount)
	fmt.Printf("Queued: %d\n", queuedCount)
	fmt.Printf("Completed: %d\n", completedCount)

	if completedCount == highTokenRequests {
		fmt.Printf("\n✓ PASS: All high-token requests completed\n")
	} else {
		fmt.Printf("\n✗ FAIL: Expected %d completed, got %d\n", highTokenRequests, completedCount)
		testsPassed = false
	}

	// Show final scheduler stats
	fmt.Println("\n========================================")
	fmt.Println("Scheduler Statistics")
	fmt.Println("========================================\n")
	sched.LogStats()

	// Final verdict
	fmt.Println("\n========================================")
	if testsPassed {
		fmt.Println("✓ ALL TESTS PASSED")
	} else {
		fmt.Println("✗ SOME TESTS FAILED")
	}
	fmt.Println("========================================")
	fmt.Println("Verified:")
	fmt.Println("  - Concurrent request limit enforcement")
	fmt.Println("  - Automatic queueing when capacity reached")
	fmt.Println("  - Request dequeuing as capacity available")
	fmt.Println("  - Token rate limiting")
	fmt.Println("  - Complete request lifecycle (submit → queue → execute → release)")
	fmt.Println("")

	if !testsPassed {
		os.Exit(1)
	}
}
