package ratelimit

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Middleware wraps an HTTP handler with rate limiting.
type Middleware struct {
	limiter *Limiter
	enabled bool
	logger  *log.Logger
}

// NewMiddleware creates a new rate limiting middleware.
func NewMiddleware(limiter *Limiter, enabled bool, logger *log.Logger) *Middleware {
	return &Middleware{
		limiter: limiter,
		enabled: enabled,
		logger:  logger,
	}
}

// Wrap applies rate limiting to an HTTP handler.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract user ID and API key ID from context
		userID, apiKeyID := m.extractIDs(r)

		// Check rate limit
		allowed := m.limiter.Allow(r.Context(), userID, apiKeyID)

		if !allowed {
			// Add rate limit headers
			m.addRateLimitHeaders(w, userID, apiKeyID)

			// Log rate limit event (without sensitive api_key_id)
			if m.logger != nil {
				m.logger.Printf("rate limit exceeded: user_id=%d path=%s", userID, r.URL.Path)
			}

			// Return 429 Too Many Requests
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}

		// Add rate limit headers to successful responses
		m.addRateLimitHeaders(w, userID, apiKeyID)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// extractIDs extracts user ID and API key ID from the request context.
// These should be set by the auth middleware earlier in the chain.
func (m *Middleware) extractIDs(r *http.Request) (userID, apiKeyID int64) {
	// Try to get from context (set by auth middleware)
	if val := r.Context().Value("user_id"); val != nil {
		if id, ok := val.(int64); ok {
			userID = id
		}
	}

	if val := r.Context().Value("api_key_id"); val != nil {
		if id, ok := val.(int64); ok {
			apiKeyID = id
		}
	}

	return userID, apiKeyID
}

// addRateLimitHeaders adds standard rate limit headers to the response.
// See: https://datatracker.ietf.org/doc/html/draft-polli-ratelimit-headers
func (m *Middleware) addRateLimitHeaders(w http.ResponseWriter, userID, apiKeyID int64) {
	// Use the more restrictive limit for headers
	userRemaining := m.limiter.GetUserRemaining(userID)
	apiKeyRemaining := m.limiter.GetAPIKeyRemaining(apiKeyID)

	var remaining float64
	var limit float64
	var limitType string

	if userID > 0 && apiKeyID > 0 {
		// Both limits apply, use the more restrictive one
		if apiKeyRemaining < userRemaining {
			remaining = apiKeyRemaining
			limit = m.limiter.apiKeyCapacity
			limitType = "api_key"
		} else {
			remaining = userRemaining
			limit = m.limiter.userCapacity
			limitType = "user"
		}
	} else if userID > 0 {
		remaining = userRemaining
		limit = m.limiter.userCapacity
		limitType = "user"
	} else if apiKeyID > 0 {
		remaining = apiKeyRemaining
		limit = m.limiter.apiKeyCapacity
		limitType = "api_key"
	} else {
		// No limits apply
		return
	}

	// Standard rate limit headers
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%.0f", remaining))
	w.Header().Set("X-RateLimit-Type", limitType)

	// Add reset time (when bucket will be full again)
	if remaining < limit {
		resetTime := time.Now().Add(m.calculateResetDuration(remaining, limit, limitType))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
	}
}

// calculateResetDuration calculates when the bucket will be full.
func (m *Middleware) calculateResetDuration(remaining, limit float64, limitType string) time.Duration {
	tokensNeeded := limit - remaining

	var refillRate float64
	if limitType == "user" {
		refillRate = m.limiter.userRefillRate
	} else {
		refillRate = m.limiter.apiKeyRefillRate
	}

	secondsNeeded := tokensNeeded / refillRate
	return time.Duration(secondsNeeded * float64(time.Second))
}
