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

	// Build Anthropic API request
	payload := map[string]interface{}{
		"model":      model,
		"messages":   messages,
		"max_tokens": 4096, // Anthropic requires max_tokens
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
	Role    string                   `json:"role"`
	Content []anthropicContentBlock  `json:"content,omitempty"`
}

// anthropicContentBlock represents a content block (text or other types).
type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
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
    } `json:"delta,omitempty"`
}

// convertMessages converts OpenAI messages to Anthropic format.
// Returns messages array, system prompt, and error.
func convertMessages(openaiMessages []openai.ChatMessage) ([]anthropicMessage, string, error) {
	var messages []anthropicMessage
	var systemPrompt string

	for _, msg := range openaiMessages {
		role := strings.ToLower(msg.Role)

		// Extract system messages
		if role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n\n"
			}
			systemPrompt += msg.Content
			continue
		}

		// Convert role names
		if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}

		messages = append(messages, anthropicMessage{
			Role: role,
			Content: []anthropicContentBlock{
				{
					Type: "text",
					Text: msg.Content,
				},
			},
		})
	}

	if len(messages) == 0 {
		return nil, "", errors.New("no user/assistant messages after filtering system messages")
	}

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
	// Extract text content
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
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
				Message: openai.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
			},
		},
		Usage: openai.UsageBreakdown{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}
