package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	Environment    string
	BaseURL        string
	Email          string
	DisplayName    string
	EnableProvider bool
	PublishName    string
	ModelFamily    string
	PricePer1K     float64
	LogFile        string
	LogLevel       string
	HTTPAddress    string
	LedgerPath     string
	AuthSecret     string
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
		Environment:    s.Environment,
		BaseURL:        firstNonEmpty(merged["base_url"], "http://localhost:8080"),
		Email:          merged["email"],
		DisplayName:    merged["display_name"],
		EnableProvider: parseBool(merged["enable_provider"]),
		PublishName:    merged["publish_name"],
		ModelFamily:    merged["model_family"],
		LogFile:        merged["log_file"],
		LogLevel:       firstNonEmpty(merged["log_level"], "info"),
		HTTPAddress:    firstNonEmpty(merged["http_address"], ":8081"),
		LedgerPath:     firstNonEmpty(merged["ledger_path"], DefaultLedgerPath()),
		AuthSecret:     firstNonEmpty(os.Getenv("TOKLIGENCE_AUTH_SECRET"), merged["auth_secret"], "tokligence-dev-secret"),
		PricePer1K:     0.5,
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// DefaultLedgerPath returns the fallback ledger location under the user's home directory.
func DefaultLedgerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "ledger.db"
	}
	return filepath.Join(home, ".mfg", "ledger.db")
}
