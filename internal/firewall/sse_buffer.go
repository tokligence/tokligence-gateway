package firewall

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// Default values for SSE PII buffer
const (
	// DefaultMaxBufferLength is the maximum characters to buffer before force flushing.
	// Normal PII tokens are ~20 chars (e.g., [PERSON_abc123]), so 30 gives buffer.
	DefaultMaxBufferLength = 30

	// DefaultBufferTimeout is the maximum time to wait for closing bracket.
	// If exceeded, buffer is flushed as-is to prevent stalling.
	// Note: SSE streaming has ~20-50ms delays between chunks, so we need a longer timeout.
	// Typical token takes 10-15 chunks at ~30ms each = ~300-450ms total.
	DefaultBufferTimeout = 500 * time.Millisecond
)

// SSEPIIBuffer handles buffering and detokenization of SSE stream chunks
// when PII token patterns might span across chunks.
//
// Token format: [TYPE_HASH] e.g., [PERSON_25c0fe], [EMAIL_abc123]
// The buffer accumulates content between [ and ] and detokenizes complete tokens.
//
// Safety limits:
// - maxBufferLength (default 30 chars): Force flush if buffer exceeds this
// - bufferTimeout (default 500ms): Force flush if waiting too long for ]
type SSEPIIBuffer struct {
	tokenizer *PIITokenizer
	sessionID string
	buffer    strings.Builder
	enabled   bool
	inBracket bool // true when we've seen [ but not yet ]
	bracketStartTime time.Time // when [ was encountered

	// Configurable limits
	maxBufferLength int
	bufferTimeout   time.Duration

	// Pattern to detect complete tokens: [TYPE_HASH]
	completePattern *regexp.Regexp
}

// SSEBufferConfig holds optional configuration for SSEPIIBuffer.
type SSEBufferConfig struct {
	MaxBufferLength int           // Max chars to buffer (default: 30)
	BufferTimeout   time.Duration // Max time to wait for ] (default: 500ms)
}

// NewSSEPIIBuffer creates a new SSE buffer for PII detokenization.
// If tokenizer is nil or sessionID is empty, buffering is disabled and
// chunks pass through unchanged.
func NewSSEPIIBuffer(tokenizer *PIITokenizer, sessionID string) *SSEPIIBuffer {
	return NewSSEPIIBufferWithConfig(tokenizer, sessionID, SSEBufferConfig{})
}

// NewSSEPIIBufferWithConfig creates a new SSE buffer with custom configuration.
func NewSSEPIIBufferWithConfig(tokenizer *PIITokenizer, sessionID string, cfg SSEBufferConfig) *SSEPIIBuffer {
	enabled := tokenizer != nil && sessionID != ""

	maxLen := cfg.MaxBufferLength
	if maxLen <= 0 {
		maxLen = DefaultMaxBufferLength
	}
	timeout := cfg.BufferTimeout
	if timeout <= 0 {
		timeout = DefaultBufferTimeout
	}

	return &SSEPIIBuffer{
		tokenizer:       tokenizer,
		sessionID:       sessionID,
		enabled:         enabled,
		inBracket:       false,
		maxBufferLength: maxLen,
		bufferTimeout:   timeout,
		// Match complete token: [TYPE_HASH] where TYPE can include underscores
		// (e.g., MEDICAL_LICENSE, US_BANK_NUMBER, NRP) and HASH is alphanumeric
		completePattern: regexp.MustCompile(`^\[[A-Z][A-Z0-9_]*_[a-zA-Z0-9]+\]$`),
	}
}

// ProcessChunk processes a streaming chunk and returns the content that
// should be sent to the client.
//
// If buffering is disabled, returns the chunk unchanged.
// Otherwise, it buffers content between [ and ] brackets, then attempts
// to detokenize when a complete bracket pair is found.
//
// Safety: Force flushes buffer if MaxBufferLength or BufferTimeout exceeded.
func (b *SSEPIIBuffer) ProcessChunk(ctx context.Context, chunk string) string {
	if !b.enabled {
		return chunk
	}

	var result strings.Builder

	for _, ch := range chunk {
		if ch == '[' {
			// Start buffering - flush any previous content first
			if b.inBracket {
				// Nested bracket or incomplete - flush previous buffer as-is
				result.WriteString(b.buffer.String())
				b.buffer.Reset()
			}
			b.inBracket = true
			b.bracketStartTime = time.Now()
			b.buffer.WriteRune(ch)
		} else if ch == ']' && b.inBracket {
			// End of bracket - try to match token
			b.buffer.WriteRune(ch)
			token := b.buffer.String()
			b.buffer.Reset()
			b.inBracket = false

			// Check if it's a valid PII token
			if b.completePattern.MatchString(token) {
				// Try to detokenize
				detokenized, err := b.tokenizer.DetokenizeAll(ctx, b.sessionID, token)
				if err == nil && detokenized != token {
					result.WriteString(detokenized)
				} else {
					// No mapping found, output as-is
					// Debug: this means token was not stored during input processing
					result.WriteString(token)
				}
			} else {
				// Not a PII token pattern, output as-is
				result.WriteString(token)
			}
		} else if b.inBracket {
			// Inside bracket, keep buffering
			b.buffer.WriteRune(ch)

			// Safety check: force flush if buffer too long or timeout
			if b.shouldForceFlush() {
				result.WriteString(b.buffer.String())
				b.buffer.Reset()
				b.inBracket = false
			}
		} else {
			// Outside bracket, output directly
			result.WriteRune(ch)
		}
	}

	return result.String()
}

// shouldForceFlush returns true if buffer should be force flushed due to
// exceeding maxBufferLength or bufferTimeout.
func (b *SSEPIIBuffer) shouldForceFlush() bool {
	// Check length limit
	bufLen := b.buffer.Len()
	if bufLen > b.maxBufferLength {
		return true
	}

	// Check timeout
	elapsed := time.Since(b.bracketStartTime)
	if elapsed > b.bufferTimeout {
		return true
	}

	return false
}

// Flush returns any remaining buffered content, detokenizing if possible.
// Call this when the stream ends to ensure no content is lost.
func (b *SSEPIIBuffer) Flush(ctx context.Context) string {
	if !b.enabled || b.buffer.Len() == 0 {
		return ""
	}

	content := b.buffer.String()
	b.buffer.Reset()
	b.inBracket = false

	// Try to detokenize if it looks like a complete token
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
	b.inBracket = false
}
