package httpserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestForwardResponsesToAnthropicSetsDefaultMaxTokens(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	srv.anthAPIKey = "sk-test"
	srv.anthVersion = "2023-06-01"
	srv.anthropicMaxTokens = 2048

	var captured map[string]any
	origClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			defer r.Body.Close()
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				return nil, err
			}
			body := `{"id":"test","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"model":"claude-3","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	t.Cleanup(func() { http.DefaultClient = origClient })
	srv.anthBaseURL = "https://anthropic.test"

	rr := responsesRequest{
		Model: "claude-3",
		Messages: []openai.ChatMessage{
			{Role: "user", Content: "hello"},
		},
		Stream: false,
	}
	creq := rr.ToChatCompletionRequest()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	if err := srv.forwardResponsesToAnthropic(rec, req, rr, creq, false, time.Now(), 0, "", "anthropic"); err != nil {
		t.Fatalf("forwardResponsesToAnthropic error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	raw, ok := captured["max_tokens"]
	if !ok {
		t.Fatalf("max_tokens not set in request: %#v", captured)
	}
	switch v := raw.(type) {
	case float64:
		if int(v) != 2048 {
			t.Fatalf("expected max_tokens 2048, got %v", v)
		}
	case int:
		if v != 2048 {
			t.Fatalf("expected max_tokens 2048, got %d", v)
		}
	default:
		t.Fatalf("unexpected max_tokens type: %T", raw)
	}
}

func TestFormatOpenAIResponsesRequest_ResponseFormat(t *testing.T) {
	req := responsesRequest{
		Model: "gpt-4o-mini",
		ResponseFormat: struct {
			Type       string                 `json:"type"`
			JsonSchema map[string]interface{} `json:"json_schema,omitempty"`
		}{
			Type: "json_schema",
			JsonSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"foo": map[string]interface{}{"type": "string"}},
			},
		},
	}
	body, err := formatOpenAIResponsesRequest(req)
	if err != nil {
		t.Fatalf("formatOpenAIResponsesRequest error: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if _, ok := payload["response_format"]; ok {
		t.Fatalf("response_format should be removed")
	}
	text, ok := payload["text"].(map[string]interface{})
	if !ok {
		t.Fatalf("text field missing or wrong type: %#v", payload["text"])
	}
	formatMap, ok := text["format"].(map[string]interface{})
	if !ok {
		t.Fatalf("text.format should be object: %#v", text["format"])
	}
	if formatMap["type"] != "json_schema" {
		t.Fatalf("unexpected format.type: %v", formatMap["type"])
	}
	if _, ok := formatMap["json_schema"]; !ok {
		t.Fatalf("json_schema missing in text.format: %#v", formatMap)
	}
}

func TestDetectFunctionCallOutputInMessages_FindsActiveSession(t *testing.T) {
	t.Run("explicit chat message", func(t *testing.T) {
		srv := newTestHTTPServer(t, true)
		respID := "resp_test_messages"
		callID := "call_test_123"

		sess := newResponseSession("anthropic", responsesRequest{ID: respID}, openai.ChatCompletionRequest{})
		sess.Request.Messages = append(sess.Request.Messages, openai.ChatMessage{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{{
				ID:   callID,
				Type: "function",
				Function: openai.FunctionCall{
					Name:      "shell",
					Arguments: `{"command":["echo","hi"]}`,
				},
			}},
		})
		srv.responsesSessionsMu.Lock()
		srv.responsesSessions[respID] = sess
		srv.responsesSessionsMu.Unlock()

		msgs := []openai.ChatMessage{
			{
				Role:       "tool",
				ToolCallID: callID,
				Content:    `{"output":"hi","metadata":{"exit_code":0}}`,
			},
		}

		hasContinuation, prevID := srv.detectFunctionCallOutputInMessages(msgs)
		if !hasContinuation {
			t.Fatalf("expected continuation to be detected via messages")
		}
		if prevID != respID {
			t.Fatalf("expected response id %s, got %s", respID, prevID)
		}
	})

	t.Run("Response API function_call_output", func(t *testing.T) {
		srv := newTestHTTPServer(t, true)
		respID := "resp_test_input"
		callID := "call_input_123"

		sess := newResponseSession("anthropic", responsesRequest{ID: respID}, openai.ChatCompletionRequest{})
		sess.Request.Messages = append(sess.Request.Messages, openai.ChatMessage{
			Role: "assistant",
			ToolCalls: []openai.ToolCall{{
				ID:   callID,
				Type: "function",
				Function: openai.FunctionCall{
					Name:      "shell",
					Arguments: `{"command":["echo","hi"]}`,
				},
			}},
		})
		srv.responsesSessionsMu.Lock()
		srv.responsesSessions[respID] = sess
		srv.responsesSessionsMu.Unlock()

		req := responsesRequest{
			Input: []interface{}{
				map[string]interface{}{
					"type":    "function_call_output",
					"call_id": callID,
					"output":  `{"output":"hi","metadata":{"exit_code":0}}`,
				},
			},
		}
		creq := req.ToChatCompletionRequest()

		hasContinuation, prevID := srv.detectFunctionCallOutputInMessages(creq.Messages)
		if !hasContinuation {
			t.Fatalf("expected continuation to be detected via Response API input")
		}
		if prevID != respID {
			t.Fatalf("expected response id %s, got %s", respID, prevID)
		}
	})
}

func TestDetectFunctionCallOutputInMessages_PrefersLatestMatch(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	firstID := "resp_old"
	secondID := "resp_new"
	firstCall := "call_old"
	secondCall := "call_new"

	oldSess := newResponseSession("anthropic", responsesRequest{ID: firstID}, openai.ChatCompletionRequest{})
	oldSess.Request.Messages = append(oldSess.Request.Messages, openai.ChatMessage{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{{
			ID:   firstCall,
			Type: "function",
			Function: openai.FunctionCall{
				Name:      "shell",
				Arguments: "{}",
			},
		}},
	})
	newSess := newResponseSession("anthropic", responsesRequest{ID: secondID}, openai.ChatCompletionRequest{})
	newSess.Request.Messages = append(newSess.Request.Messages, openai.ChatMessage{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{{
			ID:   secondCall,
			Type: "function",
			Function: openai.FunctionCall{
				Name:      "shell",
				Arguments: "{}",
			},
		}},
	})
	srv.responsesSessionsMu.Lock()
	srv.responsesSessions[firstID] = oldSess
	srv.responsesSessions[secondID] = newSess
	srv.responsesSessionsMu.Unlock()

	msgs := []openai.ChatMessage{
		{Role: "tool", ToolCallID: firstCall, Content: "old output"},
		{Role: "tool", ToolCallID: secondCall, Content: "new output"},
	}

	hasContinuation, respID := srv.detectFunctionCallOutputInMessages(msgs)
	if !hasContinuation {
		t.Fatalf("expected continuation to be detected")
	}
	if respID != secondID {
		t.Fatalf("expected latest matching session %s got %s", secondID, respID)
	}
}

func TestResponseSessionHasToolOutput(t *testing.T) {
	sess := newResponseSession("anthropic", responsesRequest{ID: "resp"}, openai.ChatCompletionRequest{})
	sess.Request.Messages = append(sess.Request.Messages, openai.ChatMessage{
		Role:       "tool",
		ToolCallID: "call_123",
		Content:    "result",
	})
	if !sess.hasToolOutput("call_123") {
		t.Fatalf("expected hasToolOutput to find existing call id")
	}
	if sess.hasToolOutput("missing") {
		t.Fatalf("expected missing call id to return false")
	}
}

func TestApplyToolOutputsToSession_WarnsAfterThreeDuplicateTools(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	srv.SetDuplicateToolDetectionEnabled(true)
	respID := "resp_dup_warning"
	sess := newResponseSession("anthropic", responsesRequest{ID: respID}, openai.ChatCompletionRequest{})

	dupContent := `{"output":"Hello, World!","metadata":{"exit_code":0}}`
	for i := 0; i < 3; i++ {
		sess.Request.Messages = append(sess.Request.Messages, openai.ChatMessage{
			Role:       "tool",
			ToolCallID: fmt.Sprintf("call_dup_%d", i),
			Content:    dupContent,
		})
	}

	srv.responsesSessionsMu.Lock()
	srv.responsesSessions[respID] = sess
	srv.responsesSessionsMu.Unlock()

	outputs := []openai.ResponseToolOutput{{
		ToolCallID: "call_new_warning",
		Output:     dupContent,
	}}
	creq, base, adapter, err := srv.applyToolOutputsToSession(respID, outputs)
	if err != nil {
		t.Fatalf("applyToolOutputsToSession returned error: %v", err)
	}
	if adapter != "anthropic" {
		t.Fatalf("unexpected adapter: %s", adapter)
	}
	if base.ID != respID {
		t.Fatalf("base ID mismatch: got %s", base.ID)
	}
	if len(creq.Messages) == 0 || !strings.EqualFold(creq.Messages[0].Role, "system") {
		t.Fatalf("expected warning system message at head of chat, got %+v", creq.Messages)
	}
	if !strings.Contains(creq.Messages[0].Content, "CRITICAL WARNING") {
		t.Fatalf("system message missing warning text: %q", creq.Messages[0].Content)
	}
	last := creq.Messages[len(creq.Messages)-1]
	if !strings.EqualFold(last.Role, "tool") || last.ToolCallID != "call_new_warning" {
		t.Fatalf("expected new tool output appended, got %+v", last)
	}
}

func TestApplyToolOutputsToSession_ErrorsAfterFiveDuplicateTools(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	srv.SetDuplicateToolDetectionEnabled(true)
	respID := "resp_dup_error"
	sess := newResponseSession("anthropic", responsesRequest{ID: respID}, openai.ChatCompletionRequest{})

	dupContent := `{"output":"repeat","metadata":{"exit_code":0}}`
	for i := 0; i < 5; i++ {
		sess.Request.Messages = append(sess.Request.Messages, openai.ChatMessage{
			Role:       "tool",
			ToolCallID: fmt.Sprintf("call_dup_stop_%d", i),
			Content:    dupContent,
		})
	}

	srv.responsesSessionsMu.Lock()
	srv.responsesSessions[respID] = sess
	srv.responsesSessionsMu.Unlock()

	_, _, _, err := srv.applyToolOutputsToSession(respID, []openai.ResponseToolOutput{{
		ToolCallID: "call_stop",
		Output:     dupContent,
	}})
	if err == nil {
		t.Fatalf("expected error when duplicates exceed threshold")
	}
	if !strings.Contains(err.Error(), "infinite loop detected") {
		t.Fatalf("unexpected error: %v", err)
	}
}
