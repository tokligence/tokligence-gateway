package adapterhttp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/testutil"
)

// Helper to collect SSE lines from a ResponseRecorder body
func collectSSE(body string) []string {
	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "data:") {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestMessagesStream_TextOnly(t *testing.T) {
	// Fake OpenAI upstream that emits a minimal text stream then [DONE]
	upstream := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// one text delta
		io := func(s string) { _, _ = w.Write([]byte(s)) }
		io("data: {\"id\":\"cmpl\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n")
		io("data: [DONE]\n\n")
	}))
	defer upstream.Close()

	cfg := Config{OpenAIBaseURL: upstream.URL, OpenAIAPIKey: "sk-test", DefaultOpenAIModel: "gpt-4o", MaxTokensCap: 16384}
	h := NewMessagesHandler(cfg, upstream.Client())

	// Anthropic-style request with stream=true
	reqBody := map[string]any{
		"model":    "claude-3-5-haiku-20241022",
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
		"stream":   true,
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", rec.Code, rec.Body.String())
	}
	lines := collectSSE(rec.Body.String())
	// Expect event/data pairs in order
	wantEvents := []string{"event: message_start", "event: content_block_start", "event: content_block_delta", "event: content_block_stop", "event: message_delta", "event: message_stop"}
	idx := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "event:") {
			if idx >= len(wantEvents) {
				t.Fatalf("too many events, got %v", lines)
			}
			if l != wantEvents[idx] {
				t.Fatalf("event[%d]=%q want %q", idx, l, wantEvents[idx])
			}
			idx++
		}
	}
	if idx != len(wantEvents) {
		t.Fatalf("missing events: want %d, got %d", len(wantEvents), idx)
	}
}

func TestMessagesJSON_TextOnly(t *testing.T) {
	// Fake OpenAI upstream returns JSON
	upstream := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`))
	}))
	defer upstream.Close()

	cfg := Config{OpenAIBaseURL: upstream.URL, OpenAIAPIKey: "sk-test", DefaultOpenAIModel: "gpt-4o", MaxTokensCap: 16384}
	h := NewMessagesHandler(cfg, upstream.Client())

	reqBody := map[string]any{
		"model":    "claude-3-5-haiku-20241022",
		"messages": []map[string]any{{"role": "user", "content": "hi"}},
		"stream":   false,
	}
	b, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if out["type"] != "message" || out["role"] != "assistant" {
		t.Fatalf("unexpected type/role: %v", out)
	}
}
