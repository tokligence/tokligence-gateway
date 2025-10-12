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
	content := "base_url=http://example.com\ndisplay_name=Test\nenable_provider=true\nprice_per_1k=1.25\nlog_file=/tmp/env.log\nhttp_address=:9090\nledger_path=/tmp/custom-ledger.db\nauth_secret=override-secret\n"
	if err := os.WriteFile(filepath.Join(tmp, "config", "dev", "gateway.ini"), []byte(content), 0o644); err != nil {
		t.Fatalf("write env config: %v", err)
	}
	os.Setenv("TOKLIGENCE_AUTH_SECRET", "env-secret")
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
	if cfg.HTTPAddress != ":9090" {
		t.Fatalf("unexpected http address %s", cfg.HTTPAddress)
	}
	if cfg.LedgerPath != "/tmp/custom-ledger.db" {
		t.Fatalf("unexpected ledger path %s", cfg.LedgerPath)
	}
	if cfg.AuthSecret != "env-secret" {
		t.Fatalf("unexpected auth secret %s", cfg.AuthSecret)
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
	if cfg.HTTPAddress != ":8081" {
		t.Fatalf("expected default http address :8081, got %s", cfg.HTTPAddress)
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
