package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient abstracts the Do method for easier testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// ExchangeClient communicates with the Token-Exchange MVP.
type ExchangeClient struct {
	baseURL    *url.URL
	httpClient HTTPClient
	logger     *log.Logger
}

// NewExchangeClient constructs a client using the provided base URL.
func NewExchangeClient(baseURL string, httpClient HTTPClient) (*ExchangeClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &ExchangeClient{
		baseURL:    parsed,
		httpClient: httpClient,
		logger:     log.New(log.Writer(), "[mfg/http] ", log.LstdFlags|log.Lmicroseconds),
	}, nil
}

// SetLogger overrides the default logger; nil keeps the current logger.
func (c *ExchangeClient) SetLogger(logger *log.Logger) {
	if logger != nil {
		c.logger = logger
	}
}

func (c *ExchangeClient) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

// RegisterUserRequest mirrors the Token Exchange payload.
type RegisterUserRequest struct {
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description,omitempty"`
}

// RegisterUserResponse captures user + optional provider.
type RegisterUserResponse struct {
	User     User             `json:"user"`
	Provider *ProviderProfile `json:"provider,omitempty"`
}

// User describes a marketplace account.
type User struct {
	ID    int64    `json:"id"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// ProviderProfile represents provider metadata.
type ProviderProfile struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// ServiceOffering corresponds to Token Exchange services.
type ServiceOffering struct {
	ID               int64   `json:"id"`
	ProviderID       int64   `json:"provider_id"`
	Name             string  `json:"name"`
	ModelFamily      string  `json:"model_family"`
	PricePer1KTokens float64 `json:"price_per_1k_tokens"`
	TrialTokens      int64   `json:"trial_tokens"`
}

// PublishServiceRequest describes service creation payload.
type PublishServiceRequest struct {
	ProviderID       int64   `json:"provider_id"`
	Name             string  `json:"name"`
	ModelFamily      string  `json:"model_family"`
	PricePer1KTokens float64 `json:"price_per_1k_tokens"`
	TrialTokens      int64   `json:"trial_tokens"`
}

// UsagePayload is shared for reporting usage to Token Exchange.
type UsagePayload struct {
	UserID           int64  `json:"user_id"`
	ServiceID        int64  `json:"service_id"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	Direction        string `json:"direction"`
	Memo             string `json:"memo,omitempty"`
}

// UsageSummary response from Token Exchange.
type UsageSummary struct {
	UserID         int64 `json:"user_id"`
	ConsumedTokens int64 `json:"consumed_tokens"`
	SuppliedTokens int64 `json:"supplied_tokens"`
	NetTokens      int64 `json:"net_tokens"`
}

// errorResponse matches the standard error payload.
type errorResponse struct {
	Error string `json:"error"`
}

func (c *ExchangeClient) post(ctx context.Context, path string, payload any, out any) error {
	return c.doJSON(ctx, http.MethodPost, path, payload, out)
}

func (c *ExchangeClient) get(ctx context.Context, path string, out any) error {
	return c.doJSON(ctx, http.MethodGet, path, nil, out)
}

func (c *ExchangeClient) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}

	rel, err := url.Parse(path)
	if err != nil {
		return err
	}
	endpoint := c.baseURL.ResolveReference(rel)

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	c.logf("http_request method=%s url=%s", method, endpoint.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logf("http_error method=%s url=%s err=%v", method, endpoint.String(), err)
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	c.logf("http_response method=%s url=%s status=%d duration=%s", method, endpoint.String(), resp.StatusCode, duration)

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		var errPayload errorResponse
		if err := json.Unmarshal(data, &errPayload); err == nil && strings.TrimSpace(errPayload.Error) != "" {
			err = fmt.Errorf("token-exchange error: %s", errPayload.Error)
			c.logf("http_error method=%s url=%s status=%d err=%v", method, endpoint.String(), resp.StatusCode, err)
			return err
		}
		err = fmt.Errorf("token-exchange error: status %d", resp.StatusCode)
		c.logf("http_error method=%s url=%s status=%d err=%v", method, endpoint.String(), resp.StatusCode, err)
		return err
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			c.logf("decode_error method=%s url=%s err=%v", method, endpoint.String(), err)
			return err
		}
	}
	return nil
}

// RegisterUser registers a user.
func (c *ExchangeClient) RegisterUser(ctx context.Context, req RegisterUserRequest) (RegisterUserResponse, error) {
	var resp RegisterUserResponse
	if err := c.post(ctx, "/users", req, &resp); err != nil {
		return RegisterUserResponse{}, err
	}
	c.logf("register_user success user_id=%d", resp.User.ID)
	return resp, nil
}

// ListProviders retrieves provider catalogue.
func (c *ExchangeClient) ListProviders(ctx context.Context) ([]ProviderProfile, error) {
	var resp struct {
		Providers []ProviderProfile `json:"providers"`
	}
	if err := c.get(ctx, "/providers", &resp); err != nil {
		return nil, err
	}
	c.logf("list_providers success count=%d", len(resp.Providers))
	return resp.Providers, nil
}

// PublishService publishes a service offering.
func (c *ExchangeClient) PublishService(ctx context.Context, req PublishServiceRequest) (ServiceOffering, error) {
	var resp struct {
		Service ServiceOffering `json:"service"`
	}
	if err := c.post(ctx, "/services", req, &resp); err != nil {
		return ServiceOffering{}, err
	}
	c.logf("publish_service success service_id=%d", resp.Service.ID)
	return resp.Service, nil
}

// ListServices retrieves offerings (optionally filtered by provider).
func (c *ExchangeClient) ListServices(ctx context.Context, providerID *int64) ([]ServiceOffering, error) {
	path := "/services"
	if providerID != nil {
		path = fmt.Sprintf("/services?provider_id=%d", *providerID)
	}
	var resp struct {
		Services []ServiceOffering `json:"services"`
	}
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	c.logf("list_services success provider_id=%v count=%d", providerOrAll(providerID), len(resp.Services))
	return resp.Services, nil
}

// ReportUsage posts token usage data.
func (c *ExchangeClient) ReportUsage(ctx context.Context, payload UsagePayload) error {
	if err := c.post(ctx, "/usage", payload, nil); err != nil {
		return err
	}
	c.logf("report_usage success user_id=%d service_id=%d direction=%s", payload.UserID, payload.ServiceID, payload.Direction)
	return nil
}

// GetUsageSummary fetches per-user token summary.
func (c *ExchangeClient) GetUsageSummary(ctx context.Context, userID int64) (UsageSummary, error) {
	var resp struct {
		Summary UsageSummary `json:"summary"`
	}
	if err := c.get(ctx, fmt.Sprintf("/usage/summary?user_id=%d", userID), &resp); err != nil {
		return UsageSummary{}, err
	}
	c.logf("usage_summary success user_id=%d", userID)
	return resp.Summary, nil
}

func providerOrAll(providerID *int64) any {
	if providerID == nil {
		return "<all>"
	}
	return *providerID
}
