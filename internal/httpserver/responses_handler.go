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
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	anthpkg "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	openairesp "github.com/tokligence/tokligence-gateway/internal/httpserver/openai/responses"
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

func (s *Server) HandleResponses(w http.ResponseWriter, r *http.Request) {
	s.handleResponses(w, r)
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	reqStart := time.Now()
	var rr responsesRequest
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		s.respondError(w, http.StatusBadRequest, fmt.Errorf("invalid responses json: %w", err))
		return
	}

	stream := rr.Stream || strings.EqualFold(r.URL.Query().Get("stream"), "true")
	mode := strings.ToLower(strings.TrimSpace(s.responsesDelegateOpenAI))
	if mode == "" {
		mode = "auto"
	}
	adapterName, adapterErr := s.resolveAdapterForModel(rr.Model)
	adapterName = strings.ToLower(strings.TrimSpace(adapterName))

	switch mode {
	case "always":
		if strings.TrimSpace(s.openaiAPIKey) == "" {
			s.respondError(w, http.StatusBadGateway, errors.New("responses delegation forced to OpenAI (mode=always) but TOKLIGENCE_OPENAI_API_KEY is not configured"))
			return
		}
		if adapterErr != nil {
			s.respondError(w, http.StatusBadRequest, fmt.Errorf("responses delegation forced to OpenAI (mode=always) but model %q is not routable: %v", rr.Model, adapterErr))
			return
		}
		if adapterName != "" && adapterName != "openai" {
			s.respondError(w, http.StatusBadRequest, fmt.Errorf("responses delegation forced to OpenAI (mode=always) but model %q is routed to %q; disable mode=always to translate instead", rr.Model, adapterName))
			return
		}
		s.delegateOpenAIResponses(w, r.Context(), rr, stream, reqStart)
		return
	case "auto":
		if adapterErr == nil && adapterName == "openai" && strings.TrimSpace(s.openaiAPIKey) != "" {
			s.delegateOpenAIResponses(w, r.Context(), rr, stream, reqStart)
			return
		}
	}

	buildStart := time.Now()
	creq := rr.ToChatCompletionRequest()
	creq.Stream = stream
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
		if err := s.forwardResponsesToAnthropic(w, r, rr, creq, stream, reqStart, buildDur); err != nil {
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
			s.streamResponses(w, r, rr, creq, reqStart, buildDur, func(ctx context.Context) (responsesStreamInit, error) {
				ch, err := sa.CreateCompletionStream(ctx, creq)
				if err != nil {
					return responsesStreamInit{}, err
				}
				return responsesStreamInit{Channel: ch}, nil
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
	body, _ := json.Marshal(rr)
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

func (s *Server) forwardResponsesToAnthropic(w http.ResponseWriter, r *http.Request, rr responsesRequest, creq openai.ChatCompletionRequest, stream bool, reqStart time.Time, buildDur time.Duration) error {
	if strings.TrimSpace(s.anthAPIKey) == "" {
		return errors.New("anthropic API key not configured for Responses translation")
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
	areq.Stream = stream
	if err := s.guardAnthropicTokens(areq.MaxTokens); err != nil {
		return err
	}
	s.debugf("responses translator converting to anthropic model=%s stream=%v", areq.Model, stream)
	body, err := anthpkg.MarshalRequest(areq)
	if err != nil {
		return err
	}
	url := strings.TrimRight(s.anthBaseURL, "/") + "/v1/messages"
	if q := strings.TrimSpace(r.URL.RawQuery); q != "" {
		url = url + "?" + q
	}

	if stream {
		s.streamResponses(w, r, rr, creq, reqStart, buildDur, func(ctx context.Context) (responsesStreamInit, error) {
			reqUp, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				return responsesStreamInit{}, err
			}
			reqUp.Header.Set("Content-Type", "application/json")
			reqUp.Header.Set("Accept", "text/event-stream")
			reqUp.Header.Set("anthropic-version", s.anthVersion)
			if s.anthAPIKey != "" {
				reqUp.Header.Set("x-api-key", s.anthAPIKey)
			}
			resp, err := http.DefaultClient.Do(reqUp)
			if err != nil {
				return responsesStreamInit{}, err
			}
			if resp.StatusCode >= 400 {
				raw, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				return responsesStreamInit{}, bridgeUpstreamError{status: resp.StatusCode, body: raw}
			}
			ch := make(chan adapter.StreamEvent)
			go func() {
				defer close(ch)
				emit := func(ev adapter.StreamEvent) {
					select {
					case <-ctx.Done():
					case ch <- ev:
					}
				}
				err := s.responsesTranslator.StreamNativeToOpenAI(ctx, areq.Model, resp.Body, func(chunk openai.ChatCompletionChunk) error {
					chunk.Model = creq.Model
					emit(adapter.StreamEvent{Chunk: &chunk})
					return nil
				})
				if err != nil && !errors.Is(err, context.Canceled) {
					emit(adapter.StreamEvent{Error: err})
				}
			}()
			return responsesStreamInit{
				Channel: ch,
				Cleanup: func() { _ = resp.Body.Close() },
			}, nil
		})
		if s.logger != nil {
			s.logger.Printf("responses.anthropic.stream total_ms=%d model=%s", time.Since(reqStart).Milliseconds(), areq.Model)
		}
		return nil
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
	return nil
}
