package protocol

import (
	"context"
	"errors"
	"io"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

var ErrNotImplemented = errors.New("protocol translator: not implemented")

type ProtocolTranslator[NativeReq any, NativeResp any] interface {
	Name() string
	NativeToOpenAIRequest(NativeReq) (openai.ChatCompletionRequest, error)
	OpenAIToNativeRequest(openai.ChatCompletionRequest) (NativeReq, error)
	NativeToOpenAIResponse(NativeResp) (openai.ChatCompletionResponse, error)
	OpenAIToNativeResponse(openai.ChatCompletionResponse) (NativeResp, error)
	StreamOpenAIToNative(ctx context.Context, model string, r io.Reader, emit func(event string, payload interface{})) error
	StreamNativeToOpenAI(ctx context.Context, model string, r io.Reader, emit func(chunk openai.ChatCompletionChunk) error) error
}

type NoopTranslator[NativeReq any, NativeResp any] struct{}

func (NoopTranslator[NativeReq, NativeResp]) Name() string {
	return "noop"
}

func (NoopTranslator[NativeReq, NativeResp]) NativeToOpenAIRequest(NativeReq) (openai.ChatCompletionRequest, error) {
	return openai.ChatCompletionRequest{}, ErrNotImplemented
}

func (NoopTranslator[NativeReq, NativeResp]) OpenAIToNativeRequest(openai.ChatCompletionRequest) (NativeReq, error) {
	var zero NativeReq
	return zero, ErrNotImplemented
}

func (NoopTranslator[NativeReq, NativeResp]) NativeToOpenAIResponse(NativeResp) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{}, ErrNotImplemented
}

func (NoopTranslator[NativeReq, NativeResp]) OpenAIToNativeResponse(openai.ChatCompletionResponse) (NativeResp, error) {
	var zero NativeResp
	return zero, ErrNotImplemented
}

func (NoopTranslator[NativeReq, NativeResp]) StreamOpenAIToNative(ctx context.Context, model string, r io.Reader, emit func(event string, payload interface{})) error {
	return ErrNotImplemented
}

func (NoopTranslator[NativeReq, NativeResp]) StreamNativeToOpenAI(ctx context.Context, model string, r io.Reader, emit func(chunk openai.ChatCompletionChunk) error) error {
	return ErrNotImplemented
}
