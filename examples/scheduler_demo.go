package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/config"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// This demo shows how to use the priority scheduler with LocalProvider

func main() {
	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	fmt.Println("========================================")
	fmt.Println("Tokligence Gateway Scheduler Demo")
	fmt.Println("========================================")
	fmt.Println()

	// Load configuration
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if !cfg.SchedulerEnabled {
		fmt.Println("⚠️  Scheduler is DISABLED in config")
		fmt.Println("To enable, set scheduler_enabled=true in config/scheduler.ini")
		fmt.Println("or export TOKLIGENCE_SCHEDULER_ENABLED=true")
		fmt.Println()
		return
	}

	fmt.Printf("✓ Scheduler ENABLED\n")
	fmt.Printf("  Priority Levels: %d\n", cfg.SchedulerPriorityLevels)
	fmt.Printf("  Default Priority: P%d\n", cfg.SchedulerDefaultPriority)
	fmt.Printf("  Policy: %s\n", cfg.SchedulerPolicy)
	fmt.Printf("  Max Concurrent: %d\n", cfg.SchedulerMaxConcurrent)
	fmt.Printf("  Max Tokens/sec: %d\n", cfg.SchedulerMaxTokensPerSec)
	fmt.Printf("  Max RPS: %d\n", cfg.SchedulerMaxRPS)
	fmt.Printf("  Queue Timeout: %ds\n\n", cfg.SchedulerQueueTimeoutSec)

	// Build scheduler config
	schedConfig, err := scheduler.ConfigFromGatewayConfig(
		cfg.SchedulerEnabled,
		cfg.SchedulerPriorityLevels,
		cfg.SchedulerDefaultPriority,
		cfg.SchedulerMaxQueueDepth,
		cfg.SchedulerQueueTimeoutSec,
		cfg.SchedulerWeights,
	)
	if err != nil {
		log.Fatalf("Failed to build scheduler config: %v", err)
	}

	// Build capacity
	capacity := scheduler.CapacityFromGatewayConfig(
		cfg.SchedulerMaxTokensPerSec,
		cfg.SchedulerMaxRPS,
		cfg.SchedulerMaxConcurrent,
		cfg.SchedulerMaxContextLength,
	)

	// Create scheduler
	policy := scheduler.PolicyFromString(cfg.SchedulerPolicy)
	sched := scheduler.NewScheduler(schedConfig, capacity, policy)
	defer sched.Shutdown()

	fmt.Println("========================================")
	fmt.Println("Submitting Test Requests")
	fmt.Println("========================================")
	fmt.Println()

	// Submit test requests with different priorities
	requests := []struct {
		id       string
		priority scheduler.PriorityTier
		tokens   int64
	}{
		{"req-normal-1", scheduler.PriorityNormal, 500},       // P5
		{"req-background-1", scheduler.PriorityBackground, 300}, // P9
		{"req-critical-1", scheduler.PriorityCritical, 100},   // P0
		{"req-high-1", scheduler.PriorityHigh, 200},           // P2
		{"req-low-1", scheduler.PriorityLow, 400},             // P7
	}

	for _, req := range requests {
		schedReq := &scheduler.Request{
			ID:              req.id,
			Priority:        req.priority,
			EstimatedTokens: req.tokens,
			AccountID:       "demo-account",
			Model:           "gpt-4",
			ResultChan:      make(chan *scheduler.ScheduleResult, 2),
		}

		err := sched.Submit(schedReq)
		if err != nil {
			fmt.Printf("✗ Request %s (P%d) REJECTED: %v\n", req.id, req.priority, err)
		} else {
			// Wait for scheduling result
			select {
			case result := <-schedReq.ResultChan:
				if result.Accepted {
					if result.Reason == "queued" {
						fmt.Printf("✓ Request %s (P%d) QUEUED at position %d\n", req.id, req.priority, result.QueuePos)
					} else {
						fmt.Printf("✓ Request %s (P%d) SCHEDULED immediately\n", req.id, req.priority)
					}
				} else {
					fmt.Printf("✗ Request %s (P%d) REJECTED: %s\n", req.id, req.priority, result.Reason)
				}
			case <-time.After(1 * time.Second):
				fmt.Printf("⚠ Request %s (P%d) TIMEOUT waiting for scheduler\n", req.id, req.priority)
			}
		}

		// Simulate request completion after a short delay
		go func(r *scheduler.Request) {
			time.Sleep(100 * time.Millisecond)
			sched.Release(r)
			fmt.Printf("  → Request %s completed and released\n", r.ID)
		}(schedReq)
	}

	// Wait for requests to process
	time.Sleep(2 * time.Second)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("Scheduler Statistics")
	fmt.Println("========================================")
	fmt.Println()

	sched.LogStats()

	fmt.Println("\n========================================")
	fmt.Println("Demo Complete")
	fmt.Println("========================================")
}
