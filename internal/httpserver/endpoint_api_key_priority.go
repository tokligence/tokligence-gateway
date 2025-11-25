package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// apiKeyPriorityEndpoint wraps the API key priority CRUD endpoints
type apiKeyPriorityEndpoint struct {
	server *Server
}

func newAPIKeyPriorityEndpoint(server *Server) protocol.Endpoint {
	return &apiKeyPriorityEndpoint{server: server}
}

func (e *apiKeyPriorityEndpoint) Name() string { return "api_key_priority" }

func (e *apiKeyPriorityEndpoint) Routes() []protocol.EndpointRoute {
	wrap := e.server.wrapAdminHandler
	return []protocol.EndpointRoute{
		{Method: http.MethodGet, Path: "/admin/api-key-priority/mappings", Handler: wrap(e.server.HandleListAPIKeyMappings)},
		{Method: http.MethodPost, Path: "/admin/api-key-priority/mappings", Handler: wrap(e.server.HandleCreateAPIKeyMapping)},
		{Method: http.MethodPut, Path: "/admin/api-key-priority/mappings/{id}", Handler: wrap(e.server.HandleUpdateAPIKeyMapping)},
		{Method: http.MethodDelete, Path: "/admin/api-key-priority/mappings/{id}", Handler: wrap(e.server.HandleDeleteAPIKeyMapping)},
		{Method: http.MethodPost, Path: "/admin/api-key-priority/reload", Handler: wrap(e.server.HandleReloadAPIKeyMappings)},
	}
}

// CreateMappingRequest represents the request body for creating a new mapping
type CreateMappingRequest struct {
	Pattern     string `json:"pattern"`
	Priority    int    `json:"priority"`
	MatchType   string `json:"match_type"`
	TenantID    string `json:"tenant_id,omitempty"`
	TenantName  string `json:"tenant_name,omitempty"`
	TenantType  string `json:"tenant_type,omitempty"`
	Description string `json:"description,omitempty"`
	CreatedBy   string `json:"created_by,omitempty"`
}

// UpdateMappingRequest represents the request body for updating a mapping
type UpdateMappingRequest struct {
	Priority    int    `json:"priority"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	UpdatedBy   string `json:"updated_by,omitempty"`
}

// MappingResponse represents the response body for a mapping
type MappingResponse struct {
	ID          string `json:"id"`
	Pattern     string `json:"pattern"`
	Priority    int    `json:"priority"`
	MatchType   string `json:"match_type"`
	TenantID    string `json:"tenant_id,omitempty"`
	TenantName  string `json:"tenant_name,omitempty"`
	TenantType  string `json:"tenant_type,omitempty"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	CreatedBy   string `json:"created_by,omitempty"`
	UpdatedBy   string `json:"updated_by,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// HandleListAPIKeyMappings handles GET /admin/api-key-priority/mappings
func (s *Server) HandleListAPIKeyMappings(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if s.apiKeyMapper == nil || !s.apiKeyMapper.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "API key priority mapping is not enabled (Personal Edition)",
		})
		return
	}

	ctx := r.Context()
	mappings, err := s.apiKeyMapper.ListMappings(ctx)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	// Convert to response format
	response := make([]MappingResponse, 0, len(mappings))
	for _, m := range mappings {
		response = append(response, MappingResponse{
			ID:          m.ID,
			Pattern:     m.Pattern,
			Priority:    m.Priority,
			MatchType:   m.MatchType,
			TenantID:    m.TenantID,
			TenantName:  m.TenantName,
			TenantType:  m.TenantType,
			Description: m.Description,
			Enabled:     m.Enabled,
			CreatedAt:   m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedBy:   m.CreatedBy,
			UpdatedBy:   m.UpdatedBy,
		})
	}

	s.respondJSON(w, http.StatusOK, response)
}

// HandleCreateAPIKeyMapping handles POST /admin/api-key-priority/mappings
func (s *Server) HandleCreateAPIKeyMapping(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if s.apiKeyMapper == nil || !s.apiKeyMapper.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "API key priority mapping is not enabled (Personal Edition)",
		})
		return
	}

	var req CreateMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Validate required fields
	if req.Pattern == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "pattern is required",
		})
		return
	}
	if req.Priority < 0 || req.Priority > 9 {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "priority must be between 0 and 9",
		})
		return
	}
	if req.MatchType == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "match_type is required",
		})
		return
	}

	// Validate match_type
	matchType := scheduler.ParseMatchType(req.MatchType)
	if matchType.String() == "unknown" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "invalid match_type (must be: exact, prefix, suffix, contains, regex)",
		})
		return
	}

	// Validate tenant_type if provided
	if req.TenantType != "" && req.TenantType != "internal" && req.TenantType != "external" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "tenant_type must be 'internal' or 'external'",
		})
		return
	}

	ctx := r.Context()
	id, err := s.apiKeyMapper.AddMapping(ctx,
		req.Pattern,
		scheduler.PriorityTier(req.Priority),
		matchType,
		req.TenantID,
		req.TenantName,
		req.TenantType,
		req.Description,
		req.CreatedBy,
	)
	if err != nil {
		// Check for unique constraint violation
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "UNIQUE constraint") {
			s.respondJSON(w, http.StatusConflict, ErrorResponse{
				Error:   http.StatusText(http.StatusConflict),
				Message: "pattern already exists",
			})
			return
		}
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	// Return created mapping UUID
	s.respondJSON(w, http.StatusCreated, map[string]string{
		"id":      id,
		"message": "Mapping created successfully",
	})
}

// HandleUpdateAPIKeyMapping handles PUT /admin/api-key-priority/mappings/:id
func (s *Server) HandleUpdateAPIKeyMapping(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if s.apiKeyMapper == nil || !s.apiKeyMapper.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "API key priority mapping is not enabled (Personal Edition)",
		})
		return
	}

	// Get UUID from path
	id := chi.URLParam(r, "id")
	if id == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "id is required",
		})
		return
	}

	var req UpdateMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Validate priority
	if req.Priority < 0 || req.Priority > 9 {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "priority must be between 0 and 9",
		})
		return
	}

	ctx := r.Context()
	err := s.apiKeyMapper.UpdateMapping(ctx,
		id,
		scheduler.PriorityTier(req.Priority),
		req.Description,
		req.Enabled,
		req.UpdatedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "deleted") {
			s.respondJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   http.StatusText(http.StatusNotFound),
				Message: "mapping not found or already deleted",
			})
			return
		}
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"id":      id,
		"message": "Mapping updated successfully",
	})
}

// HandleDeleteAPIKeyMapping handles DELETE /admin/api-key-priority/mappings/:id (soft delete)
func (s *Server) HandleDeleteAPIKeyMapping(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if s.apiKeyMapper == nil || !s.apiKeyMapper.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "API key priority mapping is not enabled (Personal Edition)",
		})
		return
	}

	// Get UUID from path
	id := chi.URLParam(r, "id")
	if id == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   http.StatusText(http.StatusBadRequest),
			Message: "id is required",
		})
		return
	}

	// Get deletedBy from query param or default to "api"
	deletedBy := r.URL.Query().Get("deleted_by")
	if deletedBy == "" {
		deletedBy = "api"
	}

	ctx := r.Context()
	err := s.apiKeyMapper.DeleteMapping(ctx, id, deletedBy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "deleted") {
			s.respondJSON(w, http.StatusNotFound, ErrorResponse{
				Error:   http.StatusText(http.StatusNotFound),
				Message: "mapping not found or already deleted",
			})
			return
		}
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"id":      id,
		"message": "Mapping deleted successfully (soft delete)",
	})
}

// HandleReloadAPIKeyMappings handles POST /admin/api-key-priority/reload
func (s *Server) HandleReloadAPIKeyMappings(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if s.apiKeyMapper == nil || !s.apiKeyMapper.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "API key priority mapping is not enabled (Personal Edition)",
		})
		return
	}

	err := s.apiKeyMapper.Reload()
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]string{
		"message": "Cache reloaded successfully",
	})
}
