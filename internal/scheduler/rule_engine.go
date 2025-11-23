package scheduler

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RuleEngine manages time-based rules and applies them to scheduler components
type RuleEngine struct {
	// Configuration
	enabled         bool
	checkInterval   time.Duration
	defaultTimezone *time.Location
	logger          *log.Logger

	// Rules
	weightRules   []*WeightAdjustmentRule
	quotaRules    []*QuotaAdjustmentRule
	capacityRules []*CapacityAdjustmentRule
	mu            sync.RWMutex

	// Target components
	scheduler    *Scheduler     // For weight/capacity adjustments
	quotaManager *QuotaManager  // For quota adjustments

	// State tracking
	activeRules     map[string]*RuleStatus
	lastEvaluation  time.Time
	stopCh          chan struct{}
	doneCh          chan struct{}

	// For testing - allows mocking current time
	timeNow func() time.Time
}

// RuleEngineConfig contains configuration for the rule engine
type RuleEngineConfig struct {
	Enabled         bool
	CheckInterval   time.Duration
	DefaultTimezone string
	Logger          *log.Logger
}

// NewRuleEngine creates a new time-based rule engine
func NewRuleEngine(config RuleEngineConfig) (*RuleEngine, error) {
	// Parse timezone
	var location *time.Location
	var err error
	if config.DefaultTimezone != "" {
		location, err = time.LoadLocation(config.DefaultTimezone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", config.DefaultTimezone, err)
		}
	} else {
		location = time.Local
	}

	// Default check interval
	if config.CheckInterval == 0 {
		config.CheckInterval = 60 * time.Second
	}

	return &RuleEngine{
		enabled:         config.Enabled,
		checkInterval:   config.CheckInterval,
		defaultTimezone: location,
		logger:          config.Logger,
		activeRules:     make(map[string]*RuleStatus),
		timeNow:         time.Now, // Can be overridden for testing
	}, nil
}

// SetLogger sets the logger for the rule engine
func (re *RuleEngine) SetLogger(logger *log.Logger) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.logger = logger
}

// SetScheduler sets the scheduler instance for weight/capacity adjustments
func (re *RuleEngine) SetScheduler(scheduler *Scheduler) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.scheduler = scheduler
}

// SetQuotaManager sets the quota manager instance for quota adjustments
func (re *RuleEngine) SetQuotaManager(qm *QuotaManager) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.quotaManager = qm
}

// AddWeightRule adds a weight adjustment rule
func (re *RuleEngine) AddWeightRule(rule *WeightAdjustmentRule) {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Set default timezone if not specified
	if rule.Window.Location == nil {
		rule.Window.Location = re.defaultTimezone
	}

	re.weightRules = append(re.weightRules, rule)

	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Added weight rule %q: %s", rule.Name, rule.Window.String())
	}
}

// AddQuotaRule adds a quota adjustment rule
func (re *RuleEngine) AddQuotaRule(rule *QuotaAdjustmentRule) {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Set default timezone if not specified
	if rule.Window.Location == nil {
		rule.Window.Location = re.defaultTimezone
	}

	re.quotaRules = append(re.quotaRules, rule)

	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Added quota rule %q: %s", rule.Name, rule.Window.String())
	}
}

// AddCapacityRule adds a capacity adjustment rule
func (re *RuleEngine) AddCapacityRule(rule *CapacityAdjustmentRule) {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Set default timezone if not specified
	if rule.Window.Location == nil {
		rule.Window.Location = re.defaultTimezone
	}

	re.capacityRules = append(re.capacityRules, rule)

	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Added capacity rule %q: %s", rule.Name, rule.Window.String())
	}
}

// Start begins the rule evaluation loop
func (re *RuleEngine) Start(ctx context.Context) error {
	if !re.enabled {
		if re.logger != nil {
			re.logger.Printf("[INFO] RuleEngine: Disabled, not starting")
		}
		return nil
	}

	re.mu.Lock()
	if re.stopCh != nil {
		re.mu.Unlock()
		return fmt.Errorf("rule engine already started")
	}

	re.stopCh = make(chan struct{})
	re.doneCh = make(chan struct{})
	re.mu.Unlock()

	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Starting (check_interval=%s, rules: weight=%d quota=%d capacity=%d)",
			re.checkInterval,
			len(re.weightRules),
			len(re.quotaRules),
			len(re.capacityRules))
	}

	// Apply rules immediately on startup
	if err := re.ApplyRulesNow(); err != nil {
		if re.logger != nil {
			re.logger.Printf("[WARN] RuleEngine: Failed to apply rules on startup: %v", err)
		}
	}

	// Start background evaluation loop
	go re.evaluationLoop()

	return nil
}

// Stop stops the rule evaluation loop
func (re *RuleEngine) Stop() error {
	re.mu.Lock()
	if re.stopCh == nil {
		re.mu.Unlock()
		return nil // Not started
	}

	close(re.stopCh)
	re.mu.Unlock()

	// Wait for loop to finish
	<-re.doneCh

	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Stopped")
	}

	re.mu.Lock()
	re.stopCh = nil
	re.doneCh = nil
	re.mu.Unlock()

	return nil
}

// evaluationLoop periodically evaluates and applies rules
func (re *RuleEngine) evaluationLoop() {
	defer close(re.doneCh)

	ticker := time.NewTicker(re.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := re.ApplyRulesNow(); err != nil {
				if re.logger != nil {
					re.logger.Printf("[ERROR] RuleEngine: Failed to apply rules: %v", err)
				}
			}

		case <-re.stopCh:
			return
		}
	}
}

// ApplyRulesNow evaluates and applies all rules immediately
func (re *RuleEngine) ApplyRulesNow() error {
	now := re.timeNow()

	re.mu.Lock()
	defer re.mu.Unlock()

	re.lastEvaluation = now

	// Apply weight adjustment rules
	for _, rule := range re.weightRules {
		if rule.IsActive(now) {
			if err := re.applyWeightRule(rule, now); err != nil {
				if re.logger != nil {
					re.logger.Printf("[ERROR] RuleEngine: Failed to apply weight rule %q: %v", rule.Name, err)
				}
			}
		} else {
			// Mark as inactive
			delete(re.activeRules, rule.Name)
		}
	}

	// Apply quota adjustment rules
	for _, rule := range re.quotaRules {
		if rule.IsActive(now) {
			if err := re.applyQuotaRule(rule, now); err != nil {
				if re.logger != nil {
					re.logger.Printf("[ERROR] RuleEngine: Failed to apply quota rule %q: %v", rule.Name, err)
				}
			}
		} else {
			delete(re.activeRules, rule.Name)
		}
	}

	// Apply capacity adjustment rules
	for _, rule := range re.capacityRules {
		if rule.IsActive(now) {
			if err := re.applyCapacityRule(rule, now); err != nil {
				if re.logger != nil {
					re.logger.Printf("[ERROR] RuleEngine: Failed to apply capacity rule %q: %v", rule.Name, err)
				}
			}
		} else {
			delete(re.activeRules, rule.Name)
		}
	}

	return nil
}

// applyWeightRule applies a weight adjustment rule to the scheduler
func (re *RuleEngine) applyWeightRule(rule *WeightAdjustmentRule, now time.Time) error {
	// Check if already active
	if status, exists := re.activeRules[rule.Name]; exists && status.Active {
		return nil // Already applied
	}

	// TODO: Implement scheduler weight adjustment
	// For now, just log the application
	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Applying weight rule %q: weights=%v", rule.Name, rule.Weights)
	}

	// Mark as active
	re.activeRules[rule.Name] = &RuleStatus{
		Name:        rule.Name,
		Type:        rule.Type,
		Active:      true,
		Window:      rule.Window.String(),
		Description: rule.Description,
		LastApplied: now,
	}

	return nil
}

// applyQuotaRule applies a quota adjustment rule
func (re *RuleEngine) applyQuotaRule(rule *QuotaAdjustmentRule, now time.Time) error {
	// Check if already active
	if status, exists := re.activeRules[rule.Name]; exists && status.Active {
		return nil // Already applied
	}

	// TODO: Implement quota adjustments via quotaManager
	// For now, just log the application
	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Applying quota rule %q: %d adjustments", rule.Name, len(rule.Adjustments))
		for _, adj := range rule.Adjustments {
			re.logger.Printf("[INFO]   - Pattern %q: concurrent=%v rps=%v tokens/sec=%v",
				adj.AccountPattern,
				formatIntPtr(adj.MaxConcurrent),
				formatIntPtr(adj.MaxRPS),
				formatInt64Ptr(adj.MaxTokensPerSec))
		}
	}

	// Mark as active
	re.activeRules[rule.Name] = &RuleStatus{
		Name:        rule.Name,
		Type:        rule.Type,
		Active:      true,
		Window:      rule.Window.String(),
		Description: rule.Description,
		LastApplied: now,
	}

	return nil
}

// applyCapacityRule applies a capacity adjustment rule
func (re *RuleEngine) applyCapacityRule(rule *CapacityAdjustmentRule, now time.Time) error {
	// Check if already active
	if status, exists := re.activeRules[rule.Name]; exists && status.Active {
		return nil // Already applied
	}

	// TODO: Implement scheduler capacity adjustment
	// For now, just log the application
	if re.logger != nil {
		re.logger.Printf("[INFO] RuleEngine: Applying capacity rule %q: concurrent=%v rps=%v tokens/sec=%v",
			rule.Name,
			formatIntPtr(rule.MaxConcurrent),
			formatIntPtr(rule.MaxRPS),
			formatInt64Ptr(rule.MaxTokensPerSec))
	}

	// Mark as active
	re.activeRules[rule.Name] = &RuleStatus{
		Name:        rule.Name,
		Type:        rule.Type,
		Active:      true,
		Window:      rule.Window.String(),
		Description: rule.Description,
		LastApplied: now,
	}

	return nil
}

// GetActiveRules returns the current status of all rules
func (re *RuleEngine) GetActiveRules() []RuleStatus {
	re.mu.RLock()
	defer re.mu.RUnlock()

	statuses := make([]RuleStatus, 0, len(re.activeRules))
	for _, status := range re.activeRules {
		statuses = append(statuses, *status)
	}

	return statuses
}

// GetAllRules returns status of all rules (active and inactive)
func (re *RuleEngine) GetAllRules() []RuleStatus {
	re.mu.RLock()
	defer re.mu.RUnlock()

	now := re.timeNow()
	statuses := make([]RuleStatus, 0)

	// Weight rules
	for _, rule := range re.weightRules {
		active := rule.IsActive(now)
		status := RuleStatus{
			Name:        rule.Name,
			Type:        rule.Type,
			Active:      active,
			Window:      rule.Window.String(),
			Description: rule.Description,
		}
		if activeStatus, exists := re.activeRules[rule.Name]; exists {
			status.LastApplied = activeStatus.LastApplied
		}
		statuses = append(statuses, status)
	}

	// Quota rules
	for _, rule := range re.quotaRules {
		active := rule.IsActive(now)
		status := RuleStatus{
			Name:        rule.Name,
			Type:        rule.Type,
			Active:      active,
			Window:      rule.Window.String(),
			Description: rule.Description,
		}
		if activeStatus, exists := re.activeRules[rule.Name]; exists {
			status.LastApplied = activeStatus.LastApplied
		}
		statuses = append(statuses, status)
	}

	// Capacity rules
	for _, rule := range re.capacityRules {
		active := rule.IsActive(now)
		status := RuleStatus{
			Name:        rule.Name,
			Type:        rule.Type,
			Active:      active,
			Window:      rule.Window.String(),
			Description: rule.Description,
		}
		if activeStatus, exists := re.activeRules[rule.Name]; exists {
			status.LastApplied = activeStatus.LastApplied
		}
		statuses = append(statuses, status)
	}

	return statuses
}

// IsEnabled returns whether the rule engine is enabled
func (re *RuleEngine) IsEnabled() bool {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.enabled
}

// Helper function to match account ID against pattern
func matchPattern(accountID, pattern string) bool {
	// Simple wildcard matching (* at end only for now)
	if strings.HasSuffix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(accountID, prefix)
	}
	return accountID == pattern
}

// Helper functions for formatting pointers
func formatIntPtr(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}

func formatInt64Ptr(p *int64) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", *p)
}

// matchPatternWithWildcard matches account ID with wildcard patterns
// Supports:
// - "exact-match" - exact string match
// - "prefix-*" - prefix match (ends with *)
// - "*-suffix" - suffix match (starts with *)
// - "*substring*" - substring match (both ends with *)
func matchPatternWithWildcard(accountID, pattern string) bool {
	// Exact match
	if pattern == accountID {
		return true
	}

	// No wildcards
	if !strings.Contains(pattern, "*") {
		return false
	}

	// Prefix match: "dept-a-*"
	if strings.HasSuffix(pattern, "*") && !strings.HasPrefix(pattern, "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(accountID, prefix)
	}

	// Suffix match: "*-premium"
	if strings.HasPrefix(pattern, "*") && !strings.HasSuffix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(accountID, suffix)
	}

	// Substring match: "*premium*"
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		substring := pattern[1 : len(pattern)-1]
		return strings.Contains(accountID, substring)
	}

	return false
}
