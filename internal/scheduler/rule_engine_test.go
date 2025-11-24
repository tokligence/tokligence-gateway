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

func (t *testAdjustableScheduler) UpdateCapacity(maxTokensPerSec *int64, maxRPS *int, maxConcurrent *int) error {
	if maxTokensPerSec != nil {
		t.capacity.MaxTokensPerSec = int(*maxTokensPerSec)
	}
	if maxRPS != nil {
		t.capacity.MaxRPS = *maxRPS
	}
	if maxConcurrent != nil {
		t.capacity.MaxConcurrent = *maxConcurrent
	}
	t.updateCapacityCalls++
	return nil
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
