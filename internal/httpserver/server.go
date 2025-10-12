package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// GatewayFacade describes the gateway methods required by the HTTP layer.
type GatewayFacade interface {
	Account() (*client.User, *client.ProviderProfile)
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
}

// New constructs a Server with the required dependencies.
func New(gateway GatewayFacade, chatAdapter adapter.ChatAdapter, store ledger.Store) *Server {
	return &Server{gateway: gateway, adapter: chatAdapter, ledger: store}
}

// Router returns a configured chi router for embedding in HTTP servers.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/profile", s.handleProfile)
		api.Get("/providers", s.handleProviders)
		api.Get("/services", s.handleServices)
		api.Get("/usage/summary", s.handleUsageSummary)
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
