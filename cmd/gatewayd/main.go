package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/config"
	"github.com/tokligence/tokligence-gateway/internal/core"
	"github.com/tokligence/tokligence-gateway/internal/httpserver"
	ledgersql "github.com/tokligence/tokligence-gateway/internal/ledger/sqlite"
)

func main() {
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	exchangeClient, err := client.NewExchangeClient(cfg.BaseURL, nil)
	if err != nil {
		log.Fatalf("create exchange client: %v", err)
	}
	gateway := core.NewGateway(exchangeClient)

	ctx := context.Background()
	roles := []string{"consumer"}
	if cfg.EnableProvider {
		roles = append(roles, "provider")
	}
	if _, _, err := gateway.EnsureAccount(ctx, cfg.Email, roles, cfg.DisplayName); err != nil {
		log.Fatalf("ensure account: %v", err)
	}

	ledgerStore, err := ledgersql.New(cfg.LedgerPath)
	if err != nil {
		log.Fatalf("open ledger: %v", err)
	}
	defer ledgerStore.Close()

	httpSrv := httpserver.New(gateway, loopback.New(), ledgerStore)

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
