package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// LoadRulesFromINI loads time-based rules from an INI configuration file
func LoadRulesFromINI(filepath string, defaultTimezone string) (*RuleEngine, error) {
	cfg, err := ini.Load(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	// Parse main config section
	mainSec := cfg.Section("time_rules")
	if mainSec == nil {
		return nil, fmt.Errorf("missing [time_rules] section")
	}

	enabled := mainSec.Key("enabled").MustBool(false)
	checkIntervalSec := mainSec.Key("check_interval_sec").MustInt(60)
	timezone := mainSec.Key("default_timezone").MustString(defaultTimezone)

	// Create rule engine
	engineConfig := RuleEngineConfig{
		Enabled:         enabled,
		CheckInterval:   time.Duration(checkIntervalSec) * time.Second,
		DefaultTimezone: timezone,
	}

	engine, err := NewRuleEngine(engineConfig)
	if err != nil {
		return nil, err
	}

	// Set config file path for hot reload support (default check interval: 30 seconds)
	fileCheckIntervalSec := mainSec.Key("file_check_interval_sec").MustInt(30)
	if err := engine.SetConfigFilePath(filepath, time.Duration(fileCheckIntervalSec)*time.Second); err != nil {
		// Log warning but don't fail
		// This can fail if file doesn't exist, which is handled elsewhere
	}

	// Parse all rule sections
	for _, section := range cfg.Sections() {
		sectionName := section.Name()

		// Skip non-rule sections
		if !strings.HasPrefix(sectionName, "rule.") {
			continue
		}

		ruleType := section.Key("type").String()
		if ruleType == "" {
			continue // Skip sections without type
		}

		switch RuleType(ruleType) {
		case RuleTypeWeightAdjustment:
			rule, err := parseWeightRule(section, engine.defaultTimezone)
			if err != nil {
				return nil, fmt.Errorf("failed to parse weight rule [%s]: %w", sectionName, err)
			}
			engine.AddWeightRule(rule)

		case RuleTypeQuotaAdjustment:
			rule, err := parseQuotaRule(section, engine.defaultTimezone)
			if err != nil {
				return nil, fmt.Errorf("failed to parse quota rule [%s]: %w", sectionName, err)
			}
			engine.AddQuotaRule(rule)

		case RuleTypeCapacityAdjustment:
			rule, err := parseCapacityRule(section, engine.defaultTimezone)
			if err != nil {
				return nil, fmt.Errorf("failed to parse capacity rule [%s]: %w", sectionName, err)
			}
			engine.AddCapacityRule(rule)

		default:
			return nil, fmt.Errorf("unknown rule type %q in section [%s]", ruleType, sectionName)
		}
	}

	return engine, nil
}

// parseWeightRule parses a weight adjustment rule from INI section
func parseWeightRule(sec *ini.Section, defaultTz *time.Location) (*WeightAdjustmentRule, error) {
	name := sec.Key("name").String()
	if name == "" {
		return nil, fmt.Errorf("missing 'name' field")
	}

	description := sec.Key("description").String()
	enabled := sec.Key("enabled").MustBool(true)

	// Parse time window
	window, err := parseTimeWindow(sec, defaultTz)
	if err != nil {
		return nil, err
	}

	// Parse weights
	weightsStr := sec.Key("weights").String()
	if weightsStr == "" {
		return nil, fmt.Errorf("missing 'weights' field")
	}

	weights, err := parseFloatArray(weightsStr)
	if err != nil {
		return nil, fmt.Errorf("invalid 'weights': %w", err)
	}

	return &WeightAdjustmentRule{
		BaseRule: BaseRule{
			Name:        name,
			Type:        RuleTypeWeightAdjustment,
			Window:      *window,
			Description: description,
			Enabled:     enabled,
		},
		Weights: weights,
	}, nil
}

// parseQuotaRule parses a quota adjustment rule from INI section
func parseQuotaRule(sec *ini.Section, defaultTz *time.Location) (*QuotaAdjustmentRule, error) {
	name := sec.Key("name").String()
	if name == "" {
		return nil, fmt.Errorf("missing 'name' field")
	}

	description := sec.Key("description").String()
	enabled := sec.Key("enabled").MustBool(true)

	// Parse time window
	window, err := parseTimeWindow(sec, defaultTz)
	if err != nil {
		return nil, err
	}

	// Parse quota adjustments
	adjustments := make([]QuotaAdjustment, 0)

	for _, key := range sec.Keys() {
		if !strings.HasPrefix(key.Name(), "quota.") {
			continue
		}

		// Extract account pattern from key name
		// e.g., "quota.dept-a-*" -> "dept-a-*"
		pattern := key.Name()[6:] // Skip "quota." prefix

		// Parse quota values
		// Format: "concurrent:50,rps:100,tokens_per_sec:2000"
		adj, err := parseQuotaAdjustment(pattern, key.String())
		if err != nil {
			return nil, fmt.Errorf("invalid quota adjustment for %q: %w", pattern, err)
		}

		adjustments = append(adjustments, adj)
	}

	if len(adjustments) == 0 {
		return nil, fmt.Errorf("no quota adjustments defined (use quota.PATTERN keys)")
	}

	return &QuotaAdjustmentRule{
		BaseRule: BaseRule{
			Name:        name,
			Type:        RuleTypeQuotaAdjustment,
			Window:      *window,
			Description: description,
			Enabled:     enabled,
		},
		Adjustments: adjustments,
	}, nil
}

// parseCapacityRule parses a capacity adjustment rule from INI section
func parseCapacityRule(sec *ini.Section, defaultTz *time.Location) (*CapacityAdjustmentRule, error) {
	name := sec.Key("name").String()
	if name == "" {
		return nil, fmt.Errorf("missing 'name' field")
	}

	description := sec.Key("description").String()
	enabled := sec.Key("enabled").MustBool(true)

	// Parse time window
	window, err := parseTimeWindow(sec, defaultTz)
	if err != nil {
		return nil, err
	}

	rule := &CapacityAdjustmentRule{
		BaseRule: BaseRule{
			Name:        name,
			Type:        RuleTypeCapacityAdjustment,
			Window:      *window,
			Description: description,
			Enabled:     enabled,
		},
	}

	// Parse capacity values (all optional)
	if sec.HasKey("max_concurrent") {
		v := sec.Key("max_concurrent").MustInt(0)
		rule.MaxConcurrent = &v
	}

	if sec.HasKey("max_rps") {
		v := sec.Key("max_rps").MustInt(0)
		rule.MaxRPS = &v
	}

	if sec.HasKey("max_tokens_per_sec") {
		v := sec.Key("max_tokens_per_sec").MustInt64(0)
		rule.MaxTokensPerSec = &v
	}

	return rule, nil
}

// parseTimeWindow parses a time window from INI section
func parseTimeWindow(sec *ini.Section, defaultTz *time.Location) (*TimeWindow, error) {
	// Parse start time (required)
	startTimeStr := sec.Key("start_time").String()
	if startTimeStr == "" {
		return nil, fmt.Errorf("missing 'start_time' field")
	}

	startHour, startMinute, err := parseTime(startTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid 'start_time': %w", err)
	}

	// Parse end time (required)
	endTimeStr := sec.Key("end_time").String()
	if endTimeStr == "" {
		return nil, fmt.Errorf("missing 'end_time' field")
	}

	endHour, endMinute, err := parseTime(endTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid 'end_time': %w", err)
	}

	window := &TimeWindow{
		StartHour:   startHour,
		StartMinute: startMinute,
		EndHour:     endHour,
		EndMinute:   endMinute,
		Location:    defaultTz,
	}

	// Parse days of week (optional)
	if sec.HasKey("days_of_week") {
		daysStr := sec.Key("days_of_week").String()
		days, err := parseDaysOfWeek(daysStr)
		if err != nil {
			return nil, fmt.Errorf("invalid 'days_of_week': %w", err)
		}
		window.DaysOfWeek = days
	}

	// Parse timezone (optional, overrides default)
	if sec.HasKey("timezone") {
		tzStr := sec.Key("timezone").String()
		tz, err := time.LoadLocation(tzStr)
		if err != nil {
			return nil, fmt.Errorf("invalid 'timezone': %w", err)
		}
		window.Location = tz
	}

	return window, nil
}

// parseTime parses a time string in format "HH:MM" (24-hour)
func parseTime(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected format HH:MM, got %q", s)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour: %q", parts[0])
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute: %q", parts[1])
	}

	return hour, minute, nil
}

// parseDaysOfWeek parses a comma-separated list of weekday names
// Format: "Mon,Tue,Wed,Thu,Fri" or "Monday,Tuesday,Wednesday"
func parseDaysOfWeek(s string) ([]time.Weekday, error) {
	parts := strings.Split(s, ",")
	days := make([]time.Weekday, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		day, err := parseWeekday(part)
		if err != nil {
			return nil, err
		}
		days = append(days, day)
	}

	return days, nil
}

// parseWeekday parses a weekday name (short or long form)
func parseWeekday(s string) (time.Weekday, error) {
	s = strings.ToLower(s)

	switch s {
	case "mon", "monday":
		return time.Monday, nil
	case "tue", "tuesday":
		return time.Tuesday, nil
	case "wed", "wednesday":
		return time.Wednesday, nil
	case "thu", "thursday":
		return time.Thursday, nil
	case "fri", "friday":
		return time.Friday, nil
	case "sat", "saturday":
		return time.Saturday, nil
	case "sun", "sunday":
		return time.Sunday, nil
	default:
		return 0, fmt.Errorf("unknown weekday: %q", s)
	}
}

// parseFloatArray parses a comma-separated list of floats
// Format: "256,128,64,32,16,8,4,2,1,1"
func parseFloatArray(s string) ([]float64, error) {
	parts := strings.Split(s, ",")
	result := make([]float64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		f, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float %q: %w", part, err)
		}
		result = append(result, f)
	}

	return result, nil
}

// parseQuotaAdjustment parses a quota adjustment value string
// Format: "concurrent:50,rps:100,tokens_per_sec:2000"
func parseQuotaAdjustment(pattern, valueStr string) (QuotaAdjustment, error) {
	adj := QuotaAdjustment{
		AccountPattern: pattern,
	}

	parts := strings.Split(valueStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return adj, fmt.Errorf("invalid format, expected key:value, got %q", part)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		switch key {
		case "concurrent", "max_concurrent":
			v, err := strconv.Atoi(value)
			if err != nil {
				return adj, fmt.Errorf("invalid concurrent value %q: %w", value, err)
			}
			adj.MaxConcurrent = &v

		case "rps", "max_rps":
			v, err := strconv.Atoi(value)
			if err != nil {
				return adj, fmt.Errorf("invalid rps value %q: %w", value, err)
			}
			adj.MaxRPS = &v

		case "tokens_per_sec", "max_tokens_per_sec":
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return adj, fmt.Errorf("invalid tokens_per_sec value %q: %w", value, err)
			}
			adj.MaxTokensPerSec = &v

		default:
			return adj, fmt.Errorf("unknown quota key %q", key)
		}
	}

	return adj, nil
}
