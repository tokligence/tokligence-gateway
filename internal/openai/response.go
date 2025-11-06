package openai

import (
	"fmt"
	"strings"
)

// ResponseRequest represents OpenAI's Response API request format.
// Response API uses a different structure than Chat Completions API.
// https://platform.openai.com/docs/api-reference/responses/create
type ResponseRequest struct {
	Model           string           `json:"model"`
	Input           interface{}      `json:"input,omitempty"`           // Can be messages array
	Messages        []ChatMessage    `json:"messages,omitempty"`        // Alternative to Input
	Instructions    string           `json:"instructions,omitempty"`    // System-level instructions
	Temperature     *float64         `json:"temperature,omitempty"`
	TopP            *float64         `json:"top_p,omitempty"`
	MaxOutputTokens *int             `json:"max_output_tokens,omitempty"` // Response API naming
	MaxTokens       *int             `json:"max_tokens,omitempty"`        // Chat Completions compatibility
	Stream          bool             `json:"stream,omitempty"`
	Tools           []ResponseTool   `json:"tools,omitempty"`      // Response API tool format (flat)
	ToolChoice      interface{}      `json:"tool_choice,omitempty"` // "auto", "none", true, false, or specific tool
	ResponseFormat  struct {
		Type       string                 `json:"type"`             // "text", "json_object", "json_schema"
		JsonSchema map[string]interface{} `json:"json_schema,omitempty"`
	} `json:"response_format,omitempty"`
}

// ResponseTool represents a tool in Response API format (flat structure).
// Unlike Chat Completions API which uses nested {type, function: {name, ...}},
// Response API uses flat structure {type, name, description, parameters}.
type ResponseTool struct {
	Type        string                 `json:"type"` // always "function"
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToTool converts ResponseTool (flat format) to Tool (nested format) for Chat Completions.
func (rt ResponseTool) ToTool() Tool {
	return Tool{
		Type: rt.Type,
		Function: ToolFunction{
			Name:        rt.Name,
			Description: rt.Description,
			Parameters:  rt.Parameters,
		},
	}
}

// ToChatCompletionRequest converts Response API request to Chat Completions request.
func (rr ResponseRequest) ToChatCompletionRequest() ChatCompletionRequest {
	creq := ChatCompletionRequest{
		Model:       rr.Model,
		Stream:      rr.Stream,
		Temperature: rr.Temperature,
		TopP:        rr.TopP,
	}

	// Handle Input or Messages
	fmt.Printf("[DEBUG] ToChatCompletionRequest: Input type=%T, Input value=%+v\n", rr.Input, rr.Input)
	fmt.Printf("[DEBUG] ToChatCompletionRequest: Messages count=%d\n", len(rr.Messages))
	if rr.Input != nil {
		switch v := rr.Input.(type) {
		case []interface{}:
			// Array of message objects
			fmt.Printf("[DEBUG] Input is []interface{} with %d elements\n", len(v))
			creq.Messages = make([]ChatMessage, 0, len(v))
			for i, m := range v {
				fmt.Printf("[DEBUG] Input[%d]: type=%T, value=%+v\n", i, m, m)
				if msgMap, ok := m.(map[string]interface{}); ok {
					msg := ChatMessage{}
					if role, ok := msgMap["role"].(string); ok {
						msg.Role = role
					}
					// Response API: content can be string OR array of content blocks
					if contentStr, ok := msgMap["content"].(string); ok {
						// Simple string content
						msg.Content = contentStr
					} else if contentArray, ok := msgMap["content"].([]interface{}); ok {
						// Array of content blocks - extract text from each block
						var textParts []string
						for _, block := range contentArray {
							if blockMap, ok := block.(map[string]interface{}); ok {
								if text, ok := blockMap["text"].(string); ok {
									textParts = append(textParts, text)
								}
							}
						}
						msg.Content = strings.Join(textParts, "\n")
					}
					fmt.Printf("[DEBUG] Parsed message: role=%s, content_len=%d\n", msg.Role, len(msg.Content))
					if msg.Role != "" && msg.Content != "" {
						creq.Messages = append(creq.Messages, msg)
					}
				}
			}
		case string:
			// Single string -> user message
			fmt.Printf("[DEBUG] Input is string: %q\n", v)
			if v != "" {
				creq.Messages = []ChatMessage{{Role: "user", Content: v}}
			}
		default:
			fmt.Printf("[DEBUG] Input has unexpected type: %T\n", v)
		}
	} else if len(rr.Messages) > 0 {
		creq.Messages = rr.Messages
	}
	fmt.Printf("[DEBUG] After Input processing: creq.Messages count=%d\n", len(creq.Messages))

	// Instructions -> system message
	if rr.Instructions != "" {
		systemMsg := ChatMessage{Role: "system", Content: rr.Instructions}
		creq.Messages = append([]ChatMessage{systemMsg}, creq.Messages...)
	}

	// Max tokens
	if rr.MaxTokens != nil {
		creq.MaxTokens = rr.MaxTokens
	} else if rr.MaxOutputTokens != nil {
		creq.MaxTokens = rr.MaxOutputTokens
	}

	// Convert tools from flat to nested format
	if len(rr.Tools) > 0 {
		creq.Tools = make([]Tool, len(rr.Tools))
		for i, rtool := range rr.Tools {
			creq.Tools[i] = rtool.ToTool()
		}
	}

	// Tool choice
	creq.ToolChoice = rr.ToolChoice

	// Response format
	if rr.ResponseFormat.Type != "" {
		creq.ResponseFormat = map[string]interface{}{
			"type": rr.ResponseFormat.Type,
		}
		if rr.ResponseFormat.JsonSchema != nil {
			creq.ResponseFormat["json_schema"] = rr.ResponseFormat.JsonSchema
		}
	}

	return creq
}
