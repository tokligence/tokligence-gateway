package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadGatewayConfig(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setting := "environment=dev\nemail=base@example.com\ndisplay_name=Base\nprice_per_1k=0.75\nlog_file=/tmp/base.log\nlog_level=debug\n"
	if err := os.WriteFile(filepath.Join(tmp, "config", "setting.ini"), []byte(setting), 0o644); err != nil {
		t.Fatalf("write setting: %v", err)
	}
	content := "base_url=http://example.com\ndisplay_name=Test\nenable_provider=true\nprice_per_1k=1.25\nlog_file=/tmp/env.log\nfacade_port=9090\nledger_path=/tmp/custom-ledger.db\nauth_secret=override-secret\n"
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte(content), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}
	os.Setenv("TOKLIGENCE_AUTH_SECRET", "env-secret")
	os.Unsetenv("TOKLIGENCE_MARKETPLACE_ENABLED") // Clear env var to test default behavior
	t.Cleanup(func() { os.Unsetenv("TOKLIGENCE_AUTH_SECRET") })

	cfg, err := LoadGatewayConfig(tmp)
	if err != nil {
		t.Fatalf("LoadGatewayConfig: %v", err)
	}
	if !cfg.EnableProvider {
		t.Fatalf("expected provider enabled")
	}
	if cfg.PricePer1K != 1.25 {
		t.Fatalf("unexpected price %v", cfg.PricePer1K)
	}
	if cfg.LogFile != "/tmp/env.log" {
		t.Fatalf("unexpected log file %s", cfg.LogFile)
	}
	if cfg.Email != "base@example.com" {
		t.Fatalf("expected email from base config, got %s", cfg.Email)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected log level from base config, got %s", cfg.LogLevel)
	}
	if cfg.FacadePort != 9090 {
		t.Fatalf("unexpected facade port %d", cfg.FacadePort)
	}
	if cfg.LedgerPath != "/tmp/custom-ledger.db" {
		t.Fatalf("unexpected ledger path %s", cfg.LedgerPath)
	}
	if cfg.AuthSecret != "env-secret" {
		t.Fatalf("unexpected auth secret %s", cfg.AuthSecret)
	}
	if !cfg.MarketplaceEnabled {
		t.Fatalf("marketplace should be enabled by default")
	}
	if cfg.AdminEmail != "admin@local" {
		t.Fatalf("unexpected admin email %s", cfg.AdminEmail)
	}
	if cfg.IdentityPath != DefaultIdentityPath() {
		t.Fatalf("unexpected identity path %s", cfg.IdentityPath)
	}
}

func TestLoadGatewayConfigHooks(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setting := "environment=dev\n"
	if err := os.WriteFile(filepath.Join(tmp, "config", "setting.ini"), []byte(setting), 0o644); err != nil {
		t.Fatalf("write setting: %v", err)
	}
	hookIni := strings.Join([]string{
		"hooks_enabled=true",
		"hooks_script_path=/usr/local/bin/sync-hooks",
		"hooks_script_args=--seed, --refresh",
		"hooks_script_env=FOO=BAR,BIZ=BUZ",
		"hooks_timeout=45s",
	}, "\n")
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte(hookIni), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}
	os.Setenv("TOKLIGENCE_HOOK_SCRIPT_ARGS", "--from-env")
	os.Setenv("TOKLIGENCE_HOOK_SCRIPT_ENV", "ENVSET=1")
	os.Setenv("TOKLIGENCE_HOOK_TIMEOUT", "30s")
	os.Unsetenv("TOKLIGENCE_MARKETPLACE_ENABLED") // Clear env var to test default behavior
	t.Cleanup(func() {
		os.Unsetenv("TOKLIGENCE_HOOK_SCRIPT_ARGS")
		os.Unsetenv("TOKLIGENCE_HOOK_SCRIPT_ENV")
		os.Unsetenv("TOKLIGENCE_HOOK_TIMEOUT")
	})

	cfg, err := LoadGatewayConfig(tmp)
	if err != nil {
		t.Fatalf("LoadGatewayConfig: %v", err)
	}
	if !cfg.Hooks.Enabled {
		t.Fatalf("expected hooks to be enabled")
	}
	if cfg.Hooks.ScriptPath != "/usr/local/bin/sync-hooks" {
		t.Fatalf("unexpected script path %s", cfg.Hooks.ScriptPath)
	}
	if len(cfg.Hooks.ScriptArgs) != 1 || cfg.Hooks.ScriptArgs[0] != "--from-env" {
		t.Fatalf("env override for script args not applied: %#v", cfg.Hooks.ScriptArgs)
	}
	if cfg.Hooks.Timeout != 30*time.Second {
		t.Fatalf("unexpected timeout %s", cfg.Hooks.Timeout)
	}
	if cfg.Hooks.Env["ENVSET"] != "1" || len(cfg.Hooks.Env) != 1 {
		t.Fatalf("unexpected env map %#v", cfg.Hooks.Env)
	}
	if !cfg.MarketplaceEnabled {
		t.Fatalf("marketplace should remain enabled when not overridden")
	}
	if cfg.AdminEmail != "admin@local" {
		t.Fatalf("unexpected admin email %s", cfg.AdminEmail)
	}
	if cfg.IdentityPath != DefaultIdentityPath() {
		t.Fatalf("unexpected identity path %s", cfg.IdentityPath)
	}
}

func TestLoadGatewayConfigHooksInvalidTimeout(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	setting := "environment=dev\n"
	if err := os.WriteFile(filepath.Join(tmp, "config", "setting.ini"), []byte(setting), 0o644); err != nil {
		t.Fatalf("write setting: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte("hooks_enabled=true\nhooks_script_path=/tmp/sync\nhooks_timeout=not-a-duration\n"), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}

	if _, err := LoadGatewayConfig(tmp); err == nil {
		t.Fatalf("expected error for invalid hooks timeout")
	}
}

func TestLoadGatewayConfigDefaults(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte(""), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}
	os.Unsetenv("TOKLIGENCE_MARKETPLACE_ENABLED") // Clear env var to test default behavior

	cfg, err := LoadGatewayConfig(tmp)
	if err != nil {
		t.Fatalf("LoadGatewayConfig: %v", err)
	}
	if cfg.Environment != "dev" {
		t.Fatalf("expected dev environment, got %s", cfg.Environment)
	}
	if cfg.PricePer1K != 0.5 {
		t.Fatalf("expected default price 0.5, got %v", cfg.PricePer1K)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("expected default log level info, got %s", cfg.LogLevel)
	}
	if cfg.FacadePort != 8081 {
		t.Fatalf("expected default facade port 8081, got %d", cfg.FacadePort)
	}
	defaultLedger := DefaultLedgerPath()
	if cfg.LedgerPath != defaultLedger {
		t.Fatalf("expected default ledger path %s, got %s", defaultLedger, cfg.LedgerPath)
	}
	if cfg.AuthSecret != "tokligence-dev-secret" {
		t.Fatalf("expected default auth secret, got %s", cfg.AuthSecret)
	}
	if cfg.BaseURL != DefaultExchangeBaseURL("dev") {
		t.Fatalf("expected default base url %s, got %s", DefaultExchangeBaseURL("dev"), cfg.BaseURL)
	}
	if !cfg.MarketplaceEnabled {
		t.Fatalf("expected marketplace enabled by default")
	}
	if cfg.AdminEmail != "admin@local" {
		t.Fatalf("expected default admin email, got %s", cfg.AdminEmail)
	}
	if cfg.IdentityPath != DefaultIdentityPath() {
		t.Fatalf("expected default identity path, got %s", cfg.IdentityPath)
	}
}

func TestLoadGatewayConfigInvalidPrice(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte("price_per_1k=not-a-number\n"), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}

	if _, err := LoadGatewayConfig(tmp); err == nil {
		t.Fatalf("expected error for invalid price")
	}
}

func TestLoadGatewayConfigExchangeDisabled(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte("marketplace_enabled=false\n"), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}

	cfg, err := LoadGatewayConfig(tmp)
	if err != nil {
		t.Fatalf("LoadGatewayConfig: %v", err)
	}
	if cfg.MarketplaceEnabled {
		t.Fatalf("expected marketplace disabled from ini")
	}

	os.Setenv("TOKLIGENCE_MARKETPLACE_ENABLED", "true")
	t.Cleanup(func() { os.Unsetenv("TOKLIGENCE_MARKETPLACE_ENABLED") })

	cfg, err = LoadGatewayConfig(tmp)
	if err != nil {
		t.Fatalf("LoadGatewayConfig: %v", err)
	}
	if !cfg.MarketplaceEnabled {
		t.Fatalf("env override should enable marketplace")
	}
}

func TestLoadGatewayConfigAdminOverrides(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config", "dev"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte("identity_path=/tmp/identity.db\n"), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}
	os.Setenv("TOKLIGENCE_ADMIN_EMAIL", "root@example.com")
	os.Setenv("TOKLIGENCE_IDENTITY_PATH", "/tmp/override.db")
	t.Cleanup(func() {
		os.Unsetenv("TOKLIGENCE_ADMIN_EMAIL")
		os.Unsetenv("TOKLIGENCE_IDENTITY_PATH")
	})

	cfg, err := LoadGatewayConfig(tmp)
	if err != nil {
		t.Fatalf("LoadGatewayConfig: %v", err)
	}
	if cfg.AdminEmail != "root@example.com" {
		t.Fatalf("expected admin email override, got %s", cfg.AdminEmail)
	}
	if cfg.IdentityPath != "/tmp/override.db" {
		t.Fatalf("expected identity path override, got %s", cfg.IdentityPath)
	}
}

func TestParseRoutes(t *testing.T) {
	cases := []struct {
		in   string
		want map[string]string
	}{
		{"gpt-*=>openai, claude*=>anthropic, loopback=>loopback", map[string]string{"gpt-*": "openai", "claude*": "anthropic", "loopback": "loopback"}},
		{"gpt-* = openai\nclaude-3-5-sonnet => anthropic", map[string]string{"gpt-*": "openai", "claude-3-5-sonnet": "anthropic"}},
		{"  ", nil},
	}
	for _, c := range cases {
		got := parseRoutes(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("len(routes)=%d want=%d for %q", len(got), len(c.want), c.in)
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Fatalf("route %q=%q want %q", k, got[k], v)
			}
		}
	}
}
