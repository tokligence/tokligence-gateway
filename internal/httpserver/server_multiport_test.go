package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	srv.SetEndpointConfig([]string{"health"}, []string{"health"}, []string{"health"}, []string{"health"})

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
