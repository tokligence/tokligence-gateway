package fallback

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// mockAdapter simulates an adapter that can succeed or fail.
type mockAdapter struct {
	name         string
	shouldFail   bool
	failCount    int
	currentFails int32
	err          error
	delay        time.Duration
}

func (m *mockAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	// Check context cancellation first
	select {
	case <-ctx.Done():
		return openai.ChatCompletionResponse{}, ctx.Err()
	default:
	}

	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return openai.ChatCompletionResponse{}, ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.shouldFail {
		if m.failCount > 0 {
			current := atomic.AddInt32(&m.currentFails, 1)
			if int(current) <= m.failCount {
				return openai.ChatCompletionResponse{}, m.err
			}
		} else {
			return openai.ChatCompletionResponse{}, m.err
		}
	}

	return openai.ChatCompletionResponse{
		ID:     "test-" + m.name,
		Object: "chat.completion",
		Model:  req.Model,
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
	tests := []struct {
		name        string
		cfg         Config
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config",
			cfg: Config{
				Adapters:   []adapter.ChatAdapter{&mockAdapter{name: "test"}},
				RetryCount: 2,
				RetryDelay: 100 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "no adapters",
			cfg: Config{
				Adapters: []adapter.ChatAdapter{},
			},
			wantErr:     true,
			errContains: "at least one adapter required",
		},
		{
			name: "default retry values",
			cfg: Config{
				Adapters: []adapter.ChatAdapter{&mockAdapter{name: "test"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fa, err := New(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("New() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("New() unexpected error = %v", err)
				return
			}
			if fa == nil {
				t.Error("New() returned nil")
			}
		})
	}
}

func TestCreateCompletion_Success(t *testing.T) {
	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{&mockAdapter{name: "primary"}},
		RetryCount: 2,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	resp, err := fa.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if !strings.Contains(resp.ID, "primary") {
		t.Errorf("expected response from primary adapter, got ID %q", resp.ID)
	}
}

func TestCreateCompletion_Fallback(t *testing.T) {
	primary := &mockAdapter{
		name:       "primary",
		shouldFail: true,
		err:        errors.New("primary failed"),
	}
	fallback := &mockAdapter{
		name: "fallback",
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{primary, fallback},
		RetryCount: 1,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	resp, err := fa.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if !strings.Contains(resp.ID, "fallback") {
		t.Errorf("expected response from fallback adapter, got ID %q", resp.ID)
	}
}

func TestCreateCompletion_MultipleFallbacks(t *testing.T) {
	adapter1 := &mockAdapter{
		name:       "adapter1",
		shouldFail: true,
		err:        errors.New("adapter1 failed"),
	}
	adapter2 := &mockAdapter{
		name:       "adapter2",
		shouldFail: true,
		err:        errors.New("adapter2 failed"),
	}
	adapter3 := &mockAdapter{
		name: "adapter3",
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{adapter1, adapter2, adapter3},
		RetryCount: 1,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	resp, err := fa.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if !strings.Contains(resp.ID, "adapter3") {
		t.Errorf("expected response from adapter3, got ID %q", resp.ID)
	}
}

func TestCreateCompletion_AllFail(t *testing.T) {
	adapter1 := &mockAdapter{
		name:       "adapter1",
		shouldFail: true,
		err:        errors.New("adapter1 failed"),
	}
	adapter2 := &mockAdapter{
		name:       "adapter2",
		shouldFail: true,
		err:        errors.New("adapter2 failed"),
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{adapter1, adapter2},
		RetryCount: 1,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	_, err = fa.CreateCompletion(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletion() expected error when all adapters fail, got nil")
	}
	if !strings.Contains(err.Error(), "all adapters failed") {
		t.Errorf("error = %v, want error containing 'all adapters failed'", err)
	}
}

func TestCreateCompletion_RetrySuccess(t *testing.T) {
	// Adapter fails first 2 times, succeeds on 3rd
	primary := &mockAdapter{
		name:       "primary",
		shouldFail: true,
		failCount:  2,
		err:        errors.New("timeout"),
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{primary},
		RetryCount: 3,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	resp, err := fa.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	if !strings.Contains(resp.ID, "primary") {
		t.Errorf("expected response from primary adapter after retry, got ID %q", resp.ID)
	}
}

func TestCreateCompletion_ContextCancellation(t *testing.T) {
	slow := &mockAdapter{
		name:  "slow",
		delay: 200 * time.Millisecond,
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{slow},
		RetryCount: 2,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	_, err = fa.CreateCompletion(ctx, req)
	if err == nil {
		t.Error("CreateCompletion() expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"timeout", errors.New("timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"429 error", errors.New("http 429"), true},
		{"500 error", errors.New("http 500 internal server error"), true},
		{"502 error", errors.New("502 bad gateway"), true},
		{"503 error", errors.New("503 service unavailable"), true},
		{"504 error", errors.New("504 gateway timeout"), true},
		{"400 error", errors.New("http 400 bad request"), false},
		{"401 error", errors.New("http 401 unauthorized"), false},
		{"403 error", errors.New("http 403 forbidden"), false},
		{"404 error", errors.New("http 404 not found"), false},
		{"invalid api key", errors.New("invalid api key"), false},
		{"unknown error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestCreateCompletion_NonRetryableError(t *testing.T) {
	primary := &mockAdapter{
		name:       "primary",
		shouldFail: true,
		err:        errors.New("401 unauthorized"),
	}
	fallback := &mockAdapter{
		name: "fallback",
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{primary, fallback},
		RetryCount: 3,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	resp, err := fa.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	// Should immediately fallback without retrying
	if !strings.Contains(resp.ID, "fallback") {
		t.Errorf("expected immediate fallback for non-retryable error, got ID %q", resp.ID)
	}
}

func TestCreateCompletion_RetryDelay(t *testing.T) {
	primary := &mockAdapter{
		name:       "primary",
		shouldFail: true,
		failCount:  1,
		err:        errors.New("timeout"),
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{primary},
		RetryCount: 2,
		RetryDelay: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "test-model",
		Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
	}

	start := time.Now()
	resp, err := fa.CreateCompletion(context.Background(), req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}

	// Should have waited at least one retry delay
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected delay of at least 100ms, got %v", elapsed)
	}

	if !strings.Contains(resp.ID, "primary") {
		t.Errorf("expected response from primary after retry, got ID %q", resp.ID)
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "WORLD", true}, // case insensitive
		{"HELLO WORLD", "world", true}, // case insensitive
		{"hello world", "foo", false},
		{"", "", true},
		{"hello", "", true},
		{"", "hello", false},
		{"timeout error", "timeout", true},
		{"HTTP 429 Too Many Requests", "429", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_contains_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestConcurrentRequests(t *testing.T) {
	primary := &mockAdapter{
		name:       "primary",
		shouldFail: true,
		failCount:  1,
		err:        errors.New("timeout"),
	}

	fa, err := New(Config{
		Adapters:   []adapter.ChatAdapter{primary},
		RetryCount: 2,
		RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req := openai.ChatCompletionRequest{
				Model:    "test-model",
				Messages: []openai.ChatMessage{{Role: "user", Content: "test"}},
			}
			_, err := fa.CreateCompletion(context.Background(), req)
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
