package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	adapteranthropic "github.com/tokligence/tokligence-gateway/internal/adapter/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
	adapteropenai "github.com/tokligence/tokligence-gateway/internal/adapter/openai"
	adapterrouter "github.com/tokligence/tokligence-gateway/internal/adapter/router"
	"github.com/tokligence/tokligence-gateway/internal/auth"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/config"
	"github.com/tokligence/tokligence-gateway/internal/core"
	"github.com/tokligence/tokligence-gateway/internal/hooks"
	"github.com/tokligence/tokligence-gateway/internal/httpserver"
	ledgersql "github.com/tokligence/tokligence-gateway/internal/ledger/sqlite"
	"github.com/tokligence/tokligence-gateway/internal/logging"
	"github.com/tokligence/tokligence-gateway/internal/modelmeta"
	"github.com/tokligence/tokligence-gateway/internal/telemetry"
	userstoresqlite "github.com/tokligence/tokligence-gateway/internal/userstore/sqlite"
)

var (
	buildVersion = "v0.1.0"
	buildCommit  = "unknown"
	buildBuiltAt = "unknown"
)

func main() {
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// Normalize log level tag for prefixes
	levelTag := strings.ToUpper(strings.TrimSpace(cfg.LogLevel))
	if levelTag == "" {
		levelTag = "INFO"
	}

	// Initialize rotating file logging (default enabled when log_file provided)
	const maxLogBytes = int64(300 * 1024 * 1024) // 300MB
	logTarget := strings.TrimSpace(cfg.LogFileDaemon)
	if logTarget != "" {
		rot, err := logging.NewRotatingWriter(logTarget, maxLogBytes)
		if err != nil {
			log.Fatalf("init rotating log: %v", err)
		}
		// Mirror to stdout as well for foreground runs (include level + file:line)
		log.SetOutput(io.MultiWriter(os.Stdout, rot))
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		log.SetPrefix("[gatewayd][" + levelTag + "] ")
		defer rot.Close()
	}

	log.Printf("Tokligence Gateway version=%s commit=%s built_at=%s (https://tokligence.ai)", buildVersion, buildCommit, buildBuiltAt)

	var marketplaceAPI core.MarketplaceAPI
	marketplaceEnabled := cfg.MarketplaceEnabled
	if marketplaceEnabled {
		marketplaceClient, err := client.NewMarketplaceClient(cfg.BaseURL, nil)
		if err != nil {
			log.Printf("Tokligence Marketplace unavailable (%v); running gatewayd in local-only mode", err)
			marketplaceEnabled = false
		} else {
			marketplaceClient.SetLogger(log.New(log.Writer(), "[gatewayd/http]["+levelTag+"] ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile))
			marketplaceAPI = marketplaceClient
		}
	} else {
		log.Printf("Tokligence Marketplace (https://tokligence.ai) disabled by configuration; running gatewayd in local-only mode")
	}

	gateway := core.NewGateway(marketplaceAPI)
	modelMeta := modelmeta.NewStore()
	modelMeta.StartAutoRefresh(modelmeta.LoaderConfig{
		LocalPath:      cfg.ModelMetadataFile,
		RemoteURL:      cfg.ModelMetadataURL,
		RefreshInterval: cfg.ModelMetadataRefresh,
	})

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

	var authManager *auth.Manager
	if !cfg.AuthDisabled {
		authManager = auth.NewManager(cfg.AuthSecret)
	} else {
		log.Printf("authorization disabled: skipping token validation")
	}
	// Build adapter routing: loopback + optional OpenAI/Anthropic based on config
	r := adapterrouter.New()
	// Always include loopback
	lb := loopback.New()
	_ = r.RegisterAdapter("loopback", lb)

	// Optional OpenAI
	var openaiRegistered, anthropicRegistered bool
	if strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
		oa, err := adapteropenai.New(adapteropenai.Config{
			APIKey:         cfg.OpenAIAPIKey,
			BaseURL:        cfg.OpenAIBaseURL,
			Organization:   cfg.OpenAIOrg,
			RequestTimeout: 60 * time.Second,
		})
		if err == nil {
			if err := r.RegisterAdapter("openai", oa); err == nil {
				openaiRegistered = true
			}
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
			if err := r.RegisterAdapter("anthropic", aa); err == nil {
				anthropicRegistered = true
			}
		} else {
			log.Printf("anthropic adapter init failed: %v", err)
		}
	}

	// Always include loopback route for internal diagnostics
	if err := r.RegisterRoute("loopback", "loopback"); err != nil {
		log.Printf("route rule %q=>%q rejected: %v", "loopback", "loopback", err)
	}
	// Legacy defaults for OpenAI/Anthropic; new model-first rules may override these
	if openaiRegistered {
		_ = r.RegisterRoute("gpt-*", "openai")
		_ = r.RegisterRoute("o*", "openai")
	}
	if anthropicRegistered {
		_ = r.RegisterRoute("claude*", "anthropic")
	} else if openaiRegistered {
		_ = r.RegisterRoute("claude*", "openai")
	}
	// Model-first provider routing (overrides defaults, order preserved from config)
	for _, rule := range cfg.ModelProviderRoutes {
		pattern := strings.TrimSpace(rule.Pattern)
		target := strings.ToLower(strings.TrimSpace(rule.Target))
		if pattern == "" || target == "" {
			continue
		}
		if err := r.RegisterRoute(pattern, target); err != nil {
			log.Printf("model provider route %q=>%q rejected: %v", pattern, target, err)
			if target == "anthropic" && !anthropicRegistered && openaiRegistered {
				if err := r.RegisterRoute(pattern, "openai"); err != nil {
					log.Printf("fallback route %q=>openai rejected: %v", pattern, err)
				} else {
					log.Printf("model provider route %q=>anthropic fell back to openai (anthropic unavailable)", pattern)
				}
			}
		}
	}
	// Explicit routes keep highest priority for advanced overrides
	for pattern, name := range cfg.Routes {
		if err := r.RegisterRoute(pattern, name); err != nil {
			log.Printf("route rule %q=>%q rejected: %v", pattern, name, err)
		}
	}
	// Log adapters and routes for diagnostics
	log.Printf("adapters registered: %v", r.ListAdapters())
	log.Printf("routes configured: %v", r.ListRoutes())
	// Register model aliases (optional): incoming model -> target provider model id
	if len(cfg.ModelAliases) > 0 {
		for pattern, target := range cfg.ModelAliases {
			if err := r.RegisterAlias(pattern, target); err != nil {
				log.Printf("alias rule %q=>%q rejected: %v", pattern, target, err)
			}
		}
	}
	// Fallback
	r.SetFallback(lb)

	httpSrv := httpserver.New(gateway, r, ledgerStore, authManager, identityStore, rootAdmin, hookDispatcher, cfg.AnthropicNativeEnabled)
	httpSrv.SetAuthDisabled(cfg.AuthDisabled)
	// Configure upstreams for native endpoint and bridges
	httpSrv.SetUpstreams(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.AnthropicAPIKey, cfg.AnthropicBaseURL, cfg.AnthropicVersion, cfg.OpenAIToolBridgeStreamEnabled, cfg.AnthropicForceSSE, cfg.AnthropicTokenCheckEnabled, cfg.AnthropicMaxTokens, cfg.OpenAICompletionMaxTokens, cfg.SidecarModelMap, modelMeta)
	// Configure global work mode (passthrough/translation/auto)
	httpSrv.SetWorkMode(cfg.WorkMode)
	var providerRules []httpserver.ModelProviderRule
	for _, rule := range cfg.ModelProviderRoutes {
		providerRules = append(providerRules, httpserver.ModelProviderRule{
			Pattern:  rule.Pattern,
			Provider: rule.Target,
		})
	}
	httpSrv.SetModelProviderRules(providerRules)
	httpSrv.SetDuplicateToolDetectionEnabled(cfg.DuplicateToolDetectionEnabled)
	httpSrv.SetModelMetadataResolver(modelMeta)
	log.Printf("work mode: %s (auto=smart routing, passthrough=delegation only, translation=translation only)", cfg.WorkMode)
	// Configure endpoint exposure per port
	httpSrv.SetEndpointConfig(cfg.FacadeEndpoints, cfg.OpenAIEndpoints, cfg.AnthropicEndpoints, cfg.AdminEndpoints)
	// Configure bridge session management for tool deduplication
	log.Printf("bridge session config: enabled=%v ttl=%s max_count=%d", cfg.BridgeSessionEnabled, cfg.BridgeSessionTTL, cfg.BridgeSessionMaxCount)
	if err := httpSrv.SetBridgeSessionConfig(cfg.BridgeSessionEnabled, cfg.BridgeSessionTTL, cfg.BridgeSessionMaxCount); err != nil {
		log.Printf("bridge session config error: %v", err)
	}
	// Pass logger and level to HTTP server for debug logs (include level tag)
	httpSrv.SetLogger(cfg.LogLevel, log.New(log.Writer(), "[gatewayd/http]["+levelTag+"] ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile))

	// Send anonymous telemetry ping if enabled
	if cfg.TelemetryEnabled {
		go sendTelemetryPing(cfg)
	}

	type namedServer struct {
		name string
		srv  *http.Server
	}
	var servers []namedServer

buildServer := func(name, addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       0,
		WriteTimeout:      0,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

	if cfg.MultiPortMode {
		if cfg.EnableFacade {
			addr := fmt.Sprintf(":%d", cfg.FacadePort)
			// In multi-port mode, use explicit facade_port, don't let http_address override it
			servers = append(servers, namedServer{"facade", buildServer("facade", addr, httpSrv.Router())})
		}
		if handler := httpSrv.RouterAdmin(); handler != nil && cfg.AdminPort != 0 {
			addr := fmt.Sprintf(":%d", cfg.AdminPort)
			servers = append(servers, namedServer{"admin", buildServer("admin", addr, handler)})
		}
		if handler := httpSrv.RouterOpenAI(); handler != nil && cfg.OpenAIPort != 0 {
			addr := fmt.Sprintf(":%d", cfg.OpenAIPort)
			servers = append(servers, namedServer{"openai", buildServer("openai", addr, handler)})
		}
		if handler := httpSrv.RouterAnthropic(); handler != nil && cfg.AnthropicPort != 0 {
			addr := fmt.Sprintf(":%d", cfg.AnthropicPort)
			servers = append(servers, namedServer{"anthropic", buildServer("anthropic", addr, handler)})
		}
	} else {
		// Single-port mode: all endpoints on facade_port (default 8081)
		addr := fmt.Sprintf(":%d", cfg.FacadePort)
		servers = append(servers, namedServer{"gateway", buildServer("gateway", addr, httpSrv.Router())})
	}

	for _, ns := range servers {
		srv := ns.srv
		go func(name string, server *http.Server) {
			log.Printf("%s server listening on %s", name, server.Addr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("%s server error: %v", name, err)
			}
		}(ns.name, srv)
	}

	// Hot-reload model aliases from optional files/dir without restart
	startAliasesHotReload(r, cfg)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, ns := range servers {
		if err := ns.srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("%s graceful shutdown failed: %v", ns.name, err)
		}
	}
}

// startAliasesHotReload periodically reloads model alias files/dir and updates the router
func startAliasesHotReload(r *adapterrouter.Router, cfg config.GatewayConfig) {
	base := make(map[string]string)
	for k, v := range cfg.ModelAliases {
		base[k] = v
	}
	current := make(map[string]string)
	merge := func(dst, src map[string]string) {
		for k, v := range src {
			dst[k] = v
		}
	}
	parseLines := func(s string) map[string]string {
		out := map[string]string{}
		for _, line := range strings.Split(s, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
				continue
			}
			// allow commas per line
			parts := strings.Split(line, ",")
			for _, p := range parts {
				e := strings.TrimSpace(p)
				if e == "" {
					continue
				}
				var kv []string
				if strings.Contains(e, "=>") {
					kv = strings.SplitN(e, "=>", 2)
				} else {
					kv = strings.SplitN(e, "=", 2)
				}
				if len(kv) != 2 {
					continue
				}
				k := strings.TrimSpace(kv[0])
				v := strings.TrimSpace(kv[1])
				if k != "" && v != "" {
					out[k] = v
				}
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	// verbosity for no-change scans
	verbose := false
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("TOKLIGENCE_MODEL_ALIASES_VERBOSE"))); v == "1" || v == "true" || v == "yes" {
		verbose = true
	}
	// warn-once state
	warnedFileMissing := false
	warnedDirMissing := false
	// read helpers with error visibility
	readFile := func(p string) (string, error) {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	readDir := func(dir string) (string, error) {
		var sb strings.Builder
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			if !e.Type().IsRegular() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			fp := dir + string(os.PathSeparator) + name
			if s, err := readFile(fp); err == nil {
				sb.WriteString(s)
				sb.WriteString("\n")
			}
		}
		return sb.String(), nil
	}
	equal := func(a, b map[string]string) bool {
		if len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if b[k] != v {
				return false
			}
		}
		return true
	}
	// initial apply combines base + file/dir
	build := func() map[string]string {
		combined := make(map[string]string)
		merge(combined, base)
		if strings.TrimSpace(cfg.ModelAliasesFile) != "" {
			if s, err := readFile(cfg.ModelAliasesFile); err == nil {
				if strings.TrimSpace(s) != "" {
					merge(combined, parseLines(s))
				}
			} else if os.IsNotExist(err) {
				if !warnedFileMissing {
					log.Printf("model aliases file not found: %s", cfg.ModelAliasesFile)
					warnedFileMissing = true
				}
			} else {
				log.Printf("model aliases read file error: %s: %v", cfg.ModelAliasesFile, err)
			}
		}
		if strings.TrimSpace(cfg.ModelAliasesDir) != "" {
			if s, err := readDir(cfg.ModelAliasesDir); err == nil {
				if strings.TrimSpace(s) != "" {
					merge(combined, parseLines(s))
				}
			} else if os.IsNotExist(err) {
				if !warnedDirMissing {
					log.Printf("model aliases dir not found: %s", cfg.ModelAliasesDir)
					warnedDirMissing = true
				}
			} else {
				log.Printf("model aliases read dir error: %s: %v", cfg.ModelAliasesDir, err)
			}
		}
		return combined
	}
	apply := func(m map[string]string) {
		r.SetAliases(m)
		log.Printf("model aliases reloaded: %v", r.ListAliases())
	}
	// interval default 5s; override via TOKLIGENCE_MODEL_ALIASES_RELOAD_SEC
	sec := 5
	if v := strings.TrimSpace(os.Getenv("TOKLIGENCE_MODEL_ALIASES_RELOAD_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sec = n
		}
	}
	log.Printf("model aliases hot-reload enabled file=%q dir=%q interval=%ds base_count=%d", cfg.ModelAliasesFile, cfg.ModelAliasesDir, sec, len(base))
	initial := build()
	if !equal(current, initial) {
		current = initial
		apply(current)
	} else if verbose {
		log.Printf("model aliases unchanged (initial)")
	}
	ticker := time.NewTicker(time.Duration(sec) * time.Second)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			next := build()
			if !equal(current, next) {
				current = next
				apply(current)
			} else if verbose {
				log.Printf("model aliases scan: no changes")
			}
		}
	}()
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
