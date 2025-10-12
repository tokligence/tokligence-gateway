package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/auth"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// GatewayFacade describes the gateway methods required by the HTTP layer.
type GatewayFacade interface {
	Account() (*client.User, *client.ProviderProfile)
	EnsureAccount(ctx context.Context, email string, roles []string, displayName string) (*client.User, *client.ProviderProfile, error)
	ListProviders(ctx context.Context) ([]client.ProviderProfile, error)
	ListServices(ctx context.Context, providerID *int64) ([]client.ServiceOffering, error)
	ListMyServices(ctx context.Context) ([]client.ServiceOffering, error)
	UsageSnapshot(ctx context.Context) (client.UsageSummary, error)
}

// Server exposes REST endpoints for the Tokligence Gateway.
type Server struct {
	gateway GatewayFacade
	adapter adapter.ChatAdapter
	ledger  ledger.Store
	auth    *auth.Manager
}

// New constructs a Server with the required dependencies.
func New(gateway GatewayFacade, chatAdapter adapter.ChatAdapter, store ledger.Store, authManager *auth.Manager) *Server {
	return &Server{gateway: gateway, adapter: chatAdapter, ledger: store, auth: authManager}
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
			if s.auth != nil {
				private.Use(s.sessionMiddleware)
			}
			private.Get("/profile", s.handleProfile)
			private.Get("/providers", s.handleProviders)
			private.Get("/services", s.handleServices)
			private.Get("/usage/summary", s.handleUsageSummary)
			private.Get("/usage/logs", s.handleUsageLogs)
		})
	})

	r.Post("/v1/chat/completions", s.handleChatCompletions)

	return r
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	user, provider := s.gateway.Account()
	if user == nil {
		s.respondError(w, http.StatusServiceUnavailable, errors.New("gateway not initialised"))
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]any{
		"user":     user,
		"provider": provider,
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
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.adapter.CreateCompletion(r.Context(), req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	if s.ledger != nil {
		if user, _ := s.gateway.Account(); user != nil {
			_ = s.ledger.Record(r.Context(), ledger.Entry{
				UserID:           user.ID,
				ServiceID:        0,
				PromptTokens:     int64(resp.Usage.PromptTokens),
				CompletionTokens: int64(resp.Usage.CompletionTokens),
				Direction:        ledger.DirectionConsume,
				Memo:             "chat.completions",
			})
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
	email, err := s.auth.VerifyChallenge(strings.TrimSpace(req.ChallengeID), strings.TrimSpace(req.Code))
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
	user, _, err := s.gateway.EnsureAccount(r.Context(), email, roles, display)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
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
		"token": token,
		"user":  user,
	})
}

func (s *Server) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie("tokligence_session")
		if err != nil || cookie.Value == "" {
			s.respondError(w, http.StatusUnauthorized, errors.New("missing session"))
			return
		}
		email, err := s.auth.ValidateToken(cookie.Value)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		if err := s.ensureGatewayAccount(r.Context(), email); err != nil {
			s.respondError(w, http.StatusBadGateway, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionEmailKey{}, email)))
	})
}

func (s *Server) ensureGatewayAccount(ctx context.Context, email string) error {
	user, _ := s.gateway.Account()
	if user != nil && strings.EqualFold(user.Email, email) {
		return nil
	}
	_, _, err := s.gateway.EnsureAccount(ctx, email, []string{"consumer"}, email)
	return err
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) respondError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	s.respondJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

type sessionEmailKey struct{}
