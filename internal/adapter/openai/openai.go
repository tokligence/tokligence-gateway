package openai

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

// Ensure OpenAIAdapter implements ChatAdapter.
var _ adapter.ChatAdapter = (*OpenAIAdapter)(nil)

// OpenAIAdapter sends requests to the OpenAI API.
type OpenAIAdapter struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	org        string // optional organization ID
}

// Config holds configuration for the OpenAI adapter.
type Config struct {
	APIKey         string
	BaseURL        string // optional, defaults to https://api.openai.com/v1
	Organization   string // optional
	RequestTimeout time.Duration
}

// New creates an OpenAIAdapter instance.
func New(cfg Config) (*OpenAIAdapter, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("openai: api key required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OpenAIAdapter{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		org:     cfg.Organization,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// CreateCompletion sends a chat completion request to OpenAI.
func (a *OpenAIAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if len(req.Messages) == 0 {
		return openai.ChatCompletionResponse{}, errors.New("openai: no messages provided")
	}

	// Build OpenAI API request
	payload := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
	}

	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}

	// OpenAI doesn't support streaming in this adapter yet (will be added in streaming task)
	payload["stream"] = false

	body, err := json.Marshal(payload)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("openai: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	if a.org != "" {
		httpReq.Header.Set("OpenAI-Organization", a.org)
	}

	// Send request
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("openai: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("openai: read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return openai.ChatCompletionResponse{}, fmt.Errorf("openai: %s (type=%s, code=%s)", errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
		}
		return openai.ChatCompletionResponse{}, fmt.Errorf("openai: http %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse successful response
	var completion openai.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return openai.ChatCompletionResponse{}, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	return completion, nil
}
