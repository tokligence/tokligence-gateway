package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/tokenstreaming/model-free-gateway/internal/client"
	"github.com/tokenstreaming/model-free-gateway/internal/core"
)

func main() {
	baseURL := os.Getenv("TOKEN_EXCHANGE_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	httpClient, err := client.NewExchangeClient(baseURL, nil)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	gateway := core.NewGateway(httpClient)
	ctx := context.Background()

	roles := []string{"consumer"}
	if strings.EqualFold(os.Getenv("MFG_ENABLE_PROVIDER"), "true") {
		roles = append(roles, "provider")
	}

	user, provider, err := gateway.EnsureAccount(ctx, os.Getenv("MFG_EMAIL"), roles, os.Getenv("MFG_DISPLAY_NAME"))
	if err != nil {
		log.Fatalf("ensure account failed: %v", err)
	}

	log.Printf("connected as user %d (%s) roles=%v", user.ID, user.Email, user.Roles)
	if provider != nil {
		log.Printf("provider profile %d (%s)", provider.ID, provider.DisplayName)

		if os.Getenv("MFG_PUBLISH_NAME") != "" {
			svc, err := gateway.PublishService(ctx, client.PublishServiceRequest{
				Name:             os.Getenv("MFG_PUBLISH_NAME"),
				ModelFamily:      os.Getenv("MFG_MODEL_FAMILY"),
				PricePer1KTokens: 0.5,
			})
			if err != nil {
				log.Fatalf("publish service failed: %v", err)
			}
			log.Printf("published service %d (%s)", svc.ID, svc.Name)
		}
	}

	summary, err := gateway.UsageSnapshot(ctx)
	if err != nil {
		log.Printf("usage snapshot unavailable: %v", err)
	} else {
		log.Printf("usage summary consumed=%d supplied=%d net=%d", summary.ConsumedTokens, summary.SuppliedTokens, summary.NetTokens)
	}
}
