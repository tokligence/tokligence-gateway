package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/auth"
    translationhttp "github.com/tokligence/tokligence-gateway/internal/translation/adapterhttp"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/hooks"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// GatewayFacade describes the gateway methods required by the HTTP layer.
type GatewayFacade interface {
	Account() (*client.User, *client.ProviderProfile)
	EnsureAccount(ctx context.Context, email string, roles []string, displayName string) (*client.User, *client.ProviderProfile, error)
	ListProviders(ctx context.Context) ([]client.ProviderProfile, error)
	ListServices(ctx context.Context, providerID *int64) ([]client.ServiceOffering, error)
	ListMyServices(ctx context.Context) ([]client.ServiceOffering, error)
	UsageSnapshot(ctx context.Context) (client.UsageSummary, error)
	MarketplaceAvailable() bool
	SetLocalAccount(user client.User, provider *client.ProviderProfile)
}

// Server exposes REST endpoints for the Tokligence Gateway.
type Server struct {
	gateway               GatewayFacade
	adapter               adapter.ChatAdapter
	embeddingAdapter      adapter.EmbeddingAdapter
	ledger                ledger.Store
	auth                  *auth.Manager
	identity              userstore.Store
	rootAdmin             *userstore.User
	hooks                 *hooks.Dispatcher
	enableAnthropicNative bool
	// passthrough + upstream configs
	anthPassthroughEnabled bool
	anthAPIKey             string
	anthBaseURL            string
	anthVersion            string
	openaiAPIKey           string
	openaiBaseURL          string
	// tool bridge behavior
	openaiToolBridgeStreamEnabled bool
	anthropicForceSSE             bool
	anthropicTokenCheckEnabled    bool
	anthropicMaxTokens            int
	authDisabled                  bool
	// logging
    logger   *log.Logger
    logLevel string
    // in-process sidecar handler
    sidecarMsgsHandler http.Handler
}

type bridgeExecResult struct {
	response         anthropicNativeResponse
	promptTokens     int
	completionTokens int
}

type bridgeUpstreamError struct {
	status int
	body   []byte
}

func (e bridgeUpstreamError) Error() string {
	if len(e.body) == 0 {
		return fmt.Sprintf("openai bridge upstream status %d", e.status)
	}
	preview := string(previewBytes(e.body, 256))
	return fmt.Sprintf("openai bridge upstream status %d: %s", e.status, preview)
}

// New constructs a Server with the required dependencies.
func New(gateway GatewayFacade, chatAdapter adapter.ChatAdapter, store ledger.Store, authManager *auth.Manager, identity userstore.Store, rootAdmin *userstore.User, dispatcher *hooks.Dispatcher, enableAnthropicNative bool) *Server {
	var rootCopy *userstore.User
	if rootAdmin != nil {
		copy := *rootAdmin
		copy.Email = strings.TrimSpace(strings.ToLower(copy.Email))
		rootCopy = &copy
	}

	// Check if chat adapter also supports embeddings
	var embAdapter adapter.EmbeddingAdapter
	if ea, ok := chatAdapter.(adapter.EmbeddingAdapter); ok {
		embAdapter = ea
	}

	server := &Server{gateway: gateway, adapter: chatAdapter, embeddingAdapter: embAdapter, ledger: store, auth: authManager, identity: identity, rootAdmin: rootCopy, hooks: dispatcher, enableAnthropicNative: enableAnthropicNative}

	return server
}

// Router returns a configured chi router for embedding in HTTP servers.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", s.handleAuthLogin)
		api.Post("/auth/verify", s.handleAuthVerify)

		api.Group(func(private chi.Router) {
			if s.auth != nil && !s.authDisabled {
				private.Use(s.sessionMiddleware)
			}
			private.Get("/profile", s.handleProfile)
			private.Get("/providers", s.handleProviders)
			private.Get("/services", s.handleServices)
			private.Get("/usage/summary", s.handleUsageSummary)
			private.Get("/usage/logs", s.handleUsageLogs)
		})

		api.Group(func(admin chi.Router) {
			if s.auth != nil {
				admin.Use(s.sessionMiddleware)
			}
			admin.Use(s.requireRootAdmin)
			admin.Get("/admin/users", s.handleAdminListUsers)
			admin.Post("/admin/users", s.handleAdminCreateUser)
			admin.Post("/admin/users/import", s.handleAdminImportUsers)
			admin.Patch("/admin/users/{id}", s.handleAdminUpdateUser)
			admin.Delete("/admin/users/{id}", s.handleAdminDeleteUser)
			admin.Get("/admin/users/{id}/api-keys", s.handleAdminListAPIKeys)
			admin.Post("/admin/users/{id}/api-keys", s.handleAdminCreateAPIKey)
			admin.Delete("/admin/api-keys/{id}", s.handleAdminDeleteAPIKey)
		})
	})

	r.Post("/v1/chat/completions", s.handleChatCompletions)
	r.Get("/v1/models", s.handleModels)
	r.Post("/v1/embeddings", s.handleEmbeddings)

	// Native provider endpoints (Anthropic-compatible)
	if s.enableAnthropicNative {
		r.Post("/anthropic/v1/messages", s.handleAnthropicMessages)
		// Minimal support for Claude clients that call count_tokens
		r.Post("/anthropic/v1/messages/count_tokens", s.handleAnthropicCountTokens)
	}

	return r
}

// handleAnthropicCountTokens provides a minimal implementation of the
// Anthropic-compatible count_tokens endpoint used by some clients (e.g. Claude Code)
// to budget max_tokens. It estimates tokens using a simple heuristic (4 chars ≈ 1 token).
func (s *Server) handleAnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
    rawBody, _ := io.ReadAll(r.Body)
    _ = r.Body.Close()
    var req anthropicNativeRequest
    if err := json.NewDecoder(bytes.NewReader(rawBody)).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, err)
        return
    }
    totalChars := 0
    if sys := extractSystemText(req.System); strings.TrimSpace(sys) != "" { totalChars += len(sys) }
    for _, m := range req.Messages {
        for _, b := range m.Content.Blocks {
            switch strings.ToLower(b.Type) {
            case "text":
                totalChars += len(b.Text)
            case "tool_use":
                if b.Input != nil { if bs, err := json.Marshal(b.Input); err == nil { totalChars += len(bs) } }
            case "tool_result":
                if b.Text != "" { totalChars += len(b.Text) }
                for _, sub := range b.Content { if strings.EqualFold(sub.Type, "text") { totalChars += len(sub.Text) } }
            }
        }
    }
    tokens := totalChars/4 + 1
    if tokens < len(req.Messages)*2 { tokens = len(req.Messages) * 2 }
    if s.isDebug() { s.debugf("anthropic.count_tokens: model=%s input_chars=%d input_tokens~=%d", req.Model, totalChars, tokens) }
    s.respondJSON(w, http.StatusOK, map[string]any{"input_tokens": tokens})
}

// SetUpstreams configures upstream credentials and mode toggles for native endpoints and bridges.
func (s *Server) SetUpstreams(openaiKey, openaiBase string, anthKey, anthBase, anthVer string, anthPassthrough bool, openaiToolBridgeStream bool, forceSSE bool, tokenCheck bool, maxTokens int, openaiCompletionMax int) {
	s.openaiAPIKey = strings.TrimSpace(openaiKey)
	s.openaiBaseURL = strings.TrimRight(strings.TrimSpace(openaiBase), "/")
	if s.openaiBaseURL == "" {
		s.openaiBaseURL = "https://api.openai.com/v1"
	}
	s.anthAPIKey = strings.TrimSpace(anthKey)
	s.anthBaseURL = strings.TrimRight(strings.TrimSpace(anthBase), "/")
	if s.anthBaseURL == "" {
		s.anthBaseURL = "https://api.anthropic.com"
	}
	s.anthVersion = strings.TrimSpace(anthVer)
	if s.anthVersion == "" {
		s.anthVersion = "2023-06-01"
	}
	s.anthPassthroughEnabled = anthPassthrough
	s.openaiToolBridgeStreamEnabled = openaiToolBridgeStream
	s.anthropicForceSSE = forceSSE
	s.anthropicTokenCheckEnabled = tokenCheck
    s.anthropicMaxTokens = maxTokens

    // Build an in-process sidecar messages handler for the Anthropic endpoint
    // so we avoid duplicating proxy logic. This mirrors the proven sidecar behavior.
    scfg := translationhttp.Config{
        OpenAIBaseURL:      s.openaiBaseURL,
        OpenAIAPIKey:       s.openaiAPIKey,
        AnthropicBaseURL:   s.anthBaseURL,
        AnthropicAPIKey:    s.anthAPIKey,
        AnthropicVersion:   s.anthVersion,
        DefaultOpenAIModel: "gpt-4o",
        MaxTokensCap:       openaiCompletionMax,
    }
    s.sidecarMsgsHandler = translationhttp.NewMessagesHandler(scfg, http.DefaultClient)
}

func (s *Server) SetAuthDisabled(disabled bool) {
	s.authDisabled = disabled
	if disabled && s.isDebug() {
		s.debugf("authorization disabled via configuration")
	}
}

// SetLogger configures server-level logger and verbosity ("debug", "info", ...).
func (s *Server) SetLogger(level string, logger *log.Logger) {
	s.logLevel = strings.ToLower(strings.TrimSpace(level))
	if logger != nil {
		s.logger = logger
	}
}

// SetBridgeSessionConfig configures the bridge session manager
func (s *Server) SetBridgeSessionConfig(enabled bool, ttl string, maxCount int) error {
	// Session-based deduplication is intentionally disabled for stateless bridging.
	// This method is kept as a no-op to preserve config compatibility.
	if s.isDebug() {
		s.debugf("Bridge session manager disabled (stateless mode). Requested enabled=%v ttl=%s max=%d", enabled, ttl, maxCount)
	}
	return nil
}

func (s *Server) isDebug() bool { return s.logLevel == "debug" }
func (s *Server) debugf(format string, args ...any) {
	if s.logger != nil && s.isDebug() {
		s.logger.Printf("DEBUG "+format, args...)
	}
}

func (s *Server) recordBridgeLedger(ctx context.Context, memo string, promptTokens, completionTokens int, sessionUser *userstore.User, apiKey *userstore.APIKey) {
	if s.ledger == nil {
		return
	}
	var uid int64
	if sessionUser != nil {
		uid = sessionUser.ID
	} else if user, _ := s.gateway.Account(); user != nil {
		uid = user.ID
	}
	if uid == 0 {
		return
	}
	entry := ledger.Entry{
		UserID:           uid,
		PromptTokens:     int64(promptTokens),
		CompletionTokens: int64(completionTokens),
		Direction:        ledger.DirectionConsume,
		Memo:             memo,
	}
	if apiKey != nil {
		id := apiKey.ID
		entry.APIKeyID = &id
	}
	_ = s.ledger.Record(ctx, entry)
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	info := sessionFromContext(r.Context())
	user, provider := s.gateway.Account()
	if info != nil && info.clientUser != nil {
		user = info.clientUser
	}
	if user == nil {
		s.respondError(w, http.StatusServiceUnavailable, errors.New("gateway not initialised"))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"provider":    provider,
		"marketplace": map[string]any{"connected": s.gateway.MarketplaceAvailable()},
	})
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := s.gateway.ListProviders(r.Context())
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	var (
		services []client.ServiceOffering
		err      error
	)
	switch scope {
	case "mine":
		services, err = s.gateway.ListMyServices(r.Context())
	default:
		services, err = s.gateway.ListServices(r.Context(), nil)
	}
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"services": services})
}

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	user, _ := s.gateway.Account()
	if s.ledger != nil && user != nil {
		summary, err := s.ledger.Summary(r.Context(), user.ID)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err)
			return
		}
		s.respondJSON(w, http.StatusOK, map[string]any{"summary": summary})
		return
	}
	summary, err := s.gateway.UsageSnapshot(r.Context())
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	var (
		sessionUser *userstore.User
		apiKey      *userstore.APIKey
	)
	if s.identity != nil && !s.authDisabled {
		var err error
		sessionUser, apiKey, err = s.authenticateAPIKeyRequest(r)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		if sessionUser != nil {
			s.applySessionUser(sessionUser)
		}
	}
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	// Streaming branch
	if req.Stream {
		if sa, ok := s.adapter.(adapter.StreamingChatAdapter); ok {
                w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, _ := w.(http.Flusher)

			ch, err := sa.CreateCompletionStream(r.Context(), req)
			if err != nil {
				s.respondError(w, http.StatusBadGateway, err)
				return
			}
			// Stream loop
			enc := json.NewEncoder(w)
			// Approximate accounting for streaming
			var completionChars int
			approxPromptTokens := approximatePromptTokens(req)
			for ev := range ch {
				if ev.Error != nil {
					// End the stream on error
					_, _ = io.WriteString(w, "data: {\"error\": \"stream error\"}\n\n")
					if flusher != nil {
						flusher.Flush()
					}
					return
				}
				if ev.Chunk != nil {
					// Encode chunk payload following OpenAI SSE semantics
					completionChars += len(ev.Chunk.GetDelta().Content)
					_, _ = io.WriteString(w, "data: ")
					if err := enc.Encode(ev.Chunk); err != nil {
						return
					}
					_, _ = io.WriteString(w, "\n")
					if flusher != nil {
						flusher.Flush()
					}
				}
			}
			// Finish signal
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			// Record ledger if enabled
			if s.ledger != nil {
				var ledgerUserID int64
				if sessionUser != nil {
					ledgerUserID = sessionUser.ID
				} else if user, _ := s.gateway.Account(); user != nil {
					ledgerUserID = user.ID
				}
				if ledgerUserID != 0 {
					entry := ledger.Entry{
						UserID:           ledgerUserID,
						ServiceID:        0,
						PromptTokens:     int64(approxPromptTokens),
						CompletionTokens: int64(completionChars / 4),
						Direction:        ledger.DirectionConsume,
						Memo:             "chat.completions(stream)",
					}
					if apiKey != nil {
						id := apiKey.ID
						entry.APIKeyID = &id
					}
					_ = s.ledger.Record(r.Context(), entry)
				}
			}
			return
		}
		// If adapter doesn't support streaming, fall back to non-streaming
	}

	resp, err := s.adapter.CreateCompletion(r.Context(), req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	if s.ledger != nil {
		var ledgerUserID int64
		if sessionUser != nil {
			ledgerUserID = sessionUser.ID
		} else if user, _ := s.gateway.Account(); user != nil {
			ledgerUserID = user.ID
		}
		if ledgerUserID != 0 {
			entry := ledger.Entry{
				UserID:           ledgerUserID,
				ServiceID:        0,
				PromptTokens:     int64(resp.Usage.PromptTokens),
				CompletionTokens: int64(resp.Usage.CompletionTokens),
				Direction:        ledger.DirectionConsume,
				Memo:             "chat.completions",
			}
			if apiKey != nil {
				id := apiKey.ID
				entry.APIKeyID = &id
			}
			_ = s.ledger.Record(r.Context(), entry)
		}
	}
	s.respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUsageLogs(w http.ResponseWriter, r *http.Request) {
	if s.ledger == nil {
		s.respondJSON(w, http.StatusOK, map[string]any{"entries": []ledger.Entry{}})
		return
	}
	user, _ := s.gateway.Account()
	if user == nil {
		s.respondError(w, http.StatusServiceUnavailable, errors.New("gateway not initialised"))
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	entries, err := s.ledger.ListRecent(r.Context(), user.ID, limit)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		s.respondError(w, http.StatusNotImplemented, errors.New("auth disabled"))
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		s.respondError(w, http.StatusBadRequest, errors.New("email required"))
		return
	}
	if s.rootAdmin != nil && strings.EqualFold(email, s.rootAdmin.Email) {
		token, err := s.auth.IssueToken(email, 24*time.Hour)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, err)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "tokligence_session",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   false,
			Expires:  time.Now().Add(24 * time.Hour),
		})

		localUser := &userstore.User{ID: s.rootAdmin.ID, Email: s.rootAdmin.Email, Role: userstore.RoleRootAdmin, Status: userstore.StatusActive}
		clientUser := s.applySessionUser(localUser)
		s.respondJSON(w, http.StatusOK, map[string]any{
			"token":       token,
			"user":        clientUser,
			"provider":    nil,
			"marketplace": map[string]any{"connected": s.gateway.MarketplaceAvailable()},
		})
		return
	}
	challengeID, code, expires, err := s.auth.CreateChallenge(email)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	s.respondJSON(w, http.StatusOK, map[string]any{
		"challenge_id": challengeID,
		"expires_at":   expires.UTC().Format(time.RFC3339),
		"code":         code,
	})
}

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		s.respondError(w, http.StatusNotImplemented, errors.New("auth disabled"))
		return
	}
	var req struct {
		ChallengeID    string `json:"challenge_id"`
		Code           string `json:"code"`
		DisplayName    string `json:"display_name"`
		EnableProvider bool   `json:"enable_provider"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	challengeID := strings.TrimSpace(req.ChallengeID)
	code := strings.TrimSpace(req.Code)
	if challengeID == "" || code == "" {
		s.respondError(w, http.StatusBadRequest, errors.New("challenge id and code required"))
		return
	}
	email, err := s.auth.VerifyChallenge(challengeID, code)
	if err != nil {
		s.respondError(w, http.StatusUnauthorized, err)
		return
	}
	roles := []string{"consumer"}
	if req.EnableProvider {
		roles = append(roles, "provider")
	}
	display := strings.TrimSpace(req.DisplayName)
	if display == "" {
		display = email
	}
	user, provider, err := s.gateway.EnsureAccount(r.Context(), email, roles, display)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	if s.identity != nil {
		stored, serr := s.identity.FindByEmail(r.Context(), email)
		if serr == nil && stored == nil {
			stored, serr = s.identity.CreateUser(r.Context(), email, userstore.RoleGatewayUser, display)
		}
		if serr == nil && stored != nil {
			if stored.DisplayName != display {
				stored, _ = s.identity.UpdateUser(r.Context(), stored.ID, display, stored.Role)
			}
			user = s.applySessionUser(stored)
		}
	}
	token, err := s.auth.IssueToken(email, 24*time.Hour)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "tokligence_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	s.respondJSON(w, http.StatusOK, map[string]any{
		"token":       token,
		"user":        user,
		"provider":    provider,
		"marketplace": map[string]any{"connected": s.gateway.MarketplaceAvailable()},
	})
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.identity.ListUsers(r.Context())
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]map[string]any, 0, len(users))
	for i := range users {
		user := users[i]
		resp = append(resp, toUserPayload(&user))
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"users": resp})
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	role := userstore.Role(strings.TrimSpace(req.Role))
	user, err := s.identity.CreateUser(r.Context(), req.Email, role, strings.TrimSpace(req.DisplayName))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	s.emitUserHook(r.Context(), hooks.EventUserProvisioned, user)
	s.respondJSON(w, http.StatusCreated, map[string]any{"user": toUserPayload(user)})
}

func (s *Server) handleAdminImportUsers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Users []struct {
			Email       string `json:"email"`
			Role        string `json:"role"`
			DisplayName string `json:"display_name"`
		} `json:"users"`
		SkipExisting bool `json:"skip_existing"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Users) == 0 {
		s.respondJSON(w, http.StatusOK, map[string]any{"created": []map[string]any{}, "skipped": []map[string]string{}})
		return
	}
	created := make([]map[string]any, 0, len(req.Users))
	skipped := make([]map[string]string, 0)
	for idx, item := range req.Users {
		email := strings.TrimSpace(item.Email)
		if email == "" {
			skipped = append(skipped, map[string]string{"index": strconv.Itoa(idx), "reason": "missing email"})
			continue
		}
		role := strings.TrimSpace(item.Role)
		if role == "" {
			role = string(userstore.RoleGatewayUser)
		}
		user, err := s.identity.CreateUser(r.Context(), email, userstore.Role(role), strings.TrimSpace(item.DisplayName))
		if err != nil {
			if req.SkipExisting && isDuplicateUserError(err) {
				skipped = append(skipped, map[string]string{"email": email, "reason": "already exists"})
				continue
			}
			s.respondError(w, http.StatusBadRequest, fmt.Errorf("user %s: %w", email, err))
			return
		}
		s.emitUserHook(r.Context(), hooks.EventUserProvisioned, user)
		created = append(created, toUserPayload(user))
	}
	status := http.StatusCreated
	if len(created) == 0 {
		status = http.StatusOK
	}
	s.respondJSON(w, status, map[string]any{"created": created, "skipped": skipped})
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Status      string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.identity.GetUser(r.Context(), id)
	if err != nil || user == nil {
		s.respondError(w, http.StatusNotFound, errors.New("user not found"))
		return
	}
	if display := strings.TrimSpace(req.DisplayName); display != "" || strings.TrimSpace(req.Role) != "" {
		role := user.Role
		if roleOverride := strings.TrimSpace(req.Role); roleOverride != "" {
			role = userstore.Role(roleOverride)
		}
		user, err = s.identity.UpdateUser(r.Context(), id, display, role)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, err)
			return
		}
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		if err := s.identity.SetUserStatus(r.Context(), id, userstore.Status(status)); err != nil {
			state := http.StatusBadRequest
			if errors.Is(err, sql.ErrNoRows) {
				state = http.StatusNotFound
			}
			s.respondError(w, state, err)
			return
		}
		user.Status = userstore.Status(status)
		user.UpdatedAt = time.Now().UTC()
	}
	s.emitUserHook(r.Context(), hooks.EventUserUpdated, user)
	s.respondJSON(w, http.StatusOK, map[string]any{"user": toUserPayload(user)})
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	user, _ := s.identity.GetUser(r.Context(), id)
	if err := s.identity.DeleteUser(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		s.respondError(w, status, err)
		return
	}
	if user != nil {
		s.emitUserHook(r.Context(), hooks.EventUserDeleted, user)
	}
	s.respondJSON(w, http.StatusNoContent, nil)
}

func (s *Server) handleAdminListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	keys, err := s.identity.ListAPIKeys(r.Context(), userID)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, toAPIKeyPayload(k))
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"api_keys": resp})
}

func (s *Server) handleAdminCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}
	var req struct {
		Scopes    []string `json:"scopes"`
		ExpiresAt string   `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	var expires *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, err)
			return
		}
		expires = &t
	}
	key, token, err := s.identity.CreateAPIKey(r.Context(), userID, req.Scopes, expires)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	s.respondJSON(w, http.StatusCreated, map[string]any{
		"token":   token,
		"api_key": toAPIKeyPayload(*key),
	})
}

func (s *Server) handleAdminDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, errors.New("invalid api key id"))
		return
	}
	if err := s.identity.DeleteAPIKey(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		s.respondError(w, status, err)
		return
	}
	s.respondJSON(w, http.StatusNoContent, nil)
}

func (s *Server) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := s.authenticateRequest(r)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		ctx := context.WithValue(r.Context(), sessionContextKey{}, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authenticateRequest(r *http.Request) (*sessionInfo, error) {
	if s.identity == nil {
		return nil, errors.New("identity store unavailable")
	}

	if token := bearerToken(r.Header.Get("Authorization")); token != "" {
		key, user, err := s.identity.LookupAPIKey(r.Context(), token)
		if err != nil {
			return nil, err
		}
		if key == nil || user == nil || user.Status != userstore.StatusActive {
			return nil, errors.New("invalid api key")
		}
		clientUser := s.applySessionUser(user)
		return &sessionInfo{user: user, clientUser: clientUser, viaAPIKey: true}, nil
	}

	cookie, err := r.Cookie("tokligence_session")
	if err != nil || cookie.Value == "" {
		return nil, errors.New("missing session")
	}
	email, err := s.auth.ValidateToken(cookie.Value)
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(strings.ToLower(email))
	var user *userstore.User
	if s.identity != nil {
		user, err = s.identity.FindByEmail(r.Context(), email)
		if err != nil {
			return nil, err
		}
	}
	if user == nil && s.rootAdmin != nil && strings.EqualFold(s.rootAdmin.Email, email) {
		user = &userstore.User{ID: s.rootAdmin.ID, Email: s.rootAdmin.Email, Role: userstore.RoleRootAdmin, Status: userstore.StatusActive}
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if user.Status != userstore.StatusActive {
		return nil, errors.New("user inactive")
	}
	clientUser := s.applySessionUser(user)
	return &sessionInfo{user: user, clientUser: clientUser}, nil
}

func (s *Server) authenticateAPIKeyRequest(r *http.Request) (*userstore.User, *userstore.APIKey, error) {
	if s.authDisabled {
		return nil, nil, nil
	}
	if s.identity == nil {
		return nil, nil, errors.New("identity store unavailable")
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if token == "" {
		return nil, nil, errors.New("missing api key")
	}
	key, user, err := s.identity.LookupAPIKey(r.Context(), token)
	if err != nil {
		return nil, nil, err
	}
	if key == nil || user == nil || user.Status != userstore.StatusActive {
		return nil, nil, errors.New("invalid api key")
	}
	return user, key, nil
}

func (s *Server) applySessionUser(user *userstore.User) *client.User {
	if user == nil {
		return nil
	}
	roles := []string{}
	switch user.Role {
	case userstore.RoleRootAdmin:
		roles = append(roles, "root_admin", "consumer")
	case userstore.RoleGatewayAdmin:
		roles = append(roles, "gateway_admin", "consumer")
	default:
		roles = append(roles, "consumer")
	}
	cUser := client.User{
		ID:    user.ID,
		Email: user.Email,
		Roles: roles,
	}
	_, existingProvider := s.gateway.Account()
	s.gateway.SetLocalAccount(cUser, existingProvider)
	return &cUser
}

func (s *Server) requireRootAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := sessionFromContext(r.Context())
		if info == nil || info.user == nil || info.user.Role != userstore.RoleRootAdmin {
			s.respondError(w, http.StatusForbidden, errors.New("admin access required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) emitUserHook(ctx context.Context, eventType hooks.EventType, user *userstore.User) {
	if s.hooks == nil || user == nil {
		return
	}
	metadata := map[string]any{
		"email":        user.Email,
		"role":         user.Role,
		"display_name": user.DisplayName,
		"status":       user.Status,
	}
	evt := hooks.Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		UserID:     strconv.FormatInt(user.ID, 10),
		Metadata:   metadata,
	}
	_ = s.hooks.Emit(ctx, evt)
}

func sessionFromContext(ctx context.Context) *sessionInfo {
	info, _ := ctx.Value(sessionContextKey{}).(*sessionInfo)
	return info
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func isDuplicateUserError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, payload any) {
	if payload == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) respondError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	s.respondJSON(w, status, map[string]any{"error": err.Error()})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	// Try to build dynamic model list from router routes if available
	now := time.Now().Unix()
	if lr, ok := s.adapter.(interface{ ListRoutes() map[string]string }); ok {
		routes := lr.ListRoutes()
		models := make([]openai.Model, 0, len(routes)+1)
		seen := map[string]bool{}
		for pattern, owner := range routes {
			// Only include exact IDs (skip wildcards) for clarity
			if strings.Contains(pattern, "*") {
				continue
			}
			if pattern == "" || seen[pattern] {
				continue
			}
			models = append(models, openai.NewModel(pattern, owner, now))
			seen[pattern] = true
		}
		if !seen["loopback"] {
			models = append(models, openai.NewModel("loopback", "tokligence", now))
		}
		s.respondJSON(w, http.StatusOK, openai.NewModelsResponse(models))
		return
	}

	// Fallback to static list
	models := []openai.Model{
		openai.NewModel("loopback", "tokligence", now),
		openai.NewModel("gpt-4", "openai", now),
		openai.NewModel("gpt-4-turbo", "openai", now),
		openai.NewModel("gpt-3.5-turbo", "openai", now),
		openai.NewModel("claude-3-5-sonnet-20241022", "anthropic", now),
	}
	s.respondJSON(w, http.StatusOK, openai.NewModelsResponse(models))
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	if s.embeddingAdapter == nil {
		s.respondError(w, http.StatusNotImplemented, errors.New("embeddings not supported by current adapter"))
		return
	}

	var (
		sessionUser *userstore.User
		apiKey      *userstore.APIKey
	)
	if s.identity != nil && !s.authDisabled {
		var err error
		sessionUser, apiKey, err = s.authenticateAPIKeyRequest(r)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		if sessionUser != nil {
			s.applySessionUser(sessionUser)
		}
	}

	var req openai.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := s.embeddingAdapter.CreateEmbedding(r.Context(), req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}

	if s.ledger != nil {
		var ledgerUserID int64
		if sessionUser != nil {
			ledgerUserID = sessionUser.ID
		} else if user, _ := s.gateway.Account(); user != nil {
			ledgerUserID = user.ID
		}
		if ledgerUserID != 0 {
			entry := ledger.Entry{
				UserID:           ledgerUserID,
				ServiceID:        0,
				PromptTokens:     int64(resp.Usage.PromptTokens),
				CompletionTokens: 0,
				Direction:        ledger.DirectionConsume,
				Memo:             "embeddings",
			}
			if apiKey != nil {
				id := apiKey.ID
				entry.APIKeyID = &id
			}
			_ = s.ledger.Record(r.Context(), entry)
		}
	}

	s.respondJSON(w, http.StatusOK, resp)
}

// --- Anthropic native endpoint support ---
type anthropicNativeRequest struct {
	Model       string                   `json:"model"`
	Messages    []anthropicNativeMessage `json:"messages"`
	System      anthropicSystemField     `json:"system,omitempty"`
	Tools       []anthropicTool          `json:"tools,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
	Temperature *float64                 `json:"temperature,omitempty"`
	TopP        *float64                 `json:"top_p,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicNativeMessage struct {
	Role    string                 `json:"role"`
	Content anthropicNativeContent `json:"content"`
}

// anthropicNativeContent supports either a string or an array of content blocks.
type anthropicNativeContent struct {
	Blocks []anthropicNativeContentBlock
}

func (c *anthropicNativeContent) UnmarshalJSON(b []byte) error {
	// If it's a quoted string, wrap as a single text block
	btrim := strings.TrimSpace(string(b))
	if len(btrim) > 0 && btrim[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		c.Blocks = []anthropicNativeContentBlock{{Type: "text", Text: s}}
		return nil
	}
	// If it's an object, try to extract nested {text|content}
	if len(btrim) > 0 && btrim[0] == '{' {
		// Accept shapes like {"text":"..."} or {"content":"..."} or {"content":[blocks]}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		if raw, ok := obj["text"]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				c.Blocks = []anthropicNativeContentBlock{{Type: "text", Text: s}}
				return nil
			}
		}
		if raw, ok := obj["content"]; ok {
			// content can be string or array of blocks
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				c.Blocks = []anthropicNativeContentBlock{{Type: "text", Text: s}}
				return nil
			}
			var arr []anthropicNativeContentBlock
			if err := json.Unmarshal(raw, &arr); err == nil {
				c.Blocks = arr
				return nil
			}
		}
		// Fallback: try to parse as array-like fields
		var arr []anthropicNativeContentBlock
		if err := json.Unmarshal(b, &arr); err == nil {
			c.Blocks = arr
			return nil
		}
		// As a last resort, keep empty (caller should handle)
		c.Blocks = nil
		return nil
	}
	// Otherwise expect an array of blocks
	var arr []anthropicNativeContentBlock
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	c.Blocks = arr
	return nil
}

type anthropicNativeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`
	// tool_result
	ToolUseID string                        `json:"tool_use_id,omitempty"`
	IsError   bool                          `json:"is_error,omitempty"`
	Content   []anthropicNativeContentBlock `json:"content,omitempty"`
}

// Accept flexible shapes for nested content blocks, especially tool_result.content
// which may arrive as a string or as an array of blocks.
func (b *anthropicNativeContentBlock) UnmarshalJSON(data []byte) error {
	type alias anthropicNativeContentBlock // avoid recursion
	// Fast-path: try normal decode first
	var a alias
	if err := json.Unmarshal(data, (*alias)(&a)); err == nil {
		*b = anthropicNativeContentBlock(a)
		return nil
	}
	// Fallback tolerant parsing
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	// type
	if v, ok := raw["type"]; ok {
		_ = json.Unmarshal(v, &b.Type)
	}
	// simple fields
	if v, ok := raw["text"]; ok {
		_ = json.Unmarshal(v, &b.Text)
	}
	if v, ok := raw["id"]; ok {
		_ = json.Unmarshal(v, &b.ID)
	}
	if v, ok := raw["name"]; ok {
		_ = json.Unmarshal(v, &b.Name)
	}
	if v, ok := raw["input"]; ok {
		// keep as generic JSON value
		var anyv interface{}
		if err := json.Unmarshal(v, &anyv); err == nil {
			b.Input = anyv
		}
	}
	if v, ok := raw["tool_use_id"]; ok {
		_ = json.Unmarshal(v, &b.ToolUseID)
	}
	if v, ok := raw["is_error"]; ok {
		_ = json.Unmarshal(v, &b.IsError)
	}
	// content: accept string | object(single block) | array<blocks>
	if v, ok := raw["content"]; ok && len(v) > 0 && string(v) != "null" {
		// try string
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			b.Content = []anthropicNativeContentBlock{{Type: "text", Text: s}}
			return nil
		}
		// try array of blocks
		var arr []anthropicNativeContentBlock
		if err := json.Unmarshal(v, &arr); err == nil {
			b.Content = arr
			return nil
		}
		// try single block object
		var one anthropicNativeContentBlock
		if err := json.Unmarshal(v, &one); err == nil {
			b.Content = []anthropicNativeContentBlock{one}
			return nil
		}
	}
	return nil
}

// anthropicSystemField supports string or array<content_block>.
type anthropicSystemField struct {
	Text   string
	Blocks []anthropicNativeContentBlock
}

func (s *anthropicSystemField) UnmarshalJSON(b []byte) error {
	btrim := strings.TrimSpace(string(b))
	if btrim == "" || btrim == "null" {
		return nil
	}
	if len(btrim) > 0 && btrim[0] == '"' {
		var text string
		if err := json.Unmarshal(b, &text); err != nil {
			return err
		}
		s.Text = text
		return nil
	}
	var arr []anthropicNativeContentBlock
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	s.Blocks = arr
	return nil
}

type anthropicNativeResponse struct {
	ID         string                        `json:"id"`
	Type       string                        `json:"type"`
	Role       string                        `json:"role"`
	Content    []anthropicNativeContentBlock `json:"content"`
	Model      string                        `json:"model"`
	StopReason string                        `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
    if s.sidecarMsgsHandler != nil {
        s.sidecarMsgsHandler.ServeHTTP(w, r)
        return
    }
    s.respondError(w, http.StatusNotImplemented, errors.New("anthropic messages handler not available"))
}

// approximatePromptTokens estimates tokens from request messages (4 chars ~ 1 token).
func (s *Server) guardAnthropicTokens(req anthropicNativeRequest) error {
	if !s.anthropicTokenCheckEnabled {
		return nil
	}
	if req.MaxTokens <= 0 {
		return fmt.Errorf("anthropic: max_tokens required when token guard enabled")
	}
	if s.anthropicMaxTokens > 0 && req.MaxTokens > s.anthropicMaxTokens {
		return fmt.Errorf("anthropic: max_tokens %d exceeds limit %d", req.MaxTokens, s.anthropicMaxTokens)
	}
	return nil
}

func approximatePromptTokens(req openai.ChatCompletionRequest) int {
	total := 0
	for _, m := range req.Messages {
		total += len(m.Content)
	}
	// ensure non-zero for accounting visibility
	n := total/4 + 1
	if n < len(req.Messages)*2 { // minimum overhead per message
		n = len(req.Messages) * 2
	}
	return n
}

// extractSystemText flattens system field (string or blocks) into a single string.
func extractSystemText(sys anthropicSystemField) string {
	if strings.TrimSpace(sys.Text) != "" {
		return sys.Text
	}
	var b strings.Builder
	for _, block := range sys.Blocks {
		if strings.EqualFold(block.Type, "text") {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// --- Native passthrough implementation ---
func (s *Server) anthropicPassthrough(w http.ResponseWriter, r *http.Request, raw []byte, stream bool, sessionUser *userstore.User, apiKey *userstore.APIKey) {
    // Clamp excessive max_tokens to avoid OpenAI 400s via sidecar
    raw = clampAnthropicMaxTokens(raw, 16384)
    url := s.anthBaseURL + "/v1/messages"
	if q := r.URL.RawQuery; q != "" {
		url += "?" + q
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
    if strings.TrimSpace(s.anthAPIKey) != "" {
        req.Header.Set("x-api-key", s.anthAPIKey)
    }
	req.Header.Set("anthropic-version", s.anthVersion)
	s.debugf("anthropic.passthrough: POST %s stream=%v", url, stream)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	s.debugf("anthropic.passthrough: status=%d", resp.StatusCode)
	// Copy headers of interest
	for k, vals := range resp.Header {
		if strings.EqualFold(k, "content-type") {
			w.Header()[k] = vals
		}
	}
	w.WriteHeader(resp.StatusCode)
	if stream {
		flusher, _ := w.(http.Flusher)
		// Best-effort passthrough for SSE; accounting can be added later
		io.Copy(w, resp.Body)
		if flusher != nil {
			flusher.Flush()
		}
		return
	}
	// Non-stream: copy body and record usage if possible
	body, _ := io.ReadAll(resp.Body)
	if s.isDebug() {
		s.debugf("anthropic.passthrough: body=%s", string(previewBytes(body, 512)))
	}
	_, _ = w.Write(body)
	if s.ledger != nil && resp.StatusCode == http.StatusOK {
		var ar struct {
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(body, &ar) == nil {
			var uid int64
			if sessionUser != nil {
				uid = sessionUser.ID
			} else if u, _ := s.gateway.Account(); u != nil {
				uid = u.ID
			}
			if uid != 0 {
				entry := ledger.Entry{UserID: uid, PromptTokens: int64(ar.Usage.InputTokens), CompletionTokens: int64(ar.Usage.OutputTokens), Direction: ledger.DirectionConsume, Memo: "anthropic.messages(passthrough)"}
				if apiKey != nil {
					id := apiKey.ID
					entry.APIKeyID = &id
				}
				_ = s.ledger.Record(r.Context(), entry)
			}
		}
	}
}

// clampAnthropicMaxTokens reduces the "max_tokens" field in an Anthropic-compatible
// request body if it exceeds the provided cap. It returns the original body on parse errors.
func clampAnthropicMaxTokens(body []byte, capTokens int) []byte {
    if capTokens <= 0 || len(body) == 0 {
        return body
    }
    var m map[string]any
    if err := json.Unmarshal(body, &m); err != nil {
        return body
    }
    if v, ok := m["max_tokens"]; ok {
        switch t := v.(type) {
        case float64:
            if int(t) > capTokens {
                m["max_tokens"] = capTokens
            }
        case int:
            if t > capTokens {
                m["max_tokens"] = capTokens
            }
        }
    }
    b, err := json.Marshal(m)
    if err != nil {
        return body
    }
    return b
}

// --- OpenAI tool bridge (non-streaming) disabled in favor of sidecar
func (s *Server) executeOpenAIToolBridge(ctx context.Context, areq anthropicNativeRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) (bridgeExecResult, error) {
    return bridgeExecResult{}, errors.New("openai tool bridge disabled in favor of sidecar")
}

func (s *Server) openaiToolBridge(w http.ResponseWriter, r *http.Request, areq anthropicNativeRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) {
    s.respondError(w, http.StatusNotImplemented, errors.New("openai tool bridge disabled in favor of sidecar"))
}

func (s *Server) openaiToolBridgeBatchSSE(w http.ResponseWriter, r *http.Request, areq anthropicNativeRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) {
    s.respondError(w, http.StatusNotImplemented, errors.New("openai tool bridge disabled in favor of sidecar"))
}

// extractLastUserMessage extracts the last user text message from the request
// extractLastUserMessage was used for session-based dedup (now removed).
// Keeping this placeholder ensures stable diffs if re-introduced in future.

// logToolBlocksPreview emits a compact summary of tool_use/tool_result blocks for diagnostics.
// logToolBlocksPreview removed (legacy bridge diagnostics no longer applicable)

// --- OpenAI tool bridge (streaming): forward OpenAI SSE deltas as Anthropic-style content_block_delta ---
// openaiToolBridgeStream removed (sidecar handles streaming tool bridge)

func previewBytes(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

// toolInputChunks removed (legacy bridge)

type sessionContextKey struct{}

type sessionInfo struct {
	user       *userstore.User
	clientUser *client.User
	viaAPIKey  bool
}

func toUserPayload(user *userstore.User) map[string]any {
	if user == nil {
		return nil
	}
	return map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"role":         user.Role,
		"display_name": user.DisplayName,
		"status":       user.Status,
		"created_at":   user.CreatedAt,
		"updated_at":   user.UpdatedAt,
	}
}

func toAPIKeyPayload(key userstore.APIKey) map[string]any {
	var expires interface{}
	if key.ExpiresAt != nil {
		expires = key.ExpiresAt
	}
	return map[string]any{
		"id":         key.ID,
		"user_id":    key.UserID,
		"prefix":     key.Prefix,
		"scopes":     key.Scopes,
		"expires_at": expires,
		"created_at": key.CreatedAt,
		"updated_at": key.UpdatedAt,
	}
}
