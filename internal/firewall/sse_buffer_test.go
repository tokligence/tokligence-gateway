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

func TestSSEPIIBuffer_NormalBrackets(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Normal brackets that are not PII tokens should pass through
	result := buf.ProcessChunk(ctx, "array[0] and [some text]")
	if result != "array[0] and [some text]" {
		t.Errorf("expected 'array[0] and [some text]', got %q", result)
	}
}

func TestSSEPIIBuffer_Flush(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Partial token at end of stream (incomplete)
	result := buf.ProcessChunk(ctx, "Hello [UNKN")
	if result != "Hello " {
		t.Errorf("expected 'Hello ', got %q", result)
	}

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

func TestSSEPIIBuffer_CharByCharStreaming(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	// Store a token mapping
	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_abc123]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Simulate OpenAI's character-by-character streaming
	chunks := []string{"[", "P", "E", "R", "S", "O", "N", "_", "a", "b", "c", "1", "2", "3", "]", "好"}

	var result string
	for _, chunk := range chunks {
		out := buf.ProcessChunk(ctx, chunk)
		result += out
	}

	// Flush remaining
	result += buf.Flush(ctx)

	if result != "张三好" {
		t.Errorf("expected '张三好', got %q", result)
	}
}

func TestSSEPIIBuffer_SingleBracketAtEnd(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Chunk ending with single [
	result := buf.ProcessChunk(ctx, "Hello [")
	if result != "Hello " {
		t.Errorf("expected 'Hello ', got %q", result)
	}

	// Buffer should have [
	if !buf.HasBufferedContent() {
		t.Error("expected buffer to have content")
	}
}

func TestSSEPIIBuffer_PartialTokenAcrossChunks(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	// Store a token mapping
	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_abc123]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Token split across multiple chunks
	result1 := buf.ProcessChunk(ctx, "Hello [PERS")
	if result1 != "Hello " {
		t.Errorf("expected 'Hello ', got %q", result1)
	}

	result2 := buf.ProcessChunk(ctx, "ON_abc123] world")
	if result2 != "张三 world" {
		t.Errorf("expected '张三 world', got %q", result2)
	}
}

func TestSSEPIIBuffer_UnmatchedToken(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	// Don't store any mapping - token should pass through as-is
	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	result := buf.ProcessChunk(ctx, "Hello [PERSON_unknown]")
	// Token format matches but no mapping exists - should output as-is
	if result != "Hello [PERSON_unknown]" {
		t.Errorf("expected 'Hello [PERSON_unknown]', got %q", result)
	}
}

func TestSSEPIIBuffer_MixedContent(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_aaa111]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Mixed content with normal brackets and PII token
	result := buf.ProcessChunk(ctx, "array[0] has [PERSON_aaa111] at index [1]")
	if result != "array[0] has 张三 at index [1]" {
		t.Errorf("expected 'array[0] has 张三 at index [1]', got %q", result)
	}
}

func TestSSEPIIBuffer_ForceFlushOnMaxLength(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Buffer content longer than MaxBufferLength (30 chars)
	// This simulates a malformed or non-PII bracket that's too long
	longContent := "[THIS_IS_A_VERY_LONG_BRACKET_CONTENT_THAT_EXCEEDS_LIMIT"
	result := buf.ProcessChunk(ctx, longContent)

	// Should have force flushed the buffer
	if result != longContent {
		t.Errorf("expected force flush to return %q, got %q", longContent, result)
	}

	// Buffer should be empty
	if buf.HasBufferedContent() {
		t.Error("expected buffer to be empty after force flush")
	}
}

func TestSSEPIIBuffer_NormalTokenNotForceFlush(t *testing.T) {
	tokenizer := NewPIITokenizerWithMemoryStore()
	ctx := context.Background()
	sessionID := "test-session"

	_ = tokenizer.StoreExternalToken(ctx, sessionID, "PERSON", "张三", "[PERSON_abc123]")

	buf := NewSSEPIIBuffer(tokenizer, sessionID)

	// Normal token length (16 chars) should NOT trigger force flush
	result := buf.ProcessChunk(ctx, "[PERSON_abc123]")
	if result != "张三" {
		t.Errorf("expected '张三', got %q", result)
	}
}
