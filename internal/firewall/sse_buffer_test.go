package firewall

import (
	"context"
	"testing"
)

func TestSSEPIIBuffer_Disabled(t *testing.T) {
	// Buffer with nil tokenizer should be disabled
	buf := NewSSEPIIBuffer(nil, "session-1")
	if buf.IsEnabled() {
		t.Error("expected buffer to be disabled with nil tokenizer")
	}

	// Chunks should pass through unchanged
	chunk := "Hello [PERSON_abc123] world"
	result := buf.ProcessChunk(context.Background(), chunk)
	if result != chunk {
		t.Errorf("expected chunk to pass through, got %q", result)
	}
}

func TestSSEPIIBuffer_DisabledEmptySession(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	buf := NewSSEPIIBuffer(tokenizer, "")
	if buf.IsEnabled() {
		t.Error("expected buffer to be disabled with empty session")
	}
}

func TestSSEPIIBuffer_PartialToken(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	// Store a token mapping
	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_abc123]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)
	if !buf.IsEnabled() {
		t.Fatal("expected buffer to be enabled")
	}

	// Simulate token split across chunks
	// First chunk ends with partial token
	result1 := buf.ProcessChunk(ctx, "Hello [PERS")
	if result1 != "Hello " {
		t.Errorf("expected 'Hello ', got %q", result1)
	}

	// Buffer should have content
	if !buf.HasBufferedContent() {
		t.Error("expected buffer to have content")
	}

	// Second chunk completes the token
	result2 := buf.ProcessChunk(ctx, "ON_abc123] world")
	// Should detokenize [PERSON_abc123] to 张三
	if result2 != "张三 world" {
		t.Errorf("expected '张三 world', got %q", result2)
	}
}

func TestSSEPIIBuffer_CompleteToken(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	// Store a token mapping
	_ = tokenizer.StoreExternalToken(ctx, sessionID, "EMAIL", "test@example.com", "[EMAIL_def456]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Complete token in single chunk
	result := buf.ProcessChunk(ctx, "Contact: [EMAIL_def456] for help")
	if result != "Contact: test@example.com for help" {
		t.Errorf("expected 'Contact: test@example.com for help', got %q", result)
	}

	// No buffered content
	if buf.HasBufferedContent() {
		t.Error("expected no buffered content")
	}
}

func TestSSEPIIBuffer_NoTokens(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Regular text without tokens
	chunk := "This is regular text without any PII tokens"
	result := buf.ProcessChunk(ctx, chunk)
	if result != chunk {
		t.Errorf("expected chunk to pass through, got %q", result)
	}
}

func TestSSEPIIBuffer_Flush(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Partial token at end of stream
	_ = buf.ProcessChunk(ctx, "Hello [UNKN")

	// Flush should return remaining content
	remaining := buf.Flush(ctx)
	if remaining != "[UNKN" {
		t.Errorf("expected '[UNKN', got %q", remaining)
	}

	// Buffer should be empty after flush
	if buf.HasBufferedContent() {
		t.Error("expected buffer to be empty after flush")
	}
}

func TestSSEPIIBuffer_MultipleTokens(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	// Store multiple token mappings
	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_111111]")
	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "李四", "[PERSON_222222]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Multiple tokens in one chunk
	result := buf.ProcessChunk(ctx, "[PERSON_111111]和[PERSON_222222]开会了")
	if result != "张三和李四开会了" {
		t.Errorf("expected '张三和李四开会了', got %q", result)
	}
}
