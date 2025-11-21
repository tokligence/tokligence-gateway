package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/adapter/gemini"
	adapterrouter "github.com/tokligence/tokligence-gateway/internal/adapter/router"
	"github.com/tokligence/tokligence-gateway/internal/auth"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/firewall"
	"github.com/tokligence/tokligence-gateway/internal/hooks"
	anthpkg "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	openairesp "github.com/tokligence/tokligence-gateway/internal/httpserver/openai/responses"
	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
	tooladapter "github.com/tokligence/tokligence-gateway/internal/httpserver/tool_adapter"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
	translationhttp "github.com/tokligence/tokligence-gateway/internal/translation/adapterhttp"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

var (
	defaultFacadeEndpointKeys    = []string{"openai_core", "openai_responses", "anthropic", "gemini_native", "admin", "health"}
	defaultOpenAIEndpointKeys    = []string{"openai_core", "health"}
	defaultAnthropicEndpointKeys = []string{"anthropic", "health"}
	defaultGeminiEndpointKeys    = []string{"gemini_native", "health"}
	defaultAdminEndpointKeys     = []string{"admin", "health"}
)

// ModelProviderRule maps a model pattern (supports "*" wildcards) to a provider name.
type ModelProviderRule struct {
	Pattern  string
	Provider string
}

// GatewayFacade describes the gateway methods required by the HTTP layer.
type GatewayFacade interface {
	Account() (*client.User, *client.ProviderProfile)
	EnsureAccount(ctx context.Context, email string, roles []string, displayName string) (*client.User, *client.ProviderProfile, error)
	ListProviders(ctx context.Context) ([]client.ProviderProfile, error)
	ListServices(ctx context.Context, providerID *int64) ([]client.ServiceOffering, error)
	ListMyServices(ctx context.Context) ([]client.ServiceOffering, error)
	UsageSnapshot(ctx context.Context) (client.UsageSummary, error)
	MarketplaceAvailable() bool
	SetLocalAccount(user client.User, provider *client.ProviderProfile)
}

// Server exposes REST endpoints for the Tokligence Gateway.
type Server struct {
	gateway               GatewayFacade
	adapter               adapter.ChatAdapter
	embeddingAdapter      adapter.EmbeddingAdapter
	ledger                ledger.Store
	auth                  *auth.Manager
	identity              userstore.Store
	rootAdmin             *userstore.User
	hooks                 *hooks.Dispatcher
	enableAnthropicNative bool
	// passthrough + upstream configs
	anthAPIKey    string
	anthBaseURL   string
	anthVersion   string
	openaiAPIKey  string
	openaiBaseURL string
	// tool bridge behavior
	openaiToolBridgeStreamEnabled bool
	anthropicForceSSE             bool
	anthropicTokenCheckEnabled    bool
	anthropicMaxTokens            int
	authDisabled                  bool
	// logging
	logger   *log.Logger
	logLevel string
	// in-process Anthropic->OpenAI bridge handler
	anthropicBridgeHandler  http.Handler
	anthropicBridgeModelMap string
	// Work mode: controls passthrough vs translation globally
	// - auto: choose based on endpoint+model match
	// - passthrough: only allow passthrough/delegation, reject translation
	// - translation: only allow translation, reject passthrough
	workMode string // auto|passthrough|translation
	// streaming options
	responsesStreamAggregate bool
	ssePingInterval          time.Duration
	modelRouter              *adapterrouter.Router
	responsesTranslator      *openairesp.Translator
	// endpoint selection per port
	facadeEndpointKeys    []string
	openaiEndpointKeys    []string
	anthropicEndpointKeys []string
	geminiEndpointKeys    []string
	adminEndpointKeys     []string
	// response API sessions
	responsesSessionsMu sync.Mutex
	responsesSessions   map[string]*responseSession
	// tool adapter for API compatibility
	toolAdapter *tooladapter.Adapter
	// ordered list of model-first provider rules (pattern=>provider)
	modelProviderRules []ModelProviderRule
	// Responses duplicate-tool guard
	duplicateToolDetectionEnabled bool
	// Model metadata resolver
	modelMeta interface {
		MaxCompletionCap(model string) (int, bool)
	}
	// Anthropic beta feature toggles
	anthropicWebSearchEnabled   bool
	anthropicComputerUseEnabled bool
	anthropicMCPEnabled         bool
	anthropicPromptCaching      bool
	anthropicJSONModeEnabled    bool
	anthropicReasoningEnabled   bool
	chatToAnthropicEnabled      bool
	anthropicBetaHeader         string
	// Gemini integration
	geminiAdapter *gemini.GeminiAdapter
	geminiAPIKey  string
	geminiBaseURL string
	// Prompt firewall
	firewallPipeline *firewall.Pipeline
}

type bridgeExecResult struct {
	response         anthpkg.NativeResponse
	promptTokens     int
	completionTokens int
}

type bridgeUpstreamError struct {
	status int
	body   []byte
}

func (e bridgeUpstreamError) Error() string {
	if len(e.body) == 0 {
		return fmt.Sprintf("openai bridge upstream status %d", e.status)
	}
	preview := string(previewBytes(e.body, 256))
	return fmt.Sprintf("openai bridge upstream status %d: %s", e.status, preview)
}

// New constructs a Server with the required dependencies.
func New(gateway GatewayFacade, chatAdapter adapter.ChatAdapter, store ledger.Store, authManager *auth.Manager, identity userstore.Store, rootAdmin *userstore.User, dispatcher *hooks.Dispatcher, enableAnthropicNative bool) *Server {
	var rootCopy *userstore.User
	if rootAdmin != nil {
		copy := *rootAdmin
		copy.Email = strings.TrimSpace(strings.ToLower(copy.Email))
		rootCopy = &copy
	}

	// Check if chat adapter also supports embeddings
	var embAdapter adapter.EmbeddingAdapter
	if ea, ok := chatAdapter.(adapter.EmbeddingAdapter); ok {
		embAdapter = ea
	}

	server := &Server{
		gateway:               gateway,
		adapter:               chatAdapter,
		embeddingAdapter:      embAdapter,
		ledger:                store,
		auth:                  authManager,
		identity:              identity,
		rootAdmin:             rootCopy,
		hooks:                 dispatcher,
		enableAnthropicNative: enableAnthropicNative,
		responsesTranslator:   openairesp.NewTranslator(),
		responsesSessions:     make(map[string]*responseSession),
		toolAdapter:           tooladapter.NewAdapter(),
	}
	if rt, ok := chatAdapter.(*adapterrouter.Router); ok {
		server.modelRouter = rt
	}

	server.SetEndpointConfig(nil, nil, nil, nil, nil)

	return server
}

// Router returns a configured chi router for embedding in HTTP servers.
func (s *Server) Router() http.Handler {
	r := s.newBaseRouter()

	r.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/login", s.handleAuthLogin)
		api.Post("/auth/verify", s.handleAuthVerify)

		api.Group(func(private chi.Router) {
			if s.auth != nil && !s.authDisabled {
				private.Use(s.sessionMiddleware)
			}
			private.Get("/profile", s.handleProfile)
			private.Get("/providers", s.handleProviders)
			private.Get("/services", s.handleServices)
			private.Get("/usage/summary", s.handleUsageSummary)
			private.Get("/usage/logs", s.handleUsageLogs)
		})

	})

	s.registerEndpointKeys(r, s.facadeEndpointKeys...)
	return r
}

func (s *Server) newBaseRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	return r
}

func (s *Server) registerEndpoints(r chi.Router, endpoints ...protocol.Endpoint) {
	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		s.debugf("registering endpoint %s", ep.Name())
		for _, route := range ep.Routes() {
			r.Method(route.Method, route.Path, route.Handler)
		}
	}
}

func (s *Server) registerEndpointKeys(r chi.Router, keys ...string) int {
	if len(keys) == 0 {
		return 0
	}
	var endpoints []protocol.Endpoint
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if ep := s.endpointByKey(key); ep != nil {
			endpoints = append(endpoints, ep)
		} else if s.isDebug() {
			s.debugf("endpoint %s unavailable, skipping registration", key)
		}
	}
	if len(endpoints) == 0 {
		return 0
	}
	s.registerEndpoints(r, endpoints...)
	return len(endpoints)
}

func (s *Server) endpointByKey(key string) protocol.Endpoint {
	switch key {
	case "openai", "openai_core", "openai-chat", "openai_default":
		return newOpenAIEndpoint(s)
	case "responses", "openai_responses":
		return newResponsesEndpoint(s)
	case "anthropic":
		if s.enableAnthropicNative {
			return newAnthropicEndpoint(s)
		}
		return nil
	case "gemini", "gemini_native":
		return newGeminiEndpoint(s)
	case "admin":
		return newAdminEndpoint(s)
	case "health", "status":
		return newHealthEndpoint(s)
	default:
		return nil
	}
}

func (s *Server) wrapAdminHandler(fn http.HandlerFunc) http.Handler {
	var handler http.Handler = fn
	handler = s.requireRootAdmin(handler)
	if s.auth != nil && !s.authDisabled {
		handler = s.sessionMiddleware(handler)
	}
	return handler
}

func (s *Server) RouterOpenAI() http.Handler {
	if len(s.openaiEndpointKeys) == 0 {
		return nil
	}
	r := s.newBaseRouter()
	if s.registerEndpointKeys(r, s.openaiEndpointKeys...) == 0 {
		return nil
	}
	return r
}

func (s *Server) RouterAnthropic() http.Handler {
	if !s.enableAnthropicNative {
		return nil
	}
	if len(s.anthropicEndpointKeys) == 0 {
		return nil
	}
	r := s.newBaseRouter()
	if s.registerEndpointKeys(r, s.anthropicEndpointKeys...) == 0 {
		return nil
	}
	return r
}

func (s *Server) RouterGemini() http.Handler {
	if len(s.geminiEndpointKeys) == 0 {
		return nil
	}
	r := s.newBaseRouter()
	if s.registerEndpointKeys(r, s.geminiEndpointKeys...) == 0 {
		return nil
	}
	return r
}

func (s *Server) RouterAdmin() http.Handler {
	if len(s.adminEndpointKeys) == 0 {
		return nil
	}
	r := s.newBaseRouter()
	if s.registerEndpointKeys(r, s.adminEndpointKeys...) == 0 {
		return nil
	}
	return r
}

// handleAnthropicCountTokens provides a minimal implementation of the
// Anthropic-compatible count_tokens endpoint used by some clients (e.g. Claude Code)
// to budget max_tokens. It estimates tokens using a simple heuristic (4 chars ≈ 1 token).
func (s *Server) handleAnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
	rawBody, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	var req anthpkg.NativeRequest
	if err := json.NewDecoder(bytes.NewReader(rawBody)).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}
	totalChars := 0
	if sys := anthpkg.ExtractSystemText(req.System); strings.TrimSpace(sys) != "" {
		totalChars += len(sys)
	}
	for _, m := range req.Messages {
		for _, b := range m.Content.Blocks {
			switch strings.ToLower(b.Type) {
			case "text":
				totalChars += len(b.Text)
			case "tool_use":
				if b.Input != nil {
					if bs, err := json.Marshal(b.Input); err == nil {
						totalChars += len(bs)
					}
				}
			case "tool_result":
				if b.Text != "" {
					totalChars += len(b.Text)
				}
				for _, sub := range b.Content {
					if strings.EqualFold(sub.Type, "text") {
						totalChars += len(sub.Text)
					}
				}
			}
		}
	}
	tokens := totalChars/4 + 1
	if tokens < len(req.Messages)*2 {
		tokens = len(req.Messages) * 2
	}
	source := "local"
	// In passthrough mode with a configured Anthropic API key, try the real
	// /v1/messages/count_tokens endpoint for higher fidelity estimates. On any
	// failure, fall back to the local heuristic above.
	if strings.EqualFold(strings.TrimSpace(s.workMode), "passthrough") && strings.TrimSpace(s.anthAPIKey) != "" {
		if upstreamTokens, err := s.callAnthropicCountTokensUpstream(r.Context(), rawBody); err == nil && upstreamTokens > 0 {
			tokens = upstreamTokens
			source = "upstream"
		} else if s.isDebug() && s.logger != nil {
			s.logger.Printf("anthropic.count_tokens upstream_failed model=%s err=%v", strings.TrimSpace(req.Model), err)
		}
	}
	if s.isDebug() {
		s.debugf("anthropic.count_tokens: model=%s source=%s input_chars=%d input_tokens~=%d", req.Model, source, totalChars, tokens)
	}
	s.respondJSON(w, http.StatusOK, map[string]any{"input_tokens": tokens})
}

func (s *Server) HandleEmbeddings(w http.ResponseWriter, r *http.Request) {
	s.handleEmbeddings(w, r)
}

func (s *Server) HandleModels(w http.ResponseWriter, r *http.Request) {
	s.handleModels(w, r)
}

func (s *Server) HandleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	s.handleAnthropicMessages(w, r)
}

func (s *Server) HandleAnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
	s.handleAnthropicCountTokens(w, r)
}

// callAnthropicCountTokensUpstream calls the real Anthropic /v1/messages/count_tokens
// endpoint using the configured upstream base URL and API key.
func (s *Server) callAnthropicCountTokensUpstream(ctx context.Context, rawBody []byte) (int, error) {
	base := strings.TrimRight(strings.TrimSpace(s.anthBaseURL), "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	url := base + "/v1/messages/count_tokens"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(s.anthVersion) != "" {
		req.Header.Set("anthropic-version", s.anthVersion)
	}
	if strings.TrimSpace(s.anthAPIKey) != "" {
		req.Header.Set("x-api-key", s.anthAPIKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("anthropic count_tokens upstream %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if payload.InputTokens <= 0 {
		return 0, fmt.Errorf("anthropic count_tokens upstream returned invalid input_tokens=%d", payload.InputTokens)
	}
	return payload.InputTokens, nil
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"status":           "ok",
		"time":             time.Now().UTC().Format(time.RFC3339),
		"anthropic_native": s.enableAnthropicNative,
		"work_mode":        s.workMode,
	}
	if s.modelRouter != nil {
		payload["adapters"] = s.modelRouter.ListAdapters()
		payload["routes"] = s.modelRouter.ListRoutes()
	}
	s.respondJSON(w, http.StatusOK, payload)
}

func normalizeEndpointKeys(list []string, defaults []string) []string {
	if len(list) == 0 {
		list = defaults
	}
	seen := make(map[string]struct{}, len(list))
	out := make([]string, 0, len(list))
	for _, key := range list {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// SetEndpointConfig configures which endpoint bundles are exposed on each port.
func (s *Server) SetEndpointConfig(facade, openai, anthropic, gemini, admin []string) {
	s.facadeEndpointKeys = normalizeEndpointKeys(facade, defaultFacadeEndpointKeys)
	s.openaiEndpointKeys = normalizeEndpointKeys(openai, defaultOpenAIEndpointKeys)
	s.anthropicEndpointKeys = normalizeEndpointKeys(anthropic, defaultAnthropicEndpointKeys)
	s.geminiEndpointKeys = normalizeEndpointKeys(gemini, defaultGeminiEndpointKeys)
	s.adminEndpointKeys = normalizeEndpointKeys(admin, defaultAdminEndpointKeys)
}

// SetUpstreams configures upstream credentials and mode toggles for native endpoints and bridges.
func (s *Server) SetUpstreams(openaiKey, openaiBase string, anthKey, anthBase, anthVer string, openaiToolBridgeStream bool, forceSSE bool, tokenCheck bool, maxTokens int, openaiCompletionMax int, anthropicBridgeModelMap string, meta interface {
	MaxCompletionCap(model string) (int, bool)
}) {
	s.openaiAPIKey = strings.TrimSpace(openaiKey)
	s.openaiBaseURL = strings.TrimRight(strings.TrimSpace(openaiBase), "/")
	if s.openaiBaseURL == "" {
		s.openaiBaseURL = "https://api.openai.com/v1"
	}
	s.anthAPIKey = strings.TrimSpace(anthKey)
	s.anthBaseURL = strings.TrimRight(strings.TrimSpace(anthBase), "/")
	if s.anthBaseURL == "" {
		s.anthBaseURL = "https://api.anthropic.com"
	}
	s.anthVersion = strings.TrimSpace(anthVer)
	if s.anthVersion == "" {
		s.anthVersion = "2023-06-01"
	}
	s.openaiToolBridgeStreamEnabled = openaiToolBridgeStream
	s.anthropicForceSSE = forceSSE
	s.anthropicTokenCheckEnabled = tokenCheck
	s.anthropicMaxTokens = maxTokens
	s.anthropicBridgeModelMap = anthropicBridgeModelMap
	s.modelMeta = meta
	// Streaming config from env (optional)
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TOKLIGENCE_RESPONSES_STREAM_MODE"))) {
	case "aggregate", "agg", "buffered":
		s.responsesStreamAggregate = true
	default:
		s.responsesStreamAggregate = false
	}
	if v := strings.TrimSpace(os.Getenv("TOKLIGENCE_RESPONSES_SSE_PING_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			s.ssePingInterval = time.Duration(n) * time.Millisecond
		}
	}
	// Default to 2s for better keepalive (was 6s, but Codex needs frequent heartbeats)
	if s.ssePingInterval == 0 {
		s.ssePingInterval = 2 * time.Second
	}

	// Build an in-process Anthropic->OpenAI bridge handler for the Anthropic endpoint
	// so we avoid duplicating proxy logic.
	scfg := translationhttp.Config{
		OpenAIBaseURL:      s.openaiBaseURL,
		OpenAIAPIKey:       s.openaiAPIKey,
		AnthropicBaseURL:   s.anthBaseURL,
		AnthropicAPIKey:    s.anthAPIKey,
		AnthropicVersion:   s.anthVersion,
		ModelMap:           s.anthropicBridgeModelMap,
		DefaultOpenAIModel: "gpt-4o",
		MaxTokensCap:       openaiCompletionMax,
		ModelCap: func(model string) (int, bool) {
			if s.modelMeta != nil {
				return s.modelMeta.MaxCompletionCap(model)
			}
			return 0, false
		},
		EnableWebSearch:     s.anthropicWebSearchEnabled,
		EnableComputerUse:   s.anthropicComputerUseEnabled,
		EnableMCP:           s.anthropicMCPEnabled,
		EnablePromptCaching: s.anthropicPromptCaching,
		EnableJSONMode:      s.anthropicJSONModeEnabled,
		EnableReasoning:     s.anthropicReasoningEnabled,
	}
	s.anthropicBridgeHandler = translationhttp.NewMessagesHandler(scfg, http.DefaultClient)
	if s.logger != nil {
		// count non-empty, non-comment lines with '=' present
		count := 0
		for _, line := range strings.Split(s.anthropicBridgeModelMap, "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
				continue
			}
			if strings.Contains(t, "=") {
				count++
			}
		}
		s.logger.Printf("anthropic_bridge.model_map rules=%d", count)
	}
}

func (s *Server) SetAuthDisabled(disabled bool) {
	s.authDisabled = disabled
	if disabled && s.isDebug() {
		s.debugf("authorization disabled via configuration")
	}
}

// SetLogger configures server-level logger and verbosity ("debug", "info", ...).
func (s *Server) SetLogger(level string, logger *log.Logger) {
	s.logLevel = strings.ToLower(strings.TrimSpace(level))
	if logger != nil {
		s.logger = logger
	}
}

// SetBridgeSessionConfig configures the bridge session manager
func (s *Server) SetBridgeSessionConfig(enabled bool, ttl string, maxCount int) error {
	// Session-based deduplication is intentionally disabled for stateless bridging.
	// This method is kept as a no-op to preserve config compatibility.
	if s.isDebug() {
		s.debugf("Bridge session manager disabled (stateless mode). Requested enabled=%v ttl=%s max=%d", enabled, ttl, maxCount)
	}
	return nil
}

func (s *Server) isDebug() bool { return s.logLevel == "debug" }
func (s *Server) debugf(format string, args ...any) {
	if s.logger != nil && s.isDebug() {
		s.logger.Printf("DEBUG "+format, args...)
	}
}

// SetWorkMode configures the global work mode: auto|passthrough|translation
// - auto: automatically choose passthrough or translation based on endpoint+model match
// - passthrough: only allow direct passthrough/delegation, reject translation requests
// - translation: only allow translation, reject passthrough requests
func (s *Server) SetWorkMode(mode string) {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "auto", "passthrough", "translation":
		s.workMode = m
	default:
		s.workMode = "auto"
	}
}

// SetModelProviderRules configures ordered pattern=>provider overrides for model-first routing.
func (s *Server) SetModelProviderRules(rules []ModelProviderRule) {
	s.modelProviderRules = s.modelProviderRules[:0]
	for _, rule := range rules {
		pattern := strings.ToLower(strings.TrimSpace(rule.Pattern))
		provider := strings.ToLower(strings.TrimSpace(rule.Provider))
		if pattern == "" || provider == "" {
			continue
		}
		s.modelProviderRules = append(s.modelProviderRules, ModelProviderRule{
			Pattern:  pattern,
			Provider: provider,
		})
	}
}

// SetDuplicateToolDetectionEnabled toggles duplicate tool-call detection for Responses flows.
func (s *Server) SetDuplicateToolDetectionEnabled(enabled bool) {
	s.duplicateToolDetectionEnabled = enabled
}

// SetModelMetadataResolver wires in an optional metadata source for per-model caps.
func (s *Server) SetModelMetadataResolver(resolver interface {
	MaxCompletionCap(model string) (int, bool)
}) {
	s.modelMeta = resolver
}

// SetAnthropicBetaFeatures toggles Anthropic beta capabilities.
func (s *Server) SetAnthropicBetaFeatures(webSearch, computerUse, mcp, promptCaching, jsonMode, reasoning bool) {
	s.anthropicWebSearchEnabled = webSearch
	s.anthropicComputerUseEnabled = computerUse
	s.anthropicMCPEnabled = mcp
	s.anthropicPromptCaching = promptCaching
	s.anthropicJSONModeEnabled = jsonMode
	s.anthropicReasoningEnabled = reasoning
}

// SetChatToAnthropicEnabled toggles translation of /v1/chat/completions to Anthropic Messages.
func (s *Server) SetChatToAnthropicEnabled(enabled bool) {
	s.chatToAnthropicEnabled = enabled
}

// SetAnthropicBetaHeader overrides the beta header string (comma-separated tokens).
func (s *Server) SetAnthropicBetaHeader(header string) {
	s.anthropicBetaHeader = strings.TrimSpace(header)
}

// SetFirewallPipeline configures the prompt firewall pipeline.
func (s *Server) SetFirewallPipeline(pipeline *firewall.Pipeline) {
	s.firewallPipeline = pipeline
	if s.logger != nil && s.isDebug() {
		if pipeline != nil {
			stats := pipeline.Stats()
			s.logger.Printf("firewall configured: mode=%s filters=%d", stats["mode"], stats["total_filters"])
		} else {
			s.logger.Printf("firewall disabled")
		}
	}
}

func (s *Server) buildAnthropicBetaHeader() string {
	if s.anthropicBetaHeader != "" {
		return s.anthropicBetaHeader
	}
	var tokens []string
	if s.anthropicWebSearchEnabled {
		tokens = append(tokens, "web-search-2023-07-01")
	}
	if s.anthropicComputerUseEnabled {
		tokens = append(tokens, "computer-use-2024-12-05")
	}
	if s.anthropicMCPEnabled {
		tokens = append(tokens, "mcp-2024-10-22")
	}
	if s.anthropicPromptCaching {
		tokens = append(tokens, "prompt-caching-2024-07-01")
	}
	if s.anthropicJSONModeEnabled {
		tokens = append(tokens, "json-mode-2024-12-17")
	}
	if s.anthropicReasoningEnabled {
		tokens = append(tokens, "reasoning-2024-12-17")
	}
	return strings.Join(tokens, ",")
}

// translateChatToAnthropic bridges OpenAI Chat -> Anthropic Messages using the translator.
func (s *Server) translateChatToAnthropic(w http.ResponseWriter, r *http.Request, reqStart time.Time, req openai.ChatCompletionRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) {
	if strings.TrimSpace(s.anthAPIKey) == "" {
		s.respondError(w, http.StatusBadGateway, errors.New("anthropic API key not configured"))
		return
	}
	if s.responsesTranslator == nil {
		s.responsesTranslator = openairesp.NewTranslator()
	}
	anthReq, err := s.responsesTranslator.ChatToAnthropic(req)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	url := strings.TrimRight(s.anthBaseURL, "/") + "/v1/messages"
	if req.Stream {
		s.forwardChatStreamToAnthropic(w, r, reqStart, req, anthReq, url)
		return
	}
	s.forwardChatOnceToAnthropic(w, r, reqStart, req, anthReq, url)
}

func (s *Server) forwardChatOnceToAnthropic(w http.ResponseWriter, r *http.Request, reqStart time.Time, req openai.ChatCompletionRequest, anthReq anthpkg.NativeRequest, url string) {
	body, err := anthpkg.MarshalRequest(anthReq)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	httpReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", s.anthAPIKey)
	httpReq.Header.Set("anthropic-version", s.anthVersion)
	if beta := s.buildAnthropicBetaHeader(); beta != "" {
		httpReq.Header.Set("anthropic-beta", beta)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	if s.logger != nil {
		s.logger.Printf("chat.to.anthropic total_ms=%d model=%s status=%d", time.Since(reqStart).Milliseconds(), req.Model, resp.StatusCode)
	}
}

func (s *Server) forwardChatStreamToAnthropic(w http.ResponseWriter, r *http.Request, reqStart time.Time, req openai.ChatCompletionRequest, anthReq anthpkg.NativeRequest, url string) {
	anthReq.Stream = true
	body, err := anthpkg.MarshalRequest(anthReq)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	httpReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("x-api-key", s.anthAPIKey)
	httpReq.Header.Set("anthropic-version", s.anthVersion)
	if beta := s.buildAnthropicBetaHeader(); beta != "" {
		httpReq.Header.Set("anthropic-beta", beta)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		s.respondError(w, http.StatusBadGateway, fmt.Errorf("anthropic http %d: %s", resp.StatusCode, string(raw)))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.respondError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		resp.Body.Close()
		return
	}
	translator := s.responsesTranslator
	if translator == nil {
		translator = openairesp.NewTranslator()
	}
	err = translator.StreamNativeToOpenAI(r.Context(), req.Model, resp.Body, func(chunk openai.ChatCompletionChunk) error {
		b, _ := json.Marshal(chunk)
		if _, err := w.Write([]byte("data: ")); err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n\n")); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	resp.Body.Close()
	if err != nil && !errors.Is(err, context.Canceled) {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	if _, err := w.Write([]byte("data: [DONE]\n\n")); err == nil && flusher != nil {
		flusher.Flush()
	}
	if s.logger != nil {
		s.logger.Printf("chat.to.anthropic.stream total_ms=%d model=%s", time.Since(reqStart).Milliseconds(), req.Model)
	}
}

// workModeDecision determines how to handle a request based on work mode, endpoint, and model.
// Returns (usePassthrough bool, err error)
// - usePassthrough=true means direct passthrough/delegation to upstream
// - usePassthrough=false means translation is needed
// - err != nil means request should be rejected with error
func (s *Server) workModeDecision(endpoint, model string) (bool, error) {
	model = strings.TrimSpace(model)
	// Determine native provider for endpoint
	var endpointProvider string
	switch endpoint {
	case "/v1/chat/completions", "/v1/embeddings":
		endpointProvider = "openai"
	case "/v1/messages":
		endpointProvider = "anthropic"
	case "/v1/responses":
		endpointProvider = "openai" // Responses API is OpenAI format
	default:
		endpointProvider = ""
	}

	// Determine provider for model via routing
	routerProvider := ""
	if s.modelRouter != nil {
		if adapterName, err := s.modelRouter.GetAdapterForModel(model); err == nil {
			routerProvider = strings.ToLower(strings.TrimSpace(adapterName))
		}
	}
	overrideProvider, overridePattern := s.matchModelProvider(model)
	modelProvider := routerProvider
	if overrideProvider != "" {
		modelProvider = overrideProvider
	}
	providerAvailable := s.providerAvailable(modelProvider)

	// Check if endpoint and model match (passthrough possible) or mismatch (translation needed)
	needsTranslation := (endpointProvider != "" && modelProvider != "" && endpointProvider != modelProvider)
	needsPassthrough := (endpointProvider != "" && modelProvider != "" && endpointProvider == modelProvider)
	if needsPassthrough && !providerAvailable {
		needsPassthrough = false
		if endpointProvider != "" {
			needsTranslation = true
		}
	}

	mode := strings.ToLower(strings.TrimSpace(s.workMode))
	if mode == "" {
		mode = "auto"
	}

	decision := "translation"
	reason := "endpoint/provider mismatch"
	logDecision := func(action, why string) {
		if s.logger != nil && s.isDebug() {
			s.logger.Printf("workmode: endpoint=%s model=%s endpoint_provider=%s router_provider=%s override_provider=%s override_pattern=%s final_provider=%s provider_available=%t mode=%s action=%s reason=%s",
				endpoint, model, endpointProvider, routerProvider, overrideProvider, overridePattern, modelProvider, providerAvailable, mode, action, why)
			// Human-friendly summary line for quick debugging.
			s.logger.Printf("workmode.summary: endpoint=%s model=%s mode=%s path=%s provider=%s (router=%s override=%s pattern=%s) reason=%s",
				endpoint, model, mode, action, modelProvider, routerProvider, overrideProvider, overridePattern, why)
		}
	}

	switch mode {
	case "auto":
		if needsPassthrough && providerAvailable {
			decision = "passthrough"
			reason = "endpoint/provider aligned"
			logDecision(decision, reason)
			return true, nil
		}
		if needsPassthrough && !providerAvailable {
			reason = fmt.Sprintf("provider %s unavailable", modelProvider)
		} else if !needsTranslation {
			reason = "no provider match; translation fallback"
		}
		logDecision(decision, reason)
		return false, nil
	case "passthrough":
		if needsTranslation {
			reason = fmt.Sprintf("endpoint expects %s but model routes to %s", endpointProvider, modelProvider)
			logDecision("error", reason)
			return false, fmt.Errorf("work_mode=passthrough does not support translation (endpoint=%s expects %s provider, but model=%s routes to %s provider); set work_mode=auto or work_mode=translation to enable translation", endpoint, endpointProvider, model, modelProvider)
		}
		if !providerAvailable && endpointProvider != "" {
			reason = fmt.Sprintf("provider %s unavailable", modelProvider)
			logDecision("error", reason)
			return false, fmt.Errorf("work_mode=passthrough requires %s provider, but no credentials are configured", modelProvider)
		}
		decision = "passthrough"
		reason = "mode=passthrough forced passthrough"
		logDecision(decision, reason)
		return true, nil
	case "translation":
		decision = "translation"
		if needsPassthrough {
			reason = fmt.Sprintf("mode=translation forcing translation even though endpoint=%s and model=%s both use provider %s", endpoint, model, modelProvider)
		} else {
			reason = "mode=translation forced translation"
		}
		logDecision(decision, reason)
		return false, nil
	default:
		if needsPassthrough && providerAvailable {
			decision = "passthrough"
			reason = "default passthrough"
			logDecision(decision, reason)
			return true, nil
		}
		if needsPassthrough && !providerAvailable {
			reason = fmt.Sprintf("provider %s unavailable", modelProvider)
		} else if !needsTranslation {
			reason = "no provider match; translation fallback"
		}
		logDecision(decision, reason)
		return false, nil
	}
}

func (s *Server) matchModelProvider(model string) (provider, pattern string) {
	if len(s.modelProviderRules) == 0 {
		return "", ""
	}
	m := strings.ToLower(strings.TrimSpace(model))
	if m == "" {
		return "", ""
	}
	for _, rule := range s.modelProviderRules {
		if matchModelPattern(m, rule.Pattern) {
			return rule.Provider, rule.Pattern
		}
	}
	return "", ""
}

func matchModelPattern(model, pattern string) bool {
	model = strings.ToLower(model)
	pattern = strings.ToLower(pattern)
	if model == pattern {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	if strings.HasSuffix(pattern, "*") && !strings.HasPrefix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}
	if strings.HasPrefix(pattern, "*") && !strings.HasSuffix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(model, suffix)
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		sub := strings.Trim(pattern, "*")
		return strings.Contains(model, sub)
	}
	return false
}

func (s *Server) providerAvailable(provider string) bool {
	switch provider {
	case "":
		return false
	case "openai":
		return strings.TrimSpace(s.openaiAPIKey) != ""
	case "anthropic":
		return strings.TrimSpace(s.anthAPIKey) != ""
	default:
		return true
	}
}

func (s *Server) recordBridgeLedger(ctx context.Context, memo string, promptTokens, completionTokens int, sessionUser *userstore.User, apiKey *userstore.APIKey) {
	if s.ledger == nil {
		return
	}
	var uid int64
	if sessionUser != nil {
		uid = sessionUser.ID
	} else if user, _ := s.gateway.Account(); user != nil {
		uid = user.ID
	}
	if uid == 0 {
		return
	}
	entry := ledger.Entry{
		UserID:           uid,
		PromptTokens:     int64(promptTokens),
		CompletionTokens: int64(completionTokens),
		Direction:        ledger.DirectionConsume,
		Memo:             memo,
	}
	if apiKey != nil {
		id := apiKey.ID
		entry.APIKeyID = &id
	}
	_ = s.ledger.Record(ctx, entry)
}
func (s *Server) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := s.authenticateRequest(r)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		ctx := context.WithValue(r.Context(), sessionContextKey{}, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) authenticateRequest(r *http.Request) (*sessionInfo, error) {
	if s.identity == nil {
		return nil, errors.New("identity store unavailable")
	}

	if token := bearerToken(r.Header.Get("Authorization")); token != "" {
		key, user, err := s.identity.LookupAPIKey(r.Context(), token)
		if err != nil {
			return nil, err
		}
		if key == nil || user == nil || user.Status != userstore.StatusActive {
			return nil, errors.New("invalid api key")
		}
		clientUser := s.applySessionUser(user)
		return &sessionInfo{user: user, clientUser: clientUser, viaAPIKey: true}, nil
	}

	cookie, err := r.Cookie("tokligence_session")
	if err != nil || cookie.Value == "" {
		return nil, errors.New("missing session")
	}
	email, err := s.auth.ValidateToken(cookie.Value)
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(strings.ToLower(email))
	var user *userstore.User
	if s.identity != nil {
		user, err = s.identity.FindByEmail(r.Context(), email)
		if err != nil {
			return nil, err
		}
	}
	if user == nil && s.rootAdmin != nil && strings.EqualFold(s.rootAdmin.Email, email) {
		user = &userstore.User{ID: s.rootAdmin.ID, Email: s.rootAdmin.Email, Role: userstore.RoleRootAdmin, Status: userstore.StatusActive}
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if user.Status != userstore.StatusActive {
		return nil, errors.New("user inactive")
	}
	clientUser := s.applySessionUser(user)
	return &sessionInfo{user: user, clientUser: clientUser}, nil
}

func (s *Server) authenticateAPIKeyRequest(r *http.Request) (*userstore.User, *userstore.APIKey, error) {
	if s.authDisabled {
		return nil, nil, nil
	}
	if s.identity == nil {
		return nil, nil, errors.New("identity store unavailable")
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if token == "" {
		return nil, nil, errors.New("missing api key")
	}
	key, user, err := s.identity.LookupAPIKey(r.Context(), token)
	if err != nil {
		return nil, nil, err
	}
	if key == nil || user == nil || user.Status != userstore.StatusActive {
		return nil, nil, errors.New("invalid api key")
	}
	return user, key, nil
}

func (s *Server) applySessionUser(user *userstore.User) *client.User {
	if user == nil {
		return nil
	}
	roles := []string{}
	switch user.Role {
	case userstore.RoleRootAdmin:
		roles = append(roles, "root_admin", "consumer")
	case userstore.RoleGatewayAdmin:
		roles = append(roles, "gateway_admin", "consumer")
	default:
		roles = append(roles, "consumer")
	}
	cUser := client.User{
		ID:    user.ID,
		Email: user.Email,
		Roles: roles,
	}
	_, existingProvider := s.gateway.Account()
	s.gateway.SetLocalAccount(cUser, existingProvider)
	return &cUser
}

func (s *Server) requireRootAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := sessionFromContext(r.Context())
		if info == nil || info.user == nil || info.user.Role != userstore.RoleRootAdmin {
			s.respondError(w, http.StatusForbidden, errors.New("admin access required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) emitUserHook(ctx context.Context, eventType hooks.EventType, user *userstore.User) {
	if s.hooks == nil || user == nil {
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
	_ = s.hooks.Emit(ctx, evt)
}

func sessionFromContext(ctx context.Context) *sessionInfo {
	info, _ := ctx.Value(sessionContextKey{}).(*sessionInfo)
	return info
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

func isDuplicateUserError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func (s *Server) respondJSON(w http.ResponseWriter, status int, payload any) {
	if payload == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) respondError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	s.respondJSON(w, status, map[string]any{"error": err.Error()})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	reqStart := time.Now()
	// Try to build dynamic model list from router routes if available
	now := time.Now().Unix()
	if lr, ok := s.adapter.(interface{ ListRoutes() map[string]string }); ok {
		routes := lr.ListRoutes()
		models := make([]openai.Model, 0, len(routes)+1)
		seen := map[string]bool{}
		for pattern, owner := range routes {
			// Only include exact IDs (skip wildcards) for clarity
			if strings.Contains(pattern, "*") {
				continue
			}
			if pattern == "" || seen[pattern] {
				continue
			}
			models = append(models, openai.NewModel(pattern, owner, now))
			seen[pattern] = true
		}
		if !seen["loopback"] {
			models = append(models, openai.NewModel("loopback", "tokligence", now))
		}
		s.respondJSON(w, http.StatusOK, openai.NewModelsResponse(models))
		if s.logger != nil {
			s.logger.Printf("models total_ms=%d", time.Since(reqStart).Milliseconds())
		}
		return
	}

	// Fallback to static list
	models := []openai.Model{
		openai.NewModel("loopback", "tokligence", now),
		openai.NewModel("gpt-4", "openai", now),
		openai.NewModel("gpt-4-turbo", "openai", now),
		openai.NewModel("gpt-3.5-turbo", "openai", now),
		openai.NewModel("claude-3-5-sonnet-20241022", "anthropic", now),
	}
	s.respondJSON(w, http.StatusOK, openai.NewModelsResponse(models))
	if s.logger != nil {
		s.logger.Printf("models total_ms=%d", time.Since(reqStart).Milliseconds())
	}
}

func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request) {
	reqStart := time.Now()
	if s.embeddingAdapter == nil {
		s.respondError(w, http.StatusNotImplemented, errors.New("embeddings not supported by current adapter"))
		return
	}

	var (
		sessionUser *userstore.User
		apiKey      *userstore.APIKey
	)
	if s.identity != nil && !s.authDisabled {
		var err error
		sessionUser, apiKey, err = s.authenticateAPIKeyRequest(r)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		if sessionUser != nil {
			s.applySessionUser(sessionUser)
		}
	}

	var req openai.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	upstreamStart := time.Now()
	resp, err := s.embeddingAdapter.CreateEmbedding(r.Context(), req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	upstreamDur := time.Since(upstreamStart)

	if s.ledger != nil {
		var ledgerUserID int64
		if sessionUser != nil {
			ledgerUserID = sessionUser.ID
		} else if user, _ := s.gateway.Account(); user != nil {
			ledgerUserID = user.ID
		}
		if ledgerUserID != 0 {
			entry := ledger.Entry{
				UserID:           ledgerUserID,
				ServiceID:        0,
				PromptTokens:     int64(resp.Usage.PromptTokens),
				CompletionTokens: 0,
				Direction:        ledger.DirectionConsume,
				Memo:             "embeddings",
			}
			if apiKey != nil {
				id := apiKey.ID
				entry.APIKeyID = &id
			}
			_ = s.ledger.Record(r.Context(), entry)
		}
	}

	s.respondJSON(w, http.StatusOK, resp)
	if s.logger != nil {
		total := time.Since(reqStart)
		s.logger.Printf("embeddings total_ms=%d upstream_ms=%d model=%s", total.Milliseconds(), upstreamDur.Milliseconds(), req.Model)
	}
}

// --- Anthropic native endpoint support ---
func (s *Server) handleAnthropicMessages(w http.ResponseWriter, r *http.Request) {
	// Decide between Anthropic->OpenAI bridge or direct passthrough based on work mode
	reqStart := time.Now()
	rawBody, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	var stream bool
	var model string
	var tmp map[string]any
	if json.Unmarshal(rawBody, &tmp) == nil {
		if v, ok := tmp["stream"].(bool); ok {
			stream = v
		}
		if v, ok := tmp["model"].(string); ok {
			model = v
		}
	}

	// Use work mode decision to determine passthrough vs translation
	usePassthrough, err := s.workModeDecision("/v1/messages", model)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	if usePassthrough && strings.TrimSpace(s.anthAPIKey) != "" {
		s.anthropicPassthrough(w, r, rawBody, stream, model, nil, nil)
		if s.logger != nil {
			s.logger.Printf("anthropic.messages mode=passthrough model=%s total_ms=%d", strings.TrimSpace(model), time.Since(reqStart).Milliseconds())
		}
		return
	}
	if s.anthropicBridgeHandler != nil {
		// Rebuild request body for bridge handler (translation mode)
		r2 := r.Clone(r.Context())
		r2.Body = io.NopCloser(bytes.NewReader(rawBody))
		s.anthropicBridgeHandler.ServeHTTP(w, r2)
		if s.logger != nil {
			s.logger.Printf("anthropic.messages mode=translation provider=openai model=%s total_ms=%d", strings.TrimSpace(model), time.Since(reqStart).Milliseconds())
		}
		return
	}
	s.respondError(w, http.StatusNotImplemented, errors.New("anthropic messages handler not available"))
}

// --- OpenAI tool bridge (non-streaming) disabled in favor of Anthropic->OpenAI bridge
func (s *Server) executeOpenAIToolBridge(ctx context.Context, areq anthpkg.NativeRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) (bridgeExecResult, error) {
	return bridgeExecResult{}, errors.New("openai tool bridge disabled in favor of Anthropic->OpenAI bridge")
}

func (s *Server) openaiToolBridge(w http.ResponseWriter, r *http.Request, areq anthpkg.NativeRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) {
	s.respondError(w, http.StatusNotImplemented, errors.New("openai tool bridge disabled in favor of Anthropic->OpenAI bridge"))
}

func (s *Server) openaiToolBridgeBatchSSE(w http.ResponseWriter, r *http.Request, areq anthpkg.NativeRequest, sessionUser *userstore.User, apiKey *userstore.APIKey) {
	s.respondError(w, http.StatusNotImplemented, errors.New("openai tool bridge disabled in favor of Anthropic->OpenAI bridge"))
}

// extractLastUserMessage extracts the last user text message from the request
// extractLastUserMessage was used for session-based dedup (now removed).
// Keeping this placeholder ensures stable diffs if re-introduced in future.

// logToolBlocksPreview emits a compact summary of tool_use/tool_result blocks for diagnostics.
// logToolBlocksPreview removed (legacy bridge diagnostics no longer applicable)

// --- OpenAI tool bridge (streaming): forward OpenAI SSE deltas as Anthropic-style content_block_delta ---
// openaiToolBridgeStream removed (Anthropic->OpenAI bridge handles streaming tool bridge)

// toolInputChunks removed (legacy bridge)

type sessionContextKey struct{}

type sessionInfo struct {
	user       *userstore.User
	clientUser *client.User
	viaAPIKey  bool
}

func toUserPayload(user *userstore.User) map[string]any {
	if user == nil {
		return nil
	}
	return map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"role":         user.Role,
		"display_name": user.DisplayName,
		"status":       user.Status,
		"created_at":   user.CreatedAt,
		"updated_at":   user.UpdatedAt,
	}
}

func toAPIKeyPayload(key userstore.APIKey) map[string]any {
	var expires interface{}
	if key.ExpiresAt != nil {
		expires = key.ExpiresAt
	}
	return map[string]any{
		"id":         key.ID,
		"user_id":    key.UserID,
		"prefix":     key.Prefix,
		"scopes":     key.Scopes,
		"expires_at": expires,
		"created_at": key.CreatedAt,
		"updated_at": key.UpdatedAt,
	}
}
