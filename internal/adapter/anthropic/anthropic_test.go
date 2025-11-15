package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	translatorpkg "github.com/tokligence/openai-anthropic-endpoint-translation/pkg/translator"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func TestNew(t *testing.T) {
	t.Run("missing key", func(t *testing.T) {
		if _, err := New(Config{}); err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		adapter, err := New(Config{APIKey: "sk-test"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if adapter.baseURL != "https://api.anthropic.com" {
			t.Fatalf("base url = %s", adapter.baseURL)
		}
		if adapter.version != "2023-06-01" {
			t.Fatalf("version = %s", adapter.version)
		}
	})
}

func TestCreateCompletion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing api key header")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] == "" {
			t.Fatalf("missing model field")
		}
		resp := translatorpkg.AnthropicResponse{
			ID:    "msg_abc",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-3-sonnet",
			Content: []translatorpkg.ContentBlock{
				translatorpkg.NewTextBlock("Hello from Claude"),
			},
			StopReason: "end_turn",
			Usage:      translatorpkg.AnthropicUsage{InputTokens: 11, OutputTokens: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	adapter, err := New(Config{APIKey: "sk-test", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	req := openai.ChatCompletionRequest{
		Model:    "claude-3-sonnet",
		Messages: []openai.ChatMessage{{Role: "system", Content: "You are Claude"}, {Role: "user", Content: "Hello"}},
	}

	resp, err := adapter.CreateCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected one choice")
	}
	if !strings.Contains(resp.Choices[0].Message.Content, "Claude") {
		t.Fatalf("unexpected content: %s", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 11 || resp.Usage.CompletionTokens != 5 {
		t.Fatalf("usage not propagated: %+v", resp.Usage)
	}
}

func TestCreateCompletionStream(t *testing.T) {
	sse := "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hi\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n" +
		"data: [DONE]\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		reader := bufio.NewReader(strings.NewReader(sse))
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				_, _ = w.Write([]byte(line))
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	adapter, err := New(Config{APIKey: "sk-test", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	req := openai.ChatCompletionRequest{Model: "claude-3-sonnet", Messages: []openai.ChatMessage{{Role: "user", Content: "hi"}}, Stream: true}
	ch, err := adapter.CreateCompletionStream(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateCompletionStream: %v", err)
	}

	received := false
	for ev := range ch {
		if ev.Error != nil {
			t.Fatalf("stream error: %v", ev.Error)
		}
		if ev.Chunk != nil {
			if txt := ev.Chunk.Choices[0].Delta.Content; strings.TrimSpace(txt) != "" {
				received = true
				if txt != "hi" {
					t.Fatalf("unexpected delta text: %s", txt)
				}
			}
		}
	}
	if !received {
		t.Fatal("no stream chunks received")
	}
}

func TestBuildTranslatorRequest(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "claude-3-sonnet",
		Messages: []openai.ChatMessage{
			{Role: "system", Content: "policy"},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "call_1", Type: "function", Function: openai.FunctionCall{Name: "do", Arguments: "{}"}}}},
		},
		Metadata:   map[string]string{"team": "tok"},
		Tools:      []openai.Tool{{Type: "function", Function: openai.ToolFunction{Name: "weather"}}},
		ToolChoice: "auto",
	}

	treq, err := buildTranslatorRequest(req)
	if err != nil {
		t.Fatalf("buildTranslatorRequest: %v", err)
	}
	if len(treq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(treq.Messages))
	}
	if treq.Metadata["team"] != "tok" {
		t.Fatalf("metadata not copied: %+v", treq.Metadata)
	}
	if treq.ToolChoice == nil || treq.ToolChoice.Kind != "auto" {
		t.Fatalf("tool choice not mapped: %+v", treq.ToolChoice)
	}
}

func TestMapModelName(t *testing.T) {
	cases := map[string]string{
		"gpt-4o":                  "claude-3-5-sonnet-20241022",
		"claude-3-haiku-20240307": "claude-3-haiku-20240307",
		"unknown":                 "claude-3-5-sonnet-20241022",
	}
	for in, want := range cases {
		if got := mapModelName(in); got != want {
			t.Fatalf("mapModelName(%s)=%s want %s", in, got, want)
		}
	}
}

func TestTranslatorConvert(t *testing.T) {
	resp := translatorpkg.OpenAIChatResponse{
		ID:    "msg_1",
		Model: "claude",
		Choices: []translatorpkg.OpenAIChoice{{
			Index:        0,
			FinishReason: "stop",
			Message: translatorpkg.OpenAIMessage{
				Role:    "assistant",
				Content: []translatorpkg.ContentBlock{translatorpkg.NewTextBlock("hi")},
			},
		}},
		Usage: translatorpkg.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
	}
	got := convertTranslatorResponse(resp, "claude")
	if got.Choices[0].Message.Content != "hi" {
		t.Fatalf("unexpected message: %+v", got.Choices[0].Message)
	}
}
