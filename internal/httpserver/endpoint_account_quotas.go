package httpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
	"github.com/tokligence/tokligence-gateway/internal/scheduler"
)

// accountQuotasEndpoint wraps the account quotas CRUD endpoints
type accountQuotasEndpoint struct {
	server *Server
}

func newAccountQuotasEndpoint(server *Server) protocol.Endpoint {
	return &accountQuotasEndpoint{server: server}
}

func (e *accountQuotasEndpoint) Name() string { return "account_quotas" }

func (e *accountQuotasEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{Method: http.MethodGet, Path: "/admin/account-quotas", Handler: http.HandlerFunc(e.server.HandleListAccountQuotas)},
		{Method: http.MethodPost, Path: "/admin/account-quotas", Handler: http.HandlerFunc(e.server.HandleCreateAccountQuota)},
		{Method: http.MethodPut, Path: "/admin/account-quotas/{id}", Handler: http.HandlerFunc(e.server.HandleUpdateAccountQuota)},
		{Method: http.MethodDelete, Path: "/admin/account-quotas/{id}", Handler: http.HandlerFunc(e.server.HandleDeleteAccountQuota)},
		{Method: http.MethodGet, Path: "/admin/account-quotas/status/{account_id}", Handler: http.HandlerFunc(e.server.HandleGetAccountQuotaStatus)},
	}
}

// Quota management endpoints (Team Edition only)
// Provides RESTful CRUD API for account quotas

// HandleListAccountQuotas handles GET /admin/account-quotas
func (s *Server) HandleListAccountQuotas(w http.ResponseWriter, r *http.Request) {
	if !s.isQuotaManagementEnabled() {
		s.respondQuotaNotEnabled(w)
		return
	}

	// Get account filter from query params
	accountID := r.URL.Query().Get("account_id")

	query := `
		SELECT id, account_id, team_id, environment,
		       quota_type, limit_dimension, limit_value,
		       allow_borrow, max_borrow_pct,
		       window_type, window_start, window_end,
		       used_value, last_sync_at,
		       alert_at_pct, alert_webhook_url, alert_triggered, last_alert_at,
		       description, enabled,
		       created_at, updated_at, created_by, updated_by
		FROM account_quotas
		WHERE deleted_at IS NULL
	`

	args := []interface{}{}
	if accountID != "" {
		query += " AND account_id = $1"
		args = append(args, accountID)
	}

	query += " ORDER BY account_id, window_start DESC"

	db := s.getPostgresDB()
	rows, err := db.Query(query, args...)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to query quotas: " + err.Error(),
		})
		return
	}
	defer rows.Close()

	quotas := make([]scheduler.AccountQuota, 0)
	for rows.Next() {
		var q scheduler.AccountQuota
		err := rows.Scan(
			&q.ID, &q.AccountID, &q.TeamID, &q.Environment,
			&q.QuotaType, &q.LimitDimension, &q.LimitValue,
			&q.AllowBorrow, &q.MaxBorrowPct,
			&q.WindowType, &q.WindowStart, &q.WindowEnd,
			&q.UsedValue, &q.LastSyncAt,
			&q.AlertAtPct, &q.AlertWebhookURL, &q.AlertTriggered, &q.LastAlertAt,
			&q.Description, &q.Enabled,
			&q.CreatedAt, &q.UpdatedAt, &q.CreatedBy, &q.UpdatedBy,
		)
		if err != nil {
			s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "scan_error",
				Message: "Failed to scan quota: " + err.Error(),
			})
			return
		}

		q.ComputeUtilization()
		q.CheckExpired()

		quotas = append(quotas, q)
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"quotas": quotas,
		"count":  len(quotas),
	})
}

// HandleCreateAccountQuota handles POST /admin/account-quotas
func (s *Server) HandleCreateAccountQuota(w http.ResponseWriter, r *http.Request) {
	if !s.isQuotaManagementEnabled() {
		s.respondQuotaNotEnabled(w)
		return
	}

	var req struct {
		AccountID       string   `json:"account_id"`
		TeamID          *string  `json:"team_id,omitempty"`
		Environment     *string  `json:"environment,omitempty"`
		QuotaType       string   `json:"quota_type"`
		LimitDimension  string   `json:"limit_dimension"`
		LimitValue      int64    `json:"limit_value"`
		AllowBorrow     bool     `json:"allow_borrow"`
		MaxBorrowPct    float64  `json:"max_borrow_pct"`
		WindowType      string   `json:"window_type"`
		WindowStart     *string  `json:"window_start,omitempty"`
		WindowEnd       *string  `json:"window_end,omitempty"`
		AlertAtPct      float64  `json:"alert_at_pct"`
		AlertWebhookURL *string  `json:"alert_webhook_url,omitempty"`
		Description     string   `json:"description,omitempty"`
		Enabled         bool     `json:"enabled"`
		CreatedBy       string   `json:"created_by,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Validation
	if req.AccountID == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "account_id is required",
		})
		return
	}

	if req.QuotaType == "" || req.LimitDimension == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "quota_type and limit_dimension are required",
		})
		return
	}

	if req.LimitValue <= 0 {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "limit_value must be positive",
		})
		return
	}

	// Default values
	if req.WindowType == "" {
		req.WindowType = "monthly"
	}
	if req.AlertAtPct == 0 {
		req.AlertAtPct = 0.80 // 80% default
	}

	// Parse window start/end
	var windowStart time.Time
	var windowEnd *time.Time

	if req.WindowStart != nil {
		var err error
		windowStart, err = time.Parse(time.RFC3339, *req.WindowStart)
		if err != nil {
			s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Invalid window_start format (use RFC3339)",
			})
			return
		}
	} else {
		windowStart = time.Now()
	}

	if req.WindowEnd != nil {
		t, err := time.Parse(time.RFC3339, *req.WindowEnd)
		if err != nil {
			s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
				Error:   "validation_error",
				Message: "Invalid window_end format (use RFC3339)",
			})
			return
		}
		windowEnd = &t
	}

	// Insert into database
	db := s.getPostgresDB()
	var quotaID string

	query := `
		INSERT INTO account_quotas (
			account_id, team_id, environment,
			quota_type, limit_dimension, limit_value,
			allow_borrow, max_borrow_pct,
			window_type, window_start, window_end,
			alert_at_pct, alert_webhook_url,
			description, enabled, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id
	`

	err := db.QueryRow(query,
		req.AccountID, req.TeamID, req.Environment,
		req.QuotaType, req.LimitDimension, req.LimitValue,
		req.AllowBorrow, req.MaxBorrowPct,
		req.WindowType, windowStart, windowEnd,
		req.AlertAtPct, req.AlertWebhookURL,
		req.Description, req.Enabled, req.CreatedBy,
	).Scan(&quotaID)

	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to create quota: " + err.Error(),
		})
		return
	}

	// Trigger quota manager reload
	if s.quotaManager != nil {
		if err := s.quotaManager.Reload(); err != nil {
			s.log.Printf("[WARN] Failed to reload quota manager: %v", err)
		}
	}

	s.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      quotaID,
		"message": "Quota created successfully",
	})
}

// HandleUpdateAccountQuota handles PUT /admin/account-quotas/:id
func (s *Server) HandleUpdateAccountQuota(w http.ResponseWriter, r *http.Request) {
	if !s.isQuotaManagementEnabled() {
		s.respondQuotaNotEnabled(w)
		return
	}

	quotaID := chi.URLParam(r, "id")
	if quotaID == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Quota ID is required",
		})
		return
	}

	var req struct {
		LimitValue      *int64   `json:"limit_value,omitempty"`
		AllowBorrow     *bool    `json:"allow_borrow,omitempty"`
		MaxBorrowPct    *float64 `json:"max_borrow_pct,omitempty"`
		AlertAtPct      *float64 `json:"alert_at_pct,omitempty"`
		AlertWebhookURL *string  `json:"alert_webhook_url,omitempty"`
		Description     *string  `json:"description,omitempty"`
		Enabled         *bool    `json:"enabled,omitempty"`
		UpdatedBy       string   `json:"updated_by,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Build dynamic UPDATE query
	updates := make([]string, 0)
	args := make([]interface{}, 0)
	argPos := 1

	if req.LimitValue != nil {
		updates = append(updates, fmt.Sprintf("limit_value = $%d", argPos))
		args = append(args, *req.LimitValue)
		argPos++
	}

	if req.AllowBorrow != nil {
		updates = append(updates, fmt.Sprintf("allow_borrow = $%d", argPos))
		args = append(args, *req.AllowBorrow)
		argPos++
	}

	if req.MaxBorrowPct != nil {
		updates = append(updates, fmt.Sprintf("max_borrow_pct = $%d", argPos))
		args = append(args, *req.MaxBorrowPct)
		argPos++
	}

	if req.AlertAtPct != nil {
		updates = append(updates, fmt.Sprintf("alert_at_pct = $%d", argPos))
		args = append(args, *req.AlertAtPct)
		argPos++
	}

	if req.AlertWebhookURL != nil {
		updates = append(updates, fmt.Sprintf("alert_webhook_url = $%d", argPos))
		args = append(args, *req.AlertWebhookURL)
		argPos++
	}

	if req.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *req.Description)
		argPos++
	}

	if req.Enabled != nil {
		updates = append(updates, fmt.Sprintf("enabled = $%d", argPos))
		args = append(args, *req.Enabled)
		argPos++
	}

	if len(updates) == 0 {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation_error",
			Message: "No fields to update",
		})
		return
	}

	// Always update updated_at and updated_by
	updates = append(updates, "updated_at = NOW()")
	if req.UpdatedBy != "" {
		updates = append(updates, fmt.Sprintf("updated_by = $%d", argPos))
		args = append(args, req.UpdatedBy)
		argPos++
	}

	// Add WHERE clause
	args = append(args, quotaID)
	whereClause := fmt.Sprintf("$%d", argPos)

	query := "UPDATE account_quotas SET " + strings.Join(updates, ", ") +
		" WHERE id = " + whereClause + " AND deleted_at IS NULL"

	db := s.getPostgresDB()
	result, err := db.Exec(query, args...)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to update quota: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		s.respondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Quota not found or already deleted",
		})
		return
	}

	// Trigger quota manager reload
	if s.quotaManager != nil {
		if err := s.quotaManager.Reload(); err != nil {
			s.log.Printf("[WARN] Failed to reload quota manager: %v", err)
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Quota updated successfully",
	})
}

// HandleDeleteAccountQuota handles DELETE /admin/account-quotas/:id (soft delete)
func (s *Server) HandleDeleteAccountQuota(w http.ResponseWriter, r *http.Request) {
	if !s.isQuotaManagementEnabled() {
		s.respondQuotaNotEnabled(w)
		return
	}

	quotaID := chi.URLParam(r, "id")
	if quotaID == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Quota ID is required",
		})
		return
	}

	var req struct {
		DeletedBy string `json:"deleted_by,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Soft delete
	db := s.getPostgresDB()
	query := `
		UPDATE account_quotas
		SET deleted_at = NOW(), updated_at = NOW(), updated_by = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := db.Exec(query, req.DeletedBy, quotaID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to delete quota: " + err.Error(),
		})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		s.respondJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Quota not found or already deleted",
		})
		return
	}

	// Trigger quota manager reload
	if s.quotaManager != nil {
		if err := s.quotaManager.Reload(); err != nil {
			s.log.Printf("[WARN] Failed to reload quota manager: %v", err)
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Quota deleted successfully (soft delete)",
	})
}

// HandleGetAccountQuotaStatus handles GET /admin/account-quotas/status/:account_id
func (s *Server) HandleGetAccountQuotaStatus(w http.ResponseWriter, r *http.Request) {
	if !s.isQuotaManagementEnabled() {
		s.respondQuotaNotEnabled(w)
		return
	}

	accountID := chi.URLParam(r, "account_id")
	if accountID == "" {
		s.respondJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Account ID is required",
		})
		return
	}

	if s.quotaManager == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "quota_manager_unavailable",
			Message: "Quota manager not initialized",
		})
		return
	}

	quotas, err := s.quotaManager.GetQuotaStatus(accountID)
	if err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: err.Error(),
		})
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"account_id": accountID,
		"quotas":     quotas,
		"count":      len(quotas),
	})
}

// Helper functions

func (s *Server) isQuotaManagementEnabled() bool {
	return s.quotaManager != nil && s.quotaManager.IsEnabled()
}

func (s *Server) respondQuotaNotEnabled(w http.ResponseWriter) {
	s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
		Error:   http.StatusText(http.StatusNotImplemented),
		Message: "Account quota management is not enabled (Personal Edition)",
	})
}

func (s *Server) getPostgresDB() *sql.DB {
	// Assuming identityStore has a DB() method (added in Phase 1)
	if pgStore, ok := s.identityStore.(interface{ DB() *sql.DB }); ok {
		return pgStore.DB()
	}
	return nil
}
