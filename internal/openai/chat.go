package openai

import "time"

// ChatCompletionRequest captures the subset of OpenAI's request we currently support.
type ChatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []ChatMessage     `json:"messages"`
	Stream      bool              `json:"stream,omitempty"`
	Temperature *float64          `json:"temperature,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ChatMessage follows OpenAI's role/content schema (simplified to plain text for P0).
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse mirrors the OpenAI schema with a single choice for now.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   UsageBreakdown         `json:"usage"`
}

// ChatCompletionChoice contains the generated message.
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	FinishReason string      `json:"finish_reason"`
	Message      ChatMessage `json:"message"`
	Logprobs     interface{} `json:"logprobs"`
}

// UsageBreakdown provides token accounting placeholders.
type UsageBreakdown struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewCompletionResponse builds a response with the provided message.
func NewCompletionResponse(model string, message ChatMessage, usage UsageBreakdown) ChatCompletionResponse {
	return ChatCompletionResponse{
		ID:      "cmpl-loopback",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []ChatCompletionChoice{{
			Index:        0,
			FinishReason: "stop",
			Message:      message,
		}},
		Usage: usage,
	}
}
