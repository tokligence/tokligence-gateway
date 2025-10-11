package core

import (
	"context"
	"errors"
	"strings"

	"github.com/tokenstreaming/model-free-gateway/internal/client"
)

// ExchangeAPI defines the contract Gateway expects from the Token Exchange client.
type ExchangeAPI interface {
	RegisterUser(ctx context.Context, req client.RegisterUserRequest) (client.RegisterUserResponse, error)
	ListProviders(ctx context.Context) ([]client.ProviderProfile, error)
	PublishService(ctx context.Context, req client.PublishServiceRequest) (client.ServiceOffering, error)
	ListServices(ctx context.Context, providerID *int64) ([]client.ServiceOffering, error)
	ReportUsage(ctx context.Context, payload client.UsagePayload) error
	GetUsageSummary(ctx context.Context, userID int64) (client.UsageSummary, error)
}

// Gateway encapsulates interactions with Token Exchange for the MFA client.
type Gateway struct {
	exchange ExchangeAPI
	user     *client.User
	provider *client.ProviderProfile
}

// NewGateway creates a new Gateway instance.
func NewGateway(exchange ExchangeAPI) *Gateway {
	return &Gateway{exchange: exchange}
}

// EnsureAccount registers the user (if not already registered).
func (g *Gateway) EnsureAccount(ctx context.Context, email string, roles []string, displayName string) (*client.User, *client.ProviderProfile, error) {
	if strings.TrimSpace(email) == "" {
		return nil, nil, errors.New("email is required")
	}
	resp, err := g.exchange.RegisterUser(ctx, client.RegisterUserRequest{
		Email:       email,
		Roles:       roles,
		DisplayName: displayName,
	})
	if err != nil {
		return nil, nil, err
	}
	g.user = &resp.User
	g.provider = resp.Provider
	return g.user, g.provider, nil
}

// ListProviders proxies catalogue retrieval.
func (g *Gateway) ListProviders(ctx context.Context) ([]client.ProviderProfile, error) {
	return g.exchange.ListProviders(ctx)
}

// ChooseProvider selects a provider by predicate.
func (g *Gateway) ChooseProvider(ctx context.Context, match func(client.ProviderProfile) bool) (*client.ProviderProfile, error) {
	providers, err := g.exchange.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if match(p) {
			return &p, nil
		}
	}
	return nil, errors.New("no provider matched the criteria")
}

// PublishService publishes a local service under the authenticated provider.
func (g *Gateway) PublishService(ctx context.Context, req client.PublishServiceRequest) (client.ServiceOffering, error) {
	if g.provider == nil {
		return client.ServiceOffering{}, errors.New("gateway is not registered as provider")
	}
	req.ProviderID = g.provider.ID
	return g.exchange.PublishService(ctx, req)
}

// ListMyServices lists services for the authenticated provider.
func (g *Gateway) ListMyServices(ctx context.Context) ([]client.ServiceOffering, error) {
	if g.provider == nil {
		return nil, errors.New("gateway is not registered as provider")
	}
	return g.exchange.ListServices(ctx, &g.provider.ID)
}

// RecordConsumption reports consumed tokens for the current user.
func (g *Gateway) RecordConsumption(ctx context.Context, serviceID int64, promptTokens, completionTokens int64) error {
	if g.user == nil {
		return errors.New("gateway has no authenticated user")
	}
	return g.exchange.ReportUsage(ctx, client.UsagePayload{
		UserID:           g.user.ID,
		ServiceID:        serviceID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Direction:        "consume",
	})
}

// RecordSupply reports supplied tokens when selling through the gateway.
func (g *Gateway) RecordSupply(ctx context.Context, serviceID int64, promptTokens, completionTokens int64) error {
	if g.user == nil {
		return errors.New("gateway has no authenticated user")
	}
	return g.exchange.ReportUsage(ctx, client.UsagePayload{
		UserID:           g.user.ID,
		ServiceID:        serviceID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Direction:        "supply",
	})
}

// UsageSnapshot retrieves aggregate usage for the current user.
func (g *Gateway) UsageSnapshot(ctx context.Context) (client.UsageSummary, error) {
	if g.user == nil {
		return client.UsageSummary{}, errors.New("gateway has no authenticated user")
	}
	return g.exchange.GetUsageSummary(ctx, g.user.ID)
}

// Account returns the cached user/provider references.
func (g *Gateway) Account() (*client.User, *client.ProviderProfile) {
	return g.user, g.provider
}
