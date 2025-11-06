package anthropic

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
		StopReason  string `json:"stop_reason,omitempty"`
	} `json:"delta,omitempty"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block,omitempty"`
}

// StreamAnthropicToOpenAI converts Anthropic SSE events to OpenAI chat completion chunks.
func StreamAnthropicToOpenAI(ctx context.Context, model string, r io.Reader, emit func(chunk openai.ChatCompletionChunk) error) error {
	if emit == nil {
		return errors.New("anthropic: openai chunk emitter required")
	}
	log.Printf("[DEBUG] anthropic.StreamAnthropicToOpenAI: begin model=%s", model)

	reader := bufio.NewReader(r)
	roleEmitted := false

	type toolState struct {
		id, name string
		args     strings.Builder
	}
	tools := map[int]*toolState{}

	newChunk := func() openai.ChatCompletionChunk {
		return openai.ChatCompletionChunk{
			ID:      fmt.Sprintf("anthropic-%d", time.Now().UnixNano()),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Model:   model,
			Choices: []openai.ChatCompletionChunkChoice{{Index: 0}},
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("[DEBUG] anthropic.StreamAnthropicToOpenAI: EOF model=%s", model)
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			// event type carried inside data payload as well; no-op.
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			break
		}

		var evt anthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			log.Printf("[DEBUG] anthropic.StreamAnthropicToOpenAI: unable to parse event: %v payload=%s", err, payload)
			continue
		}

		switch evt.Type {
		case "content_block_start":
			if !strings.EqualFold(evt.ContentBlock.Type, "tool_use") {
				continue
			}
			state := &toolState{id: evt.ContentBlock.ID, name: evt.ContentBlock.Name}
			tools[evt.Index] = state

			chunk := newChunk()
			delta := &chunk.Choices[0].Delta
			if !roleEmitted {
				delta.Role = "assistant"
				roleEmitted = true
			}
			delta.ToolCalls = []openai.ToolCallDelta{{
				Index: evt.Index,
				ID:    state.id,
				Type:  "function",
				Function: &openai.ToolFunctionPart{
					Name: state.name,
				},
			}}
			if err := emit(chunk); err != nil {
				return err
			}
		case "content_block_delta":
			if evt.Delta.Text != "" {
				chunk := newChunk()
				delta := &chunk.Choices[0].Delta
				if !roleEmitted {
					delta.Role = "assistant"
					roleEmitted = true
				}
				delta.Content = evt.Delta.Text
				if err := emit(chunk); err != nil {
					return err
				}
				continue
			}
			if evt.Delta.PartialJSON != "" {
				state, ok := tools[evt.Index]
				if !ok {
					state = &toolState{}
					tools[evt.Index] = state
				}
				state.args.WriteString(evt.Delta.PartialJSON)
				chunkDelta := evt.Delta.PartialJSON
				chunk := newChunk()
				delta := &chunk.Choices[0].Delta
				if !roleEmitted {
					delta.Role = "assistant"
					roleEmitted = true
				}
				name := state.name
				delta.ToolCalls = []openai.ToolCallDelta{{
					Index: evt.Index,
					Type:  "function",
					Function: &openai.ToolFunctionPart{
						Name:      name,
						Arguments: chunkDelta,
					},
				}}
				if state.id != "" {
					delta.ToolCalls[0].ID = state.id
				}
				if err := emit(chunk); err != nil {
					return err
				}
			}
		case "message_delta":
			if evt.Delta.StopReason == "" {
				continue
			}
			var finish string
			switch evt.Delta.StopReason {
			case "tool_use":
				finish = "tool_calls"
			case "max_tokens":
				finish = "length"
			default:
				finish = "stop"
			}
			chunk := newChunk()
			chunk.Choices[0].FinishReason = &finish
			if err := emit(chunk); err != nil {
				return err
			}
		case "message_stop":
			log.Printf("[DEBUG] anthropic.StreamAnthropicToOpenAI: message_stop model=%s", model)
			return nil
		}
	}
	return nil
}
