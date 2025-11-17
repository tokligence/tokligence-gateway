package responses

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	openairesp "github.com/tokligence/tokligence-gateway/internal/httpserver/openai/responses"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type recordingClient struct {
	req *http.Request
}

func (c *recordingClient) Do(r *http.Request) (*http.Response, error) {
	c.req = r
	return nil, fmt.Errorf("sentinel error")
}

func TestAnthropicStreamProvider_SetsBetaHeader(t *testing.T) {
	rc := &recordingClient{}
	provider := &AnthropicStreamProvider{
		URL:        "https://example.com/v1/messages",
		APIKey:     "sk-anthropic",
		Version:    "2023-06-01",
		Translator: openairesp.NewTranslator(),
		Client:     rc,
		BetaHeader: "beta-stream",
	}

	base := openai.ResponseRequest{
		Model: "claude-3-5-haiku-20241022",
	}
	chat := openai.ChatCompletionRequest{
		Model: "claude-3-5-haiku-20241022",
		Messages: []openai.ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}
	conv := NewConversation(base, chat)

	if _, err := provider.Stream(context.Background(), conv); err == nil {
		t.Fatalf("expected error from recording client")
	}
	if rc.req == nil {
		t.Fatalf("expected HTTP request to be issued")
	}
	if got := rc.req.Header.Get("anthropic-beta"); got != "beta-stream" {
		t.Fatalf("expected anthropic-beta header %q, got %q", "beta-stream", got)
	}
}
