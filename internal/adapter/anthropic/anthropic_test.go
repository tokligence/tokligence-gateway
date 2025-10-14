package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
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
				APIKey:         "sk-ant-test123",
				BaseURL:        "https://api.anthropic.com",
				Version:        "2023-06-01",
				RequestTimeout: 30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			cfg: Config{
				APIKey: "sk-ant-test123",
			},
			wantErr: false,
		},
		{
			name: "missing api key",
			cfg: Config{
				BaseURL: "https://api.anthropic.com",
			},
			wantErr: true,
			errMsg:  "api key required",
		},
		{
			name:    "empty api key",
			cfg:     Config{APIKey: ""},
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if key := r.Header.Get("x-api-key"); key == "" {
			t.Error("missing x-api-key header")
		}
		if version := r.Header.Get("anthropic-version"); version == "" {
			t.Error("missing anthropic-version header")
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
		if _, ok := reqBody["max_tokens"]; !ok {
			t.Error("request missing 'max_tokens' field")
		}

		// Return mock Anthropic response
		response := anthropicResponse{
			ID:   "msg_test123",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{
					Type: "text",
					Text: "Hello! I'm Claude, an AI assistant. How can I help you today?",
				},
			},
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "end_turn",
			Usage: anthropicUsage{
				InputTokens:  10,
				OutputTokens: 20,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-ant-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model: "claude-3-sonnet",
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
	if resp.Object != "chat.completion" {
		t.Errorf("response.Object = %q, want chat.completion", resp.Object)
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
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("response.Usage.PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 20 {
		t.Errorf("response.Usage.CompletionTokens = %d, want 20", resp.Usage.CompletionTokens)
	}
}

func TestCreateCompletion_WithSystemMessage(t *testing.T) {
	var capturedSystem string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Capture system prompt
		if sys, ok := reqBody["system"].(string); ok {
			capturedSystem = sys
		}

		response := anthropicResponse{
			ID:   "msg_test",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{Type: "text", Text: "Response"},
			},
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 5},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-ant-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model: "claude-3-sonnet",
		Messages: []openai.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if capturedSystem != "You are a helpful assistant." {
		t.Errorf("system prompt = %q, want 'You are a helpful assistant.'", capturedSystem)
	}
}

func TestCreateCompletion_EmptyMessages(t *testing.T) {
	adapter, err := New(Config{
		APIKey:  "sk-ant-test123",
		BaseURL: "https://api.anthropic.com",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "claude-3-sonnet",
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

func TestCreateCompletion_OnlySystemMessages(t *testing.T) {
	adapter, err := New(Config{
		APIKey:  "sk-ant-test123",
		BaseURL: "https://api.anthropic.com",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model: "claude-3-sonnet",
		Messages: []openai.ChatMessage{
			{Role: "system", Content: "You are a helpful assistant."},
		},
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected error for only system messages, got nil")
	}
	if !strings.Contains(err.Error(), "no user/assistant messages") {
		t.Errorf("error = %v, want error containing 'no user/assistant messages'", err)
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
				"type": "error",
				"error": map[string]interface{}{
					"type":    "authentication_error",
					"message": "Invalid API key",
				},
			},
			wantErrMsg: "Invalid API key",
		},
		{
			name:       "429 rate limit",
			statusCode: http.StatusTooManyRequests,
			response: map[string]interface{}{
				"type": "error",
				"error": map[string]interface{}{
					"type":    "rate_limit_error",
					"message": "Rate limit exceeded",
				},
			},
			wantErrMsg: "Rate limit exceeded",
		},
		{
			name:       "500 server error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]interface{}{"error": "internal error"},
			wantErrMsg: "http 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter, err := New(Config{
				APIKey:  "sk-ant-test123",
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			req := openai.ChatCompletionRequest{
				Model: "claude-3-sonnet",
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

func TestMapModelName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet-20241022"},
		{"claude-3-opus-20240229", "claude-3-opus-20240229"},
		{"claude-sonnet", "claude-3-5-sonnet-20241022"},
		{"claude-3-sonnet", "claude-3-5-sonnet-20241022"},
		{"claude-haiku", "claude-3-5-haiku-20241022"},
		{"claude-3-haiku", "claude-3-5-haiku-20241022"},
		{"claude", "claude-3-opus-20240229"},
		{"claude-3", "claude-3-opus-20240229"},
		{"unknown-model", "claude-3-5-sonnet-20241022"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapModelName(tt.input)
			if got != tt.want {
				t.Errorf("mapModelName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConvertMessages(t *testing.T) {
	tests := []struct {
		name           string
		input          []openai.ChatMessage
		wantMsgCount   int
		wantSystem     string
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "user and assistant messages",
			input: []openai.ChatMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
			wantMsgCount: 2,
			wantSystem:   "",
			wantErr:      false,
		},
		{
			name: "with system message",
			input: []openai.ChatMessage{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello"},
			},
			wantMsgCount: 1,
			wantSystem:   "You are helpful.",
			wantErr:      false,
		},
		{
			name: "multiple system messages",
			input: []openai.ChatMessage{
				{Role: "system", Content: "First system."},
				{Role: "system", Content: "Second system."},
				{Role: "user", Content: "Hello"},
			},
			wantMsgCount: 1,
			wantSystem:   "First system.\n\nSecond system.",
			wantErr:      false,
		},
		{
			name: "only system messages",
			input: []openai.ChatMessage{
				{Role: "system", Content: "You are helpful."},
			},
			wantErr:        true,
			wantErrContain: "no user/assistant messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, system, err := convertMessages(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("convertMessages() expected error, got nil")
					return
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error = %v, want error containing %q", err, tt.wantErrContain)
				}
				return
			}
			if err != nil {
				t.Errorf("convertMessages() unexpected error = %v", err)
				return
			}
			if len(messages) != tt.wantMsgCount {
				t.Errorf("len(messages) = %d, want %d", len(messages), tt.wantMsgCount)
			}
			if system != tt.wantSystem {
				t.Errorf("system = %q, want %q", system, tt.wantSystem)
			}
		})
	}
}

func TestCreateCompletion_WithOptionalParams(t *testing.T) {
	temperature := 0.7
	topP := 0.9
	var capturedTemp, capturedTopP float64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Capture optional parameters
		if temp, ok := reqBody["temperature"].(float64); ok {
			capturedTemp = temp
		}
		if tp, ok := reqBody["top_p"].(float64); ok {
			capturedTopP = tp
		}

		response := anthropicResponse{
			ID:         "msg_test",
			Type:       "message",
			Role:       "assistant",
			Content:    []anthropicContentBlock{{Type: "text", Text: "Test"}},
			Model:      "claude-3-5-sonnet-20241022",
			StopReason: "end_turn",
			Usage:      anthropicUsage{InputTokens: 5, OutputTokens: 5},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-ant-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:       "claude-3-sonnet",
		Messages:    []openai.ChatMessage{{Role: "user", Content: "Hello"}},
		Temperature: &temperature,
		TopP:        &topP,
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Errorf("CreateCompletion() error = %v", err)
	}

	if capturedTemp != temperature {
		t.Errorf("temperature = %f, want %f", capturedTemp, temperature)
	}
	if capturedTopP != topP {
		t.Errorf("top_p = %f, want %f", capturedTopP, topP)
	}
}

func TestCreateCompletion_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:         "sk-ant-test123",
		BaseURL:        server.URL,
		RequestTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "claude-3-sonnet",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err = adapter.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected timeout error, got nil")
	}
}

func TestCreateCompletion_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-ant-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := openai.ChatCompletionRequest{
		Model:    "claude-3-sonnet",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err = adapter.CreateCompletion(ctx, req)
	if err == nil {
		t.Error("CreateCompletion() expected context cancellation error, got nil")
	}
}
