package core

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/client"
)

type fakeExchange struct {
	lastUsage client.UsagePayload
}

func (f *fakeExchange) RegisterUser(ctx context.Context, req client.RegisterUserRequest) (client.RegisterUserResponse, error) {
	return client.RegisterUserResponse{User: client.User{ID: 42, Email: req.Email, Roles: req.Roles}, Provider: &client.ProviderProfile{ID: 7, UserID: 42, DisplayName: req.DisplayName}}, nil
}

func (f *fakeExchange) ListProviders(ctx context.Context) ([]client.ProviderProfile, error) {
	return []client.ProviderProfile{{ID: 11, DisplayName: "Anthropic"}, {ID: 12, DisplayName: "OpenAI"}}, nil
}

func (f *fakeExchange) PublishService(ctx context.Context, req client.PublishServiceRequest) (client.ServiceOffering, error) {
	if req.ProviderID == 0 {
		return client.ServiceOffering{}, errors.New("provider missing")
	}
	return client.ServiceOffering{ID: 99, ProviderID: req.ProviderID, Name: req.Name}, nil
}

func (f *fakeExchange) ListServices(ctx context.Context, providerID *int64) ([]client.ServiceOffering, error) {
	if providerID == nil {
		return nil, nil
	}
	return []client.ServiceOffering{{ID: 1, ProviderID: *providerID, Name: "claude"}}, nil
}

func (f *fakeExchange) ReportUsage(ctx context.Context, payload client.UsagePayload) error {
	f.lastUsage = payload
	return nil
}

func (f *fakeExchange) GetUsageSummary(ctx context.Context, userID int64) (client.UsageSummary, error) {
	return client.UsageSummary{UserID: userID, ConsumedTokens: 200, SuppliedTokens: 50, NetTokens: -150}, nil
}

func TestGatewayLifecycle(t *testing.T) {
	fx := &fakeExchange{}
	gw := NewGateway(fx)
	gw.SetLogger(log.New(io.Discard, "", 0))

	ctx := context.Background()
	user, provider, err := gw.EnsureAccount(ctx, "agent@example.com", []string{"provider", "consumer"}, "Agent")
	if err != nil {
		t.Fatalf("EnsureAccount: %v", err)
	}
	if user == nil || provider == nil {
		t.Fatalf("expected provider account")
	}

	chosen, err := gw.ChooseProvider(ctx, func(p client.ProviderProfile) bool { return p.DisplayName == "Anthropic" })
	if err != nil {
		t.Fatalf("ChooseProvider: %v", err)
	}
	if chosen.ID != 11 {
		t.Fatalf("unexpected provider %v", chosen)
	}

	svc, err := gw.PublishService(ctx, client.PublishServiceRequest{Name: "local-claude", ModelFamily: "claude", PricePer1KTokens: 0.6})
	if err != nil {
		t.Fatalf("PublishService: %v", err)
	}
	if svc.ProviderID != provider.ID {
		t.Fatalf("service provider mismatch")
	}

	if err := gw.RecordConsumption(ctx, svc.ID, 100, 50); err != nil {
		t.Fatalf("RecordConsumption: %v", err)
	}
	if fx.lastUsage.Direction != "consume" {
		t.Fatalf("expected consume direction, got %s", fx.lastUsage.Direction)
	}

	if err := gw.RecordSupply(ctx, svc.ID, 130, 70); err != nil {
		t.Fatalf("RecordSupply: %v", err)
	}
	if fx.lastUsage.Direction != "supply" {
		t.Fatalf("expected supply direction")
	}

	summary, err := gw.UsageSnapshot(ctx)
	if err != nil {
		t.Fatalf("UsageSnapshot: %v", err)
	}
	if summary.ConsumedTokens != 200 || summary.NetTokens != -150 {
		t.Fatalf("unexpected summary %#v", summary)
	}
}
