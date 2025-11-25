package firewall

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// TokenStore is an interface for storing PII token mappings
// This allows different implementations: in-memory, Redis, etc.
type TokenStore interface {
	// Store saves a token mapping
	Store(ctx context.Context, sessionID string, token *PIIToken) error

	// Get retrieves the original value for a token
	Get(ctx context.Context, sessionID, tokenValue string) (string, bool, error)

	// GetAll retrieves all tokens for a session
	GetAll(ctx context.Context, sessionID string) (map[string]*PIIToken, error)

	// Delete removes a session's mappings
	Delete(ctx context.Context, sessionID string) error

	// CleanupExpired removes expired sessions
	CleanupExpired(ctx context.Context, ttl time.Duration) error
}

// PIIToken represents a tokenized (fake) replacement for detected PII
type PIIToken struct {
	OriginalValue string    // Real PII value (e.g., "john@example.com")
	TokenValue    string    // Fake token (e.g., "user_a7f3e2@demo.local")
	PIIType       string    // Type of PII (EMAIL, PHONE, SSN, etc.)
	DetectedAt    time.Time // When this was detected
}

// PIITokenizer manages the mapping between real PII and fake tokens
// It uses a pluggable TokenStore for persistence (in-memory, Redis, etc.)
type PIITokenizer struct {
	store TokenStore

	// TTL for session cleanup
	sessionTTL time.Duration

	// Random seed for token generation
	mu   sync.Mutex
	rand *rand.Rand
}

// NewPIITokenizer creates a new tokenizer with the given store
func NewPIITokenizer(store TokenStore) *PIITokenizer {
	return &PIITokenizer{
		store:      store,
		sessionTTL: 1 * time.Hour, // Default 1 hour TTL
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewPIITokenizerWithMemoryStore creates a tokenizer with in-memory storage
func NewPIITokenizerWithMemoryStore() *PIITokenizer {
	return NewPIITokenizer(NewMemoryTokenStore())
}

// StoreExternalToken stores a token mapping provided by an external service (e.g., Presidio)
// This is used when the external service generates the token, not the gateway
func (t *PIITokenizer) StoreExternalToken(ctx context.Context, sessionID, piiType, originalValue, tokenValue string) error {
	piiToken := &PIIToken{
		OriginalValue: originalValue,
		TokenValue:    tokenValue,
		PIIType:       piiType,
		DetectedAt:    time.Now(),
	}
	return t.store.Store(ctx, sessionID, piiToken)
}

// Tokenize replaces PII with a fake token and stores the mapping
func (t *PIITokenizer) Tokenize(ctx context.Context, sessionID, piiType, originalValue string) (string, error) {
	// Check if we already tokenized this value
	allTokens, err := t.store.GetAll(ctx, sessionID)
	if err != nil {
		// Log error but continue - generate new token
	} else {
		// Look for existing token for this original value
		for _, existing := range allTokens {
			if existing.OriginalValue == originalValue && existing.PIIType == piiType {
				return existing.TokenValue, nil
			}
		}
	}

	// Generate a new token
	t.mu.Lock()
	token := t.generateToken(piiType, originalValue)
	t.mu.Unlock()

	// Store the mapping
	piiToken := &PIIToken{
		OriginalValue: originalValue,
		TokenValue:    token,
		PIIType:       piiType,
		DetectedAt:    time.Now(),
	}

	if err := t.store.Store(ctx, sessionID, piiToken); err != nil {
		return token, err // Return token even if storage fails
	}

	return token, nil
}

// Detokenize replaces fake tokens back to original PII values
func (t *PIITokenizer) Detokenize(ctx context.Context, sessionID, tokenValue string) (string, bool, error) {
	original, found, err := t.store.Get(ctx, sessionID, tokenValue)
	if err != nil {
		return tokenValue, false, err
	}
	return original, found, nil
}

// DetokenizeAll replaces all fake tokens in the text with original values
func (t *PIITokenizer) DetokenizeAll(ctx context.Context, sessionID, text string) (string, error) {
	allTokens, err := t.store.GetAll(ctx, sessionID)
	if err != nil {
		return text, err
	}

	result := text
	for _, token := range allTokens {
		// Replace token with original value
		result = replaceAll(result, token.TokenValue, token.OriginalValue)
	}

	return result, nil
}

// GetMappings returns all mappings for a session (for debugging)
func (t *PIITokenizer) GetMappings(ctx context.Context, sessionID string) (map[string]*PIIToken, error) {
	return t.store.GetAll(ctx, sessionID)
}

// CleanupSession removes all mappings for a session
func (t *PIITokenizer) CleanupSession(ctx context.Context, sessionID string) error {
	return t.store.Delete(ctx, sessionID)
}

// CleanupExpired removes sessions older than TTL
func (t *PIITokenizer) CleanupExpired(ctx context.Context) error {
	return t.store.CleanupExpired(ctx, t.sessionTTL)
}

// generateToken creates a fake but realistic-looking token for the given PII type
func (t *PIITokenizer) generateToken(piiType, originalValue string) string {
	piiType = strings.ToUpper(piiType)

	// Create a deterministic but unique hash based on value + timestamp + random
	hash := md5.Sum([]byte(fmt.Sprintf("%s:%d:%d", originalValue, time.Now().UnixNano(), t.rand.Int63())))
	hashStr := hex.EncodeToString(hash[:])[:7] // Take first 7 chars

	// Standardize all tokens to bracket form for SSE-friendly matching: [TYPE_hash]
	return fmt.Sprintf("[%s_%s]", piiType, hashStr)
}

// Helper function to replace all occurrences
func replaceAll(s, old, new string) string {
	if old == "" {
		return s
	}
	// Use a simple approach - in production you might want to use strings.ReplaceAll
	// or a more sophisticated token replacement algorithm
	result := s
	for {
		idx := indexOf(result, old)
		if idx == -1 {
			break
		}
		result = result[:idx] + new + result[idx+len(old):]
	}
	return result
}

// Helper function to find substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
