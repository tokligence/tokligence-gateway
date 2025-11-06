package anthropic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

func TestMarshalRequestEncodesContentBlocks(t *testing.T) {
	creq := openai.ChatCompletionRequest{
		Model: "claude",
		Messages: []openai.ChatMessage{
			{Role: "user", Content: "hello world"},
		},
	}

	native, err := ConvertChatToNative(creq)
	if err != nil {
		t.Fatalf("ConvertChatToNative failed: %v", err)
	}

	body, err := MarshalRequest(native)
	if err != nil {
		t.Fatalf("MarshalRequest failed: %v", err)
	}

	var serialized map[string]any
	if err := json.Unmarshal(body, &serialized); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	rawMessages, ok := serialized["messages"].([]any)
	if !ok || len(rawMessages) == 0 {
		t.Fatalf("messages not found: %v", serialized["messages"])
	}
	first, ok := rawMessages[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected message shape: %#v", rawMessages[0])
	}
	contentJSON, err := json.Marshal(first["content"])
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	if got := string(contentJSON); !strings.Contains(got, `"type":"text"`) {
		t.Fatalf("content not encoded as text block: %s", got)
	}
	if got := string(contentJSON); !strings.Contains(got, `"text":"hello world"`) {
		t.Fatalf("content missing text: %s", got)
	}
	if got := string(contentJSON); !strings.HasPrefix(got, "[") {
		t.Fatalf("content not marshaled as array: %s", got)
	}
}

func TestMarshalSystemField(t *testing.T) {
	sys := SystemField{Text: "system message"}
	out, err := json.Marshal(sys)
	if err != nil {
		t.Fatalf("marshal system: %v", err)
	}
	if string(out) != `"system message"` {
		t.Fatalf("expected system text string, got %s", out)
	}

	sysWithBlocks := SystemField{
		Text: "prepend",
		Blocks: []ContentBlock{
			{Type: "text", Text: "block"},
		},
	}
	out, err = json.Marshal(sysWithBlocks)
	if err != nil {
		t.Fatalf("marshal system blocks: %v", err)
	}
	if !strings.HasPrefix(string(out), "[") {
		t.Fatalf("expected array output, got %s", out)
	}
	if !strings.Contains(string(out), `"text":"prepend"`) {
		t.Fatalf("missing text block: %s", out)
	}
}
