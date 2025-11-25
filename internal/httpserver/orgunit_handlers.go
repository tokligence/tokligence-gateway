package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// OrgUnit Management Handlers
// ==============================================================================

// handleListOrgUnits handles GET /api/v1/gateways/{gateway_id}/org-units
func (s *Server) handleListOrgUnits(w http.ResponseWriter, r *http.Request) {
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

	includeChildren := r.URL.Query().Get("include_children") == "true"
	parentIDStr := r.URL.Query().Get("parent_id")

	if includeChildren || parentIDStr == "" {
		// Return full tree
		tree, err := store.GetOrgUnitTree(r.Context(), gatewayID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to get org units: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"org_units": tree})
		return
	}

	// Return children of specific parent
	parentID, err := uuid.Parse(parentIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid parent_id")
		return
	}

	children, err := store.GetOrgUnitChildren(r.Context(), parentID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get children: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"org_units": children})
}

// handleCreateOrgUnit handles POST /api/v1/gateways/{gateway_id}/org-units
func (s *Server) handleCreateOrgUnit(w http.ResponseWriter, r *http.Request) {
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
		ParentID      *string                `json:"parent_id"`
		Name          string                 `json:"name"`
		Slug          string                 `json:"slug"`
		UnitType      string                 `json:"unit_type"`
		BudgetID      *string                `json:"budget_id"`
		AllowedModels []string               `json:"allowed_models"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Name == "" || req.Slug == "" {
		writeJSONError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	params := userstore.CreateOrgUnitParams{
		GatewayID:     gatewayID,
		Name:          req.Name,
		Slug:          req.Slug,
		UnitType:      userstore.OrgUnitType(req.UnitType),
		AllowedModels: req.AllowedModels,
		Metadata:      req.Metadata,
	}

	if req.ParentID != nil {
		parentID, err := uuid.Parse(*req.ParentID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid parent_id")
			return
		}
		params.ParentID = &parentID
	}

	if req.BudgetID != nil {
		budgetID, err := uuid.Parse(*req.BudgetID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid budget_id")
			return
		}
		params.BudgetID = &budgetID
	}

	if params.UnitType == "" {
		params.UnitType = userstore.OrgUnitTypeTeam
	}

	ou, err := store.CreateOrgUnit(r.Context(), params)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create org unit: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, orgUnitToResponse(ou))
}

// handleGetOrgUnit handles GET /api/v1/gateways/{gateway_id}/org-units/{org_unit_id}
func (s *Server) handleGetOrgUnit(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	orgUnitID, err := parseUUIDParam(r, "org_unit_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid org_unit_id")
		return
	}

	ou, err := store.GetOrgUnit(r.Context(), orgUnitID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get org unit: "+err.Error())
		return
	}
	if ou == nil {
		writeJSONError(w, http.StatusNotFound, "OrgUnit not found")
		return
	}

	writeJSON(w, http.StatusOK, orgUnitToResponse(ou))
}

// handleUpdateOrgUnit handles PATCH /api/v1/gateways/{gateway_id}/org-units/{org_unit_id}
func (s *Server) handleUpdateOrgUnit(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	orgUnitID, err := parseUUIDParam(r, "org_unit_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid org_unit_id")
		return
	}

	var req struct {
		Name          *string                `json:"name"`
		Slug          *string                `json:"slug"`
		UnitType      *string                `json:"unit_type"`
		BudgetID      *string                `json:"budget_id"`
		AllowedModels *[]string              `json:"allowed_models"`
		Metadata      map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	updates := userstore.OrgUnitUpdate{
		Name:          req.Name,
		Slug:          req.Slug,
		AllowedModels: req.AllowedModels,
	}

	if req.UnitType != nil {
		ut := userstore.OrgUnitType(*req.UnitType)
		updates.UnitType = &ut
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

	ou, err := store.UpdateOrgUnit(r.Context(), orgUnitID, updates)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update org unit: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, orgUnitToResponse(ou))
}

// handleMoveOrgUnit handles POST /api/v1/gateways/{gateway_id}/org-units/{org_unit_id}/move
func (s *Server) handleMoveOrgUnit(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	orgUnitID, err := parseUUIDParam(r, "org_unit_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid org_unit_id")
		return
	}

	// Get old path for response
	oldOU, err := store.GetOrgUnit(r.Context(), orgUnitID)
	if err != nil || oldOU == nil {
		writeJSONError(w, http.StatusNotFound, "OrgUnit not found")
		return
	}
	oldPath := oldOU.Path
	oldParentID := oldOU.ParentID

	var req struct {
		NewParentID *string `json:"new_parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	var newParentID *uuid.UUID
	if req.NewParentID != nil && *req.NewParentID != "" {
		id, err := uuid.Parse(*req.NewParentID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid new_parent_id")
			return
		}
		newParentID = &id
	}

	ou, err := store.MoveOrgUnit(r.Context(), orgUnitID, newParentID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to move org unit: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":            ou.ID,
		"old_path":      oldPath,
		"new_path":      ou.Path,
		"old_parent_id": oldParentID,
		"new_parent_id": ou.ParentID,
	})
}

// handleMergeOrgUnits handles POST /api/v1/gateways/{gateway_id}/org-units/{source_id}/merge
func (s *Server) handleMergeOrgUnits(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	sourceID, err := parseUUIDParam(r, "source_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid source_id")
		return
	}

	var req struct {
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid target_id")
		return
	}

	if err := store.MergeOrgUnits(r.Context(), sourceID, targetID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to merge org units: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "Merge completed successfully",
		"source_id": sourceID,
		"target_id": targetID,
	})
}

// handleDeleteOrgUnit handles DELETE /api/v1/gateways/{gateway_id}/org-units/{org_unit_id}
func (s *Server) handleDeleteOrgUnit(w http.ResponseWriter, r *http.Request) {
	store, ok := s.getStoreV2()
	if !ok {
		writeJSONError(w, http.StatusNotImplemented, "V2 features not available")
		return
	}

	orgUnitID, err := parseUUIDParam(r, "org_unit_id")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid org_unit_id")
		return
	}

	force := r.URL.Query().Get("force") == "true"

	if err := store.DeleteOrgUnit(r.Context(), orgUnitID, force); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete org unit: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ==============================================================================
// Helper Functions
// ==============================================================================

func orgUnitToResponse(ou *userstore.OrgUnit) map[string]interface{} {
	return map[string]interface{}{
		"id":             ou.ID,
		"gateway_id":     ou.GatewayID,
		"parent_id":      ou.ParentID,
		"path":           ou.Path,
		"depth":          ou.Depth,
		"name":           ou.Name,
		"slug":           ou.Slug,
		"unit_type":      ou.UnitType,
		"budget_id":      ou.BudgetID,
		"allowed_models": ou.AllowedModels,
		"metadata":       ou.Metadata,
		"created_at":     ou.CreatedAt,
		"updated_at":     ou.UpdatedAt,
	}
}
