package firewall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
	Token     string            // Bearer token (auto-prefixed with "Bearer ")
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
	token     string // Bearer token
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
		token:     config.Token,
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
	// Extract text content from the request body (handles OpenAI/Anthropic message formats)
	extractedText, textPositions := extractTextFromRequest(ctx.RequestBody)

	// If no text was extracted, send the raw body as fallback
	inputText := extractedText
	if inputText == "" {
		inputText = string(ctx.RequestBody)
	}

	req := HTTPFilterRequest{
		Input:     inputText,
		Model:     ctx.RequestModel,
		Endpoint:  ctx.Endpoint,
		UserID:    ctx.UserID,
		TenantID:  ctx.TenantID,
		SessionID: ctx.SessionID,
		Metadata:  ctx.Metadata,
	}

	resp, err := f.callService(ctx.Context, req)
	if err != nil {
		log.Printf("[http_filter] callService error: %v", err)
		return f.handleError(ctx, err)
	}

	// Apply redactions back to the original JSON structure
	f.applyResponseWithTextPositions(ctx, resp, true, extractedText, textPositions)
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
	// Add Bearer token if configured (auto-prefixed)
	if f.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+f.token)
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

	// Store token mappings from external filter (e.g., Presidio)
	// This enables proper detokenization in output responses
	if isInput && ctx.Mode == ModeRedact && ctx.Tokenizer != nil && len(resp.Entities) > 0 {
		// Convert bytes to runes for proper Unicode character indexing
		// Presidio returns character positions, not byte positions
		inputRunes := []rune(string(ctx.RequestBody))
		for _, entity := range resp.Entities {
			// Entity from Presidio: Type, Mask (token), and we need original value
			// The original value can be extracted from the original input at Start:End (character indices)
			originalValue := ""
			if entity.Start >= 0 && entity.End > entity.Start && entity.End <= len(inputRunes) {
				originalValue = string(inputRunes[entity.Start:entity.End])
			}

			if originalValue != "" && entity.Mask != "" {
				// Store the mapping: token (mask) -> original value
				_ = ctx.Tokenizer.StoreExternalToken(
					ctx.Context,
					ctx.SessionID,
					entity.Type,
					originalValue,
					entity.Mask,
				)
			}
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

// textPosition tracks where extracted text came from in the original JSON
type textPosition struct {
	JSONPath    string // e.g., "messages[0].content"
	StartInText int    // start position in extracted text
	EndInText   int    // end position in extracted text
	OriginalVal string // original value for replacement
}

// extractTextFromRequest extracts user message content from OpenAI/Anthropic request formats.
// Returns the concatenated text and positions for mapping redactions back.
func extractTextFromRequest(body []byte) (string, []textPosition) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", nil
	}

	var texts []string
	var positions []textPosition
	currentOffset := 0

	// OpenAI format: messages[].content (string or array of content blocks)
	if messages, ok := data["messages"].([]interface{}); ok {
		for i, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}

			// Skip system messages for now, focus on user content
			role, _ := msgMap["role"].(string)
			if role != "user" && role != "assistant" {
				continue
			}

			switch content := msgMap["content"].(type) {
			case string:
				if content != "" {
					if len(texts) > 0 {
						texts = append(texts, "\n")
						currentOffset++
					}
					positions = append(positions, textPosition{
						JSONPath:    fmt.Sprintf("messages[%d].content", i),
						StartInText: currentOffset,
						EndInText:   currentOffset + len(content),
						OriginalVal: content,
					})
					texts = append(texts, content)
					currentOffset += len(content)
				}
			case []interface{}:
				// Array of content blocks (for multimodal)
				for j, block := range content {
					blockMap, ok := block.(map[string]interface{})
					if !ok {
						continue
					}
					if blockMap["type"] == "text" {
						if text, ok := blockMap["text"].(string); ok && text != "" {
							if len(texts) > 0 {
								texts = append(texts, "\n")
								currentOffset++
							}
							positions = append(positions, textPosition{
								JSONPath:    fmt.Sprintf("messages[%d].content[%d].text", i, j),
								StartInText: currentOffset,
								EndInText:   currentOffset + len(text),
								OriginalVal: text,
							})
							texts = append(texts, text)
							currentOffset += len(text)
						}
					}
				}
			}
		}
	}

	// Anthropic native format: content blocks at top level or in messages
	if content, ok := data["content"].([]interface{}); ok {
		for i, block := range content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}
			if blockMap["type"] == "text" {
				if text, ok := blockMap["text"].(string); ok && text != "" {
					if len(texts) > 0 {
						texts = append(texts, "\n")
						currentOffset++
					}
					positions = append(positions, textPosition{
						JSONPath:    fmt.Sprintf("content[%d].text", i),
						StartInText: currentOffset,
						EndInText:   currentOffset + len(text),
						OriginalVal: text,
					})
					texts = append(texts, text)
					currentOffset += len(text)
				}
			}
		}
	}

	return strings.Join(texts, ""), positions
}

// applyResponseWithTextPositions applies the filter response, mapping redactions back to the original JSON.
func (f *HTTPFilter) applyResponseWithTextPositions(ctx *FilterContext, resp *HTTPFilterResponse, isInput bool, extractedText string, positions []textPosition) {
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

	// Store token mappings from external filter (e.g., Presidio)
	if isInput && ctx.Mode == ModeRedact && ctx.Tokenizer != nil && len(resp.Entities) > 0 {
		// Use the extracted text for correct position mapping
		inputText := extractedText
		if inputText == "" {
			inputText = string(ctx.RequestBody)
		}
		inputRunes := []rune(inputText)
		for _, entity := range resp.Entities {
			originalValue := ""
			if entity.Start >= 0 && entity.End > entity.Start && entity.End <= len(inputRunes) {
				originalValue = string(inputRunes[entity.Start:entity.End])
			}

			if originalValue != "" && entity.Mask != "" {
				_ = ctx.Tokenizer.StoreExternalToken(
					ctx.Context,
					ctx.SessionID,
					entity.Type,
					originalValue,
					entity.Mask,
				)
			}
		}
	}

	// Apply redactions back to the original JSON structure
	if isInput && resp.RedactedInput != "" && len(positions) > 0 {
		// Map redactions back to original JSON
		modifiedBody := applyRedactionsToJSON(ctx.RequestBody, extractedText, resp.RedactedInput, positions)
		if modifiedBody != nil {
			ctx.ModifiedRequestBody = modifiedBody
		}
	} else if isInput && resp.RedactedInput != "" {
		// Fallback: use redacted input directly if no positions
		ctx.ModifiedRequestBody = []byte(resp.RedactedInput)
	}

	if !isInput && resp.RedactedOutput != "" {
		ctx.ModifiedResponseBody = []byte(resp.RedactedOutput)
	}
}

// applyRedactionsToJSON applies redacted text back to the original JSON structure.
func applyRedactionsToJSON(originalBody []byte, extractedText, redactedText string, positions []textPosition) []byte {
	// Parse the original JSON
	var data map[string]interface{}
	if err := json.Unmarshal(originalBody, &data); err != nil {
		return nil
	}

	// For each text position, find the corresponding redacted segment and apply it
	for _, pos := range positions {
		// Extract the original and redacted segments for this position
		originalSegment := pos.OriginalVal
		redactedSegment := redactedText
		if pos.EndInText <= len(redactedText) {
			redactedSegment = redactedText[pos.StartInText:pos.EndInText]
			// Adjust for length changes from previous redactions
			// This is a simplified approach - in practice, we need to track offset changes
		}

		// If lengths differ, we need to find the redacted segment more carefully
		// Use the entities to map old positions to new
		if len(extractedText) != len(redactedText) {
			// Find all differences between original and redacted text
			// For now, just replace the entire content field with the redacted version
			// This works because redaction tokens are same or similar length
			start := pos.StartInText
			end := pos.EndInText

			// Adjust end based on length change
			lenDiff := len(redactedText) - len(extractedText)
			adjustedEnd := end + lenDiff
			if adjustedEnd > len(redactedText) {
				adjustedEnd = len(redactedText)
			}
			if start < len(redactedText) {
				redactedSegment = redactedText[start:adjustedEnd]
			}
		}

		// Apply the redacted segment to the JSON path
		if redactedSegment != originalSegment {
			applyToJSONPath(data, pos.JSONPath, redactedSegment)
		}
	}

	// Re-marshal the modified JSON
	result, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	return result
}

// applyToJSONPath sets a value at a JSON path like "messages[0].content" or "content[0].text"
func applyToJSONPath(data map[string]interface{}, path string, value string) {
	// Simple parser for paths like "messages[0].content" or "messages[0].content[1].text"
	parts := strings.Split(path, ".")
	current := interface{}(data)

	for i, part := range parts {
		isLast := i == len(parts)-1

		// Check for array index
		if idx := strings.Index(part, "["); idx != -1 {
			key := part[:idx]
			indexStr := strings.TrimSuffix(part[idx+1:], "]")
			var index int
			fmt.Sscanf(indexStr, "%d", &index)

			// Navigate to array
			if m, ok := current.(map[string]interface{}); ok {
				if arr, ok := m[key].([]interface{}); ok {
					if index < len(arr) {
						if isLast {
							// Set the value (for array element that's a string)
							arr[index] = value
						} else {
							current = arr[index]
						}
					}
				}
			}
		} else {
			// Regular key
			if m, ok := current.(map[string]interface{}); ok {
				if isLast {
					m[part] = value
				} else {
					current = m[part]
				}
			}
		}
	}
}
