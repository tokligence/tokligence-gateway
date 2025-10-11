package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/config"
	"github.com/tokligence/tokligence-gateway/internal/core"
)

func main() {
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	logOutput := io.Writer(os.Stdout)
	var file *os.File
	if cfg.LogFile != "" {
		logPath := cfg.LogFile
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(".", logPath)
		}
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			log.Fatalf("create log directory: %v", err)
		}
		file, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("open log file: %v", err)
		}
		defer file.Close()
		logOutput = io.MultiWriter(os.Stdout, file)
	}

	levelTag := strings.ToUpper(cfg.LogLevel)
	rootLogger := log.New(logOutput, fmt.Sprintf("[mfg/main][%s][%s] ", cfg.Environment, levelTag), log.LstdFlags|log.Lmicroseconds)

	baseURL := stringFromEnv("TOKEN_EXCHANGE_BASE_URL", cfg.BaseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	rootLogger.Printf("starting Model-Free Gateway CLI base_url=%s", baseURL)

	email := stringFromEnv("MFG_EMAIL", cfg.Email)
	if email == "" {
		rootLogger.Fatal("missing email configuration (MFG_EMAIL or config)")
	}
	displayName := stringFromEnv("MFG_DISPLAY_NAME", cfg.DisplayName)
	enableProvider := boolFromEnv("MFG_ENABLE_PROVIDER", cfg.EnableProvider)
	publishName := stringFromEnv("MFG_PUBLISH_NAME", cfg.PublishName)
	modelFamily := stringFromEnv("MFG_MODEL_FAMILY", cfg.ModelFamily)

	exchangeClient, err := client.NewExchangeClient(baseURL, nil)
	if err != nil {
		rootLogger.Fatalf("failed to create client: %v", err)
	}
	exchangeClient.SetLogger(log.New(logOutput, fmt.Sprintf("[mfg/http][%s][%s] ", cfg.Environment, levelTag), log.LstdFlags|log.Lmicroseconds))

	gateway := core.NewGateway(exchangeClient)
	gateway.SetLogger(log.New(logOutput, fmt.Sprintf("[mfg/gateway][%s][%s] ", cfg.Environment, levelTag), log.LstdFlags|log.Lmicroseconds))

	ctx := context.Background()

	roles := []string{"consumer"}
	if enableProvider {
		roles = append(roles, "provider")
	}
	rootLogger.Printf("ensuring account email=%s roles=%v", email, roles)

	user, provider, err := gateway.EnsureAccount(ctx, email, roles, displayName)
	if err != nil {
		rootLogger.Fatalf("ensure account failed: %v", err)
	}

	rootLogger.Printf("connected as user id=%d email=%s roles=%v", user.ID, user.Email, user.Roles)
	if provider != nil && publishName != "" {
		rootLogger.Printf("publishing service name=%s model=%s price_per_1k=%.4f", publishName, modelFamily, cfg.PricePer1K)
		svc, err := gateway.PublishService(ctx, client.PublishServiceRequest{
			Name:             publishName,
			ModelFamily:      modelFamily,
			PricePer1KTokens: cfg.PricePer1K,
		})
		if err != nil {
			rootLogger.Fatalf("publish service failed: %v", err)
		}
		rootLogger.Printf("published service id=%d name=%s", svc.ID, svc.Name)
	}

	summary, err := gateway.UsageSnapshot(ctx)
	if err != nil {
		rootLogger.Printf("usage snapshot unavailable: %v", err)
	} else {
		rootLogger.Printf("usage summary consumed=%d supplied=%d net=%d", summary.ConsumedTokens, summary.SuppliedTokens, summary.NetTokens)
	}
}

func stringFromEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func boolFromEnv(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
