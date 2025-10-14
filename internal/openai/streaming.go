package openai

// ChatCompletionChunk represents a chunk in SSE streaming response.
type ChatCompletionChunk struct {
	ID      string                     `json:"id"`
	Object  string                     `json:"object"`
	Created int64                      `json:"created"`
	Model   string                     `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
}

// ChatCompletionChunkChoice represents a choice in a streaming chunk.
type ChatCompletionChunkChoice struct {
	Index        int                `json:"index"`
	Delta        ChatMessageDelta   `json:"delta"`
	FinishReason *string            `json:"finish_reason"`
	Logprobs     interface{}        `json:"logprobs"`
}

// ChatMessageDelta represents the incremental content in a stream chunk.
type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// StreamChunk is a generic interface for processing stream chunks.
type StreamChunk interface {
	GetID() string
	GetModel() string
	GetDelta() ChatMessageDelta
	GetFinishReason() *string
}

// Ensure ChatCompletionChunk implements StreamChunk
var _ StreamChunk = (*ChatCompletionChunk)(nil)

func (c *ChatCompletionChunk) GetID() string {
	return c.ID
}

func (c *ChatCompletionChunk) GetModel() string {
	return c.Model
}

func (c *ChatCompletionChunk) GetDelta() ChatMessageDelta {
	if len(c.Choices) > 0 {
		return c.Choices[0].Delta
	}
	return ChatMessageDelta{}
}

func (c *ChatCompletionChunk) GetFinishReason() *string {
	if len(c.Choices) > 0 {
		return c.Choices[0].FinishReason
	}
	return nil
}
