package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/hooks"
)

const (
	settingsFile     = "config/setting.ini"
	defaultEnv       = "dev"
	envConfigPattern = "config/%s/gateway.ini"
)

// Settings contains global toggles such as the active environment.
type Settings struct {
	Environment string
	Defaults    map[string]string
}

// GatewayConfig describes runtime options for the CLI.
type GatewayConfig struct {
	Environment      string
	BaseURL          string
	Email              string
	DisplayName        string
	EnableProvider     bool
	MarketplaceEnabled bool
	TelemetryEnabled   bool
	AdminEmail       string
	PublishName      string
	ModelFamily      string
	PricePer1K       float64
	LogFile          string
	LogLevel         string
	HTTPAddress      string
	LedgerPath       string
	AuthSecret       string
	IdentityPath     string
	Hooks            hooks.Config
}

// LoadGatewayConfig reads the current environment and loads the appropriate gateway config file.
func LoadGatewayConfig(root string) (GatewayConfig, error) {
	if root == "" {
		root = "."
	}
	s, err := loadSettings(root)
	if err != nil {
		return GatewayConfig{}, err
	}

	envValues, err := parseINI(filepath.Join(root, fmt.Sprintf(envConfigPattern, s.Environment)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			envValues = map[string]string{}
		} else {
			return GatewayConfig{}, err
		}
	}

	merged := make(map[string]string)
	for k, v := range s.Defaults {
		merged[k] = v
	}
	for k, v := range envValues {
		merged[k] = v
	}

	cfg := GatewayConfig{
		Environment:      s.Environment,
		BaseURL:          firstNonEmpty(merged["base_url"], DefaultExchangeBaseURL(s.Environment)),
		Email:            merged["email"],
		DisplayName:      merged["display_name"],
		EnableProvider:   parseBool(merged["enable_provider"]),
		MarketplaceEnabled: parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_MARKETPLACE_ENABLED"), merged["marketplace_enabled"]), true),
		TelemetryEnabled: parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_TELEMETRY_ENABLED"), merged["telemetry_enabled"]), true),
		AdminEmail:       firstNonEmpty(os.Getenv("TOKLIGENCE_ADMIN_EMAIL"), merged["admin_email"], "admin@local"),
		PublishName:      merged["publish_name"],
		ModelFamily:      merged["model_family"],
		LogFile:          merged["log_file"],
		LogLevel:         firstNonEmpty(merged["log_level"], "info"),
		HTTPAddress:      firstNonEmpty(merged["http_address"], ":8081"),
		LedgerPath:       firstNonEmpty(merged["ledger_path"], DefaultLedgerPath()),
		AuthSecret:       firstNonEmpty(os.Getenv("TOKLIGENCE_AUTH_SECRET"), merged["auth_secret"], "tokligence-dev-secret"),
		PricePer1K:       0.5,
		IdentityPath:     firstNonEmpty(os.Getenv("TOKLIGENCE_IDENTITY_PATH"), merged["identity_path"], DefaultIdentityPath()),
	}
	hookArgs := firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_SCRIPT_ARGS"), merged["hooks_script_args"])
	hookEnv := firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_SCRIPT_ENV"), merged["hooks_script_env"])
	cfg.Hooks = hooks.Config{
		Enabled:    parseBool(firstNonEmpty(os.Getenv("TOKLIGENCE_HOOKS_ENABLED"), merged["hooks_enabled"])),
		ScriptPath: firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_SCRIPT"), merged["hooks_script_path"]),
		ScriptArgs: parseCSV(hookArgs),
		Env:        parseMap(hookEnv),
	}
	if v := firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_TIMEOUT"), merged["hooks_timeout"]); v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil {
			return GatewayConfig{}, fmt.Errorf("invalid hooks_timeout %q: %w", v, err)
		}
		cfg.Hooks.Timeout = dur
	}
	if err := cfg.Hooks.Validate(); err != nil {
		return GatewayConfig{}, err
	}
	if v := merged["price_per_1k"]; v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.PricePer1K = parsed
		} else {
			return GatewayConfig{}, fmt.Errorf("invalid price_per_1k %q: %w", v, err)
		}
	}
	return cfg, nil
}

func loadSettings(root string) (Settings, error) {
	values, err := parseINI(filepath.Join(root, settingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return Settings{Environment: defaultEnv, Defaults: map[string]string{}}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	env := values["environment"]
	if env == "" {
		env = defaultEnv
	}
	defaults := make(map[string]string)
	for k, v := range values {
		if k == "environment" {
			continue
		}
		defaults[k] = v
	}
	return Settings{Environment: env, Defaults: defaults}, nil
}

func parseINI(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		values[strings.ToLower(key)] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseOptionalBool(v string, fallback bool) bool {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return parseBool(v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var out []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseMap(input string) map[string]string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	entries := strings.Split(input, ",")
	result := make(map[string]string)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if key != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// DefaultLedgerPath returns the fallback ledger location under the user's home directory.
func DefaultLedgerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "ledger.db"
	}
	return filepath.Join(home, ".tokligence", "ledger.db")
}

// DefaultIdentityPath returns the fallback identity database path.
func DefaultIdentityPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "identity.db"
	}
	return filepath.Join(home, ".tokligence", "identity.db")
}

// DefaultExchangeBaseURL returns the canonical Token Marketplace host for the given environment.
func DefaultExchangeBaseURL(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "dev":
		return "https://dev.tokligence.ai"
	case "test":
		return "https://test.tokligence.ai"
	case "live", "prod", "production":
		return "https://marketplace.tokligence.ai"
	default:
		return "http://localhost:8080"
	}
}
