package gateway

import (
	"github.com/tokenstreaming/model-free-gateway/internal/client"
	"github.com/tokenstreaming/model-free-gateway/internal/core"
)

type ExchangeAPI = core.ExchangeAPI
type Gateway = core.Gateway

func NewGateway(api ExchangeAPI) *Gateway {
	return core.NewGateway(api)
}

type Client = client.ExchangeClient

func NewClient(baseURL string, httpClient client.HTTPClient) (*client.ExchangeClient, error) {
	return client.NewExchangeClient(baseURL, httpClient)
}

type RegisterUserRequest = client.RegisterUserRequest
type RegisterUserResponse = client.RegisterUserResponse
type ProviderProfile = client.ProviderProfile
type ServiceOffering = client.ServiceOffering
type PublishServiceRequest = client.PublishServiceRequest
type UsagePayload = client.UsagePayload
type UsageSummary = client.UsageSummary
type User = client.User
