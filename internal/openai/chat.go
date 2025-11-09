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
    Tools       []Tool            `json:"tools,omitempty"`
    ToolChoice  interface{}       `json:"tool_choice,omitempty"` // "auto", "none", "required", or specific tool
    MaxTokens   *int              `json:"max_tokens,omitempty"`
    // Optional: pass-through response_format for JSON mode and schemas
    ResponseFormat map[string]interface{} `json:"response_format,omitempty"`
}

// Tool represents a function that the model can call (Chat Completions API format).
// Uses nested structure with function field.
type Tool struct {
	Type     string       `json:"type"` // always "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction describes a function available to the model.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ChatMessage follows OpenAI's role/content schema (simplified to plain text for P0).
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"` // For tool response messages
}

// ToolCall represents a function call made by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // always "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the function name and arguments.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
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
