package httpserver

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/tokligence/tokligence-gateway/internal/adapter"
    "github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
    "github.com/tokligence/tokligence-gateway/internal/auth"
    "github.com/tokligence/tokligence-gateway/internal/client"
    "github.com/tokligence/tokligence-gateway/internal/ledger"
    "github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
    adapter2 "github.com/tokligence/tokligence-gateway/internal/sidecar/adapter"
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
    // Ensure anthropicNativeContentBlock accepts tool_result.content as a plain string
    raw := `{
        "model": "claude-3-5-haiku-20241022",
        "messages": [
            {"role": "assistant", "content": [{"type":"tool_use","id":"call_1","name":"Read","input":{"file_path":"/tmp/README.md"}}]},
            {"role": "user", "content": [{"type":"tool_result","tool_use_id":"call_1","content":"file content here"}]}
        ]
    }`
    var req anthropicNativeRequest
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
        var req anthropicNativeRequest
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
    if err != nil { t.Fatalf("AnthropicToOpenAI err: %v", err) }
    if len(oreq.Messages) < 3 { t.Fatalf("expected >=3 messages, got %d", len(oreq.Messages)) }
    // find assistant with tool_calls
    ai := -1
    for i, m := range oreq.Messages {
        if m.Role == "assistant" && len(m.ToolCalls) > 0 {
            ai = i
            break
        }
    }
    if ai == -1 { t.Fatalf("no assistant message with tool_calls: %#v", oreq.Messages) }
    if ai+2 >= len(oreq.Messages) { t.Fatalf("not enough messages after assistant") }
    if oreq.Messages[ai+1].Role != "tool" { t.Fatalf("expected tool message after assistant, got %s", oreq.Messages[ai+1].Role) }
    if oreq.Messages[ai+2].Role != "user" { t.Fatalf("expected user message after tool, got %s", oreq.Messages[ai+2].Role) }
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
