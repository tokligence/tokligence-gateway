package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// Gateway Management Handlers
// ==============================================================================

// handleCreateGateway handles POST /api/v1/gateways
func (s *Server) handleCreateGateway(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	user, ok := s.authenticatedUser(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	var req struct {
		Alias    string                 `json:"gateway_alias"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Alias == "" {
		writeJSONError(w, http.StatusBadRequest, "gateway_alias is required")
		return
	}

	// Parse user UUID
	userUUID, err := uuid.Parse(user.UUID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Invalid user UUID")
		return
	}

	gw, err := store.CreateGateway(r.Context(), userUUID, req.Alias)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create gateway: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":               gw.ID,
		"gateway_alias":    gw.Alias,
		"owner_user_id":    gw.OwnerUserID,
		"provider_enabled": gw.ProviderEnabled,
		"consumer_enabled": gw.ConsumerEnabled,
		"created_at":       gw.CreatedAt,
	})
}

// handleListGateways handles GET /api/v1/gateways
func (s *Server) handleListGateways(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	user, ok := s.authenticatedUser(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	userUUID, err := uuid.Parse(user.UUID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Invalid user UUID")
		return
	}

	gateways, err := store.ListGatewaysForUser(r.Context(), userUUID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list gateways: "+err.Error())
		return
	}

	result := make([]map[string]interface{}, 0, len(gateways))
	for _, gw := range gateways {
		result = append(result, map[string]interface{}{
			"id":               gw.ID,
			"gateway_alias":    gw.Alias,
			"owner_user_id":    gw.OwnerUserID,
			"provider_enabled": gw.ProviderEnabled,
			"consumer_enabled": gw.ConsumerEnabled,
			"created_at":       gw.CreatedAt,
			"updated_at":       gw.UpdatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"gateways": result})
}

// handleGetGateway handles GET /api/v1/gateways/{gateway_id}
func (s *Server) handleGetGateway(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	gatewayID, err := parseUUIDParam(r, "gateway_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid gateway_id")
		return
	}

	gw, err := store.GetGateway(r.Context(), gatewayID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get gateway: "+err.Error())
		return
	}
	if gw == nil {
		writeJSONError(w, http.StatusNotFound, "Gateway not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":               gw.ID,
		"gateway_alias":    gw.Alias,
		"owner_user_id":    gw.OwnerUserID,
		"provider_enabled": gw.ProviderEnabled,
		"consumer_enabled": gw.ConsumerEnabled,
		"metadata":         gw.Metadata,
		"created_at":       gw.CreatedAt,
		"updated_at":       gw.UpdatedAt,
	})
}

// handleUpdateGateway handles PATCH /api/v1/gateways/{gateway_id}
func (s *Server) handleUpdateGateway(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	gatewayID, err := parseUUIDParam(r, "gateway_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid gateway_id")
		return
	}

	var req struct {
		Alias           *string                `json:"gateway_alias"`
		ProviderEnabled *bool                  `json:"provider_enabled"`
		ConsumerEnabled *bool                  `json:"consumer_enabled"`
		Metadata        map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	updates := userstore.GatewayUpdate{
		Alias:           req.Alias,
		ProviderEnabled: req.ProviderEnabled,
		ConsumerEnabled: req.ConsumerEnabled,
	}
	if req.Metadata != nil {
		m := userstore.JSONMap(req.Metadata)
		updates.Metadata = &m
	}

	gw, err := store.UpdateGateway(r.Context(), gatewayID, updates)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update gateway: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":               gw.ID,
		"gateway_alias":    gw.Alias,
		"owner_user_id":    gw.OwnerUserID,
		"provider_enabled": gw.ProviderEnabled,
		"consumer_enabled": gw.ConsumerEnabled,
		"metadata":         gw.Metadata,
		"created_at":       gw.CreatedAt,
		"updated_at":       gw.UpdatedAt,
	})
}

// handleDeleteGateway handles DELETE /api/v1/gateways/{gateway_id}
func (s *Server) handleDeleteGateway(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	gatewayID, err := parseUUIDParam(r, "gateway_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid gateway_id")
		return
	}

	if err := store.DeleteGateway(r.Context(), gatewayID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete gateway: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==============================================================================
// Gateway Members Handlers
// ==============================================================================

// handleListGatewayMembers handles GET /api/v1/gateways/{gateway_id}/members
func (s *Server) handleListGatewayMembers(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	gatewayID, err := parseUUIDParam(r, "gateway_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid gateway_id")
		return
	}

	members, err := store.ListGatewayMembers(r.Context(), gatewayID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list members: "+err.Error())
		return
	}

	result := make([]map[string]interface{}, 0, len(members))
	for _, m := range members {
		result = append(result, map[string]interface{}{
			"id": m.ID,
			"user": map[string]interface{}{
				"id":           m.User.ID,
				"email":        m.User.Email,
				"display_name": m.User.DisplayName,
			},
			"role":       m.Role,
			"created_at": m.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"members": result,
	})
}

// handleAddGatewayMember handles POST /api/v1/gateways/{gateway_id}/members
func (s *Server) handleAddGatewayMember(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	gatewayID, err := parseUUIDParam(r, "gateway_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid gateway_id")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Find user by email
	user, err := s.identity.FindByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to find user: "+err.Error())
		return
	}
	if user == nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	userUUID, _ := uuid.Parse(user.UUID)
	role := userstore.GatewayMemberRole(req.Role)
	if role != userstore.GatewayRoleAdmin && role != userstore.GatewayRoleMember {
		role = userstore.GatewayRoleMember
	}

	membership, err := store.AddGatewayMember(r.Context(), gatewayID, userUUID, role)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to add member: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         membership.ID,
		"user_id":    membership.UserID,
		"gateway_id": membership.GatewayID,
		"role":       membership.Role,
		"created_at": membership.CreatedAt,
	})
}

// handleUpdateGatewayMember handles PATCH /api/v1/gateways/{gateway_id}/members/{membership_id}
func (s *Server) handleUpdateGatewayMember(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	membershipID, err := parseUUIDParam(r, "membership_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid membership_id")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	role := userstore.GatewayMemberRole(req.Role)
	if err := store.UpdateGatewayMember(r.Context(), membershipID, role); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update member: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveGatewayMember handles DELETE /api/v1/gateways/{gateway_id}/members/{membership_id}
func (s *Server) handleRemoveGatewayMember(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	membershipID, err := parseUUIDParam(r, "membership_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid membership_id")
		return
	}

	if err := store.RemoveGatewayMember(r.Context(), membershipID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to remove member: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==============================================================================
// Helper Functions
// ==============================================================================

func (s *Server) getStoreV2() (userstore.StoreV2, bool) {
	store := GetIdentityV2()
	if store == nil {
		return nil, false
	}
	return store, true
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, error) {
	// This would typically use a router's path parameter extraction
	// For now, we'll use a simple query parameter fallback
	value := r.PathValue(name)
	if value == "" {
		value = r.URL.Query().Get(name)
	}
	return uuid.Parse(value)
}
