package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesConfigFiles(t *testing.T) {
	tmp := t.TempDir()
	opts := InitOptions{
		Root:        tmp,
		Email:       "agent@example.com",
		DisplayName: "Agent",
		BaseURL:     "https://exchange.example.com",
	}
	if err := Init(opts); err != nil {
		t.Fatalf("Init: %v", err)
	}

	settingBytes, err := os.ReadFile(filepath.Join(tmp, "config", "setting.ini"))
	if err != nil {
		t.Fatalf("read setting: %v", err)
	}
	content := string(settingBytes)
	if !strings.Contains(content, "environment=dev") {
		t.Fatalf("missing environment: %s", content)
	}
	if !strings.Contains(content, "email=agent@example.com") {
		t.Fatalf("missing email: %s", content)
	}

	gatewayBytes, err := os.ReadFile(filepath.Join(tmp, "config", "dev", "gateway.ini"))
	if err != nil {
		t.Fatalf("read gateway: %v", err)
	}
	gatewayContent := string(gatewayBytes)
	if !strings.Contains(gatewayContent, "base_url=https://exchange.example.com") {
		t.Fatalf("missing base url: %s", gatewayContent)
	}
	if !strings.Contains(gatewayContent, "price_per_1k=0.5000") {
		t.Fatalf("unexpected price: %s", gatewayContent)
	}
}

func TestInitRespectsForce(t *testing.T) {
	tmp := t.TempDir()
	opts := InitOptions{Root: tmp, Email: "a@b.com"}
	if err := Init(opts); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := Init(opts); err == nil {
		t.Fatalf("expected error when files exist")
	}
	opts.Force = true
	if err := Init(opts); err != nil {
		t.Fatalf("Init with force: %v", err)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(InitOptions{Email: "invalid"}); err == nil {
		t.Fatalf("expected invalid email error")
	}
	if err := Validate(InitOptions{Email: "valid@example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
