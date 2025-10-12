package loopback

import (
	"context"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func TestLoopbackAdapter(t *testing.T) {
	adapter := New()
	resp, err := adapter.CreateCompletion(context.Background(), openai.ChatCompletionRequest{
		Model: "loopback",
		Messages: []openai.ChatMessage{
			{Role: "system", Content: "echo"},
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Fatalf("unexpected role %s", resp.Choices[0].Message.Role)
	}
	if resp.Choices[0].Message.Content != "[loopback] Hello" {
		t.Fatalf("unexpected content %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens == 0 {
		t.Fatalf("expected usage to be recorded")
	}
}

func TestLoopbackAdapterNoMessages(t *testing.T) {
	adapter := New()
	if _, err := adapter.CreateCompletion(context.Background(), openai.ChatCompletionRequest{}); err == nil {
		t.Fatalf("expected error for missing messages")
	}
}
