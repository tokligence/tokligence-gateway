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

// Ensure OpenAIAdapter implements StreamingChatAdapter and EmbeddingAdapter.
var _ adapter.StreamingChatAdapter = (*OpenAIAdapter)(nil)
var _ adapter.EmbeddingAdapter = (*OpenAIAdapter)(nil)

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

// CreateCompletionStream sends a streaming chat completion request to OpenAI.
func (a *OpenAIAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	if len(req.Messages) == 0 {
		return nil, errors.New("openai: no messages provided")
	}

	// Build OpenAI API request
    payload := map[string]interface{}{
        "model":    req.Model,
        "messages": req.Messages,
        "stream":   true,
    }

	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = *req.TopP
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	if a.org != "" {
		httpReq.Header.Set("OpenAI-Organization", a.org)
	}

	// Send request
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("openai: %s (type=%s, code=%s)", errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
		}
		return nil, fmt.Errorf("openai: http %d: %s", resp.StatusCode, string(respBody))
	}

	// Create channel for streaming events
	eventChan := make(chan adapter.StreamEvent, 10)

	// Start goroutine to read and parse SSE stream
	go func() {
		defer close(eventChan)
		defer resp.Body.Close()

		reader := io.Reader(resp.Body)
		buffer := make([]byte, 8192)
		leftover := ""

		for {
			// Check context cancellation
			select {
			case <-ctx.Done():
				eventChan <- adapter.StreamEvent{Error: ctx.Err()}
				return
			default:
			}

			// Read from stream
			n, err := reader.Read(buffer)
			if n > 0 {
				data := leftover + string(buffer[:n])
				lines := strings.Split(data, "\n")

				// Keep the last incomplete line for next iteration
				leftover = lines[len(lines)-1]
				lines = lines[:len(lines)-1]

				// Process complete lines
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" || !strings.HasPrefix(line, "data: ") {
						continue
					}

					payload := strings.TrimPrefix(line, "data: ")

					// Check for [DONE] signal
					if payload == "[DONE]" {
						return
					}

					// Parse chunk
					var chunk openai.ChatCompletionChunk
					if parseErr := json.Unmarshal([]byte(payload), &chunk); parseErr != nil {
						eventChan <- adapter.StreamEvent{Error: fmt.Errorf("openai: parse chunk: %w", parseErr)}
						return
					}

					// Send chunk event
					eventChan <- adapter.StreamEvent{Chunk: &chunk}
				}
			}

			if err != nil {
				if err == io.EOF {
					return
				}
				eventChan <- adapter.StreamEvent{Error: fmt.Errorf("openai: read stream: %w", err)}
				return
			}
		}
	}()

	return eventChan, nil
}

// CreateEmbedding sends an embedding request to OpenAI.
func (a *OpenAIAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	if req.Input == nil {
		return openai.EmbeddingResponse{}, errors.New("openai: input required")
	}

	// Build OpenAI API request
	payload := map[string]interface{}{
		"model": req.Model,
		"input": req.Input,
	}

	if req.EncodingFormat != "" {
		payload["encoding_format"] = req.EncodingFormat
	}
	if req.Dimensions != nil {
		payload["dimensions"] = *req.Dimensions
	}
	if req.User != "" {
		payload["user"] = req.User
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return openai.EmbeddingResponse{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return openai.EmbeddingResponse{}, fmt.Errorf("openai: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	if a.org != "" {
		httpReq.Header.Set("OpenAI-Organization", a.org)
	}

	// Send request
	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return openai.EmbeddingResponse{}, fmt.Errorf("openai: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.EmbeddingResponse{}, fmt.Errorf("openai: read response: %w", err)
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
			return openai.EmbeddingResponse{}, fmt.Errorf("openai: %s (type=%s, code=%s)", errResp.Error.Message, errResp.Error.Type, errResp.Error.Code)
		}
		return openai.EmbeddingResponse{}, fmt.Errorf("openai: http %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse successful response
	var embedding openai.EmbeddingResponse
	if err := json.Unmarshal(respBody, &embedding); err != nil {
		return openai.EmbeddingResponse{}, fmt.Errorf("openai: unmarshal response: %w", err)
	}

	return embedding, nil
}
