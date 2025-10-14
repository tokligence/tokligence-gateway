package adapter

import (
	"context"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// ChatAdapter converts OpenAI compatible chat requests into provider specific responses.
type ChatAdapter interface {
	CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// StreamingChatAdapter extends ChatAdapter with streaming support.
type StreamingChatAdapter interface {
	ChatAdapter
	CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan StreamEvent, error)
}

// StreamEvent represents a single event in a streaming response.
type StreamEvent struct {
	Chunk *openai.ChatCompletionChunk
	Error error
}

// IsError checks if this event contains an error.
func (e StreamEvent) IsError() bool {
	return e.Error != nil
}

// IsDone checks if this is a stream completion event (no chunk, no error).
func (e StreamEvent) IsDone() bool {
	return e.Chunk == nil && e.Error == nil
}
