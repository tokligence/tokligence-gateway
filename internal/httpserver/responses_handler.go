package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tokligence/tokligence-gateway/internal/adapter"
	anthpkg "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	openairesp "github.com/tokligence/tokligence-gateway/internal/httpserver/openai/responses"
	respconv "github.com/tokligence/tokligence-gateway/internal/httpserver/responses"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// responsesRequest is an alias for openai.ResponseRequest for backward compatibility.
type responsesRequest = openai.ResponseRequest

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// responsesMessage is a simplified Responses output message.
type responsesMessage struct {
	Type    string                   `json:"type"`    // "message"
	Role    string                   `json:"role"`    // "assistant"
	Content []map[string]interface{} `json:"content"` // [{"type":"output_text","text":"..."}]
}

type responsesResponse struct {
	ID         string             `json:"id"`
	Object     string             `json:"object"` // "response"
	Created    int64              `json:"created"`
	Model      string             `json:"model"`
	Output     []responsesMessage `json:"output"`
	OutputText string             `json:"output_text,omitempty"`
	Usage      *responsesUsage    `json:"usage,omitempty"`
}

type responseSession struct {
	Adapter string
	Base    responsesRequest
	Request openai.ChatCompletionRequest
	Outputs chan []openai.ResponseToolOutput
	Done    chan struct{}
	once    sync.Once
}

func newResponseSession(adapter string, base responsesRequest, req openai.ChatCompletionRequest) *responseSession {
	return &responseSession{
		Adapter: adapter,
		Base:    base,
		Request: req,
		Outputs: make(chan []openai.ResponseToolOutput),
		Done:    make(chan struct{}),
	}
}

func (rs *responseSession) close() {
	rs.once.Do(func() {
		close(rs.Done)
		close(rs.Outputs)
	})
}

func (rs *responseSession) hasToolOutput(callID string) bool {
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return false
	}
	for _, msg := range rs.Request.Messages {
		if !strings.EqualFold(msg.Role, "tool") {
			continue
		}
		if strings.TrimSpace(msg.ToolCallID) == callID {
			return true
		}
	}
	return false
}

func (s *Server) HandleResponses(w http.ResponseWriter, r *http.Request) {
	s.handleResponses(w, r)
}

func (s *Server) HandleResponsesSubmitToolOutputs(w http.ResponseWriter, r *http.Request) {
	reqStart := time.Now()
	var rr responsesRequest
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("invalid tool_outputs json: %w", err))
		return
	}
	if strings.TrimSpace(rr.ID) == "" {
		rr.ID = strings.TrimSpace(chi.URLParam(r, "id"))
	}
	rr.Stream = rr.Stream || strings.EqualFold(r.URL.Query().Get("stream"), "true")
	s.handleResponsesToolOutputs(w, r, rr, reqStart)
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	reqStart := time.Now()
	var rr responsesRequest
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("invalid responses json: %w", err))
		return
	}

	stream := rr.Stream || strings.EqualFold(r.URL.Query().Get("stream"), "true")
	rr.Stream = stream

	buildStart := time.Now()
	creq := rr.ToChatCompletionRequest()
	creq.Stream = stream

	// Responses API standard: detect function_call_output continuation (tool outputs)
	hasFunctionCallOutput, previousRespID := s.detectFunctionCallOutputInMessages(creq.Messages)
	if hasFunctionCallOutput && previousRespID != "" {
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses: detected function_call_output continuation, resuming session id=%s", previousRespID)
		}
		s.handleFunctionCallOutputContinuation(w, r, rr, previousRespID, reqStart)
		return
	}

	// Legacy: handle tool outputs submitted via submit_tool_outputs endpoint
	if len(rr.ToolOutputs) > 0 && strings.TrimSpace(rr.ID) != "" {
		s.handleResponsesToolOutputs(w, r, rr, reqStart)
		return
	}

	// Use work mode decision to determine passthrough vs translation
	usePassthrough, err := s.workModeDecision("/v1/responses", rr.Model)
	if err != nil {
		s.respondError(w, http.StatusBadRequest, err)
		return
	}

	// If passthrough mode and OpenAI API key is available, delegate
	if usePassthrough && strings.TrimSpace(s.openaiAPIKey) != "" {
		if stream {
			if s.logger != nil {
				s.logger.Printf("responses: disabling stream for openai delegation to avoid unsupported streaming")
			}
			stream = false
			rr.Stream = false
		}
		s.delegateOpenAIResponses(w, r.Context(), rr, stream, reqStart)
		return
	}

	// For translation mode, determine which adapter to use
	adapterName, adapterErr := s.resolveAdapterForModel(rr.Model)
	adapterName = strings.ToLower(strings.TrimSpace(adapterName))

	if s.isDebug() && s.logger != nil {
		for idx, msg := range creq.Messages {
			if len(msg.ToolCalls) > 0 {
				s.logger.Printf("responses.converted[%d] role=%s tool_calls=%d", idx, msg.Role, len(msg.ToolCalls))
			} else if strings.TrimSpace(msg.ToolCallID) != "" {
				s.logger.Printf("responses.converted[%d] role=%s tool_call_id=%s content=%q", idx, msg.Role, msg.ToolCallID, msg.Content)
			} else {
				s.logger.Printf("responses.converted[%d] role=%s content=%q", idx, msg.Role, msg.Content)
			}
		}
	}
	switch strings.ToLower(strings.TrimSpace(rr.ResponseFormat.Type)) {
	case "json_object":
		creq.Messages = append([]openai.ChatMessage{{Role: "system", Content: "Return ONLY a valid JSON object and no extra prose."}}, creq.Messages...)
	case "json_schema":
		if rr.ResponseFormat.JsonSchema != nil {
			if b, err := json.Marshal(rr.ResponseFormat.JsonSchema); err == nil {
				msg := "Return ONLY JSON strictly matching this JSON Schema (no prose):\n" + string(b)
				creq.Messages = append([]openai.ChatMessage{{Role: "system", Content: msg}}, creq.Messages...)
			}
		}
	}
	buildDur := time.Since(buildStart)

	if adapterErr == nil && adapterName == "anthropic" {
		if err := s.forwardResponsesToAnthropic(w, r, rr, creq, stream, reqStart, buildDur, "", "anthropic"); err != nil {
			s.respondError(w, http.StatusBadGateway, err)
		}
		return
	}

	if stream {
		if s.responsesStreamAggregate {
			resp, err := s.adapter.CreateCompletion(r.Context(), creq)
			if err != nil {
				if s.logger != nil {
					s.logger.Printf("responses.stream.aggregate error: %v", err)
				}
				s.respondError(w, http.StatusBadGateway, err)
				return
			}
			var text string
			if len(resp.Choices) > 0 {
				text = resp.Choices[0].Message.Content
			}
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-transform")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
			flusher, _ := w.(http.Flusher)
			enc := json.NewEncoder(w)
			fmt.Fprintf(w, "event: response.created\ndata: ")
			_ = enc.Encode(map[string]any{"type": "response.created", "id": fmt.Sprintf("resp_%d", time.Now().UnixNano()), "created": time.Now().Unix()})
			fmt.Fprint(w, "\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			const chunkSize = 128
			for i := 0; i < len(text); i += chunkSize {
				end := i + chunkSize
				if end > len(text) {
					end = len(text)
				}
				part := text[i:end]
				fmt.Fprintf(w, "event: response.output_text.delta\ndata: ")
				_ = enc.Encode(map[string]string{"type": "response.output_text.delta", "delta": part})
				fmt.Fprint(w, "\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			}
			if strings.TrimSpace(text) != "" {
				fmt.Fprintf(w, "event: response.output_text.done\ndata: ")
				_ = enc.Encode(map[string]any{"type": "response.output_text.done"})
				fmt.Fprint(w, "\n\n")
			}
			fmt.Fprintf(w, "event: response.completed\ndata: ")
			_ = enc.Encode(map[string]any{"type": "response.completed"})
			fmt.Fprint(w, "\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(200 * time.Millisecond)
			if s.logger != nil {
				s.logger.Printf("responses.stream.aggregate model=%s bytes=%d", creq.Model, len(text))
			}
			return
		}
		if sa, ok := s.adapter.(adapter.StreamingChatAdapter); ok {
			s.streamResponses(w, r, rr, creq, reqStart, buildDur, strings.TrimSpace(rr.ID), adapterName, func(ctx context.Context, conv respconv.Conversation) (respconv.StreamInit, error) {
				ch, err := sa.CreateCompletionStream(ctx, conv.Chat)
				if err != nil {
					return respconv.StreamInit{}, err
				}
				return respconv.StreamInit{Channel: ch}, nil
			})
			return
		}

		resp, err := s.adapter.CreateCompletion(r.Context(), creq)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("responses.stream.fallback error: %v", err)
			}
			s.respondError(w, http.StatusBadGateway, err)
			return
		}
		var text string
		if len(resp.Choices) > 0 {
			text = resp.Choices[0].Message.Content
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, _ := w.(http.Flusher)
		emit := func(event string, payload any) {
			fmt.Fprintf(w, "event: %s\n", event)
			if payload != nil {
				b, _ := json.Marshal(payload)
				fmt.Fprintf(w, "data: %s\n\n", string(b))
			} else {
				fmt.Fprint(w, "data: {}\n\n")
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if strings.TrimSpace(text) != "" {
			emit("response.output_text.delta", map[string]string{"type": "response.output_text.delta", "delta": text})
			emit("response.output_text.done", map[string]any{"type": "response.output_text.done"})
			if strings.EqualFold(rr.ResponseFormat.Type, "json_object") || strings.EqualFold(rr.ResponseFormat.Type, "json_schema") {
				emit("response.output_json.delta", map[string]string{"type": "response.output_json.delta", "delta": text})
				emit("response.output_json.done", map[string]any{"type": "response.output_json.done"})
			}
		}
		emit("response.completed", map[string]any{"type": "response.completed"})
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(200 * time.Millisecond)
		if s.logger != nil {
			total := time.Since(reqStart)
			s.logger.Printf("responses.stream.fallback total_ms=%d build_ms=%d model=%s", total.Milliseconds(), buildDur.Milliseconds(), creq.Model)
		}
		return
	}

	upstreamStart := time.Now()
	resp, err := s.adapter.CreateCompletion(r.Context(), creq)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("responses error: %v", err)
		}
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	upstreamDur := time.Since(upstreamStart)
	rrsp := buildResponsesResponseFromChat(creq, resp)
	s.respondJSON(w, http.StatusOK, rrsp)
	if s.logger != nil {
		total := time.Since(reqStart)
		s.logger.Printf("responses total_ms=%d upstream_ms=%d build_ms=%d model=%s", total.Milliseconds(), upstreamDur.Milliseconds(), buildDur.Milliseconds(), creq.Model)
	}
}

func (s *Server) handleResponsesToolOutputs(w http.ResponseWriter, r *http.Request, rr responsesRequest, reqStart time.Time) {
	if strings.TrimSpace(rr.ID) == "" {
		s.respondError(w, http.StatusBadRequest, errors.New("response id required for tool_outputs submission"))
		return
	}
	if len(rr.ToolOutputs) == 0 {
		s.respondError(w, http.StatusBadRequest, errors.New("tool_outputs payload required"))
		return
	}

	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.tool_outputs id=%s count=%d", rr.ID, len(rr.ToolOutputs))
	}

	if err := s.deliverToolOutputs(rr.ID, rr.ToolOutputs); err != nil {
		if errors.Is(err, errResponseSessionNotFound) {
			s.respondError(w, http.StatusNotFound, fmt.Errorf("response session %q not found", rr.ID))
			return
		}
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (s *Server) deliverToolOutputs(id string, outputs []openai.ResponseToolOutput) error {
	if strings.TrimSpace(id) == "" {
		return errResponseSessionNotFound
	}
	s.responsesSessionsMu.Lock()
	sess, ok := s.responsesSessions[id]
	s.responsesSessionsMu.Unlock()
	if !ok {
		return errResponseSessionNotFound
	}
	select {
	case <-sess.Done:
		return errors.New("response session already closed")
	case sess.Outputs <- outputs:
		return nil
	}
}

func (s *Server) waitForToolOutputs(ctx context.Context, id string) ([]openai.ResponseToolOutput, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errResponseSessionNotFound
	}
	s.responsesSessionsMu.Lock()
	sess, ok := s.responsesSessions[id]
	s.responsesSessionsMu.Unlock()
	if !ok {
		return nil, errResponseSessionNotFound
	}
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.waitForToolOutputs waiting for tool outputs id=%s", id)
	}
	select {
	case outs, ok := <-sess.Outputs:
		if !ok {
			return nil, errors.New("response session closed")
		}
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.waitForToolOutputs received %d outputs id=%s", len(outs), id)
		}
		return outs, nil
	case <-sess.Done:
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.waitForToolOutputs session done id=%s", id)
		}
		return nil, errors.New("response session closed")
	case <-ctx.Done():
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.waitForToolOutputs context done id=%s err=%v", id, ctx.Err())
		}
		return nil, ctx.Err()
	}
}

func (s *Server) resolveAdapterForModel(model string) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", errors.New("model name required")
	}
	if s.modelRouter == nil {
		return "", errors.New("model router unavailable")
	}
	return s.modelRouter.GetAdapterForModel(model)
}

func (s *Server) delegateOpenAIResponses(w http.ResponseWriter, ctx context.Context, rr responsesRequest, stream bool, reqStart time.Time) {
	base := strings.TrimRight(s.openaiBaseURL, "/")
	url := base + "/responses"
	body, _ := formatOpenAIResponsesRequest(rr)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if s.openaiAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.openaiAPIKey)
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if stream {
		for k, v := range resp.Header {
			if strings.EqualFold(k, "content-type") {
				w.Header()[k] = v
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		if s.logger != nil {
			s.logger.Printf("responses.openai.stream total_ms=%d", time.Since(reqStart).Milliseconds())
		}
		return
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	if s.logger != nil {
		s.logger.Printf("responses.openai total_ms=%d", time.Since(reqStart).Milliseconds())
	}
}

func (s *Server) forwardResponsesToAnthropic(w http.ResponseWriter, r *http.Request, rr responsesRequest, creq openai.ChatCompletionRequest, stream bool, reqStart time.Time, buildDur time.Duration, responseID string, adapterName string) error {
	if strings.TrimSpace(s.anthAPIKey) == "" {
		return errors.New("anthropic API key not configured for Responses translation")
	}

	// Detect duplicate tool calls in message history (for Codex which sends full history in each request)
	if strings.TrimSpace(responseID) == "" {
		duplicateCount, duplicateWarning := s.detectDuplicateToolCalls(creq.Messages)
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.anthropic: duplicate_check count=%d has_warning=%v", duplicateCount, duplicateWarning != "")
		}

		// EMERGENCY STOP: Reject if 5+ duplicates
		if duplicateCount >= 5 {
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("responses.anthropic: EMERGENCY STOP - %d duplicate tool calls detected", duplicateCount)
			}
			http.Error(w, fmt.Sprintf(`{"error":{"message":"infinite loop detected: tool called %d times consecutively with identical arguments - execution halted","type":"invalid_request_error"}}`, duplicateCount), http.StatusBadRequest)
			return fmt.Errorf("infinite loop detected: %d duplicate tool calls", duplicateCount)
		}

		// Inject warning for 3-4 duplicates
		if duplicateWarning != "" {
			// Prepend warning to system message or create new one
			foundSystem := false
			for i, msg := range creq.Messages {
				if msg.Role == "system" {
					creq.Messages[i].Content = msg.Content + "\n\n" + duplicateWarning
					foundSystem = true
					break
				}
			}
			if !foundSystem {
				creq.Messages = append([]openai.ChatMessage{{
					Role:    "system",
					Content: duplicateWarning,
				}}, creq.Messages...)
			}
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("responses.anthropic: duplicate warning injected count=%d", duplicateCount)
			}
		}
	}

	// Adapt tools for OpenAI->Anthropic translation (only for initial requests, continuations use session)
	if strings.TrimSpace(responseID) == "" && s.toolAdapter != nil {
		if s.isDebug() && s.logger != nil {
			var toolNames []string
			for i, t := range creq.Tools {
				name := t.Function.Name
				if name == "" {
					name = fmt.Sprintf("(empty_%d)", i)
				}
				toolNames = append(toolNames, name)
			}
			s.logger.Printf("responses.anthropic: before adaptation tools=%d names=[%s] adapter=%s responseID=%q", len(creq.Tools), strings.Join(toolNames, ", "), adapterName, responseID)
		}
		creq = s.toolAdapter.AdaptChatRequest(creq, "openai", adapterName)
		if s.isDebug() && s.logger != nil {
			var toolNames []string
			for i, t := range creq.Tools {
				name := t.Function.Name
				if name == "" {
					name = fmt.Sprintf("(empty_%d)", i)
				}
				toolNames = append(toolNames, name)
			}
			s.logger.Printf("responses.anthropic: after adaptation tools=%d names=[%s]", len(creq.Tools), strings.Join(toolNames, ", "))
		}
	}

	if creq.MaxTokens == nil || (creq.MaxTokens != nil && *creq.MaxTokens <= 0) {
		def := s.anthropicMaxTokens
		if def <= 0 {
			def = 4096
		}
		mt := def
		creq.MaxTokens = &mt
	}
	if s.modelRouter != nil {
		creq.Model = s.modelRouter.RewriteModelPublic(creq.Model)
	}
	url := strings.TrimRight(s.anthBaseURL, "/") + "/v1/messages"
	if q := strings.TrimSpace(r.URL.RawQuery); q != "" {
		url = url + "?" + q
	}

	if stream {
		if s.responsesTranslator == nil {
			s.responsesTranslator = openairesp.NewTranslator()
		}
		provider := &respconv.AnthropicStreamProvider{
			URL:         url,
			APIKey:      s.anthAPIKey,
			Version:     s.anthVersion,
			Translator:  s.responsesTranslator,
			Client:      http.DefaultClient,
			GuardTokens: s.guardAnthropicTokens,
		}
		s.streamResponses(w, r, rr, creq, reqStart, buildDur, responseID, adapterName, func(ctx context.Context, conv respconv.Conversation) (respconv.StreamInit, error) {
			return provider.Stream(ctx, conv)
		})
		if s.logger != nil {
			s.logger.Printf("responses.anthropic.stream total_ms=%d model=%s", time.Since(reqStart).Milliseconds(), creq.Model)
		}
		return nil
	}

	if s.responsesTranslator == nil {
		s.responsesTranslator = openairesp.NewTranslator()
	}
	areq, err := s.responsesTranslator.OpenAIToNativeRequest(creq)
	if err != nil {
		return err
	}
	if areq.MaxTokens == 0 && creq.MaxTokens != nil {
		areq.MaxTokens = *creq.MaxTokens
	}
	areq.Stream = false
	if err := s.guardAnthropicTokens(areq.MaxTokens); err != nil {
		return err
	}
	s.debugf("responses translator converting to anthropic model=%s stream=%v", areq.Model, false)
	body, err := anthpkg.MarshalRequest(areq)
	if err != nil {
		return err
	}
	reqUp, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	reqUp.Header.Set("Content-Type", "application/json")
	reqUp.Header.Set("anthropic-version", s.anthVersion)
	if s.anthAPIKey != "" {
		reqUp.Header.Set("x-api-key", s.anthAPIKey)
	}
	resp, err := http.DefaultClient.Do(reqUp)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return bridgeUpstreamError{status: resp.StatusCode, body: raw}
	}
	if s.isDebug() {
		s.debugf("responses.anthropic upstream=%s", string(previewBytes(raw, 256)))
	}
	var nativeResp anthpkg.NativeResponse
	if err := json.Unmarshal(raw, &nativeResp); err != nil {
		return fmt.Errorf("anthropic decode: %w", err)
	}
	openaiResp, err := s.responsesTranslator.NativeToOpenAIResponse(nativeResp)
	if err != nil {
		return err
	}
	if openaiResp.Model == "" {
		openaiResp.Model = creq.Model
	}
	rrsp := buildResponsesResponseFromChat(creq, openaiResp)
	s.respondJSON(w, http.StatusOK, rrsp)
	if s.logger != nil {
		total := time.Since(reqStart)
		s.logger.Printf("responses.anthropic total_ms=%d build_ms=%d model=%s", total.Milliseconds(), buildDur.Milliseconds(), areq.Model)
	}
	if responseID != "" {
		s.clearResponseSession(responseID)
	}
	return nil
}

var errResponseSessionNotFound = errors.New("response session not found")

func formatOpenAIResponsesRequest(rr responsesRequest) ([]byte, error) {
	raw, err := json.Marshal(rr)
	if err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw, nil
	}

	// Convert messages to input if needed (OpenAI Responses API expects input, not messages)
	if messages, hasMessages := payload["messages"]; hasMessages {
		if _, hasInput := payload["input"]; !hasInput {
			payload["input"] = messages
		}
		delete(payload, "messages")
	}

	if rf, ok := payload["response_format"].(map[string]interface{}); ok {
		delete(payload, "response_format")
		textField, _ := payload["text"].(map[string]interface{})
		if textField == nil {
			textField = map[string]interface{}{}
		}
		formatMap := map[string]interface{}{}
		if format, ok := rf["type"].(string); ok && strings.TrimSpace(format) != "" {
			formatMap["type"] = strings.TrimSpace(format)
		}
		if _, ok := formatMap["type"]; !ok {
			formatMap["type"] = "text"
		}
		if schema, ok := rf["json_schema"]; ok {
			formatMap["json_schema"] = schema
		}
		if len(formatMap) > 0 {
			textField["format"] = formatMap
		}
		if len(textField) > 0 {
			payload["text"] = textField
		}
	}
	return json.Marshal(payload)
}

func (s *Server) setResponseSession(id string, base responsesRequest, creq openai.ChatCompletionRequest, adapter string) {
	if strings.TrimSpace(adapter) == "" || strings.TrimSpace(id) == "" {
		return
	}
	baseCopy := base
	baseCopy.ID = id
	baseCopy.ToolOutputs = nil
	reqCopy := respconv.CloneChatCompletionRequest(creq)

	// Note: creq should already be adapted by caller (forwardResponsesToAnthropic)
	// before being passed here, so we don't need to adapt again

	s.responsesSessionsMu.Lock()
	if existing, ok := s.responsesSessions[id]; ok {
		existing.close()
	}
	s.responsesSessions[id] = newResponseSession(adapter, baseCopy, reqCopy)
	s.responsesSessionsMu.Unlock()
}

func (s *Server) recordResponseToolCall(id string, content string, tc openai.ToolCall) {
	if strings.TrimSpace(id) == "" {
		return
	}
	s.responsesSessionsMu.Lock()
	defer s.responsesSessionsMu.Unlock()
	sess, ok := s.responsesSessions[id]
	if !ok {
		return
	}
	msg := openai.ChatMessage{Role: "assistant", Content: content}
	msg.ToolCalls = []openai.ToolCall{tc}
	sess.Request.Messages = append(sess.Request.Messages, msg)
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.session tool_call stored id=%s call_id=%s", id, tc.ID)
	}
}

func (s *Server) applyToolOutputsToSession(id string, outputs []openai.ResponseToolOutput) (openai.ChatCompletionRequest, responsesRequest, string, error) {
	if strings.TrimSpace(id) == "" {
		return openai.ChatCompletionRequest{}, responsesRequest{}, "", errResponseSessionNotFound
	}
	s.responsesSessionsMu.Lock()
	defer s.responsesSessionsMu.Unlock()
	sess, ok := s.responsesSessions[id]
	if !ok {
		return openai.ChatCompletionRequest{}, responsesRequest{}, "", errResponseSessionNotFound
	}

	// Detect duplicate tool calls (same name + args in recent history)
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.session checking for duplicates id=%s, message_count=%d", id, len(sess.Request.Messages))
	}
	duplicateCount, duplicateWarning := s.detectDuplicateToolCalls(sess.Request.Messages)
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.session duplicate_check_result id=%s, count=%d, has_warning=%v", id, duplicateCount, duplicateWarning != "")
	}

	// EMERGENCY STOP: Reject request if 5+ consecutive duplicates detected
	if duplicateCount >= 5 {
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.session EMERGENCY STOP: %d duplicate tool calls detected, rejecting continuation id=%s", duplicateCount, id)
		}
		return openai.ChatCompletionRequest{}, responsesRequest{}, "", fmt.Errorf("infinite loop detected: tool called %d times consecutively with identical arguments - execution halted", duplicateCount)
	}

	for _, out := range outputs {
		toolMsg := openai.ChatMessage{
			Role:       "tool",
			ToolCallID: strings.TrimSpace(out.ToolCallID),
			Content:    out.Output,
		}
		sess.Request.Messages = append(sess.Request.Messages, toolMsg)
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.session tool_output appended id=%s call_id=%s bytes=%d", id, toolMsg.ToolCallID, len(out.Output))
		}
	}

	// If duplicates detected (3-4 times), inject warning into messages
	if duplicateWarning != "" {
		// Find or create system message to add warning
		foundSystem := false
		for i, msg := range sess.Request.Messages {
			if msg.Role == "system" {
				sess.Request.Messages[i].Content = msg.Content + "\n\n" + duplicateWarning
				foundSystem = true
				break
			}
		}
		if !foundSystem {
			// Prepend system message with warning
			sess.Request.Messages = append([]openai.ChatMessage{{
				Role:    "system",
				Content: duplicateWarning,
			}}, sess.Request.Messages...)
		}
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.session duplicate tool calls detected count=%d, warning injected id=%s", duplicateCount, id)
		}
	}

	sess.Request.ToolChoice = nil
	base := sess.Base
	base.ID = id
	base.ToolOutputs = nil
	return respconv.CloneChatCompletionRequest(sess.Request), base, sess.Adapter, nil
}

// detectDuplicateToolCalls checks if the last 3+ tool calls have the same name and arguments
// Returns (duplicateCount, warningMessage). warningMessage is empty if no duplicates detected.
func (s *Server) detectDuplicateToolCalls(messages []openai.ChatMessage) (int, string) {
	// Look for duplicate tool results (Codex sends tool messages, not assistant tool calls)
	type toolResult struct {
		toolCallID string
		content    string
	}

	var recentTools []toolResult
	lookback := 20
	if len(messages) < lookback {
		lookback = len(messages)
	}

	// Scan backwards for tool messages
	for i := len(messages) - lookback; i < len(messages); i++ {
		msg := messages[i]
		// Check for tool messages (Codex pattern)
		if msg.Role == "tool" && msg.ToolCallID != "" {
			recentTools = append(recentTools, toolResult{
				toolCallID: msg.ToolCallID,
				content:    msg.Content,
			})
			if s.isDebug() && s.logger != nil {
				preview := msg.Content
				if len(preview) > 80 {
					preview = preview[:80] + "..."
				}
				s.logger.Printf("responses.duplicate_detection found_tool_result id=%s content_preview=%s", msg.ToolCallID, preview)
			}
		}
		// Also check for assistant messages with ToolCalls (traditional pattern)
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				// Use tool call as signature
				recentTools = append(recentTools, toolResult{
					toolCallID: tc.ID,
					content:    tc.Function.Name + ":" + tc.Function.Arguments,
				})
				if s.isDebug() && s.logger != nil {
					previewArgs := tc.Function.Arguments
					if len(previewArgs) > 80 {
						previewArgs = previewArgs[:80] + "..."
					}
					s.logger.Printf("responses.duplicate_detection found_tool_call name=%s args_preview=%s", tc.Function.Name, previewArgs)
				}
			}
		}
	}

	// Check if last 3+ tool results are identical (indicating repeated execution)
	if len(recentTools) >= 3 {
		last := recentTools[len(recentTools)-1]
		duplicateCount := 1
		// Check up to last 10 tools for duplicates
		for i := len(recentTools) - 2; i >= 0 && i >= len(recentTools)-10; i-- {
			// Compare content (for tool messages) - identical output means same command
			if recentTools[i].content == last.content && last.content != "" {
				duplicateCount++
			} else {
				break
			}
		}

		if duplicateCount >= 3 {
			// Escalating warnings based on severity
			var warningMsg string
			if duplicateCount == 4 {
				warningMsg = fmt.Sprintf("🚨 URGENT WARNING: You have executed the same tool %d times in a row with identical results. This is an infinite loop! The task has already been completed successfully. DO NOT call this tool again. If you call it one more time, execution will be halted. Instead, provide a final text response to the user confirming the task is complete.", duplicateCount)
			} else if duplicateCount >= 3 {
				warningMsg = fmt.Sprintf("⚠️ CRITICAL WARNING: You have executed the same tool %d times in a row with identical results. This is an infinite loop! The task has already been completed successfully. DO NOT call this tool again. Instead, provide a final text response to the user confirming the task is complete.", duplicateCount)
			}

			if s.isDebug() && s.logger != nil {
				contentPreview := last.content
				if len(contentPreview) > 100 {
					contentPreview = contentPreview[:100] + "..."
				}
				s.logger.Printf("responses.duplicate_detection: Detected %d consecutive identical tool results, content_preview=%s", duplicateCount, contentPreview)
			}

			return duplicateCount, warningMsg
		}
	}

	return 0, ""
}

func (s *Server) clearResponseSession(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	s.responsesSessionsMu.Lock()
	sess, ok := s.responsesSessions[id]
	if ok {
		delete(s.responsesSessions, id)
	}
	s.responsesSessionsMu.Unlock()
	if ok && sess != nil {
		sess.close()
	}
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.session cleared id=%s", id)
	}
}

// detectFunctionCallOutputInMessages scans chat history for the most recent tool-role
// message whose tool_call_id matches an active session waiting on tool outputs.
func (s *Server) detectFunctionCallOutputInMessages(messages []openai.ChatMessage) (bool, string) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if !strings.EqualFold(strings.TrimSpace(msg.Role), "tool") {
			continue
		}
		callID := strings.TrimSpace(msg.ToolCallID)
		if callID == "" {
			continue
		}
		if respID := s.findSessionByToolCallID(callID); respID != "" {
			return true, respID
		}
	}
	return false, ""
}

// findSessionByToolCallID finds a session that has the given tool call ID
func (s *Server) findSessionByToolCallID(callID string) string {
	s.responsesSessionsMu.Lock()
	defer s.responsesSessionsMu.Unlock()

	for respID, sess := range s.responsesSessions {
		// Check if this session has a tool call with this ID
		for _, msg := range sess.Request.Messages {
			for _, tc := range msg.ToolCalls {
				if tc.ID == callID {
					return respID
				}
			}
		}
	}
	return ""
}

// handleFunctionCallOutputContinuation handles tool call continuation via new request
func (s *Server) handleFunctionCallOutputContinuation(
	w http.ResponseWriter,
	r *http.Request,
	rr responsesRequest,
	previousRespID string,
	reqStart time.Time,
) {
	// Retrieve session
	s.responsesSessionsMu.Lock()
	sess, ok := s.responsesSessions[previousRespID]
	s.responsesSessionsMu.Unlock()
	if !ok {
		s.respondError(w, http.StatusNotFound, fmt.Errorf("session not found for response_id=%s", previousRespID))
		return
	}

	// Convert the new request to ChatCompletionRequest to get tool outputs
	creq := rr.ToChatCompletionRequest()

	// Append tool output messages to session (filter out unsupported tool errors)
	for _, msg := range creq.Messages {
		if strings.ToLower(msg.Role) == "tool" || strings.TrimSpace(msg.ToolCallID) != "" {
			// Filter out errors that cause infinite retry loops
			if strings.Contains(msg.Content, "unsupported call:") ||
				strings.Contains(msg.Content, "failed to parse function arguments") {
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.continuation: filtered error call_id=%s content=%q", msg.ToolCallID, msg.Content)
				}
				continue
			}
			callID := strings.TrimSpace(msg.ToolCallID)
			if callID == "" || sess.hasToolOutput(callID) {
				continue
			}
			sess.Request.Messages = append(sess.Request.Messages, msg)
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("responses.continuation: appended tool output call_id=%s content=%q", msg.ToolCallID, msg.Content)
			}
		}
	}

	// Update session
	s.responsesSessionsMu.Lock()
	s.responsesSessions[previousRespID] = sess
	s.responsesSessionsMu.Unlock()

	// Continue streaming with updated messages
	stream := rr.Stream || strings.EqualFold(r.URL.Query().Get("stream"), "true")
	buildDur := time.Duration(0)

	// Use fresh timestamp for continuation requests to avoid negative TTFB
	continuationStart := time.Now()

	if sess.Adapter == "anthropic" {
		if err := s.forwardResponsesToAnthropic(w, r, sess.Base, sess.Request, stream, continuationStart, buildDur, previousRespID, sess.Adapter); err != nil {
			s.respondError(w, http.StatusBadGateway, err)
		}
		return
	}

	// Fallback to generic streaming
	s.respondError(w, http.StatusNotImplemented, fmt.Errorf("continuation not implemented for adapter=%s", sess.Adapter))
}
