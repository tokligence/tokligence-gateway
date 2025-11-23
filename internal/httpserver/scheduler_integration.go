package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// extractPriorityFromHeader extracts priority tier from X-Priority header
// Returns default priority if header is missing or invalid
func (s *Server) extractPriorityFromHeader(r *http.Request) scheduler.PriorityTier {
	priorityStr := r.Header.Get("X-Priority")
	if priorityStr == "" {
		// No header, use default priority (P5 - Normal)
		return scheduler.PriorityNormal
	}

	priority, err := strconv.Atoi(priorityStr)
	if err != nil || priority < 0 || priority > 9 {
		// Invalid priority, use default
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("[WARN] Invalid X-Priority header: %q, using default P5", priorityStr)
		}
		return scheduler.PriorityNormal
	}

	return scheduler.PriorityTier(priority)
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

	// Extract priority from header
	priority := s.extractPriorityFromHeader(r)

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

	// Submit to scheduler (this is a type assertion, assuming SchedulerInstance is *scheduler.Scheduler)
	schedulerImpl, ok := s.schedulerInst.(*scheduler.Scheduler)
	if !ok {
		// Fallback: scheduler instance doesn't match expected type
		if s.logger != nil {
			s.logger.Printf("[WARN] Scheduler instance type assertion failed, bypassing scheduler")
		}
		return nil, nil
	}

	err := schedulerImpl.Submit(schedReq)
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

	schedulerImpl, ok := s.schedulerInst.(*scheduler.Scheduler)
	if !ok {
		return
	}

	waitTime := time.Since(schedReq.startTime)

	schedulerImpl.Release(schedReq.Request)

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
