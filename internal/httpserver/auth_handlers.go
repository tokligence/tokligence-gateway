package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

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
