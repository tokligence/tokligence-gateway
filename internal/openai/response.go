package openai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResponseRequest represents OpenAI's Response API request format.
// Response API uses a different structure than Chat Completions API.
// https://platform.openai.com/docs/api-reference/responses/create
type ResponseRequest struct {
	Model           string               `json:"model"`
	Input           interface{}          `json:"input,omitempty"`        // Can be messages array
	Messages        []ChatMessage        `json:"messages,omitempty"`     // Alternative to Input
	Instructions    string               `json:"instructions,omitempty"` // System-level instructions
	ID              string               `json:"id,omitempty"`           // Response identifier for follow-up actions
	Temperature     *float64             `json:"temperature,omitempty"`
	TopP            *float64             `json:"top_p,omitempty"`
	MaxOutputTokens *int                 `json:"max_output_tokens,omitempty"` // Response API naming
	MaxTokens       *int                 `json:"max_tokens,omitempty"`        // Chat Completions compatibility
	Stream          bool                 `json:"stream,omitempty"`
	Tools           []ResponseTool       `json:"tools,omitempty"`       // Response API tool format (flat)
	ToolChoice      interface{}          `json:"tool_choice,omitempty"` // "auto", "none", true, false, or specific tool
	ToolOutputs     []ResponseToolOutput `json:"tool_outputs,omitempty"`
	ResponseFormat  struct {
		Type       string                 `json:"type"` // "text", "json_object", "json_schema"
		JsonSchema map[string]interface{} `json:"json_schema,omitempty"`
	} `json:"response_format,omitempty"`
	// Advanced features for Anthropic translation
	WebSearchOptions *WebSearchOptions `json:"web_search_options,omitempty"` // Web search configuration
	ReasoningEffort  string            `json:"reasoning_effort,omitempty"`   // "low", "medium", "high"
	Thinking         *ThinkingConfig   `json:"thinking,omitempty"`           // Thinking/reasoning configuration
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

// ResponseToolOutput describes a tool output entry submitted back to the Responses API.
type ResponseToolOutput struct {
	ToolCallID string `json:"tool_call_id"`
	Output     string `json:"output"`
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

	var converted []ChatMessage

	// Existing chat-style history (already in OpenAI format)
	if len(rr.Messages) > 0 {
		converted = append(converted, rr.Messages...)
	}

	// Response API style input/messages
	for _, msg := range convertResponsesInput(rr.Input) {
		if msg.Role == "" {
			continue
		}
		converted = append(converted, msg)
	}

	creq.Messages = append(creq.Messages, converted...)

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

	// Advanced features
	creq.WebSearchOptions = rr.WebSearchOptions
	creq.ReasoningEffort = rr.ReasoningEffort
	creq.Thinking = rr.Thinking

	return creq
}

func convertResponsesInput(input interface{}) []ChatMessage {
	if input == nil {
		return nil
	}

	switch v := input.(type) {
	case []interface{}:
		var out []ChatMessage
		for _, item := range v {
			out = append(out, convertResponsesMessage(item)...)
		}
		return out
	case map[string]interface{}:
		return convertResponsesMessage(v)
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []ChatMessage{{Role: "user", Content: v}}
	default:
		fmt.Printf("convertResponsesInput: unsupported type %T\n", v)
		return nil
	}
}

func convertResponsesMessage(item interface{}) []ChatMessage {
	m, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}

	role := strings.ToLower(strings.TrimSpace(asString(m["role"])))

	// If no role, check if it's a function_call_output (Responses API continuation)
	if role == "" {
		typ := strings.ToLower(strings.TrimSpace(asString(m["type"])))
		if typ == "function_call_output" {
			// Convert function_call_output to tool message
			callID := asString(m["call_id"])
			output := asString(m["output"])
			if callID != "" && output != "" {
				return []ChatMessage{{
					Role:       "tool",
					Content:    output,
					ToolCallID: callID,
				}}
			}
		}
		return nil
	}

	content := m["content"]

	switch role {
	case "assistant":
		return buildAssistantMessages(content)
	case "tool":
		return buildToolMessages(content)
	case "system", "developer":
		text := collectTextBlocks(content, true)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []ChatMessage{{Role: "system", Content: text}}
	default:
		text := collectTextBlocks(content, true)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []ChatMessage{{Role: "user", Content: text}}
	}
}

func buildAssistantMessages(content interface{}) []ChatMessage {
	var textParts []string
	var toolCalls []ToolCall
	textParts, toolCalls = extractAssistantContent(content)

	var out []ChatMessage
	msg := ChatMessage{Role: "assistant"}
	if len(textParts) > 0 {
		msg.Content = strings.Join(textParts, "\n")
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	if strings.TrimSpace(msg.Content) != "" || len(toolCalls) > 0 {
		out = append(out, msg)
	}
	out = append(out, extractToolResultMessages(content)...)
	return out
}

func buildToolMessages(content interface{}) []ChatMessage {
	return extractToolResultMessages(content)
}

func extractAssistantContent(content interface{}) ([]string, []ToolCall) {
	var textParts []string
	var toolCalls []ToolCall

	appendText := func(v string) {
		if strings.TrimSpace(v) != "" {
			textParts = append(textParts, v)
		}
	}

	switch blocks := content.(type) {
	case string:
		appendText(blocks)
	case []interface{}:
		for _, b := range blocks {
			if block, ok := b.(map[string]interface{}); ok {
				typ := strings.ToLower(asString(block["type"]))
				switch typ {
				case "tool_call", "function_call":
					tc := convertToolCallBlock(block)
					if strings.TrimSpace(tc.ID) == "" && strings.TrimSpace(tc.Function.Name) == "" && strings.TrimSpace(tc.Function.Arguments) == "" {
						continue
					}
					if strings.TrimSpace(tc.Function.Arguments) == "" {
						tc.Function.Arguments = "{}"
					}
					toolCalls = append(toolCalls, tc)
				case "output_text", "input_text", "text":
					appendText(asString(block["text"]))
				}
			}
		}
	case map[string]interface{}:
		appendText(asString(blocks["text"]))
	}

	return textParts, toolCalls
}

func extractToolResultMessages(content interface{}) []ChatMessage {
	var out []ChatMessage

	addMessage := func(callID, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		out = append(out, ChatMessage{
			Role:       "tool",
			Content:    text,
			ToolCallID: callID,
		})
	}

	switch blocks := content.(type) {
	case []interface{}:
		for _, b := range blocks {
			block, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			typ := strings.ToLower(asString(block["type"]))
			// Support both tool_result and function_call_output (Responses API)
			if typ != "tool_result" && typ != "function_call_output" {
				continue
			}
			callID := firstNonEmpty(asString(block["tool_use_id"]), asString(block["tool_call_id"]), asString(block["call_id"]))
			// For function_call_output, extract from "output" field as well
			text := collectResultText(block)
			if typ == "function_call_output" && strings.TrimSpace(text) == "" {
				text = asString(block["output"])
			}
			addMessage(callID, text)
		}
	case map[string]interface{}:
		typ := strings.ToLower(asString(blocks["type"]))
		if typ == "tool_result" || typ == "function_call_output" {
			callID := firstNonEmpty(asString(blocks["tool_use_id"]), asString(blocks["tool_call_id"]), asString(blocks["call_id"]))
			text := collectResultText(blocks)
			if typ == "function_call_output" && strings.TrimSpace(text) == "" {
				text = asString(blocks["output"])
			}
			addMessage(callID, text)
		}
	case string:
		// If role is tool and content is plain string, treat it as direct output with unknown call id.
		addMessage("", blocks)
	}

	return out
}

func convertToolCallBlock(block map[string]interface{}) ToolCall {
	id := firstNonEmpty(asString(block["id"]), asString(block["call_id"]), asString(block["tool_call_id"]))
	name := asString(block["name"])

	var args string
	if fn, ok := block["function"].(map[string]interface{}); ok {
		if n := asString(fn["name"]); n != "" && name == "" {
			name = n
		}
		if raw := fn["arguments"]; raw != nil {
			args = stringifyJSON(raw)
		}
	}
	if raw := block["arguments"]; raw != nil && strings.TrimSpace(args) == "" {
		args = stringifyJSON(raw)
	}
	if strings.TrimSpace(args) == "" {
		args = "{}"
	}

	return ToolCall{
		ID:   id,
		Type: "function",
		Function: FunctionCall{
			Name:      name,
			Arguments: args,
		},
	}
}

func collectTextBlocks(content interface{}, allowString bool) string {
	if s, ok := content.(string); ok {
		if allowString {
			return s
		}
		return ""
	}

	blocks, ok := content.([]interface{})
	if !ok {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		block, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		typ := strings.ToLower(asString(block["type"]))
		switch typ {
		case "input_text", "output_text", "text":
			if val := strings.TrimSpace(asString(block["text"])); val != "" {
				parts = append(parts, val)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func collectResultText(block map[string]interface{}) string {
	if text := strings.TrimSpace(asString(block["text"])); text != "" {
		return text
	}

	if content := block["content"]; content != nil {
		switch v := content.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		case []interface{}:
			var parts []string
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if strings.EqualFold(asString(itemMap["type"]), "output_text") {
						if txt := strings.TrimSpace(asString(itemMap["text"])); txt != "" {
							parts = append(parts, txt)
						}
					}
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "\n")
			}
		}
	}

	// Fallback: marshal entire block without type metadata.
	b, err := json.Marshal(block)
	if err != nil {
		return ""
	}
	return string(b)
}

func stringifyJSON(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case json.RawMessage:
		return string(t)
	default:
		data, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
