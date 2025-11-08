package responses

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	anthpkg "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// AnthropicsTranslator captures the subset of translator behavior the provider relies on.
type AnthropicsTranslator interface {
	OpenAIToNativeRequest(openai.ChatCompletionRequest) (anthpkg.NativeRequest, error)
	StreamNativeToOpenAI(context.Context, string, io.Reader, func(openai.ChatCompletionChunk) error) error
}

// HTTPClient abstracts http.Client for improved testability.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// TokenGuard enforces provider-specific token ceilings.
type TokenGuard func(maxTokens int) error

// AnthropicStreamProvider issues native Messages API requests for streaming runs.
type AnthropicStreamProvider struct {
	URL         string
	APIKey      string
	Version     string
	Translator  AnthropicsTranslator
	Client      HTTPClient
	GuardTokens TokenGuard
}

// Stream implements the StreamProvider interface.
func (p *AnthropicStreamProvider) Stream(ctx context.Context, conv Conversation) (StreamInit, error) {
	if p == nil {
		return StreamInit{}, errors.New("anthropic provider is nil")
	}
	if p.Translator == nil {
		return StreamInit{}, errors.New("anthropic translator missing")
	}
	if strings.TrimSpace(p.URL) == "" {
		return StreamInit{}, errors.New("anthropic stream URL missing")
	}
	chatReq := conv.CloneChat()
	chatReq.Stream = true

	// Debug: log system message content
	// for _, msg := range chatReq.Messages {
	// 	if strings.ToLower(msg.Role) == "system" {
	// 		fmt.Printf("[provider_anthropic] System message to Anthropic: %s\n", msg.Content[:min(200, len(msg.Content))])
	// 	}
	// }

	nativeReq, err := p.Translator.OpenAIToNativeRequest(chatReq)
	if err != nil {
		return StreamInit{}, err
	}
	if nativeReq.MaxTokens == 0 && chatReq.MaxTokens != nil {
		nativeReq.MaxTokens = *chatReq.MaxTokens
	}
	if p.GuardTokens != nil {
		if err := p.GuardTokens(nativeReq.MaxTokens); err != nil {
			return StreamInit{}, err
		}
	}
	nativeReq.Stream = true
	body, err := anthpkg.MarshalRequest(nativeReq)
	if err != nil {
		return StreamInit{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return StreamInit{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(p.Version) != "" {
		req.Header.Set("anthropic-version", p.Version)
	}
	if strings.TrimSpace(p.APIKey) != "" {
		req.Header.Set("x-api-key", p.APIKey)
	}
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return StreamInit{}, err
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return StreamInit{}, fmt.Errorf("anthropic stream status %d: %s", resp.StatusCode, string(previewBytes(raw, 512)))
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
		err := p.Translator.StreamNativeToOpenAI(ctx, nativeReq.Model, resp.Body, func(chunk openai.ChatCompletionChunk) error {
			chunk.Model = chatReq.Model
			emit(adapter.StreamEvent{Chunk: &chunk})
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			emit(adapter.StreamEvent{Error: err})
		}
	}()
	return StreamInit{
		Channel: ch,
		Cleanup: func() { _ = resp.Body.Close() },
	}, nil
}

func (p *AnthropicStreamProvider) httpClient() HTTPClient {
	if p.Client != nil {
		return p.Client
	}
	return http.DefaultClient
}

func previewBytes(b []byte, max int) []byte {
	if len(b) <= max {
		return b
	}
	return b[:max]
}
