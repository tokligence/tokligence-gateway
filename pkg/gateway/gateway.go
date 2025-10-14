package gateway

import (
	"github.com/tokligence/tokligence-gateway/internal/client"
	"github.com/tokligence/tokligence-gateway/internal/core"
)

type MarketplaceAPI = core.MarketplaceAPI
type Gateway = core.Gateway

func NewGateway(api MarketplaceAPI) *Gateway {
	return core.NewGateway(api)
}

type Client = client.MarketplaceClient

func NewClient(baseURL string, httpClient client.HTTPClient) (*client.MarketplaceClient, error) {
	return client.NewMarketplaceClient(baseURL, httpClient)
}

type RegisterUserRequest = client.RegisterUserRequest
type RegisterUserResponse = client.RegisterUserResponse
type ProviderProfile = client.ProviderProfile
type ServiceOffering = client.ServiceOffering
type PublishServiceRequest = client.PublishServiceRequest
type UsagePayload = client.UsagePayload
type UsageSummary = client.UsageSummary
type User = client.User
