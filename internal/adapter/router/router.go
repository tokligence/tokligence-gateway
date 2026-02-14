package router

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// Router routes requests to the appropriate adapter based on model name.
type Router struct {
	mu       sync.RWMutex
	adapters map[string]adapter.ChatAdapter
	routes   map[string]string // model pattern -> adapter name
	fallback adapter.ChatAdapter
	aliases  map[string]string // model pattern -> target model id
}

// New creates a new Router instance.
func New() *Router {
	return &Router{
		adapters: make(map[string]adapter.ChatAdapter),
		routes:   make(map[string]string),
		aliases:  make(map[string]string),
	}
}

// RegisterAdapter registers an adapter with a name.
func (r *Router) RegisterAdapter(name string, adapter adapter.ChatAdapter) error {
	if name == "" {
		return errors.New("router: adapter name cannot be empty")
	}
	if adapter == nil {
		return errors.New("router: adapter cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.adapters[name] = adapter
	return nil
}

// RegisterRoute registers a model pattern to adapter mapping.
// Model patterns support:
// - Exact match: "gpt-4"
// - Prefix match: "gpt-*" (matches gpt-4, gpt-3.5-turbo, etc.)
// - Suffix match: "*-turbo" (matches gpt-3.5-turbo, etc.)
func (r *Router) RegisterRoute(modelPattern, adapterName string) error {
	if modelPattern == "" {
		return errors.New("router: model pattern cannot be empty")
	}
	if adapterName == "" {
		return errors.New("router: adapter name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[adapterName]; !exists {
		return fmt.Errorf("router: adapter %q not registered", adapterName)
	}

	r.routes[modelPattern] = adapterName
	return nil
}

// SetFallback sets a fallback adapter for unmatched models.
func (r *Router) SetFallback(adapter adapter.ChatAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fallback = adapter
}

// RegisterAlias registers a model alias mapping: incoming model pattern -> rewritten target model.
// Example: "claude-*" => "gpt-4o"
func (r *Router) RegisterAlias(modelPattern, target string) error {
	if strings.TrimSpace(modelPattern) == "" {
		return errors.New("router: alias pattern cannot be empty")
	}
	if strings.TrimSpace(target) == "" {
		return errors.New("router: alias target cannot be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aliases[modelPattern] = target
	return nil
}

// SetAliases replaces all alias rules atomically.
// Passing nil clears all aliases.
func (r *Router) SetAliases(aliases map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if aliases == nil {
		r.aliases = make(map[string]string)
		return
	}
	// copy to avoid external mutation
	fresh := make(map[string]string, len(aliases))
	for k, v := range aliases {
		fresh[k] = v
	}
	r.aliases = fresh
}

// CreateCompletion routes the request to the appropriate adapter.
func (r *Router) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if req.Model == "" {
		return openai.ChatCompletionResponse{}, errors.New("router: model name required")
	}
	// Choose adapter based on the original incoming model name
	originalModel := req.Model
	adapterName, err := r.findAdapter(originalModel)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	// Apply alias rewrite after selecting adapter (provider-specific model IDs)
	req.Model = r.rewriteModel(originalModel)

	r.mu.RLock()
	selectedAdapter, exists := r.adapters[adapterName]
	r.mu.RUnlock()

	if !exists {
		return openai.ChatCompletionResponse{}, fmt.Errorf("router: adapter %q not found", adapterName)
	}

	return selectedAdapter.CreateCompletion(ctx, req)
}

// CreateCompletionStream forwards streaming requests to the underlying streaming-capable adapter.
func (r *Router) CreateCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (<-chan adapter.StreamEvent, error) {
	if req.Model == "" {
		return nil, errors.New("router: model name required")
	}
	originalModel := req.Model
	name, err := r.findAdapter(originalModel)
	if err != nil {
		return nil, err
	}
	req.Model = r.rewriteModel(originalModel)
	r.mu.RLock()
	a, ok := r.adapters[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("router: adapter %q not found", name)
	}
	sa, ok := a.(adapter.StreamingChatAdapter)
	if !ok {
		return nil, errors.New("router: selected adapter does not support streaming")
	}
	return sa.CreateCompletionStream(ctx, req)
}

// findAdapter finds the appropriate adapter for a given model.
func (r *Router) findAdapter(model string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model = strings.ToLower(strings.TrimSpace(model))

	// Try exact match first
	if adapterName, exists := r.routes[model]; exists {
		return adapterName, nil
	}

	// Try pattern matching
	for pattern, adapterName := range r.routes {
		if matchPattern(model, pattern) {
			return adapterName, nil
		}
	}

	// Use fallback if available
	if r.fallback != nil {
		// Find fallback adapter name
		for name, adapter := range r.adapters {
			if adapter == r.fallback {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("router: no adapter found for model %q", model)
}

// matchPattern checks if a model matches a pattern.
// Supports:
// - Exact match: "gpt-4"
// - Prefix match: "gpt-*"
// - Suffix match: "*-turbo"
// - Contains match: "*3.5*"
func matchPattern(model, pattern string) bool {
	model = strings.ToLower(model)
	pattern = strings.ToLower(pattern)

	// Exact match
	if model == pattern {
		return true
	}

	// No wildcards
	if !strings.Contains(pattern, "*") {
		return false
	}

	// Prefix match: "gpt-*"
	if strings.HasSuffix(pattern, "*") && !strings.HasPrefix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(model, prefix)
	}

	// Suffix match: "*-turbo"
	if strings.HasPrefix(pattern, "*") && !strings.HasSuffix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(model, suffix)
	}

	// Contains match: "*3.5*"
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		contains := strings.Trim(pattern, "*")
		return strings.Contains(model, contains)
	}

	return false
}

// GetAdapterForModel returns the adapter name for a given model (for debugging).
func (r *Router) GetAdapterForModel(model string) (string, error) {
	return r.findAdapter(model)
}

// ListAdapters returns all registered adapter names.
func (r *Router) ListAdapters() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// ListRoutes returns all registered routes.
func (r *Router) ListRoutes() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make(map[string]string, len(r.routes))
	for pattern, adapter := range r.routes {
		routes[pattern] = adapter
	}
	return routes
}

// Ensure Router satisfies additional interfaces when possible.
var _ adapter.ChatAdapter = (*Router)(nil)
var _ adapter.StreamingChatAdapter = (*Router)(nil)

// Embedding support: choose adapter by model pattern if underlying adapter supports it.
func (r *Router) CreateEmbedding(ctx context.Context, req openai.EmbeddingRequest) (openai.EmbeddingResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return openai.EmbeddingResponse{}, errors.New("router: model name required for embeddings")
	}
	originalModel := req.Model
	name, err := r.findAdapter(originalModel)
	if err != nil {
		return openai.EmbeddingResponse{}, err
	}
	req.Model = r.rewriteModel(originalModel)
	r.mu.RLock()
	a := r.adapters[name]
	r.mu.RUnlock()
	ea, ok := a.(adapter.EmbeddingAdapter)
	if !ok {
		return openai.EmbeddingResponse{}, errors.New("router: selected adapter does not support embeddings")
	}
	return ea.CreateEmbedding(ctx, req)
}

var _ adapter.EmbeddingAdapter = (*Router)(nil)

// ListAliases returns all registered alias rules.
func (r *Router) ListAliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.aliases))
	for k, v := range r.aliases {
		out[k] = v
	}
	return out
}

// rewriteModel applies alias rules to the provided model id.
func (r *Router) rewriteModel(model string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := strings.ToLower(strings.TrimSpace(model))
	if target, ok := r.aliases[m]; ok {
		return target
	}
	for pattern, target := range r.aliases {
		if matchPattern(m, strings.ToLower(pattern)) {
			return target
		}
	}
	return model
}

// RewriteModelPublic exposes model alias rewriting for external consumers
// (e.g., HTTP server bridges) without importing router internals.
func (r *Router) RewriteModelPublic(model string) string {
	return r.rewriteModel(model)
}
