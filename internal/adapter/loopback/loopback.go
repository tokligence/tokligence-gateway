package loopback

import (
	"context"
	"errors"
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// Ensure LoopbackAdapter implements ChatAdapter.
var _ adapter.ChatAdapter = (*LoopbackAdapter)(nil)

// LoopbackAdapter echoes the last user message back to the caller.
type LoopbackAdapter struct{}

// New creates a LoopbackAdapter instance.
func New() *LoopbackAdapter {
	return &LoopbackAdapter{}
}

// CreateCompletion fabricates a deterministic completion for testing the gateway pipeline.
func (a *LoopbackAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if len(req.Messages) == 0 {
		return openai.ChatCompletionResponse{}, errors.New("no messages provided")
	}

	// find last user message; default to final message if none
	message := req.Messages[len(req.Messages)-1]
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.ToLower(req.Messages[i].Role) == "user" {
			message = req.Messages[i]
			break
		}
	}

	reply := openai.ChatMessage{
		Role:    "assistant",
		Content: "[loopback] " + strings.TrimSpace(message.Content),
	}

	usage := openai.UsageBreakdown{
		PromptTokens:     len(req.Messages) * 10,
		CompletionTokens: len(reply.Content) / 4,
		TotalTokens:      len(req.Messages)*10 + len(reply.Content)/4,
	}

	return openai.NewCompletionResponse(req.Model, reply, usage), nil
}
