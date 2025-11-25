package scheduler

import (
	"reflect"
	"testing"
	"time"
)

type testAdjustableScheduler struct {
	weights             []float64
	capacity            CapacityLimits
	updateWeightsCalls  int
	updateCapacityCalls int
}

func (t *testAdjustableScheduler) CurrentWeights() []float64 {
	return append([]float64{}, t.weights...)
}

func (t *testAdjustableScheduler) UpdateWeights(weights []float64) error {
	t.weights = append([]float64{}, weights...)
	t.updateWeightsCalls++
	return nil
}

func (t *testAdjustableScheduler) CurrentCapacity() CapacityLimits {
	return t.capacity
}

func (t *testAdjustableScheduler) UpdateCapacity(maxTokensPerSec *int64, maxRPS *int, maxConcurrent *int, maxContextLength *int) error {
	if maxTokensPerSec != nil {
		t.capacity.MaxTokensPerSec = int(*maxTokensPerSec)
	}
	if maxRPS != nil {
		t.capacity.MaxRPS = *maxRPS
	}
	if maxConcurrent != nil {
		t.capacity.MaxConcurrent = *maxConcurrent
	}
	if maxContextLength != nil {
		t.capacity.MaxContextLength = *maxContextLength
	}
	t.updateCapacityCalls++
	return nil
}

// TestRuleEngineTickerManagement tests that tickers are properly managed during config changes
func TestRuleEngineTickerManagement(t *testing.T) {
	// This test verifies the ticker recreation logic doesn't leak
	engine, err := NewRuleEngine(RuleEngineConfig{
		Enabled:         true,
		CheckInterval:   100 * time.Millisecond,
		DefaultTimezone: "UTC",
	})
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// Start the engine
	if err := engine.Start(nil); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}

	// Let it run a few cycles
	time.Sleep(250 * time.Millisecond)

	// Change the check interval (simulates hot reload)
	engine.mu.Lock()
	engine.checkInterval = 50 * time.Millisecond
	engine.mu.Unlock()

	// Let it run with new interval
	time.Sleep(150 * time.Millisecond)

	// Stop the engine - this should not leak
	if err := engine.Stop(); err != nil {
		t.Fatalf("failed to stop engine: %v", err)
	}

	// Calling Stop again should be safe (idempotent)
	if err := engine.Stop(); err != nil {
		t.Fatalf("second Stop() should be idempotent: %v", err)
	}
}

func TestRuleEngineRefreshesBaselinesWhenIdle(t *testing.T) {
	sched := &testAdjustableScheduler{
		weights:  []float64{1, 2, 3},
		capacity: CapacityLimits{MaxTokensPerSec: 100, MaxRPS: 10, MaxConcurrent: 5},
	}

	engine, err := NewRuleEngine(RuleEngineConfig{
		Enabled:         true,
		CheckInterval:   time.Second,
		DefaultTimezone: "UTC",
	})
	if err != nil {
		t.Fatalf("failed to build rule engine: %v", err)
	}
	engine.timeNow = func() time.Time { return time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC) }
	engine.SetScheduler(sched)

	if err := engine.ApplyRulesNow(); err != nil {
		t.Fatalf("apply rules failed: %v", err)
	}

	// Simulate manual runtime adjustments to scheduler
	sched.weights = []float64{9, 9, 9}
	sched.capacity = CapacityLimits{MaxTokensPerSec: 200, MaxRPS: 20, MaxConcurrent: 10}

	if err := engine.ApplyRulesNow(); err != nil {
		t.Fatalf("apply rules failed: %v", err)
	}

	if !reflect.DeepEqual(engine.baseWeights, []float64{9, 9, 9}) {
		t.Fatalf("expected base weights to refresh to manual values, got %v", engine.baseWeights)
	}
	if !capacityEqual(engine.baseCapacity, sched.capacity) {
		t.Fatalf("expected base capacity to refresh to manual values, got %+v", engine.baseCapacity)
	}

	if sched.updateWeightsCalls != 0 {
		t.Fatalf("expected no weight updates during idle refresh, got %d", sched.updateWeightsCalls)
	}
	if sched.updateCapacityCalls != 0 {
		t.Fatalf("expected no capacity updates during idle refresh, got %d", sched.updateCapacityCalls)
	}
}
