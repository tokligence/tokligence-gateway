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
}

// PingResponse represents the response from marketplace server.
type PingResponse struct {
	// Version update information
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	UpdateURL       string `json:"update_url,omitempty"`
	UpdateMessage   string `json:"update_message,omitempty"`
	SecurityUpdate  bool   `json:"security_update,omitempty"`

	// Marketplace announcements
	Announcements []Announcement `json:"announcements,omitempty"`

	// Server acknowledgment
	Message string `json:"message,omitempty"`
}

// Announcement represents a marketplace announcement or promotion.
type Announcement struct {
	ID        string     `json:"id"`
	Type      string     `json:"type"` // "promotion", "maintenance", "feature", "provider"
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	URL       string     `json:"url,omitempty"`
	Priority  string     `json:"priority,omitempty"` // "low", "medium", "high", "critical"
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Client sends anonymous telemetry pings to the marketplace and receives updates.
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

// SendPing sends a telemetry ping to the marketplace and returns server response.
// The server may include version update notifications and marketplace announcements.
func (c *Client) SendPing(ctx context.Context, payload PingPayload) (*PingResponse, error) {
	// Set default platform if empty
	if payload.Platform == "" {
		payload.Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := c.baseURL + "/api/v1/gateway/ping"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("Tokligence-Gateway/%s", payload.GatewayVersion))

	c.logger.Printf("sending ping to %s (install_id=%s version=%s)", url, payload.InstallID, payload.GatewayVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var pingResp PingResponse
	if err := json.Unmarshal(bodyBytes, &pingResp); err != nil {
		// Log but don't fail if we can't parse the response
		c.logger.Printf("warning: failed to parse ping response: %v", err)
		return &PingResponse{Message: "ping successful"}, nil
	}

	// Log important information
	if pingResp.UpdateAvailable {
		if pingResp.SecurityUpdate {
			c.logger.Printf("⚠️  SECURITY UPDATE available: %s → %s", payload.GatewayVersion, pingResp.LatestVersion)
		} else {
			c.logger.Printf("✨ Update available: %s → %s", payload.GatewayVersion, pingResp.LatestVersion)
		}
		if pingResp.UpdateMessage != "" {
			c.logger.Printf("   %s", pingResp.UpdateMessage)
		}
		if pingResp.UpdateURL != "" {
			c.logger.Printf("   Download: %s", pingResp.UpdateURL)
		}
	}

	// Log announcements
	for _, ann := range pingResp.Announcements {
		priority := ann.Priority
		if priority == "" {
			priority = "medium"
		}

		prefix := "📢"
		if priority == "critical" || priority == "high" {
			prefix = "⚠️ "
		}

		c.logger.Printf("%s [%s] %s: %s", prefix, ann.Type, ann.Title, ann.Message)
		if ann.URL != "" {
			c.logger.Printf("   Learn more: %s", ann.URL)
		}
	}

	c.logger.Printf("ping successful (status=%d)", resp.StatusCode)
	return &pingResp, nil
}
