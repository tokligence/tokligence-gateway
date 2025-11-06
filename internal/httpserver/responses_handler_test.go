package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestForwardResponsesToAnthropicSetsDefaultMaxTokens(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	srv.anthAPIKey = "sk-test"
	srv.anthVersion = "2023-06-01"
	srv.anthropicMaxTokens = 2048

	var captured map[string]any
	origClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				return nil, err
			}
			body := `{"id":"test","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude-3","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = origClient })
	srv.anthBaseURL = "https://anthropic.test"

	rr := responsesRequest{
		Model: "claude-3",
		Messages: []openai.ChatMessage{
			{Role: "user", Content: "hello"},
		},
		Stream: false,
	}
	creq := rr.ToChatCompletionRequest()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	if err := srv.forwardResponsesToAnthropic(rec, req, rr, creq, false, time.Now(), 0); err != nil {
		t.Fatalf("forwardResponsesToAnthropic error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	raw, ok := captured["max_tokens"]
	if !ok {
		t.Fatalf("max_tokens not set in request: %#v", captured)
	}
	switch v := raw.(type) {
	case float64:
		if int(v) != 2048 {
			t.Fatalf("expected max_tokens 2048, got %v", v)
		}
	case int:
		if v != 2048 {
			t.Fatalf("expected max_tokens 2048, got %d", v)
		}
	default:
		t.Fatalf("unexpected max_tokens type: %T", raw)
	}
}
