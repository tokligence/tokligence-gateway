package firewall

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete firewall configuration.
type Config struct {
	Enabled       bool                 `yaml:"enabled"`
	Mode          string               `yaml:"mode"` // "monitor" or "enforce"
	InputFilters  []FilterConfig       `yaml:"input_filters,omitempty"`
	OutputFilters []FilterConfig       `yaml:"output_filters,omitempty"`
	Policies      PolicyConfig         `yaml:"policies,omitempty"`
	// Redact mode SSE streaming configuration
	RedactSSEBufferTimeoutMs int `yaml:"redact_sse_buffer_timeout_ms,omitempty"` // Max time to wait for closing bracket (default: 500ms)
	RedactSSEMaxBufferLength int `yaml:"redact_sse_max_buffer_length,omitempty"` // Max chars to buffer before force flush (default: 30)
}

// FilterConfig is a generic filter configuration.
type FilterConfig struct {
	Type     string                 `yaml:"type"` // "pii_regex", "http", "custom"
	Name     string                 `yaml:"name,omitempty"`
	Priority int                    `yaml:"priority,omitempty"`
	Enabled  bool                   `yaml:"enabled"`
	Config   map[string]interface{} `yaml:"config,omitempty"`
}

// PolicyConfig defines high-level policy rules.
type PolicyConfig struct {
	BlockOnCategories []string `yaml:"block_on_categories,omitempty"`
	RedactPII         bool     `yaml:"redact_pii"`
	MaxPIIEntities    int      `yaml:"max_pii_entities,omitempty"`
}

// PIIRegexConfig is the configuration for PII regex filter.
type PIIRegexConfig struct {
	RedactEnabled  bool     `yaml:"redact_enabled"`
	EnabledTypes   []string `yaml:"enabled_types,omitempty"`
	CustomPatterns []struct {
		Name    string  `yaml:"name"`
		Type    string  `yaml:"type"`
		Pattern string  `yaml:"pattern"`
		Mask    string  `yaml:"mask"`
		Confidence float64 `yaml:"confidence,omitempty"`
	} `yaml:"custom_patterns,omitempty"`
}

// HTTPConfig is the configuration for HTTP filter.
type HTTPConfig struct {
	Endpoint  string            `yaml:"endpoint"`
	TimeoutMs int               `yaml:"timeout_ms,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	OnError   string            `yaml:"on_error,omitempty"` // "allow", "block", "bypass"
}

// BuildPipeline constructs a Pipeline from the configuration.
func (c *Config) BuildPipeline() (*Pipeline, error) {
	if !c.Enabled {
		return NewPipeline(ModeDisabled, nil), nil
	}

	mode := ModeDisabled // Default to disabled
	switch c.Mode {
	case "enforce":
		mode = ModeEnforce
	case "monitor":
		mode = ModeMonitor
	case "redact":
		mode = ModeRedact
	case "disabled":
		mode = ModeDisabled
	default:
		// If mode is not specified or unknown, default to disabled
		mode = ModeDisabled
	}

	pipeline := NewPipeline(mode, nil)

	// Configure SSE buffer settings for redact mode
	if c.RedactSSEBufferTimeoutMs > 0 || c.RedactSSEMaxBufferLength > 0 {
		cfg := SSEBufferConfig{
			BufferTimeout:   time.Duration(c.RedactSSEBufferTimeoutMs) * time.Millisecond,
			MaxBufferLength: c.RedactSSEMaxBufferLength,
		}
		pipeline.SetSSEBufferConfig(cfg)
	}

	// Build input filters
	for _, fc := range c.InputFilters {
		if !fc.Enabled {
			continue
		}

		filter, err := c.buildFilter(fc, DirectionInput)
		if err != nil {
			return nil, fmt.Errorf("failed to build input filter %s: %w", fc.Name, err)
		}

		if filter != nil {
			pipeline.AddFilter(filter)
		}
	}

	// Build output filters
	for _, fc := range c.OutputFilters {
		if !fc.Enabled {
			continue
		}

		filter, err := c.buildFilter(fc, DirectionOutput)
		if err != nil {
			return nil, fmt.Errorf("failed to build output filter %s: %w", fc.Name, err)
		}

		if filter != nil {
			pipeline.AddFilter(filter)
		}
	}

	return pipeline, nil
}

func (c *Config) buildFilter(fc FilterConfig, direction Direction) (Filter, error) {
	switch fc.Type {
	case "sd_regex", "pii_regex": // sd_regex is preferred, pii_regex kept for backward compatibility
		return c.buildPIIRegexFilter(fc, direction)
	case "http":
		return c.buildHTTPFilter(fc, direction)
	default:
		return nil, fmt.Errorf("unknown filter type: %s", fc.Type)
	}
}

func (c *Config) buildPIIRegexFilter(fc FilterConfig, direction Direction) (Filter, error) {
	config := PIIRegexFilterConfig{
		Name:          fc.Name,
		Priority:      fc.Priority,
		Direction:     direction,
		RedactEnabled: c.Policies.RedactPII, // Default from policy
	}

	// Parse specific config
	if configMap, ok := fc.Config["redact_enabled"].(bool); ok {
		config.RedactEnabled = configMap
	}

	if enabledTypes, ok := fc.Config["enabled_types"].([]interface{}); ok {
		config.EnabledTypes = make([]string, 0, len(enabledTypes))
		for _, t := range enabledTypes {
			if s, ok := t.(string); ok {
				config.EnabledTypes = append(config.EnabledTypes, s)
			}
		}
	}

	return NewPIIRegexFilter(config), nil
}

func (c *Config) buildHTTPFilter(fc FilterConfig, direction Direction) (Filter, error) {
	endpoint, ok := fc.Config["endpoint"].(string)
	if !ok || endpoint == "" {
		return nil, fmt.Errorf("http filter requires 'endpoint' in config")
	}

	config := HTTPFilterConfig{
		Name:      fc.Name,
		Priority:  fc.Priority,
		Direction: direction,
		Endpoint:  endpoint,
		Timeout:   5 * time.Second,
		OnError:   ErrorActionBypass,
	}

	if timeoutMs, ok := fc.Config["timeout_ms"].(int); ok && timeoutMs > 0 {
		config.Timeout = time.Duration(timeoutMs) * time.Millisecond
	}

	if onError, ok := fc.Config["on_error"].(string); ok {
		config.OnError = ErrorAction(onError)
	}

	if headers, ok := fc.Config["headers"].(map[string]interface{}); ok {
		config.Headers = make(map[string]string)
		for k, v := range headers {
			if s, ok := v.(string); ok {
				config.Headers[k] = s
			}
		}
	}

	return NewHTTPFilter(config), nil
}

// LoadConfig loads firewall configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	return &config, nil
}

// LoadConfigFromINI loads firewall configuration from an INI file.
func LoadConfigFromINI(path string) (*Config, error) {
	merged, err := parseINI(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse INI file %s: %w", path, err)
	}

	return LoadConfigFromMap(merged)
}

// LoadConfigFromMap loads firewall configuration from a map (from INI file).
func LoadConfigFromMap(merged map[string]string) (*Config, error) {
	config := DefaultConfig()

	// Helper function to get value with environment variable override
	firstNonEmpty := func(values ...string) string {
		for _, v := range values {
			if v != "" {
				return v
			}
		}
		return ""
	}

	// Parse [prompt_firewall] section with environment variable overrides
	// Priority: TOKLIGENCE_PROMPT_FIREWALL_* > INI file > defaults
	if enabled := firstNonEmpty(os.Getenv("TOKLIGENCE_PROMPT_FIREWALL_ENABLED"), merged["prompt_firewall.enabled"]); enabled != "" {
		config.Enabled = strings.ToLower(enabled) == "true"
	}

	if mode := firstNonEmpty(os.Getenv("TOKLIGENCE_PROMPT_FIREWALL_MODE"), merged["prompt_firewall.mode"]); mode != "" {
		config.Mode = strings.ToLower(strings.TrimSpace(mode))
	}

	// Parse redact mode SSE streaming configuration (under [prompt_firewall] section)
	if timeoutMs := firstNonEmpty(os.Getenv("TOKLIGENCE_PROMPT_FIREWALL_SSE_BUFFER_TIMEOUT_MS"), merged["prompt_firewall.redact_sse_buffer_timeout_ms"]); timeoutMs != "" {
		if ti, err := strconv.Atoi(timeoutMs); err == nil && ti > 0 {
			config.RedactSSEBufferTimeoutMs = ti
		}
	}
	if maxLen := firstNonEmpty(os.Getenv("TOKLIGENCE_PROMPT_FIREWALL_SSE_MAX_BUFFER_LENGTH"), merged["prompt_firewall.redact_sse_max_buffer_length"]); maxLen != "" {
		if mi, err := strconv.Atoi(maxLen); err == nil && mi > 0 {
			config.RedactSSEMaxBufferLength = mi
		}
	}

	// Parse [firewall_input_filters] section
	config.InputFilters = []FilterConfig{}

	// Parse SD (Sensitive Data) regex filter
	// Supports both new name (filter_sd_regex_*) and legacy name (filter_pii_regex_*) for backward compatibility
	sdRegexEnabled := merged["firewall_input_filters.filter_sd_regex_enabled"]
	if sdRegexEnabled == "" {
		sdRegexEnabled = merged["firewall_input_filters.filter_pii_regex_enabled"] // Legacy fallback
	}
	if strings.ToLower(sdRegexEnabled) == "true" {
		priority := 10
		if p, ok := merged["firewall_input_filters.filter_sd_regex_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		} else if p, ok := merged["firewall_input_filters.filter_pii_regex_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		}
		config.InputFilters = append(config.InputFilters, FilterConfig{
			Type:     "sd_regex",
			Name:     "sd_regex_input",
			Priority: priority,
			Enabled:  true,
			Config:   map[string]interface{}{},
		})
	}

	// Parse HTTP filter (input) - generic external scanning service
	// Supports both new name (filter_http_*) and legacy name (filter_presidio_*) for backward compatibility
	httpEnabled := merged["firewall_input_filters.filter_http_enabled"]
	if httpEnabled == "" {
		httpEnabled = merged["firewall_input_filters.filter_presidio_enabled"] // Legacy fallback
	}
	if strings.ToLower(httpEnabled) == "true" {
		priority := 20
		if p, ok := merged["firewall_input_filters.filter_http_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		} else if p, ok := merged["firewall_input_filters.filter_presidio_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		}

		endpoint := merged["firewall_input_filters.filter_http_endpoint"]
		if endpoint == "" {
			endpoint = merged["firewall_input_filters.filter_presidio_endpoint"] // Legacy fallback
		}

		timeoutMs := 5000
		if t, ok := merged["firewall_input_filters.filter_http_timeout_ms"]; ok {
			if ti, err := strconv.Atoi(t); err == nil {
				timeoutMs = ti
			}
		} else if t, ok := merged["firewall_input_filters.filter_presidio_timeout_ms"]; ok {
			if ti, err := strconv.Atoi(t); err == nil {
				timeoutMs = ti
			}
		}

		onError := "allow"
		if oe, ok := merged["firewall_input_filters.filter_http_on_error"]; ok {
			onError = oe
		} else if oe, ok := merged["firewall_input_filters.filter_presidio_on_error"]; ok {
			onError = oe
		}

		// Parse custom headers (filter_http_header_<HeaderName> = <value>)
		headers := make(map[string]interface{})
		for key, value := range merged {
			if strings.HasPrefix(key, "firewall_input_filters.filter_http_header_") {
				headerName := strings.TrimPrefix(key, "firewall_input_filters.filter_http_header_")
				headers[headerName] = value
			}
		}

		config.InputFilters = append(config.InputFilters, FilterConfig{
			Type:     "http",
			Name:     "http_input",
			Priority: priority,
			Enabled:  true,
			Config: map[string]interface{}{
				"endpoint":   endpoint,
				"timeout_ms": timeoutMs,
				"on_error":   onError,
				"headers":    headers,
			},
		})
	}

	// Parse [firewall_output_filters] section
	config.OutputFilters = []FilterConfig{}

	// Parse SD (Sensitive Data) regex filter (output)
	// Supports both new name (filter_sd_regex_*) and legacy name (filter_pii_regex_*) for backward compatibility
	sdRegexOutputEnabled := merged["firewall_output_filters.filter_sd_regex_enabled"]
	if sdRegexOutputEnabled == "" {
		sdRegexOutputEnabled = merged["firewall_output_filters.filter_pii_regex_enabled"] // Legacy fallback
	}
	if strings.ToLower(sdRegexOutputEnabled) == "true" {
		priority := 10
		if p, ok := merged["firewall_output_filters.filter_sd_regex_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		} else if p, ok := merged["firewall_output_filters.filter_pii_regex_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		}
		config.OutputFilters = append(config.OutputFilters, FilterConfig{
			Type:     "sd_regex",
			Name:     "sd_regex_output",
			Priority: priority,
			Enabled:  true,
			Config:   map[string]interface{}{},
		})
	}

	// Parse HTTP filter (output) - generic external scanning service
	// Supports both new name (filter_http_*) and legacy name (filter_presidio_*) for backward compatibility
	httpOutputEnabled := merged["firewall_output_filters.filter_http_enabled"]
	if httpOutputEnabled == "" {
		httpOutputEnabled = merged["firewall_output_filters.filter_presidio_enabled"] // Legacy fallback
	}
	if strings.ToLower(httpOutputEnabled) == "true" {
		priority := 20
		if p, ok := merged["firewall_output_filters.filter_http_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		} else if p, ok := merged["firewall_output_filters.filter_presidio_priority"]; ok {
			if pi, err := strconv.Atoi(p); err == nil {
				priority = pi
			}
		}

		endpoint := merged["firewall_output_filters.filter_http_endpoint"]
		if endpoint == "" {
			endpoint = merged["firewall_output_filters.filter_presidio_endpoint"] // Legacy fallback
		}

		timeoutMs := 5000
		if t, ok := merged["firewall_output_filters.filter_http_timeout_ms"]; ok {
			if ti, err := strconv.Atoi(t); err == nil {
				timeoutMs = ti
			}
		} else if t, ok := merged["firewall_output_filters.filter_presidio_timeout_ms"]; ok {
			if ti, err := strconv.Atoi(t); err == nil {
				timeoutMs = ti
			}
		}

		onError := "allow"
		if oe, ok := merged["firewall_output_filters.filter_http_on_error"]; ok {
			onError = oe
		} else if oe, ok := merged["firewall_output_filters.filter_presidio_on_error"]; ok {
			onError = oe
		}

		// Parse custom headers (filter_http_header_<HeaderName> = <value>)
		headers := make(map[string]interface{})
		for key, value := range merged {
			if strings.HasPrefix(key, "firewall_output_filters.filter_http_header_") {
				headerName := strings.TrimPrefix(key, "firewall_output_filters.filter_http_header_")
				headers[headerName] = value
			}
		}

		config.OutputFilters = append(config.OutputFilters, FilterConfig{
			Type:     "http",
			Name:     "http_output",
			Priority: priority,
			Enabled:  true,
			Config: map[string]interface{}{
				"endpoint":   endpoint,
				"timeout_ms": timeoutMs,
				"on_error":   onError,
				"headers":    headers,
			},
		})
	}

	return &config, nil
}

// parseINI parses an INI file and returns a flat map with dotted keys
func parseINI(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	var section string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Build dotted key
		if section != "" {
			key = section + "." + key
		}

		result[key] = value
	}

	return result, nil
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
	return Config{
		Enabled: false,
		Mode:    "disabled", // Default to disabled (recommended: use "redact" mode in production)
		InputFilters: []FilterConfig{
			{
				Type:     "sd_regex",
				Name:     "input_sd",
				Priority: 10,
				Enabled:  true,
				Config: map[string]interface{}{
					"redact_enabled": false,
					"enabled_types":  []string{"EMAIL", "PHONE", "SSN", "CREDIT_CARD", "API_KEY"},
				},
			},
		},
		OutputFilters: []FilterConfig{
			{
				Type:     "sd_regex",
				Name:     "output_sd",
				Priority: 10,
				Enabled:  true,
				Config: map[string]interface{}{
					"redact_enabled": false,
					"enabled_types":  []string{"EMAIL", "PHONE", "SSN", "CREDIT_CARD", "API_KEY"},
				},
			},
		},
		Policies: PolicyConfig{
			RedactPII:      false,
			MaxPIIEntities: 5,
		},
	}
}
