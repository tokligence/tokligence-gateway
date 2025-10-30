package adapterhttp

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "strings"
    "time"

    adapter "github.com/tokligence/tokligence-gateway/internal/sidecar/adapter"
)

var (
    debugEnabled = false
    logEvents    = false
)

func SetDebug(v bool)    { debugEnabled = v }
func SetLogEvents(v bool){ logEvents = v }

type Config struct {
    AnthropicBaseURL   string
    AnthropicAPIKey    string
    AnthropicVersion   string
    OpenAIBaseURL      string
    OpenAIAPIKey       string
    ModelMap           string // line-delimited: "claude-x=gpt-y"
    DefaultOpenAIModel string // fallback
    // MaxTokensCap caps completion tokens sent to OpenAI (0 = disable clamp, use upstream default)
    MaxTokensCap       int
}

func trimRightSlash(s string) string { return strings.TrimRight(s, "/") }

func mapModelFromConfig(anthropicModel string, cfg Config) string {
    if cfg.ModelMap != "" {
        for _, line := range strings.Split(cfg.ModelMap, "\n") {
            line = strings.TrimSpace(line)
            if line == "" || strings.HasPrefix(line, "#") { continue }
            kv := strings.SplitN(line, "=", 2)
            if len(kv) == 2 && strings.TrimSpace(kv[0]) == anthropicModel { return strings.TrimSpace(kv[1]) }
        }
    }
    if cfg.DefaultOpenAIModel != "" { return cfg.DefaultOpenAIModel }
    return anthropicModel
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(v)
}

// clampMaxTokens caps max_tokens to a safe default for OpenAI models that typically
// accept up to 16384 completion tokens. If v <= 0, it returns v unchanged (let upstream decide).
func clampMaxTokens(v int, capTokens int) int {
    if capTokens <= 0 {
        return v
    }
    if v <= 0 {
        return v
    }
    if v > capTokens {
        return capTokens
    }
    return v
}

// Messages handler (Anthropic-compatible) that proxies to OpenAI
func NewMessagesHandler(cfg Config, client *http.Client) http.Handler {
    if client == nil { client = http.DefaultClient }
    base := trimRightSlash(cfg.OpenAIBaseURL)
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
        var areq adapter.AnthropicMessageRequest
        if err := json.NewDecoder(r.Body).Decode(&areq); err != nil { http.Error(w, "invalid json", http.StatusBadRequest); return }
        oreq, err := adapter.AnthropicToOpenAI(areq)
        if err != nil { http.Error(w, "invalid messages: "+err.Error(), http.StatusBadRequest); return }
        // Apply model mapping via config
        oreq.Model = mapModelFromConfig(areq.Model, cfg)
        if areq.Stream {
            proxyStream(w, r.Context(), client, base, cfg.OpenAIAPIKey, oreq, areq, cfg.MaxTokensCap)
            return
        }
        proxyOnce(w, r.Context(), client, base, cfg.OpenAIAPIKey, oreq, areq, cfg.MaxTokensCap)
    })
}

// proxyOnce forwards a single OpenAI request and maps the non-streaming response to Anthropic
func proxyOnce(w http.ResponseWriter, ctx context.Context, client *http.Client, base, apiKey string, oreq adapter.OpenAIChatRequest, areq adapter.AnthropicMessageRequest, maxCap int) {
    // Clamp completion tokens to avoid OpenAI 400 invalid_value errors from large Anthropic max_tokens
    if areq.MaxTokens > 0 {
        oreq.MaxTokens = clampMaxTokens(areq.MaxTokens, maxCap)
    } else if oreq.MaxTokens > 0 {
        oreq.MaxTokens = clampMaxTokens(oreq.MaxTokens, maxCap)
    }
    reqBody, _ := json.Marshal(oreq)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")
    if apiKey != "" { req.Header.Set("Authorization", "Bearer "+apiKey) }
    resp, err := client.Do(req)
    if err != nil { http.Error(w, "openai request failed: "+err.Error(), http.StatusBadGateway); return }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
        http.Error(w, fmt.Sprintf("openai error %d: %s", resp.StatusCode, string(b)), http.StatusBadGateway)
        return
    }
    var oresp struct{ ID, Object, Model string; Choices []struct{ Index int; FinishReason string; Message adapter.OpenAIMessage } `json:"choices"`; Usage *struct{ PromptTokens, CompletionTokens, TotalTokens int } `json:"usage,omitempty"` }
    if err := json.NewDecoder(resp.Body).Decode(&oresp); err != nil { http.Error(w, "invalid openai response", http.StatusBadGateway); return }
    // Convert to Anthropic-style JSON one-shot
    content := []map[string]interface{}{}
    if s, ok := oresp.Choices[0].Message.Content.(string); ok && strings.TrimSpace(s) != "" {
        content = append(content, map[string]interface{}{"type":"text","text": s})
    }
    for _, tc := range oresp.Choices[0].Message.ToolCalls {
        var argsObj interface{}
        if json.Valid([]byte(tc.Function.Arguments)) { _ = json.Unmarshal([]byte(tc.Function.Arguments), &argsObj) } else { argsObj = map[string]interface{}{"_": tc.Function.Arguments} }
        content = append(content, map[string]interface{}{"type":"tool_use","id": tc.ID, "name": tc.Function.Name, "input": argsObj})
    }
    var usage *map[string]int
    if oresp.Usage != nil { u := map[string]int{"input_tokens": oresp.Usage.PromptTokens, "output_tokens": oresp.Usage.CompletionTokens}; usage = &u }
    aret := map[string]interface{}{
        "id": fmt.Sprintf("msg_%d", time.Now().UnixNano()),
        "type": "message",
        "role": "assistant",
        "model": areq.Model,
        "content": content,
        "stop_reason": "end_turn",
    }
    if usage != nil { aret["usage"] = usage }
    writeJSON(w, http.StatusOK, aret)
}

// proxyStream forwards OpenAI stream and maps to Anthropic SSE
func proxyStream(w http.ResponseWriter, ctx context.Context, client *http.Client, base, apiKey string, oreq adapter.OpenAIChatRequest, areq adapter.AnthropicMessageRequest, maxCap int) {
    oreq.Stream = true
    // Clamp completion tokens for streaming as well
    if areq.MaxTokens > 0 {
        oreq.MaxTokens = clampMaxTokens(areq.MaxTokens, maxCap)
    } else if oreq.MaxTokens > 0 {
        oreq.MaxTokens = clampMaxTokens(oreq.MaxTokens, maxCap)
    }
    reqBody, _ := json.Marshal(oreq)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(reqBody))
    req.Header.Set("Content-Type", "application/json")
    if apiKey != "" { req.Header.Set("Authorization", "Bearer "+apiKey) }
    resp, err := client.Do(req)
    if err != nil { http.Error(w, "openai stream failed: "+err.Error(), http.StatusBadGateway); return }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
        http.Error(w, fmt.Sprintf("openai error %d: %s", resp.StatusCode, string(b)), http.StatusBadGateway)
        return
    }
    w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher, ok := w.(http.Flusher)
    if !ok { http.Error(w, "streaming unsupported", http.StatusInternalServerError); return }
    _ = adapter.ConvertOpenAIStreamToAnthropic(ctx, areq.Model, resp.Body, func(event string, payload interface{}) {
        if logEvents && debugEnabled {
            if payload != nil { pb, _ := json.Marshal(payload); fmt.Printf("[adapter/sse->anthropic] event=%s payload=%s\n", event, string(preview(pb, 256))) } else { fmt.Printf("[adapter/sse->anthropic] event=%s\n", event) }
        }
        fmt.Fprintf(w, "event: %s\n", event)
        if payload != nil { b, _ := json.Marshal(payload); fmt.Fprintf(w, "data: %s\n\n", string(b)) } else { fmt.Fprintf(w, "data: {}\n\n") }
        flusher.Flush()
    })
}

func preview(b []byte, max int) []byte {
    if len(b) <= max { return b }
    if max < 3 { return b[:max] }
    out := make([]byte, max)
    copy(out, b[:max-3])
    copy(out[max-3:], []byte("..."))
    return out
}

// Optional request logging (not used by gateway)
func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        sw := &statusWriter{ResponseWriter: w, status: 200}
        next.ServeHTTP(sw, r)
        dur := time.Since(start)
        fmt.Printf("%s %s %s %d %dB %s\n", r.RemoteAddr, r.Method, r.URL.Path, sw.status, sw.written, strconv.FormatInt(dur.Milliseconds(), 10)+"ms")
    })
}

type statusWriter struct { http.ResponseWriter; status int; written int }
func (s *statusWriter) WriteHeader(code int) { s.status = code; s.ResponseWriter.WriteHeader(code) }
func (s *statusWriter) Write(b []byte) (int, error) { n, err := s.ResponseWriter.Write(b); s.written += n; return n, err }
