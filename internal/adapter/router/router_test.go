package router

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// mockAdapter is a mock implementation of ChatAdapter for testing.
type mockAdapter struct {
	name string
	err  error
}

func (m *mockAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if m.err != nil {
		return openai.ChatCompletionResponse{}, m.err
	}
	return openai.ChatCompletionResponse{
		ID:     "test-" + m.name,
		Model:  req.Model,
		Object: "chat.completion",
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatMessage{
					Role:    "assistant",
					Content: "Response from " + m.name,
				},
			},
		},
	}, nil
}

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.adapters == nil {
		t.Error("adapters map is nil")
	}
	if r.routes == nil {
		t.Error("routes map is nil")
	}
}

func TestRegisterAdapter(t *testing.T) {
	tests := []struct {
		name        string
		adapterName string
		adapter     adapter.ChatAdapter
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid adapter",
			adapterName: "openai",
			adapter:     &mockAdapter{name: "openai"},
			wantErr:     false,
		},
		{
			name:        "empty adapter name",
			adapterName: "",
			adapter:     &mockAdapter{name: "test"},
			wantErr:     true,
			errContains: "name cannot be empty",
		},
		{
			name:        "nil adapter",
			adapterName: "test",
			adapter:     nil,
			wantErr:     true,
			errContains: "adapter cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			err := r.RegisterAdapter(tt.adapterName, tt.adapter)
			if tt.wantErr {
				if err == nil {
					t.Error("RegisterAdapter() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("RegisterAdapter() unexpected error = %v", err)
			}
		})
	}
}

func TestRegisterRoute(t *testing.T) {
	tests := []struct {
		name          string
		setupAdapters map[string]adapter.ChatAdapter
		pattern       string
		adapterName   string
		wantErr       bool
		errContains   string
	}{
		{
			name: "valid route",
			setupAdapters: map[string]adapter.ChatAdapter{
				"openai": &mockAdapter{name: "openai"},
			},
			pattern:     "gpt-4",
			adapterName: "openai",
			wantErr:     false,
		},
		{
			name: "empty pattern",
			setupAdapters: map[string]adapter.ChatAdapter{
				"openai": &mockAdapter{name: "openai"},
			},
			pattern:     "",
			adapterName: "openai",
			wantErr:     true,
			errContains: "pattern cannot be empty",
		},
		{
			name: "empty adapter name",
			setupAdapters: map[string]adapter.ChatAdapter{
				"openai": &mockAdapter{name: "openai"},
			},
			pattern:     "gpt-4",
			adapterName: "",
			wantErr:     true,
			errContains: "adapter name cannot be empty",
		},
		{
			name: "adapter not registered",
			setupAdapters: map[string]adapter.ChatAdapter{
				"openai": &mockAdapter{name: "openai"},
			},
			pattern:     "gpt-4",
			adapterName: "anthropic",
			wantErr:     true,
			errContains: "adapter \"anthropic\" not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			for name, adapter := range tt.setupAdapters {
				r.RegisterAdapter(name, adapter)
			}

			err := r.RegisterRoute(tt.pattern, tt.adapterName)
			if tt.wantErr {
				if err == nil {
					t.Error("RegisterRoute() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("RegisterRoute() unexpected error = %v", err)
			}
		})
	}
}

func TestCreateCompletion_ExactMatch(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterAdapter("anthropic", &mockAdapter{name: "anthropic"})
	r.RegisterRoute("gpt-4", "openai")
	r.RegisterRoute("claude-3-sonnet", "anthropic")

	tests := []struct {
		model       string
		wantAdapter string
	}{
		{"gpt-4", "openai"},
		{"claude-3-sonnet", "anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			req := openai.ChatCompletionRequest{
				Model:    tt.model,
				Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
			}

			resp, err := r.CreateCompletion(context.Background(), req)
			if err != nil {
				t.Fatalf("CreateCompletion() error = %v", err)
			}

			if !strings.Contains(resp.ID, tt.wantAdapter) {
				t.Errorf("response ID = %q, want to contain %q", resp.ID, tt.wantAdapter)
			}
		})
	}
}

func TestCreateCompletion_PrefixMatch(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterRoute("gpt-*", "openai")

	tests := []struct {
		model string
	}{
		{"gpt-4"},
		{"gpt-3.5-turbo"},
		{"gpt-4-turbo"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			req := openai.ChatCompletionRequest{
				Model:    tt.model,
				Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
			}

			resp, err := r.CreateCompletion(context.Background(), req)
			if err != nil {
				t.Fatalf("CreateCompletion() error = %v", err)
			}

			if !strings.Contains(resp.ID, "openai") {
				t.Errorf("expected response from openai adapter, got ID %q", resp.ID)
			}
		})
	}
}

func TestCreateCompletion_SuffixMatch(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterRoute("*-turbo", "openai")

	tests := []struct {
		model string
	}{
		{"gpt-3.5-turbo"},
		{"gpt-4-turbo"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			req := openai.ChatCompletionRequest{
				Model:    tt.model,
				Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
			}

			resp, err := r.CreateCompletion(context.Background(), req)
			if err != nil {
				t.Fatalf("CreateCompletion() error = %v", err)
			}

			if !strings.Contains(resp.ID, "openai") {
				t.Errorf("expected response from openai adapter, got ID %q", resp.ID)
			}
		})
	}
}

func TestCreateCompletion_ContainsMatch(t *testing.T) {
	r := New()
	r.RegisterAdapter("anthropic", &mockAdapter{name: "anthropic"})
	r.RegisterRoute("*claude*", "anthropic")

	tests := []struct {
		model string
	}{
		{"claude-3-opus"},
		{"claude-3-sonnet"},
		{"claude-instant"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			req := openai.ChatCompletionRequest{
				Model:    tt.model,
				Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
			}

			resp, err := r.CreateCompletion(context.Background(), req)
			if err != nil {
				t.Fatalf("CreateCompletion() error = %v", err)
			}

			if !strings.Contains(resp.ID, "anthropic") {
				t.Errorf("expected response from anthropic adapter, got ID %q", resp.ID)
			}
		})
	}
}

func TestCreateCompletion_Fallback(t *testing.T) {
	r := New()
	fallbackAdapter := &mockAdapter{name: "fallback"}
	r.RegisterAdapter("fallback", fallbackAdapter)
	r.SetFallback(fallbackAdapter)

	req := openai.ChatCompletionRequest{
		Model:    "unknown-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	resp, err := r.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if !strings.Contains(resp.ID, "fallback") {
		t.Errorf("expected response from fallback adapter, got ID %q", resp.ID)
	}
}

func TestCreateCompletion_NoMatch(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterRoute("gpt-4", "openai")

	req := openai.ChatCompletionRequest{
		Model:    "unknown-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	_, err := r.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected error for unknown model, got nil")
	}
	if !strings.Contains(err.Error(), "no adapter found") {
		t.Errorf("error = %v, want error containing 'no adapter found'", err)
	}
}

func TestCreateCompletion_EmptyModel(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})

	req := openai.ChatCompletionRequest{
		Model:    "",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	_, err := r.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected error for empty model, got nil")
	}
	if !strings.Contains(err.Error(), "model name required") {
		t.Errorf("error = %v, want error containing 'model name required'", err)
	}
}

func TestCreateCompletion_AdapterError(t *testing.T) {
	r := New()
	expectedErr := errors.New("adapter error")
	r.RegisterAdapter("failing", &mockAdapter{name: "failing", err: expectedErr})
	r.RegisterRoute("failing-model", "failing")

	req := openai.ChatCompletionRequest{
		Model:    "failing-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	_, err := r.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected error, got nil")
		return
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		model   string
		pattern string
		want    bool
	}{
		// Exact match
		{"gpt-4", "gpt-4", true},
		{"gpt-4", "gpt-3", false},

		// Prefix match
		{"gpt-4", "gpt-*", true},
		{"gpt-3.5-turbo", "gpt-*", true},
		{"claude-3", "gpt-*", false},

		// Suffix match
		{"gpt-3.5-turbo", "*-turbo", true},
		{"gpt-4-turbo", "*-turbo", true},
		{"gpt-4", "*-turbo", false},

		// Contains match
		{"claude-3-opus", "*claude*", true},
		{"claude-instant", "*claude*", true},
		{"gpt-4", "*claude*", false},

		// Case insensitive
		{"GPT-4", "gpt-*", true},
		{"gpt-4", "GPT-*", true},

		// No wildcards
		{"gpt-4", "gpt-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.model+"_vs_"+tt.pattern, func(t *testing.T) {
			got := matchPattern(tt.model, tt.pattern)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.model, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestGetAdapterForModel(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterAdapter("anthropic", &mockAdapter{name: "anthropic"})
	r.RegisterRoute("gpt-*", "openai")
	r.RegisterRoute("claude-*", "anthropic")

	tests := []struct {
		model       string
		wantAdapter string
		wantErr     bool
	}{
		{"gpt-4", "openai", false},
		{"gpt-3.5-turbo", "openai", false},
		{"claude-3-opus", "anthropic", false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			adapter, err := r.GetAdapterForModel(tt.model)
			if tt.wantErr {
				if err == nil {
					t.Error("GetAdapterForModel() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("GetAdapterForModel() unexpected error = %v", err)
				return
			}
			if adapter != tt.wantAdapter {
				t.Errorf("GetAdapterForModel() = %q, want %q", adapter, tt.wantAdapter)
			}
		})
	}
}

func TestListAdapters(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterAdapter("anthropic", &mockAdapter{name: "anthropic"})
	r.RegisterAdapter("loopback", &mockAdapter{name: "loopback"})

	adapters := r.ListAdapters()
	if len(adapters) != 3 {
		t.Errorf("ListAdapters() returned %d adapters, want 3", len(adapters))
	}

	expected := map[string]bool{"openai": true, "anthropic": true, "loopback": true}
	for _, name := range adapters {
		if !expected[name] {
			t.Errorf("unexpected adapter name: %q", name)
		}
	}
}

func TestListRoutes(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterAdapter("anthropic", &mockAdapter{name: "anthropic"})
	r.RegisterRoute("gpt-*", "openai")
	r.RegisterRoute("claude-*", "anthropic")

	routes := r.ListRoutes()
	if len(routes) != 2 {
		t.Errorf("ListRoutes() returned %d routes, want 2", len(routes))
	}

	if routes["gpt-*"] != "openai" {
		t.Errorf("routes[gpt-*] = %q, want openai", routes["gpt-*"])
	}
	if routes["claude-*"] != "anthropic" {
		t.Errorf("routes[claude-*] = %q, want anthropic", routes["claude-*"])
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := New()
	r.RegisterAdapter("openai", &mockAdapter{name: "openai"})
	r.RegisterRoute("gpt-*", "openai")

	// Simulate concurrent requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			req := openai.ChatCompletionRequest{
				Model:    "gpt-4",
				Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
			}
			_, err := r.CreateCompletion(context.Background(), req)
			if err != nil {
				t.Errorf("CreateCompletion() error = %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
