package scheduler

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestChannelScheduler_Submit_ImmediateAccept(t *testing.T) {
	config := &Config{
		NumPriorityLevels: 10,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     100,
		QueueTimeout:      30 * time.Second,
		Weights:           GenerateDefaultWeights(10),
	}

	capacity := &Capacity{
		MaxTokensPerSec:  10000,
		MaxRPS:           100,
		MaxConcurrent:    10,
		MaxContextLength: 100000,
	}

	cs := NewChannelScheduler(config, capacity, PolicyHybrid)
	defer cs.Shutdown()

	// Test immediate acceptance (capacity available)
	req := &Request{
		ID:              "test-1",
		Priority:        PriorityNormal,
		EstimatedTokens: 100,
		AccountID:       "test-account",
		Model:           "gpt-4",
		ResultChan:      make(chan *ScheduleResult, 2),
		EnqueuedAt:      time.Now(),
		Deadline:        time.Now().Add(30 * time.Second),
	}

	err := cs.Submit(req)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Should get immediate acceptance
	select {
	case result := <-req.ResultChan:
		if !result.Accepted {
			t.Errorf("Expected acceptance, got rejection: %s", result.Reason)
		}
		if result.Reason != "capacity available" {
			t.Errorf("Expected 'capacity available', got: %s", result.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for result")
	}

	stats := cs.GetStats()
	if stats["total_scheduled"].(uint64) != 1 {
		t.Errorf("Expected 1 scheduled, got: %d", stats["total_scheduled"])
	}
}

func TestChannelScheduler_Submit_Queueing(t *testing.T) {
	config := &Config{
		NumPriorityLevels: 10,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     10,
		QueueTimeout:      30 * time.Second,
		Weights:           GenerateDefaultWeights(10),
	}

	capacity := &Capacity{
		MaxTokensPerSec:  1000,
		MaxRPS:           10,
		MaxConcurrent:    2, // Low limit to force queueing
		MaxContextLength: 100000,
	}

	cs := NewChannelScheduler(config, capacity, PolicyHybrid)
	defer cs.Shutdown()

	// Submit 3 requests (max concurrent = 2, so 3rd should queue)
	var wg sync.WaitGroup
	results := make([]*ScheduleResult, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := &Request{
				ID:              fmt.Sprintf("test-%d", idx),
				Priority:        PriorityNormal,
				EstimatedTokens: 100,
				AccountID:       "test-account",
				Model:           "gpt-4",
				ResultChan:      make(chan *ScheduleResult, 2),
				EnqueuedAt:      time.Now(),
				Deadline:        time.Now().Add(30 * time.Second),
			}

			err := cs.Submit(req)
			if err != nil {
				t.Errorf("Submit failed for request %d: %v", idx, err)
				return
			}

			select {
			case result := <-req.ResultChan:
				results[idx] = result
			case <-time.After(2 * time.Second):
				t.Errorf("Timeout waiting for result %d", idx)
			}
		}(i)
	}

	wg.Wait()

	// Check that at least one was queued
	immediateAccepts := 0
	queued := 0

	for i, result := range results {
		if result == nil {
			t.Errorf("Result %d is nil", i)
			continue
		}
		if !result.Accepted {
			t.Errorf("Request %d was rejected: %s", i, result.Reason)
			continue
		}
		if result.Reason == "capacity available" {
			immediateAccepts++
		} else if result.Reason == "queued" {
			queued++
		}
	}

	t.Logf("Results: %d immediate accepts, %d queued", immediateAccepts, queued)

	if queued == 0 {
		t.Error("Expected at least one request to be queued")
	}
}

func TestChannelScheduler_PriorityOrdering(t *testing.T) {
	config := &Config{
		NumPriorityLevels: 10,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     100,
		QueueTimeout:      30 * time.Second,
		Weights:           GenerateDefaultWeights(10),
	}

	capacity := &Capacity{
		MaxTokensPerSec:  1000,
		MaxRPS:           10,
		MaxConcurrent:    1, // Force all to queue
		MaxContextLength: 100000,
	}

	cs := NewChannelScheduler(config, capacity, PolicyStrictPriority)
	defer cs.Shutdown()

	// Fill capacity first
	firstReq := &Request{
		ID:              "first",
		Priority:        PriorityNormal,
		EstimatedTokens: 100,
		AccountID:       "test",
		Model:           "gpt-4",
		ResultChan:      make(chan *ScheduleResult, 2),
		EnqueuedAt:      time.Now(),
		Deadline:        time.Now().Add(30 * time.Second),
	}
	cs.Submit(firstReq)
	<-firstReq.ResultChan

	// Now submit high and low priority (both should queue)
	time.Sleep(100 * time.Millisecond)

	lowPriorityReq := &Request{
		ID:              "low-priority",
		Priority:        PriorityLow, // P7
		EstimatedTokens: 100,
		AccountID:       "test",
		Model:           "gpt-4",
		ResultChan:      make(chan *ScheduleResult, 2),
		EnqueuedAt:      time.Now(),
		Deadline:        time.Now().Add(30 * time.Second),
	}

	highPriorityReq := &Request{
		ID:              "high-priority",
		Priority:        PriorityHigh, // P2
		EstimatedTokens: 100,
		AccountID:       "test",
		Model:           "gpt-4",
		ResultChan:      make(chan *ScheduleResult, 2),
		EnqueuedAt:      time.Now(),
		Deadline:        time.Now().Add(30 * time.Second),
	}

	cs.Submit(lowPriorityReq)
	time.Sleep(50 * time.Millisecond)
	cs.Submit(highPriorityReq)

	// Both should be queued
	lowResult := <-lowPriorityReq.ResultChan
	highResult := <-highPriorityReq.ResultChan

	if lowResult.Reason != "queued" || highResult.Reason != "queued" {
		t.Errorf("Expected both to be queued, got: low=%s, high=%s",
			lowResult.Reason, highResult.Reason)
	}

	// Release first request
	cs.Release(firstReq)

	// Wait for scheduler to process
	time.Sleep(200 * time.Millisecond)

	// High priority should be scheduled first
	select {
	case result := <-highPriorityReq.ResultChan:
		if result.Reason != "scheduled" {
			t.Errorf("Expected high priority to be scheduled, got: %s", result.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for high priority to be scheduled")
	}
}

func TestChannelScheduler_Concurrency(t *testing.T) {
	config := &Config{
		NumPriorityLevels: 10,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     1000,
		QueueTimeout:      30 * time.Second,
		Weights:           GenerateDefaultWeights(10),
	}

	capacity := &Capacity{
		MaxTokensPerSec:  100000,
		MaxRPS:           1000,
		MaxConcurrent:    100,
		MaxContextLength: 100000,
	}

	cs := NewChannelScheduler(config, capacity, PolicyHybrid)
	defer cs.Shutdown()

	// Submit 100 requests concurrently
	numRequests := 100
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := &Request{
				ID:              fmt.Sprintf("concurrent-%d", idx),
				Priority:        PriorityTier(idx % 10),
				EstimatedTokens: 100,
				AccountID:       fmt.Sprintf("account-%d", idx%10),
				Model:           "gpt-4",
				ResultChan:      make(chan *ScheduleResult, 2),
				EnqueuedAt:      time.Now(),
				Deadline:        time.Now().Add(30 * time.Second),
			}

			err := cs.Submit(req)
			if err != nil {
				errors <- err
				return
			}

			select {
			case result := <-req.ResultChan:
				if !result.Accepted {
					errors <- fmt.Errorf("request %s rejected: %s", req.ID, result.Reason)
				}
			case <-time.After(5 * time.Second):
				errors <- fmt.Errorf("timeout for request %s", req.ID)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Errorf("Error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Got %d errors out of %d requests", errorCount, numRequests)
	}

	stats := cs.GetStats()
	totalScheduled := stats["total_scheduled"].(uint64)

	if totalScheduled != uint64(numRequests) {
		t.Errorf("Expected %d scheduled, got: %d", numRequests, totalScheduled)
	}

	t.Logf("Successfully processed %d concurrent requests", numRequests)
}

func TestChannelScheduler_HighCapacity(t *testing.T) {
	// Test with high queue depth (production settings)
	config := &Config{
		NumPriorityLevels: 10,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     10000, // High capacity
		QueueTimeout:      30 * time.Second,
		Weights:           GenerateDefaultWeights(10),
	}

	capacity := &Capacity{
		MaxTokensPerSec:  100000,
		MaxRPS:           1000,
		MaxConcurrent:    10, // Low to force queueing
		MaxContextLength: 100000,
	}

	cs := NewChannelScheduler(config, capacity, PolicyHybrid)
	defer cs.Shutdown()

	// Submit 1000 requests quickly (burst test)
	numRequests := 1000
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)
	accepted := make(chan bool, numRequests)

	startTime := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			req := &Request{
				ID:              fmt.Sprintf("burst-%d", idx),
				Priority:        PriorityTier(idx % 10),
				EstimatedTokens: 100,
				AccountID:       fmt.Sprintf("account-%d", idx%10),
				Model:           "gpt-4",
				ResultChan:      make(chan *ScheduleResult, 2),
				EnqueuedAt:      time.Now(),
				Deadline:        time.Now().Add(30 * time.Second),
			}

			err := cs.Submit(req)
			if err != nil {
				errors <- err
				return
			}

			select {
			case result := <-req.ResultChan:
				if result.Accepted {
					accepted <- true
				} else {
					errors <- fmt.Errorf("request %s rejected: %s", req.ID, result.Reason)
				}
			case <-time.After(5 * time.Second):
				errors <- fmt.Errorf("timeout for request %s", req.ID)
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	close(accepted)

	elapsedTime := time.Since(startTime)

	errorCount := 0
	for err := range errors {
		t.Logf("Error: %v", err)
		errorCount++
	}

	acceptedCount := len(accepted)

	t.Logf("Burst test results:")
	t.Logf("  - Submitted: %d requests", numRequests)
	t.Logf("  - Accepted: %d requests", acceptedCount)
	t.Logf("  - Errors: %d", errorCount)
	t.Logf("  - Time: %v", elapsedTime)
	t.Logf("  - Throughput: %.1f req/s", float64(numRequests)/elapsedTime.Seconds())

	if errorCount > 0 {
		t.Errorf("Got %d errors out of %d requests", errorCount, numRequests)
	}

	// Should accept all requests (queue depth is 10000 per priority)
	if acceptedCount < numRequests {
		t.Errorf("Expected all %d requests accepted, got %d", numRequests, acceptedCount)
	}
}

func TestChannelScheduler_Release(t *testing.T) {
	config := &Config{
		NumPriorityLevels: 10,
		DefaultPriority:   PriorityNormal,
		MaxQueueDepth:     100,
		QueueTimeout:      30 * time.Second,
		Weights:           GenerateDefaultWeights(10),
	}

	capacity := &Capacity{
		MaxTokensPerSec:  1000,
		MaxRPS:           10,
		MaxConcurrent:    1,
		MaxContextLength: 100000,
	}

	cs := NewChannelScheduler(config, capacity, PolicyHybrid)
	defer cs.Shutdown()

	// Submit first request
	req1 := &Request{
		ID:              "req1",
		Priority:        PriorityNormal,
		EstimatedTokens: 100,
		AccountID:       "test",
		Model:           "gpt-4",
		ResultChan:      make(chan *ScheduleResult, 2),
		EnqueuedAt:      time.Now(),
		Deadline:        time.Now().Add(30 * time.Second),
	}

	cs.Submit(req1)
	result1 := <-req1.ResultChan

	if result1.Reason != "capacity available" {
		t.Errorf("Expected immediate acceptance, got: %s", result1.Reason)
	}

	// Submit second request (should queue)
	req2 := &Request{
		ID:              "req2",
		Priority:        PriorityNormal,
		EstimatedTokens: 100,
		AccountID:       "test",
		Model:           "gpt-4",
		ResultChan:      make(chan *ScheduleResult, 2),
		EnqueuedAt:      time.Now(),
		Deadline:        time.Now().Add(30 * time.Second),
	}

	cs.Submit(req2)
	result2 := <-req2.ResultChan

	if result2.Reason != "queued" {
		t.Errorf("Expected queued, got: %s", result2.Reason)
	}

	// Release first request
	cs.Release(req1)

	// Wait for scheduler to process
	time.Sleep(200 * time.Millisecond)

	// Second request should be scheduled
	select {
	case result := <-req2.ResultChan:
		if result.Reason != "scheduled" {
			t.Errorf("Expected scheduled, got: %s", result.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for req2 to be scheduled")
	}
}
