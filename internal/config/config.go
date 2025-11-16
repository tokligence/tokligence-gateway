package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/hooks"
)

const (
	settingsFile     = "config/setting.ini"
	defaultEnv       = "dev"
	envConfigPattern = "config/%s/gateway.ini"
)

// Settings contains global toggles such as the active environment.
type Settings struct {
	Environment string
	Defaults    map[string]string
}

// GatewayConfig describes runtime options for the CLI.
type GatewayConfig struct {
	Environment        string
	BaseURL            string
	Email              string
	DisplayName        string
	EnableProvider     bool
	MarketplaceEnabled bool
	TelemetryEnabled   bool
	AdminEmail         string
	PublishName        string
	ModelFamily        string
	PricePer1K         float64
	// Backward-compatible base log file; used if specific files unset
	LogFile string
	// Separate log files for CLI and daemon (preferred)
	LogFileCLI    string
	LogFileDaemon string
	LogLevel      string
	LedgerPath    string
	AuthSecret    string
	AuthDisabled  bool
	IdentityPath  string
	Hooks         hooks.Config
	// Upstream adapter configuration
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	OpenAIOrg        string
	AnthropicAPIKey  string
	AnthropicBaseURL string
	AnthropicVersion string
	// Routing configuration: pattern=adapter pairs, comma-separated
	Routes          map[string]string
	FallbackAdapter string
	ModelAliases    map[string]string
	// Optional external model alias sources
	ModelAliasesFile string
	ModelAliasesDir  string
	// Feature toggles
	AnthropicNativeEnabled     bool
	AnthropicForceSSE          bool
	AnthropicTokenCheckEnabled bool
	AnthropicMaxTokens         int
	// OpenAI completion max_tokens cap used by in-process sidecar (0 = disabled)
	OpenAICompletionMaxTokens int
	// OpenAI tool bridge streaming (default false for coding agents)
	OpenAIToolBridgeStreamEnabled bool
	// Multi-port configuration
	EnableFacade       bool
	MultiPortMode      bool
	FacadePort         int
	AdminPort          int
	AnthropicPort      int
	OpenAIPort         int
	FacadeEndpoints    []string
	OpenAIEndpoints    []string
	AnthropicEndpoints []string
	AdminEndpoints     []string
	// Bridge session management for deduplication
	BridgeSessionEnabled  bool
	BridgeSessionTTL      string // Duration string like "5m"
	BridgeSessionMaxCount int
	// Work mode: controls passthrough vs translation behavior globally
	// - auto: automatically choose passthrough or translation based on endpoint+model match
	// - passthrough: only allow direct passthrough/delegation, reject translation requests
	// - translation: only allow translation, reject passthrough requests
	WorkMode string // auto|passthrough|translation
	// Sidecar (Anthropic->OpenAI) model map lines: "claude-x=gpt-y"; may also be loaded from file
	SidecarModelMap     string
	SidecarModelMapFile string
	// Model-first provider routing (pattern => provider) applied before legacy routes
	ModelProviderRoutes []RouteRule
	// Optional duplicate tool detection guard for Responses flow
	DuplicateToolDetectionEnabled bool
}

// RouteRule captures an ordered pattern => target mapping while preserving declaration order.
type RouteRule struct {
	Pattern string
	Target  string
}

// LoadGatewayConfig reads the current environment and loads the appropriate gateway config file.
func LoadGatewayConfig(root string) (GatewayConfig, error) {
	if root == "" {
		root = "."
	}
	s, err := loadSettings(root)
	if err != nil {
		return GatewayConfig{}, err
	}

	envValues, err := parseINI(filepath.Join(root, fmt.Sprintf(envConfigPattern, s.Environment)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			envValues = map[string]string{}
		} else {
			return GatewayConfig{}, err
		}
	}

	merged := make(map[string]string)
	for k, v := range s.Defaults {
		merged[k] = v
	}
	for k, v := range envValues {
		merged[k] = v
	}

	cfg := GatewayConfig{
		Environment:        s.Environment,
		BaseURL:            firstNonEmpty(merged["base_url"], DefaultExchangeBaseURL(s.Environment)),
		Email:              merged["email"],
		DisplayName:        merged["display_name"],
		EnableProvider:     parseBool(merged["enable_provider"]),
		MarketplaceEnabled: parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_MARKETPLACE_ENABLED"), merged["marketplace_enabled"]), true),
		TelemetryEnabled:   parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_TELEMETRY_ENABLED"), merged["telemetry_enabled"]), true),
		AdminEmail:         firstNonEmpty(os.Getenv("TOKLIGENCE_ADMIN_EMAIL"), merged["admin_email"], "admin@local"),
		PublishName:        merged["publish_name"],
		ModelFamily:        merged["model_family"],
		LogFile:            firstNonEmpty(os.Getenv("TOKLIGENCE_LOG_FILE"), merged["log_file"]),
		LogLevel:           firstNonEmpty(merged["log_level"], "info"),
		LedgerPath:         firstNonEmpty(merged["ledger_path"], DefaultLedgerPath()),
		AuthSecret:         firstNonEmpty(os.Getenv("TOKLIGENCE_AUTH_SECRET"), merged["auth_secret"], "tokligence-dev-secret"),
		AuthDisabled:       parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_AUTH_DISABLED"), merged["auth_disabled"]), true),
		PricePer1K:         0.5,
		IdentityPath:       firstNonEmpty(os.Getenv("TOKLIGENCE_IDENTITY_PATH"), merged["identity_path"], DefaultIdentityPath()),
	}
	// Preferred separate log files with env override precedence
	cliLog := firstNonEmpty(os.Getenv("TOKLIGENCE_LOG_FILE_CLI"), os.Getenv("TOKLIGENCE_LOG_FILE"), merged["log_file_cli"], merged["log_file"])
	daemonLog := firstNonEmpty(os.Getenv("TOKLIGENCE_LOG_FILE_DAEMON"), os.Getenv("TOKLIGENCE_LOG_FILE"), merged["log_file_daemon"], merged["log_file"])
	cfg.LogFileCLI = cliLog
	cfg.LogFileDaemon = daemonLog
	hookArgs := firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_SCRIPT_ARGS"), merged["hooks_script_args"])
	hookEnv := firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_SCRIPT_ENV"), merged["hooks_script_env"])
	cfg.Hooks = hooks.Config{
		Enabled:    parseBool(firstNonEmpty(os.Getenv("TOKLIGENCE_HOOKS_ENABLED"), merged["hooks_enabled"])),
		ScriptPath: firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_SCRIPT"), merged["hooks_script_path"]),
		ScriptArgs: parseCSV(hookArgs),
		Env:        parseMap(hookEnv),
	}
	if v := firstNonEmpty(os.Getenv("TOKLIGENCE_HOOK_TIMEOUT"), merged["hooks_timeout"]); v != "" {
		dur, err := time.ParseDuration(v)
		if err != nil {
			return GatewayConfig{}, fmt.Errorf("invalid hooks_timeout %q: %w", v, err)
		}
		cfg.Hooks.Timeout = dur
	}
	if err := cfg.Hooks.Validate(); err != nil {
		return GatewayConfig{}, err
	}
	if v := merged["price_per_1k"]; v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.PricePer1K = parsed
		} else {
			return GatewayConfig{}, fmt.Errorf("invalid price_per_1k %q: %w", v, err)
		}
	}

	// Upstream adapter env overrides
	cfg.OpenAIAPIKey = firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_API_KEY"), merged["openai_api_key"])
	cfg.OpenAIBaseURL = firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_BASE_URL"), merged["openai_base_url"])
	cfg.OpenAIOrg = firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_ORG"), merged["openai_org"])
	cfg.AnthropicAPIKey = firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_API_KEY"), merged["anthropic_api_key"])
	cfg.AnthropicBaseURL = firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_BASE_URL"), merged["anthropic_base_url"])
	cfg.AnthropicVersion = firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_VERSION"), merged["anthropic_version"], "2023-06-01")
	cfg.EnableFacade = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_ENABLE_FACADE"), merged["enable_facade"]), true)
	cfg.MultiPortMode = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_MULTIPORT_MODE"), merged["multiport_mode"]), false)
	cfg.FacadePort = parseOptionalInt(firstNonEmpty(os.Getenv("TOKLIGENCE_FACADE_PORT"), merged["facade_port"]), 8081)
	cfg.AdminPort = parseOptionalInt(firstNonEmpty(os.Getenv("TOKLIGENCE_ADMIN_PORT"), merged["admin_port"]), 8079)
	cfg.AnthropicPort = parseOptionalInt(firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_PORT"), merged["anthropic_port"]), 8083)
	cfg.OpenAIPort = parseOptionalInt(firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_PORT"), merged["openai_port"]), 8082)
	cfg.FacadeEndpoints = parseCSV(firstNonEmpty(os.Getenv("TOKLIGENCE_FACADE_ENDPOINTS"), merged["facade_endpoints"]))
	cfg.AdminEndpoints = parseCSV(firstNonEmpty(os.Getenv("TOKLIGENCE_ADMIN_ENDPOINTS"), merged["admin_endpoints"]))
	cfg.OpenAIEndpoints = parseCSV(firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_ENDPOINTS"), merged["openai_endpoints"]))
	cfg.AnthropicEndpoints = parseCSV(firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_ENDPOINTS"), merged["anthropic_endpoints"]))
	cfg.FallbackAdapter = firstNonEmpty(os.Getenv("TOKLIGENCE_FALLBACK_ADAPTER"), merged["fallback_adapter"], "loopback")
	cfg.Routes = parseRoutes(firstNonEmpty(os.Getenv("TOKLIGENCE_ROUTES"), merged["routes"]))
	cfg.ModelProviderRoutes = parseRouteList(firstNonEmpty(os.Getenv("TOKLIGENCE_MODEL_PROVIDER_ROUTES"), merged["model_provider_routes"]))
	cfg.DuplicateToolDetectionEnabled = parseBool(firstNonEmpty(os.Getenv("TOKLIGENCE_DUPLICATE_TOOL_DETECTION"), merged["duplicate_tool_detection"]))
	cfg.ModelAliases = parseRoutes(firstNonEmpty(os.Getenv("TOKLIGENCE_MODEL_ALIASES"), merged["model_aliases"]))
	// Optional aliases from file and directory
	cfg.ModelAliasesFile = firstNonEmpty(os.Getenv("TOKLIGENCE_MODEL_ALIASES_FILE"), merged["model_aliases_file"])
	cfg.ModelAliasesDir = firstNonEmpty(os.Getenv("TOKLIGENCE_MODEL_ALIASES_DIR"), merged["model_aliases_dir"])
	extraAliasContent := strings.Builder{}
	if strings.TrimSpace(cfg.ModelAliasesFile) != "" {
		if b, err := os.ReadFile(cfg.ModelAliasesFile); err == nil {
			extraAliasContent.WriteString(string(b))
			extraAliasContent.WriteString("\n")
		}
	}
	if strings.TrimSpace(cfg.ModelAliasesDir) != "" {
		if entries, err := os.ReadDir(cfg.ModelAliasesDir); err == nil {
			for _, e := range entries {
				if !e.Type().IsRegular() {
					continue
				}
				// skip hidden files
				name := e.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				fp := filepath.Join(cfg.ModelAliasesDir, name)
				if b, err := os.ReadFile(fp); err == nil {
					extraAliasContent.WriteString(string(b))
					extraAliasContent.WriteString("\n")
				}
			}
		}
	}
	if s := strings.TrimSpace(extraAliasContent.String()); s != "" {
		fileAliases := parseRoutes(s)
		if len(fileAliases) > 0 {
			if cfg.ModelAliases == nil {
				cfg.ModelAliases = map[string]string{}
			}
			for k, v := range fileAliases {
				cfg.ModelAliases[k] = v
			}
		}
	}
	cfg.AnthropicNativeEnabled = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_NATIVE_ENABLED"), merged["anthropic_native_enabled"]), true)
	cfg.AnthropicForceSSE = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_FORCE_SSE"), merged["anthropic_force_sse"]), true)
	cfg.AnthropicTokenCheckEnabled = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_TOKEN_CHECK_ENABLED"), merged["anthropic_token_check_enabled"]), false)
	if v := firstNonEmpty(os.Getenv("TOKLIGENCE_ANTHROPIC_MAX_TOKENS"), merged["anthropic_max_tokens"]); strings.TrimSpace(v) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.AnthropicMaxTokens = parsed
		} else {
			return GatewayConfig{}, fmt.Errorf("invalid anthropic_max_tokens %q: %w", v, err)
		}
	} else {
		cfg.AnthropicMaxTokens = 8192
	}
	// For coding-agent scenarios, default to non-streaming tool bridge (batch) to improve continuity
	cfg.OpenAIToolBridgeStreamEnabled = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM"), merged["openai_tool_bridge_stream"]), false)
	// Optional cap for OpenAI completion max_tokens used by sidecar bridge (default 16384)
	if v := firstNonEmpty(os.Getenv("TOKLIGENCE_OPENAI_COMPLETION_MAX_TOKENS"), merged["openai_completion_max_tokens"], "16384"); strings.TrimSpace(v) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			cfg.OpenAICompletionMaxTokens = parsed
		} else {
			cfg.OpenAICompletionMaxTokens = 16384
		}
	} else {
		cfg.OpenAICompletionMaxTokens = 16384
	}

	// Bridge session management configuration
	cfg.BridgeSessionEnabled = parseOptionalBool(firstNonEmpty(os.Getenv("TOKLIGENCE_BRIDGE_SESSION_ENABLED"), merged["bridge_session_enabled"]), true)
	cfg.BridgeSessionTTL = firstNonEmpty(os.Getenv("TOKLIGENCE_BRIDGE_SESSION_TTL"), merged["bridge_session_ttl"], "5m")
	if v := firstNonEmpty(os.Getenv("TOKLIGENCE_BRIDGE_SESSION_MAX_COUNT"), merged["bridge_session_max_count"], "1000"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.BridgeSessionMaxCount = parsed
		} else {
			cfg.BridgeSessionMaxCount = 1000
		}
	}

	// Work mode: auto|passthrough|translation (default: auto)
	// Controls whether gateway operates in passthrough, translation, or auto mode globally
	cfg.WorkMode = strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("TOKLIGENCE_WORK_MODE"), merged["work_mode"], "auto")))
	// Validate work mode
	switch cfg.WorkMode {
	case "auto", "passthrough", "translation":
		// valid
	default:
		cfg.WorkMode = "auto"
	}

	// Optional sidecar model map (string + file content concatenated)
	cfg.SidecarModelMap = firstNonEmpty(os.Getenv("TOKLIGENCE_SIDECAR_MODEL_MAP"), merged["sidecar_model_map"])
	cfg.SidecarModelMapFile = firstNonEmpty(os.Getenv("TOKLIGENCE_SIDECAR_MODEL_MAP_FILE"), merged["sidecar_model_map_file"])
	if strings.TrimSpace(cfg.SidecarModelMapFile) != "" {
		if b, err := os.ReadFile(cfg.SidecarModelMapFile); err == nil {
			if strings.TrimSpace(cfg.SidecarModelMap) == "" {
				cfg.SidecarModelMap = string(b)
			} else {
				cfg.SidecarModelMap = cfg.SidecarModelMap + "\n" + string(b)
			}
		}
	}
	if len(cfg.ModelProviderRoutes) == 0 {
		cfg.ModelProviderRoutes = []RouteRule{
			{Pattern: "gpt*", Target: "openai"},
			{Pattern: "claude*", Target: "anthropic"},
		}
	}
	return cfg, nil
}

func loadSettings(root string) (Settings, error) {
	values, err := parseINI(filepath.Join(root, settingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return Settings{Environment: defaultEnv, Defaults: map[string]string{}}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	env := values["environment"]
	if env == "" {
		env = defaultEnv
	}
	defaults := make(map[string]string)
	for k, v := range values {
		if k == "environment" {
			continue
		}
		defaults[k] = v
	}
	return Settings{Environment: env, Defaults: defaults}, nil
}

func parseINI(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		values[strings.ToLower(key)] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseOptionalBool(v string, fallback bool) bool {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return parseBool(v)
}

func parseOptionalInt(v string, fallback int) int {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return parsed
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var out []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseMap(input string) map[string]string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	entries := strings.Split(input, ",")
	result := make(map[string]string)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		kv := strings.SplitN(entry, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])
		if key != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseRoutes parses model routing rules from a CSV or newline-separated string.
// Format examples:
//
//	gpt-* = openai, claude-* = anthropic, loopback = loopback
//	gpt-*=>openai\nclaude-3-5-sonnet=>anthropic\nloopback=>loopback
func parseRoutes(input string) map[string]string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	routes := make(map[string]string)
	// Support both comma-separated and newline-separated entries
	entries := []string{}
	for _, line := range strings.Split(input, "\n") {
		parts := strings.Split(line, ",")
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				entries = append(entries, p)
			}
		}
	}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		// support '=' or '=>'
		var kv []string
		if strings.Contains(e, "=>") {
			kv = strings.SplitN(e, "=>", 2)
		} else {
			kv = strings.SplitN(e, "=", 2)
		}
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key != "" && val != "" {
			routes[key] = val
		}
	}
	if len(routes) == 0 {
		return nil
	}
	return routes
}

// parseRouteList preserves ordering for pattern=>target rules (comma or newline separated).
func parseRouteList(input string) []RouteRule {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	var rules []RouteRule
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		for _, part := range strings.Split(line, ",") {
			entry := strings.TrimSpace(part)
			if entry == "" {
				continue
			}
			var kv []string
			if strings.Contains(entry, "=>") {
				kv = strings.SplitN(entry, "=>", 2)
			} else {
				kv = strings.SplitN(entry, "=", 2)
			}
			if len(kv) != 2 {
				continue
			}
			pattern := strings.TrimSpace(kv[0])
			target := strings.TrimSpace(kv[1])
			if pattern == "" || target == "" {
				continue
			}
			rules = append(rules, RouteRule{Pattern: pattern, Target: target})
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return rules
}

// DefaultLedgerPath returns the fallback ledger location under the user's home directory.
func DefaultLedgerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "ledger.db"
	}
	return filepath.Join(home, ".tokligence", "ledger.db")
}

// DefaultIdentityPath returns the fallback identity database path.
func DefaultIdentityPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "identity.db"
	}
	return filepath.Join(home, ".tokligence", "identity.db")
}

// DefaultExchangeBaseURL returns the canonical Token Marketplace host for the given environment.
func DefaultExchangeBaseURL(env string) string {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "dev":
		return "https://dev.tokligence.ai"
	case "test":
		return "https://test.tokligence.ai"
	case "live", "prod", "production":
		return "https://marketplace.tokligence.ai"
	default:
		return "http://localhost:8080"
	}
}
