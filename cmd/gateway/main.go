package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/bootstrap"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/config"
	"github.com/tokligence/tokligence-gateway/internal/core"
	"github.com/tokligence/tokligence-gateway/internal/hooks"
	"github.com/tokligence/tokligence-gateway/internal/logging"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
	userstoresqlite "github.com/tokligence/tokligence-gateway/internal/userstore/sqlite"
)

var (
	buildVersion = "v0.1.0"
	buildCommit  = "unknown"
	buildBuiltAt = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			if err := runInit(os.Args[2:]); err != nil {
				log.Fatalf("gateway init failed: %v", err)
			}
			fmt.Println("gateway config initialised")
			return
		case "admin":
			if err := runAdmin(os.Args[2:]); err != nil {
				log.Fatalf("gateway admin failed: %v", err)
			}
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	runGateway()
}

func printUsage() {
	fmt.Print(`Tokligence Gateway CLI

Usage:
  gateway init [flags]      Generate config/setting.ini and environment overrides
  gateway                   Ensure account and publish configured services
  gateway admin ...         Manage local users and API keys

Flags for init:
  --root string            output directory (default '.')
  --env string             environment name (default 'dev')
  --email string           account email (default 'dev@example.com')
  --display-name string    display name for the account
  --base-url string        token marketplace base URL (default 'http://localhost:8080')
  --provider               enable provider role in settings
  --http-address string    bind address for gatewayd (default ':8081')
  --ledger-path string     ledger SQLite path (default ~/.tokligence/ledger.db)
  --publish-name string    default published service name
  --model-family string    default model family name
  --price float            price per 1K tokens (default 0.5)
  --force                  overwrite existing files
`)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "config root")
	env := fs.String("env", "dev", "environment name")
	email := fs.String("email", "dev@example.com", "account email")
	display := fs.String("display-name", "Tokligence Gateway", "display name")
	baseURL := fs.String("base-url", "http://localhost:8080", "token marketplace base URL")
	provider := fs.Bool("provider", false, "enable provider role")
	ledgerPath := fs.String("ledger-path", "", "ledger sqlite path")
	publishName := fs.String("publish-name", "local-loopback", "default service name")
	modelFamily := fs.String("model-family", "claude-3.5-sonnet", "default model family")
	price := fs.Float64("price", 0.5, "price per 1K tokens")
	force := fs.Bool("force", false, "overwrite existing files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := bootstrap.InitOptions{
		Root:           *root,
		Environment:    *env,
		Email:          *email,
		DisplayName:    *display,
		BaseURL:        *baseURL,
		EnableProvider: *provider,
		LedgerPath:     *ledgerPath,
		PublishName:    *publishName,
		ModelFamily:    *modelFamily,
		PricePer1K:     *price,
		Force:          *force,
	}
	if err := bootstrap.Validate(opts); err != nil {
		return err
	}
	return bootstrap.Init(opts)
}

func runGateway() {
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	const maxLogBytes = int64(300 * 1024 * 1024) // 300MB
	logOutput := io.Writer(os.Stdout)
	var rotCloser io.Closer
	logTarget := strings.TrimSpace(cfg.LogFileCLI)
	if logTarget != "" {
		rot, err := logging.NewRotatingWriter(logTarget, maxLogBytes)
		if err != nil {
			log.Fatalf("init rotating log: %v", err)
		}
		rotCloser = rot
		logOutput = io.MultiWriter(os.Stdout, rot)
	}

	levelTag := strings.ToUpper(cfg.LogLevel)
	rootLogger := log.New(logOutput, fmt.Sprintf("[gateway/main][%s][%s] ", cfg.Environment, levelTag), log.LstdFlags|log.Lmicroseconds)
	rootLogger.Printf("Tokligence Gateway CLI version=%s commit=%s built_at=%s", buildVersion, buildCommit, buildBuiltAt)

	baseURL := stringFromEnv("TOKEN_EXCHANGE_BASE_URL", cfg.BaseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	rootLogger.Printf("starting Tokligence Gateway CLI base_url=%s", baseURL)

	hookDispatcher := setupHookDispatcher(cfg, rootLogger)

	email := stringFromEnv("TOKLIGENCE_EMAIL", cfg.Email)
	if email == "" {
		rootLogger.Fatal("missing email configuration (TOKLIGENCE_EMAIL or config)")
	}
	displayName := stringFromEnv("TOKLIGENCE_DISPLAY_NAME", cfg.DisplayName)
	enableProvider := boolFromEnv("TOKLIGENCE_ENABLE_PROVIDER", cfg.EnableProvider)
	publishName := stringFromEnv("TOKLIGENCE_PUBLISH_NAME", cfg.PublishName)
	modelFamily := stringFromEnv("TOKLIGENCE_MODEL_FAMILY", cfg.ModelFamily)

	ctx := context.Background()

	identityStore, err := userstoresqlite.New(cfg.IdentityPath)
	if err != nil {
		rootLogger.Fatalf("open identity store failed: %v", err)
	}
	defer identityStore.Close()

	rootAdmin, err := identityStore.EnsureRootAdmin(ctx, cfg.AdminEmail)
	if err != nil {
		rootLogger.Fatalf("ensure root admin failed: %v", err)
	}

	var marketplaceAPI core.MarketplaceAPI
	marketplaceEnabled := cfg.MarketplaceEnabled
	var marketplaceClient *client.MarketplaceClient
	if marketplaceEnabled {
		marketplaceClient, err = client.NewMarketplaceClient(baseURL, nil)
		if err != nil {
			rootLogger.Printf("Tokligence Marketplace unavailable (%v); running in local-only mode", err)
			marketplaceEnabled = false
		} else {
			marketplaceClient.SetLogger(log.New(logOutput, fmt.Sprintf("[gateway/http][%s][%s] ", cfg.Environment, levelTag), log.LstdFlags|log.Lmicroseconds))
			marketplaceAPI = marketplaceClient
		}
	} else {
		rootLogger.Printf("Tokligence Marketplace (https://tokligence.ai) disabled by configuration; running in local-only mode")
	}

	gateway := core.NewGateway(marketplaceAPI)
	gateway.SetLogger(log.New(logOutput, fmt.Sprintf("[gateway/core][%s][%s] ", cfg.Environment, levelTag), log.LstdFlags|log.Lmicroseconds))
	if hookDispatcher != nil {
		gateway.SetHooksDispatcher(hookDispatcher)
	}

	roles := []string{"consumer"}
	if enableProvider {
		roles = append(roles, "provider")
	}
	var (
		user     *client.User
		provider *client.ProviderProfile
	)

	if marketplaceEnabled {
		rootLogger.Printf("ensuring account email=%s roles=%v", email, roles)
		var ensureErr error
		user, provider, ensureErr = gateway.EnsureAccount(ctx, email, roles, displayName)
		if ensureErr != nil {
			if errors.Is(ensureErr, core.ErrMarketplaceUnavailable) {
				rootLogger.Printf("Tokligence Marketplace unavailable; skipping remote account provisioning")
			} else {
				rootLogger.Printf("failed to ensure marketplace account: %v", ensureErr)
			}
			marketplaceEnabled = false
		}
	}

	if !marketplaceEnabled {
		gateway.SetMarketplaceAvailable(false)
		localRoles := append([]string{"root_admin"}, roles...)
		localUser := client.User{ID: rootAdmin.ID, Email: rootAdmin.Email, Roles: localRoles}
		gateway.SetLocalAccount(localUser, nil)
		user = &localUser
		provider = nil
		rootLogger.Printf("running in local-only mode email=%s roles=%v", localUser.Email, localRoles)
	} else {
		rootLogger.Printf("connected as user id=%d email=%s roles=%v", user.ID, user.Email, user.Roles)
	}

	if marketplaceEnabled && provider != nil && publishName != "" {
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

	if marketplaceEnabled {
		summary, err := gateway.UsageSnapshot(ctx)
		if err != nil {
			rootLogger.Printf("usage snapshot unavailable: %v", err)
		} else {
			rootLogger.Printf("usage summary consumed=%d supplied=%d net=%d", summary.ConsumedTokens, summary.SuppliedTokens, summary.NetTokens)
		}
	} else {
		rootLogger.Printf("usage snapshot skipped: Tokligence Marketplace offline")
	}

	if rotCloser != nil {
		defer rotCloser.Close()
	}
}

func runAdmin(args []string) error {
	if len(args) == 0 {
		printAdminUsage()
		return nil
	}
	cfg, err := config.LoadGatewayConfig(".")
	if err != nil {
		return err
	}
	logger := log.New(os.Stdout, "[gateway/admin] ", log.LstdFlags)
	hookDispatcher := setupHookDispatcher(cfg, logger)
	store, err := userstoresqlite.New(cfg.IdentityPath)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx := context.Background()
	switch args[0] {
	case "users":
		return runAdminUsers(ctx, store, hookDispatcher, args[1:], logger)
	case "api-keys":
		return runAdminAPIKeys(ctx, store, args[1:], logger)
	case "help", "--help", "-h":
		printAdminUsage()
		return nil
	default:
		printAdminUsage()
		return fmt.Errorf("unknown admin subcommand %q", args[0])
	}
}

func runAdminUsers(ctx context.Context, store userstore.Store, dispatcher *hooks.Dispatcher, args []string, logger *log.Logger) error {
	if len(args) == 0 {
		printAdminUsersUsage()
		return nil
	}
	switch args[0] {
	case "list":
		users, err := store.ListUsers(ctx)
		if err != nil {
			return err
		}
		for _, u := range users {
			fmt.Printf("%4d  %-25s %-14s %-8s %s\n", u.ID, u.Email, u.Role, u.Status, u.DisplayName)
		}
		return nil
	case "create":
		fs := flag.NewFlagSet("gateway admin users create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		email := fs.String("email", "", "email address")
		role := fs.String("role", string(userstore.RoleGatewayUser), "role (gateway_admin|gateway_user)")
		name := fs.String("name", "", "display name")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		user, err := store.CreateUser(ctx, *email, userstore.Role(strings.TrimSpace(*role)), strings.TrimSpace(*name))
		if err != nil {
			return err
		}
		emitUserEvent(ctx, dispatcher, hooks.EventUserProvisioned, user)
		fmt.Printf("Created user id=%d email=%s role=%s\n", user.ID, user.Email, user.Role)
		return nil
	case "update":
		fs := flag.NewFlagSet("gateway admin users update", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		id := fs.Int64("id", 0, "user id")
		role := fs.String("role", "", "new role")
		name := fs.String("name", "", "display name")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		user, err := store.GetUser(ctx, *id)
		if err != nil || user == nil {
			return fmt.Errorf("user %d not found", *id)
		}
		updatedRole := user.Role
		if strings.TrimSpace(*role) != "" {
			updatedRole = userstore.Role(strings.TrimSpace(*role))
		}
		updated, err := store.UpdateUser(ctx, *id, strings.TrimSpace(*name), updatedRole)
		if err != nil {
			return err
		}
		emitUserEvent(ctx, dispatcher, hooks.EventUserUpdated, updated)
		fmt.Printf("Updated user id=%d role=%s display=%s\n", updated.ID, updated.Role, updated.DisplayName)
		return nil
	case "status":
		fs := flag.NewFlagSet("gateway admin users status", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		id := fs.Int64("id", 0, "user id")
		status := fs.String("status", string(userstore.StatusActive), "status (active|inactive)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := store.SetUserStatus(ctx, *id, userstore.Status(strings.TrimSpace(*status))); err != nil {
			return err
		}
		user, err := store.GetUser(ctx, *id)
		if err == nil && user != nil {
			emitUserEvent(ctx, dispatcher, hooks.EventUserUpdated, user)
		}
		fmt.Printf("Updated user %d status=%s\n", *id, strings.TrimSpace(*status))
		return nil
	case "delete":
		fs := flag.NewFlagSet("gateway admin users delete", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		id := fs.Int64("id", 0, "user id")
		hard := fs.Bool("hard", false, "permanently delete (default is soft delete)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		user, _ := store.GetUser(ctx, *id)
		if *hard {
			// Check if the store supports hard delete
			if hardDeleter, ok := store.(interface {
				HardDeleteUser(context.Context, int64) error
			}); ok {
				if err := hardDeleter.HardDeleteUser(ctx, *id); err != nil {
					return err
				}
				fmt.Printf("Permanently deleted user %d\n", *id)
			} else {
				return fmt.Errorf("hard delete not supported by this store implementation")
			}
		} else {
			if err := store.DeleteUser(ctx, *id); err != nil {
				return err
			}
			fmt.Printf("Soft deleted user %d (marked as deleted)\n", *id)
		}
		emitUserEvent(ctx, dispatcher, hooks.EventUserDeleted, user)
		return nil
	case "import":
		fs := flag.NewFlagSet("gateway admin users import", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		path := fs.String("file", "", "CSV file containing users")
		skipExisting := fs.Bool("skip-existing", false, "skip users that already exist")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if strings.TrimSpace(*path) == "" {
			return errors.New("--file is required")
		}
		file, err := os.Open(*path)
		if err != nil {
			return fmt.Errorf("open csv: %w", err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.TrimLeadingSpace = true
		records, err := reader.ReadAll()
		if err != nil {
			return fmt.Errorf("parse csv: %w", err)
		}
		if len(records) == 0 {
			fmt.Println("no users to import")
			return nil
		}
		start := 0
		emailIdx, roleIdx, displayIdx, nameIdx := 0, 1, 2, -1
		if isHeader(records[0]) {
			header := normalizeHeader(records[0])
			emailIdx = indexOf(header, "email")
			roleIdx = indexOf(header, "role")
			displayIdx = indexOf(header, "display_name")
			nameIdx = indexOf(header, "name")
			start = 1
		}
		if emailIdx < 0 {
			return errors.New("csv missing email column")
		}
		created := 0
		skipped := 0
		for i := start; i < len(records); i++ {
			row := records[i]
			email := valueAt(row, emailIdx)
			role := ""
			if roleIdx >= 0 {
				role = valueAt(row, roleIdx)
			}
			if role == "" {
				role = string(userstore.RoleGatewayUser)
			}
			display := ""
			if displayIdx >= 0 {
				display = valueAt(row, displayIdx)
			}
			if display == "" && nameIdx >= 0 {
				display = valueAt(row, nameIdx)
			}
			if strings.TrimSpace(email) == "" {
				fmt.Printf("row %d skipped: missing email\n", i+1)
				skipped++
				continue
			}
			user, err := store.CreateUser(ctx, email, userstore.Role(strings.TrimSpace(role)), strings.TrimSpace(display))
			if err != nil {
				if isDuplicateUserError(err) && *skipExisting {
					fmt.Printf("row %d skipped: %v\n", i+1, err)
					skipped++
					continue
				}
				return fmt.Errorf("row %d: %w", i+1, err)
			}
			emitUserEvent(ctx, dispatcher, hooks.EventUserProvisioned, user)
			fmt.Printf("row %d created user id=%d email=%s\n", i+1, user.ID, user.Email)
			created++
		}
		fmt.Printf("import complete: created=%d skipped=%d\n", created, skipped)
		return nil
	default:
		printAdminUsersUsage()
		return fmt.Errorf("unknown users subcommand %q", args[0])
	}
}

func runAdminAPIKeys(ctx context.Context, store userstore.Store, args []string, logger *log.Logger) error {
	if len(args) == 0 {
		printAdminAPIKeysUsage()
		return nil
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("gateway admin api-keys list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		userID := fs.Int64("user", 0, "user id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		keys, err := store.ListAPIKeys(ctx, *userID)
		if err != nil {
			return err
		}
		for _, key := range keys {
			var expiry string
			if key.ExpiresAt != nil {
				expiry = key.ExpiresAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("%4d  user=%d prefix=%s expires=%s scopes=%v\n", key.ID, key.UserID, key.Prefix, expiry, key.Scopes)
		}
		return nil
	case "create":
		fs := flag.NewFlagSet("gateway admin api-keys create", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		userID := fs.Int64("user", 0, "user id")
		scopes := fs.String("scopes", "", "comma separated scopes")
		ttl := fs.String("ttl", "", "lifetime (e.g. 720h)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		var expires *time.Time
		if strings.TrimSpace(*ttl) != "" {
			dur, err := time.ParseDuration(strings.TrimSpace(*ttl))
			if err != nil {
				return err
			}
			t := time.Now().Add(dur)
			expires = &t
		}
		scopeList := parseScopes(*scopes)
		key, token, err := store.CreateAPIKey(ctx, *userID, scopeList, expires)
		if err != nil {
			return err
		}
		fmt.Printf("API key id=%d token=%s\n", key.ID, token)
		return nil
	case "delete":
		fs := flag.NewFlagSet("gateway admin api-keys delete", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		id := fs.Int64("id", 0, "api key id")
		hard := fs.Bool("hard", false, "permanently delete (default is soft delete)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *hard {
			// Check if the store supports hard delete
			if hardDeleter, ok := store.(interface {
				HardDeleteAPIKey(context.Context, int64) error
			}); ok {
				if err := hardDeleter.HardDeleteAPIKey(ctx, *id); err != nil {
					return err
				}
				fmt.Printf("Permanently deleted api key %d\n", *id)
			} else {
				return fmt.Errorf("hard delete not supported by this store implementation")
			}
		} else {
			if err := store.DeleteAPIKey(ctx, *id); err != nil {
				return err
			}
			fmt.Printf("Soft deleted api key %d (marked as deleted)\n", *id)
		}
		return nil
	default:
		printAdminAPIKeysUsage()
		return fmt.Errorf("unknown api-keys subcommand %q", args[0])
	}
}

func setupHookDispatcher(cfg config.GatewayConfig, logger *log.Logger) *hooks.Dispatcher {
	if handler := cfg.Hooks.BuildScriptHandler(); handler != nil {
		dispatcher := &hooks.Dispatcher{}
		dispatcher.Register(handler)
		logger.Printf("hooks dispatcher enabled script=%s", cfg.Hooks.ScriptPath)
		return dispatcher
	}
	return nil
}

func emitUserEvent(ctx context.Context, dispatcher *hooks.Dispatcher, eventType hooks.EventType, user *userstore.User) {
	if dispatcher == nil || user == nil {
		return
	}
	metadata := map[string]any{
		"email":        user.Email,
		"role":         user.Role,
		"display_name": user.DisplayName,
		"status":       user.Status,
	}
	evt := hooks.Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		UserID:     strconv.FormatInt(user.ID, 10),
		Metadata:   metadata,
	}
	_ = dispatcher.Emit(ctx, evt)
}

func printAdminUsage() {
	fmt.Print(`Usage:
  gateway admin users <list|create|update|status|delete> [flags]
  gateway admin api-keys <list|create|delete> [flags]
`)
}

func printAdminUsersUsage() {
	fmt.Print(`Usage:
  gateway admin users list
  gateway admin users create --email <email> [--role gateway_user|gateway_admin] [--name "Display"]
  gateway admin users update --id <id> [--role gateway_user|gateway_admin] [--name "Display"]
  gateway admin users status --id <id> --status active|inactive
  gateway admin users delete --id <id> [--hard]  (--hard for permanent deletion, default is soft delete)
  gateway admin users import --file users.csv [--skip-existing]
`)
}

func printAdminAPIKeysUsage() {
	fmt.Print(`Usage:
  gateway admin api-keys list --user <id>
  gateway admin api-keys create --user <id> [--scopes scope1,scope2] [--ttl 720h]
  gateway admin api-keys delete --id <id> [--hard]  (--hard for permanent deletion, default is soft delete)
`)
}

func parseScopes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var scopes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			scopes = append(scopes, p)
		}
	}
	return scopes
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

func isHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	for _, col := range row {
		col = strings.TrimSpace(strings.ToLower(col))
		if col == "email" {
			return true
		}
	}
	return false
}

func normalizeHeader(row []string) []string {
	normalized := make([]string, len(row))
	for i, col := range row {
		normalized[i] = strings.TrimSpace(strings.ToLower(col))
	}
	return normalized
}

func valueAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func indexOf(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return -1
}

func isDuplicateUserError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}
