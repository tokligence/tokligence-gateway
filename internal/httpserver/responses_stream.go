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
	respconv "github.com/tokligence/tokligence-gateway/internal/httpserver/responses"
	"github.com/tokligence/tokligence-gateway/internal/openai"
)

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
	responseID string,
	adapterName string,
	init func(context.Context, respconv.Conversation) (respconv.StreamInit, error),
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

	respID := strings.TrimSpace(responseID)
	if respID == "" {
		respID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}
	currentReq := creq
	currentBase := rr
	currentBase.ID = respID
	respCreated := time.Now().Unix()
	if adapterName != "" {
		s.setResponseSession(respID, currentBase, currentReq, adapterName)
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.stream session created id=%s adapter=%s", respID, adapterName)
		}
	}

	emit("response.created", map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":         respID,
			"object":     "response",
			"status":     "in_progress",
			"created_at": respCreated,
		},
	})

	var ticker *time.Ticker
	if s.ssePingInterval > 0 {
		ticker = time.NewTicker(s.ssePingInterval)
		defer ticker.Stop()
	}

	var firstDeltaAt time.Time

	for {
		convForRun := respconv.NewConversation(currentBase, currentReq).EnsureID(respID)
		initRes, err := init(r.Context(), convForRun)
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
					"model":      currentReq.Model,
				},
			})
			if flusher != nil {
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
			s.clearResponseSession(respID)
			return
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
					"model":      currentReq.Model,
				},
			})
			if flusher != nil {
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
			if initRes.Cleanup != nil {
				initRes.Cleanup()
			}
			s.clearResponseSession(respID)
			return
		}

		runFirstDeltaAt := time.Time{}
		var refused bool
		var refusalReason string
		var jsonBuf strings.Builder
		structured := strings.EqualFold(currentBase.ResponseFormat.Type, "json_object") || strings.EqualFold(currentBase.ResponseFormat.Type, "json_schema")
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

		var toolCallPending bool

		// fixShellCommandArgs normalizes shell commands to ["bash","-c",...] when needed.
		fixShellCommandArgs := func(args string) string {
			// Input validation: check for empty args
			if strings.TrimSpace(args) == "" {
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream WARNING: Empty command args")
				}
				return args
			}

			// Input validation: check for size limits (prevent DoS)
			const maxCommandSize = 50000 // 50KB limit
			if len(args) > maxCommandSize {
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream WARNING: Command too large: %d bytes (max %d)", len(args), maxCommandSize)
				}
				return args // Return original, don't process oversized commands
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream WARNING: Args not valid JSON: %v", err)
				}
				return args // Return original if not valid JSON
			}

			needsShellWrapper := func(cmd string) bool {
				if strings.TrimSpace(cmd) == "" {
					return false
				}
				if strings.ContainsAny(cmd, "\n|&;><") {
					return true
				}
				if strings.Contains(cmd, ">>") || strings.Contains(cmd, "<<") {
					return true
				}
				if strings.ContainsAny(cmd, " '\"") {
					return true
				}
				return strings.IndexFunc(cmd, func(r rune) bool {
					return r == '\t'
				}) >= 0
			}

			if cmdVal, ok := parsed["command"]; ok {
				switch v := cmdVal.(type) {
				case string:
					originalCmdStr := v
					cmdStr := strings.TrimSpace(v)
					// Check if the string is actually a JSON-encoded array
					// Anthropic sometimes returns: {"command": "[\"cmd\"]"} instead of {"command": "cmd"}
					if strings.HasPrefix(cmdStr, "[") && strings.HasSuffix(cmdStr, "]") {
						// Try to parse as JSON array
						var cmdArray []string
						if err := json.Unmarshal([]byte(cmdStr), &cmdArray); err == nil && len(cmdArray) > 0 {
							// Successfully parsed as array, use the first element as the actual command
							cmdStr = cmdArray[0]
							if s.isDebug() && s.logger != nil {
								previewLen := 100
								if len(originalCmdStr) < previewLen {
									previewLen = len(originalCmdStr)
								}
								s.logger.Printf("responses.stream INFO: Parsed JSON array command successfully, preview: %s...", originalCmdStr[:previewLen])
							}
						} else {
							// JSON parsing failed, likely malformed JSON - strip [ and ] manually
							// This happens when Anthropic returns malformed JSON like: ["cmd with unescaped "quotes"]
							if s.isDebug() && s.logger != nil {
								previewLen := 100
								if len(cmdStr) < previewLen {
									previewLen = len(cmdStr)
								}
								s.logger.Printf("responses.stream WARNING: JSON array parse failed, using manual strip fallback. Error: %v, preview: %s...", err, cmdStr[:previewLen])
							}
							stripped := strings.TrimPrefix(cmdStr, "[")
							stripped = strings.TrimSuffix(stripped, "]")
							stripped = strings.TrimSpace(stripped)
							// Remove surrounding quotes if present
							if strings.HasPrefix(stripped, "\"") && strings.HasSuffix(stripped, "\"") {
								stripped = stripped[1 : len(stripped)-1]
							}
							// Unescape common sequences - comprehensive handling
							stripped = strings.ReplaceAll(stripped, "\\n", "\n")
							stripped = strings.ReplaceAll(stripped, "\\t", "\t")
							stripped = strings.ReplaceAll(stripped, "\\r", "\r")
							stripped = strings.ReplaceAll(stripped, "\\\"", "\"")
							// Handle escaped backslash (must be done last to avoid double-processing)
							stripped = strings.ReplaceAll(stripped, "\\\\", "\\")
							cmdStr = stripped
						}
					}

					parsed["command"] = []string{"bash", "-c", cmdStr}
					if fixed, err := json.Marshal(parsed); err == nil {
						return string(fixed)
					} else if s.isDebug() && s.logger != nil {
						s.logger.Printf("responses.stream WARNING: Failed to marshal fixed command: %v", err)
					}
				case []interface{}:
					cmdParts := make([]string, 0, len(v))
					for _, part := range v {
						if str, ok := part.(string); ok {
							cmdParts = append(cmdParts, str)
						} else {
							cmdParts = nil
							break
						}
					}
					if cmdParts != nil && len(cmdParts) == 1 && needsShellWrapper(cmdParts[0]) {
						parsed["command"] = []string{"bash", "-c", cmdParts[0]}
						if fixed, err := json.Marshal(parsed); err == nil {
							return string(fixed)
						} else if s.isDebug() && s.logger != nil {
							s.logger.Printf("responses.stream WARNING: Failed to marshal fixed command array: %v", err)
						}
					}
				}
			}

			return args // Return original if no conversion needed
		}

		finalizeOnce := func() {
			if completedSent {
				return
			}
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("responses.stream finalizing (pending_tool=%t)...", toolCallOutputItemAdded)
			}
			var outputArray []any
			var requiredAction any
			if toolCallOutputItemAdded {
				completeArgs := toolCallArgs.String()
				if strings.TrimSpace(completeArgs) == "" {
					completeArgs = "{}"
				}

				// Fix shell command arguments if needed (string -> array conversion)
				if toolCallName == "shell" {
					originalArgs := completeArgs
					completeArgs = fixShellCommandArgs(completeArgs)
					if s.isDebug() && s.logger != nil && originalArgs != completeArgs {
						s.logger.Printf("responses.stream converted shell args in finalizeOnce: %s -> %s", originalArgs, completeArgs)
					}
				}

				tc := openai.ToolCall{
					ID:   toolCallID,
					Type: "function",
					Function: openai.FunctionCall{
						Name:      toolCallName,
						Arguments: completeArgs,
					},
				}
				s.recordResponseToolCall(respID, fullText.String(), tc)
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream tool_call finalize id=%s call_id=%s name=%s args=%s", respID, toolCallID, toolCallName, completeArgs)
				}
				outputArray = []any{
					map[string]any{
						"id":        toolCallItemID,
						"type":      "function_call",
						"name":      toolCallName,
						"status":    "in_progress",
						"call_id":   toolCallID,
						"arguments": completeArgs,
					},
				}
				requiredAction = map[string]any{
					"type": "submit_tool_outputs",
					"submit_tool_outputs": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"id":   toolCallID,
								"type": "function",
								"function": map[string]any{
									"name":      toolCallName,
									"arguments": completeArgs,
								},
							},
						},
					},
				}
				toolCallPending = true
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
				if strings.EqualFold(currentBase.ResponseFormat.Type, "json_object") || strings.EqualFold(currentBase.ResponseFormat.Type, "json_schema") {
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
			if toolCallPending {
				responseStatus = "incomplete"
				incompleteDetails = map[string]any{
					"reason": "tool_calls",
				}
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream required_action=%v", requiredAction)
				}
			}
			responsePayload := map[string]any{
				"id":                 respID,
				"object":             "response",
				"created_at":         respCreated,
				"status":             responseStatus,
				"model":              currentReq.Model,
				"output":             outputArray,
				"incomplete_details": incompleteDetails,
				"required_action":    requiredAction,
				"reasoning": map[string]any{
					"effort":  nil,
					"summary": nil,
				},
			}
			if requiredAction != nil {
				emit("response.required_action", map[string]any{
					"type":            "response.required_action",
					"response":        responsePayload,
					"required_action": requiredAction,
				})
			}
			emit("response.completed", map[string]any{
				"type":     "response.completed",
				"response": responsePayload,
			})
			completedSent = true
			if s.isDebug() && s.logger != nil {
				s.logger.Printf("responses.stream emitted response.completed status=%s", responseStatus)
			}
			if flusher != nil {
				flusher.Flush()
			}
		}

	streamLoop:
		for {
			var ev adapter.StreamEvent
			var ok bool
			if ticker != nil {
				select {
				case ev, ok = <-ch:
				case <-ticker.C:
					emit("ping", map[string]any{})
					continue
				case <-r.Context().Done():
					ok = false
				}
			} else {
				select {
				case ev, ok = <-ch:
				case <-r.Context().Done():
					ok = false
				}
			}
			if !ok {
				break streamLoop
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
				if initRes.Cleanup != nil {
					initRes.Cleanup()
				}
				s.clearResponseSession(respID)
				if s.isDebug() && s.logger != nil {
					s.logger.Printf("responses.stream error (mid): %v", ev.Error)
				}
				return
			}
			if ev.Chunk == nil {
				continue
			}
			chunk := ev.Chunk
			delta := chunk.GetDelta().Content
			if strings.TrimSpace(delta) != "" {
				if firstDeltaAt.IsZero() {
					firstDeltaAt = time.Now()
				}
				if runFirstDeltaAt.IsZero() {
					runFirstDeltaAt = time.Now()
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
					// Fix shell command arguments if needed (string -> array conversion)
					if toolCallName == "shell" {
						originalArgs := completeArgs
						completeArgs = fixShellCommandArgs(completeArgs)
						if s.isDebug() && s.logger != nil && originalArgs != completeArgs {
							s.logger.Printf("responses.stream converted shell args: %s -> %s", originalArgs, completeArgs)
						}
					}
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
				break streamLoop
			}
		}

		finalizeOnce()
		if refused {
			emit("response.refusal.delta", map[string]string{"type": "response.refusal.delta", "reason": refusalReason})
			emit("response.refusal.done", map[string]any{"type": "response.refusal.done"})
		}
		if initRes.Cleanup != nil {
			initRes.Cleanup()
		}

		if !toolCallPending {
			break
		}

		// Responses API standard: close stream after emitting required_action
		// Client will send a new request with function_call_output in input
		if s.isDebug() && s.logger != nil {
			s.logger.Printf("responses.stream closing with pending tool calls (standard Responses API), session preserved id=%s", respID)
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
			time.Sleep(50 * time.Millisecond)
		}
		// Session is preserved for next request with tool outputs
		// Do NOT clear session here - client will resume via new request
		if s.logger != nil {
			total := time.Since(reqStart)
			ttfb := firstDeltaAt.Sub(reqStart)
			if s.isDebug() {
				s.logger.Printf("responses.stream total_ms=%d ttfb_ms=%d build_ms=%d model=%s (incomplete, awaiting tool outputs)",
					total.Milliseconds(), ttfb.Milliseconds(), buildDur.Milliseconds(), creq.Model)
			}
			if adapterName != "" {
				s.logger.Printf("responses.%s.stream total_ms=%d model=%s", adapterName, total.Milliseconds(), creq.Model)
			}
		}
		return
	}

	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
	}
	s.clearResponseSession(respID)
	if s.logger != nil {
		total := time.Since(reqStart)
		var ttfb time.Duration
		if !firstDeltaAt.IsZero() {
			ttfb = firstDeltaAt.Sub(reqStart)
		}
		s.logger.Printf("responses.stream total_ms=%d ttfb_ms=%d build_ms=%d model=%s", total.Milliseconds(), ttfb.Milliseconds(), buildDur.Milliseconds(), currentReq.Model)
	}
}
