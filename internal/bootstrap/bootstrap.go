package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/config"
)

// InitOptions configures the bootstrap process for generating config files.
type InitOptions struct {
	Root           string
	Environment    string
	Email          string
	DisplayName    string
	BaseURL        string
	EnableProvider bool
	LedgerPath     string
	PublishName    string
	ModelFamily    string
	PricePer1K     float64
	Force          bool
}

// Init scaffolds configuration files for the gateway.
func Init(opts InitOptions) error {
	applyDefaults(&opts)
	if err := ensureDir(filepath.Join(opts.Root, "config", opts.Environment)); err != nil {
		return err
	}

	settingPath := filepath.Join(opts.Root, "config", "setting.ini")
	if err := writeFile(settingPath, settingTemplate(opts), opts.Force); err != nil {
		return err
	}

	gatewayPath := filepath.Join(opts.Root, "config", opts.Environment, "gateway.ini")
	if err := writeFile(gatewayPath, gatewayTemplate(opts), opts.Force); err != nil {
		return err
	}

	return nil
}

func applyDefaults(opts *InitOptions) {
	if strings.TrimSpace(opts.Root) == "" {
		opts.Root = "."
	}
	if strings.TrimSpace(opts.Environment) == "" {
		opts.Environment = "dev"
	}
	if strings.TrimSpace(opts.Email) == "" {
		opts.Email = "dev@example.com"
	}
	if strings.TrimSpace(opts.DisplayName) == "" {
		opts.DisplayName = "Tokligence Gateway"
	}
	if strings.TrimSpace(opts.BaseURL) == "" {
		opts.BaseURL = config.DefaultExchangeBaseURL(opts.Environment)
	}
	if strings.TrimSpace(opts.LedgerPath) == "" {
		opts.LedgerPath = config.DefaultLedgerPath()
	}
	if strings.TrimSpace(opts.PublishName) == "" {
		opts.PublishName = "local-loopback"
	}
	if strings.TrimSpace(opts.ModelFamily) == "" {
		opts.ModelFamily = "claude-3.5-sonnet"
	}
	if opts.PricePer1K <= 0 {
		opts.PricePer1K = 0.5
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFile(path, contents string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file already exists: %s", path)
		}
	}
	return os.WriteFile(path, []byte(contents), 0o644)
}

func settingTemplate(opts InitOptions) string {
	return fmt.Sprintf(`# Tokligence Gateway settings
environment=%s
email=%s
display_name=%s
enable_provider=%t
`, opts.Environment, opts.Email, opts.DisplayName, opts.EnableProvider)
}

func gatewayTemplate(opts InitOptions) string {
	return fmt.Sprintf(`# Environment specific overrides for %s
base_url=%s
log_level=info
# Separate log files (CLI and daemon). Dash '-' disables file output.
log_file_cli=logs/gateway-cli.log
log_file_daemon=logs/gatewayd.log
ledger_path=%s
publish_name=%s
model_family=%s
price_per_1k=%.4f
`, opts.Environment, opts.BaseURL, opts.LedgerPath, opts.PublishName, opts.ModelFamily, opts.PricePer1K)
}

// Validate ensures required fields are present without modifying files.
func Validate(opts InitOptions) error {
	applyDefaults(&opts)
	if strings.TrimSpace(opts.Email) == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(opts.Email, "@") {
		return errors.New("email must contain '@'")
	}
	return nil
}
