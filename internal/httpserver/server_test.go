package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	"github.com/tokligence/tokligence-gateway/internal/auth"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type gatewayData struct {
	user         *client.User
	provider     *client.ProviderProfile
	providers    []client.ProviderProfile
	servicesAll  []client.ServiceOffering
	servicesMine []client.ServiceOffering
	summary      client.UsageSummary
	err          error
}

type configurableGateway struct {
	data gatewayData
}

func (c *configurableGateway) Account() (*client.User, *client.ProviderProfile) {
	return c.data.user, c.data.provider
}

func (c *configurableGateway) EnsureAccount(ctx context.Context, email string, roles []string, displayName string) (*client.User, *client.ProviderProfile, error) {
	if c.data.user == nil || c.data.user.Email != email {
		c.data.user = &client.User{ID: 1, Email: email, Roles: roles}
	} else {
		c.data.user.Roles = roles
	}
	if containsRole(roles, "provider") {
		c.data.provider = &client.ProviderProfile{ID: 10, UserID: c.data.user.ID, DisplayName: displayName}
	} else {
		c.data.provider = nil
	}
	return c.data.user, c.data.provider, c.data.err
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

var defaultGatewayData = gatewayData{
	user:         &client.User{ID: 1, Email: "user@example.com", Roles: []string{"consumer"}},
	providers:    []client.ProviderProfile{{ID: 10, DisplayName: "Tokligence"}},
	servicesAll:  []client.ServiceOffering{{ID: 101, Name: "public"}},
	servicesMine: []client.ServiceOffering{{ID: 201, Name: "mine"}},
	summary:      client.UsageSummary{ConsumedTokens: 100, SuppliedTokens: 10, NetTokens: 90},
}

type stubLedger struct {
	entries []ledger.Entry
	summary ledger.Summary
}

func (s *stubLedger) Record(ctx context.Context, entry ledger.Entry) error {
	s.entries = append(s.entries, entry)
	return nil
}

func (s *stubLedger) Summary(ctx context.Context, userID int64) (ledger.Summary, error) {
	if s.summary != (ledger.Summary{}) {
		return s.summary, nil
	}
	var consumed, supplied int64
	for _, e := range s.entries {
		if e.UserID != userID {
			continue
		}
		total := e.PromptTokens + e.CompletionTokens
		if e.Direction == ledger.DirectionConsume {
			consumed += total
		} else if e.Direction == ledger.DirectionSupply {
			supplied += total
		}
	}
	return ledger.Summary{ConsumedTokens: consumed, SuppliedTokens: supplied, NetTokens: supplied - consumed}, nil
}

func (s *stubLedger) ListRecent(ctx context.Context, userID int64, limit int) ([]ledger.Entry, error) {
	var filtered []ledger.Entry
	for _, e := range s.entries {
		if e.UserID == userID {
			filtered = append(filtered, e)
		}
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (s *stubLedger) Close() error { return nil }

func TestAuthLoginAndVerify(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData}
	authManager := auth.NewManager("secret")
	srv := New(gw, loopback.New(), nil, authManager)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"agent@example.com"}`))
	loginRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("unexpected login status %d", loginRec.Code)
	}
	var loginPayload map[string]string
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	challengeID := loginPayload["challenge_id"]
	code := loginPayload["code"]

	verifyReqBody := map[string]any{
		"challenge_id":    challengeID,
		"code":            code,
		"display_name":    "Agent",
		"enable_provider": true,
	}
	bodyBytes, _ := json.Marshal(verifyReqBody)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(bodyBytes))
	verifyRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("unexpected verify status %d", verifyRec.Code)
	}
	if len(verifyRec.Result().Cookies()) == 0 {
		t.Fatalf("expected session cookie")
	}
	if gw.data.provider == nil {
		t.Fatalf("expected provider to be set")
	}
}

func TestProtectedEndpointsRequireSession(t *testing.T) {
	srv := New(&configurableGateway{data: defaultGatewayData}, loopback.New(), nil, auth.NewManager("secret"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestServicesEndpointWithSession(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData}
	authManager := auth.NewManager("secret")
	token, _ := authManager.IssueToken("user@example.com", time.Hour)
	srv := New(gw, loopback.New(), nil, authManager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	req.AddCookie(&http.Cookie{Name: "tokligence_session", Value: token})
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestChatCompletionRecordsLedger(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData}
	ledgerStub := &stubLedger{}
	srv := New(gw, loopback.New(), ledgerStub, nil)

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
	if len(ledgerStub.entries) != 1 {
		t.Fatalf("expected ledger entry recorded")
	}
}

func TestUsageSummaryFromLedger(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData}
	ledgerStub := &stubLedger{summary: ledger.Summary{ConsumedTokens: 10, SuppliedTokens: 4, NetTokens: -6}}
	authManager := auth.NewManager("secret")
	token, _ := authManager.IssueToken("user@example.com", time.Hour)
	srv := New(gw, loopback.New(), ledgerStub, authManager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/summary", nil)
	req.AddCookie(&http.Cookie{Name: "tokligence_session", Value: token})
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var payload map[string]ledger.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["summary"].ConsumedTokens != 10 {
		t.Fatalf("unexpected summary %#v", payload)
	}
}

func TestUsageLogsEndpoint(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData}
	ledgerStub := &stubLedger{}
	ledgerStub.entries = []ledger.Entry{{
		ID:               1,
		UserID:           1,
		ServiceID:        101,
		PromptTokens:     100,
		CompletionTokens: 50,
		Direction:        ledger.DirectionConsume,
		Memo:             "test",
		CreatedAt:        time.Now(),
	}}
	authManager := auth.NewManager("secret")
	token, _ := authManager.IssueToken("user@example.com", time.Hour)
	srv := New(gw, loopback.New(), ledgerStub, authManager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/logs?limit=5", nil)
	req.AddCookie(&http.Cookie{Name: "tokligence_session", Value: token})
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	var payload map[string][]ledger.Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode logs: %v", err)
	}
	if len(payload["entries"]) != 1 {
		t.Fatalf("unexpected entries %#v", payload)
	}
}

func containsRole(roles []string, target string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, target) {
			return true
		}
	}
	return false
}
