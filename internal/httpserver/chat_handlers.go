package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/firewall"
	"github.com/tokligence/tokligence-gateway/internal/ledger"
	"github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// HandleChatCompletions is the public entry point registered on the router.
func (s *Server) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	s.handleChatCompletions(w, r)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	reqStart := time.Now()
	var (
		sessionUser *userstore.User
		apiKey      *userstore.APIKey
	)
	if s.identity != nil && !s.authDisabled {
		var err error
		sessionUser, apiKey, err = s.authenticateAPIKeyRequest(r)
		if err != nil {
			s.respondError(w, http.StatusUnauthorized, err)
			return
		}
		if sessionUser != nil {
			s.applySessionUser(sessionUser)
		}
	}
	// Read and parse request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// Apply input firewall before processing
	userID := ""
	if sessionUser != nil {
		userID = fmt.Sprintf("%d", sessionUser.ID)
	}
	// Generate a firewall session ID to link input/output token mappings
	// This is an internal ID used only within this request lifecycle
	firewallSessionID := uuid.New().String()
	filteredBody, err := s.applyInputFirewall(r.Context(), "/v1/chat/completions", "", userID, firewallSessionID, bodyBytes)
	if err != nil {
		s.respondError(w, http.StatusForbidden, err)
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.Unmarshal(filteredBody, &req); err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// If chat->anthropic translation is enabled and model suggests Anthropic provider, use bridge
	usePassthrough, err := s.workModeDecision("/v1/chat/completions", req.Model)
	if err == nil && !usePassthrough && s.chatToAnthropicEnabled && strings.TrimSpace(s.anthAPIKey) != "" {
		s.translateChatToAnthropic(w, r, reqStart, req, sessionUser, apiKey)
		return
	}

	// Streaming branch
	if req.Stream {
		if sa, ok := s.adapter.(adapter.StreamingChatAdapter); ok {
			s.handleChatStreamWithFirewall(w, r, reqStart, req, sessionUser, apiKey, sa, firewallSessionID)
			return
		}
		// If adapter doesn't support streaming, fall back to non-streaming
	}

	upstreamStart := time.Now()
	resp, err := s.adapter.CreateCompletion(r.Context(), req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	upstreamDur := time.Since(upstreamStart)
	s.recordUsageLedger(r.Context(), sessionUser, apiKey, int64(resp.Usage.PromptTokens), int64(resp.Usage.CompletionTokens), "chat.completions")

	// Apply output firewall (use same firewallSessionID for token restoration)
	respBytes, err := json.Marshal(resp)
	if err == nil {
		filteredResp, err := s.applyOutputFirewall(r.Context(), "/v1/chat/completions", req.Model, userID, firewallSessionID, respBytes)
		if err != nil {
			s.respondError(w, http.StatusForbidden, err)
			return
		}
		if !bytes.Equal(respBytes, filteredResp) {
			// Response was modified by firewall, unmarshal back
			if err := json.Unmarshal(filteredResp, &resp); err != nil {
				s.respondError(w, http.StatusInternalServerError, err)
				return
			}
		}
	}

	s.respondJSON(w, http.StatusOK, resp)
	if s.logger != nil {
		total := time.Since(reqStart)
		s.logger.Printf("chat.completions total_ms=%d upstream_ms=%d model=%s", total.Milliseconds(), upstreamDur.Milliseconds(), req.Model)
	}
}

func (s *Server) handleChatStream(
	w http.ResponseWriter,
	r *http.Request,
	reqStart time.Time,
	req openai.ChatCompletionRequest,
	sessionUser *userstore.User,
	apiKey *userstore.APIKey,
	adapter adapter.StreamingChatAdapter,
) {
	s.handleChatStreamWithFirewall(w, r, reqStart, req, sessionUser, apiKey, adapter, "")
}

func (s *Server) handleChatStreamWithFirewall(
	w http.ResponseWriter,
	r *http.Request,
	reqStart time.Time,
	req openai.ChatCompletionRequest,
	sessionUser *userstore.User,
	apiKey *userstore.APIKey,
	adapter adapter.StreamingChatAdapter,
	firewallSessionID string,
) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, _ := w.(http.Flusher)

	ch, err := adapter.CreateCompletionStream(r.Context(), req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}

	// Create SSE buffer for PII detokenization if firewall is in redact mode
	var sseBuffer *firewall.SSEPIIBuffer
	if s.firewallPipeline != nil && firewallSessionID != "" {
		sseBuffer = s.firewallPipeline.NewSSEBuffer(firewallSessionID)
	}

	enc := json.NewEncoder(w)
	approxPromptTokens := approximatePromptTokens(req)
	completionChars := 0
	firstDeltaAt := time.Time{}

	for ev := range ch {
		if ev.Error != nil {
			_, _ = io.WriteString(w, "data: {\"error\": \"stream error\"}\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		if ev.Chunk == nil {
			continue
		}
		deltaStr := ev.Chunk.GetDelta().Content
		completionChars += len(deltaStr)
		if firstDeltaAt.IsZero() && strings.TrimSpace(deltaStr) != "" {
			firstDeltaAt = time.Now()
		}

		// Apply SSE buffer for PII detokenization
		if sseBuffer != nil && sseBuffer.IsEnabled() && deltaStr != "" {
			processed := sseBuffer.ProcessChunk(r.Context(), deltaStr)
			// Always use processed result (may be empty when buffering, or detokenized)
			ev.Chunk.SetDeltaContent(processed)
			deltaStr = processed // Update for empty check below
		}

		_, _ = io.WriteString(w, "data: ")
		if err := enc.Encode(ev.Chunk); err != nil {
			return
		}
		_, _ = io.WriteString(w, "\n")
		if flusher != nil {
			flusher.Flush()
		}
	}

	// Flush any remaining buffered content and send to client
	if sseBuffer != nil && sseBuffer.HasBufferedContent() {
		remaining := sseBuffer.Flush(r.Context())
		if remaining != "" {
			s.debugf("firewall.sse: flushing remaining buffered content: %d chars", len(remaining))
			// Send remaining content as a final delta to avoid truncation
			finalDelta := openai.ChatCompletionChunk{
				Model: req.Model,
				Choices: []openai.ChatCompletionChunkChoice{{
					Delta: openai.ChatMessageDelta{
						Content: remaining,
					},
				}},
			}
			if jsonBytes, err := json.Marshal(finalDelta); err == nil {
				_, _ = io.WriteString(w, "data: "+string(jsonBytes)+"\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}

	s.recordUsageLedger(r.Context(), sessionUser, apiKey, int64(approxPromptTokens), int64(completionChars/4), "chat.completions(stream)")

	if s.logger != nil {
		total := time.Since(reqStart)
		ttfb := time.Duration(0)
		if !firstDeltaAt.IsZero() {
			ttfb = firstDeltaAt.Sub(reqStart)
		}
		s.logger.Printf("chat.completions.stream total_ms=%d ttfb_ms=%d model=%s", total.Milliseconds(), ttfb.Milliseconds(), req.Model)
	}
}

func (s *Server) recordUsageLedger(
	ctx context.Context,
	sessionUser *userstore.User,
	apiKey *userstore.APIKey,
	promptTokens int64,
	completionTokens int64,
	memo string,
) {
	if s.ledger == nil {
		return
	}
	uid := s.lookupLedgerUserID(sessionUser)
	if uid == 0 {
		return
	}
	entry := ledger.Entry{
		UserID:           uid,
		ServiceID:        0,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		Direction:        ledger.DirectionConsume,
		Memo:             memo,
	}
	if apiKey != nil {
		id := apiKey.ID
		entry.APIKeyID = &id
	}
	_ = s.ledger.Record(ctx, entry)
}

func (s *Server) lookupLedgerUserID(sessionUser *userstore.User) int64 {
	if sessionUser != nil {
		return sessionUser.ID
	}
	if user, _ := s.gateway.Account(); user != nil {
		return user.ID
	}
	return 0
}

// approximatePromptTokens estimates tokens from request messages (4 chars ~ 1 token).
func approximatePromptTokens(req openai.ChatCompletionRequest) int {
	total := 0
	for _, m := range req.Messages {
		// Extract content as string (supports string and structured content blocks)
		total += len(extractContentStr(m.Content))
	}
	n := total/4 + 1
	if n < len(req.Messages)*2 {
		n = len(req.Messages) * 2
	}
	return n
}
