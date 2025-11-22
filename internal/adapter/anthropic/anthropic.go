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

	translatorpkg "github.com/tokligence/openai-anthropic-endpoint-translation/pkg/translator"
	"github.com/tokligence/tokligence-gateway/internal/adapter"
	anthropicstream "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

var _ adapter.ChatAdapter = (*AnthropicAdapter)(nil)
var _ adapter.StreamingChatAdapter = (*AnthropicAdapter)(nil)

// AnthropicAdapter sends OpenAI-compatible chat requests to Anthropic using the
// shared translator module for request/response conversion.
type AnthropicAdapter struct {
	apiKey     string
	baseURL    string
	version    string
	httpClient *http.Client
	translator translatorpkg.Translator
}

// Config defines Anthropic adapter settings.
type Config struct {
	APIKey         string
	BaseURL        string
	Version        string
	RequestTimeout time.Duration
}

// New constructs an AnthropicAdapter.
func New(cfg Config) (*AnthropicAdapter, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("anthropic: api key required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

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
		translator: translatorpkg.NewTranslator(),
	}, nil
}

// CreateCompletion converts the request to Anthropic format, calls /v1/messages,
// and maps the response back to OpenAI format.
func (a *AnthropicAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	treq, err := buildTranslatorRequest(req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	prefix := translatorpkg.PrefixPrompt(treq.Messages)
	anthropicReq, _, err := a.translator.BuildRequest(treq)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: create request: %w", err)
	}
	setCommonHeaders(httpReq.Header, a.apiKey, a.version)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var anthropicResp translatorpkg.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: decode response: %w", err)
	}

	oaiResp, err := a.translator.ConvertResponse(anthropicResp, prefix, anthropicReq.JsonMode)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("anthropic: convert response: %w", err)
	}

	return convertTranslatorResponseWithUsage(oaiResp, anthropicResp.Usage, req.Model), nil
}

// CreateCompletionStream sends a streaming request to Anthropic and forwards
// SSE deltas as OpenAI chunks.
func (a *AnthropicAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	treq, err := buildTranslatorRequest(req)
	if err != nil {
		return nil, err
	}
	treq.Stream = true
	anthropicReq, _, err := a.translator.BuildRequest(treq)
	if err != nil {
		return nil, err
	}
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create stream request: %w", err)
	}
	setCommonHeaders(httpReq.Header, a.apiKey, a.version)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: send stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return nil, fmt.Errorf("anthropic: stream http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	ch := make(chan adapter.StreamEvent, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		emit := func(chunk openai.ChatCompletionChunk) error {
			ch <- adapter.StreamEvent{Chunk: &chunk}
			return nil
		}
		if err := anthropicstream.StreamAnthropicToOpenAI(ctx, req.Model, resp.Body, emit); err != nil && !errors.Is(err, context.Canceled) {
			ch <- adapter.StreamEvent{Error: err}
		}
	}()

	return ch, nil
}

func setCommonHeaders(h http.Header, apiKey, version string) {
	h.Set("Content-Type", "application/json")
	h.Set("x-api-key", apiKey)
	h.Set("anthropic-version", version)
}

func buildTranslatorRequest(req openai.ChatCompletionRequest) (translatorpkg.OpenAIChatRequest, error) {
	tr := translatorpkg.OpenAIChatRequest{
		Model:           mapModelName(req.Model),
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		ResponseFormat:  req.ResponseFormat,
		ReasoningEffort: req.ReasoningEffort,
	}
	if req.MaxTokens != nil {
		tr.MaxTokens = new(int)
		*tr.MaxTokens = *req.MaxTokens
	}
	if len(req.Metadata) > 0 {
		meta := make(map[string]any, len(req.Metadata))
		for k, v := range req.Metadata {
			meta[k] = v
		}
		tr.Metadata = meta
	}

	// Web search options
	if req.WebSearchOptions != nil {
		tr.WebSearchOptions = &translatorpkg.WebSearchOptions{
			UserLocation:      req.WebSearchOptions.UserLocation,
			SearchContextSize: req.WebSearchOptions.SearchContextSize,
		}
	}

	// Thinking configuration
	if req.Thinking != nil {
		tr.Thinking = &translatorpkg.ThinkingConfig{
			Type:         req.Thinking.Type,
			BudgetTokens: 0, // Will be set if BudgetTokens is provided
		}
		if req.Thinking.BudgetTokens != nil {
			tr.Thinking.BudgetTokens = *req.Thinking.BudgetTokens
		}
	}

	// P0.5 Quick Fields
	if req.MaxCompletionTokens != nil {
		tr.MaxCompletion = new(int)
		*tr.MaxCompletion = *req.MaxCompletionTokens
	}
	tr.ParallelToolCalls = req.ParallelToolCalls
	if req.User != "" {
		tr.User = req.User
	}

	msgs := make([]translatorpkg.OpenAIMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, convertMessage(m))
	}
	tr.Messages = msgs

	if len(req.Tools) > 0 {
		tr.Tools = convertTools(req.Tools)
	}
	if req.ToolChoice != nil {
		if tc := convertToolChoice(req.ToolChoice); tc != nil {
			tr.ToolChoice = tc
		}
	}

	return tr, nil
}

func convertMessage(msg openai.ChatMessage) translatorpkg.OpenAIMessage {
	tMsg := translatorpkg.OpenAIMessage{
		Role:       msg.Role,
		ToolCallID: msg.ToolCallID,
	}
	trimmed := strings.TrimSpace(msg.Content)
	if trimmed != "" {
		tMsg.Content = []translatorpkg.ContentBlock{translatorpkg.NewTextBlock(trimmed)}
	}
	if len(msg.ToolCalls) > 0 {
		calls := make([]translatorpkg.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			calls = append(calls, translatorpkg.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: translatorpkg.ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		tMsg.ToolCalls = calls
	}
	// Cache control for prompt caching
	if len(msg.CacheControl) > 0 {
		tMsg.CacheControl = msg.CacheControl
	}
	return tMsg
}

func convertTools(tools []openai.Tool) []translatorpkg.Tool {
	out := make([]translatorpkg.Tool, 0, len(tools))
	for _, t := range tools {
		raw := map[string]any{"type": t.Type}

		switch t.Type {
		case "function":
			// Standard function tools
			if t.Function != nil {
				rawFn := map[string]any{
					"name": t.Function.Name,
				}
				if strings.TrimSpace(t.Function.Description) != "" {
					rawFn["description"] = t.Function.Description
				}
				if len(t.Function.Parameters) > 0 {
					rawFn["parameters"] = t.Function.Parameters
				}
				if len(t.Function.CacheControl) > 0 {
					rawFn["cache_control"] = t.Function.CacheControl
				}
				raw["function"] = rawFn
			}

		case "url", "mcp":
			// MCP Server tools
			if t.URL != "" {
				raw["url"] = t.URL
			}
			if t.ServerURL != "" {
				raw["server_url"] = t.ServerURL
			}
			if t.Name != "" {
				raw["name"] = t.Name
			}
			if t.ServerLabel != "" {
				raw["server_label"] = t.ServerLabel
			}
			if len(t.ToolConfiguration) > 0 {
				raw["tool_configuration"] = t.ToolConfiguration
			}
			if len(t.Headers) > 0 {
				raw["headers"] = t.Headers
			}
			if t.AuthorizationToken != "" {
				raw["authorization_token"] = t.AuthorizationToken
			}

		default:
			// Computer tools (computer_*) and other Anthropic-hosted tools
			// Pass through as-is from the request
			if t.Name != "" {
				raw["name"] = t.Name
			}
			if t.DisplayWidthPx > 0 {
				raw["display_width_px"] = t.DisplayWidthPx
			}
			if t.DisplayHeightPx > 0 {
				raw["display_height_px"] = t.DisplayHeightPx
			}
			if t.DisplayNumber > 0 {
				raw["display_number"] = t.DisplayNumber
			}
			// For fully custom tools, include function if provided
			if t.Function != nil {
				rawFn := map[string]any{
					"name": t.Function.Name,
				}
				if strings.TrimSpace(t.Function.Description) != "" {
					rawFn["description"] = t.Function.Description
				}
				if len(t.Function.Parameters) > 0 {
					rawFn["parameters"] = t.Function.Parameters
				}
				raw["function"] = rawFn
			}
		}

		// Cache control can be on any tool
		if len(t.CacheControl) > 0 {
			raw["cache_control"] = t.CacheControl
		}

		out = append(out, translatorpkg.Tool{Type: t.Type, Raw: raw})
	}
	return out
}

func convertToolChoice(value interface{}) *translatorpkg.ToolChoice {
	switch v := value.(type) {
	case string:
		tc := &translatorpkg.ToolChoice{Kind: strings.ToLower(v)}
		return tc
	case map[string]interface{}:
		raw := make(map[string]any, len(v))
		for k, val := range v {
			raw[k] = val
		}
		tc := &translatorpkg.ToolChoice{RawObject: raw}
		if kind, ok := raw["type"].(string); ok {
			tc.Kind = strings.ToLower(kind)
		}
		if fn, ok := raw["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				tc.FunctionName = name
			}
		}
		return tc
	default:
		return nil
	}
}

func convertTranslatorResponse(resp translatorpkg.OpenAIChatResponse, fallbackModel string) openai.ChatCompletionResponse {
	return convertTranslatorResponseWithUsage(resp, translatorpkg.AnthropicUsage{}, fallbackModel)
}

func convertTranslatorResponseWithUsage(resp translatorpkg.OpenAIChatResponse, anthropicUsage translatorpkg.AnthropicUsage, fallbackModel string) openai.ChatCompletionResponse {
	model := fallbackModel
	if strings.TrimSpace(resp.Model) != "" {
		model = resp.Model
	}

	choices := make([]openai.ChatCompletionChoice, 0, len(resp.Choices))
	for _, ch := range resp.Choices {
		choice := openai.ChatCompletionChoice{
			Index:        ch.Index,
			FinishReason: ch.FinishReason,
			Message:      convertTranslatorMessage(ch.Message),
		}
		choices = append(choices, choice)
	}

	usage := openai.UsageBreakdown{
		PromptTokens:        resp.Usage.PromptTokens,
		CompletionTokens:    resp.Usage.CompletionTokens,
		TotalTokens:         resp.Usage.TotalTokens,
		CacheCreationTokens: anthropicUsage.CacheCreationInput,
		CacheReadTokens:     anthropicUsage.CacheReadInput,
		// ReasoningTokens would come from anthropicUsage if Anthropic adds it
	}

	return openai.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: choices,
		Usage:   usage,
	}
}

func convertTranslatorMessage(msg translatorpkg.OpenAIMessage) openai.ChatMessage {
	out := openai.ChatMessage{Role: msg.Role, ToolCallID: msg.ToolCallID}
	var sb strings.Builder
	for idx, block := range msg.Content {
		if text, ok := block.AsText(); ok {
			if sb.Len() > 0 && strings.TrimSpace(text) != "" {
				sb.WriteString("\n")
			}
			sb.WriteString(text)
			continue
		}
		// Fallback: serialize non-text blocks to JSON
		if block.Data != nil {
			if idx > 0 {
				sb.WriteString("\n")
			}
			if data, err := json.Marshal(block.Data); err == nil {
				sb.Write(data)
			}
		}
	}
	out.Content = sb.String()

	if len(msg.ToolCalls) > 0 {
		calls := make([]openai.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			calls = append(calls, openai.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: openai.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		out.ToolCalls = calls
	}
	return out
}

// mapModelName maps OpenAI-style aliases to Anthropic model identifiers.
func mapModelName(model string) string {
	name := strings.TrimSpace(strings.ToLower(model))
	switch name {
	case "claude-instant", "claude-instant-1", "claude-instant-v1":
		return "claude-instant-v1"
	case "claude-3-haiku", "claude-3-haiku-20240307", "claude-3-haiku-20240307-v1", "claude-3-haiku-v1":
		return "claude-3-haiku-20240307"
	case "claude-3-sonnet", "claude-3-sonnet-20240229", "claude-3-sonnet-20240229-v1", "claude-3-sonnet-v1":
		return "claude-3-5-sonnet-20241022"
	case "claude-3-opus", "claude-3-opus-20240229":
		return "claude-3-opus-20240229"
	case "gpt-4", "gpt-4o", "gpt-4.1", "gpt-4.1-mini":
		return "claude-3-5-sonnet-20241022"
	default:
		if strings.HasPrefix(name, "claude") {
			return model
		}
		return "claude-3-5-sonnet-20241022"
	}
}
