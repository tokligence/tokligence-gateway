package httpserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	adapterrouter "github.com/tokligence/tokligence-gateway/internal/adapter/router"
)

// fake upstream that records request bodies/headers
type captureServer struct {
	t      *testing.T
	last   map[string]any
	header http.Header
	srv    *httptest.Server
}

func newCaptureServer(t *testing.T) *captureServer {
	cs := &captureServer{t: t}
	cs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &cs.last)
		cs.header = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"test","choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	return cs
}

func (cs *captureServer) Close() { cs.srv.Close() }

func TestAnthropicBetaToggles_OnEnabled(t *testing.T) {
	cs := newCaptureServer(t)
	defer cs.Close()

	router := adapterrouter.New()
	_ = router.RegisterAdapter("openai", loopback.New())
	router.SetFallback(loopback.New())

	gw := &configurableGateway{}
	srv := New(gw, router, nil, nil, nil, rootAdminUser, nil, true)
	srv.SetWorkMode("translation")
	srv.SetAnthropicBetaFeatures(true, true, true, true, true, true)
	srv.SetUpstreams("test-openai", cs.srv.URL, "sk-anthropic", "https://api.anthropic.com", "2023-06-01", false, true, false, 8192, 16384, "", nil)

	reqBody, _ := json.Marshal(map[string]any{
		"model": "claude-3-5-haiku-20241022",
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
		"web_search":      map[string]any{"enable": true, "query": "test"},
		"computer_use":    map[string]any{"enable": true},
		"mcp":             map[string]any{"servers": []map[string]any{{"id": "svc", "url": "http://localhost"}}},
		"prompt_caching":  map[string]any{"fingerprint": "abc"},
		"response_format": map[string]any{"type": "json_object", "json_schema": map[string]any{"type": "object"}},
		"reasoning":       map[string]any{"effort": "medium"},
		"max_tokens":      100,
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(reqBody))
	srv.RouterAnthropic().ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if cs.last == nil {
		t.Fatalf("no downstream request captured")
	}
	// Currently Anthropic->OpenAI bridge does not forward beta fields to OpenAI payload;
	// ensure request still succeeds and unsupported fields are stripped.
	forbidden := []string{"web_search", "computer_use", "mcp", "prompt_caching", "response_format", "reasoning"}
	for _, k := range forbidden {
		if _, ok := cs.last[k]; ok {
			t.Fatalf("expected %s to be stripped in bridge payload", k)
		}
	}
}

func TestAnthropicBetaToggles_StrippedWhenDisabled(t *testing.T) {
	cs := newCaptureServer(t)
	defer cs.Close()

	router := adapterrouter.New()
	_ = router.RegisterAdapter("openai", loopback.New())
	router.SetFallback(loopback.New())

	gw := &configurableGateway{}
	srv := New(gw, router, nil, nil, nil, rootAdminUser, nil, true)
	srv.SetWorkMode("translation")
	srv.SetAnthropicBetaFeatures(false, false, false, false, false, false)
	srv.SetUpstreams("test-openai", cs.srv.URL, "sk-anthropic", "https://api.anthropic.com", "2023-06-01", false, true, false, 8192, 16384, "", nil)

	reqBody, _ := json.Marshal(map[string]any{
		"model": "claude-3-5-haiku-20241022",
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
		"web_search":      map[string]any{"enable": true},
		"computer_use":    map[string]any{"enable": true},
		"mcp":             map[string]any{"servers": []map[string]any{{"id": "svc", "url": "http://localhost"}}},
		"prompt_caching":  map[string]any{"fingerprint": "abc"},
		"response_format": map[string]any{"type": "json_object"},
		"reasoning":       map[string]any{"effort": "medium"},
		"max_tokens":      100,
	})

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(reqBody))
	srv.RouterAnthropic().ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if cs.last == nil {
		t.Fatalf("no downstream request captured")
	}
	forbidden := []string{"web_search", "computer_use", "mcp", "prompt_caching", "response_format", "reasoning"}
	for _, k := range forbidden {
		if _, ok := cs.last[k]; ok {
			t.Fatalf("expected %s to be stripped when disabled", k)
		}
	}
}
