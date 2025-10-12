package adapter

import (
	"context"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// ChatAdapter converts OpenAI compatible chat requests into provider specific responses.
type ChatAdapter interface {
	CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}
