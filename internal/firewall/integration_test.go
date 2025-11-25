package firewall

import (
	"context"
	"testing"
)

func TestPipelineSSEBufferIntegration(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session"

	// Create pipeline in redact mode
	pipeline := NewPipeline(ModeRedact, nil)

	// Process input - this sets up fctx.Tokenizer
	fctx := NewFilterContext(ctx)
	fctx.SessionID = sessionID
	fctx.RequestBody = []byte(`{"test": "data"}`)

	err := pipeline.ProcessInput(ctx, fctx)
	if err != nil {
		t.Fatalf("ProcessInput failed: %v", err)
	}

	// Store a token via fctx.Tokenizer (simulating http_filter behavior)
	err = fctx.Tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_abc123]")
	if err != nil {
		t.Fatalf("StoreExternalToken failed: %v", err)
	}

	// Create SSE buffer using same pipeline
	sseBuffer := pipeline.NewSSEBuffer(sessionID)
	if sseBuffer == nil {
		t.Fatal("NewSSEBuffer returned nil")
	}

	if !sseBuffer.IsEnabled() {
		t.Fatal("SSE buffer should be enabled in redact mode")
	}

	// Process a chunk containing the token
	result := sseBuffer.ProcessChunk(ctx, "[PERSON_abc123]好")

	if result != "张三好" {
		t.Errorf("expected '张三好', got %q", result)
	}
}

func TestPipelineSSEBufferTokenizerSharing(t *testing.T) {
	ctx := context.Background()
	sessionID := "test-session-2"

	pipeline := NewPipeline(ModeRedact, nil)

	// Get tokenizer via ProcessInput
	fctx := NewFilterContext(ctx)
	fctx.SessionID = sessionID
	fctx.RequestBody = []byte(`{}`)
	_ = pipeline.ProcessInput(ctx, fctx)

	// Store token with hex-only hash (like Presidio generates)
	_ = fctx.Tokenizer.StoreExternalToken(ctx, sessionID, "EMAIL", "test@example.com", "[EMAIL_def456]")

	// Verify token can be retrieved via pipeline's tokenizer
	sseBuffer := pipeline.NewSSEBuffer(sessionID)
	result := sseBuffer.ProcessChunk(ctx, "Contact: [EMAIL_def456]")

	if result != "Contact: test@example.com" {
		t.Errorf("expected 'Contact: test@example.com', got %q", result)
	}
}
