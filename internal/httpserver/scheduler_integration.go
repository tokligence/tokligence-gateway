package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// extractPriorityFromRequest extracts priority tier from request
// Priority sources (in order of precedence):
// 1. X-Priority header (if set)
// 2. API key mapping (if APIKeyMapper enabled and key matches pattern)
// 3. Default priority (P5 - Normal)
func (s *Server) extractPriorityFromRequest(r *http.Request) scheduler.PriorityTier {
	// Priority 1: Check X-Priority header (explicit override)
	priorityStr := r.Header.Get("X-Priority")
	if priorityStr != "" {
		priority, err := strconv.Atoi(priorityStr)
		if err == nil && priority >= 0 && priority <= 9 {
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("[DEBUG] Using priority from X-Priority header: P%d", priority)
			}
			return scheduler.PriorityTier(priority)
		}
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("[WARN] Invalid X-Priority header: %q, checking API key mapping", priorityStr)
		}
	}

	// Priority 2: Check API key mapping (database-driven)
	if s.apiKeyMapper != nil && s.apiKeyMapper.IsEnabled() {
		apiKey := s.extractAPIKey(r)
		if apiKey != "" {
			priority := s.apiKeyMapper.GetPriority(apiKey)
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("[DEBUG] Using priority from API key mapping: P%d (key=%s)", priority, maskAPIKey(apiKey))
			}
			return priority
		}
	}

	// Priority 3: Default priority
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("[DEBUG] Using default priority: P%d", scheduler.PriorityNormal)
	}
	return scheduler.PriorityNormal
}

// extractAPIKey extracts API key from Authorization header
func (s *Server) extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Strip "Bearer " prefix
	const prefix = "Bearer "
	if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
		return auth[len(prefix):]
	}

	return auth
}

// maskAPIKey masks API key for logging (show first 8 chars, mask rest)
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:8] + "***"
}

// estimateTokensFromRequest estimates token count from request
// This is a rough estimate for scheduler capacity planning
func (s *Server) estimateTokensFromRequest(req openai.ChatCompletionRequest) int64 {
	// Rough estimation: average ~4 characters per token
	totalChars := 0

	// Count message content
	for _, msg := range req.Messages {
		switch content := msg.Content.(type) {
		case string:
			totalChars += len(content)
		case []openai.ContentBlock:
			for _, block := range content {
				if block.Type == "text" {
					totalChars += len(block.Text)
				}
			}
		}
	}

	// Add max_tokens for output estimation
	estimatedTokens := int64(totalChars / 4) // Input tokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		estimatedTokens += int64(*req.MaxTokens) // Expected output tokens
	} else {
		// Default max_tokens estimate if not specified
		estimatedTokens += 1000
	}

	return estimatedTokens
}

// schedRequest represents a scheduler request wrapper
type schedRequest struct {
	*scheduler.Request
	startTime time.Time
}

// submitToScheduler submits a request to the priority scheduler
// Returns schedRequest if accepted, or error if rejected
func (s *Server) submitToScheduler(
	r *http.Request,
	model string,
	chatReq openai.ChatCompletionRequest,
	accountID string,
) (*schedRequest, error) {
	if !s.schedulerEnabled || s.schedulerInst == nil {
		// Scheduler not enabled, return nil to indicate passthrough
		return nil, nil
	}

	// Extract priority from request (header, API key mapping, or default)
	priority := s.extractPriorityFromRequest(r)

	// Estimate tokens
	estimatedTokens := s.estimateTokensFromRequest(chatReq)

	// Create scheduler request
	schedReq := &scheduler.Request{
		ID:              fmt.Sprintf("req-%s", time.Now().Format("20060102-150405.000000")),
		Priority:        priority,
		EstimatedTokens: estimatedTokens,
		AccountID:       accountID,
		Model:           model,
		ResultChan:      make(chan *scheduler.ScheduleResult, 2),
	}

	err := s.schedulerInst.Submit(schedReq)
	if err != nil {
		// Submission rejected immediately (e.g., queue full, context too large)
		return nil, fmt.Errorf("scheduler rejected request: %w", err)
	}

	// Wait for scheduler decision
	select {
	case result := <-schedReq.ResultChan:
		if !result.Accepted {
			// Request rejected after queueing (e.g., timeout, expired)
			return nil, fmt.Errorf("scheduler rejected: %s", result.Reason)
		}

		// Request accepted - log queue info if queued
		if result.Reason == "queued" {
			if s.logger != nil {
				s.logger.Printf("[INFO] Request %s queued at position %d (priority=P%d)",
					schedReq.ID, result.QueuePos, priority)
			}
			// Set response headers to inform client about queueing
			if w, ok := r.Context().Value("responseWriter").(http.ResponseWriter); ok {
				w.Header().Set("X-Tokligence-Queue-Position", fmt.Sprintf("%d", result.QueuePos))
			}
		} else {
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("[DEBUG] Request %s scheduled immediately (priority=P%d)",
					schedReq.ID, priority)
			}
		}

		return &schedRequest{
			Request:   schedReq,
			startTime: time.Now(),
		}, nil

	case <-time.After(35 * time.Second):
		// Timeout waiting for scheduler (shouldn't happen with proper queue timeout)
		return nil, fmt.Errorf("scheduler timeout: no response after 35s")
	}
}

// releaseScheduler releases capacity after request completion
func (s *Server) releaseScheduler(schedReq *schedRequest) {
	if schedReq == nil || !s.schedulerEnabled || s.schedulerInst == nil {
		return
	}

	// Release any per-account override concurrency reservations
	if s.quotaManager != nil && schedReq.Request.AccountID != "" {
		s.quotaManager.ReleaseOverride(schedReq.Request.AccountID)
	}

	waitTime := time.Since(schedReq.startTime)

	s.schedulerInst.Release(schedReq.Request)

	if s.isDebug() && s.logger != nil {
		s.logger.Printf("[DEBUG] Request %s released after %v (priority=P%d)",
			schedReq.Request.ID, waitTime, schedReq.Request.Priority)
	}

	// Set response header with wait time
	// Note: This would need to be set before response is written
	// For now, just log it
}

// respondSchedulerError returns appropriate error for scheduler rejections
func (s *Server) respondSchedulerError(w http.ResponseWriter, err error) {
	// Default to 503 Service Unavailable for scheduler errors
	statusCode := http.StatusServiceUnavailable

	// Check for specific error types
	errMsg := err.Error()
	switch {
	case contains(errMsg, "queue full"):
		statusCode = http.StatusServiceUnavailable
	case contains(errMsg, "timeout"):
		statusCode = http.StatusServiceUnavailable
	case contains(errMsg, "capacity exceeded"):
		statusCode = http.StatusTooManyRequests
	case contains(errMsg, "context"):
		statusCode = http.StatusRequestEntityTooLarge
	}

	s.respondError(w, statusCode, err)
}

// contains checks if string contains substring (helper)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// checkQuotaBeforeRequest checks quota availability before processing request
// Returns nil if quota is available, or error if quota is exceeded
func (s *Server) checkQuotaBeforeRequest(
	ctx context.Context,
	r *http.Request,
	chatReq openai.ChatCompletionRequest,
	accountID string,
) error {
	if s.quotaManager == nil || !s.quotaManager.IsEnabled() {
		// Quota management not enabled, allow request
		return nil
	}

	// Extract metadata from request
	apiKey := s.extractAPIKey(r)
	model := chatReq.Model
	estimatedTokens := s.estimateTokensFromRequest(chatReq)

	// Build quota check request
	quotaReq := scheduler.QuotaCheckRequest{
		AccountID:       accountID,
		TeamID:          r.Header.Get("X-Team-ID"),     // Optional team ID from header
		Environment:     r.Header.Get("X-Environment"), // Optional environment from header
		EstimatedTokens: estimatedTokens,
		Model:           model,
		RequestID:       fmt.Sprintf("req-%s", time.Now().Format("20060102-150405.000000")),
	}

	// Check and reserve quota
	result, err := s.quotaManager.CheckAndReserve(ctx, quotaReq)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("[ERROR] Quota check failed: %v", err)
		}
		return fmt.Errorf("quota check failed: %w", err)
	}

	if !result.Allowed {
		// Quota exceeded
		if s.logger != nil {
			s.logger.Printf("[WARN] Quota exceeded: account=%s code=%s message=%s api_key=%s",
				accountID, result.RejectionCode, result.Message, maskAPIKey(apiKey))
		}
		return fmt.Errorf("quota exceeded: %s", result.Message)
	}

	// Quota available - store estimated tokens in context for later commit
	// (The actual commit will happen after we know the real token count from the response)
	// For now, reservation is done in-memory and will be committed later

	if s.isDebug() && s.logger != nil {
		s.logger.Printf("[DEBUG] Quota check passed: account=%s estimated_tokens=%d quotas_checked=%d",
			accountID, estimatedTokens, len(result.QuotasChecked))
	}

	return nil
}

// commitQuotaUsage commits actual token usage after request completion
func (s *Server) commitQuotaUsage(
	ctx context.Context,
	r *http.Request,
	accountID string,
	actualTokens, estimatedTokens int64,
) {
	if s.quotaManager == nil || !s.quotaManager.IsEnabled() {
		return
	}

	teamID := r.Header.Get("X-Team-ID")
	environment := r.Header.Get("X-Environment")

	err := s.quotaManager.CommitUsage(ctx, accountID, teamID, environment, actualTokens, estimatedTokens)
	if err != nil && s.logger != nil {
		s.logger.Printf("[ERROR] Failed to commit quota usage: %v", err)
	}
}
