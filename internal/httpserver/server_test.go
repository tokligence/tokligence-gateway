package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type fakeGateway struct{}

type gatewayData struct {
	user         *client.User
	provider     *client.ProviderProfile
	providers    []client.ProviderProfile
	servicesAll  []client.ServiceOffering
	servicesMine []client.ServiceOffering
	summary      client.UsageSummary
	err          error
}

var defaultData = gatewayData{
	user:         &client.User{ID: 1, Email: "user@example.com", Roles: []string{"consumer"}},
	providers:    []client.ProviderProfile{{ID: 10, DisplayName: "Tokligence"}},
	servicesAll:  []client.ServiceOffering{{ID: 101, Name: "public"}},
	servicesMine: []client.ServiceOffering{{ID: 201, Name: "mine"}},
	summary:      client.UsageSummary{ConsumedTokens: 100, SuppliedTokens: 10, NetTokens: 90},
}

type stubLedger struct {
	entries    []ledger.Entry
	summary    ledger.Summary
	recordErr  error
	summaryErr error
}

func (s *stubLedger) Record(ctx context.Context, entry ledger.Entry) error {
	s.entries = append(s.entries, entry)
	return s.recordErr
}

func (s *stubLedger) Summary(ctx context.Context, userID int64) (ledger.Summary, error) {
	if s.summaryErr != nil {
		return ledger.Summary{}, s.summaryErr
	}
	return s.summary, nil
}

func (s *stubLedger) ListRecent(ctx context.Context, userID int64, limit int) ([]ledger.Entry, error) {
	return nil, nil
}

func (s *stubLedger) Close() error { return nil }

type configurableGateway struct {
	data gatewayData
}

func (c *configurableGateway) Account() (*client.User, *client.ProviderProfile) {
	return c.data.user, c.data.provider
}

func (c *configurableGateway) ListProviders(ctx context.Context) ([]client.ProviderProfile, error) {
	return c.data.providers, c.data.err
}

func (c *configurableGateway) ListServices(ctx context.Context, _ *int64) ([]client.ServiceOffering, error) {
	return c.data.servicesAll, c.data.err
}

func (c *configurableGateway) ListMyServices(ctx context.Context) ([]client.ServiceOffering, error) {
	return c.data.servicesMine, c.data.err
}

func (c *configurableGateway) UsageSnapshot(ctx context.Context) (client.UsageSummary, error) {
	return c.data.summary, c.data.err
}

func TestProfileEndpoint(t *testing.T) {
	gw := &configurableGateway{data: defaultData}
	srv := New(gw, loopback.New(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	user := payload["user"].(map[string]any)
	if user["email"].(string) != "user@example.com" {
		t.Fatalf("unexpected user payload %#v", payload)
	}
}

func TestServicesEndpointScopes(t *testing.T) {
	gw := &configurableGateway{data: defaultData}
	srv := New(gw, loopback.New(), nil)

	reqAll := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	recAll := httptest.NewRecorder()
	srv.Router().ServeHTTP(recAll, reqAll)
	if recAll.Code != http.StatusOK {
		t.Fatalf("all services status %d", recAll.Code)
	}

	reqMine := httptest.NewRequest(http.MethodGet, "/api/v1/services?scope=mine", nil)
	recMine := httptest.NewRecorder()
	srv.Router().ServeHTTP(recMine, reqMine)
	if recMine.Code != http.StatusOK {
		t.Fatalf("mine services status %d", recMine.Code)
	}

	var payload map[string][]map[string]any
	if err := json.Unmarshal(recMine.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode mine: %v", err)
	}
	if len(payload["services"]) != 1 || payload["services"][0]["name"] != "mine" {
		t.Fatalf("unexpected mine services %#v", payload)
	}
}

func TestChatCompletionLoopback(t *testing.T) {
	ledgerStub := &stubLedger{}
	gw := &configurableGateway{data: defaultData}
	srv := New(gw, loopback.New(), ledgerStub)

	reqBody, _ := json.Marshal(openai.ChatCompletionRequest{
		Model:    "loopback",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload openai.ChatCompletionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Choices[0].Message.Content != "[loopback] Hello" {
		t.Fatalf("unexpected completion %q", payload.Choices[0].Message.Content)
	}
	if len(ledgerStub.entries) != 1 {
		t.Fatalf("expected ledger entry recorded")
	}
}

func TestUsageSummaryFromLedger(t *testing.T) {
	gw := &configurableGateway{data: defaultData}
	ledgerStub := &stubLedger{summary: ledger.Summary{ConsumedTokens: 10, SuppliedTokens: 4, NetTokens: -6}}
	srv := New(gw, loopback.New(), ledgerStub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/summary", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var payload map[string]map[string]float64
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["summary"]["consumed_tokens"] != 10 {
		t.Fatalf("unexpected summary %#v", payload)
	}
}
