package firewall

import (
	"context"
	"testing"
)

func TestPIIRegexFilter_Email(t *testing.T) {
	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "test_pii",
		Direction:     DirectionInput,
		RedactEnabled: true,
		EnabledTypes:  []string{"EMAIL"},
	})

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("Contact me at john.doe@example.com for more info")

	err := filter.ApplyInput(ctx)
	if err != nil {
		t.Fatalf("ApplyInput failed: %v", err)
	}

	// Check detection
	piiCount, ok := ctx.Annotations["pii_count"].(int)
	if !ok || piiCount != 1 {
		t.Errorf("Expected 1 PII detection, got %v", piiCount)
	}

	// Check redaction
	if len(ctx.ModifiedRequestBody) == 0 {
		t.Error("Expected redacted body, got none")
	}

	redacted := string(ctx.ModifiedRequestBody)
	if redacted != "Contact me at [EMAIL] for more info" {
		t.Errorf("Unexpected redaction: %s", redacted)
	}
}

func TestPIIRegexFilter_SSN(t *testing.T) {
	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "test_pii",
		Direction:     DirectionInput,
		RedactEnabled: true,
		EnabledTypes:  []string{"SSN"},
	})

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("My SSN is 123-45-6789")

	err := filter.ApplyInput(ctx)
	if err != nil {
		t.Fatalf("ApplyInput failed: %v", err)
	}

	// Check detection
	piiCount, ok := ctx.Annotations["pii_count"].(int)
	if !ok || piiCount != 1 {
		t.Errorf("Expected 1 PII detection, got %v", piiCount)
	}

	// Check redaction
	redacted := string(ctx.ModifiedRequestBody)
	if redacted != "My SSN is [SSN]" {
		t.Errorf("Unexpected redaction: %s", redacted)
	}
}

func TestPIIRegexFilter_Multiple(t *testing.T) {
	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "test_pii",
		Direction:     DirectionInput,
		RedactEnabled: true,
	})

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("Email: john@example.com, Phone: 555-123-4567, SSN: 123-45-6789")

	err := filter.ApplyInput(ctx)
	if err != nil {
		t.Fatalf("ApplyInput failed: %v", err)
	}

	// Check detection count
	piiCount, ok := ctx.Annotations["pii_count"].(int)
	if !ok || piiCount < 2 {
		t.Errorf("Expected at least 2 PII detections, got %v", piiCount)
	}

	// Check that redaction occurred
	if len(ctx.ModifiedRequestBody) == 0 {
		t.Error("Expected redacted body, got none")
	}
}

func TestPIIRegexFilter_NoDetection(t *testing.T) {
	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "test_pii",
		Direction:     DirectionInput,
		RedactEnabled: true,
	})

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("This is a clean message with no PII")

	err := filter.ApplyInput(ctx)
	if err != nil {
		t.Fatalf("ApplyInput failed: %v", err)
	}

	// Check no detection
	piiCount, _ := ctx.Annotations["pii_count"].(int)
	if piiCount != 0 {
		t.Errorf("Expected 0 PII detections, got %v", piiCount)
	}

	// Check no modification
	if len(ctx.ModifiedRequestBody) > 0 {
		t.Error("Expected no modification, but body was modified")
	}
}

func TestPIIRegexFilter_RedactDisabled(t *testing.T) {
	filter := NewPIIRegexFilter(PIIRegexFilterConfig{
		Name:          "test_pii",
		Direction:     DirectionInput,
		RedactEnabled: false, // Detection only
	})

	ctx := NewFilterContext(context.Background())
	ctx.RequestBody = []byte("My email is test@example.com")

	err := filter.ApplyInput(ctx)
	if err != nil {
		t.Fatalf("ApplyInput failed: %v", err)
	}

	// Should detect but not redact
	piiCount, ok := ctx.Annotations["pii_count"].(int)
	if !ok || piiCount != 1 {
		t.Errorf("Expected 1 PII detection, got %v", piiCount)
	}

	if len(ctx.ModifiedRequestBody) > 0 {
		t.Error("Body should not be modified when redact is disabled")
	}
}
