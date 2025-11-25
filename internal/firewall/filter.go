package firewall

import (
	"context"
	"time"
)

// FirewallMode determines how the firewall behaves when violations are detected.
type FirewallMode string

const (
	// ModeMonitor logs violations but allows requests to proceed
	ModeMonitor FirewallMode = "monitor"
	// ModeEnforce blocks requests that violate firewall rules
	ModeEnforce FirewallMode = "enforce"
	// ModeRedact detects PII, replaces with tokens, and maps back in responses
	ModeRedact FirewallMode = "redact"
	// ModeDisabled disables the firewall entirely
	ModeDisabled FirewallMode = "disabled"
)

// Direction indicates whether the filter applies to input or output.
type Direction string

const (
	DirectionInput  Direction = "input"
	DirectionOutput Direction = "output"
	DirectionBoth   Direction = "both"
)

// FilterContext contains all information needed for filtering operations.
// It supports both input (request) and output (response) filtering.
type FilterContext struct {
	// Request data
	RequestBody  []byte
	RequestModel string
	Endpoint     string

	// Response data
	ResponseBody []byte

	// Metadata
	UserID    string
	TenantID  string
	SessionID string
	Metadata  map[string]any

	// Filter results
	Block       bool
	BlockReason string
	Annotations map[string]any // Signals for policy engine

	// Modified content (after redaction/masking)
	ModifiedRequestBody  []byte
	ModifiedResponseBody []byte

	// PII tokenization mappings (for redact mode)
	PIITokens map[string]*PIIToken // Maps token -> PIIToken for this request
	Tokenizer *PIITokenizer        // Reference to global tokenizer

	// Firewall mode (monitor, enforce, redact, disabled)
	Mode FirewallMode

	// Context for cancellation and timeouts
	Context context.Context

	// Timing information
	StartTime time.Time
}

// NewFilterContext creates a new FilterContext with defaults.
func NewFilterContext(ctx context.Context) *FilterContext {
	return &FilterContext{
		Context:     ctx,
		Metadata:    make(map[string]any),
		Annotations: make(map[string]any),
		StartTime:   time.Now(),
	}
}

// Filter is the base interface for all firewall filters.
type Filter interface {
	// Name returns a unique identifier for this filter
	Name() string

	// Priority determines execution order (lower = earlier)
	Priority() int

	// Direction indicates if this filter applies to input, output, or both
	Direction() Direction
}

// InputFilter processes incoming requests before they reach the LLM provider.
type InputFilter interface {
	Filter

	// ApplyInput examines and potentially modifies the request.
	// Returns error if the filter fails to process (not the same as blocking).
	ApplyInput(ctx *FilterContext) error
}

// OutputFilter processes LLM responses before returning to the client.
type OutputFilter interface {
	Filter

	// ApplyOutput examines and potentially modifies the response.
	// Returns error if the filter fails to process (not the same as blocking).
	ApplyOutput(ctx *FilterContext) error
}

// Detection represents a specific violation or finding.
type Detection struct {
	FilterName string         `json:"filter_name"`
	Type       string         `json:"type"`     // e.g., "pii", "prompt_injection", "content_safety"
	Severity   string         `json:"severity"` // "low", "medium", "high", "critical"
	Message    string         `json:"message"`
	Location   string         `json:"location"` // "input" or "output"
	Details    map[string]any `json:"details,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

// RedactionResult contains information about redacted content.
type RedactionResult struct {
	Original string           `json:"original,omitempty"`
	Redacted string           `json:"redacted"`
	Entities []RedactedEntity `json:"entities,omitempty"`
}

// RedactedEntity represents a single redacted piece of information.
type RedactedEntity struct {
	Type       string  `json:"type"`            // e.g., "EMAIL", "PHONE", "SSN"
	Value      string  `json:"value,omitempty"` // Original value (only in monitor mode)
	Mask       string  `json:"mask"`            // Redacted representation
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence,omitempty"`
}
