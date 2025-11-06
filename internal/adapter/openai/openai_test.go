package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/testutil"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with all fields",
			cfg: Config{
				APIKey:         "sk-test123",
				BaseURL:        "https://api.openai.com/v1",
				Organization:   "org-123",
				RequestTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			cfg: Config{
				APIKey: "sk-test123",
			},
			wantErr: false,
		},
		{
			name: "missing api key",
			cfg: Config{
				BaseURL: "https://api.openai.com/v1",
			},
			wantErr: true,
			errMsg:  "api key required",
		},
		{
			name: "empty api key",
			cfg: Config{
				APIKey: "",
			},
			wantErr: true,
			errMsg:  "api key required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, err := New(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("New() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("New() error = %v, want error containing %q", err, tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("New() unexpected error = %v", err)
				return
			}
			if adapter == nil {
				t.Error("New() returned nil adapter")
				return
			}
			if adapter.apiKey != tt.cfg.APIKey {
				t.Errorf("adapter.apiKey = %q, want %q", adapter.apiKey, tt.cfg.APIKey)
			}
		})
	}
}

func TestCreateCompletion_Success(t *testing.T) {
	// Mock OpenAI server
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("missing or invalid Authorization header: %q", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verify required fields
		if _, ok := reqBody["model"]; !ok {
			t.Error("request missing 'model' field")
		}
		if _, ok := reqBody["messages"]; !ok {
			t.Error("request missing 'messages' field")
		}

		// Return mock response
		response := openai.ChatCompletionResponse{
			ID:      "chatcmpl-test123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []openai.ChatCompletionChoice{
				{
					Index:        0,
					FinishReason: "stop",
					Message: openai.ChatMessage{
						Role:    "assistant",
						Content: "Hello! How can I help you today?",
					},
				},
			},
			Usage: openai.UsageBreakdown{
				PromptTokens:     10,
				CompletionTokens: 9,
				TotalTokens:      19,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create adapter
	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Test request
	req := openai.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openai.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	resp, err := adapter.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	// Verify response
	if resp.ID == "" {
		t.Error("response missing ID")
	}
	if resp.Model != "gpt-4" {
		t.Errorf("response.Model = %q, want gpt-4", resp.Model)
	}
	if len(resp.Choices) == 0 {
		t.Fatal("response has no choices")
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("choice.Message.Role = %q, want assistant", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].Message.Content == "" {
		t.Error("choice.Message.Content is empty")
	}
	if resp.Usage.TotalTokens == 0 {
		t.Error("response.Usage.TotalTokens is 0")
	}
}

func TestCreateCompletion_EmptyMessages(t *testing.T) {
	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: "https://api.openai.com/v1",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openai.ChatMessage{},
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected error for empty messages, got nil")
	}
	if !strings.Contains(err.Error(), "no messages") {
		t.Errorf("error = %v, want error containing 'no messages'", err)
	}
}

func TestCreateCompletion_APIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErrMsg string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Incorrect API key provided",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			},
			wantErrMsg: "Incorrect API key provided",
		},
		{
			name:       "429 rate limit",
			statusCode: http.StatusTooManyRequests,
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Rate limit exceeded",
					"type":    "rate_limit_error",
				},
			},
			wantErrMsg: "Rate limit exceeded",
		},
		{
			name:       "500 server error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]interface{}{"error": "internal server error"},
			wantErrMsg: "http 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter, err := New(Config{
				APIKey:  "sk-test123",
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			req := openai.ChatCompletionRequest{
				Model: "gpt-4",
				Messages: []openai.ChatMessage{
					{Role: "user", Content: "Hello"},
				},
			}

			_, err = adapter.CreateCompletion(context.Background(), req)
			if err == nil {
				t.Error("CreateCompletion() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error = %v, want error containing %q", err, tt.wantErrMsg)
			}
		})
	}
}

func TestCreateCompletion_WithOptionalParams(t *testing.T) {
	temperature := 0.7
	topP := 0.9

	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verify optional parameters were passed
		if temp, ok := reqBody["temperature"].(float64); !ok || temp != temperature {
			t.Errorf("temperature = %v, want %f", reqBody["temperature"], temperature)
		}
		if tp, ok := reqBody["top_p"].(float64); !ok || tp != topP {
			t.Errorf("top_p = %v, want %f", reqBody["top_p"], topP)
		}

		response := openai.ChatCompletionResponse{
			ID:      "test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []openai.ChatCompletionChoice{
				{
					Index:        0,
					FinishReason: "stop",
					Message:      openai.ChatMessage{Role: "assistant", Content: "Test"},
				},
			},
			Usage: openai.UsageBreakdown{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:       "gpt-4",
		Messages:    []openai.ChatMessage{{Role: "user", Content: "Hello"}},
		Temperature: &temperature,
		TopP:        &topP,
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Errorf("CreateCompletion() error = %v", err)
	}
}

func TestCreateCompletion_Timeout(t *testing.T) {
	// Server that delays response
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:         "sk-test123",
		BaseURL:        server.URL,
		RequestTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected timeout error, got nil")
	}
}

func TestCreateCompletion_ContextCancellation(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay to allow context cancellation
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := openai.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err = adapter.CreateCompletion(ctx, req)
	if err == nil {
		t.Error("CreateCompletion() expected context cancellation error, got nil")
	}
}

func TestCreateCompletion_OrganizationHeader(t *testing.T) {
	orgID := "org-test123"

	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if org := r.Header.Get("OpenAI-Organization"); org != orgID {
			t.Errorf("OpenAI-Organization = %q, want %q", org, orgID)
		}

		response := openai.ChatCompletionResponse{
			ID:      "test",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4",
			Choices: []openai.ChatCompletionChoice{
				{Message: openai.ChatMessage{Role: "assistant", Content: "Test"}},
			},
			Usage: openai.UsageBreakdown{TotalTokens: 10},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:       "sk-test123",
		BaseURL:      server.URL,
		Organization: orgID,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Errorf("CreateCompletion() error = %v", err)
	}
}
