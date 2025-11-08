package openai

import (
	"encoding/json"
	"testing"
)

func TestResponseRequest_ToChatCompletionRequest_ToolCallConversion(t *testing.T) {
	rr := ResponseRequest{
		Model: "claude-3-5-sonnet-20241022",
		Input: []interface{}{
			map[string]interface{}{
				"role": "user",
				"content": []interface{}{
					map[string]interface{}{
						"type": "input_text",
						"text": "Run a command",
					},
				},
			},
			map[string]interface{}{
				"role": "assistant",
				"content": []interface{}{
					map[string]interface{}{
						"type": "tool_call",
						"id":   "call_123",
						"name": "shell",
						"arguments": map[string]interface{}{
							"command": []interface{}{"ls", "-l"},
						},
					},
				},
			},
			map[string]interface{}{
				"role": "tool",
				"content": []interface{}{
					map[string]interface{}{
						"type":        "tool_result",
						"tool_use_id": "call_123",
						"content": []interface{}{
							map[string]interface{}{
								"type": "output_text",
								"text": "total 0",
							},
						},
					},
				},
			},
		},
	}

	creq := rr.ToChatCompletionRequest()

	if creq.Model != rr.Model {
		t.Fatalf("expected model %s, got %s", rr.Model, creq.Model)
	}

	if len(creq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(creq.Messages))
	}

	user := creq.Messages[0]
	if user.Role != "user" {
		t.Fatalf("expected first role user, got %s", user.Role)
	}
	if user.Content != "Run a command" {
		t.Fatalf("unexpected user content: %q", user.Content)
	}

	assistant := creq.Messages[1]
	if assistant.Role != "assistant" {
		t.Fatalf("expected assistant role, got %s", assistant.Role)
	}
	if len(assistant.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistant.ToolCalls))
	}
	tc := assistant.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Fatalf("expected tool call id call_123, got %s", tc.ID)
	}
	if tc.Function.Name != "shell" {
		t.Fatalf("expected tool name shell, got %s", tc.Function.Name)
	}
	var args map[string][]string
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		t.Fatalf("tool arguments not valid JSON: %v", err)
	}
	if got := args["command"]; len(got) != 2 || got[0] != "ls" || got[1] != "-l" {
		t.Fatalf("unexpected tool arguments: %+v", args)
	}

	tool := creq.Messages[2]
	if tool.Role != "tool" {
		t.Fatalf("expected tool role, got %s", tool.Role)
	}
	if tool.ToolCallID != "call_123" {
		t.Fatalf("expected tool_call_id call_123, got %s", tool.ToolCallID)
	}
	if tool.Content != "total 0" {
		t.Fatalf("unexpected tool content %q", tool.Content)
	}
}
