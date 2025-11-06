package responses

import (
	"context"
	"io"
	"log"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

var _ protocol.ProtocolTranslator[anthropic.NativeRequest, anthropic.NativeResponse] = (*Translator)(nil)

// Translator converts OpenAI-style payloads to Anthropic native payloads for the Responses bridge.
type Translator struct {
	protocol.NoopTranslator[anthropic.NativeRequest, anthropic.NativeResponse]
}

func NewTranslator() *Translator {
	return &Translator{}
}

func (t *Translator) Name() string {
	return "openai.responses_to_anthropic.messages"
}

func (t *Translator) OpenAIToNativeRequest(req openai.ChatCompletionRequest) (anthropic.NativeRequest, error) {
	return anthropic.ConvertChatToNative(req)
}

func (t *Translator) NativeToOpenAIRequest(req anthropic.NativeRequest) (openai.ChatCompletionRequest, error) {
	return anthropic.ConvertNativeToOpenAIRequest(req)
}

func (t *Translator) NativeToOpenAIResponse(resp anthropic.NativeResponse) (openai.ChatCompletionResponse, error) {
	return anthropic.ConvertNativeToOpenAIResponse(resp, resp.Model), nil
}

func (t *Translator) StreamOpenAIToNative(ctx context.Context, model string, r io.Reader, emit func(event string, payload interface{})) error {
	log.Printf("[DEBUG] responses.Translator StreamOpenAIToNative model=%s", model)
	return anthropic.StreamOpenAIToAnthropic(ctx, model, r, emit)
}

func (t *Translator) StreamNativeToOpenAI(ctx context.Context, model string, r io.Reader, emit func(chunk openai.ChatCompletionChunk) error) error {
	log.Printf("[DEBUG] responses.Translator StreamNativeToOpenAI model=%s", model)
	return anthropic.StreamAnthropicToOpenAI(ctx, model, r, emit)
}
