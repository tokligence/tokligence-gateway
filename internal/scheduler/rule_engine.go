package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
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
	scheduler    AdjustableScheduler // For weight/capacity adjustments
	quotaManager *QuotaManager       // For quota adjustments

	// Baselines for reversion when rules expire
	baseWeights     []float64
	baseCapacity    CapacityLimits
	baseCapacitySet bool

	// State tracking
	activeRules    map[string]*RuleStatus
	lastEvaluation time.Time
	stopCh         chan struct{}
	doneCh         chan struct{}

	// Hot reload support
	configFilePath    string        // Path to config file for monitoring
	lastModTime       time.Time     // Last modification time of config file
	fileCheckInterval time.Duration // How often to check for file changes

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

// AdjustableScheduler exposes the knobs the rule engine can tune dynamically
type AdjustableScheduler interface {
	CurrentWeights() []float64
	UpdateWeights(weights []float64) error
	CurrentCapacity() CapacityLimits
	UpdateCapacity(maxTokensPerSec *int64, maxRPS *int, maxConcurrent *int, maxContextLength *int) error
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
func (re *RuleEngine) SetScheduler(scheduler AdjustableScheduler) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.scheduler = scheduler
	if scheduler != nil {
		re.baseWeights = scheduler.CurrentWeights()
		re.baseCapacity = scheduler.CurrentCapacity()
		re.baseCapacitySet = true
	}
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

	re.mu.RLock()
	checkInterval := re.checkInterval
	fileInterval := re.fileCheckInterval
	filePath := re.configFilePath
	re.mu.RUnlock()

	if checkInterval <= 0 {
		checkInterval = 60 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	var fileTicker *time.Ticker
	var fileC <-chan time.Time
	if filePath != "" && fileInterval > 0 {
		fileTicker = time.NewTicker(fileInterval)
		fileC = fileTicker.C
	}
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
		if fileTicker != nil {
			fileTicker.Stop()
		}
	}()

	for {
		select {
		case <-ticker.C:
			re.mu.RLock()
			currEnabled := re.enabled
			currInterval := re.checkInterval
			currFileInterval := re.fileCheckInterval
			currFilePath := re.configFilePath
			logger := re.logger
			re.mu.RUnlock()

			if currEnabled {
				if err := re.ApplyRulesNow(); err != nil && logger != nil {
					logger.Printf("[ERROR] RuleEngine: Failed to apply rules: %v", err)
				}
			}

			// Adjust evaluation interval dynamically after reloads
			if currInterval <= 0 {
				currInterval = 60 * time.Second
			}
			if currInterval != checkInterval {
				ticker.Stop()
				ticker = time.NewTicker(currInterval)
				checkInterval = currInterval
			}

			// Adjust file monitoring ticker dynamically
			// Determine desired state: should we have a file ticker?
			wantFileTicker := currFilePath != "" && currFileInterval > 0
			haveFileTicker := fileTicker != nil

			if !wantFileTicker && haveFileTicker {
				// Need to disable file monitoring
				fileTicker.Stop()
				fileTicker = nil
				fileC = nil
				fileInterval = 0
			} else if wantFileTicker && (!haveFileTicker || currFileInterval != fileInterval) {
				// Need to create or recreate file ticker
				if fileTicker != nil {
					fileTicker.Stop()
				}
				fileInterval = currFileInterval
				fileTicker = time.NewTicker(fileInterval)
				fileC = fileTicker.C
			}

		case <-fileC:
			// Check if config file was modified
			if re.checkConfigFileModified() {
				if re.logger != nil {
					re.logger.Printf("[INFO] RuleEngine: Config file changed, reloading...")
				}
				if err := re.ReloadFromFile(); err != nil {
					if re.logger != nil {
						re.logger.Printf("[ERROR] RuleEngine: Failed to reload config: %v", err)
					}
				} else {
					if re.logger != nil {
						re.logger.Printf("[INFO] RuleEngine: Config reloaded successfully")
					}
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

	// Copy state under read lock to minimise contention
	re.mu.RLock()
	weightRules := append([]*WeightAdjustmentRule{}, re.weightRules...)
	quotaRules := append([]*QuotaAdjustmentRule{}, re.quotaRules...)
	capacityRules := append([]*CapacityAdjustmentRule{}, re.capacityRules...)
	scheduler := re.scheduler
	quotaManager := re.quotaManager
	baseWeights := append([]float64{}, re.baseWeights...)
	baseCapacity := re.baseCapacity
	baseCapacitySet := re.baseCapacitySet
	re.mu.RUnlock()

	if scheduler != nil && len(baseWeights) == 0 {
		baseWeights = scheduler.CurrentWeights()
	}
	if scheduler != nil && !baseCapacitySet {
		baseCapacity = scheduler.CurrentCapacity()
		baseCapacitySet = true
	}

	// Persist baselines for future evaluations
	if scheduler != nil {
		re.mu.Lock()
		if len(re.baseWeights) == 0 && len(baseWeights) > 0 {
			re.baseWeights = append([]float64{}, baseWeights...)
		}
		if !re.baseCapacitySet && baseCapacitySet {
			re.baseCapacity = baseCapacity
			re.baseCapacitySet = true
		}
		re.mu.Unlock()
	}

	// Start from baselines
	targetWeights := append([]float64{}, baseWeights...)
	targetCapacity := baseCapacity

	capacityRuleApplied := false
	adjustmentsApplied := false
	weightRuleActive := false

	// Track active rules map updates under write lock
	updateStatus := func(name string, rType RuleType, windowDesc, description string, active bool) {
		re.mu.Lock()
		defer re.mu.Unlock()
		if !active {
			delete(re.activeRules, name)
			return
		}
		re.activeRules[name] = &RuleStatus{
			Name:        name,
			Type:        rType,
			Active:      true,
			Window:      windowDesc,
			Description: description,
			LastApplied: now,
		}
	}

	re.mu.Lock()
	re.lastEvaluation = now
	re.mu.Unlock()

	// Apply weight adjustment rules (last matching wins)
	for _, rule := range weightRules {
		active := rule.IsActive(now)
		if active {
			targetWeights = append([]float64{}, rule.Weights...)
		}
		if active {
			weightRuleActive = true
		}
		updateStatus(rule.Name, rule.Type, rule.Window.String(), rule.Description, active)
	}

	// Apply capacity adjustment rules (overlay, last assignment wins)
	for _, rule := range capacityRules {
		active := rule.IsActive(now)
		if active {
			if rule.MaxConcurrent != nil {
				targetCapacity.MaxConcurrent = *rule.MaxConcurrent
			}
			if rule.MaxRPS != nil {
				targetCapacity.MaxRPS = *rule.MaxRPS
			}
			if rule.MaxTokensPerSec != nil {
				targetCapacity.MaxTokensPerSec = int(*rule.MaxTokensPerSec)
			}
			capacityRuleApplied = true
		}
		updateStatus(rule.Name, rule.Type, rule.Window.String(), rule.Description, active)
	}

	// Apply quota adjustment rules (currently best-effort: mirror active status, no DB writes)
	for _, rule := range quotaRules {
		active := rule.IsActive(now)
		if active && quotaManager != nil {
			quotaManager.ApplyAdjustments(rule.Adjustments)
			adjustmentsApplied = true
		}
		updateStatus(rule.Name, rule.Type, rule.Window.String(), rule.Description, active)
	}

	if quotaManager != nil && !adjustmentsApplied {
		quotaManager.ApplyAdjustments(nil)
	}

	// Push computed targets to scheduler (revert to base when no rule is active)
	if scheduler != nil {
		// Refresh baselines when no rules are active so manual adjustments persist
		if !weightRuleActive {
			current := scheduler.CurrentWeights()
			if len(current) > 0 {
				targetWeights = append([]float64{}, current...)
				re.mu.Lock()
				re.baseWeights = append([]float64{}, current...)
				re.mu.Unlock()
			}
		}
		if !capacityRuleApplied {
			currentCap := scheduler.CurrentCapacity()
			targetCapacity = currentCap
			baseCapacity = currentCap
			baseCapacitySet = true
			re.mu.Lock()
			re.baseCapacity = currentCap
			re.baseCapacitySet = true
			re.mu.Unlock()
		}

		if len(targetWeights) > 0 {
			current := scheduler.CurrentWeights()
			if !weightsEqual(current, targetWeights) {
				if err := scheduler.UpdateWeights(targetWeights); err != nil && re.logger != nil {
					re.logger.Printf("[WARN] RuleEngine: Failed to update weights: %v", err)
				}
			}
		}

		if baseCapacitySet {
			currentCap := scheduler.CurrentCapacity()
			// Revert if no capacity rules applied, otherwise apply computed overlay
			desired := targetCapacity
			if !capacityRuleApplied {
				desired = baseCapacity
			}
			if !capacityEqual(currentCap, desired) {
				// Copy to avoid taking address of loop vars
				maxTokens := int64(desired.MaxTokensPerSec)
				maxRPS := desired.MaxRPS
				maxConcurrent := desired.MaxConcurrent
				maxContext := desired.MaxContextLength
				if err := scheduler.UpdateCapacity(&maxTokens, &maxRPS, &maxConcurrent, &maxContext); err != nil && re.logger != nil {
					re.logger.Printf("[WARN] RuleEngine: Failed to update capacity: %v", err)
				}
			}
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

func weightsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func capacityEqual(a, b CapacityLimits) bool {
	return a.MaxConcurrent == b.MaxConcurrent &&
		a.MaxRPS == b.MaxRPS &&
		a.MaxTokensPerSec == b.MaxTokensPerSec &&
		a.MaxContextLength == b.MaxContextLength
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

// checkConfigFileModified checks if the config file has been modified since last load
func (re *RuleEngine) checkConfigFileModified() bool {
	if re.configFilePath == "" {
		return false
	}

	// Get file info
	info, err := os.Stat(re.configFilePath)
	if err != nil {
		// File doesn't exist or can't be accessed
		if re.logger != nil {
			re.logger.Printf("[WARN] RuleEngine: Failed to stat config file: %v", err)
		}
		return false
	}

	// Check if modification time changed
	modTime := info.ModTime()
	if modTime.After(re.lastModTime) {
		return true
	}

	return false
}

// ReloadFromFile reloads rules from the config file
func (re *RuleEngine) ReloadFromFile() error {
	if re.configFilePath == "" {
		return fmt.Errorf("no config file path configured")
	}

	// Load new rules from file (no lock needed for loading)
	newEngine, err := LoadRulesFromINI(re.configFilePath, re.defaultTimezone.String())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Capture mod time for bookkeeping (best-effort)
	info, statErr := os.Stat(re.configFilePath)
	var newModTime time.Time
	if statErr == nil {
		newModTime = info.ModTime()
	}

	// Lock only for updating the engine state, with rollback on failure
	re.mu.Lock()
	oldWeightRules := re.weightRules
	oldQuotaRules := re.quotaRules
	oldCapacityRules := re.capacityRules
	oldEnabled := re.enabled
	oldCheckInterval := re.checkInterval
	oldDefaultTZ := re.defaultTimezone
	oldFileInterval := re.fileCheckInterval
	oldActive := re.activeRules
	oldLastMod := re.lastModTime

	re.weightRules = newEngine.weightRules
	re.quotaRules = newEngine.quotaRules
	re.capacityRules = newEngine.capacityRules
	re.enabled = newEngine.enabled
	re.checkInterval = newEngine.checkInterval
	re.defaultTimezone = newEngine.defaultTimezone
	re.fileCheckInterval = newEngine.fileCheckInterval
	if !newModTime.IsZero() {
		re.lastModTime = newModTime
	}

	// Clear active rules (will be re-evaluated)
	re.activeRules = make(map[string]*RuleStatus)
	re.mu.Unlock()

	// Apply new rules immediately (ApplyRulesNow handles its own locking)
	if err := re.ApplyRulesNow(); err != nil {
		// Roll back on failure to avoid losing active state
		re.mu.Lock()
		re.weightRules = oldWeightRules
		re.quotaRules = oldQuotaRules
		re.capacityRules = oldCapacityRules
		re.enabled = oldEnabled
		re.checkInterval = oldCheckInterval
		re.defaultTimezone = oldDefaultTZ
		re.fileCheckInterval = oldFileInterval
		re.activeRules = oldActive
		re.lastModTime = oldLastMod
		re.mu.Unlock()
		return fmt.Errorf("failed to apply new rules: %w", err)
	}

	return nil
}

// SetConfigFilePath sets the config file path for hot reload support
func (re *RuleEngine) SetConfigFilePath(path string, checkInterval time.Duration) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.configFilePath = path
	re.fileCheckInterval = checkInterval

	// Get initial mod time
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat config file: %w", err)
	}
	re.lastModTime = info.ModTime()

	return nil
}
