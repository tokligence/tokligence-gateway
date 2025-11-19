package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	"github.com/tokligence/tokligence-gateway/internal/hooks"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func newTestHTTPServer(t *testing.T, enableAnthropic bool) *Server {
	t.Helper()
	gw := &configurableGateway{}
	loop := loopback.New()
	identity := newMemoryIdentityStore()
	rootAdmin, err := identity.EnsureRootAdmin(context.Background(), "admin@test.dev")
	if err != nil {
		t.Fatalf("failed to seed root admin: %v", err)
	}
	srv := New(gw, loop, nil, nil, identity, rootAdmin, &hooks.Dispatcher{}, enableAnthropic)
	srv.SetAuthDisabled(true)
	return srv
}

func TestRouterOpenAIProvidesModels(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	handler := srv.RouterOpenAI()
	if handler == nil {
		t.Fatal("RouterOpenAI returned nil handler")
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /v1/models, got %d", rec.Code)
	}
	var resp struct {
		Data []openai.Model `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode models response: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected at least one model entry")
	}
}

func TestRouterAnthropicDisabled(t *testing.T) {
	srv := newTestHTTPServer(t, false)
	if handler := srv.RouterAnthropic(); handler != nil {
		t.Fatal("expected RouterAnthropic to return nil when disabled")
	}
}

func TestRouterAnthropicEnabled(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	handler := srv.RouterAnthropic()
	if handler == nil {
		t.Fatal("expected RouterAnthropic to return handler when enabled")
	}
}

func TestEndpointConfigCustomizesRoutes(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	// Expose only health endpoints on every port
	srv.SetEndpointConfig([]string{"health"}, []string{"health"}, []string{"health"}, []string{"health"}, []string{"health"})

	// Facade router should expose /health but not /v1/responses
	facade := srv.Router()
	if facade == nil {
		t.Fatal("facade router should not be nil")
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	facade.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected facade /health 200, got %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader([]byte(`{}`)))
	facade.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when responses endpoint disabled, got %d", rec.Code)
	}

	// OpenAI router configured with health only
	openaiRouter := srv.RouterOpenAI()
	if openaiRouter == nil {
		t.Fatal("openai router should not be nil when health endpoint configured")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	openaiRouter.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected openai /health 200, got %d", rec.Code)
	}

	// Anthropic router should also respond to health despite native API being disabled in config keys
	anthRouter := srv.RouterAnthropic()
	if anthRouter == nil {
		t.Fatal("anthropic router should expose health endpoint")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	anthRouter.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected anthropic /health 200, got %d", rec.Code)
	}
}

func TestMultiPortResponsesStreaming(t *testing.T) {
	srv := newStreamingTestServer(t)
	srv.SetWorkMode("translation") // translation mode = never delegate, always translate
	srv.SetEndpointConfig(
		[]string{"openai_responses", "health"},
		[]string{"openai_responses", "health"},
		[]string{"openai_responses", "health"},
		[]string{"health"},
		[]string{"health"},
	)

	payload := openai.ResponseRequest{
		Model:  "stream-test-model",
		Input:  "ping",
		Stream: true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	routers := []struct {
		name    string
		handler http.Handler
	}{
		{"facade", srv.Router()},
		{"openai", srv.RouterOpenAI()},
		{"anthropic", srv.RouterAnthropic()},
	}

	for _, rt := range routers {
		if rt.handler == nil {
			t.Fatalf("%s handler is nil", rt.name)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rt.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s /v1/responses expected 200, got %d", rt.name, rec.Code)
		}

		events := parseSSE(rec.Body.String())
		if len(events) == 0 {
			t.Fatalf("%s expected SSE events, got none", rt.name)
		}
		required := []string{
			"response.created",
			"response.output_item.added",
			"response.output_text.delta",
			"response.output_text.done",
			"response.output_item.done",
			"response.completed",
		}
		for _, name := range required {
			if len(eventsByName(events, name)) == 0 {
				t.Fatalf("%s missing event %s", rt.name, name)
			}
		}
	}
}

func newStreamingTestServer(t *testing.T) *Server {
	t.Helper()
	gw := &configurableGateway{}
	identity := newMemoryIdentityStore()
	rootAdmin, err := identity.EnsureRootAdmin(context.Background(), "admin@test.dev")
	if err != nil {
		t.Fatalf("failed to seed root admin: %v", err)
	}
	srv := New(gw, &multiportStreamingAdapter{}, nil, nil, identity, rootAdmin, &hooks.Dispatcher{}, true)
	srv.SetAuthDisabled(true)
	return srv
}

type multiportStreamingAdapter struct{}

func (a *multiportStreamingAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	reply := openai.ChatMessage{Role: "assistant", Content: "stream completed"}
	return openai.NewCompletionResponse(req.Model, reply, openai.UsageBreakdown{}), nil
}

func (a *multiportStreamingAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	ch := make(chan adapter.StreamEvent, 2)
	go func() {
		defer close(ch)
		chunk1 := openai.ChatCompletionChunk{
			Model: req.Model,
			Choices: []openai.ChatCompletionChunkChoice{{
				Delta: openai.ChatMessageDelta{
					Role:    "assistant",
					Content: "Hello",
				},
			}},
		}
		ch <- adapter.StreamEvent{Chunk: &chunk1}
		chunk2 := openai.ChatCompletionChunk{
			Model: req.Model,
			Choices: []openai.ChatCompletionChunkChoice{{
				Delta: openai.ChatMessageDelta{
					Content: " World",
				},
			}},
		}
		ch <- adapter.StreamEvent{Chunk: &chunk2}
	}()
	return ch, nil
}

var _ adapter.StreamingChatAdapter = (*multiportStreamingAdapter)(nil)
