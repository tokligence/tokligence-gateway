package firewall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPFilterConfig configures an HTTP-based external filter service.
type HTTPFilterConfig struct {
	Name      string
	Priority  int
	Direction Direction
	Endpoint  string            // POST endpoint URL
	Timeout   time.Duration     // Request timeout
	Headers   map[string]string // Custom headers
	OnError   ErrorAction       // What to do on service error
}

// ErrorAction defines behavior when external service fails.
type ErrorAction string

const (
	ErrorActionAllow  ErrorAction = "allow"  // Allow request to proceed
	ErrorActionBlock  ErrorAction = "block"  // Block request
	ErrorActionBypass ErrorAction = "bypass" // Skip this filter
)

// HTTPFilterRequest is the payload sent to external filter services.
type HTTPFilterRequest struct {
	Input      string         `json:"input,omitempty"`
	Output     string         `json:"output,omitempty"`
	Model      string         `json:"model,omitempty"`
	Endpoint   string         `json:"endpoint,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	TenantID   string         `json:"tenant_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// HTTPFilterResponse is the response expected from external filter services.
type HTTPFilterResponse struct {
	Allowed         bool                   `json:"allowed"`
	Block           bool                   `json:"block"` // Alternative to !allowed
	BlockReason     string                 `json:"block_reason,omitempty"`
	RedactedInput   string                 `json:"redacted_input,omitempty"`
	RedactedOutput  string                 `json:"redacted_output,omitempty"`
	Detections      []Detection            `json:"detections,omitempty"`
	Entities        []RedactedEntity       `json:"entities,omitempty"`
	Annotations     map[string]any         `json:"annotations,omitempty"`
}

// HTTPFilter calls an external HTTP service for filtering.
type HTTPFilter struct {
	name      string
	priority  int
	direction Direction
	endpoint  string
	timeout   time.Duration
	headers   map[string]string
	onError   ErrorAction
	client    *http.Client
}

// NewHTTPFilter creates a new HTTP-based filter.
func NewHTTPFilter(config HTTPFilterConfig) *HTTPFilter {
	if config.Name == "" {
		config.Name = "http_filter"
	}
	if config.Direction == "" {
		config.Direction = DirectionBoth
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.OnError == "" {
		config.OnError = ErrorActionBypass
	}

	return &HTTPFilter{
		name:      config.Name,
		priority:  config.Priority,
		direction: config.Direction,
		endpoint:  config.Endpoint,
		timeout:   config.Timeout,
		headers:   config.Headers,
		onError:   config.OnError,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

func (f *HTTPFilter) Name() string {
	return f.name
}

func (f *HTTPFilter) Priority() int {
	return f.priority
}

func (f *HTTPFilter) Direction() Direction {
	return f.direction
}

func (f *HTTPFilter) ApplyInput(ctx *FilterContext) error {
	req := HTTPFilterRequest{
		Input:     string(ctx.RequestBody),
		Model:     ctx.RequestModel,
		Endpoint:  ctx.Endpoint,
		UserID:    ctx.UserID,
		TenantID:  ctx.TenantID,
		SessionID: ctx.SessionID,
		Metadata:  ctx.Metadata,
	}

	resp, err := f.callService(ctx.Context, req)
	if err != nil {
		return f.handleError(ctx, err)
	}

	f.applyResponse(ctx, resp, true)
	return nil
}

func (f *HTTPFilter) ApplyOutput(ctx *FilterContext) error {
	req := HTTPFilterRequest{
		Output:    string(ctx.ResponseBody),
		Model:     ctx.RequestModel,
		Endpoint:  ctx.Endpoint,
		UserID:    ctx.UserID,
		TenantID:  ctx.TenantID,
		SessionID: ctx.SessionID,
		Metadata:  ctx.Metadata,
	}

	resp, err := f.callService(ctx.Context, req)
	if err != nil {
		return f.handleError(ctx, err)
	}

	f.applyResponse(ctx, resp, false)
	return nil
}

func (f *HTTPFilter) callService(ctx context.Context, payload HTTPFilterRequest) (*HTTPFilterResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range f.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call service: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(httpResp.Body, 1024))
		return nil, fmt.Errorf("service returned error %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var resp HTTPFilterResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

func (f *HTTPFilter) applyResponse(ctx *FilterContext, resp *HTTPFilterResponse, isInput bool) {
	// Merge annotations
	if len(resp.Annotations) > 0 {
		for k, v := range resp.Annotations {
			ctx.Annotations[k] = v
		}
	}

	// Add detections
	if len(resp.Detections) > 0 {
		key := "detections"
		if !isInput {
			key = "output_detections"
		}
		existing, _ := ctx.Annotations[key].([]Detection)
		ctx.Annotations[key] = append(existing, resp.Detections...)
	}

	// Handle blocking
	if resp.Block || !resp.Allowed {
		ctx.Block = true
		if resp.BlockReason != "" {
			ctx.BlockReason = resp.BlockReason
		} else {
			ctx.BlockReason = fmt.Sprintf("blocked by %s", f.name)
		}
	}

	// Apply redactions
	if isInput && resp.RedactedInput != "" {
		ctx.ModifiedRequestBody = []byte(resp.RedactedInput)
	}
	if !isInput && resp.RedactedOutput != "" {
		ctx.ModifiedResponseBody = []byte(resp.RedactedOutput)
	}
}

func (f *HTTPFilter) handleError(ctx *FilterContext, err error) error {
	switch f.onError {
	case ErrorActionBlock:
		ctx.Block = true
		ctx.BlockReason = fmt.Sprintf("filter service error: %v", err)
		return nil
	case ErrorActionAllow:
		// Continue processing, ignore the error
		return nil
	case ErrorActionBypass:
		fallthrough
	default:
		// Return the error to skip this filter
		return err
	}
}
