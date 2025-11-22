package responses

import (
	"context"
	"strings"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func TestTranslator_OpenAIToNativeRequest_WithToolCall(t *testing.T) {
	req := openai.ChatCompletionRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []openai.ChatMessage{
			{
				Role: "assistant",
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "lookup",
							Arguments: `{"query":"status"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content:    "all systems go",
			},
		},
	}

	translator := NewTranslator()
	native, err := translator.OpenAIToNativeRequest(req)
	if err != nil {
		t.Fatalf("OpenAIToNativeRequest returned error: %v", err)
	}
	if len(native.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(native.Messages))
	}

	assistant := native.Messages[0]
	if assistant.Role != "assistant" {
		t.Fatalf("expected assistant role, got %s", assistant.Role)
	}
	if len(assistant.Content.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(assistant.Content.Blocks))
	}
	tc := assistant.Content.Blocks[0]
	if tc.Type != "tool_use" {
		t.Fatalf("expected tool_use block, got %s", tc.Type)
	}
	if tc.ID != "call_1" {
		t.Fatalf("expected tool id call_1, got %s", tc.ID)
	}
	if tc.Name != "lookup" {
		t.Fatalf("expected tool name lookup, got %s", tc.Name)
	}
	if tc.Input["query"] != "status" {
		t.Fatalf("expected input query=status, got %v", tc.Input["query"])
	}

	toolResult := native.Messages[1]
	if toolResult.Role != "user" {
		t.Fatalf("expected user role for tool result, got %s", toolResult.Role)
	}
	if len(toolResult.Content.Blocks) != 1 {
		t.Fatalf("expected single tool_result block, got %d", len(toolResult.Content.Blocks))
	}
	resultBlock := toolResult.Content.Blocks[0]
	if resultBlock.Type != "tool_result" {
		t.Fatalf("expected tool_result type, got %s", resultBlock.Type)
	}
	if resultBlock.ToolUseID != "call_1" {
		t.Fatalf("expected ToolUseID call_1, got %s", resultBlock.ToolUseID)
	}
	if len(resultBlock.Content) == 0 || resultBlock.Content[0].Text != "all systems go" {
		t.Fatalf("expected tool result text preserved, got %+v", resultBlock.Content)
	}
}

func TestTranslator_NativeToOpenAIRequest_RoundTripTools(t *testing.T) {
	native := anthropic.NativeRequest{
		Model: "claude-3-5-sonnet-20241022",
		System: anthropic.SystemField{
			Text: "You are helpful.",
		},
		Messages: []anthropic.NativeMessage{
			{
				Role: "user",
				Content: anthropic.NativeContent{
					Blocks: []anthropic.ContentBlock{
						{Type: "text", Text: "hello"},
					},
				},
			},
			{
				Role: "assistant",
				Content: anthropic.NativeContent{
					Blocks: []anthropic.ContentBlock{
						{
							Type: "tool_use",
							ID:   "call_1",
							Name: "lookup",
							Input: map[string]interface{}{
								"query": "status",
							},
						},
					},
				},
			},
			{
				Role: "user",
				Content: anthropic.NativeContent{
					Blocks: []anthropic.ContentBlock{
						{
							Type:      "tool_result",
							ToolUseID: "call_1",
							Content: []anthropic.ContentBlock{
								{Type: "text", Text: "all systems go"},
							},
						},
					},
				},
			},
		},
	}

	translator := NewTranslator()
	creq, err := translator.NativeToOpenAIRequest(native)
	if err != nil {
		t.Fatalf("NativeToOpenAIRequest returned error: %v", err)
	}
	if len(creq.Messages) != 4 {
		t.Fatalf("expected 4 messages (system+user+assistant+tool), got %d", len(creq.Messages))
	}
	if creq.Messages[0].Role != "system" || creq.Messages[0].Content != "You are helpful." {
		t.Fatalf("unexpected system message: %+v", creq.Messages[0])
	}
	assistant := creq.Messages[2]
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistant.ToolCalls))
	}
	if assistant.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("unexpected tool call name: %s", assistant.ToolCalls[0].Function.Name)
	}
	if assistant.ToolCalls[0].Function.Arguments != `{"query":"status"}` {
		t.Fatalf("unexpected tool call args: %s", assistant.ToolCalls[0].Function.Arguments)
	}
	toolResult := creq.Messages[3]
	contentStr, _ := toolResult.Content.(string)
	if toolResult.Role != "tool" || toolResult.ToolCallID != "call_1" || strings.TrimSpace(contentStr) == "" {
		t.Fatalf("unexpected tool result message: %+v", toolResult)
	}
}

func TestTranslator_NativeToOpenAIResponse(t *testing.T) {
	resp := anthropic.NativeResponse{
		ID:         "resp_1",
		Type:       "message",
		Role:       "assistant",
		Model:      "claude-3-5-sonnet-20241022",
		StopReason: "tool_use",
		Content: []anthropic.ContentBlock{
			{Type: "text", Text: "Calling tool..."},
			{
				Type: "tool_use",
				ID:   "call_1",
				Name: "lookup",
				Input: map[string]interface{}{
					"query": "status",
				},
			},
		},
	}

	translator := NewTranslator()
	or, err := translator.NativeToOpenAIResponse(resp)
	if err != nil {
		t.Fatalf("NativeToOpenAIResponse returned error: %v", err)
	}
	if len(or.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(or.Choices))
	}
	choice := or.Choices[0]
	if choice.FinishReason != "tool_calls" {
		t.Fatalf("expected finish_reason tool_calls, got %s", choice.FinishReason)
	}
	if len(choice.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(choice.Message.ToolCalls))
	}
	if choice.Message.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("unexpected tool name: %s", choice.Message.ToolCalls[0].Function.Name)
	}
	if choice.Message.ToolCalls[0].Function.Arguments != `{"query":"status"}` {
		t.Fatalf("unexpected tool args: %s", choice.Message.ToolCalls[0].Function.Arguments)
	}
}

func TestTranslator_StreamOpenAIToNative(t *testing.T) {
	streamData := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"name\":\"lookup\",\"arguments\":\"{\\\"query\\\":\\\"status\\\"}\"}}]}}]}\n\n" +
		"data: [DONE]\n"
	var events []string
	emit := func(event string, payload interface{}) {
		events = append(events, event)
	}
	tr := NewTranslator()
	if err := tr.StreamOpenAIToNative(context.Background(), "claude-3-5-sonnet-20241022", strings.NewReader(streamData), emit); err != nil {
		t.Fatalf("StreamOpenAIToNative error: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected events to be emitted")
	}
	foundMessageStop := false
	for _, ev := range events {
		if ev == "message_stop" {
			foundMessageStop = true
			break
		}
	}
	if !foundMessageStop {
		t.Fatalf("expected message_stop event, got %v", events)
	}
}

func TestTranslator_StreamNativeToOpenAI(t *testing.T) {
	streamData := "" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: content_block_start\n" +
		"data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call_1\",\"name\":\"lookup\"}}\n\n" +
		"event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"partial_json\":\"{\\\"query\\\":\\\"status\\\"}\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"}}\n\n" +
		"event: message_stop\n" +
		"data: {}\n\n"

	var chunks []openai.ChatCompletionChunk
	tr := NewTranslator()
	err := tr.StreamNativeToOpenAI(context.Background(), "claude-3-5-sonnet-20241022", strings.NewReader(streamData), func(chunk openai.ChatCompletionChunk) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamNativeToOpenAI error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected at least one chunk")
	}
	foundFinish := false
	for _, c := range chunks {
		if c.GetDelta().Content != "" {
			if c.GetDelta().Content != "Hello" {
				t.Fatalf("unexpected content delta: %s", c.GetDelta().Content)
			}
		}
		if fr := c.GetFinishReason(); fr != nil && *fr == "tool_calls" {
			foundFinish = true
		}
	}
	if !foundFinish {
		t.Fatalf("expected finish reason tool_calls, chunks=%+v", chunks)
	}
}
