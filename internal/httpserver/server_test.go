package httpserver

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	adapterrouter "github.com/tokligence/tokligence-gateway/internal/adapter/router"
	"github.com/tokligence/tokligence-gateway/internal/auth"
	"github.com/tokligence/tokligence-gateway/internal/client"
	anthpkg "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/testutil"
	adapter2 "github.com/tokligence/tokligence-gateway/internal/translation/adapter"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

type gatewayData struct {
	user         *client.User
	provider     *client.ProviderProfile
	providers    []client.ProviderProfile
	servicesAll  []client.ServiceOffering
	servicesMine []client.ServiceOffering
	summary      client.UsageSummary
	err          error
	marketplace  bool
}

type memoryIdentityStore struct {
	users      map[int64]*userstore.User
	emails     map[string]int64
	apiKeys    map[int64][]userstore.APIKey
	tokens     map[string]userstore.APIKey
	nextUserID int64
	nextKeyID  int64
}

func newMemoryIdentityStore() *memoryIdentityStore {
	return &memoryIdentityStore{
		users:   make(map[int64]*userstore.User),
		emails:  make(map[string]int64),
		apiKeys: make(map[int64][]userstore.APIKey),
		tokens:  make(map[string]userstore.APIKey),
		// start IDs at 100 to avoid collisions with fixtures
		nextUserID: 100,
		nextKeyID:  200,
	}
}

type configurableGateway struct {
	data                 gatewayData
	marketplaceAvailable bool
}

func (s *memoryIdentityStore) Close() error { return nil }

func (s *memoryIdentityStore) ensureIDs() {
	if s.nextUserID == 0 {
		s.nextUserID = 1
	}
	if s.nextKeyID == 0 {
		s.nextKeyID = 1
	}
}

func (s *memoryIdentityStore) EnsureRootAdmin(ctx context.Context, email string) (*userstore.User, error) {
	s.ensureIDs()
	email = strings.ToLower(email)
	for _, u := range s.users {
		if strings.EqualFold(u.Email, email) && u.Role == userstore.RoleRootAdmin {
			return u, nil
		}
	}
	id := s.nextUserID
	s.nextUserID++
	user := &userstore.User{ID: id, Email: email, Role: userstore.RoleRootAdmin, Status: userstore.StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.users[id] = user
	s.emails[email] = id
	return user, nil
}

func (s *memoryIdentityStore) FindByEmail(ctx context.Context, email string) (*userstore.User, error) {
	if id, ok := s.emails[strings.ToLower(email)]; ok {
		return s.users[id], nil
	}
	return nil, nil
}

func (s *memoryIdentityStore) GetUser(ctx context.Context, id int64) (*userstore.User, error) {
	if user, ok := s.users[id]; ok {
		return user, nil
	}
	return nil, nil
}

func (s *memoryIdentityStore) ListUsers(ctx context.Context) ([]userstore.User, error) {
	var users []userstore.User
	for _, u := range s.users {
		users = append(users, *u)
	}
	return users, nil
}

func (s *memoryIdentityStore) CreateUser(ctx context.Context, email string, role userstore.Role, displayName string) (*userstore.User, error) {
	s.ensureIDs()
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if _, exists := s.emails[normEmail]; exists {
		return nil, fmt.Errorf("user with email %s already exists", normEmail)
	}
	id := s.nextUserID
	s.nextUserID++
	user := &userstore.User{ID: id, Email: normEmail, Role: role, DisplayName: displayName, Status: userstore.StatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.users[id] = user
	s.emails[user.Email] = id
	return user, nil
}

func (s *memoryIdentityStore) UpdateUser(ctx context.Context, id int64, displayName string, role userstore.Role) (*userstore.User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	if strings.TrimSpace(displayName) != "" {
		user.DisplayName = displayName
	}
	if role != "" {
		user.Role = role
	}
	user.UpdatedAt = time.Now()
	return user, nil
}

func (s *memoryIdentityStore) SetUserStatus(ctx context.Context, id int64, status userstore.Status) error {
	if user, ok := s.users[id]; ok {
		user.Status = status
		user.UpdatedAt = time.Now()
		return nil
	}
	return sql.ErrNoRows
}

func (s *memoryIdentityStore) DeleteUser(ctx context.Context, id int64) error {
	if user, ok := s.users[id]; ok {
		delete(s.emails, strings.ToLower(user.Email))
		delete(s.users, id)
		delete(s.apiKeys, id)
		return nil
	}
	return sql.ErrNoRows
}

func (s *memoryIdentityStore) CreateAPIKey(ctx context.Context, userID int64, scopes []string, expiresAt *time.Time) (*userstore.APIKey, string, error) {
	s.ensureIDs()
	s.nextKeyID++
	id := s.nextKeyID
	prefix := fmt.Sprintf("key-%d", id)
	key := userstore.APIKey{
		ID:        id,
		UserID:    userID,
		Prefix:    prefix,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.apiKeys[userID] = append(s.apiKeys[userID], key)
	token := fmt.Sprintf("token-%d", id)
	s.tokens[token] = key
	return &key, token, nil
}

func (s *memoryIdentityStore) ListAPIKeys(ctx context.Context, userID int64) ([]userstore.APIKey, error) {
	return append([]userstore.APIKey{}, s.apiKeys[userID]...), nil
}

func (s *memoryIdentityStore) DeleteAPIKey(ctx context.Context, id int64) error {
	for userID, keys := range s.apiKeys {
		for idx, key := range keys {
			if key.ID == id {
				s.apiKeys[userID] = append(keys[:idx], keys[idx+1:]...)
				for token, stored := range s.tokens {
					if stored.ID == id {
						delete(s.tokens, token)
						break
					}
				}
				return nil
			}
		}
	}
	return sql.ErrNoRows
}

func (s *memoryIdentityStore) LookupAPIKey(ctx context.Context, token string) (*userstore.APIKey, *userstore.User, error) {
	if key, ok := s.tokens[token]; ok {
		user := s.users[key.UserID]
		return &key, user, nil
	}
	return nil, nil, nil
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

func (c *configurableGateway) MarketplaceAvailable() bool {
	return c.marketplaceAvailable
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

func (c *configurableGateway) SetLocalAccount(user client.User, provider *client.ProviderProfile) {
	c.data.user = &user
	c.data.provider = provider
}

var defaultGatewayData = gatewayData{
	user:         &client.User{ID: 1, Email: "user@example.com", Roles: []string{"consumer"}},
	providers:    []client.ProviderProfile{{ID: 10, DisplayName: "Tokligence"}},
	servicesAll:  []client.ServiceOffering{{ID: 101, Name: "public"}},
	servicesMine: []client.ServiceOffering{{ID: 201, Name: "mine"}},
	summary:      client.UsageSummary{ConsumedTokens: 100, SuppliedTokens: 10, NetTokens: 90},
	marketplace:  true,
}

type stubLedger struct {
	entries []ledger.Entry
	summary ledger.Summary
}

func performLogin(t *testing.T, srv *Server, email string, enableProvider bool) *http.Cookie {
	loginBody := map[string]any{"email": email}
	body, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status %d", loginRec.Code)
	}
	var loginPayload map[string]string
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	verifyPayload := map[string]any{
		"challenge_id": loginPayload["challenge_id"],
		"code":         loginPayload["code"],
	}
	if enableProvider {
		verifyPayload["enable_provider"] = true
	}
	verifyBody, _ := json.Marshal(verifyPayload)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status %d", verifyRec.Code)
	}
	cookies := verifyRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie")
	}
	return cookies[0]
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

var rootAdminUser = &userstore.User{ID: 999, Email: "admin@local", Role: userstore.RoleRootAdmin, Status: userstore.StatusActive}

func TestAuthLoginAndVerify(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	authManager := auth.NewManager("secret")
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	srv := New(gw, loopback.New(), nil, authManager, identity, rootAdminUser, nil, true)

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

func TestRootAdminLoginBypassesChallenge(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: false}
	authManager := auth.NewManager("secret")
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	srv := New(gw, loopback.New(), nil, authManager, identity, rootAdminUser, nil, true)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@local"}`))
	loginRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("unexpected login status %d", loginRec.Code)
	}
	if len(loginRec.Result().Cookies()) == 0 {
		t.Fatalf("expected session cookie for root admin")
	}
	var payload map[string]any
	if err := json.Unmarshal(loginRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := payload["token"].(string); !ok {
		t.Fatalf("expected token in payload, got %#v", payload)
	}
}

func TestProtectedEndpointsRequireSession(t *testing.T) {
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	srv := New(&configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}, loopback.New(), nil, auth.NewManager("secret"), identity, rootAdminUser, nil, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/profile", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestServicesEndpointWithSession(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	authManager := auth.NewManager("secret")
	token, _ := authManager.IssueToken("user@example.com", time.Hour)
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	if _, err := identity.CreateUser(context.Background(), "user@example.com", userstore.RoleGatewayUser, ""); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	srv := New(gw, loopback.New(), nil, authManager, identity, rootAdminUser, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	req.AddCookie(&http.Cookie{Name: "tokligence_session", Value: token})
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestChatCompletionRecordsLedger(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	ledgerStub := &stubLedger{}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "tester@example.com", userstore.RoleGatewayUser, "Tester")
	key, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), ledgerStub, nil, identity, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.ChatCompletionRequest{
		Model:    "loopback",
		Messages: []openai.ChatMessage{{Role: "user", Content: "Hello"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if len(ledgerStub.entries) != 1 {
		t.Fatalf("expected ledger entry recorded")
	}
	if ledgerStub.entries[0].APIKeyID == nil || *ledgerStub.entries[0].APIKeyID != key.ID {
		t.Fatalf("expected api key id recorded")
	}
}

func TestUsageSummaryFromLedger(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	ledgerStub := &stubLedger{summary: ledger.Summary{ConsumedTokens: 10, SuppliedTokens: 4, NetTokens: -6}}
	authManager := auth.NewManager("secret")
	token, _ := authManager.IssueToken("user@example.com", time.Hour)
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	identity.users[1] = &userstore.User{ID: 1, Email: "user@example.com", Role: userstore.RoleGatewayUser, Status: userstore.StatusActive}
	identity.emails["user@example.com"] = 1
	srv := New(gw, loopback.New(), ledgerStub, authManager, identity, rootAdminUser, nil, true)

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
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
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
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	identity.users[1] = &userstore.User{ID: 1, Email: "user@example.com", Role: userstore.RoleGatewayUser, Status: userstore.StatusActive}
	identity.emails["user@example.com"] = 1
	srv := New(gw, loopback.New(), ledgerStub, authManager, identity, rootAdminUser, nil, true)

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

// --- /v1/responses compatibility tests ---

// recAdapter records the last chat completion request it received and returns a fixed response.
type recAdapter struct{ last openai.ChatCompletionRequest }

func (r *recAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	r.last = req
	reply := openai.ChatMessage{Role: "assistant", Content: "ok"}
	return openai.NewCompletionResponse(req.Model, reply, openai.UsageBreakdown{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}), nil
}

func (r *recAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	return openai.NewEmbeddingResponse(req.Model, [][]float64{{0.1}}, 1), nil
}

// streamAdapter streams a few deltas and then closes.
type streamAdapter struct{}

func (s *streamAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	reply := openai.ChatMessage{Role: "assistant", Content: "ok"}
	return openai.NewCompletionResponse(req.Model, reply, openai.UsageBreakdown{}), nil
}

func (s *streamAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	ch := make(chan adapter.StreamEvent, 3)
	go func() {
		defer close(ch)
		// emit two tiny deltas then end
		ch <- adapter.StreamEvent{Chunk: &openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{{Delta: openai.ChatMessageDelta{Content: "A"}}}}}
		ch <- adapter.StreamEvent{Chunk: &openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{{Delta: openai.ChatMessageDelta{Content: "B"}}}}}
	}()
	return ch, nil
}

func (s *streamAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	return openai.NewEmbeddingResponse(req.Model, [][]float64{{0.1}}, 1), nil
}

func TestResponses_NonStream_Loopback(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, loopback.New(), nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	body := []byte(`{"model":"loopback","input":"Hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var payload struct {
		OutputText string `json:"output_text"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(payload.OutputText, "Hello") {
		t.Fatalf("unexpected output_text: %q", payload.OutputText)
	}
}

func TestResponses_MapsToolsAndInstructions(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	rec := &recAdapter{}
	srv := New(gw, rec, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	body := []byte(`{
        "model":"x",
        "instructions":"You are JSON only",
        "messages":[{"role":"user","content":"hi"}],
        "tools":[{"type":"function","function":{"name":"t","parameters":{}}}],
        "tool_choice":"auto",
        "response_format":{"type":"json_object"}
    }`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	recw := httptest.NewRecorder()
	srv.Router().ServeHTTP(recw, req)
	if recw.Code != http.StatusOK {
		t.Fatalf("status=%d", recw.Code)
	}
	// Verify mapping hit adapter
	if len(rec.last.Messages) == 0 || strings.ToLower(rec.last.Messages[0].Role) != "system" {
		t.Fatalf("expected system message injected, got %#v", rec.last.Messages)
	}
	if len(rec.last.Tools) != 1 {
		t.Fatalf("expected tools length 1, got %d", len(rec.last.Tools))
	}
	if rec.last.ToolChoice == nil {
		t.Fatalf("expected tool_choice mapped")
	}
}

func TestResponses_Stream_SendsSSE(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, &streamAdapter{}, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	body := []byte(`{"model":"x","input":"hi","stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.output_text.delta") {
		t.Fatalf("missing delta events: %s", out)
	}
	if !strings.Contains(out, "event: response.completed") {
		t.Fatalf("missing completed event: %s", out)
	}
}

// streamRefusalAdapter emits a finish_reason content_filter
type streamRefusalAdapter struct{}

func (s *streamRefusalAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	reply := openai.ChatMessage{Role: "assistant", Content: ""}
	return openai.NewCompletionResponse(req.Model, reply, openai.UsageBreakdown{}), nil
}

func (s *streamRefusalAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	ch := make(chan adapter.StreamEvent, 2)
	go func() {
		defer close(ch)
		fr := "content_filter"
		ch <- adapter.StreamEvent{Chunk: &openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{{FinishReason: &fr}}}}
	}()
	return ch, nil
}

func (s *streamRefusalAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	return openai.NewEmbeddingResponse(req.Model, [][]float64{{0.1}}, 1), nil
}

func TestResponses_Stream_EmitsRefusal(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, &streamRefusalAdapter{}, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	body := []byte(`{"model":"x","input":"hi","stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.refusal.delta") {
		t.Fatalf("missing refusal delta event: %s", out)
	}
	if !strings.Contains(out, "event: response.refusal.done") {
		t.Fatalf("missing refusal done event: %s", out)
	}
}

// streamJSONInvalidAdapter emits non-JSON text under structured mode
type streamJSONInvalidAdapter struct{}

func (s *streamJSONInvalidAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	reply := openai.ChatMessage{Role: "assistant", Content: ""}
	return openai.NewCompletionResponse(req.Model, reply, openai.UsageBreakdown{}), nil
}

func (s *streamJSONInvalidAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	ch := make(chan adapter.StreamEvent, 2)
	go func() {
		defer close(ch)
		ch <- adapter.StreamEvent{Chunk: &openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{{Delta: openai.ChatMessageDelta{Content: "not json"}}}}}
	}()
	return ch, nil
}

func (s *streamJSONInvalidAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	return openai.NewEmbeddingResponse(req.Model, [][]float64{{0.1}}, 1), nil
}

func TestResponses_Stream_StructuredJSONValidation(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, &streamJSONInvalidAdapter{}, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	// structured mode
	body := []byte(`{"model":"x","input":"hi","stream":true, "response_format": {"type":"json_object"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.output_json.delta") {
		t.Fatalf("missing json delta event: %s", out)
	}
	if !strings.Contains(out, "event: response.error") {
		t.Fatalf("expected validation error event: %s", out)
	}
}

// streamToolAdapter emits a tool_call delta
type streamToolAdapter struct{}

func (s *streamToolAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	reply := openai.ChatMessage{Role: "assistant", Content: ""}
	return openai.NewCompletionResponse(req.Model, reply, openai.UsageBreakdown{}), nil
}

func (s *streamToolAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	ch := make(chan adapter.StreamEvent, 2)
	go func() {
		defer close(ch)
		// Emit a tool_call delta
		ch <- adapter.StreamEvent{Chunk: &openai.ChatCompletionChunk{Choices: []openai.ChatCompletionChunkChoice{{
			Delta: openai.ChatMessageDelta{ToolCalls: []openai.ToolCallDelta{{Function: &openai.ToolFunctionPart{Name: "t", Arguments: "{}"}}}},
		}}}}
	}()
	return ch, nil
}

func (s *streamToolAdapter) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	return openai.NewEmbeddingResponse(req.Model, [][]float64{{0.1}}, 1), nil
}

func TestResponses_Stream_EmitsToolCallDeltas(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, &streamToolAdapter{}, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	body := []byte(`{"model":"x","input":"hi","stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.tool_call.delta") {
		t.Fatalf("missing tool_call delta event: %s", out)
	}
}

func TestResponses_OpenAI_Delegate_NonStream(t *testing.T) {
	// Upstream mock OpenAI /responses
	upstream := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"resp_mock","object":"response","created":123,"model":"gpt-4o","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"output_text":"hi"}`)
	}))
	defer upstream.Close()

	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, loopback.New(), nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	// Configure upstream OpenAI
	srv.openaiAPIKey = "sk"
	srv.openaiBaseURL = upstream.URL

	body := []byte(`{"model":"gpt-4o","input":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"object\":\"response\"") {
		t.Fatalf("unexpected resp: %s", rec.Body.String())
	}
}

func TestResponses_OpenAI_Delegate_Stream(t *testing.T) {
	upstream := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		io.WriteString(w, "event: response.created\n")
		io.WriteString(w, "data: {}\n\n")
		io.WriteString(w, "event: response.output_text.delta\n")
		io.WriteString(w, "data: {\"delta\":\"hello\"}\n\n")
		io.WriteString(w, "event: response.completed\n")
		io.WriteString(w, "data: {}\n\n")
	}))
	defer upstream.Close()

	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, loopback.New(), nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	srv.openaiAPIKey = "sk"
	srv.openaiBaseURL = upstream.URL

	body := []byte(`{"model":"gpt-4o","input":"hi","stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	out := rec.Body.String()
	if !strings.Contains(out, "event: response.output_text.delta") {
		t.Fatalf("no delta in %s", out)
	}
	if !strings.Contains(out, "event: response.completed") {
		t.Fatalf("no completed in %s", out)
	}
}

func TestResponses_AnthropicBridge_NonStream(t *testing.T) {
	var capturedBody []byte
	anth := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"id":"msg_1",
			"type":"message",
			"role":"assistant",
			"model":"claude-3-sonnet",
			"stop_reason":"end_turn",
			"content":[{"type":"text","text":"Hello from Anthropic"}],
			"usage":{"input_tokens":12,"output_tokens":7}
		}`)
	}))
	defer anth.Close()

	rt := adapterrouter.New()
	if err := rt.RegisterAdapter("anthropic", loopback.New()); err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	if err := rt.RegisterRoute("claude-*", "anthropic"); err != nil {
		t.Fatalf("register route: %v", err)
	}

	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, rt, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	srv.SetUpstreams("", "", "test-key", anth.URL, "2023-06-01", false, false, false, 0, 0, "", nil)

	body := []byte(`{"model":"claude-3-sonnet","input":"Hello","stream":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(capturedBody) == 0 {
		t.Fatalf("anthropic request body was empty")
	}
	// Non-stream bridge should not force stream flag
	var payload responsesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.OutputText != "Hello from Anthropic" {
		t.Fatalf("unexpected output_text: %q", payload.OutputText)
	}
}

func TestResponses_AnthropicBridge_Stream(t *testing.T) {
	var capturedBody []byte
	anth := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		defer r.Body.Close()
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		fmt.Fprintf(w, "event: content_block_delta\n")
		fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		fmt.Fprintf(w, "event: message_delta\n")
		fmt.Fprintf(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n")
		fmt.Fprintf(w, "event: message_stop\n")
		fmt.Fprintf(w, "data: {}\n\n")
	}))
	defer anth.Close()

	rt := adapterrouter.New()
	if err := rt.RegisterAdapter("anthropic", loopback.New()); err != nil {
		t.Fatalf("register adapter: %v", err)
	}
	if err := rt.RegisterRoute("claude-*", "anthropic"); err != nil {
		t.Fatalf("register route: %v", err)
	}

	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, rt, nil, nil, newMemoryIdentityStore(), rootAdminUser, nil, true)
	srv.SetUpstreams("", "", "test-key", anth.URL, "2023-06-01", false, false, false, 0, 0, "", nil)

	body := []byte(`{"model":"claude-3-sonnet","input":"Hello","stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(capturedBody) == 0 {
		t.Fatalf("anthropic request body was empty")
	}
	if !bytes.Contains(capturedBody, []byte(`"stream":true`)) {
		t.Fatalf("expected stream=true in anthropic payload: %s", string(capturedBody))
	}
	out := rec.Body.String()
	if !strings.Contains(out, "response.output_text.delta") {
		t.Fatalf("missing output_text delta, got: %s", out)
	}
	if !strings.Contains(out, "Hi") {
		t.Fatalf("expected anthropic text mapped into stream, got: %s", out)
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /health 200, got %d", rec.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected status: %#v", payload["status"])
	}
}

func TestAdminImportUsers(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	authManager := auth.NewManager("secret")
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	srv := New(gw, loopback.New(), nil, authManager, identity, rootAdminUser, nil, true)

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"admin@local"}`))
	loginRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("unexpected login status %d", loginRec.Code)
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie")
	}
	cookie := cookies[0]

	firstPayload := map[string]any{
		"users": []map[string]string{
			{"email": "alpha@example.com", "role": "gateway_user", "display_name": "Alpha"},
			{"email": "beta@example.com", "role": "gateway_admin"},
		},
	}
	body, _ := json.Marshal(firstPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/import", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Cookie", cookie.String())
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	var importResp struct {
		Created []map[string]any    `json:"created"`
		Skipped []map[string]string `json:"skipped"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &importResp); err != nil {
		t.Fatalf("decode import: %v", err)
	}
	if len(importResp.Created) != 2 {
		t.Fatalf("expected 2 created, got %d", len(importResp.Created))
	}

	secondPayload := map[string]any{
		"skip_existing": true,
		"users": []map[string]string{
			{"email": "alpha@example.com"},
			{"email": ""},
		},
	}
	secondBody, _ := json.Marshal(secondPayload)
	repeatReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/import", bytes.NewReader(secondBody))
	repeatReq.AddCookie(cookie)
	repeatReq.Header.Set("Cookie", cookie.String())
	repeatRec := httptest.NewRecorder()
	srv.Router().ServeHTTP(repeatRec, repeatReq)
	if repeatRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", repeatRec.Code)
	}
	if err := json.Unmarshal(repeatRec.Body.Bytes(), &importResp); err != nil {
		t.Fatalf("decode second import: %v", err)
	}
	if len(importResp.Created) != 0 {
		t.Fatalf("expected 0 created, got %d", len(importResp.Created))
	}
	if len(importResp.Skipped) != 2 {
		t.Fatalf("expected 2 skipped, got %d", len(importResp.Skipped))
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

func TestModelsEndpoint(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, loopback.New(), nil, nil, nil, rootAdminUser, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response openai.ModelsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode models response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("response.Object = %q, want 'list'", response.Object)
	}

	if len(response.Data) == 0 {
		t.Fatal("expected at least one model in response")
	}

	// Check for specific models
	modelIDs := make(map[string]bool)
	for _, model := range response.Data {
		modelIDs[model.ID] = true
		if model.Object != "model" {
			t.Errorf("model.Object = %q, want 'model'", model.Object)
		}
		if model.OwnedBy == "" {
			t.Errorf("model %s has empty OwnedBy", model.ID)
		}
	}

	// Verify expected models are present
	expectedModels := []string{
		"loopback",
		"gpt-4",
		"gpt-3.5-turbo",
		"claude-3-5-sonnet-20241022",
	}

	for _, expected := range expectedModels {
		if !modelIDs[expected] {
			t.Errorf("expected model %q not found in response", expected)
		}
	}
}

// streamingAdapter is a minimal adapter that emits two chunks then closes.
type streamingAdapter struct{}

func (s *streamingAdapter) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.NewCompletionResponse(req.Model, openai.ChatMessage{Role: "assistant", Content: "ok"}, openai.UsageBreakdown{}), nil
}
func (s *streamingAdapter) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	ch := make(chan adapter.StreamEvent, 2)
	go func() {
		defer close(ch)
		// first chunk with role
		chunk1 := openai.ChatCompletionChunk{Model: req.Model, Choices: []openai.ChatCompletionChunkChoice{{Delta: openai.ChatMessageDelta{Role: "assistant", Content: "Hello"}}}}
		ch <- adapter.StreamEvent{Chunk: &chunk1}
		// second chunk
		chunk2 := openai.ChatCompletionChunk{Model: req.Model, Choices: []openai.ChatCompletionChunkChoice{{Delta: openai.ChatMessageDelta{Content: " World"}}}}
		ch <- adapter.StreamEvent{Chunk: &chunk2}
	}()
	return ch, nil
}

// Ensure streamingAdapter satisfies interfaces
var _ adapter.StreamingChatAdapter = (*streamingAdapter)(nil)

func TestChatCompletionsStreaming(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	sa := &streamingAdapter{}
	srv := New(gw, sa, nil, nil, nil, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.ChatCompletionRequest{Model: "gpt-4", Stream: true, Messages: []openai.ChatMessage{{Role: "user", Content: "hi"}}})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data:") {
		t.Fatalf("expected SSE data lines, got: %s", body)
	}
}

func TestAnthropicNativeToggle(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srvDisabled := New(gw, loopback.New(), nil, nil, nil, rootAdminUser, nil, false)
	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	srvDisabled.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when native endpoint disabled, got %d", rec.Code)
	}
}

func TestModelsEndpointStructure(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, loopback.New(), nil, nil, nil, rootAdminUser, nil, true)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Verify response is valid JSON with expected structure
	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := response["object"]; !ok {
		t.Error("response missing 'object' field")
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("response 'data' field is not an array")
	}

	if len(data) == 0 {
		t.Fatal("response 'data' array is empty")
	}

	// Check first model structure
	firstModel, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatal("first model is not an object")
	}

	requiredFields := []string{"id", "object", "created", "owned_by"}
	for _, field := range requiredFields {
		if _, ok := firstModel[field]; !ok {
			t.Errorf("model missing required field %q", field)
		}
	}
}

func TestAnthropicDecodeToolResultContentString(t *testing.T) {
	// Ensure anthropic.ContentBlock accepts tool_result.content as a plain string
	raw := `{
        "model": "claude-3-5-haiku-20241022",
        "messages": [
            {"role": "assistant", "content": [{"type":"tool_use","id":"call_1","name":"Read","input":{"file_path":"/tmp/README.md"}}]},
            {"role": "user", "content": [{"type":"tool_result","tool_use_id":"call_1","content":"file content here"}]}
        ]
    }`
	var req anthpkg.NativeRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(req.Messages))
	}
	blocks := req.Messages[1].Content.Blocks
	if len(blocks) != 1 || !strings.EqualFold(blocks[0].Type, "tool_result") {
		t.Fatalf("unexpected second message blocks: %#v", blocks)
	}
	if len(blocks[0].Content) != 1 || !strings.EqualFold(blocks[0].Content[0].Type, "text") || blocks[0].Content[0].Text == "" {
		t.Fatalf("expected tool_result.content as single text block, got %#v", blocks[0].Content)
	}
}

func TestNormalizeAnthropicRequest_ContentShapes(t *testing.T) {
	cases := []string{
		`{"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"hello"}]}`,
		`{"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content": {"text":"hello"}}]}`,
		`{"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content": {"content":"hello"}}]}`,
		`{"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content": {"content":[{"type":"text","text":"hello"}]}}]}`,
	}
	for i, raw := range cases {
		var req anthpkg.NativeRequest
		if err := json.NewDecoder(bytes.NewReader([]byte(raw))).Decode(&req); err != nil {
			t.Fatalf("case %d decode err: %v", i, err)
		}
		if len(req.Messages) != 1 || len(req.Messages[0].Content.Blocks) == 0 || !strings.EqualFold(req.Messages[0].Content.Blocks[0].Type, "text") {
			t.Fatalf("case %d unexpected blocks: %#v", i, req.Messages[0].Content.Blocks)
		}
		if req.Messages[0].Content.Blocks[0].Text != "hello" {
			t.Fatalf("case %d text mismatch: %q", i, req.Messages[0].Content.Blocks[0].Text)
		}
	}
}

func TestAdapterMapping_ToolsSequence(t *testing.T) {
	// assistant proposes a tool call, user returns tool_result, then user asks to continue
	areq := adapter2.AnthropicMessageRequest{
		Model: "claude-x",
		Messages: []adapter2.AnthropicMsg{
			{
				Role:    "assistant",
				Content: json.RawMessage(`[{"type":"tool_use","id":"call_1","name":"lookup","input":{"q":"hi"}}]`),
			},
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"text","text":"ok"}]}]`),
			},
			{
				Role:    "user",
				Content: json.RawMessage(`[{"type":"text","text":"continue"}]`),
			},
		},
	}
	oreq, err := adapter2.AnthropicToOpenAI(areq)
	if err != nil {
		t.Fatalf("AnthropicToOpenAI err: %v", err)
	}
	if len(oreq.Messages) < 3 {
		t.Fatalf("expected >=3 messages, got %d", len(oreq.Messages))
	}
	// find assistant with tool_calls
	ai := -1
	for i, m := range oreq.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			ai = i
			break
		}
	}
	if ai == -1 {
		t.Fatalf("no assistant message with tool_calls: %#v", oreq.Messages)
	}
	if ai+2 >= len(oreq.Messages) {
		t.Fatalf("not enough messages after assistant")
	}
	if oreq.Messages[ai+1].Role != "tool" {
		t.Fatalf("expected tool message after assistant, got %s", oreq.Messages[ai+1].Role)
	}
	if oreq.Messages[ai+2].Role != "user" {
		t.Fatalf("expected user message after tool, got %s", oreq.Messages[ai+2].Role)
	}
}

func TestEmbeddingsEndpointSuccess(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	ledgerStub := &stubLedger{}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "tester@example.com", userstore.RoleGatewayUser, "Tester")
	_, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), ledgerStub, nil, identity, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "The quick brown fox jumps over the lazy dog",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response openai.EmbeddingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Object != "list" {
		t.Errorf("response.Object = %q, want 'list'", response.Object)
	}

	if len(response.Data) == 0 {
		t.Fatal("expected embedding data in response")
	}

	if response.Data[0].Object != "embedding" {
		t.Errorf("data[0].Object = %q, want 'embedding'", response.Data[0].Object)
	}

	if len(response.Data[0].Embedding) == 0 {
		t.Error("embedding vector is empty")
	}

	if response.Usage.PromptTokens == 0 {
		t.Error("usage.PromptTokens is 0")
	}
}

func TestEmbeddingsRecordsLedger(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	ledgerStub := &stubLedger{}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "embedder@example.com", userstore.RoleGatewayUser, "Embedder")
	key, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), ledgerStub, nil, identity, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Test input for embeddings",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if len(ledgerStub.entries) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledgerStub.entries))
	}

	entry := ledgerStub.entries[0]
	if entry.UserID != user.ID {
		t.Errorf("entry.UserID = %d, want %d", entry.UserID, user.ID)
	}

	if entry.APIKeyID == nil || *entry.APIKeyID != key.ID {
		t.Error("expected API key ID recorded in ledger")
	}

	if entry.Direction != ledger.DirectionConsume {
		t.Errorf("entry.Direction = %q, want %q", entry.Direction, ledger.DirectionConsume)
	}

	if entry.PromptTokens == 0 {
		t.Error("entry.PromptTokens is 0")
	}
}

func TestEmbeddingsMultipleInputs(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "multi@example.com", userstore.RoleGatewayUser, "Multi")
	_, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), nil, nil, identity, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: []string{"First input", "Second input", "Third input"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response openai.EmbeddingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Data) != 3 {
		t.Errorf("len(response.Data) = %d, want 3", len(response.Data))
	}

	// Verify indices
	for i, data := range response.Data {
		if data.Index != i {
			t.Errorf("data[%d].Index = %d, want %d", i, data.Index, i)
		}
	}
}

func TestEmbeddingsRequiresAuth(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	srv := New(gw, loopback.New(), nil, nil, identity, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Unauthorized request",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	// No Authorization header
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestEmbeddingsInvalidJSON(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "invalid@example.com", userstore.RoleGatewayUser, "Invalid")
	_, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), nil, nil, identity, rootAdminUser, nil, true)

	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewBufferString("{invalid json}"))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestEmbeddingsNoInput(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "noinput@example.com", userstore.RoleGatewayUser, "NoInput")
	_, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), nil, nil, identity, rootAdminUser, nil, true)

	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: nil,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	// Accept 400 (Bad Request), 500 (Internal Server Error), or 502 (Bad Gateway)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusInternalServerError && rec.Code != http.StatusBadGateway {
		t.Errorf("expected 400, 500, or 502 for nil input, got %d", rec.Code)
	}
}

func TestEmbeddingsWithOptionalParams(t *testing.T) {
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "optional@example.com", userstore.RoleGatewayUser, "Optional")
	_, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)
	srv := New(gw, loopback.New(), nil, nil, identity, rootAdminUser, nil, true)

	dimensions := 512
	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model:          "text-embedding-3-large",
		Input:          "Test with optional parameters",
		EncodingFormat: "float",
		Dimensions:     &dimensions,
		User:           "test-user-id",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var response openai.EmbeddingResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Data) == 0 {
		t.Fatal("expected embedding data")
	}
}

func TestEmbeddingsUnsupportedAdapter(t *testing.T) {
	// Create a mock adapter that doesn't support embeddings
	type nonEmbeddingAdapter struct{}

	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	identity := newMemoryIdentityStore()
	identity.users[rootAdminUser.ID] = rootAdminUser
	identity.emails[strings.ToLower(rootAdminUser.Email)] = rootAdminUser.ID
	user, _ := identity.CreateUser(context.Background(), "unsupported@example.com", userstore.RoleGatewayUser, "Unsupported")
	_, token, _ := identity.CreateAPIKey(context.Background(), user.ID, nil, nil)

	// Create server with non-embedding adapter
	srv := New(gw, loopback.New(), nil, nil, identity, rootAdminUser, nil, true)
	// Override the embedding adapter to nil to simulate unsupported
	srv.embeddingAdapter = nil

	reqBody, _ := json.Marshal(openai.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Test",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	srv.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rec.Code)
	}
}

func TestWorkModeDecision_ModelOverrideForcesTranslation(t *testing.T) {
	srv := newWorkModeTestServer(t, "sk-openai", "", []ModelProviderRule{
		{Pattern: "CLAUDE*", Provider: "ANTHROPIC"},
	})
	usePassthrough, err := srv.workModeDecision("/v1/chat/completions", "claude-3-5-sonnet-20241022")
	if err != nil {
		t.Fatalf("workModeDecision returned error: %v", err)
	}
	if usePassthrough {
		t.Fatalf("expected translation fallback when anthropic provider unavailable, got passthrough")
	}
}

func TestWorkModeDecision_GPTOnAnthropicEndpointTranslates(t *testing.T) {
	srv := newWorkModeTestServer(t, "sk-openai", "sk-anthropic", []ModelProviderRule{
		{Pattern: "gpt*", Provider: "openai"},
		{Pattern: "claude*", Provider: "anthropic"},
	})
	usePassthrough, err := srv.workModeDecision("/v1/messages", "gpt-4o")
	if err != nil {
		t.Fatalf("workModeDecision returned error: %v", err)
	}
	if usePassthrough {
		t.Fatalf("expected translation for gpt model on anthropic endpoint, got passthrough")
	}
}

func newWorkModeTestServer(t *testing.T, openaiKey, anthropicKey string, rules []ModelProviderRule) *Server {
	t.Helper()
	router := adapterrouter.New()
	lb := loopback.New()
	_ = router.RegisterAdapter("loopback", lb)
	if strings.TrimSpace(openaiKey) != "" {
		_ = router.RegisterAdapter("openai", loopback.New())
	}
	if strings.TrimSpace(anthropicKey) != "" {
		_ = router.RegisterAdapter("anthropic", loopback.New())
	}
	_ = router.RegisterRoute("loopback", "loopback")
	if strings.TrimSpace(openaiKey) != "" {
		_ = router.RegisterRoute("gpt-*", "openai")
	}
	if strings.TrimSpace(anthropicKey) != "" {
		_ = router.RegisterRoute("claude*", "anthropic")
	} else if strings.TrimSpace(openaiKey) != "" {
		_ = router.RegisterRoute("claude*", "openai")
	}
	router.SetFallback(lb)

	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	srv := New(gw, router, nil, nil, nil, rootAdminUser, nil, true)
	srv.SetUpstreams(openaiKey, "", anthropicKey, "", "", false, false, false, 0, 0, "", nil)
	srv.SetModelProviderRules(rules)
	srv.SetWorkMode("auto")
	return srv
}

func TestDuplicateToolDetectionToggle(t *testing.T) {
	// Build a server with an in-memory responses session containing duplicate tools
	gw := &configurableGateway{data: defaultGatewayData, marketplaceAvailable: defaultGatewayData.marketplace}
	router := adapterrouter.New()
	lb := loopback.New()
	_ = router.RegisterAdapter("loopback", lb)
	router.SetFallback(lb)
	srv := New(gw, router, nil, nil, nil, rootAdminUser, nil, true)
	srv.SetUpstreams("sk-openai", "", "sk-anthropic", "", "", false, false, false, 0, 0, "", nil)
	srv.SetDuplicateToolDetectionEnabled(true)

	// Seed a response session with 5 identical tool outputs (should trigger EMERGENCY STOP)
	id := "resp_test"
	msgs := []openai.ChatMessage{
		{Role: "tool", ToolCallID: "call_1", Content: "same-output"},
		{Role: "tool", ToolCallID: "call_2", Content: "same-output"},
		{Role: "tool", ToolCallID: "call_3", Content: "same-output"},
		{Role: "tool", ToolCallID: "call_4", Content: "same-output"},
		{Role: "tool", ToolCallID: "call_5", Content: "same-output"},
	}
	srv.responsesSessions[id] = &responseSession{
		Adapter: "anthropic",
		Base:    openai.ResponseRequest{ID: id},
		Request: openai.ChatCompletionRequest{Model: "claude-3-5-haiku-20241022", Messages: msgs},
		Outputs: make(chan []openai.ResponseToolOutput),
		Done:    make(chan struct{}),
	}

	// Detection enabled: expect error
	if _, _, _, err := srv.applyToolOutputsToSession(id, nil); err == nil {
		t.Fatalf("expected duplicate detection error when enabled")
	}

	// Disable detection and ensure no error
	srv.SetDuplicateToolDetectionEnabled(false)
	srv.responsesSessions[id] = &responseSession{
		Adapter: "anthropic",
		Base:    openai.ResponseRequest{ID: id},
		Request: openai.ChatCompletionRequest{Model: "claude-3-5-haiku-20241022", Messages: msgs},
		Outputs: make(chan []openai.ResponseToolOutput),
		Done:    make(chan struct{}),
	}
	if _, _, _, err := srv.applyToolOutputsToSession(id, nil); err != nil {
		t.Fatalf("expected no duplicate detection error when disabled, got %v", err)
	}
}
