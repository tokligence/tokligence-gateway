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
)

func main() {
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	var exchangeAPI core.MarketplaceAPI
	exchangeEnabled := cfg.ExchangeEnabled
	if exchangeEnabled {
		exchangeClient, err := client.NewMarketplaceClient(cfg.BaseURL, nil)
		if err != nil {
			log.Printf("Tokligence Marketplace unavailable (%v); running gatewayd in local-only mode", err)
			exchangeEnabled = false
		} else {
			exchangeClient.SetLogger(log.New(log.Writer(), "[gatewayd/http] ", log.LstdFlags|log.Lmicroseconds))
			exchangeAPI = exchangeClient
		}
	} else {
		log.Printf("Tokligence Marketplace (https://tokligence.ai) disabled by configuration; running gatewayd in local-only mode")
	}

	gateway := core.NewGateway(exchangeAPI)

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
	if exchangeEnabled {
		if _, _, err := gateway.EnsureAccount(ctx, cfg.Email, roles, cfg.DisplayName); err != nil {
			if errors.Is(err, core.ErrMarketplaceUnavailable) {
				log.Printf("Tokligence Marketplace unavailable; continuing without marketplace integration")
			} else {
				log.Printf("ensure account failed: %v", err)
			}
			exchangeEnabled = false
		}
	}
	if !exchangeEnabled {
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
	httpSrv := httpserver.New(gateway, loopback.New(), ledgerStore, authManager, identityStore, rootAdmin, hookDispatcher)

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
		log.Printf("telemetry: failed to get install_id: %v", err)
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
		TotalAPICalls:  0, // TODO: Add API call counter
		UniqueModels:   0, // TODO: Add unique model tracker
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("Tokligence Gateway v%s (https://tokligence.ai)", gatewayVersion)
	log.Printf("Installation ID: %s", installID)
	log.Printf("Anonymous telemetry enabled (disable: TOKLIGENCE_TELEMETRY_ENABLED=false)")

	if err := telemetryClient.SendPing(ctx, payload); err != nil {
		log.Printf("telemetry ping failed (non-fatal): %v", err)
	}
}
