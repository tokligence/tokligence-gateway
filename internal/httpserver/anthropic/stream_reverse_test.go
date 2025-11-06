package anthropic

import (
	"context"
	"strings"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func TestStreamAnthropicToOpenAI_ToolCallSequence(t *testing.T) {
	const sse = "" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"shell\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"partial_json\",\"partial_json\":\"{\\\"command\\\":[\\\"echo\\\"\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"partial_json\",\"partial_json\":\",\\\"args\\\":[]}\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"type\":\"stop_reason\",\"stop_reason\":\"tool_use\"}}\n\n" +
		"event: message_stop\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	var chunks []openai.ChatCompletionChunk
	err := StreamAnthropicToOpenAI(context.Background(), "claude-3-sonnet", strings.NewReader(sse), func(chunk openai.ChatCompletionChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamAnthropicToOpenAI returned error: %v", err)
	}

	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 chunks, got %d", len(chunks))
	}

	first := chunks[0]
	if len(first.Choices) != 1 {
		t.Fatalf("first chunk choices=%d", len(first.Choices))
	}
	tc := first.Choices[0].Delta.ToolCalls
	if len(tc) != 1 {
		t.Fatalf("expected first chunk to include tool call, got %v", first.Choices[0].Delta)
	}
	if tc[0].Function == nil || tc[0].Function.Name != "shell" {
		t.Fatalf("unexpected tool name: %#v", tc[0].Function)
	}
	if tc[0].ID != "toolu_1" {
		t.Fatalf("tool ID not propagated: %#v", tc[0])
	}

	partial := chunks[1].Choices[0].Delta.ToolCalls
	if partial[0].Function == nil || partial[0].Function.Arguments != "{\"command\":[\"echo\"" {
		t.Fatalf("first argument chunk mismatch: %#v", partial[0].Function)
	}

	combined := chunks[2].Choices[0].Delta.ToolCalls
	if combined[0].Function == nil || combined[0].Function.Arguments != ",\"args\":[]}" {
		t.Fatalf("second argument chunk mismatch: %#v", combined[0].Function)
	}

	final := chunks[len(chunks)-1]
	if final.Choices[0].FinishReason == nil || *final.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish reason mismatch: %#v", final.Choices[0].FinishReason)
	}
}
