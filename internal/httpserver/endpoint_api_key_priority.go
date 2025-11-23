package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// APIKeyPriorityEndpoints provides HTTP CRUD API for priority mappings
type APIKeyPriorityEndpoints struct {
	mapper *scheduler.APIKeyMapper
}

// NewAPIKeyPriorityEndpoints creates a new endpoints handler
func NewAPIKeyPriorityEndpoints(mapper *scheduler.APIKeyMapper) *APIKeyPriorityEndpoints {
	return &APIKeyPriorityEndpoints{mapper: mapper}
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

// HandleListMappings handles GET /admin/api-key-priority/mappings
func (e *APIKeyPriorityEndpoints) HandleListMappings(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if !e.mapper.IsEnabled() {
		e.writeError(w, http.StatusNotImplemented, "API key priority mapping is not enabled (Personal Edition)")
		return
	}

	ctx := r.Context()
	mappings, err := e.mapper.ListMappings(ctx)
	if err != nil {
		e.writeError(w, http.StatusInternalServerError, err.Error())
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

	e.writeJSON(w, http.StatusOK, response)
}

// HandleCreateMapping handles POST /admin/api-key-priority/mappings
func (e *APIKeyPriorityEndpoints) HandleCreateMapping(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if !e.mapper.IsEnabled() {
		e.writeError(w, http.StatusNotImplemented, "API key priority mapping is not enabled (Personal Edition)")
		return
	}

	var req CreateMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		e.writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if req.Pattern == "" {
		e.writeError(w, http.StatusBadRequest, "pattern is required")
		return
	}
	if req.Priority < 0 || req.Priority > 9 {
		e.writeError(w, http.StatusBadRequest, "priority must be between 0 and 9")
		return
	}
	if req.MatchType == "" {
		e.writeError(w, http.StatusBadRequest, "match_type is required")
		return
	}

	// Validate match_type
	matchType := scheduler.ParseMatchType(req.MatchType)
	if matchType.String() == "unknown" {
		e.writeError(w, http.StatusBadRequest, "invalid match_type (must be: exact, prefix, suffix, contains, regex)")
		return
	}

	// Validate tenant_type if provided
	if req.TenantType != "" && req.TenantType != "internal" && req.TenantType != "external" {
		e.writeError(w, http.StatusBadRequest, "tenant_type must be 'internal' or 'external'")
		return
	}

	ctx := r.Context()
	id, err := e.mapper.AddMapping(ctx,
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
			e.writeError(w, http.StatusConflict, "pattern already exists")
			return
		}
		e.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return created mapping UUID
	e.writeJSON(w, http.StatusCreated, map[string]string{
		"id":      id,
		"message": "Mapping created successfully",
	})
}

// HandleUpdateMapping handles PUT /admin/api-key-priority/mappings/:id
func (e *APIKeyPriorityEndpoints) HandleUpdateMapping(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if !e.mapper.IsEnabled() {
		e.writeError(w, http.StatusNotImplemented, "API key priority mapping is not enabled (Personal Edition)")
		return
	}

	// Get UUID from path
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		e.writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req UpdateMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		e.writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	// Validate priority
	if req.Priority < 0 || req.Priority > 9 {
		e.writeError(w, http.StatusBadRequest, "priority must be between 0 and 9")
		return
	}

	ctx := r.Context()
	err := e.mapper.UpdateMapping(ctx,
		id,
		scheduler.PriorityTier(req.Priority),
		req.Description,
		req.Enabled,
		req.UpdatedBy,
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "deleted") {
			e.writeError(w, http.StatusNotFound, "mapping not found or already deleted")
			return
		}
		e.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	e.writeJSON(w, http.StatusOK, map[string]string{
		"id":      id,
		"message": "Mapping updated successfully",
	})
}

// HandleDeleteMapping handles DELETE /admin/api-key-priority/mappings/:id (soft delete)
func (e *APIKeyPriorityEndpoints) HandleDeleteMapping(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if !e.mapper.IsEnabled() {
		e.writeError(w, http.StatusNotImplemented, "API key priority mapping is not enabled (Personal Edition)")
		return
	}

	// Get UUID from path
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		e.writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	// Get deletedBy from query param or default to "api"
	deletedBy := r.URL.Query().Get("deleted_by")
	if deletedBy == "" {
		deletedBy = "api"
	}

	ctx := r.Context()
	err := e.mapper.DeleteMapping(ctx, id, deletedBy)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "deleted") {
			e.writeError(w, http.StatusNotFound, "mapping not found or already deleted")
			return
		}
		e.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	e.writeJSON(w, http.StatusOK, map[string]string{
		"id":      id,
		"message": "Mapping deleted successfully (soft delete)",
	})
}

// HandleReloadCache handles POST /admin/api-key-priority/reload
func (e *APIKeyPriorityEndpoints) HandleReloadCache(w http.ResponseWriter, r *http.Request) {
	// Check if feature is enabled
	if !e.mapper.IsEnabled() {
		e.writeError(w, http.StatusNotImplemented, "API key priority mapping is not enabled (Personal Edition)")
		return
	}

	err := e.mapper.Reload()
	if err != nil {
		e.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	e.writeJSON(w, http.StatusOK, map[string]string{
		"message": "Cache reloaded successfully",
	})
}

// Helper methods

func (e *APIKeyPriorityEndpoints) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (e *APIKeyPriorityEndpoints) writeError(w http.ResponseWriter, status int, message string) {
	e.writeJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}
