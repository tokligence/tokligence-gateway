package firewall

import (
	"context"
	"strings"
	"testing"
)

func TestPipeline_Monitor_Mode(t *testing.T) {
	pipeline := NewPipeline(ModeMonitor, nil)

	// Add a filter that would normally block
	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "test_filter",
		Direction:     DirectionInput,
		RedactEnabled: false,
	})
	pipeline.AddFilter(filter)

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("SSN: 123-45-6789")

	// In monitor mode, should not error even with PII
	err := pipeline.ProcessInput(context.Background(), ctx)
	if err != nil {
		t.Errorf("Monitor mode should not block: %v", err)
	}

	// Should still detect
	if ctx.Annotations["pii_count"] == nil {
		t.Error("PII should be detected in monitor mode")
	}
}

func TestPipeline_Enforce_Mode(t *testing.T) {
	pipeline := NewPipeline(ModeEnforce, nil)

	// Add a filter that blocks on detection
	filter := &testBlockingFilter{}
	pipeline.AddFilter(filter)

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("bad content")

	// Should return error in enforce mode
	err := pipeline.ProcessInput(context.Background(), ctx)
	if err == nil {
		t.Error("Enforce mode should block when filter sets Block=true")
	}

	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("Expected 'blocked' in error, got: %v", err)
	}
}

func TestPipeline_Disabled_Mode(t *testing.T) {
	pipeline := NewPipeline(ModeDisabled, nil)

	filter := &testBlockingFilter{}
	pipeline.AddFilter(filter)

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("any content")

	// Should not run filters when disabled
	err := pipeline.ProcessInput(context.Background(), ctx)
	if err != nil {
		t.Errorf("Disabled mode should not run filters: %v", err)
	}

	if ctx.Block {
		t.Error("Context should not be blocked in disabled mode")
	}
}

func TestPipeline_Priority_Order(t *testing.T) {
	pipeline := NewPipeline(ModeMonitor, nil)

	var executionOrder []string

	filter1 := &testOrderFilter{name: "filter1", priority: 20, order: &executionOrder}
	filter2 := &testOrderFilter{name: "filter2", priority: 10, order: &executionOrder}
	filter3 := &testOrderFilter{name: "filter3", priority: 5, order: &executionOrder}

	// Add in random order
	pipeline.AddFilter(filter1)
	pipeline.AddFilter(filter2)
	pipeline.AddFilter(filter3)

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("test")

	err := pipeline.ProcessInput(context.Background(), ctx)
	if err != nil {
		t.Fatalf("ProcessInput failed: %v", err)
	}

	// Should execute in priority order (lower priority first)
	expected := []string{"filter3", "filter2", "filter1"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("Expected %d filters executed, got %d", len(expected), len(executionOrder))
	}

	for i, name := range expected {
		if executionOrder[i] != name {
			t.Errorf("Expected filter %s at position %d, got %s", name, i, executionOrder[i])
		}
	}
}

func TestPipeline_Redaction(t *testing.T) {
	pipeline := NewPipeline(ModeEnforce, nil)

	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "redactor",
		Direction:     DirectionInput,
		RedactEnabled: true,
		EnabledTypes:  []string{"EMAIL"},
	})
	pipeline.AddFilter(filter)

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("Contact: user@example.com")

	err := pipeline.ProcessInput(context.Background(), ctx)
	if err != nil {
		t.Fatalf("ProcessInput failed: %v", err)
	}

	// Body should be modified
	if string(ctx.RequestBody) == "Contact: user@example.com" {
		t.Error("Body should have been redacted")
	}

	if !strings.Contains(string(ctx.RequestBody), "[EMAIL]") {
		t.Errorf("Expected [EMAIL] in redacted body, got: %s", string(ctx.RequestBody))
	}
}

// Test helper: blocking filter
type testBlockingFilter struct{}

func (f *testBlockingFilter) Name() string                           { return "blocking_filter" }
func (f *testBlockingFilter) Priority() int                          { return 10 }
func (f *testBlockingFilter) Direction() Direction                   { return DirectionInput }
func (f *testBlockingFilter) ApplyInput(ctx *FilterContext) error {
	ctx.Block = true
	ctx.BlockReason = "test block"
	return nil
}

// Test helper: order tracking filter
type testOrderFilter struct {
	name     string
	priority int
	order    *[]string
}

func (f *testOrderFilter) Name() string           { return f.name }
func (f *testOrderFilter) Priority() int          { return f.priority }
func (f *testOrderFilter) Direction() Direction   { return DirectionInput }
func (f *testOrderFilter) ApplyInput(ctx *FilterContext) error {
	*f.order = append(*f.order, f.name)
	return nil
}
