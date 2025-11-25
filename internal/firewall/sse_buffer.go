package firewall

import (
	"context"
	"regexp"
	"strings"
)

// SSEPIIBuffer handles buffering and detokenization of SSE stream chunks
// when PII token patterns might span across chunks.
//
// Token format: [TYPE_HASH] e.g., [PERSON_25c0fe], [EMAIL_abc123]
// The buffer accumulates partial tokens and detokenizes complete ones.
type SSEPIIBuffer struct {
	tokenizer    *PIITokenizer
	sessionID    string
	buffer       strings.Builder
	enabled      bool

	// Pattern to detect potential token starts: [ followed by uppercase letters
	partialPattern *regexp.Regexp
	// Pattern to detect complete tokens: [TYPE_HASH]
	completePattern *regexp.Regexp
}

// NewSSEPIIBuffer creates a new SSE buffer for PII detokenization.
// If tokenizer is nil or sessionID is empty, buffering is disabled and
// chunks pass through unchanged.
func NewSSEPIIBuffer(tokenizer *PIITokenizer, sessionID string) *SSEPIIBuffer {
	enabled := tokenizer != nil && sessionID != ""
	return &SSEPIIBuffer{
		tokenizer:       tokenizer,
		sessionID:       sessionID,
		enabled:         enabled,
		// Match partial token at end: [, [P, [PERSON, [PERSON_, [PERSON_abc
		partialPattern:  regexp.MustCompile(`\[[A-Z][A-Z0-9_]*$`),
		// Match complete token: [TYPE_HASH]
		completePattern: regexp.MustCompile(`\[[A-Z]+_[a-f0-9]+\]`),
	}
}

// ProcessChunk processes a streaming chunk and returns the content that
// should be sent to the client.
//
// If buffering is disabled, returns the chunk unchanged.
// Otherwise, it:
// 1. Appends chunk to buffer
// 2. Detokenizes any complete tokens in buffer
// 3. Returns content up to any partial token at the end
// 4. Keeps partial token in buffer for next chunk
func (b *SSEPIIBuffer) ProcessChunk(ctx context.Context, chunk string) string {
	if !b.enabled {
		return chunk
	}

	// Append to buffer
	b.buffer.WriteString(chunk)
	content := b.buffer.String()

	// Check if there's a partial token at the end
	partialMatch := b.partialPattern.FindStringIndex(content)

	var toProcess string
	var toBuffer string

	if partialMatch != nil {
		// Split at partial token boundary
		toProcess = content[:partialMatch[0]]
		toBuffer = content[partialMatch[0]:]
	} else {
		// No partial token, process everything
		toProcess = content
		toBuffer = ""
	}

	// Detokenize complete tokens in the processable part
	if toProcess != "" && b.completePattern.MatchString(toProcess) {
		detokenized, err := b.tokenizer.DetokenizeAll(ctx, b.sessionID, toProcess)
		if err == nil {
			toProcess = detokenized
		}
	}

	// Reset buffer with remaining partial token
	b.buffer.Reset()
	if toBuffer != "" {
		b.buffer.WriteString(toBuffer)
	}

	return toProcess
}

// Flush returns any remaining buffered content, detokenizing if possible.
// Call this when the stream ends to ensure no content is lost.
func (b *SSEPIIBuffer) Flush(ctx context.Context) string {
	if !b.enabled || b.buffer.Len() == 0 {
		return ""
	}

	content := b.buffer.String()
	b.buffer.Reset()

	// Try to detokenize any complete tokens
	if b.completePattern.MatchString(content) {
		detokenized, err := b.tokenizer.DetokenizeAll(ctx, b.sessionID, content)
		if err == nil {
			return detokenized
		}
	}

	return content
}

// HasBufferedContent returns true if there's content waiting in the buffer.
func (b *SSEPIIBuffer) HasBufferedContent() bool {
	return b.buffer.Len() > 0
}

// IsEnabled returns true if buffering is active.
func (b *SSEPIIBuffer) IsEnabled() bool {
	return b.enabled
}

// Reset clears the buffer state.
func (b *SSEPIIBuffer) Reset() {
	b.buffer.Reset()
}
