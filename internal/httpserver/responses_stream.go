package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type responsesStreamInit struct {
	Channel <-chan adapter.StreamEvent
	Cleanup func()
}

// streamResponses orchestrates the streaming SSE lifecycle for the OpenAI Responses API.
// The init callback is responsible for provisioning a stream of adapter.StreamEvent values
// (either from a native adapter or a translator-backed bridge). It is invoked after the
// response.created prelude is emitted so that error paths can still surface structured SSE.
func (s *Server) streamResponses(
	w http.ResponseWriter,
	r *http.Request,
	rr responsesRequest,
	creq openai.ChatCompletionRequest,
	reqStart time.Time,
	buildDur time.Duration,
	init func(context.Context) (responsesStreamInit, error),
) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, _ := w.(http.Flusher)
	if s.isDebug() && s.logger != nil {
		s.logger.Printf("responses.stream start model=%s tools=%d tool_choice=%t", creq.Model, len(creq.Tools), creq.ToolChoice != nil)
	}

	var sequenceNum int
	emit := func(event string, payload any) {
		fmt.Fprintf(w, "event: %s\n", event)
		if payload != nil {
			if m, ok := payload.(map[string]any); ok {
				m["sequence_number"] = sequenceNum
				sequenceNum++
			}
			b, _ := json.Marshal(payload)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
		} else {
			fmt.Fprint(w, "data: {}\n\n")
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	respCreated := time.Now().Unix()

	emit("response.created", map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":         respID,
			"object":     "response",
			"status":     "in_progress",
			"created_at": respCreated,
		},
	})

	initRes, err := init(r.Context())
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("responses.stream error: %v", err)
		}
		emit("error", map[string]string{
			"type":    "error",
			"message": err.Error(),
		})
		emit("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         respID,
				"object":     "response",
				"status":     "failed",
				"created_at": respCreated,
				"model":      creq.Model,
			},
		})
		if flusher != nil {
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
		return
	}
	if initRes.Cleanup != nil {
		defer initRes.Cleanup()
	}
	ch := initRes.Channel
	if ch == nil {
		emit("error", map[string]string{
			"type":    "error",
			"message": "stream unavailable",
		})
		emit("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         respID,
				"object":     "response",
				"status":     "failed",
				"created_at": respCreated,
				"model":      creq.Model,
			},
		})
		if flusher != nil {
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
		return
	}

	firstDeltaAt := time.Time{}
	var refused bool
	var refusalReason string
	var jsonBuf strings.Builder
	structured := strings.EqualFold(rr.ResponseFormat.Type, "json_object") || strings.EqualFold(rr.ResponseFormat.Type, "json_schema")

	var ticker *time.Ticker
	if s.ssePingInterval > 0 {
		ticker = time.NewTicker(s.ssePingInterval)
		defer ticker.Stop()
	}
	completedSent := false
	outputItemAdded := false
	contentPartAdded := false
	var itemID string
	var fullText strings.Builder

	toolCallOutputItemAdded := false
	var toolCallItemID string
	var toolCallName string
	var toolCallID string
	var toolCallArgs strings.Builder

	finalizeOnce := func() {
		if completedSent {
			return
		}
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.stream finalizing...")
		}
		var outputArray []any
		if toolCallOutputItemAdded {
			completeArgs := toolCallArgs.String()
			outputArray = []any{
				map[string]any{
					"id":        toolCallItemID,
					"type":      "function_call",
					"name":      toolCallName,
					"call_id":   toolCallID,
					"arguments": completeArgs,
				},
			}
		} else {
			completeText := fullText.String()
			emit("response.output_text.done", map[string]any{
				"type":          "response.output_text.done",
				"item_id":       itemID,
				"output_index":  0,
				"content_index": 0,
				"text":          completeText,
				"logprobs":      []any{},
			})
			if strings.EqualFold(rr.ResponseFormat.Type, "json_object") || strings.EqualFold(rr.ResponseFormat.Type, "json_schema") {
				emit("response.output_json.done", map[string]any{
					"type":          "response.output_json.done",
					"item_id":       itemID,
					"output_index":  0,
					"content_index": 0,
				})
				if structured {
					var tmp interface{}
					if err := json.Unmarshal([]byte(jsonBuf.String()), &tmp); err != nil {
						emit("response.error", map[string]string{
							"type":    "response.error",
							"message": "invalid_json",
						})
					}
				}
			}
			if contentPartAdded {
				emit("response.content_part.done", map[string]any{
					"type":          "response.content_part.done",
					"item_id":       itemID,
					"output_index":  0,
					"content_index": 0,
					"part": map[string]any{
						"type":        "output_text",
						"annotations": []any{},
						"logprobs":    []any{},
						"text":        completeText,
					},
				})
			}
			if outputItemAdded {
				emit("response.output_item.done", map[string]any{
					"type":         "response.output_item.done",
					"output_index": 0,
					"item": map[string]any{
						"id":     itemID,
						"type":   "message",
						"status": "completed",
						"content": []any{
							map[string]any{
								"type":        "output_text",
								"annotations": []any{},
								"logprobs":    []any{},
								"text":        completeText,
							},
						},
						"role": "assistant",
					},
				})
			}
			outputArray = []any{
				map[string]any{
					"id":     itemID,
					"type":   "message",
					"status": "completed",
					"content": []any{
						map[string]any{
							"type":        "output_text",
							"annotations": []any{},
							"logprobs":    []any{},
							"text":        completeText,
						},
					},
					"role": "assistant",
				},
			}
		}

		responseStatus := "completed"
		var incompleteDetails any
		if toolCallOutputItemAdded {
			responseStatus = "incomplete"
			incompleteDetails = map[string]any{
				"reason": "tool_calls",
			}
		}
		emit("response.completed", map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":                 respID,
				"object":             "response",
				"created_at":         respCreated,
				"status":             responseStatus,
				"model":              creq.Model,
				"output":             outputArray,
				"incomplete_details": incompleteDetails,
				"reasoning": map[string]any{
					"effort":  nil,
					"summary": nil,
				},
			},
		})
		completedSent = true
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.stream emitted response.completed, flushing...")
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	for {
		var ev adapter.StreamEvent
		var ok bool
		if ticker != nil {
			select {
			case ev, ok = <-ch:
			case <-ticker.C:
				emit("ping", map[string]any{})
				continue
			}
		} else {
			ev, ok = <-ch
		}
		if !ok {
			break
		}
		if ev.Error != nil {
			emit("error", map[string]string{
				"type":    "error",
				"message": ev.Error.Error(),
			})
			finalizeOnce()
			if flusher != nil {
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("responses.stream error (mid): %v", ev.Error)
			}
			return
		}
		if ev.Chunk != nil {
			chunk := ev.Chunk
			delta := chunk.GetDelta().Content
			if strings.TrimSpace(delta) != "" {
				if firstDeltaAt.IsZero() {
					firstDeltaAt = time.Now()
				}
				if !outputItemAdded {
					itemID = fmt.Sprintf("msg_%d", time.Now().UnixNano())
					emit("response.output_item.added", map[string]any{
						"type":         "response.output_item.added",
						"output_index": 0,
						"item": map[string]any{
							"id":      itemID,
							"type":    "message",
							"status":  "in_progress",
							"content": []any{},
							"role":    "assistant",
						},
					})
					outputItemAdded = true
				}
				if !contentPartAdded {
					emit("response.content_part.added", map[string]any{
						"type":          "response.content_part.added",
						"item_id":       itemID,
						"output_index":  0,
						"content_index": 0,
						"part": map[string]any{
							"type":        "output_text",
							"annotations": []any{},
							"logprobs":    []any{},
							"text":        "",
						},
					})
					contentPartAdded = true
				}
				emit("response.output_text.delta", map[string]any{
					"type":          "response.output_text.delta",
					"item_id":       itemID,
					"output_index":  0,
					"content_index": 0,
					"delta":         delta,
					"logprobs":      []any{},
				})
				fullText.WriteString(delta)
				if flusher != nil {
					flusher.Flush()
				}
				if s.isDebug() && s.logger != nil {
					preview := delta
					if len(preview) > 64 {
						preview = preview[:64]
					}
					s.logger.Printf("responses.stream delta text bytes=%d preview=%q", len(delta), preview)
				}
				if structured {
					emit("response.output_json.delta", map[string]any{
						"type":          "response.output_json.delta",
						"output_index":  0,
						"content_index": 0,
						"delta":         delta,
					})
					jsonBuf.WriteString(delta)
				}
			}
			if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
				for _, tcd := range chunk.Choices[0].Delta.ToolCalls {
					if !toolCallOutputItemAdded {
						toolCallItemID = fmt.Sprintf("fc_%d", time.Now().UnixNano())
						toolCallID = fmt.Sprintf("call_%d", time.Now().UnixNano())
						if tcd.Function != nil && strings.TrimSpace(tcd.Function.Name) != "" {
							toolCallName = tcd.Function.Name
						}
						emit("response.output_item.added", map[string]any{
							"type":         "response.output_item.added",
							"output_index": 0,
							"item": map[string]any{
								"id":        toolCallItemID,
								"type":      "function_call",
								"status":    "in_progress",
								"arguments": "",
								"call_id":   toolCallID,
								"name":      toolCallName,
							},
						})
						toolCallOutputItemAdded = true
					}
					if tcd.Function != nil && strings.TrimSpace(tcd.Function.Arguments) != "" {
						argsChunk := tcd.Function.Arguments
						emit("response.function_call_arguments.delta", map[string]any{
							"type":         "response.function_call_arguments.delta",
							"item_id":      toolCallItemID,
							"output_index": 0,
							"delta":        argsChunk,
						})
						toolCallArgs.WriteString(argsChunk)
						emit("response.tool_call.delta", map[string]any{
							"type":         "response.tool_call.delta",
							"item_id":      toolCallItemID,
							"output_index": 0,
							"delta":        argsChunk,
						})
						if s.isDebug() && s.logger != nil {
							s.logger.Printf("responses.stream tool_call delta emitted")
						}
					}
				}
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
				fr := strings.ToLower(strings.TrimSpace(*chunk.Choices[0].FinishReason))
				if fr == "content_filter" {
					refused = true
					refusalReason = fr
				}
				if fr == "tool_calls" {
					completeArgs := toolCallArgs.String()
					emit("response.function_call_arguments.done", map[string]any{
						"type":         "response.function_call_arguments.done",
						"item_id":      toolCallItemID,
						"output_index": 0,
						"arguments":    completeArgs,
					})
					emit("response.output_item.done", map[string]any{
						"type":         "response.output_item.done",
						"output_index": 0,
						"item": map[string]any{
							"id":        toolCallItemID,
							"type":      "function_call",
							"status":    "completed",
							"arguments": completeArgs,
							"call_id":   toolCallID,
							"name":      toolCallName,
						},
					})
				}
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream finish_reason=%s, ending stream", fr)
				}
				break
			}
		}
	}

	finalizeOnce()
	if refused {
		emit("response.refusal.delta", map[string]string{"type": "response.refusal.delta", "reason": refusalReason})
		emit("response.refusal.done", map[string]any{"type": "response.refusal.done"})
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}
	if s.logger != nil {
		total := time.Since(reqStart)
		var ttfb time.Duration
		if !firstDeltaAt.IsZero() {
			ttfb = firstDeltaAt.Sub(reqStart)
		}
		s.logger.Printf("responses.stream total_ms=%d ttfb_ms=%d build_ms=%d model=%s", total.Milliseconds(), ttfb.Milliseconds(), buildDur.Milliseconds(), creq.Model)
	}
}
