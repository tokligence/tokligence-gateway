package firewall

import (
	"context"
	"strings"
	"testing"
)

func TestPIITokenizer_Tokenize(t *testing.T) {
	ctx := context.Background()
	tokenizer := NewPIITokenizerWithMemoryStore()

	tests := []struct {
		name          string
		sessionID     string
		piiType       string
		originalValue string
		wantPrefix    string // Token should start with this
	}{
		{
			name:          "Email tokenization",
			sessionID:     "session-1",
			piiType:       "EMAIL",
			originalValue: "john.doe@example.com",
			wantPrefix:    "user_",
		},
		{
			name:          "Phone tokenization",
			sessionID:     "session-2",
			piiType:       "PHONE",
			originalValue: "555-123-4567",
			wantPrefix:    "+1-555-",
		},
		{
			name:          "SSN tokenization",
			sessionID:     "session-3",
			piiType:       "SSN",
			originalValue: "123-45-6789",
			wantPrefix:    "XXX-XX-",
		},
		{
			name:          "Credit card tokenization",
			sessionID:     "session-4",
			piiType:       "CREDIT_CARD",
			originalValue: "4532-1234-5678-9012",
			wantPrefix:    "XXXX-XXXX-XXXX-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := tokenizer.Tokenize(ctx, tt.sessionID, tt.piiType, tt.originalValue)
			if err != nil {
				t.Fatalf("Tokenize() error = %v", err)
			}

			if token == tt.originalValue {
				t.Errorf("Token should not equal original value")
			}

			if !strings.HasPrefix(token, tt.wantPrefix) {
				t.Errorf("Token = %v, want prefix %v", token, tt.wantPrefix)
			}

			// Verify we can retrieve the original value
			original, found, err := tokenizer.store.Get(ctx, tt.sessionID, token)
			if err != nil {
				t.Fatalf("Get() error = %v", err)
			}
			if !found {
				t.Errorf("Token not found in store")
			}
			if original != tt.originalValue {
				t.Errorf("Retrieved value = %v, want %v", original, tt.originalValue)
			}
		})
	}
}

func TestPIITokenizer_DetokenizeAll(t *testing.T) {
	ctx := context.Background()
	tokenizer := NewPIITokenizerWithMemoryStore()
	sessionID := "test-session"

	// Tokenize some PII
	emailToken, _ := tokenizer.Tokenize(ctx, sessionID, "EMAIL", "john@example.com")
	phoneToken, _ := tokenizer.Tokenize(ctx, sessionID, "PHONE", "555-123-4567")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "Detokenize email",
			input: "Contact me at " + emailToken,
			want:  "Contact me at john@example.com",
		},
		{
			name:  "Detokenize phone",
			input: "Call " + phoneToken + " for help",
			want:  "Call 555-123-4567 for help",
		},
		{
			name:  "Detokenize multiple",
			input: "Email: " + emailToken + ", Phone: " + phoneToken,
			want:  "Email: john@example.com, Phone: 555-123-4567",
		},
		{
			name:  "No tokens to detokenize",
			input: "This has no tokens",
			want:  "This has no tokens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenizer.DetokenizeAll(ctx, sessionID, tt.input)
			if err != nil {
				t.Fatalf("DetokenizeAll() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("DetokenizeAll() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPIITokenizer_SessionIsolation(t *testing.T) {
	ctx := context.Background()
	tokenizer := NewPIITokenizerWithMemoryStore()

	email := "john@example.com"

	// Tokenize same email in different sessions
	token1, _ := tokenizer.Tokenize(ctx, "session-1", "EMAIL", email)
	token2, _ := tokenizer.Tokenize(ctx, "session-2", "EMAIL", email)

	// Tokens should be different for different sessions
	if token1 == token2 {
		t.Errorf("Same PII in different sessions should get different tokens")
	}

	// Each session should only see its own token
	text1 := "Email is " + token1
	result1, _ := tokenizer.DetokenizeAll(ctx, "session-1", text1)
	if result1 != "Email is "+email {
		t.Errorf("Session 1 should detokenize its own token")
	}

	// Session 2's token shouldn't be detokenized in session 1
	text2 := "Email is " + token2
	result2, _ := tokenizer.DetokenizeAll(ctx, "session-1", text2)
	if result2 != text2 {
		t.Errorf("Session 1 should not detokenize session 2's token")
	}
}

func TestPIITokenizer_UniquenessWithinSession(t *testing.T) {
	ctx := context.Background()
	tokenizer := NewPIITokenizerWithMemoryStore()
	sessionID := "test-session"

	// Tokenize different emails
	token1, _ := tokenizer.Tokenize(ctx, sessionID, "EMAIL", "alice@example.com")
	token2, _ := tokenizer.Tokenize(ctx, sessionID, "EMAIL", "bob@example.com")

	// Tokens should be different
	if token1 == token2 {
		t.Errorf("Different PII values should get different tokens")
	}

	// Both should maintain format
	if !strings.HasPrefix(token1, "user_") || !strings.Contains(token1, "@redacted.local") {
		t.Errorf("Token 1 has wrong format: %v", token1)
	}
	if !strings.HasPrefix(token2, "user_") || !strings.Contains(token2, "@redacted.local") {
		t.Errorf("Token 2 has wrong format: %v", token2)
	}

	// Detokenize should restore both correctly
	text := "Emails: " + token1 + " and " + token2
	result, _ := tokenizer.DetokenizeAll(ctx, sessionID, text)
	expected := "Emails: alice@example.com and bob@example.com"
	if result != expected {
		t.Errorf("DetokenizeAll() = %v, want %v", result, expected)
	}
}

func TestMemoryTokenStore_CleanupExpired(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryTokenStore()
	sessionID := "test-session"

	// Store a token
	token := &PIIToken{
		OriginalValue: "john@example.com",
		TokenValue:    "user_abc123@redacted.local",
		PIIType:       "EMAIL",
	}
	store.Store(ctx, sessionID, token)

	// Verify it's stored
	_, found, _ := store.Get(ctx, sessionID, token.TokenValue)
	if !found {
		t.Errorf("Token should be in store")
	}

	// Cleanup expired (with 0 TTL, should remove everything)
	store.CleanupExpired(ctx, 0)

	// Verify it's removed
	_, found, _ = store.Get(ctx, sessionID, token.TokenValue)
	if found {
		t.Errorf("Token should be removed after cleanup")
	}
}
