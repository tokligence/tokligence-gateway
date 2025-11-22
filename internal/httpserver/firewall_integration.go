package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tokligence/tokligence-gateway/internal/firewall"
)

// applyInputFirewall processes the request body through the firewall input pipeline.
// Returns modified body (if any) and error if request is blocked.
func (s *Server) applyInputFirewall(ctx context.Context, endpoint string, model string, userID string, body []byte) ([]byte, error) {
	if s.firewallPipeline == nil {
		return body, nil
	}

	mode := s.firewallPipeline.GetMode()
	if mode == firewall.ModeDisabled {
		return body, nil
	}

	fctx := firewall.NewFilterContext(ctx)
	fctx.RequestBody = body
	fctx.RequestModel = model
	fctx.Endpoint = endpoint
	fctx.UserID = userID

	if err := s.firewallPipeline.ProcessInput(ctx, fctx); err != nil {
		// Log the block
		if s.logger != nil {
			s.logger.Printf("firewall.input.blocked endpoint=%s model=%s user=%s reason=%s",
				endpoint, model, userID, fctx.BlockReason)
		}
		return nil, fmt.Errorf("request blocked by firewall: %s", fctx.BlockReason)
	}

	// Log detections in monitor mode
	if mode == firewall.ModeMonitor && len(fctx.Annotations) > 0 {
		s.logFirewallDetections("input", fctx)
	}

	// Return modified body if any filters redacted content
	if len(fctx.ModifiedRequestBody) > 0 && !bytes.Equal(body, fctx.ModifiedRequestBody) {
		if s.logger != nil && s.isDebug() {
			s.debugf("firewall.input.redacted original_size=%d redacted_size=%d",
				len(body), len(fctx.ModifiedRequestBody))
		}
		return fctx.ModifiedRequestBody, nil
	}

	return body, nil
}

// applyOutputFirewall processes the response body through the firewall output pipeline.
// Returns modified body (if any) and error if response is blocked.
func (s *Server) applyOutputFirewall(ctx context.Context, endpoint string, model string, userID string, body []byte) ([]byte, error) {
	if s.firewallPipeline == nil {
		return body, nil
	}

	mode := s.firewallPipeline.GetMode()
	if mode == firewall.ModeDisabled {
		return body, nil
	}

	fctx := firewall.NewFilterContext(ctx)
	fctx.ResponseBody = body
	fctx.RequestModel = model
	fctx.Endpoint = endpoint
	fctx.UserID = userID

	if err := s.firewallPipeline.ProcessOutput(ctx, fctx); err != nil {
		// Log the block
		if s.logger != nil {
			s.logger.Printf("firewall.output.blocked endpoint=%s model=%s user=%s reason=%s",
				endpoint, model, userID, fctx.BlockReason)
		}
		return nil, fmt.Errorf("response blocked by firewall: %s", fctx.BlockReason)
	}

	// Log detections in monitor mode
	if mode == firewall.ModeMonitor && len(fctx.Annotations) > 0 {
		s.logFirewallDetections("output", fctx)
	}

	// Return modified body if any filters redacted content
	if len(fctx.ModifiedResponseBody) > 0 && !bytes.Equal(body, fctx.ModifiedResponseBody) {
		if s.logger != nil && s.isDebug() {
			s.debugf("firewall.output.redacted original_size=%d redacted_size=%d",
				len(body), len(fctx.ModifiedResponseBody))
		}
		return fctx.ModifiedResponseBody, nil
	}

	return body, nil
}

// wrapRequestBody wraps the request body reading and applies firewall input filtering.
// This helper can be used in any handler that needs to apply input firewall.
func (s *Server) wrapRequestBody(ctx context.Context, r io.Reader, endpoint, model, userID string) ([]byte, error) {
	// Read original body
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	// Apply firewall
	filteredBody, err := s.applyInputFirewall(ctx, endpoint, model, userID, body)
	if err != nil {
		return nil, err
	}

	return filteredBody, nil
}

// wrapResponseBody wraps the response body and applies firewall output filtering.
func (s *Server) wrapResponseBody(ctx context.Context, body []byte, endpoint, model, userID string) ([]byte, error) {
	return s.applyOutputFirewall(ctx, endpoint, model, userID, body)
}

// logFirewallDetections logs firewall detections for monitoring purposes.
func (s *Server) logFirewallDetections(location string, fctx *firewall.FilterContext) {
	if s.logger == nil {
		return
	}

	// Extract detection counts
	var piiCount int
	var detectionTypes []string

	if location == "input" {
		if count, ok := fctx.Annotations["pii_count"].(int); ok {
			piiCount = count
		}
		if types, ok := fctx.Annotations["pii_types"].([]string); ok {
			detectionTypes = types
		}
		if detections, ok := fctx.Annotations["pii_detections"].([]firewall.Detection); ok && len(detections) > 0 {
			for _, det := range detections {
				if s.isDebug() {
					details, _ := json.Marshal(det.Details)
					s.debugf("firewall.detection location=%s type=%s severity=%s details=%s",
						location, det.Type, det.Severity, string(details))
				}
			}
		}
	} else {
		if count, ok := fctx.Annotations["pii_output_count"].(int); ok {
			piiCount = count
		}
		if types, ok := fctx.Annotations["pii_output_types"].([]string); ok {
			detectionTypes = types
		}
		if detections, ok := fctx.Annotations["pii_output_detections"].([]firewall.Detection); ok && len(detections) > 0 {
			for _, det := range detections {
				if s.isDebug() {
					details, _ := json.Marshal(det.Details)
					s.debugf("firewall.detection location=%s type=%s severity=%s details=%s",
						location, det.Type, det.Severity, string(details))
				}
			}
		}
	}

	if piiCount > 0 {
		s.logger.Printf("firewall.monitor location=%s pii_count=%d types=%v",
			location, piiCount, detectionTypes)
	}
}
