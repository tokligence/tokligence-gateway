package adapter

import (
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "sort"
    "strings"
    "time"
)

// Minimal subset of the sidecar adapter used by the in-process bridge.

type AnthropicMessageRequest struct {
    Model         string          `json:"model"`
    System        json.RawMessage `json:"system,omitempty"`
    Messages      []AnthropicMsg  `json:"messages"`
    Tools         []AnthropicTool `json:"tools,omitempty"`
    MaxTokens     int             `json:"max_tokens,omitempty"`
    Temperature   *float64        `json:"temperature,omitempty"`
    StopSequences []string        `json:"stop_sequences,omitempty"`
    Stream        bool            `json:"stream,omitempty"`
}

type AnthropicMsg struct {
    Role    string          `json:"role"`
    Content json.RawMessage `json:"content"`
}

type AnthropicContent struct {
    Type      string           `json:"type"`
    Text      string           `json:"text,omitempty"`
    ID        string           `json:"id,omitempty"`
    Name      string           `json:"name,omitempty"`
    Input     *json.RawMessage `json:"input,omitempty"`
    ToolUseID string           `json:"tool_use_id,omitempty"`
    Content   interface{}      `json:"content,omitempty"`
}

type AnthropicTool struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    InputSchema map[string]interface{} `json:"input_schema"`
}

type OpenAIChatRequest struct {
    Model       string          `json:"model"`
    Messages    []OpenAIMessage `json:"messages"`
    Tools       []OpenAITool    `json:"tools,omitempty"`
    Temperature *float64        `json:"temperature,omitempty"`
    MaxTokens   int             `json:"max_tokens,omitempty"`
    Stop        []string        `json:"stop,omitempty"`
    Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
    Role       string           `json:"role"`
    Content    interface{}      `json:"content,omitempty"`
    Name       string           `json:"name,omitempty"`
    ToolCallID string           `json:"tool_call_id,omitempty"`
    ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type OpenAITool struct {
    Type     string         `json:"type"`
    Function OpenAIFunction `json:"function"`
}
type OpenAIFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description,omitempty"`
    Parameters  map[string]interface{} `json:"parameters,omitempty"`
}
type OpenAIToolCallFunction struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}
type OpenAIToolCall struct {
    ID       string                 `json:"id"`
    Type     string                 `json:"type"`
    Function OpenAIToolCallFunction `json:"function"`
}

type OpenAIStreamChunk struct {
    ID      string `json:"id"`
    Object  string `json:"object"`
    Model   string `json:"model"`
    Choices []struct {
        Index int `json:"index"`
        Delta struct {
            Role      string `json:"role,omitempty"`
            Content   string `json:"content,omitempty"`
            ToolCalls []struct {
                ID       string `json:"id,omitempty"`
                Type     string `json:"type"`
                Index    int    `json:"index"`
                Function struct {
                    Name      string `json:"name,omitempty"`
                    Arguments string `json:"arguments,omitempty"`
                } `json:"function"`
            } `json:"tool_calls,omitempty"`
        } `json:"delta"`
        FinishReason string `json:"finish_reason,omitempty"`
    } `json:"choices"`
}

func parseAnthropicContent(raw json.RawMessage) ([]AnthropicContent, error) {
    if len(raw) == 0 || string(raw) == "null" { return nil, nil }
    var s string
    if json.Unmarshal(raw, &s) == nil {
        return []AnthropicContent{{Type: "text", Text: s}}, nil
    }
    var arr []AnthropicContent
    if json.Unmarshal(raw, &arr) == nil {
        return arr, nil
    }
    return nil, fmt.Errorf("unsupported content: %s", string(raw))
}

func mapToolsToOpenAI(tools []AnthropicTool) []OpenAITool {
    if len(tools) == 0 { return nil }
    out := make([]OpenAITool, 0, len(tools))
    for _, t := range tools {
        out = append(out, OpenAITool{ Type: "function", Function: OpenAIFunction{Name: t.Name, Description: t.Description, Parameters: t.InputSchema} })
    }
    return out
}

func systemToOpenAI(sysRaw json.RawMessage) *OpenAIMessage {
    if len(sysRaw) == 0 || string(sysRaw) == "null" { return nil }
    var s string
    if json.Unmarshal(sysRaw, &s) == nil && strings.TrimSpace(s) != "" {
        msg := OpenAIMessage{Role: "system", Content: s}
        return &msg
    }
    if parts, _ := parseAnthropicContent(sysRaw); len(parts) > 0 {
        var buf []string
        for _, p := range parts { if p.Type == "text" && strings.TrimSpace(p.Text) != "" { buf = append(buf, p.Text) } }
        if len(buf) > 0 { msg := OpenAIMessage{Role: "system", Content: strings.Join(buf, "\n\n")}; return &msg }
    }
    return nil
}

// AnthropicToOpenAI converts Anthropic request to OpenAI request.
func AnthropicToOpenAI(areq AnthropicMessageRequest) (OpenAIChatRequest, error) {
    var out []OpenAIMessage
    if sm := systemToOpenAI(areq.System); sm != nil { out = append(out, *sm) }
    for _, m := range areq.Messages {
        parts, err := parseAnthropicContent(m.Content)
        if err != nil { return OpenAIChatRequest{}, err }
        switch strings.ToLower(m.Role) {
        case "user":
            var texts []string
            for _, p := range parts { if p.Type == "text" && strings.TrimSpace(p.Text) != "" { texts = append(texts, p.Text) } }
            if len(texts) > 0 { out = append(out, OpenAIMessage{Role: "user", Content: strings.Join(texts, "\n\n")}) }
            for _, p := range parts {
                if p.Type == "tool_result" {
                    contentStr := ""
                    switch v := p.Content.(type) {
                    case string: contentStr = v
                    case nil: contentStr = ""
                    default:
                        b, _ := json.Marshal(v)
                        contentStr = string(b)
                    }
                    out = append(out, OpenAIMessage{Role: "tool", ToolCallID: p.ToolUseID, Content: contentStr})
                }
            }
        case "assistant":
            var txt []string
            var toolCalls []OpenAIToolCall
            for _, p := range parts {
                if p.Type == "text" && p.Text != "" { txt = append(txt, p.Text) }
                if p.Type == "tool_use" {
                    args := "{}"
                    if p.Input != nil && *p.Input != nil { args = string(*p.Input) }
                    toolCalls = append(toolCalls, OpenAIToolCall{ID: p.ID, Type: "function", Function: OpenAIToolCallFunction{Name: p.Name, Arguments: args}})
                }
            }
            msg := OpenAIMessage{Role: "assistant"}
            if len(txt) > 0 { msg.Content = strings.Join(txt, "\n\n") }
            if len(toolCalls) > 0 { msg.ToolCalls = toolCalls }
            out = append(out, msg)
        }
    }
    return OpenAIChatRequest{ Model: areq.Model, Messages: out, Tools: mapToolsToOpenAI(areq.Tools), Temperature: areq.Temperature, MaxTokens: areq.MaxTokens, Stop: areq.StopSequences, Stream: areq.Stream }, nil
}

// ConvertOpenAIStreamToAnthropic converts OpenAI SSE chunks to Anthropic-style events; emits event name + payload.
func ConvertOpenAIStreamToAnthropic(ctx context.Context, requestedModel string, body io.Reader, enc func(event string, payload interface{})) error {
    enc("message_start", map[string]interface{}{"type": "message_start", "message": map[string]interface{}{"id": fmt.Sprintf("msg_%d", time.Now().UnixNano()), "type": "message", "role": "assistant", "model": requestedModel, "content": []interface{}{}}})
    sentTextStart := false
    totalText := ""
    type toolBuf struct{ id, name string; idx int; args string }
    toolByIdx := map[int]*toolBuf{}
    reader := bufio.NewReader(body)
    for {
        select { case <-ctx.Done(): return ctx.Err(); default: }
        line, err := reader.ReadString('\n')
        if err != nil { if errors.Is(err, io.EOF) { break }; break }
        line = strings.TrimSpace(line)
        if line == "" || !strings.HasPrefix(line, "data: ") { continue }
        payload := strings.TrimPrefix(line, "data: ")
        if payload == "[DONE]" { break }
        var chunk OpenAIStreamChunk
        if err := json.Unmarshal([]byte(payload), &chunk); err != nil { continue }
        if len(chunk.Choices) == 0 { continue }
        d := chunk.Choices[0].Delta
        if d.Content != "" {
            if !sentTextStart {
                enc("content_block_start", map[string]interface{}{"type": "content_block_start", "index": 0, "content_block": map[string]interface{}{"type": "text", "text": ""}})
                sentTextStart = true
            }
            totalText += d.Content
            enc("content_block_delta", map[string]interface{}{"type": "content_block_delta", "index": 0, "delta": map[string]interface{}{"type": "text_delta", "text": d.Content}})
        }
        if len(d.ToolCalls) > 0 {
            for _, tc := range d.ToolCalls {
                b, ok := toolByIdx[tc.Index]
                if !ok { b = &toolBuf{idx: tc.Index}; toolByIdx[tc.Index] = b }
                if tc.ID != "" { b.id = tc.ID }
                if tc.Function.Name != "" { b.name = tc.Function.Name }
                if tc.Function.Arguments != "" { b.args += tc.Function.Arguments }
            }
        }
    }
    if sentTextStart { enc("content_block_stop", map[string]interface{}{"type": "content_block_stop", "index": 0}) }
    if len(toolByIdx) > 0 {
        idxs := make([]int, 0, len(toolByIdx))
        for k := range toolByIdx { idxs = append(idxs, k) }
        sort.Ints(idxs)
        for i, idx := range idxs {
            b := toolByIdx[idx]
            var inputObj interface{} = map[string]interface{}{}
            if strings.TrimSpace(b.args) != "" && json.Valid([]byte(b.args)) {
                var tmp interface{}
                if err := json.Unmarshal([]byte(b.args), &tmp); err == nil { inputObj = tmp }
            }
            enc("content_block_start", map[string]interface{}{"type": "content_block_start", "index": i + 1, "content_block": map[string]interface{}{"type": "tool_use", "id": b.id, "name": b.name, "input": inputObj}})
            enc("content_block_stop", map[string]interface{}{"type": "content_block_stop", "index": i + 1})
        }
    }
    enc("message_delta", map[string]interface{}{"type": "message_delta", "delta": map[string]interface{}{"stop_reason": "end_turn"}, "usage": map[string]int{"input_tokens": 0, "output_tokens": len(totalText) / 4}})
    enc("message_stop", map[string]interface{}{"type": "message_stop"})
    return nil
}

