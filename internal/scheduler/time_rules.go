package scheduler

import (
	"fmt"
	"time"
)

// TimeWindow defines when a rule is active
type TimeWindow struct {
	// Time range (24-hour format)
	StartHour   int // 0-23
	StartMinute int // 0-59
	EndHour     int // 0-23
	EndMinute   int // 0-59

	// Days of week (nil = all days)
	DaysOfWeek []time.Weekday // Monday=1, Sunday=7

	// Timezone location (nil = system timezone)
	Location *time.Location
}

// IsActive checks if the given time falls within this window
func (tw *TimeWindow) IsActive(t time.Time) bool {
	// Convert to window's timezone
	if tw.Location != nil {
		t = t.In(tw.Location)
	}

	// Get current time in minutes since midnight
	currentMinutes := t.Hour()*60 + t.Minute()
	startMinutes := tw.StartHour*60 + tw.StartMinute
	endMinutes := tw.EndHour*60 + tw.EndMinute

	// Determine which day to evaluate for day-of-week matching.
	// For wrap-around windows (e.g., 18:00-08:00), times after midnight
	// belong to the previous day for rule evaluation.
	dayToCheck := t.Weekday()
	if endMinutes < startMinutes && currentMinutes < endMinutes {
		dayToCheck = (dayToCheck + 6) % 7 // Previous day (handles Sunday->Saturday)
	}

	// Check day of week if specified
	if len(tw.DaysOfWeek) > 0 {
		dayMatch := false
		for _, day := range tw.DaysOfWeek {
			if day == dayToCheck {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Handle time ranges that wrap around midnight
	if endMinutes < startMinutes {
		// e.g., 18:00 -> 08:00 (next day)
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	// Normal time range (no wrap)
	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// String returns a human-readable description of the time window
func (tw *TimeWindow) String() string {
	tz := "system"
	if tw.Location != nil {
		tz = tw.Location.String()
	}

	days := "all days"
	if len(tw.DaysOfWeek) > 0 {
		dayNames := make([]string, len(tw.DaysOfWeek))
		for i, d := range tw.DaysOfWeek {
			dayNames[i] = d.String()
		}
		days = fmt.Sprintf("%v", dayNames)
	}

	return fmt.Sprintf("%02d:%02d-%02d:%02d %s (%s)",
		tw.StartHour, tw.StartMinute,
		tw.EndHour, tw.EndMinute,
		days, tz)
}

// RuleType defines the type of time-based rule
type RuleType string

const (
	RuleTypeWeightAdjustment   RuleType = "weight_adjustment"
	RuleTypeQuotaAdjustment    RuleType = "quota_adjustment"
	RuleTypeCapacityAdjustment RuleType = "capacity_adjustment"
)

// BaseRule contains common fields for all rules
type BaseRule struct {
	Name        string
	Type        RuleType
	Window      TimeWindow
	Description string
	Enabled     bool
}

// WeightAdjustmentRule adjusts priority weights for WFQ scheduling
type WeightAdjustmentRule struct {
	BaseRule
	Weights []float64 // Weights for each priority level (P0-P9)
}

// QuotaAdjustment defines quota changes for matching accounts
type QuotaAdjustment struct {
	AccountPattern  string // Pattern like "dept-a-*", "premium-*"
	MaxConcurrent   *int   // nil = no change
	MaxRPS          *int
	MaxTokensPerSec *int64
}

// QuotaAdjustmentRule adjusts per-account quotas
type QuotaAdjustmentRule struct {
	BaseRule
	Adjustments []QuotaAdjustment
}

// CapacityAdjustmentRule adjusts global scheduler capacity
type CapacityAdjustmentRule struct {
	BaseRule
	MaxConcurrent   *int
	MaxRPS          *int
	MaxTokensPerSec *int64
}

// Rule is a union type for all rule types
type Rule interface {
	GetName() string
	GetType() RuleType
	GetWindow() *TimeWindow
	IsEnabled() bool
	IsActive(t time.Time) bool
}

// Implement Rule interface for WeightAdjustmentRule
func (r *WeightAdjustmentRule) GetName() string        { return r.Name }
func (r *WeightAdjustmentRule) GetType() RuleType      { return r.Type }
func (r *WeightAdjustmentRule) GetWindow() *TimeWindow { return &r.Window }
func (r *WeightAdjustmentRule) IsEnabled() bool        { return r.Enabled }
func (r *WeightAdjustmentRule) IsActive(t time.Time) bool {
	return r.Enabled && r.Window.IsActive(t)
}

// Implement Rule interface for QuotaAdjustmentRule
func (r *QuotaAdjustmentRule) GetName() string        { return r.Name }
func (r *QuotaAdjustmentRule) GetType() RuleType      { return r.Type }
func (r *QuotaAdjustmentRule) GetWindow() *TimeWindow { return &r.Window }
func (r *QuotaAdjustmentRule) IsEnabled() bool        { return r.Enabled }
func (r *QuotaAdjustmentRule) IsActive(t time.Time) bool {
	return r.Enabled && r.Window.IsActive(t)
}

// Implement Rule interface for CapacityAdjustmentRule
func (r *CapacityAdjustmentRule) GetName() string        { return r.Name }
func (r *CapacityAdjustmentRule) GetType() RuleType      { return r.Type }
func (r *CapacityAdjustmentRule) GetWindow() *TimeWindow { return &r.Window }
func (r *CapacityAdjustmentRule) IsEnabled() bool        { return r.Enabled }
func (r *CapacityAdjustmentRule) IsActive(t time.Time) bool {
	return r.Enabled && r.Window.IsActive(t)
}

// RuleStatus represents the current status of a rule
type RuleStatus struct {
	Name        string    `json:"name"`
	Type        RuleType  `json:"type"`
	Active      bool      `json:"active"`
	Window      string    `json:"window"`
	Description string    `json:"description"`
	LastApplied time.Time `json:"last_applied,omitempty"`
}
