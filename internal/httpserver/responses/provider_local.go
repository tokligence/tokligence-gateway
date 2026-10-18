package responses

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// LocalProvider implements StreamProvider for self-hosted LLM with scheduling
type LocalProvider struct {
	baseProvider StreamProvider // Underlying provider (AnthropicStreamProvider, etc.)
	scheduler    *scheduler.Scheduler
}

// NewLocalProvider creates a new LocalProvider with scheduler
func NewLocalProvider(baseProvider StreamProvider, sched *scheduler.Scheduler) *LocalProvider {
	log.Printf("[INFO] LocalProvider: Initialized with scheduler")
	return &LocalProvider{
		baseProvider: baseProvider,
		scheduler:    sched,
	}
}

// Stream implements StreamProvider interface with priority scheduling
func (lp *LocalProvider) Stream(ctx context.Context, conv Conversation) (StreamInit, error) {
	// Extract request metadata from conversation
	requestID := uuid.New().String()

	// Determine priority from conversation metadata
	priority := lp.extractPriority(conv)

	// Estimate tokens for capacity planning
	estimatedTokens := lp.estimateTokens(conv)

	// Extract account and model info
	accountID := lp.extractAccountID(conv)
	model := lp.extractModel(conv)

	log.Printf("[INFO] LocalProvider.Stream: Request %s (priority=P%d, tokens=%d, account=%s, model=%s)",
		requestID, priority, estimatedTokens, accountID, model)

	// Create scheduler request
	schedReq := &scheduler.Request{
		ID:              requestID,
		Priority:        priority,
		EstimatedTokens: estimatedTokens,
		Environment:     "production", // TODO: Extract from context
		AccountID:       accountID,
		Model:           model,
		ResultChan:      make(chan *scheduler.ScheduleResult, 1),
	}

	// Submit to scheduler
	err := lp.scheduler.Submit(schedReq)
	if err != nil {
		log.Printf("[ERROR] LocalProvider.Stream: Failed to submit request %s to scheduler: %v", requestID, err)
		return StreamInit{}, fmt.Errorf("scheduler submit failed: %w", err)
	}

	// Wait for scheduling result
	select {
	case result := <-schedReq.ResultChan:
		if !result.Accepted {
			log.Printf("[WARN] LocalProvider.Stream: Request %s rejected by scheduler: %s", requestID, result.Reason)
			return StreamInit{}, fmt.Errorf("scheduler rejected: %s", result.Reason)
		}

		if result.Reason == "queued" {
			log.Printf("[INFO] LocalProvider.Stream: Request %s queued at position %d, waiting for capacity",
				requestID, result.QueuePos)

			// Wait for second notification when actually scheduled
			select {
			case result2 := <-schedReq.ResultChan:
				if !result2.Accepted {
					return StreamInit{}, fmt.Errorf("scheduler rejected after queue: %s", result2.Reason)
				}
				log.Printf("[INFO] LocalProvider.Stream: Request %s scheduled after waiting", requestID)
			case <-ctx.Done():
				log.Printf("[WARN] LocalProvider.Stream: Request %s cancelled while queued", requestID)
				return StreamInit{}, ctx.Err()
			case <-time.After(60 * time.Second):
				log.Printf("[ERROR] LocalProvider.Stream: Request %s timeout while queued", requestID)
				return StreamInit{}, fmt.Errorf("scheduler timeout")
			}
		} else {
			log.Printf("[INFO] LocalProvider.Stream: Request %s scheduled immediately (%s)", requestID, result.Reason)
		}

	case <-ctx.Done():
		log.Printf("[WARN] LocalProvider.Stream: Request %s cancelled before scheduling", requestID)
		return StreamInit{}, ctx.Err()
	}

	// Now execute via underlying provider
	log.Printf("[INFO] LocalProvider.Stream: Executing request %s via base provider", requestID)

	streamInit, err := lp.baseProvider.Stream(ctx, conv)
	if err != nil {
		log.Printf("[ERROR] LocalProvider.Stream: Base provider stream failed for request %s: %v", requestID, err)
		lp.scheduler.Release(schedReq)
		return StreamInit{}, err
	}

	// Wrap cleanup to release scheduler capacity
	originalCleanup := streamInit.Cleanup
	streamInit.Cleanup = func() {
		log.Printf("[INFO] LocalProvider.Stream: Request %s completed, releasing capacity", requestID)
		lp.scheduler.Release(schedReq)
		if originalCleanup != nil {
			originalCleanup()
		}
	}

	log.Printf("[INFO] LocalProvider.Stream: ✓ Request %s streaming started", requestID)
	return streamInit, nil
}

// extractPriority extracts priority from conversation metadata
func (lp *LocalProvider) extractPriority(conv Conversation) scheduler.PriorityTier {
	// TODO: Extract from conversation metadata (e.g., X-Priority header)
	// For now, return default
	return scheduler.PriorityNormal
}

// estimateTokens estimates token count from conversation
func (lp *LocalProvider) estimateTokens(conv Conversation) int64 {
	// Simple estimation: ~4 chars per token
	// TODO: Use proper tokenizer
	totalChars := 0

	// Count messages from Chat request
	for _, msg := range conv.Chat.Messages {
		// Estimate from content (string or array)
		contentStr := fmt.Sprintf("%v", msg.Content)
		totalChars += len(contentStr)
	}

	// Count tools if present
	for _, tool := range conv.Chat.Tools {
		toolStr := fmt.Sprintf("%v", tool)
		totalChars += len(toolStr)
	}

	estimatedTokens := int64(totalChars / 4)
	if estimatedTokens == 0 {
		estimatedTokens = 100 // Minimum estimate
	}

	log.Printf("[DEBUG] LocalProvider.estimateTokens: Estimated %d tokens from %d chars", estimatedTokens, totalChars)
	return estimatedTokens
}

// extractAccountID extracts account ID from conversation
func (lp *LocalProvider) extractAccountID(conv Conversation) string {
	// TODO: Extract from conversation metadata
	return "default"
}

// extractModel extracts model name from conversation
func (lp *LocalProvider) extractModel(conv Conversation) string {
	// Extract from Chat request
	if conv.Chat.Model != "" {
		return conv.Chat.Model
	}
	return "unknown"
}
