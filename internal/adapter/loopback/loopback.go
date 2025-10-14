package loopback

import (
	"context"
	"errors"
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// Ensure LoopbackAdapter implements ChatAdapter and EmbeddingAdapter.
var _ adapter.ChatAdapter = (*LoopbackAdapter)(nil)
var _ adapter.EmbeddingAdapter = (*LoopbackAdapter)(nil)

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

// CreateEmbedding generates deterministic embedding vectors for testing.
func (a *LoopbackAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	if req.Input == nil {
		return openai.EmbeddingResponse{}, errors.New("input required")
	}

	// Convert input to string slice
	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []string:
		inputs = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				inputs = append(inputs, s)
			}
		}
	default:
		return openai.EmbeddingResponse{}, errors.New("invalid input type")
	}

	if len(inputs) == 0 {
		return openai.EmbeddingResponse{}, errors.New("input required")
	}

	// Generate deterministic embeddings based on input length
	// Use a fixed dimension (1536 for text-embedding-ada-002)
	dimension := 1536
	if req.Dimensions != nil && *req.Dimensions > 0 {
		dimension = *req.Dimensions
	}

	embeddings := make([][]float64, len(inputs))
	for i, input := range inputs {
		embedding := make([]float64, dimension)
		// Create deterministic but varying values based on input content
		seed := float64(len(input))
		for j := 0; j < dimension; j++ {
			// Simple deterministic pattern for testing
			embedding[j] = (seed + float64(j)) / float64(dimension*100)
		}
		embeddings[i] = embedding
	}

	// Calculate token usage (approximate 4 chars per token)
	totalChars := 0
	for _, input := range inputs {
		totalChars += len(input)
	}
	promptTokens := totalChars/4 + 1

	return openai.NewEmbeddingResponse(req.Model, embeddings, promptTokens), nil
}
