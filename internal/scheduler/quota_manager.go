package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// QuotaManager manages account quotas with in-memory tracking (single instance)
// TODO: Add Redis support for distributed tracking when needed
type QuotaManager struct {
	db      *sql.DB
	enabled bool

	// In-memory quota tracking (single instance only)
	quotas      map[string]*AccountQuota // quotaID -> quota
	usageCache  map[string]int64         // quotaID -> current usage
	adjustments []QuotaAdjustment        // Active time-based overrides
	mu          sync.RWMutex

	// In-memory per-account override tracking (for time-based overrides)
	overrideState map[string]*accountOverrideState
	overrideMu    sync.Mutex

	// Sync configuration
	syncInterval time.Duration
	lastSync     time.Time

	// Alert configuration
	alertCooldown time.Duration
	alertCallback func(quota *AccountQuota)
}

type accountOverrideState struct {
	currentConcurrent int
	windowStart       time.Time
	windowRequests    int64
	windowTokens      int64
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(db *sql.DB, enabled bool, syncInterval time.Duration) (*QuotaManager, error) {
	if !enabled {
		return &QuotaManager{enabled: false}, nil
	}

	if db == nil {
		return nil, fmt.Errorf("quota manager requires PostgreSQL connection")
	}

	qm := &QuotaManager{
		db:            db,
		enabled:       true,
		quotas:        make(map[string]*AccountQuota),
		usageCache:    make(map[string]int64),
		adjustments:   make([]QuotaAdjustment, 0),
		overrideState: make(map[string]*accountOverrideState),
		syncInterval:  syncInterval,
		alertCooldown: 1 * time.Hour, // Default: 1 hour between alerts
	}

	// Initial load from database
	if err := qm.Reload(); err != nil {
		return nil, fmt.Errorf("failed to load quotas: %w", err)
	}

	log.Printf("[INFO] QuotaManager: Initialized with %d quotas (sync_interval=%s)",
		len(qm.quotas), syncInterval)

	return qm, nil
}

// IsEnabled returns whether quota management is enabled
func (qm *QuotaManager) IsEnabled() bool {
	return qm.enabled
}

// Reload reloads all active quotas from the database
func (qm *QuotaManager) Reload() error {
	if !qm.enabled {
		return nil
	}

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
		WHERE deleted_at IS NULL AND enabled = true
		ORDER BY account_id, window_start DESC
	`

	rows, err := qm.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query quotas: %w", err)
	}
	defer rows.Close()

	newQuotas := make(map[string]*AccountQuota)
	newUsageCache := make(map[string]int64)

	for rows.Next() {
		var q AccountQuota
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
			return fmt.Errorf("failed to scan quota: %w", err)
		}

		q.ComputeUtilization()
		q.CheckExpired()

		newQuotas[q.ID] = &q
		newUsageCache[q.ID] = q.UsedValue
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating quotas: %w", err)
	}

	// Atomic swap
	qm.mu.Lock()
	qm.quotas = newQuotas
	qm.usageCache = newUsageCache
	qm.lastSync = time.Now()
	qm.mu.Unlock()

	log.Printf("[INFO] QuotaManager: Reloaded %d quotas from database", len(newQuotas))
	return nil
}

// CheckAndReserve checks quota availability and reserves tokens
func (qm *QuotaManager) CheckAndReserve(ctx context.Context, req QuotaCheckRequest) (*QuotaCheckResult, error) {
	if !qm.enabled {
		return &QuotaCheckResult{Allowed: true}, nil
	}

	// Apply dynamic adjustments (time-based overrides)
	effConcurrent, effRPS, effTokens := qm.effectiveOverrides(req.AccountID)
	overrideApplied := false
	var overrideConcurrentDelta int
	var overrideTokenDelta int64
	var overrideReqDelta int64

	if effTokens != nil || effConcurrent != nil || effRPS != nil {
		qm.overrideMu.Lock()
		state := qm.getOverrideState(req.AccountID)
		qm.resetOverrideWindowIfNeeded(state)

		if effConcurrent != nil && state.currentConcurrent >= *effConcurrent {
			qm.overrideMu.Unlock()
			return &QuotaCheckResult{
				Allowed:       false,
				RejectionCode: "override_limit",
				Message:       fmt.Sprintf("concurrency limit reached for account %s (%d/%d)", req.AccountID, state.currentConcurrent, *effConcurrent),
			}, nil
		}

		if effTokens != nil {
			if req.EstimatedTokens > *effTokens {
				qm.overrideMu.Unlock()
				return &QuotaCheckResult{
					Allowed:       false,
					RejectionCode: "override_limit",
					Message:       fmt.Sprintf("token estimate %d exceeds override limit %d for account %s", req.EstimatedTokens, *effTokens, req.AccountID),
				}, nil
			}
			projectedTokens := state.windowTokens + req.EstimatedTokens
			if projectedTokens > *effTokens {
				qm.overrideMu.Unlock()
				return &QuotaCheckResult{
					Allowed:       false,
					RejectionCode: "override_limit",
					Message:       fmt.Sprintf("tokens/sec limit exceeded for account %s (%d/%d)", req.AccountID, projectedTokens, *effTokens),
				}, nil
			}
		}

		if effRPS != nil {
			projectedRPS := state.windowRequests + 1
			if projectedRPS > int64(*effRPS) {
				qm.overrideMu.Unlock()
				return &QuotaCheckResult{
					Allowed:       false,
					RejectionCode: "override_limit",
					Message:       fmt.Sprintf("rps limit exceeded for account %s (%d/%d)", req.AccountID, projectedRPS, *effRPS),
				}, nil
			}
		}

		// Reserve against overrides
		if effConcurrent != nil {
			state.currentConcurrent++
			overrideConcurrentDelta = 1
		}
		if effTokens != nil {
			state.windowTokens += req.EstimatedTokens
			overrideTokenDelta = req.EstimatedTokens
		}
		if effRPS != nil {
			state.windowRequests++
			overrideReqDelta = 1
		}
		overrideApplied = true
		qm.overrideMu.Unlock()
	}

	// Find applicable quotas for this account/team/environment
	qm.mu.RLock()
	applicableQuotas := qm.findApplicableQuotas(req.AccountID, req.TeamID, req.Environment)
	usageSnapshot := make(map[string]int64, len(applicableQuotas))
	for _, quota := range applicableQuotas {
		usageSnapshot[quota.ID] = qm.usageCache[quota.ID]
	}
	qm.mu.RUnlock()

	if len(applicableQuotas) == 0 {
		// No quotas configured = allow
		return &QuotaCheckResult{Allowed: true}, nil
	}

	result := &QuotaCheckResult{
		Allowed:       true,
		QuotasChecked: make([]string, 0, len(applicableQuotas)),
	}

	// Check each quota
	for _, quota := range applicableQuotas {
		result.QuotasChecked = append(result.QuotasChecked, quota.ID)

		currentUsage := usageSnapshot[quota.ID]
		newUsage := currentUsage + req.EstimatedTokens

		// Hard quota: strict limit
		if quota.QuotaType == string(QuotaTypeHard) {
			if newUsage > quota.LimitValue {
				result.Allowed = false
				result.RejectionCode = "hard_limit"
				result.Message = fmt.Sprintf("Hard quota exceeded for account %s (used: %d, limit: %d)",
					req.AccountID, currentUsage, quota.LimitValue)
				break
			}
		}

		// Soft quota: warn at limit, reject at 120%
		if quota.QuotaType == string(QuotaTypeSoft) {
			if newUsage > quota.LimitValue {
				log.Printf("[WARN] Soft quota exceeded: account=%s used=%d limit=%d",
					req.AccountID, currentUsage, quota.LimitValue)
				result.RejectionCode = "soft_limit"
				result.Message = fmt.Sprintf("Soft quota exceeded for account %s (used: %d, limit: %d)",
					req.AccountID, currentUsage, quota.LimitValue)
			}
			if newUsage > int64(float64(quota.LimitValue)*1.2) {
				result.Allowed = false
				result.RejectionCode = "soft_limit_exceeded"
				result.Message = fmt.Sprintf("Soft quota 120%% limit exceeded for account %s",
					req.AccountID)
				break
			}
		}

		// Reserved/Burstable: Check if we can borrow (future implementation)
		// TODO: Implement borrowing logic for distributed scenarios
	}

	if !result.Allowed {
		if overrideApplied {
			qm.rollbackOverride(req.AccountID, overrideConcurrentDelta, overrideReqDelta, overrideTokenDelta)
		}
		return result, nil
	}

	// Reserve tokens in memory
	qm.mu.Lock()
	for _, quota := range applicableQuotas {
		if _, ok := qm.usageCache[quota.ID]; ok {
			qm.usageCache[quota.ID] += req.EstimatedTokens
		}
	}
	qm.mu.Unlock()

	return result, nil
}

// CommitUsage adjusts usage after actual token count is known
func (qm *QuotaManager) CommitUsage(ctx context.Context, accountID, teamID, environment string,
	actualTokens, estimatedTokens int64) error {

	if !qm.enabled {
		return nil
	}

	delta := actualTokens - estimatedTokens
	if delta == 0 {
		return nil // No adjustment needed
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Find applicable quotas
	applicableQuotas := qm.findApplicableQuotas(accountID, teamID, environment)

	for _, quota := range applicableQuotas {
		qm.usageCache[quota.ID] += delta

		// Check alert threshold
		currentUsage := qm.usageCache[quota.ID]
		quota.UsedValue = currentUsage
		quota.ComputeUtilization()

		if quota.ShouldAlert() {
			qm.triggerAlert(quota)
		}
	}

	return nil
}

// ReleaseOverride decrements override-based concurrency reservations.
func (qm *QuotaManager) ReleaseOverride(accountID string) {
	if !qm.enabled {
		return
	}
	qm.overrideMu.Lock()
	defer qm.overrideMu.Unlock()

	state, ok := qm.overrideState[accountID]
	if !ok {
		return
	}
	if state.currentConcurrent > 0 {
		state.currentConcurrent--
	}
}

func (qm *QuotaManager) rollbackOverride(accountID string, deltaConcurrent int, deltaRequests int64, deltaTokens int64) {
	qm.overrideMu.Lock()
	defer qm.overrideMu.Unlock()

	state, ok := qm.overrideState[accountID]
	if !ok {
		return
	}
	if deltaConcurrent > 0 && state.currentConcurrent >= deltaConcurrent {
		state.currentConcurrent -= deltaConcurrent
	}
	if deltaRequests > 0 && state.windowRequests >= deltaRequests {
		state.windowRequests -= deltaRequests
	}
	if deltaTokens > 0 && state.windowTokens >= deltaTokens {
		state.windowTokens -= deltaTokens
	}
}

// SyncToDatabase persists in-memory usage to PostgreSQL
func (qm *QuotaManager) SyncToDatabase() error {
	if !qm.enabled {
		return nil
	}

	qm.mu.RLock()
	usageSnapshot := make(map[string]int64, len(qm.usageCache))
	for id, usage := range qm.usageCache {
		usageSnapshot[id] = usage
	}
	qm.mu.RUnlock()

	// Update PostgreSQL in batch
	tx, err := qm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE account_quotas
		SET used_value = $1, last_sync_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for quotaID, usage := range usageSnapshot {
		if _, err := stmt.Exec(usage, quotaID); err != nil {
			return fmt.Errorf("failed to update quota %s: %w", quotaID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("[INFO] QuotaManager: Synced %d quotas to database", len(usageSnapshot))
	return nil
}

// findApplicableQuotas finds all quotas that apply to the request (not thread-safe, must hold lock)
func (qm *QuotaManager) findApplicableQuotas(accountID, teamID, environment string) []*AccountQuota {
	var result []*AccountQuota

	for _, quota := range qm.quotas {
		// Check if expired
		if quota.IsExpired {
			continue
		}

		// Match account
		if quota.AccountID != accountID {
			continue
		}

		// Match team (if specified)
		if quota.TeamID != nil && teamID != "" && *quota.TeamID != teamID {
			continue
		}

		// Match environment (if specified)
		if quota.Environment != nil && environment != "" && *quota.Environment != environment {
			continue
		}

		result = append(result, quota)
	}

	return result
}

// triggerAlert sends an alert for quota threshold breach
func (qm *QuotaManager) triggerAlert(quota *AccountQuota) {
	// Check cooldown
	if quota.LastAlertAt != nil {
		timeSinceLastAlert := time.Since(*quota.LastAlertAt)
		if timeSinceLastAlert < qm.alertCooldown {
			return // Still in cooldown
		}
	}

	log.Printf("[ALERT] Quota threshold reached: account=%s quota_id=%s utilization=%.2f%% limit=%d",
		quota.AccountID, quota.ID, quota.UtilizationPct, quota.LimitValue)

	// TODO: Send webhook notification if AlertWebhookURL is set
	if qm.alertCallback != nil {
		qm.alertCallback(quota)
	}

	// Mark alert as triggered in database
	_, err := qm.db.Exec(`
		UPDATE account_quotas
		SET alert_triggered = true, last_alert_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, quota.ID)

	if err != nil {
		log.Printf("[ERROR] Failed to mark alert as triggered: %v", err)
	}
}

// SetAlertCallback sets a callback function for quota alerts
func (qm *QuotaManager) SetAlertCallback(callback func(quota *AccountQuota)) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.alertCallback = callback
}

// GetQuotaStatus returns current status for all quotas of an account
func (qm *QuotaManager) GetQuotaStatus(accountID string) ([]*AccountQuota, error) {
	if !qm.enabled {
		return nil, fmt.Errorf("quota management not enabled")
	}

	qm.mu.RLock()
	defer qm.mu.RUnlock()

	var result []*AccountQuota
	for _, quota := range qm.quotas {
		if quota.AccountID == accountID {
			// Create a copy with current usage
			q := *quota
			q.UsedValue = qm.usageCache[quota.ID]
			q.ComputeUtilization()
			result = append(result, &q)
		}
	}

	return result, nil
}

func (qm *QuotaManager) effectiveOverrides(accountID string) (*int, *int, *int64) {
	var effConcurrent *int
	var effRPS *int
	var effTokens *int64

	qm.mu.RLock()
	defer qm.mu.RUnlock()

	for _, adj := range qm.adjustments {
		if !matchAccountPattern(accountID, adj.AccountPattern) {
			continue
		}
		if adj.MaxConcurrent != nil {
			if effConcurrent == nil || *adj.MaxConcurrent < *effConcurrent {
				effConcurrent = adj.MaxConcurrent
			}
		}
		if adj.MaxRPS != nil {
			if effRPS == nil || *adj.MaxRPS < *effRPS {
				effRPS = adj.MaxRPS
			}
		}
		if adj.MaxTokensPerSec != nil {
			if effTokens == nil || *adj.MaxTokensPerSec < *effTokens {
				effTokens = adj.MaxTokensPerSec
			}
		}
	}

	return effConcurrent, effRPS, effTokens
}

func (qm *QuotaManager) getOverrideState(accountID string) *accountOverrideState {
	state, ok := qm.overrideState[accountID]
	if !ok {
		state = &accountOverrideState{
			windowStart: time.Now(),
		}
		qm.overrideState[accountID] = state
	}
	return state
}

func (qm *QuotaManager) resetOverrideWindowIfNeeded(state *accountOverrideState) {
	if state.windowStart.IsZero() || time.Since(state.windowStart) > time.Second {
		state.windowStart = time.Now()
		state.windowRequests = 0
		state.windowTokens = 0
	}
}

// ApplyAdjustments installs a new set of time-based overrides (best-effort)
func (qm *QuotaManager) ApplyAdjustments(adjs []QuotaAdjustment) {
	if !qm.enabled {
		return
	}
	qm.mu.Lock()
	qm.adjustments = append([]QuotaAdjustment{}, adjs...)
	qm.mu.Unlock()

	qm.overrideMu.Lock()
	qm.overrideState = make(map[string]*accountOverrideState) // reset counters on rule changes
	qm.overrideMu.Unlock()
}

// StartBackgroundSync starts a background goroutine to periodically sync to database
func (qm *QuotaManager) StartBackgroundSync(stopCh <-chan struct{}) {
	if !qm.enabled {
		return
	}

	ticker := time.NewTicker(qm.syncInterval)
	defer ticker.Stop()

	log.Printf("[INFO] QuotaManager: Background sync started (interval=%s)", qm.syncInterval)

	for {
		select {
		case <-ticker.C:
			if err := qm.SyncToDatabase(); err != nil {
				log.Printf("[ERROR] QuotaManager: Failed to sync to database: %v", err)
			}
		case <-stopCh:
			// Final sync before shutdown
			if err := qm.SyncToDatabase(); err != nil {
				log.Printf("[ERROR] QuotaManager: Failed final sync: %v", err)
			}
			log.Printf("[INFO] QuotaManager: Background sync stopped")
			return
		}
	}
}

// TODO: Future enhancements for distributed scenarios
// - RedisBackend interface for distributed quota tracking
// - Borrowing logic for burstable quotas
// - Webhook notifications for alerts
// - Quota usage analytics and reporting

func matchAccountPattern(accountID, pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == accountID {
		return true
	}
	// Simple prefix wildcard support (foo* -> prefix match)
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(accountID, prefix)
	}
	return false
}
