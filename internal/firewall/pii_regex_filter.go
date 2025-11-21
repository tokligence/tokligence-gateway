package firewall

import (
	"fmt"
	"regexp"
	"strings"
)

// PIIPattern defines a regular expression pattern for detecting PII.
type PIIPattern struct {
	Name       string
	Type       string
	Pattern    *regexp.Regexp
	Mask       string // Replacement text, e.g., "[EMAIL]", "[PHONE]"
	Confidence float64
}

// PIIRegexFilter detects and optionally redacts PII using regular expressions.
type PIIRegexFilter struct {
	name          string
	priority      int
	direction     Direction
	patterns      []PIIPattern
	redactEnabled bool
}

// Common PII patterns
var defaultPIIPatterns = []PIIPattern{
	{
		Name:       "email",
		Type:       "EMAIL",
		Pattern:    regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		Mask:       "[EMAIL]",
		Confidence: 0.95,
	},
	{
		Name:       "phone_us",
		Type:       "PHONE",
		Pattern:    regexp.MustCompile(`\b(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`),
		Mask:       "[PHONE]",
		Confidence: 0.90,
	},
	{
		Name:       "phone_intl",
		Type:       "PHONE",
		Pattern:    regexp.MustCompile(`\b\+\d{1,3}[-.\s]?\d{1,4}[-.\s]?\d{1,4}[-.\s]?\d{1,9}\b`),
		Mask:       "[PHONE]",
		Confidence: 0.85,
	},
	{
		Name:       "ssn",
		Type:       "SSN",
		Pattern:    regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		Mask:       "[SSN]",
		Confidence: 0.95,
	},
	{
		Name:       "credit_card",
		Type:       "CREDIT_CARD",
		Pattern:    regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`),
		Mask:       "[CREDIT_CARD]",
		Confidence: 0.85,
	},
	{
		Name:       "ip_address",
		Type:       "IP_ADDRESS",
		Pattern:    regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		Mask:       "[IP]",
		Confidence: 0.80,
	},
	{
		Name:       "api_key",
		Type:       "API_KEY",
		Pattern:    regexp.MustCompile(`\b(sk-[a-zA-Z0-9]{20,}|[a-zA-Z0-9_-]{32,})\b`),
		Mask:       "[API_KEY]",
		Confidence: 0.75,
	},
}

// PIIRegexFilterConfig configures the PII regex filter.
type PIIRegexFilterConfig struct {
	Name          string
	Priority      int
	Direction     Direction
	RedactEnabled bool
	CustomPatterns []PIIPattern
	EnabledTypes   []string // If empty, all default patterns are enabled
}

// NewPIIRegexFilter creates a new PII regex filter.
func NewPIIRegexFilter(config PIIRegexFilterConfig) *PIIRegexFilter {
	if config.Name == "" {
		config.Name = "pii_regex"
	}
	if config.Direction == "" {
		config.Direction = DirectionBoth
	}

	patterns := make([]PIIPattern, 0)

	// Add enabled default patterns
	if len(config.EnabledTypes) == 0 {
		// Use all default patterns
		patterns = append(patterns, defaultPIIPatterns...)
	} else {
		// Only add enabled types
		enabledMap := make(map[string]bool)
		for _, t := range config.EnabledTypes {
			enabledMap[strings.ToUpper(t)] = true
		}
		for _, p := range defaultPIIPatterns {
			if enabledMap[p.Type] {
				patterns = append(patterns, p)
			}
		}
	}

	// Add custom patterns
	patterns = append(patterns, config.CustomPatterns...)

	return &PIIRegexFilter{
		name:          config.Name,
		priority:      config.Priority,
		direction:     config.Direction,
		patterns:      patterns,
		redactEnabled: config.RedactEnabled,
	}
}

func (f *PIIRegexFilter) Name() string {
	return f.name
}

func (f *PIIRegexFilter) Priority() int {
	return f.priority
}

func (f *PIIRegexFilter) Direction() Direction {
	return f.direction
}

func (f *PIIRegexFilter) ApplyInput(ctx *FilterContext) error {
	if ctx.RequestBody == nil || len(ctx.RequestBody) == 0 {
		return nil
	}

	text := string(ctx.RequestBody)
	detections := make([]Detection, 0)
	entities := make([]RedactedEntity, 0)
	modified := text

	for _, pattern := range f.patterns {
		matches := pattern.Pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			start, end := match[0], match[1]
			originalValue := text[start:end]

			// Record detection
			detection := Detection{
				FilterName: f.name,
				Type:       "pii",
				Severity:   "medium",
				Message:    fmt.Sprintf("Detected %s in input", pattern.Type),
				Location:   "input",
				Details: map[string]any{
					"pii_type":   pattern.Type,
					"pattern":    pattern.Name,
					"confidence": pattern.Confidence,
				},
				Timestamp: ctx.StartTime,
			}
			detections = append(detections, detection)

			// Record entity
			entity := RedactedEntity{
				Type:       pattern.Type,
				Mask:       pattern.Mask,
				Start:      start,
				End:        end,
				Confidence: pattern.Confidence,
			}
			entities = append(entities, entity)

			// Redact if enabled
			if f.redactEnabled {
				modified = strings.Replace(modified, originalValue, pattern.Mask, 1)
			}
		}
	}

	// Store results in context
	if len(detections) > 0 {
		ctx.Annotations["pii_detections"] = detections
		ctx.Annotations["pii_entities"] = entities
		ctx.Annotations["pii_count"] = len(entities)

		if f.redactEnabled && modified != text {
			ctx.ModifiedRequestBody = []byte(modified)
			ctx.Annotations["pii_redacted"] = true
		}
	}

	return nil
}

func (f *PIIRegexFilter) ApplyOutput(ctx *FilterContext) error {
	if ctx.ResponseBody == nil || len(ctx.ResponseBody) == 0 {
		return nil
	}

	text := string(ctx.ResponseBody)
	detections := make([]Detection, 0)
	entities := make([]RedactedEntity, 0)
	modified := text

	for _, pattern := range f.patterns {
		matches := pattern.Pattern.FindAllStringSubmatchIndex(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}

			start, end := match[0], match[1]
			originalValue := text[start:end]

			// Record detection
			detection := Detection{
				FilterName: f.name,
				Type:       "pii",
				Severity:   "high", // Higher severity for output leaks
				Message:    fmt.Sprintf("Detected %s in output", pattern.Type),
				Location:   "output",
				Details: map[string]any{
					"pii_type":   pattern.Type,
					"pattern":    pattern.Name,
					"confidence": pattern.Confidence,
				},
				Timestamp: ctx.StartTime,
			}
			detections = append(detections, detection)

			// Record entity
			entity := RedactedEntity{
				Type:       pattern.Type,
				Mask:       pattern.Mask,
				Start:      start,
				End:        end,
				Confidence: pattern.Confidence,
			}
			entities = append(entities, entity)

			// Redact if enabled
			if f.redactEnabled {
				modified = strings.Replace(modified, originalValue, pattern.Mask, 1)
			}
		}
	}

	// Store results in context
	if len(detections) > 0 {
		ctx.Annotations["pii_output_detections"] = detections
		ctx.Annotations["pii_output_entities"] = entities
		ctx.Annotations["pii_output_count"] = len(entities)

		if f.redactEnabled && modified != text {
			ctx.ModifiedResponseBody = []byte(modified)
			ctx.Annotations["pii_output_redacted"] = true
		}
	}

	return nil
}
