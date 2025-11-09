package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	openaitypes "github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/testutil"
)

func TestCreateCompletionStream_Success(t *testing.T) {
	// Mock SSE server
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if accept := r.Header.Get("Accept"); accept != "text/event-stream" {
			t.Errorf("Accept header = %q, want text/event-stream", accept)
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send chunks
		chunks := []string{
			`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	adpt, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.ChatCompletionRequest{
		Model: "gpt-4",
		Messages: []openaitypes.ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	eventChan, err := adpt.CreateCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletionStream() error = %v", err)
	}

	var content string
	eventCount := 0

	for event := range eventChan {
		eventCount++

		if event.IsError() {
			t.Fatalf("received error event: %v", event.Error)
		}

		if event.Chunk != nil {
			content += event.Chunk.GetDelta().Content
		}
	}

	// Verify we received chunks
	if eventCount == 0 {
		t.Fatal("expected to receive stream events")
	}

	// Verify content was accumulated
	if content != "Hello world" {
		t.Errorf("accumulated content = %q, want 'Hello world'", content)
	}
}

func TestCreateCompletionStream_EmptyMessages(t *testing.T) {
	adpt, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: "https://api.openai.com/v1",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openaitypes.ChatMessage{},
	}

	_, err = adpt.CreateCompletionStream(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletionStream() expected error for empty messages, got nil")
	}
	if !strings.Contains(err.Error(), "no messages") {
		t.Errorf("error = %v, want error containing 'no messages'", err)
	}
}

func TestCreateCompletionStream_ErrorResponse(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error","code":"invalid_api_key"}}`))
	}))
	defer server.Close()

	adpt, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openaitypes.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	_, err = adpt.CreateCompletionStream(context.Background(), req)
	if err == nil {
		t.Error("CreateCompletionStream() expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("error = %v, want error containing 'Invalid API key'", err)
	}
}

func TestCreateCompletionStream_ContextCancellation(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// Send initial chunk
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"test","object":"chat.completion.chunk","created":1234,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)
		flusher.Flush()

		// Wait to allow context cancellation
		time.Sleep(200 * time.Millisecond)

		// Try to send more chunks (should be cancelled)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	adpt, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := openaitypes.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openaitypes.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	eventChan, err := adpt.CreateCompletionStream(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletionStream() error = %v", err)
	}

	var gotCancellation bool
	for event := range eventChan {
		if event.IsError() {
			if strings.Contains(event.Error.Error(), "context") ||
				strings.Contains(event.Error.Error(), "deadline") {
				gotCancellation = true
			}
		}
	}

	if !gotCancellation {
		t.Error("expected to receive context cancellation error")
	}
}

func TestCreateCompletionStream_MalformedChunk(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		// Send valid chunk
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"test","object":"chat.completion.chunk","created":1234,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}`)
		flusher.Flush()

		// Send malformed chunk
		fmt.Fprintf(w, "data: {invalid json}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	adpt, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.ChatCompletionRequest{
		Model:    "gpt-4",
		Messages: []openaitypes.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	eventChan, err := adpt.CreateCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletionStream() error = %v", err)
	}

	var gotError bool
	for event := range eventChan {
		if event.IsError() {
			if strings.Contains(event.Error.Error(), "parse chunk") {
				gotError = true
			}
		}
	}

	if !gotError {
		t.Error("expected to receive parse error for malformed chunk")
	}
}

func TestStreamEvent_Helpers(t *testing.T) {
	t.Run("IsError", func(t *testing.T) {
		tests := []struct {
			name  string
			event adapter.StreamEvent
			want  bool
		}{
			{
				name:  "event with error",
				event: adapter.StreamEvent{Error: fmt.Errorf("test error")},
				want:  true,
			},
			{
				name:  "event with chunk",
				event: adapter.StreamEvent{Chunk: &openaitypes.ChatCompletionChunk{ID: "test"}},
				want:  false,
			},
			{
				name:  "empty event",
				event: adapter.StreamEvent{},
				want:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.event.IsError(); got != tt.want {
					t.Errorf("IsError() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("IsDone", func(t *testing.T) {
		tests := []struct {
			name  string
			event adapter.StreamEvent
			want  bool
		}{
			{
				name:  "done event",
				event: adapter.StreamEvent{},
				want:  true,
			},
			{
				name:  "event with chunk",
				event: adapter.StreamEvent{Chunk: &openaitypes.ChatCompletionChunk{ID: "test"}},
				want:  false,
			},
			{
				name:  "event with error",
				event: adapter.StreamEvent{Error: fmt.Errorf("test error")},
				want:  false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := tt.event.IsDone(); got != tt.want {
					t.Errorf("IsDone() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}
