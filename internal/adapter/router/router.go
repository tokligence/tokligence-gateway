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
}

// New creates a new Router instance.
func New() *Router {
	return &Router{
		adapters: make(map[string]adapter.ChatAdapter),
		routes:   make(map[string]string),
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

// CreateCompletion routes the request to the appropriate adapter.
func (r *Router) CreateCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if req.Model == "" {
		return openai.ChatCompletionResponse{}, errors.New("router: model name required")
	}

	adapterName, err := r.findAdapter(req.Model)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}

	r.mu.RLock()
	selectedAdapter, exists := r.adapters[adapterName]
	r.mu.RUnlock()

	if !exists {
		return openai.ChatCompletionResponse{}, fmt.Errorf("router: adapter %q not found", adapterName)
	}

	return selectedAdapter.CreateCompletion(ctx, req)
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
