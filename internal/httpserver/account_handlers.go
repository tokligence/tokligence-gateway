package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
)

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
