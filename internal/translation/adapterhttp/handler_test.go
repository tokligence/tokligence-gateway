package adapterhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessagesHandler_ClampsWithModelCap(t *testing.T) {
	// fake OpenAI server captures posted body
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		OpenAIBaseURL: srv.URL,
		OpenAIAPIKey:  "test",
		// MaxTokensCap would allow 999, but modelCap should clamp to 100
		MaxTokensCap: 999,
		ModelCap: func(model string) (int, bool) {
			if model == "claude-3-5-haiku-20241022" {
				return 100, true
			}
			return 0, false
		},
	}

	req := map[string]any{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 500, // larger than the model cap
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(body))

	NewMessagesHandler(cfg, http.DefaultClient).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if captured == nil {
		t.Fatalf("did not capture downstream request")
	}
	mt, ok := captured["max_tokens"].(float64)
	if !ok {
		t.Fatalf("downstream max_tokens missing or wrong type: %#v", captured["max_tokens"])
	}
	if mt != 100 {
		t.Fatalf("expected max_tokens clamped to 100, got %v", mt)
	}
}

func TestMessagesHandler_UsesGlobalCapWhenNoModelCap(t *testing.T) {
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	t.Cleanup(srv.Close)

	cfg := Config{
		OpenAIBaseURL: srv.URL,
		OpenAIAPIKey:  "test",
		MaxTokensCap:  50,
	}

	req := map[string]any{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 500,
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	}
	body, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(body))

	NewMessagesHandler(cfg, http.DefaultClient).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if captured == nil {
		t.Fatalf("did not capture downstream request")
	}
	mt, ok := captured["max_tokens"].(float64)
	if !ok {
		t.Fatalf("downstream max_tokens missing or wrong type: %#v", captured["max_tokens"])
	}
	if mt != 50 {
		t.Fatalf("expected max_tokens clamped to 50, got %v", mt)
	}
}
