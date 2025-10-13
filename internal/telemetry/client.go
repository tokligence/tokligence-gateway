package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"time"
)

// PingPayload represents the telemetry data sent to marketplace.
type PingPayload struct {
	InstallID      string `json:"install_id"`
	GatewayVersion string `json:"gateway_version"`
	Platform       string `json:"platform"`
	DatabaseType   string `json:"database_type"`
	TotalAPICalls  int64  `json:"total_api_calls"`
	UniqueModels   int64  `json:"unique_models"`
}

// Client sends anonymous telemetry pings to the marketplace.
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *log.Logger
}

// NewClient creates a new telemetry client.
func NewClient(baseURL string, logger *log.Logger) *Client {
	if logger == nil {
		logger = log.New(log.Writer(), "[telemetry] ", log.LstdFlags|log.Lmicroseconds)
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// SendPing sends a telemetry ping to the marketplace.
func (c *Client) SendPing(ctx context.Context, payload PingPayload) error {
	// Set default platform if empty
	if payload.Platform == "" {
		payload.Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := c.baseURL + "/telemetry"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("Tokligence-Gateway/%s", payload.GatewayVersion))

	c.logger.Printf("sending telemetry ping to %s (install_id=%s version=%s)", url, payload.InstallID, payload.GatewayVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	c.logger.Printf("telemetry ping successful (status=%d)", resp.StatusCode)
	return nil
}
