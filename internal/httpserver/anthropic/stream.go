package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// StreamOpenAIToAnthropic reads OpenAI SSE chunks and emits Anthropic-style SSE events.
func StreamOpenAIToAnthropic(ctx context.Context, model string, r io.Reader, emit func(event string, payload interface{})) error {
	if emit == nil {
		return errors.New("anthropic: stream emit callback required")
	}
	log.Printf("anthropic.StreamOpenAIToAnthropic: begin model=%s", model)
	emit("message_start", map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id":      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			"type":    "message",
			"role":    "assistant",
			"model":   model,
			"content": []interface{}{},
		},
	})

	reader := bufio.NewReader(r)
	textStarted := false
	totalText := ""
	type toolBuf struct {
		id, name string
		idx      int
		args     string
	}
	toolByIdx := map[int]*toolBuf{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			break
		}

		var chunk openai.ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			log.Printf("anthropic.StreamOpenAIToAnthropic: unable to parse chunk: %v", err)
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			if !textStarted {
				emit("content_block_start", map[string]interface{}{
					"type":          "content_block_start",
					"index":         0,
					"content_block": map[string]interface{}{"type": "text", "text": ""},
				})
				textStarted = true
			}
			totalText += delta.Content
			emit("content_block_delta", map[string]interface{}{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]interface{}{"type": "text_delta", "text": delta.Content},
			})
		}
		for _, tc := range delta.ToolCalls {
			b, ok := toolByIdx[tc.Index]
			if !ok {
				b = &toolBuf{idx: tc.Index}
				toolByIdx[tc.Index] = b
			}
			if tc.ID != "" {
				b.id = tc.ID
			}
			if tc.Function.Name != "" {
				b.name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				b.args += tc.Function.Arguments
			}
		}
	}

	if textStarted {
		emit("content_block_stop", map[string]interface{}{"type": "content_block_stop", "index": 0})
	}
	if len(toolByIdx) > 0 {
		idxs := make([]int, 0, len(toolByIdx))
		for k := range toolByIdx {
			idxs = append(idxs, k)
		}
		sort.Ints(idxs)
		for i, idx := range idxs {
			tb := toolByIdx[idx]
			input := map[string]interface{}{}
			if strings.TrimSpace(tb.args) != "" && json.Valid([]byte(tb.args)) {
				var obj interface{}
				if err := json.Unmarshal([]byte(tb.args), &obj); err == nil {
					input, _ = obj.(map[string]interface{})
				}
			}
			emit("content_block_start", map[string]interface{}{
				"type":  "content_block_start",
				"index": i + 1,
				"content_block": map[string]interface{}{
					"type":  "tool_use",
					"id":    tb.id,
					"name":  tb.name,
					"input": input,
				},
			})
			emit("content_block_stop", map[string]interface{}{"type": "content_block_stop", "index": i + 1})
		}
	}

	emit("message_delta", map[string]interface{}{
		"type":  "message_delta",
		"delta": map[string]interface{}{"stop_reason": "end_turn"},
		"usage": map[string]int{
			"input_tokens":  0,
			"output_tokens": len(totalText) / 4,
		},
	})
	emit("message_stop", map[string]interface{}{"type": "message_stop"})
	log.Printf("anthropic.StreamOpenAIToAnthropic: completed model=%s text_bytes=%d tool_calls=%d", model, len(totalText), len(toolByIdx))
	return nil
}
