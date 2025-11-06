package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/openai"
)

// NativeRequest represents Anthropic /v1/messages payload.
type NativeRequest struct {
	Model       string          `json:"model"`
	Messages    []NativeMessage `json:"messages"`
	System      SystemField     `json:"system,omitempty"`
	Tools       []Tool          `json:"tools,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
}

// Tool mirrors Anthropic tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// NativeMessage represents an Anthropic conversation turn.
type NativeMessage struct {
	Role    string        `json:"role"`
	Content NativeContent `json:"content"`
}

// NativeContent supports string or array of blocks.
type NativeContent struct {
	Blocks []ContentBlock
}

// ContentBlock captures text/tool_use/tool_result blocks.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`

	// tool_use fields
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`

	// tool_result fields
	ToolUseID string         `json:"tool_use_id,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
	Content   []ContentBlock `json:"content,omitempty"`
}

// SystemField supports string or array<content_block>.
type SystemField struct {
	Text   string
	Blocks []ContentBlock
}

// NativeResponse models minimal Anthropic response.
type NativeResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	Model      string         `json:"model"`
	StopReason string         `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ConvertChatToNative maps OpenAI ChatCompletionRequest into Anthropic messages payload.
func ConvertChatToNative(req openai.ChatCompletionRequest) (NativeRequest, error) {
	out := NativeRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if req.MaxTokens != nil {
		out.MaxTokens = *req.MaxTokens
	}
	if tools := convertTools(req.Tools); len(tools) > 0 {
		out.Tools = tools
	}

	var systemParts []string
	autoToolCounter := 0
	var pendingToolIDs []string

	appendSystem := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		systemParts = append(systemParts, text)
	}

	for idx, msg := range req.Messages {
		role := strings.ToLower(msg.Role)
		switch role {
		case "system":
			appendSystem(msg.Content)
		case "user":
			text := strings.TrimSpace(msg.Content)
			if text == "" {
				continue
			}
			out.Messages = append(out.Messages, NativeMessage{
				Role: "user",
				Content: NativeContent{
					Blocks: []ContentBlock{{Type: "text", Text: text}},
				},
			})
		case "assistant":
			var blocks []ContentBlock
			if strings.TrimSpace(msg.Content) != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: msg.Content})
			}
			for toolIdx, tc := range msg.ToolCalls {
				id := strings.TrimSpace(tc.ID)
				if id == "" {
					autoToolCounter++
					id = fmt.Sprintf("tool_call_%d_%d", idx, autoToolCounter)
				}
				pendingToolIDs = append(pendingToolIDs, id)
				name := strings.TrimSpace(tc.Function.Name)
				if name == "" {
					name = fmt.Sprintf("function_%d_%d", idx, toolIdx)
				}
				input := map[string]interface{}{}
				rawArgs := strings.TrimSpace(tc.Function.Arguments)
				if rawArgs != "" {
					if err := json.Unmarshal([]byte(rawArgs), &input); err != nil {
						input = map[string]interface{}{"_raw": rawArgs}
					}
				}
				blocks = append(blocks, ContentBlock{
					Type:  "tool_use",
					ID:    id,
					Name:  name,
					Input: input,
				})
			}
			if len(blocks) == 0 {
				continue
			}
			out.Messages = append(out.Messages, NativeMessage{
				Role: "assistant",
				Content: NativeContent{
					Blocks: blocks,
				},
			})
		case "tool":
			toolID := strings.TrimSpace(msg.ToolCallID)
			if toolID == "" {
				if len(pendingToolIDs) > 0 {
					toolID = pendingToolIDs[0]
					pendingToolIDs = pendingToolIDs[1:]
				} else {
					autoToolCounter++
					toolID = fmt.Sprintf("tool_result_%d_%d", idx, autoToolCounter)
				}
			} else {
				for i, pending := range pendingToolIDs {
					if pending == toolID {
						pendingToolIDs = append(pendingToolIDs[:i], pendingToolIDs[i+1:]...)
						break
					}
				}
			}
			text := strings.TrimSpace(msg.Content)
			var content []ContentBlock
			if text != "" {
				content = append(content, ContentBlock{Type: "text", Text: text})
			}
			out.Messages = append(out.Messages, NativeMessage{
				Role: "user",
				Content: NativeContent{
					Blocks: []ContentBlock{{
						Type:      "tool_result",
						ToolUseID: toolID,
						Content:   content,
					}},
				},
			})
		default:
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			out.Messages = append(out.Messages, NativeMessage{
				Role: "user",
				Content: NativeContent{
					Blocks: []ContentBlock{{Type: "text", Text: msg.Content}},
				},
			})
		}
	}

	if len(systemParts) > 0 {
		out.System = SystemField{Text: strings.Join(systemParts, "\n\n")}
	}

	if len(out.Messages) == 0 {
		return out, errors.New("anthropic: no user/assistant messages after filtering system messages")
	}
	return out, nil
}

func ConvertNativeToOpenAIRequest(req NativeRequest) (openai.ChatCompletionRequest, error) {
	var messages []openai.ChatMessage
	systemText := ExtractSystemText(req.System)
	if strings.TrimSpace(systemText) != "" {
		messages = append(messages, openai.ChatMessage{Role: "system", Content: systemText})
	}

	for _, msg := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "user":
			var texts []string
			for _, block := range msg.Content.Blocks {
				switch strings.ToLower(block.Type) {
				case "text":
					if strings.TrimSpace(block.Text) != "" {
						texts = append(texts, block.Text)
					}
				case "tool_result":
					var contentStr string
					if len(block.Content) > 0 {
						for _, c := range block.Content {
							if strings.EqualFold(c.Type, "text") {
								contentStr += c.Text
							}
						}
					} else {
						contentStr = block.Text
					}
					messages = append(messages, openai.ChatMessage{
						Role:       "tool",
						Content:    contentStr,
						ToolCallID: block.ToolUseID,
					})
				}
			}
			if len(texts) > 0 {
				messages = append(messages, openai.ChatMessage{
					Role:    "user",
					Content: strings.Join(texts, "\n\n"),
				})
			}
		case "assistant":
			var textParts []string
			var toolCalls []openai.ToolCall
			for _, block := range msg.Content.Blocks {
				switch strings.ToLower(block.Type) {
				case "text":
					if block.Text != "" {
						textParts = append(textParts, block.Text)
					}
				case "tool_use":
					args := "{}"
					if len(block.Content) > 0 {
						// treat nested content as text builder (rare)
						var builder strings.Builder
						for _, c := range block.Content {
							if strings.EqualFold(c.Type, "text") {
								builder.WriteString(c.Text)
							}
						}
						if builder.Len() > 0 {
							args = builder.String()
						}
					}
					if block.Input != nil {
						if raw, err := json.Marshal(block.Input); err == nil {
							args = string(raw)
						}
					}
					toolCalls = append(toolCalls, openai.ToolCall{
						ID:   block.ID,
						Type: "function",
						Function: openai.FunctionCall{
							Name:      block.Name,
							Arguments: args,
						},
					})
				}
			}
			assistant := openai.ChatMessage{Role: "assistant"}
			if len(textParts) > 0 {
				assistant.Content = strings.Join(textParts, "\n\n")
			}
			if len(toolCalls) > 0 {
				assistant.ToolCalls = toolCalls
			}
			messages = append(messages, assistant)
		default:
			// Treat unknown roles as user text fallback
			var texts []string
			for _, block := range msg.Content.Blocks {
				if strings.EqualFold(block.Type, "text") && strings.TrimSpace(block.Text) != "" {
					texts = append(texts, block.Text)
				}
			}
			if len(texts) > 0 {
				messages = append(messages, openai.ChatMessage{
					Role:    "user",
					Content: strings.Join(texts, "\n\n"),
				})
			}
		}
	}

	var maxTokensPtr *int
	if req.MaxTokens > 0 {
		maxTokensPtr = &req.MaxTokens
	}

	return openai.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   maxTokensPtr,
		Tools:       nativeToolsToOpenAI(req.Tools),
	}, nil
}

func ConvertNativeToOpenAIResponse(resp NativeResponse, originalModel string) openai.ChatCompletionResponse {
	var content strings.Builder
	var toolCalls []openai.ToolCall

	for _, block := range resp.Content {
		switch strings.ToLower(block.Type) {
		case "text":
			content.WriteString(block.Text)
		case "tool_use":
			if block.Input == nil {
				continue
			}
			argsBytes, err := json.Marshal(block.Input)
			if err != nil {
				continue
			}
			toolCalls = append(toolCalls, openai.ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: openai.FunctionCall{
					Name:      block.Name,
					Arguments: string(argsBytes),
				},
			})
		}
	}

	finishReason := "stop"
	switch strings.ToLower(resp.StopReason) {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "tool_use":
		finishReason = "tool_calls"
	}

	message := openai.ChatMessage{
		Role:    "assistant",
		Content: content.String(),
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
		finishReason = "tool_calls"
	}

	return openai.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   originalModel,
		Choices: []openai.ChatCompletionChoice{
			{
				Index:        0,
				FinishReason: finishReason,
				Message:      message,
			},
		},
		Usage: openai.UsageBreakdown{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

func MarshalRequest(req NativeRequest) ([]byte, error) {
	return json.Marshal(req)
}

// ExtractSystemText flattens system field into plain text.
func ExtractSystemText(sys SystemField) string {
	if strings.TrimSpace(sys.Text) != "" {
		return sys.Text
	}
	var b strings.Builder
	for _, block := range sys.Blocks {
		if strings.EqualFold(block.Type, "text") {
			b.WriteString(block.Text)
		}
	}
	return b.String()
}

// ClampMaxTokens enforces a completion token cap in raw JSON payloads.
func ClampMaxTokens(body []byte, capTokens int) []byte {
	if capTokens <= 0 || len(body) == 0 {
		return body
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	if v, ok := m["max_tokens"]; ok {
		switch t := v.(type) {
		case float64:
			if int(t) > capTokens {
				m["max_tokens"] = capTokens
			}
		case int:
			if t > capTokens {
				m["max_tokens"] = capTokens
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return body
	}
	return b
}

func convertTools(tools []openai.Tool) []Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		if !strings.EqualFold(t.Type, "function") {
			continue
		}
		name := strings.TrimSpace(t.Function.Name)
		if name == "" {
			continue
		}
		out = append(out, Tool{
			Name:        name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func nativeToolsToOpenAI(tools []Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		if strings.TrimSpace(t.Name) == "" {
			continue
		}
		out = append(out, openai.Tool{
			Type: "function",
			Function: openai.ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MarshalJSON ensures Anthropic messages receive an array of content blocks.
func (c NativeContent) MarshalJSON() ([]byte, error) {
	if len(c.Blocks) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(c.Blocks)
}

// UnmarshalJSON for NativeContent supports multiple shapes.
func (c *NativeContent) UnmarshalJSON(b []byte) error {
	btrim := strings.TrimSpace(string(b))
	if len(btrim) > 0 && btrim[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		c.Blocks = []ContentBlock{{Type: "text", Text: s}}
		return nil
	}
	if len(btrim) > 0 && btrim[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		if raw, ok := obj["text"]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				c.Blocks = []ContentBlock{{Type: "text", Text: s}}
				return nil
			}
		}
		if raw, ok := obj["content"]; ok {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				c.Blocks = []ContentBlock{{Type: "text", Text: s}}
				return nil
			}
			var arr []ContentBlock
			if err := json.Unmarshal(raw, &arr); err == nil {
				c.Blocks = arr
				return nil
			}
		}
		var arr []ContentBlock
		if err := json.Unmarshal(b, &arr); err == nil {
			c.Blocks = arr
			return nil
		}
		c.Blocks = nil
		return nil
	}
	var arr []ContentBlock
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	c.Blocks = arr
	return nil
}

// UnmarshalJSON for ContentBlock tolerates flexible tool_result shapes.
func (b *ContentBlock) UnmarshalJSON(data []byte) error {
	type alias ContentBlock
	var a alias
	if err := json.Unmarshal(data, (*alias)(&a)); err == nil {
		*b = ContentBlock(a)
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if v, ok := raw["type"]; ok {
		_ = json.Unmarshal(v, &b.Type)
	}
	if v, ok := raw["text"]; ok {
		_ = json.Unmarshal(v, &b.Text)
	}
	if v, ok := raw["id"]; ok {
		_ = json.Unmarshal(v, &b.ID)
	}
	if v, ok := raw["name"]; ok {
		_ = json.Unmarshal(v, &b.Name)
	}
	if v, ok := raw["input"]; ok {
		var anyv interface{}
		if err := json.Unmarshal(v, &anyv); err == nil {
			if m, ok := anyv.(map[string]interface{}); ok {
				b.Input = m
			}
		}
	}
	if v, ok := raw["tool_use_id"]; ok {
		_ = json.Unmarshal(v, &b.ToolUseID)
	}
	if v, ok := raw["is_error"]; ok {
		_ = json.Unmarshal(v, &b.IsError)
	}
	if v, ok := raw["content"]; ok && len(v) > 0 && string(v) != "null" {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			b.Content = []ContentBlock{{Type: "text", Text: s}}
			return nil
		}
		var arr []ContentBlock
		if err := json.Unmarshal(v, &arr); err == nil {
			b.Content = arr
			return nil
		}
		var one ContentBlock
		if err := json.Unmarshal(v, &one); err == nil {
			b.Content = []ContentBlock{one}
			return nil
		}
	}
	return nil
}

// MarshalJSON encodes the system field in Anthropic-compatible form.
func (s SystemField) MarshalJSON() ([]byte, error) {
	text := strings.TrimSpace(s.Text)
	switch {
	case len(s.Blocks) > 0 && text != "":
		blocks := make([]ContentBlock, 0, len(s.Blocks)+1)
		blocks = append(blocks, ContentBlock{Type: "text", Text: text})
		blocks = append(blocks, s.Blocks...)
		return json.Marshal(blocks)
	case len(s.Blocks) > 0:
		return json.Marshal(s.Blocks)
	case text != "":
		return json.Marshal(text)
	default:
		return []byte("[]"), nil
	}
}

// UnmarshalJSON for SystemField allows string or array of blocks.
func (s *SystemField) UnmarshalJSON(b []byte) error {
	btrim := strings.TrimSpace(string(b))
	if btrim == "" || btrim == "null" {
		return nil
	}
	if len(btrim) > 0 && btrim[0] == '"' {
		var text string
		if err := json.Unmarshal(b, &text); err != nil {
			return err
		}
		s.Text = text
		return nil
	}
	var arr []ContentBlock
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	s.Blocks = arr
	return nil
}
