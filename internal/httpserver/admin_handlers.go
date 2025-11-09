package httpserver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tokligence/tokligence-gateway/internal/hooks"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

func (s *Server) HandleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	s.handleAdminListUsers(w, r)
}

func (s *Server) HandleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	s.handleAdminCreateUser(w, r)
}

func (s *Server) HandleAdminImportUsers(w http.ResponseWriter, r *http.Request) {
	s.handleAdminImportUsers(w, r)
}

func (s *Server) HandleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	s.handleAdminUpdateUser(w, r)
}

func (s *Server) HandleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDeleteUser(w, r)
}

func (s *Server) HandleAdminListAPIKeys(w http.ResponseWriter, r *http.Request) {
	s.handleAdminListAPIKeys(w, r)
}

func (s *Server) HandleAdminCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	s.handleAdminCreateAPIKey(w, r)
}

func (s *Server) HandleAdminDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDeleteAPIKey(w, r)
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
