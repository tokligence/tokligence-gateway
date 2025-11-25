package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// Principal Management Handlers
// ==============================================================================

// handleListPrincipals handles GET /api/v1/gateways/{gateway_id}/principals
func (s *Server) handleListPrincipals(w http.ResponseWriter, r *http.Request) {
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

	filter := userstore.PrincipalFilter{}

	if typeStr := r.URL.Query().Get("type"); typeStr != "" {
		pt := userstore.PrincipalType(typeStr)
		filter.Type = &pt
	}

	if orgUnitIDStr := r.URL.Query().Get("org_unit_id"); orgUnitIDStr != "" {
		orgUnitID, err := uuid.Parse(orgUnitIDStr)
		if err == nil {
			filter.OrgUnitID = &orgUnitID
		}
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = &search
	}

	principals, err := store.ListPrincipals(r.Context(), gatewayID, filter)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list principals: "+err.Error())
		return
	}

	result := make([]map[string]interface{}, 0, len(principals))
	for _, p := range principals {
		result = append(result, principalToResponse(&p))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"principals": result})
}

// handleCreatePrincipal handles POST /api/v1/gateways/{gateway_id}/principals
func (s *Server) handleCreatePrincipal(w http.ResponseWriter, r *http.Request) {
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
		PrincipalType   string                 `json:"principal_type"`
		ServiceName     *string                `json:"service_name"`
		EnvironmentName *string                `json:"environment_name"`
		DisplayName     string                 `json:"display_name"`
		OrgUnitID       *string                `json:"org_unit_id"`
		BudgetID        *string                `json:"budget_id"`
		AllowedModels   []string               `json:"allowed_models"`
		Metadata        map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// User principals are created automatically when adding gateway members
	// This endpoint is for service and environment principals
	if req.PrincipalType == "user" {
		writeJSONError(w, http.StatusBadRequest, "User principals are created automatically via gateway membership")
		return
	}

	if req.DisplayName == "" {
		writeJSONError(w, http.StatusBadRequest, "display_name is required")
		return
	}

	params := userstore.CreatePrincipalParams{
		GatewayID:       gatewayID,
		PrincipalType:   userstore.PrincipalType(req.PrincipalType),
		ServiceName:     req.ServiceName,
		EnvironmentName: req.EnvironmentName,
		DisplayName:     req.DisplayName,
		AllowedModels:   req.AllowedModels,
		Metadata:        req.Metadata,
	}

	if req.BudgetID != nil && *req.BudgetID != "" {
		budgetID, err := uuid.Parse(*req.BudgetID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid budget_id")
			return
		}
		params.BudgetID = &budgetID
	}

	// Validate required fields based on type
	switch params.PrincipalType {
	case userstore.PrincipalTypeService:
		if req.ServiceName == nil || *req.ServiceName == "" {
			writeJSONError(w, http.StatusBadRequest, "service_name is required for service principals")
			return
		}
	case userstore.PrincipalTypeEnvironment:
		if req.EnvironmentName == nil || *req.EnvironmentName == "" {
			writeJSONError(w, http.StatusBadRequest, "environment_name is required for environment principals")
			return
		}
	default:
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_type. Must be 'service' or 'environment'")
		return
	}

	principal, err := store.CreatePrincipal(r.Context(), params)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create principal: "+err.Error())
		return
	}

	// If org_unit_id is provided, create membership
	if req.OrgUnitID != nil && *req.OrgUnitID != "" {
		orgUnitID, err := uuid.Parse(*req.OrgUnitID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid org_unit_id")
			return
		}

		_, err = store.AddOrgMembership(r.Context(), userstore.CreateOrgMembershipParams{
			PrincipalID: principal.ID,
			OrgUnitID:   orgUnitID,
			Role:        userstore.OrgMemberRoleMember,
			IsPrimary:   true,
		})
		if err != nil {
			// Principal was created, but membership failed - log but don't fail
			// The principal can still be used, just not assigned to an org unit yet
		}
	}

	writeJSON(w, http.StatusCreated, principalToResponse(principal))
}

// handleGetPrincipal handles GET /api/v1/gateways/{gateway_id}/principals/{principal_id}
func (s *Server) handleGetPrincipal(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	principalID, err := parseUUIDParam(r, "principal_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_id")
		return
	}

	principal, err := store.GetPrincipal(r.Context(), principalID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get principal: "+err.Error())
		return
	}
	if principal == nil {
		writeJSONError(w, http.StatusNotFound, "Principal not found")
		return
	}

	// Get memberships
	memberships, err := store.ListOrgMemberships(r.Context(), principalID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get memberships: "+err.Error())
		return
	}

	response := principalToResponse(principal)
	membershipList := make([]map[string]interface{}, 0, len(memberships))
	for _, m := range memberships {
		membershipList = append(membershipList, map[string]interface{}{
			"id":            m.ID,
			"org_unit_id":   m.OrgUnitID,
			"org_unit_name": m.OrgUnit.Name,
			"org_unit_path": m.OrgUnit.Path,
			"role":          m.Role,
			"is_primary":    m.IsPrimary,
		})
	}
	response["memberships"] = membershipList

	writeJSON(w, http.StatusOK, response)
}

// handleUpdatePrincipal handles PATCH /api/v1/gateways/{gateway_id}/principals/{principal_id}
func (s *Server) handleUpdatePrincipal(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	principalID, err := parseUUIDParam(r, "principal_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_id")
		return
	}

	var req struct {
		DisplayName   *string                `json:"display_name"`
		BudgetID      *string                `json:"budget_id"`
		AllowedModels *[]string              `json:"allowed_models"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	updates := userstore.PrincipalUpdate{
		DisplayName:   req.DisplayName,
		AllowedModels: req.AllowedModels,
	}

	if req.BudgetID != nil {
		if *req.BudgetID == "" {
			nilUUID := uuid.Nil
			updates.BudgetID = &nilUUID
		} else {
			budgetID, err := uuid.Parse(*req.BudgetID)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "Invalid budget_id")
				return
			}
			updates.BudgetID = &budgetID
		}
	}

	if req.Metadata != nil {
		m := userstore.JSONMap(req.Metadata)
		updates.Metadata = &m
	}

	principal, err := store.UpdatePrincipal(r.Context(), principalID, updates)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update principal: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, principalToResponse(principal))
}

// handleDeletePrincipal handles DELETE /api/v1/gateways/{gateway_id}/principals/{principal_id}
func (s *Server) handleDeletePrincipal(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	principalID, err := parseUUIDParam(r, "principal_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_id")
		return
	}

	if err := store.DeletePrincipal(r.Context(), principalID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete principal: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==============================================================================
// OrgMembership Handlers
// ==============================================================================

// handleListPrincipalMemberships handles GET /api/v1/gateways/{gateway_id}/principals/{principal_id}/memberships
func (s *Server) handleListPrincipalMemberships(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	principalID, err := parseUUIDParam(r, "principal_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_id")
		return
	}

	memberships, err := store.ListOrgMemberships(r.Context(), principalID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list memberships: "+err.Error())
		return
	}

	result := make([]map[string]interface{}, 0, len(memberships))
	for _, m := range memberships {
		result = append(result, map[string]interface{}{
			"id":          m.ID,
			"org_unit_id": m.OrgUnitID,
			"org_unit": map[string]interface{}{
				"id":   m.OrgUnit.ID,
				"name": m.OrgUnit.Name,
				"path": m.OrgUnit.Path,
			},
			"role":       m.Role,
			"budget_id":  m.BudgetID,
			"is_primary": m.IsPrimary,
			"created_at": m.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"memberships": result})
}

// handleAddPrincipalMembership handles POST /api/v1/gateways/{gateway_id}/principals/{principal_id}/memberships
func (s *Server) handleAddPrincipalMembership(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	principalID, err := parseUUIDParam(r, "principal_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_id")
		return
	}

	var req struct {
		OrgUnitID string  `json:"org_unit_id"`
		Role      string  `json:"role"`
		BudgetID  *string `json:"budget_id"`
		IsPrimary bool    `json:"is_primary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	orgUnitID, err := uuid.Parse(req.OrgUnitID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid org_unit_id")
		return
	}

	params := userstore.CreateOrgMembershipParams{
		PrincipalID: principalID,
		OrgUnitID:   orgUnitID,
		Role:        userstore.OrgMemberRole(req.Role),
		IsPrimary:   req.IsPrimary,
	}

	if params.Role == "" {
		params.Role = userstore.OrgMemberRoleMember
	}

	if req.BudgetID != nil && *req.BudgetID != "" {
		budgetID, err := uuid.Parse(*req.BudgetID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid budget_id")
			return
		}
		params.BudgetID = &budgetID
	}

	membership, err := store.AddOrgMembership(r.Context(), params)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to add membership: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           membership.ID,
		"principal_id": membership.PrincipalID,
		"org_unit_id":  membership.OrgUnitID,
		"role":         membership.Role,
		"budget_id":    membership.BudgetID,
		"is_primary":   membership.IsPrimary,
		"created_at":   membership.CreatedAt,
	})
}

// handleUpdatePrincipalMembership handles PATCH /api/v1/gateways/{gateway_id}/principals/{principal_id}/memberships/{membership_id}
func (s *Server) handleUpdatePrincipalMembership(w http.ResponseWriter, r *http.Request) {
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
		Role      *string `json:"role"`
		BudgetID  *string `json:"budget_id"`
		IsPrimary *bool   `json:"is_primary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	updates := userstore.OrgMembershipUpdate{
		IsPrimary: req.IsPrimary,
	}

	if req.Role != nil {
		role := userstore.OrgMemberRole(*req.Role)
		updates.Role = &role
	}

	if req.BudgetID != nil {
		if *req.BudgetID == "" {
			nilUUID := uuid.Nil
			updates.BudgetID = &nilUUID
		} else {
			budgetID, err := uuid.Parse(*req.BudgetID)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "Invalid budget_id")
				return
			}
			updates.BudgetID = &budgetID
		}
	}

	if err := store.UpdateOrgMembership(r.Context(), membershipID, updates); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update membership: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRemovePrincipalMembership handles DELETE /api/v1/gateways/{gateway_id}/principals/{principal_id}/memberships/{membership_id}
func (s *Server) handleRemovePrincipalMembership(w http.ResponseWriter, r *http.Request) {
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

	if err := store.RemoveOrgMembership(r.Context(), membershipID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to remove membership: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==============================================================================
// Budget Resolution Handler
// ==============================================================================

// handleResolveBudget handles GET /api/v1/gateways/{gateway_id}/principals/{principal_id}/budget
func (s *Server) handleResolveBudget(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	principalID, err := parseUUIDParam(r, "principal_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid principal_id")
		return
	}

	inheritance, err := store.ResolveBudget(r.Context(), principalID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to resolve budget: "+err.Error())
		return
	}

	chain := make([]map[string]interface{}, 0, len(inheritance.Chain))
	for _, src := range inheritance.Chain {
		entry := map[string]interface{}{
			"type": src.Type,
			"id":   src.ID,
			"name": src.Name,
		}
		if src.Budget != nil {
			entry["budget"] = map[string]interface{}{
				"id":              src.Budget.ID,
				"max_budget":      src.Budget.MaxBudget,
				"budget_duration": src.Budget.BudgetDuration,
				"tpm_limit":       src.Budget.TPMLimit,
				"rpm_limit":       src.Budget.RPMLimit,
			}
		}
		chain = append(chain, entry)
	}

	response := map[string]interface{}{
		"source": inheritance.Source,
		"chain":  chain,
	}

	if inheritance.EffectiveBudget != nil {
		response["effective_budget"] = map[string]interface{}{
			"id":              inheritance.EffectiveBudget.ID,
			"name":            inheritance.EffectiveBudget.Name,
			"max_budget":      inheritance.EffectiveBudget.MaxBudget,
			"budget_duration": inheritance.EffectiveBudget.BudgetDuration,
			"tpm_limit":       inheritance.EffectiveBudget.TPMLimit,
			"rpm_limit":       inheritance.EffectiveBudget.RPMLimit,
			"soft_limit":      inheritance.EffectiveBudget.SoftLimit,
		}
	} else {
		response["effective_budget"] = nil
		response["unlimited"] = true
	}

	writeJSON(w, http.StatusOK, response)
}

// ==============================================================================
// Helper Functions
// ==============================================================================

func principalToResponse(p *userstore.Principal) map[string]interface{} {
	response := map[string]interface{}{
		"id":             p.ID,
		"gateway_id":     p.GatewayID,
		"principal_type": p.PrincipalType,
		"display_name":   p.DisplayName,
		"budget_id":      p.BudgetID,
		"allowed_models": p.AllowedModels,
		"metadata":       p.Metadata,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}

	if p.UserID != nil {
		response["user_id"] = p.UserID
	}
	if p.ServiceName != nil {
		response["service_name"] = p.ServiceName
	}
	if p.EnvironmentName != nil {
		response["environment_name"] = p.EnvironmentName
	}

	return response
}
