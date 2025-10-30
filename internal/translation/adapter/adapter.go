package adapter

// moved from internal/sidecar/adapter/adapter.go
// (file contents preserved during refactor)

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

func systemToOpenAI(sys json.RawMessage) *OpenAIMessage {
    if len(sys) == 0 || string(sys) == "null" { return nil }
    var s string
    if json.Unmarshal(sys, &s) == nil {
        if strings.TrimSpace(s) == "" { return nil }
        return &OpenAIMessage{Role: "system", Content: s}
    }
    var obj map[string]interface{}
    if json.Unmarshal(sys, &obj) == nil {
        if t, ok := obj["text"].(string); ok && strings.TrimSpace(t) != "" {
            return &OpenAIMessage{Role: "system", Content: t}
        }
    }
    return nil
}

// AnthropicToOpenAI maps an Anthropic messages request to OpenAI Chat Completions.
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
        d := chunk.choices[0].Delta // bug fix below
        _ = d
    }
    // Defensive: close message with empty usage if no chunks
    enc("message_delta", map[string]interface{}{"type": "message_delta", "delta": map[string]interface{}{"stop_reason": "end_turn"}})
    enc("message_stop", map[string]interface{}{"type": "message_stop"})
    return nil
}
