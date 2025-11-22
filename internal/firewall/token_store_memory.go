package firewall

import (
	"context"
	"sync"
	"time"
)

// MemoryTokenStore implements TokenStore using in-memory maps
// This is the default implementation for single-instance deployments
type MemoryTokenStore struct {
	mu sync.RWMutex

	// Maps session ID -> token value -> PIIToken
	sessions map[string]map[string]*PIIToken
}

// NewMemoryTokenStore creates a new in-memory token store
func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		sessions: make(map[string]map[string]*PIIToken),
	}
}

// Store saves a token mapping
func (s *MemoryTokenStore) Store(ctx context.Context, sessionID string, token *PIIToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions[sessionID] == nil {
		s.sessions[sessionID] = make(map[string]*PIIToken)
	}

	s.sessions[sessionID][token.TokenValue] = token
	return nil
}

// Get retrieves the original value for a token
func (s *MemoryTokenStore) Get(ctx context.Context, sessionID, tokenValue string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sessionTokens, ok := s.sessions[sessionID]; ok {
		if token, found := sessionTokens[tokenValue]; found {
			return token.OriginalValue, true, nil
		}
	}

	return "", false, nil
}

// GetAll retrieves all tokens for a session
func (s *MemoryTokenStore) GetAll(ctx context.Context, sessionID string) (map[string]*PIIToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if sessionTokens, ok := s.sessions[sessionID]; ok {
		// Return a copy to avoid concurrent modification
		result := make(map[string]*PIIToken, len(sessionTokens))
		for k, v := range sessionTokens {
			result[k] = v
		}
		return result, nil
	}

	return make(map[string]*PIIToken), nil
}

// Delete removes a session's mappings
func (s *MemoryTokenStore) Delete(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

// CleanupExpired removes expired sessions
func (s *MemoryTokenStore) CleanupExpired(ctx context.Context, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for sessionID, tokens := range s.sessions {
		// Check if all tokens in this session are expired
		allExpired := true
		for _, token := range tokens {
			if now.Sub(token.DetectedAt) < ttl {
				allExpired = false
				break
			}
		}

		if allExpired && len(tokens) > 0 {
			delete(s.sessions, sessionID)
		}
	}

	return nil
}
