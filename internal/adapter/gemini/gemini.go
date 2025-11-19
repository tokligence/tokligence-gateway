package gemini

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
)

// GeminiAdapter provides pass-through proxy for Google Gemini API.
// This adapter forwards requests directly to Gemini API without translation.
type GeminiAdapter struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// Config holds configuration for the Gemini adapter.
type Config struct {
	APIKey         string
	BaseURL        string // optional, defaults to https://generativelanguage.googleapis.com
	RequestTimeout time.Duration
}

// New creates a GeminiAdapter instance.
func New(cfg Config) (*GeminiAdapter, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("gemini: api key required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = 120 * time.Second // Gemini may need more time for generation
	}

	return &GeminiAdapter{
		apiKey:  cfg.APIKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// GenerateContent sends a request to Gemini's generateContent endpoint.
// This is a pass-through that returns the raw Gemini response.
func (a *GeminiAdapter) GenerateContent(ctx context.Context, model string, reqBody []byte) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("gemini: model name required")
	}

	// Build URL: /v1beta/{model=models/*}:generateContent
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", a.baseURL, model, a.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: http %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// StreamGenerateContent sends a streaming request to Gemini's streamGenerateContent endpoint.
// Returns a channel that emits SSE events from Gemini.
func (a *GeminiAdapter) StreamGenerateContent(ctx context.Context, model string, reqBody []byte) (<-chan StreamEvent, error) {
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("gemini: model name required")
	}

	// Build URL: /v1beta/{model=models/*}:streamGenerateContent
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?key=%s&alt=sse", a.baseURL, model, a.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: create stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send stream request: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: stream http %d: %s", resp.StatusCode, string(respBody))
	}

	// Create channel for streaming events
	eventChan := make(chan StreamEvent, 10)

	// Start goroutine to read and forward SSE stream
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
				eventChan <- StreamEvent{Error: ctx.Err()}
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
					if line == "" {
						continue
					}

					// Gemini SSE format: "data: {json}"
					if strings.HasPrefix(line, "data: ") {
						payload := strings.TrimPrefix(line, "data: ")
						eventChan <- StreamEvent{Data: []byte(payload)}
					}
				}
			}

			if err != nil {
				if err == io.EOF {
					return
				}
				eventChan <- StreamEvent{Error: fmt.Errorf("gemini: read stream: %w", err)}
				return
			}
		}
	}()

	return eventChan, nil
}

// StreamEvent represents a single event in a streaming response.
type StreamEvent struct {
	Data  []byte // Raw JSON data from SSE event
	Error error
}

// IsError checks if this event contains an error.
func (e StreamEvent) IsError() bool {
	return e.Error != nil
}

// IsDone checks if this is a stream completion event (no data, no error).
func (e StreamEvent) IsDone() bool {
	return e.Data == nil && e.Error == nil
}

// CountTokens sends a request to Gemini's countTokens endpoint.
func (a *GeminiAdapter) CountTokens(ctx context.Context, model string, reqBody []byte) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("gemini: model name required")
	}

	// Build URL: /v1beta/{model=models/*}:countTokens
	url := fmt.Sprintf("%s/v1beta/models/%s:countTokens?key=%s", a.baseURL, model, a.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: http %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ListModels retrieves available models from Gemini API.
func (a *GeminiAdapter) ListModels(ctx context.Context) ([]byte, error) {
	// Build URL: /v1beta/models
	url := fmt.Sprintf("%s/v1beta/models?key=%s", a.baseURL, a.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: http %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetModel retrieves metadata for a specific model.
func (a *GeminiAdapter) GetModel(ctx context.Context, model string) ([]byte, error) {
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("gemini: model name required")
	}

	// Build URL: /v1beta/{name=models/*}
	url := fmt.Sprintf("%s/v1beta/models/%s?key=%s", a.baseURL, model, a.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: http %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// OpenAIChatCompletion sends a request to Gemini's OpenAI-compatible endpoint
// This allows using OpenAI SDK format while using Gemini models
func (a *GeminiAdapter) OpenAIChatCompletion(ctx context.Context, reqBody []byte) ([]byte, error) {
	// Build URL: /v1beta/openai/chat/completions (no key parameter)
	url := fmt.Sprintf("%s/v1beta/openai/chat/completions", a.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	// OpenAI-compatible endpoint uses Bearer token authentication
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gemini: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: http %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// OpenAIChatCompletionStream handles streaming for OpenAI-compatible endpoint
func (a *GeminiAdapter) OpenAIChatCompletionStream(ctx context.Context, reqBody []byte) (<-chan StreamEvent, error) {
	// Build URL: /v1beta/openai/chat/completions (no key parameter)
	url := fmt.Sprintf("%s/v1beta/openai/chat/completions", a.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gemini: create stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	// OpenAI-compatible endpoint uses Bearer token authentication
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini: send stream request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("gemini: %s (code=%d, status=%s)", errResp.Error.Message, errResp.Error.Code, errResp.Error.Status)
		}
		return nil, fmt.Errorf("gemini: stream http %d: %s", resp.StatusCode, string(respBody))
	}

	eventChan := make(chan StreamEvent, 10)

	go func() {
		defer close(eventChan)
		defer resp.Body.Close()

		reader := io.Reader(resp.Body)
		buffer := make([]byte, 8192)
		leftover := ""

		for {
			select {
			case <-ctx.Done():
				eventChan <- StreamEvent{Error: ctx.Err()}
				return
			default:
			}

			n, err := reader.Read(buffer)
			if n > 0 {
				data := leftover + string(buffer[:n])
				lines := strings.Split(data, "\n")

				leftover = lines[len(lines)-1]
				lines = lines[:len(lines)-1]

				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					if strings.HasPrefix(line, "data: ") {
						payload := strings.TrimPrefix(line, "data: ")
						if payload == "[DONE]" {
							return
						}
						eventChan <- StreamEvent{Data: []byte(payload)}
					}
				}
			}

			if err != nil {
				if err == io.EOF {
					return
				}
				eventChan <- StreamEvent{Error: fmt.Errorf("gemini: read stream: %w", err)}
				return
			}
		}
	}()

	return eventChan, nil
}
