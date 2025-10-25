package main

import (
    "context"
    "errors"
    "log"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    adapteropenai "github.com/tokligence/tokligence-gateway/internal/adapter/openai"
    adapterrouter "github.com/tokligence/tokligence-gateway/internal/adapter/router"
    "github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
    "github.com/tokligence/tokligence-gateway/internal/auth"
    "github.com/tokligence/tokligence-gateway/internal/client"
    "github.com/tokligence/tokligence-gateway/internal/config"
    "github.com/tokligence/tokligence-gateway/internal/core"
    "github.com/tokligence/tokligence-gateway/internal/hooks"
    "github.com/tokligence/tokligence-gateway/internal/httpserver"
    ledgersql "github.com/tokligence/tokligence-gateway/internal/ledger/sqlite"
    "github.com/tokligence/tokligence-gateway/internal/telemetry"
    userstoresqlite "github.com/tokligence/tokligence-gateway/internal/userstore/sqlite"
    adapteranthropic "github.com/tokligence/tokligence-gateway/internal/adapter/anthropic"
)

func main() {
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	var marketplaceAPI core.MarketplaceAPI
	marketplaceEnabled := cfg.MarketplaceEnabled
	if marketplaceEnabled {
		marketplaceClient, err := client.NewMarketplaceClient(cfg.BaseURL, nil)
		if err != nil {
			log.Printf("Tokligence Marketplace unavailable (%v); running gatewayd in local-only mode", err)
			marketplaceEnabled = false
		} else {
			marketplaceClient.SetLogger(log.New(log.Writer(), "[gatewayd/http] ", log.LstdFlags|log.Lmicroseconds))
			marketplaceAPI = marketplaceClient
		}
	} else {
		log.Printf("Tokligence Marketplace (https://tokligence.ai) disabled by configuration; running gatewayd in local-only mode")
	}

	gateway := core.NewGateway(marketplaceAPI)

	ctx := context.Background()
	identityStore, err := userstoresqlite.New(cfg.IdentityPath)
	if err != nil {
		log.Fatalf("open identity store: %v", err)
	}
	defer identityStore.Close()

	rootAdmin, err := identityStore.EnsureRootAdmin(ctx, cfg.AdminEmail)
	if err != nil {
		log.Fatalf("ensure root admin: %v", err)
	}

	var hookDispatcher *hooks.Dispatcher
	if handler := cfg.Hooks.BuildScriptHandler(); handler != nil {
		hookDispatcher = &hooks.Dispatcher{}
		hookDispatcher.Register(handler)
		log.Printf("hooks dispatcher enabled script=%s", cfg.Hooks.ScriptPath)
	}

	roles := []string{"consumer"}
	if cfg.EnableProvider {
		roles = append(roles, "provider")
	}
	if marketplaceEnabled {
		if _, _, err := gateway.EnsureAccount(ctx, cfg.Email, roles, cfg.DisplayName); err != nil {
			if errors.Is(err, core.ErrMarketplaceUnavailable) {
				log.Printf("Tokligence Marketplace unavailable; continuing without marketplace integration")
			} else {
				log.Printf("ensure account failed: %v", err)
			}
			marketplaceEnabled = false
		}
	}
	if !marketplaceEnabled {
		gateway.SetMarketplaceAvailable(false)
		localRoles := append([]string{"root_admin"}, roles...)
		localUser := client.User{ID: rootAdmin.ID, Email: rootAdmin.Email, Roles: localRoles}
		gateway.SetLocalAccount(localUser, nil)
		log.Printf("gatewayd running in local-only mode email=%s roles=%v", localUser.Email, localRoles)
	}
	if hookDispatcher != nil {
		gateway.SetHooksDispatcher(hookDispatcher)
	}

	ledgerStore, err := ledgersql.New(cfg.LedgerPath)
	if err != nil {
		log.Fatalf("open ledger: %v", err)
	}
	defer ledgerStore.Close()

	authManager := auth.NewManager(cfg.AuthSecret)
    // Build adapter routing: loopback + optional OpenAI/Anthropic based on config
    r := adapterrouter.New()
    // Always include loopback
    lb := loopback.New()
    _ = r.RegisterAdapter("loopback", lb)

    // Optional OpenAI
    if strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
        oa, err := adapteropenai.New(adapteropenai.Config{
            APIKey:         cfg.OpenAIAPIKey,
            BaseURL:        cfg.OpenAIBaseURL,
            Organization:   cfg.OpenAIOrg,
            RequestTimeout: 60 * time.Second,
        })
        if err == nil {
            _ = r.RegisterAdapter("openai", oa)
        } else {
            log.Printf("openai adapter init failed: %v", err)
        }
    }

    // Optional Anthropic
    if strings.TrimSpace(cfg.AnthropicAPIKey) != "" {
        aa, err := adapteranthropic.New(adapteranthropic.Config{
            APIKey:         cfg.AnthropicAPIKey,
            BaseURL:        cfg.AnthropicBaseURL,
            Version:        cfg.AnthropicVersion,
            RequestTimeout: 60 * time.Second,
        })
        if err == nil {
            _ = r.RegisterAdapter("anthropic", aa)
        } else {
            log.Printf("anthropic adapter init failed: %v", err)
        }
    }

    // Register routing rules from config
    if len(cfg.Routes) > 0 {
        for pattern, name := range cfg.Routes {
            if err := r.RegisterRoute(pattern, name); err != nil {
                log.Printf("route rule %q=>%q rejected: %v", pattern, name, err)
            }
        }
    } else {
        // Default sensible routes if none configured
        // Route claude* to anthropic, gpt* to openai, loopback to loopback
        _ = r.RegisterRoute("loopback", "loopback")
        _ = r.RegisterRoute("gpt-*", "openai")
        _ = r.RegisterRoute("claude*", "anthropic")
    }
    // Fallback
    r.SetFallback(lb)

    httpSrv := httpserver.New(gateway, r, ledgerStore, authManager, identityStore, rootAdmin, hookDispatcher)

	// Send anonymous telemetry ping if enabled
	if cfg.TelemetryEnabled {
		go sendTelemetryPing(cfg)
	}

	srv := &http.Server{
		Addr:         cfg.HTTPAddress,
		Handler:      httpSrv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("gateway server listening on %s", cfg.HTTPAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func sendTelemetryPing(cfg config.GatewayConfig) {
	const gatewayVersion = "0.1.0" // TODO: get from build flag

	installID, err := telemetry.GetOrCreateInstallID("")
	if err != nil {
		log.Printf("marketplace communication: failed to get install_id: %v", err)
		return
	}

	// Detect database type from IdentityPath
	dbType := "sqlite"
	if strings.HasPrefix(cfg.IdentityPath, "postgres://") || strings.HasPrefix(cfg.IdentityPath, "postgresql://") {
		dbType = "postgres"
	}

	telemetryClient := telemetry.NewClient(cfg.BaseURL, nil)
	payload := telemetry.PingPayload{
		InstallID:      installID,
		GatewayVersion: gatewayVersion,
		Platform:       "", // Will be auto-filled by client
		DatabaseType:   dbType,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Tokligence Gateway v%s (https://tokligence.ai)", gatewayVersion)
	log.Printf("Installation ID: %s", installID)

	if cfg.MarketplaceEnabled {
		log.Printf("Marketplace communication enabled (disable: TOKLIGENCE_MARKETPLACE_ENABLED=false)")
		log.Printf("  - Version update checks")
		log.Printf("  - Promotional announcements")
	} else {
		log.Printf("Running in local-only mode (marketplace disabled)")
		return
	}

	// Send ping and process response
	_, err = telemetryClient.SendPing(ctx, payload)
	if err != nil {
		log.Printf("marketplace ping failed (non-fatal, will retry in 24h): %v", err)
		return
	}

	// Response logging is handled by the client
	// Schedule next ping in 24 hours
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := telemetryClient.SendPing(ctx, payload)
			cancel()

			if err != nil {
				log.Printf("scheduled marketplace ping failed (non-fatal): %v", err)
			}
		}
	}()
}
