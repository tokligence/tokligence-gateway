package httpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	respconv "github.com/tokligence/tokligence-gateway/internal/httpserver/responses"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type sseEvent struct {
	name string
	data map[string]any
}

func parseSSE(body string) []sseEvent {
	var events []sseEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	var currentName string
	var payload strings.Builder
	flush := func() {
		if currentName == "" || payload.Len() == 0 {
			currentName = ""
			payload.Reset()
			return
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(payload.String()), &data); err == nil {
			events = append(events, sseEvent{name: currentName, data: data})
		}
		currentName = ""
		payload.Reset()
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "event:"):
			currentName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			value := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if value == "[DONE]" {
				flush()
				currentName = ""
				payload.Reset()
				continue
			}
			if payload.Len() > 0 {
				payload.WriteByte('\n')
			}
			payload.WriteString(value)
		case line == "":
			flush()
		}
	}
	flush()
	return events
}

func eventsByName(events []sseEvent, name string) []map[string]any {
	var out []map[string]any
	for _, ev := range events {
		if ev.name == name {
			out = append(out, ev.data)
		}
	}
	return out
}

func TestStreamResponses_ToolCallSequence(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	srv.ssePingInterval = 0

	argsChunk1 := "{\"command\":[\"echo\"]"
	argsChunk2 := ",\"args\":[]}"

	ch := make(chan adapter.StreamEvent, 4)
	ch <- adapter.StreamEvent{
		Chunk: &openai.ChatCompletionChunk{
			ID:      "chunk1",
			Model:   "claude-3-5-sonnet",
			Created: time.Now().Unix(),
			Choices: []openai.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: openai.ChatMessageDelta{
					Role: "assistant",
					ToolCalls: []openai.ToolCallDelta{{
						Index: 0,
						ID:    "toolu_1",
						Type:  "function",
						Function: &openai.ToolFunctionPart{
							Name: "shell",
						},
					}},
				},
			}},
		},
	}
	ch <- adapter.StreamEvent{
		Chunk: &openai.ChatCompletionChunk{
			ID:      "chunk2",
			Model:   "claude-3-5-sonnet",
			Created: time.Now().Unix(),
			Choices: []openai.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: openai.ChatMessageDelta{
					ToolCalls: []openai.ToolCallDelta{{
						Index: 0,
						Type:  "function",
						Function: &openai.ToolFunctionPart{
							Name:      "shell",
							Arguments: argsChunk1,
						},
					}},
				},
			}},
		},
	}
	ch <- adapter.StreamEvent{
		Chunk: &openai.ChatCompletionChunk{
			ID:      "chunk3",
			Model:   "claude-3-5-sonnet",
			Created: time.Now().Unix(),
			Choices: []openai.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: openai.ChatMessageDelta{
					ToolCalls: []openai.ToolCallDelta{{
						Index: 0,
						Type:  "function",
						Function: &openai.ToolFunctionPart{
							Name:      "shell",
							Arguments: argsChunk2,
						},
					}},
				},
			}},
		},
	}
	finish := "tool_calls"
	ch <- adapter.StreamEvent{
		Chunk: &openai.ChatCompletionChunk{
			ID:      "chunk4",
			Model:   "claude-3-5-sonnet",
			Created: time.Now().Unix(),
			Choices: []openai.ChatCompletionChunkChoice{{
				Index:        0,
				FinishReason: &finish,
			}},
		},
	}
	close(ch)

	rr := responsesRequest{
		Model:  "claude-3-5-sonnet",
		Stream: true,
		Tools: []openai.ResponseTool{{
			Type: "function",
			Name: "shell",
		}},
	}
	creq := openai.ChatCompletionRequest{Model: rr.Model, Stream: true}

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()

	srv.streamResponses(rec, req, rr, creq, time.Now(), 0, "", "", func(context.Context, respconv.Conversation) (respconv.StreamInit, error) {
		return respconv.StreamInit{Channel: ch}, nil
	})

	events := parseSSE(rec.Body.String())
	if len(events) == 0 {
		t.Fatalf("no SSE events captured; body=%s", rec.Body.String())
	}

	created := eventsByName(events, "response.created")
	if len(created) == 0 {
		t.Fatalf("response.created missing: %+v", events)
	}

	added := eventsByName(events, "response.output_item.added")
	if len(added) == 0 {
		t.Fatalf("response.output_item.added missing")
	}
	item := added[len(added)-1]["item"].(map[string]any)
	if item["type"] != "function_call" {
		t.Fatalf("expected function_call item, got %#v", item)
	}

	deltas := eventsByName(events, "response.function_call_arguments.delta")
	if len(deltas) != 2 {
		t.Fatalf("expected 2 argument deltas, got %d", len(deltas))
	}
	if deltas[0]["delta"] != argsChunk1 {
		t.Fatalf("first delta mismatch: %v", deltas[0]["delta"])
	}
	if deltas[1]["delta"] != argsChunk2 {
		t.Fatalf("second delta mismatch: %v", deltas[1]["delta"])
	}

	done := eventsByName(events, "response.function_call_arguments.done")
	if len(done) != 1 {
		t.Fatalf("response.function_call_arguments.done missing")
	}
	fullArgs, ok := done[0]["arguments"].(string)
	if !ok {
		t.Fatalf("arguments not string: %#v", done[0]["arguments"])
	}
	if fullArgs != argsChunk1+argsChunk2 {
		t.Fatalf("expected accumulated args %q, got %q", argsChunk1+argsChunk2, fullArgs)
	}

	itemDone := eventsByName(events, "response.output_item.done")
	if len(itemDone) == 0 {
		t.Fatalf("response.output_item.done missing")
	}
	itemPayload := itemDone[len(itemDone)-1]["item"].(map[string]any)
	if itemPayload["status"] != "completed" {
		t.Fatalf("item status not completed: %#v", itemPayload)
	}
	if itemPayload["arguments"] != argsChunk1+argsChunk2 {
		t.Fatalf("item arguments mismatch: %#v", itemPayload)
	}

	completed := eventsByName(events, "response.completed")
	if len(completed) == 0 {
		t.Fatalf("response.completed missing")
	}
	required := eventsByName(events, "response.required_action")
	if len(required) != 1 {
		t.Fatalf("expected response.required_action event, got %d", len(required))
	}
	requiredResp := required[0]["response"].(map[string]any)
	if requiredResp["status"] != "incomplete" {
		t.Fatalf("required_action response status mismatch: %#v", requiredResp["status"])
	}
	requiredPayload := required[0]["required_action"].(map[string]any)
	calls, ok := requiredPayload["submit_tool_outputs"].(map[string]any)["tool_calls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("required_action tool_calls missing: %#v", requiredPayload)
	}
	responseData := completed[0]["response"].(map[string]any)
	if responseData["status"] != "incomplete" {
		t.Fatalf("expected response status incomplete, got %#v", responseData["status"])
	}
	incomplete := responseData["incomplete_details"].(map[string]any)
	if incomplete["reason"] != "tool_calls" {
		t.Fatalf("incomplete reason mismatch: %#v", incomplete)
	}
}

func TestStreamResponses_WrapsSingleArrayShellCommand(t *testing.T) {
	srv := newTestHTTPServer(t, true)
	srv.ssePingInterval = 0

	argsJSON := `{"command": ["echo 'are you ok?' >> ok.py"]}`
	finish := "tool_calls"

	ch := make(chan adapter.StreamEvent, 2)
	ch <- adapter.StreamEvent{
		Chunk: &openai.ChatCompletionChunk{
			ID:      "chunk1",
			Model:   "claude-3-5-sonnet",
			Created: time.Now().Unix(),
			Choices: []openai.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: openai.ChatMessageDelta{
					Role: "assistant",
					ToolCalls: []openai.ToolCallDelta{{
						Index: 0,
						Type:  "function",
						Function: &openai.ToolFunctionPart{
							Name: "shell",
						},
					}},
				},
			}},
		},
	}
	ch <- adapter.StreamEvent{
		Chunk: &openai.ChatCompletionChunk{
			ID:      "chunk2",
			Model:   "claude-3-5-sonnet",
			Created: time.Now().Unix(),
			Choices: []openai.ChatCompletionChunkChoice{{
				Index: 0,
				Delta: openai.ChatMessageDelta{
					ToolCalls: []openai.ToolCallDelta{{
						Index: 0,
						Type:  "function",
						Function: &openai.ToolFunctionPart{
							Name:      "shell",
							Arguments: argsJSON,
						},
					}},
				},
				FinishReason: &finish,
			}},
		},
	}
	close(ch)

	rr := responsesRequest{
		Model:  "claude-3-5-sonnet",
		Stream: true,
		Tools: []openai.ResponseTool{{
			Type: "function",
			Name: "shell",
		}},
	}
	creq := openai.ChatCompletionRequest{Model: rr.Model, Stream: true}

	req := httptest.NewRequest("POST", "/v1/responses", nil)
	rec := httptest.NewRecorder()

	srv.streamResponses(rec, req, rr, creq, time.Now(), 0, "", "", func(context.Context, respconv.Conversation) (respconv.StreamInit, error) {
		return respconv.StreamInit{Channel: ch}, nil
	})

	events := parseSSE(rec.Body.String())
	required := eventsByName(events, "response.required_action")
	if len(required) != 1 {
		t.Fatalf("expected required_action event, got %d", len(required))
	}
	toolCalls, ok := required[0]["required_action"].(map[string]any)["submit_tool_outputs"].(map[string]any)["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected tool_calls array: %#v", required[0]["required_action"])
	}
	fn, ok := toolCalls[0].(map[string]any)["function"].(map[string]any)
	if !ok {
		t.Fatalf("function payload missing: %#v", toolCalls[0])
	}
	argStr, _ := fn["arguments"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(argStr), &payload); err != nil {
		t.Fatalf("failed to parse function arguments: %v", err)
	}
	cmdSlice, ok := payload["command"].([]interface{})
	if !ok {
		t.Fatalf("command not array: %#v", payload["command"])
	}
	if len(cmdSlice) != 3 || cmdSlice[0] != "bash" || cmdSlice[1] != "-c" {
		t.Fatalf("expected bash -c wrapper, got %#v", cmdSlice)
	}
	if cmdSlice[2] != "echo 'are you ok?' >> ok.py" {
		t.Fatalf("unexpected command payload: %#v", cmdSlice[2])
	}
}

// TestStreamResponses_WaitsForToolOutputs was removed because it tested incorrect behavior.
// Per Responses API standard, streams close after emitting tool calls (returning "incomplete" status).
// Clients must submit tool outputs via a new request, not in the same stream session.
// Tool call functionality is adequately covered by TestStreamResponses_ToolCallSequence.
