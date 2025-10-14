package fallback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// FallbackAdapter wraps multiple adapters and provides automatic fallback and retry logic.
type FallbackAdapter struct {
	adapters   []adapter.ChatAdapter
	retryCount int
	retryDelay time.Duration
}

// Config holds configuration for the FallbackAdapter.
type Config struct {
	Adapters   []adapter.ChatAdapter
	RetryCount int           // number of retries per adapter (default: 2)
	RetryDelay time.Duration // delay between retries (default: 1s)
}

// New creates a new FallbackAdapter.
func New(cfg Config) (*FallbackAdapter, error) {
	if len(cfg.Adapters) == 0 {
		return nil, errors.New("fallback: at least one adapter required")
	}

	retryCount := cfg.RetryCount
	if retryCount < 0 {
		retryCount = 0
	}
	if retryCount == 0 {
		retryCount = 2 // default
	}

	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = 1 * time.Second
	}

	return &FallbackAdapter{
		adapters:   cfg.Adapters,
		retryCount: retryCount,
		retryDelay: retryDelay,
	}, nil
}

// CreateCompletion attempts to create a completion using the primary adapter,
// falling back to subsequent adapters on failure.
func (f *FallbackAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	var lastErr error
	var allErrors []error

	for adapterIdx, adapter := range f.adapters {
		// Try this adapter with retries
		for attempt := 0; attempt <= f.retryCount; attempt++ {
			// Check context cancellation
			select {
			case <-ctx.Done():
				return openai.ChatCompletionResponse{}, ctx.Err()
			default:
			}

			// Attempt completion
			resp, err := adapter.CreateCompletion(ctx, req)
			if err == nil {
				// Success!
				return resp, nil
			}

			lastErr = err
			allErrors = append(allErrors, fmt.Errorf("adapter[%d] attempt[%d]: %w", adapterIdx, attempt, err))

			// Don't retry on the last attempt of the last adapter
			isLastAdapter := adapterIdx == len(f.adapters)-1
			isLastAttempt := attempt == f.retryCount
			if isLastAdapter && isLastAttempt {
				break
			}

			// Check if error is retryable
			if !isRetryableError(err) {
				// Non-retryable error, move to next adapter
				break
			}

			// Wait before retry (only if not the last attempt)
			if attempt < f.retryCount {
				select {
				case <-ctx.Done():
					return openai.ChatCompletionResponse{}, ctx.Err()
				case <-time.After(f.retryDelay):
				}
			}
		}
	}

	// All adapters failed
	return openai.ChatCompletionResponse{}, fmt.Errorf("fallback: all adapters failed: %w (attempts: %d)", lastErr, len(allErrors))
}

// isRetryableError determines if an error should trigger a retry.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network errors are retryable
	if containsAny(errStr, []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"temporary failure",
		"context deadline exceeded",
	}) {
		return true
	}

	// Rate limits are retryable (but will use next adapter after retries)
	if containsAny(errStr, []string{
		"rate limit",
		"429",
		"too many requests",
	}) {
		return true
	}

	// Server errors (5xx) are retryable
	if containsAny(errStr, []string{
		"500",
		"502",
		"503",
		"504",
		"internal server error",
		"bad gateway",
		"service unavailable",
		"gateway timeout",
	}) {
		return true
	}

	// Client errors (4xx except 429) are NOT retryable
	if containsAny(errStr, []string{
		"400",
		"401",
		"403",
		"404",
		"invalid api key",
		"unauthorized",
		"forbidden",
		"not found",
		"bad request",
	}) {
		return false
	}

	// Default to non-retryable
	return false
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	// Simple case-insensitive contains check
	sLower := toLower(s)
	substrLower := toLower(substr)
	return stringContains(sLower, substrLower)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + ('a' - 'A')
		}
		result[i] = c
	}
	return string(result)
}

func stringContains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
