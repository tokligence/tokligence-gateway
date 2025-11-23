package scheduler

import (
	"time"
)

// QuotaType defines the type of quota enforcement
type QuotaType string

const (
	QuotaTypeHard      QuotaType = "hard"      // Strict limit, reject at boundary
	QuotaTypeSoft      QuotaType = "soft"      // Warning at limit, reject at 120%
	QuotaTypeReserved  QuotaType = "reserved"  // Guaranteed minimum capacity
	QuotaTypeBurstable QuotaType = "burstable" // Can borrow from others
)

// LimitDimension defines what the quota limits
type LimitDimension string

const (
	LimitDimensionTokensPerMonth LimitDimension = "tokens_per_month"
	LimitDimensionTokensPerDay   LimitDimension = "tokens_per_day"
	LimitDimensionTokensPerHour  LimitDimension = "tokens_per_hour"
	LimitDimensionUSDPerMonth    LimitDimension = "usd_per_month"
	LimitDimensionTPS            LimitDimension = "tps"  // Tokens per second
	LimitDimensionRPM            LimitDimension = "rpm"  // Requests per minute
)

// WindowType defines the quota time window
type WindowType string

const (
	WindowTypeHourly  WindowType = "hourly"
	WindowTypeDaily   WindowType = "daily"
	WindowTypeMonthly WindowType = "monthly"
	WindowTypeCustom  WindowType = "custom"
)

// AccountQuotaModel represents a quota record in PostgreSQL
type AccountQuotaModel struct {
	ID string `json:"id" db:"id"` // UUID

	// Hierarchy
	AccountID   string  `json:"account_id" db:"account_id"`
	TeamID      *string `json:"team_id,omitempty" db:"team_id"`
	Environment *string `json:"environment,omitempty" db:"environment"`

	// Quota configuration
	QuotaType      string `json:"quota_type" db:"quota_type"`
	LimitDimension string `json:"limit_dimension" db:"limit_dimension"`
	LimitValue     int64  `json:"limit_value" db:"limit_value"`

	// Borrowing
	AllowBorrow   bool    `json:"allow_borrow" db:"allow_borrow"`
	MaxBorrowPct  float64 `json:"max_borrow_pct" db:"max_borrow_pct"`

	// Time window
	WindowType  string     `json:"window_type" db:"window_type"`
	WindowStart time.Time  `json:"window_start" db:"window_start"`
	WindowEnd   *time.Time `json:"window_end,omitempty" db:"window_end"`

	// Current usage
	UsedValue   int64      `json:"used_value" db:"used_value"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty" db:"last_sync_at"`

	// Alerts
	AlertAtPct      float64    `json:"alert_at_pct" db:"alert_at_pct"`
	AlertWebhookURL *string    `json:"alert_webhook_url,omitempty" db:"alert_webhook_url"`
	AlertTriggered  bool       `json:"alert_triggered" db:"alert_triggered"`
	LastAlertAt     *time.Time `json:"last_alert_at,omitempty" db:"last_alert_at"`

	// Metadata
	Description string `json:"description,omitempty" db:"description"`
	Enabled     bool   `json:"enabled" db:"enabled"`

	// Audit fields
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	CreatedBy *string    `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy *string    `json:"updated_by,omitempty" db:"updated_by"`
}

// AccountQuota represents a quota with computed fields
type AccountQuota struct {
	AccountQuotaModel

	// Computed fields (not in DB)
	UtilizationPct float64 `json:"utilization_pct"`
	Remaining      int64   `json:"remaining"`
	IsExpired      bool    `json:"is_expired"`
	CanBorrow      bool    `json:"can_borrow"`
}

// QuotaUsageHistoryModel represents historical quota usage
type QuotaUsageHistoryModel struct {
	ID string `json:"id" db:"id"` // UUID

	QuotaID   string `json:"quota_id" db:"quota_id"`
	AccountID string `json:"account_id" db:"account_id"`

	SnapshotAt      time.Time `json:"snapshot_at" db:"snapshot_at"`
	UsedValue       int64     `json:"used_value" db:"used_value"`
	LimitValue      int64     `json:"limit_value" db:"limit_value"`
	UtilizationPct  float64   `json:"utilization_pct" db:"utilization_pct"`

	// Optional request metadata
	RequestID  *string `json:"request_id,omitempty" db:"request_id"`
	Model      *string `json:"model,omitempty" db:"model"`
	TokensUsed *int64  `json:"tokens_used,omitempty" db:"tokens_used"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// QuotaBorrowingLogModel represents borrowing transactions
type QuotaBorrowingLogModel struct {
	ID string `json:"id" db:"id"` // UUID

	BorrowerQuotaID string `json:"borrower_quota_id" db:"borrower_quota_id"`
	LenderQuotaID   string `json:"lender_quota_id" db:"lender_quota_id"`

	BorrowedAmount int64      `json:"borrowed_amount" db:"borrowed_amount"`
	BorrowedAt     time.Time  `json:"borrowed_at" db:"borrowed_at"`
	ReturnedAt     *time.Time `json:"returned_at,omitempty" db:"returned_at"`

	Reason string `json:"reason,omitempty" db:"reason"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// QuotaCheckRequest represents a request to check and reserve quota
type QuotaCheckRequest struct {
	AccountID      string
	TeamID         string
	Environment    string
	EstimatedTokens int64
	Model          string
	RequestID      string
}

// QuotaCheckResult represents the result of a quota check
type QuotaCheckResult struct {
	Allowed        bool
	RejectionCode  string // "quota_exceeded", "hard_limit", "soft_limit_exceeded"
	QuotasChecked  []string
	BorrowedAmount int64
	Message        string
}

// ComputeUtilization calculates utilization percentage
func (q *AccountQuota) ComputeUtilization() {
	if q.LimitValue > 0 {
		q.UtilizationPct = float64(q.UsedValue) / float64(q.LimitValue) * 100.0
	} else {
		q.UtilizationPct = 0.0
	}
	q.Remaining = q.LimitValue - q.UsedValue
	if q.Remaining < 0 {
		q.Remaining = 0
	}
}

// CheckExpired checks if the quota window has expired
func (q *AccountQuota) CheckExpired() {
	if q.WindowEnd != nil {
		q.IsExpired = time.Now().After(*q.WindowEnd)
	} else {
		q.IsExpired = false
	}
}

// ShouldAlert checks if alert threshold is reached
func (q *AccountQuota) ShouldAlert() bool {
	return q.Enabled && !q.AlertTriggered && q.UtilizationPct >= q.AlertAtPct*100.0
}
