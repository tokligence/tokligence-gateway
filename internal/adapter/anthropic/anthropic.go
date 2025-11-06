package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// Ensure AnthropicAdapter implements ChatAdapter.
var _ adapter.ChatAdapter = (*AnthropicAdapter)(nil)
var _ adapter.StreamingChatAdapter = (*AnthropicAdapter)(nil)

// AnthropicAdapter sends requests to the Anthropic API (Claude).
type AnthropicAdapter struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	version    string // API version header
}

// Config holds configuration for the Anthropic adapter.
type Config struct {
	APIKey         string
	BaseURL        string // optional, defaults to https://api.anthropic.com
	Version        string // optional, defaults to 2023-06-01
	RequestTimeout time.Duration
}

// New creates an AnthropicAdapter instance.
func New(cfg Config) (*AnthropicAdapter, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("anthropic: api key required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	version := strings.TrimSpace(cfg.Version)
	if version == "" {
		version = "2023-06-01"
	}

	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &AnthropicAdapter{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		version: version,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// CreateCompletion converts OpenAI format to Anthropic format and sends request.
func (a *AnthropicAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if len(req.Messages) == 0 {
		return openai.ChatCompletionResponse{}, errors.New("anthropic: no messages provided")
	}

	// Convert OpenAI messages to Anthropic format
	messages, systemPrompt, err := convertMessages(req.Messages)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: convert messages: %w", err)
	}

	// Map model name (support both OpenAI-style and Anthropic-style names)
	model := mapModelName(req.Model)

	// Determine max_tokens
	maxTokens := 4096 // default
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}

	// Build Anthropic API request
	payload := map[string]interface{}{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}

	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}

	// Convert and add tools if present (only when valid)
	if len(req.Tools) > 0 {
		anthropicTools := convertTools(req.Tools)
		if len(anthropicTools) > 0 {
			payload["tools"] = anthropicTools
			// Handle tool_choice only when tools present
			if req.ToolChoice != nil {
				payload["tool_choice"] = convertToolChoice(req.ToolChoice)
			}
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", a.version)

	// Send request
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Type  string `json:"type"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: %s (type=%s)", errResp.Error.Message, errResp.Error.Type)
		}
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse Anthropic response
	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: unmarshal response: %w", err)
	}

	// Convert to OpenAI format
	return convertToOpenAIResponse(anthropicResp, req.Model), nil
}

// CreateCompletionStream sends a streaming request to Anthropic and converts SSE events to OpenAI chunks.
func (a *AnthropicAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("anthropic: no messages provided")
	}

	messages, systemPrompt, err := convertMessages(req.Messages)
	if err != nil {
		return nil, fmt.Errorf("anthropic: convert messages: %w", err)
	}
	model := mapModelName(req.Model)

	payload := map[string]interface{}{
		"model":      model,
		"messages":   messages,
		"max_tokens": 4096,
		"stream":     true,
	}
	// Add tools/tool_choice for streaming as well (parity with non-streaming)
	if len(req.Tools) > 0 {
		tools := convertTools(req.Tools)
		if len(tools) > 0 {
			payload["tools"] = tools
			if req.ToolChoice != nil {
				payload["tool_choice"] = convertToolChoice(req.ToolChoice)
			}
		}
	}
	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}
	// DEBUG: Log request payload to see tools and tool_choice
	if len(req.Tools) > 0 {
		fmt.Printf("[DEBUG] Anthropic request tools: %d, tool_choice: %+v\n", len(req.Tools), payload["tool_choice"])
		if toolsJSON, err := json.MarshalIndent(payload["tools"], "", "  "); err == nil {
			fmt.Printf("[DEBUG] Anthropic tools JSON:\n%s\n", string(toolsJSON))
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", a.version)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, string(data))
	}

	ch := make(chan adapter.StreamEvent, 10)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		reader := resp.Body
		buf := make([]byte, 8192)
		leftover := ""
		// minimal state for role emission once
		roleEmitted := false
		// track tool_use content blocks by index to build arguments
		type toolState struct{ id, name string }
		toolBlocks := map[int]*toolState{}
		for {
			select {
			case <-ctx.Done():
				ch <- adapter.StreamEvent{Error: ctx.Err()}
				return
			default:
			}

			n, err := reader.Read(buf)
			if n > 0 {
				data := leftover + string(buf[:n])
				lines := strings.Split(data, "\n")
				leftover = lines[len(lines)-1]
				lines = lines[:len(lines)-1]
				var eventType string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					if strings.HasPrefix(line, "event:") {
						eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
						continue
					}
					if !strings.HasPrefix(line, "data:") {
						continue
					}
					payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
					// Some servers may send keepalive ping with '{}' or comments
					if payload == "{}" || payload == "[DONE]" {
						continue
					}
					// Parse streaming message
					var evt anthropicStreamEvent
					if perr := json.Unmarshal([]byte(payload), &evt); perr != nil {
						ch <- adapter.StreamEvent{Error: fmt.Errorf("anthropic: parse stream: %w", perr)}
						return
					}
					// Handle tool_use start
					if evt.Type == "content_block_start" && evt.ContentBlock.Type == "tool_use" {
						ts := &toolState{id: evt.ContentBlock.ID, name: evt.ContentBlock.Name}
						toolBlocks[evt.Index] = ts
						delta := openai.ChatMessageDelta{}
						if !roleEmitted {
							roleEmitted = true
							delta.Role = "assistant"
						}
						delta.ToolCalls = []openai.ToolCallDelta{{
							Index:    evt.Index,
							ID:       ts.id,
							Type:     "function",
							Function: &openai.ToolFunctionPart{Name: ts.name},
						}}
						chunk := openai.ChatCompletionChunk{ID: "msg-stream", Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: req.Model, Choices: []openai.ChatCompletionChunkChoice{{Index: 0, Delta: delta}}}
						ch <- adapter.StreamEvent{Chunk: &chunk}
						continue
					}
					// Handle tool_use input JSON partial deltas
					if evt.Type == "content_block_delta" && evt.Delta.Type == "input_json_delta" && evt.Delta.PartialJSON != "" {
						if _, ok := toolBlocks[evt.Index]; ok {
							delta := openai.ChatMessageDelta{}
							if !roleEmitted {
								roleEmitted = true
								delta.Role = "assistant"
							}
							delta.ToolCalls = []openai.ToolCallDelta{{
								Index:    evt.Index,
								Type:     "function",
								Function: &openai.ToolFunctionPart{Arguments: evt.Delta.PartialJSON},
							}}
							chunk := openai.ChatCompletionChunk{ID: "msg-stream", Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: req.Model, Choices: []openai.ChatCompletionChunkChoice{{Index: 0, Delta: delta}}}
							ch <- adapter.StreamEvent{Chunk: &chunk}
							continue
						}
					}
					// We are interested in content_block_delta with text_delta
					if evt.Type == "content_block_delta" && evt.Delta.Type == "text_delta" && evt.Delta.Text != "" {
						delta := openai.ChatMessageDelta{Content: evt.Delta.Text}
						if !roleEmitted {
							roleEmitted = true
							delta.Role = "assistant"
						}
						chunk := openai.ChatCompletionChunk{
							ID:      "msg-stream",
							Object:  "chat.completion.chunk",
							Created: time.Now().Unix(),
							Model:   req.Model,
							Choices: []openai.ChatCompletionChunkChoice{{
								Index: 0,
								Delta: delta,
							}},
						}
						ch <- adapter.StreamEvent{Chunk: &chunk}
						continue
					}
					// message_delta with stop_reason -> emit finish_reason chunk
					if evt.Type == "message_delta" && evt.Delta.StopReason != "" {
						var finishReason string
						switch evt.Delta.StopReason {
						case "tool_use":
							finishReason = "tool_calls"
						case "end_turn":
							finishReason = "stop"
						case "max_tokens":
							finishReason = "length"
						default:
							finishReason = "stop"
						}
						chunk := openai.ChatCompletionChunk{
							ID:      "msg-stream",
							Object:  "chat.completion.chunk",
							Created: time.Now().Unix(),
							Model:   req.Model,
							Choices: []openai.ChatCompletionChunkChoice{{
								Index:        0,
								Delta:        openai.ChatMessageDelta{},
								FinishReason: &finishReason,
							}},
						}
						ch <- adapter.StreamEvent{Chunk: &chunk}
						continue
					}
					// message_stop -> finish
					if evt.Type == "message_stop" || eventType == "message_stop" {
						return
					}
				}
			}
			if err != nil {
				if err == io.EOF {
					return
				}
				ch <- adapter.StreamEvent{Error: fmt.Errorf("anthropic: read stream: %w", err)}
				return
			}
		}
	}()
	return ch, nil
}

// anthropicMessage represents a message in Anthropic's format.
type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content,omitempty"`
}

// anthropicContentBlock represents a content block (text/tool_use/tool_result).
type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// Tool use fields
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
	// Tool result fields
	ToolUseID string                       `json:"tool_use_id,omitempty"`
	Content   []anthropicToolResultContent `json:"content,omitempty"`
}

// anthropicToolResultContent is a nested content block for tool results.
type anthropicToolResultContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicTool represents a tool definition in Anthropic's format.
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicResponse represents Anthropic's response format.
type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage          `json:"usage"`
}

// anthropicUsage represents token usage in Anthropic's format.
type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Streaming event minimal schema
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	// For content_block_delta
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
		// For tool_use JSON streaming
		PartialJSON string `json:"partial_json,omitempty"`
		// For message_delta events
		StopReason string `json:"stop_reason,omitempty"`
	} `json:"delta,omitempty"`
	// For content_block_start events
	ContentBlock anthropicContentBlock `json:"content_block,omitempty"`
}

// convertMessages converts OpenAI messages to Anthropic format.
// Returns messages array, system prompt, and error.
func convertMessages(openaiMessages []openai.ChatMessage) ([]anthropicMessage, string, error) {
	var messages []anthropicMessage
	var systemParts []string
	autoToolCounter := 0
	var pendingToolIDs []string

	appendSystem := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		systemParts = append(systemParts, text)
	}

	for idx, msg := range openaiMessages {
		role := strings.ToLower(msg.Role)

		// Extract system messages
		if role == "system" {
			appendSystem(msg.Content)
			continue
		}

		switch role {
		case "user":
			blocks := []anthropicContentBlock{}
			if strings.TrimSpace(msg.Content) != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: blocks,
			})
		case "assistant":
			var blocks []anthropicContentBlock
			if strings.TrimSpace(msg.Content) != "" {
				blocks = append(blocks, anthropicContentBlock{
					Type: "text",
					Text: msg.Content,
				})
			}
			for tIdx, tc := range msg.ToolCalls {
				id := strings.TrimSpace(tc.ID)
				if id == "" {
					autoToolCounter++
					id = fmt.Sprintf("tool_call_%d_%d", idx, autoToolCounter)
				}
				pendingToolIDs = append(pendingToolIDs, id)
				input := map[string]interface{}{}
				rawArgs := strings.TrimSpace(tc.Function.Arguments)
				if rawArgs != "" {
					if err := json.Unmarshal([]byte(rawArgs), &input); err != nil {
						input = map[string]interface{}{"_raw": rawArgs}
					}
				}
				if tc.Function.Name == "" {
					tc.Function.Name = fmt.Sprintf("function_%d_%d", idx, tIdx)
				}
				blocks = append(blocks, anthropicContentBlock{
					Type:  "tool_use",
					ID:    id,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			messages = append(messages, anthropicMessage{
				Role:    "assistant",
				Content: blocks,
			})
		case "tool":
			toolID := strings.TrimSpace(msg.ToolCallID)
			if toolID == "" {
				// Try to consume the next pending tool ID (if any)
				if len(pendingToolIDs) > 0 {
					toolID = pendingToolIDs[0]
					pendingToolIDs = pendingToolIDs[1:]
				} else {
					autoToolCounter++
					toolID = fmt.Sprintf("tool_result_%d_%d", idx, autoToolCounter)
				}
			} else {
				for i, pending := range pendingToolIDs {
					if pending == toolID {
						pendingToolIDs = append(pendingToolIDs[:i], pendingToolIDs[i+1:]...)
						break
					}
				}
			}
			text := strings.TrimSpace(msg.Content)
			inner := []anthropicToolResultContent{}
			if text != "" {
				inner = append(inner, anthropicToolResultContent{
					Type: "text",
					Text: text,
				})
			}
			messages = append(messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: toolID,
						Content:   inner,
					},
				},
			})
		default:
			// Treat any other role as user text to keep parity with OpenAI schema
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			messages = append(messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContentBlock{
					{
						Type: "text",
						Text: msg.Content,
					},
				},
			})
		}
	}

	if len(messages) == 0 {
		return nil, "", errors.New("no user/assistant messages after filtering system messages")
	}

	systemPrompt := strings.Join(systemParts, "\n\n")
	return messages, systemPrompt, nil
}

// mapModelName maps OpenAI-style model names to Anthropic model names.
func mapModelName(model string) string {
	model = strings.ToLower(model)

	// Map common aliases first
	switch model {
	case "claude", "claude-3":
		return "claude-3-opus-20240229"
	case "claude-sonnet", "claude-3-sonnet":
		return "claude-3-5-sonnet-20241022"
	case "claude-haiku", "claude-3-haiku":
		return "claude-3-5-haiku-20241022"
	}

	// If it's already a full Anthropic model name with date, use it directly
	if strings.HasPrefix(model, "claude-") && len(model) > 20 {
		return model
	}

	// Default to sonnet if unknown
	return "claude-3-5-sonnet-20241022"
}

// convertToOpenAIResponse converts Anthropic response to OpenAI format.
func convertToOpenAIResponse(resp anthropicResponse, originalModel string) openai.ChatCompletionResponse {
	// Extract text content and tool calls
	var content string
	var toolCalls []openai.ToolCall

	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		} else if block.Type == "tool_use" {
			// Convert Anthropic tool_use to OpenAI tool_call
			argsBytes, err := json.Marshal(block.Input)
			if err != nil {
				continue // Skip malformed tool calls
			}

			toolCalls = append(toolCalls, openai.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: openai.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	// Map stop reason
	finishReason := "stop"
	switch resp.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "stop_sequence":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	}

	// Build message
	message := openai.ChatMessage{
		Role:    "assistant",
		Content: content,
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
		// If there are tool calls, finish_reason should be tool_calls
		finishReason = "tool_calls"
	}

	return openai.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   originalModel,
		Choices: []openai.ChatCompletionChoice{
			{
				Index:        0,
				FinishReason: finishReason,
				Message:      message,
			},
		},
		Usage: openai.UsageBreakdown{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

// convertTools converts OpenAI tools to Anthropic format.
func convertTools(tools []openai.Tool) []anthropicTool {
	var result []anthropicTool
	fmt.Printf("[DEBUG] convertTools: received %d tools\n", len(tools))
	for i, tool := range tools {
		fmt.Printf("[DEBUG] Tool[%d]: Type=%s, Function=%+v\n", i, tool.Type, tool.Function)
		if tool.Type != "function" {
			fmt.Printf("[DEBUG] Tool[%d]: skipping, not function type\n", i)
			continue // Anthropic only supports function tools
		}

		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			fmt.Printf("[DEBUG] Tool[%d]: skipping, empty name\n", i)
			continue // skip invalid/empty names
		}
		fmt.Printf("[DEBUG] Tool[%d]: converting to Anthropic format, name=%s\n", i, name)
		result = append(result, anthropicTool{
			Name:        name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	return result
}

// convertToolChoice converts OpenAI tool_choice to Anthropic format.
func convertToolChoice(choice interface{}) interface{} {
	if choice == nil {
		return nil
	}

	// Handle boolean values (Codex CLI format)
	// tool_choice: true means "required" (force tool use)
	// tool_choice: false means "auto" (optional)
	if b, ok := choice.(bool); ok {
		if b {
			return map[string]interface{}{"type": "any"}
		}
		return map[string]interface{}{"type": "auto"}
	}

	// Handle string values: "auto", "none", "required"
	if str, ok := choice.(string); ok {
		switch str {
		case "auto":
			return map[string]interface{}{"type": "auto"}
		case "none":
			// Anthropic doesn't have explicit "none", just omit tools
			return nil
		case "required", "any":
			return map[string]interface{}{"type": "any"}
		default:
			return map[string]interface{}{"type": "auto"}
		}
	}

	// Handle object format: {"type": "function", "function": {"name": "..."}}
	if obj, ok := choice.(map[string]interface{}); ok {
		if typ, exists := obj["type"]; exists && typ == "function" {
			if fn, exists := obj["function"]; exists {
				if fnMap, ok := fn.(map[string]interface{}); ok {
					if name, exists := fnMap["name"]; exists {
						return map[string]interface{}{
							"type": "tool",
							"name": name,
						}
					}
				}
			}
		}
	}

	// Default to auto
	return map[string]interface{}{"type": "auto"}
}
