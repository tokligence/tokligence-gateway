package core

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/hooks"
)

// ErrExchangeUnavailable signals that the Tokligence Exchange is disabled or unreachable.
var ErrExchangeUnavailable = errors.New("tokligence exchange unavailable")

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
	exchange          ExchangeAPI
	user              *client.User
	provider          *client.ProviderProfile
	logger            *log.Logger
	hooks             *hooks.Dispatcher
	exchangeAvailable bool
}

// NewGateway creates a new Gateway instance.
func NewGateway(exchange ExchangeAPI) *Gateway {
	return &Gateway{
		exchange: exchange,
		logger:   log.New(log.Writer(), "[tokligence/gateway] ", log.LstdFlags|log.Lmicroseconds),
	}
}

// SetLogger overrides the default logger; nil keeps the current logger.
func (g *Gateway) SetLogger(logger *log.Logger) {
	if logger != nil {
		g.logger = logger
	}
}

// SetHooksDispatcher attaches a hook dispatcher. Passing nil disables hook emission.
func (g *Gateway) SetHooksDispatcher(dispatcher *hooks.Dispatcher) {
	g.hooks = dispatcher
}

// SetLocalAccount injects a locally managed user/provider pair when the exchange is unavailable.
func (g *Gateway) SetLocalAccount(user client.User, provider *client.ProviderProfile) {
	u := user
	g.user = &u
	g.provider = provider
}

// SetMarketplaceAvailable toggles the cached marketplace availability flag.
func (g *Gateway) SetMarketplaceAvailable(ok bool) {
	g.exchangeAvailable = ok
}

// MarketplaceAvailable reports whether the last exchange interaction succeeded.
func (g *Gateway) MarketplaceAvailable() bool {
	return g.exchangeAvailable
}

func (g *Gateway) logf(format string, args ...any) {
	if g.logger != nil {
		g.logger.Printf(format, args...)
	}
}

func (g *Gateway) emitUserHook(ctx context.Context, prev *client.User, current client.User) error {
	if g.hooks == nil {
		return nil
	}
	var eventType hooks.EventType
	if prev == nil || prev.ID == 0 {
		eventType = hooks.EventUserProvisioned
	} else {
		eventType = hooks.EventUserUpdated
	}
	metadata := map[string]any{
		"email": current.Email,
		"roles": current.Roles,
	}
	event := hooks.Event{
		ID:         uuid.NewString(),
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		UserID:     strconv.FormatInt(current.ID, 10),
		ActorID:    strconv.FormatInt(current.ID, 10),
		Metadata:   metadata,
	}
	return g.hooks.Emit(ctx, event)
}

// EnsureAccount registers the user (if not already registered).
func (g *Gateway) EnsureAccount(ctx context.Context, email string, roles []string, displayName string) (*client.User, *client.ProviderProfile, error) {
	if strings.TrimSpace(email) == "" {
		err := errors.New("email is required")
		g.logf("ensure_account failed: %v", err)
		return nil, nil, err
	}
	if g.exchange == nil {
		return nil, nil, ErrExchangeUnavailable
	}
	g.logf("ensure_account start email=%s roles=%v", email, roles)
	prev := g.user
	resp, err := g.exchange.RegisterUser(ctx, client.RegisterUserRequest{
		Email:       email,
		Roles:       roles,
		DisplayName: displayName,
	})
	if err != nil {
		g.logf("ensure_account error: %v", err)
		g.exchangeAvailable = false
		return nil, nil, err
	}
	g.user = &resp.User
	g.provider = resp.Provider
	g.exchangeAvailable = true
	if g.provider != nil {
		g.logf("ensure_account success user_id=%d provider_id=%d", g.user.ID, g.provider.ID)
	} else {
		g.logf("ensure_account success user_id=%d provider_id=<none>", g.user.ID)
	}
	if err := g.emitUserHook(ctx, prev, resp.User); err != nil {
		g.logf("ensure_account hook error: %v", err)
	}
	return g.user, g.provider, nil
}

// ListProviders proxies catalogue retrieval.
func (g *Gateway) ListProviders(ctx context.Context) ([]client.ProviderProfile, error) {
	if g.exchange == nil {
		return nil, ErrExchangeUnavailable
	}
	providers, err := g.exchange.ListProviders(ctx)
	if err != nil {
		g.logf("list_providers error: %v", err)
		return nil, err
	}
	g.logf("list_providers success count=%d", len(providers))
	return providers, nil
}

// ChooseProvider selects a provider by predicate.
func (g *Gateway) ChooseProvider(ctx context.Context, match func(client.ProviderProfile) bool) (*client.ProviderProfile, error) {
	providers, err := g.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if match(p) {
			g.logf("choose_provider success provider_id=%d", p.ID)
			return &p, nil
		}
	}
	err = errors.New("no provider matched the criteria")
	g.logf("choose_provider error: %v", err)
	return nil, err
}

// PublishService publishes a local service under the authenticated provider.
func (g *Gateway) PublishService(ctx context.Context, req client.PublishServiceRequest) (client.ServiceOffering, error) {
	if g.provider == nil {
		err := errors.New("gateway is not registered as provider")
		g.logf("publish_service error: %v", err)
		return client.ServiceOffering{}, err
	}
	if g.exchange == nil {
		return client.ServiceOffering{}, ErrExchangeUnavailable
	}
	req.ProviderID = g.provider.ID
	g.logf("publish_service start provider_id=%d name=%s", req.ProviderID, req.Name)
	service, err := g.exchange.PublishService(ctx, req)
	if err != nil {
		g.logf("publish_service error: %v", err)
		return client.ServiceOffering{}, err
	}
	g.logf("publish_service success service_id=%d", service.ID)
	return service, nil
}

// ListMyServices lists services for the authenticated provider.
func (g *Gateway) ListMyServices(ctx context.Context) ([]client.ServiceOffering, error) {
	if g.provider == nil {
		err := errors.New("gateway is not registered as provider")
		g.logf("list_my_services error: %v", err)
		return nil, err
	}
	if g.exchange == nil {
		return nil, ErrExchangeUnavailable
	}
	services, err := g.exchange.ListServices(ctx, &g.provider.ID)
	if err != nil {
		g.logf("list_my_services error: %v", err)
		return nil, err
	}
	g.logf("list_my_services success count=%d", len(services))
	return services, nil
}

// ListServices exposes the underlying exchange service catalogue.
func (g *Gateway) ListServices(ctx context.Context, providerID *int64) ([]client.ServiceOffering, error) {
	if g.exchange == nil {
		return nil, ErrExchangeUnavailable
	}
	services, err := g.exchange.ListServices(ctx, providerID)
	if err != nil {
		g.logf("list_services error: %v", err)
		return nil, err
	}
	g.logf("list_services success provider_id=%v count=%d", providerOrSelf(providerID), len(services))
	return services, nil
}

// RecordConsumption reports consumed tokens for the current user.
func (g *Gateway) RecordConsumption(ctx context.Context, serviceID int64, promptTokens, completionTokens int64) error {
	if g.user == nil {
		err := errors.New("gateway has no authenticated user")
		g.logf("record_consumption error: %v", err)
		return err
	}
	if g.exchange == nil {
		return ErrExchangeUnavailable
	}
	g.logf("record_consumption start user_id=%d service_id=%d", g.user.ID, serviceID)
	if err := g.exchange.ReportUsage(ctx, client.UsagePayload{
		UserID:           g.user.ID,
		ServiceID:        serviceID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Direction:        "consume",
	}); err != nil {
		g.logf("record_consumption error: %v", err)
		return err
	}
	g.logf("record_consumption success user_id=%d service_id=%d", g.user.ID, serviceID)
	return nil
}

// RecordSupply reports supplied tokens when selling through the gateway.
func (g *Gateway) RecordSupply(ctx context.Context, serviceID int64, promptTokens, completionTokens int64) error {
	if g.user == nil {
		err := errors.New("gateway has no authenticated user")
		g.logf("record_supply error: %v", err)
		return err
	}
	if g.exchange == nil {
		return ErrExchangeUnavailable
	}
	g.logf("record_supply start user_id=%d service_id=%d", g.user.ID, serviceID)
	if err := g.exchange.ReportUsage(ctx, client.UsagePayload{
		UserID:           g.user.ID,
		ServiceID:        serviceID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Direction:        "supply",
	}); err != nil {
		g.logf("record_supply error: %v", err)
		return err
	}
	g.logf("record_supply success user_id=%d service_id=%d", g.user.ID, serviceID)
	return nil
}

// UsageSnapshot retrieves aggregate usage for the current user.
func (g *Gateway) UsageSnapshot(ctx context.Context) (client.UsageSummary, error) {
	if g.user == nil {
		err := errors.New("gateway has no authenticated user")
		g.logf("usage_snapshot error: %v", err)
		return client.UsageSummary{}, err
	}
	if g.exchange == nil {
		return client.UsageSummary{}, ErrExchangeUnavailable
	}
	summary, err := g.exchange.GetUsageSummary(ctx, g.user.ID)
	if err != nil {
		g.logf("usage_snapshot error: %v", err)
		return client.UsageSummary{}, err
	}
	g.logf("usage_snapshot success user_id=%d net=%d", g.user.ID, summary.NetTokens)
	return summary, nil
}

// Account returns the cached user/provider references.
func (g *Gateway) Account() (*client.User, *client.ProviderProfile) {
	return g.user, g.provider
}

func providerOrSelf(providerID *int64) any {
	if providerID == nil {
		return "<all>"
	}
	return *providerID
}
