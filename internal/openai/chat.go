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
// Supports multiple tool types:
// - "function": Standard function tools (nested structure with function field)
// - "url" or "mcp": MCP (Model Context Protocol) servers
// - "computer_*": Computer use tools (e.g., computer_20241022)
// - Others: Anthropic-hosted tools (passed through as-is)
type Tool struct {
	Type     string                 `json:"type"` // "function", "url", "mcp", "computer_*", etc.
	Function *ToolFunction          `json:"function,omitempty"`

	// MCP Server fields (for type=="url" or type=="mcp")
	URL                string                 `json:"url,omitempty"`
	Name               string                 `json:"name,omitempty"`
	ServerURL          string                 `json:"server_url,omitempty"`
	ServerLabel        string                 `json:"server_label,omitempty"`
	ToolConfiguration  map[string]interface{} `json:"tool_configuration,omitempty"`
	Headers            map[string]interface{} `json:"headers,omitempty"`
	AuthorizationToken string                 `json:"authorization_token,omitempty"`

	// Computer tool fields (for type=="computer_*")
	DisplayWidthPx  int `json:"display_width_px,omitempty"`
	DisplayHeightPx int `json:"display_height_px,omitempty"`
	DisplayNumber   int `json:"display_number,omitempty"`

	// Cache control for prompt caching
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
}

// ToolFunction describes a function available to the model.
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
}

// ChatMessage follows OpenAI's role/content schema.
// Content can be:
// - string: Simple text content (backward compatible)
// - []ContentBlock: Structured content with multiple blocks (text, image, files, etc.)
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"` // string or []ContentBlock
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"` // For tool response messages

	// Cache control for prompt caching
	CacheControl map[string]interface{} `json:"cache_control,omitempty"`
}

// ContentBlock represents a piece of content in a message.
// Supports multiple types for rich content:
// - text: Plain text content
// - image: Image content (image_url format)
// - container_upload: File/code uploads for code execution
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "image_url", "container_upload", etc.

	// For text blocks
	Text string `json:"text,omitempty"`

	// For image blocks
	ImageURL map[string]interface{} `json:"image_url,omitempty"`

	// For container_upload (code execution files)
	// Format depends on Anthropic's container_upload specification
	Source map[string]interface{} `json:"source,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`

	// Allow arbitrary fields for extensibility
	Extra map[string]interface{} `json:"-"` // Not serialized, for internal use
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
