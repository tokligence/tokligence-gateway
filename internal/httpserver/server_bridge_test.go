package httpserver

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "context"
    adapterrouter "github.com/tokligence/tokligence-gateway/internal/adapter/router"
    "github.com/tokligence/tokligence-gateway/internal/adapter/loopback"
    "github.com/tokligence/tokligence-gateway/internal/client"
    "github.com/tokligence/tokligence-gateway/internal/config"
)

// --- helpers ---

type stubGateway struct{}

func (s *stubGateway) Account() (*client.User, *client.ProviderProfile) { return nil, nil }
func (s *stubGateway) EnsureAccount(_ context.Context, _ string, _ []string, _ string) (*client.User, *client.ProviderProfile, error) { return nil, nil, nil }
func (s *stubGateway) ListProviders(_ context.Context) ([]client.ProviderProfile, error) { return nil, nil }
func (s *stubGateway) ListServices(_ context.Context, _ *int64) ([]client.ServiceOffering, error) { return nil, nil }
func (s *stubGateway) ListMyServices(_ context.Context) ([]client.ServiceOffering, error) { return nil, nil }
func (s *stubGateway) UsageSnapshot(_ context.Context) (client.UsageSummary, error) { return client.UsageSummary{}, nil }
func (s *stubGateway) MarketplaceAvailable() bool { return false }
func (s *stubGateway) SetLocalAccount(_ client.User, _ *client.ProviderProfile) {}

// --- tests ---

func TestStripSystemReminder(t *testing.T) {
    cases := []struct{ in, want string }{
        {"hello <system-reminder>hide</system-reminder> world", "hello  world"},
        {"<system-reminder>only</system-reminder>", ""},
        {"keep <SYSTEM-REMINDER>Hide</SYSTEM-REMINDER> text", "keep  text"},
        {"no tags", "no tags"},
        {"broken <system-reminder>noend", "broken "},
        {"multi <system-reminder>a</system-reminder> x <system-reminder>b</system-reminder>", "multi  x "},
    }
    for _, c := range cases {
        got := stripSystemReminder(c.in)
        if got != c.want {
            t.Fatalf("stripSystemReminder(%q) = %q, want %q", c.in, got, c.want)
        }
    }
}

func TestOpenAIBridge_NonStreaming_DefaultAndToolChoice(t *testing.T) {
    // Fake OpenAI server to capture the posted payload
    var captured map[string]any
    openaiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
            t.Fatalf("unexpected path: %s", r.URL.Path)
        }
        body, _ := io.ReadAll(r.Body)
        _ = r.Body.Close()
        _ = json.Unmarshal(body, &captured)
        // Return a minimal tool_calls response
        w.Header().Set("Content-Type", "application/json")
        resp := map[string]any{
            "id": "chatcmpl-test",
            "object": "chat.completion",
            "created": 0,
            "model": "gpt-4o",
            "choices": []any{
                map[string]any{
                    "index": 0,
                    "message": map[string]any{
                        "role": "assistant",
                        "content": "",
                        "tool_calls": []any{
                            map[string]any{
                                "id": "call_0",
                                "type": "function",
                                "function": map[string]any{
                                    "name": "Bash",
                                    "arguments": "{\"cmd\":\"echo hi\"}",
                                },
                            },
                        },
                    },
                    "finish_reason": "tool_calls",
                },
            },
            "usage": map[string]any{
                "prompt_tokens": 1,
                "completion_tokens": 1,
            },
        }
        jb, _ := json.Marshal(resp)
        _, _ = w.Write(jb)
    }))
    defer openaiSrv.Close()

    // Router: register loopback under both names to satisfy route registration
    r := adapterrouter.New()
    lb := loopback.New()
    _ = r.RegisterAdapter("loopback", lb)
    _ = r.RegisterAdapter("openai", lb)
    if err := r.RegisterRoute("claude-*", "openai"); err != nil {
        t.Fatalf("register route: %v", err)
    }
    r.SetFallback(lb)

    // Server
    s := New(&stubGateway{}, r, nil, nil, nil, nil, nil, true)
    // Set upstreams with streaming disabled by default
    s.SetUpstreams("sk-test", openaiSrv.URL, "", "", "", false, false)

    // Build Anthropic-native request (tools declared; stream=true on input)
    areq := map[string]any{
        "model": "claude-xyz",
        "stream": true,
        "system": []map[string]any{{"type": "text", "text": "abc<system-reminder>hide</system-reminder>def"}},
        "messages": []map[string]any{{
            "role": "user",
            "content": []map[string]any{{"type":"text", "text": "hello"}},
        }},
        "tools": []map[string]any{{
            "name": "Bash",
            "input_schema": map[string]any{
                "type": "object",
                "properties": map[string]any{"cmd": map[string]any{"type":"string"}},
            },
        }},
    }
    b, _ := json.Marshal(areq)

    req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    rr := httptest.NewRecorder()
    s.handleAnthropicMessages(rr, req)

    if rr.Code != http.StatusOK {
        t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
    }
    // Assert OpenAI payload was captured
    if captured == nil {
        t.Fatalf("no payload captured")
    }
    // tool_choice must be required
    if tc, ok := captured["tool_choice"].(string); !ok || tc != "required" {
        t.Fatalf("tool_choice=%v, want 'required'", captured["tool_choice"])
    }
    // Non-streaming by default: no stream=true in payload
    if v, ok := captured["stream"]; ok {
        if vb, okb := v.(bool); okb && vb {
            t.Fatalf("unexpected stream=true in OpenAI bridge payload")
        }
    }
    // System text should be present and stripped of <system-reminder>
    msgs, ok := captured["messages"].([]any)
    if !ok || len(msgs) == 0 {
        t.Fatalf("missing messages in payload")
    }
    first, _ := msgs[0].(map[string]any)
    if first["role"] != "system" {
        t.Fatalf("first message role=%v, want system", first["role"])
    }
    if text, _ := first["content"].(string); strings.Contains(strings.ToLower(text), "system-reminder") {
        t.Fatalf("system-reminder leaked into system content: %q", text)
    }

    // Response should include tool_use block
    if !strings.Contains(rr.Body.String(), "\"type\":\"tool_use\"") {
        t.Fatalf("response missing tool_use block: %s", rr.Body.String())
    }
}

func TestConfig_OpenAIStreamToggle_DefaultFalseAndEnv(t *testing.T) {
    dir := t.TempDir()
    // Create env-specific ini
    envDir := filepath.Join(dir, "config", "dev")
    if err := os.MkdirAll(envDir, 0o755); err != nil {
        t.Fatalf("mkdir: %v", err)
    }
    ini := []byte("openai_tool_bridge_stream = true\n")
    if err := os.WriteFile(filepath.Join(envDir, "gateway.ini"), ini, 0o644); err != nil {
        t.Fatalf("write ini: %v", err)
    }
    cfg, err := config.LoadGatewayConfig(dir)
    if err != nil { t.Fatalf("load cfg: %v", err) }
    if !cfg.OpenAIToolBridgeStreamEnabled {
        t.Fatalf("expected ini to enable OpenAI tool-bridge stream")
    }
    // Env override to false
    t.Setenv("TOKLIGENCE_OPENAI_TOOL_BRIDGE_STREAM", "false")
    cfg2, err := config.LoadGatewayConfig(dir)
    if err != nil { t.Fatalf("load cfg2: %v", err) }
    if cfg2.OpenAIToolBridgeStreamEnabled {
        t.Fatalf("env override failed: expected false")
    }
}
