package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDispatcherEmit(t *testing.T) {
	d := &Dispatcher{}
	var sequence []string
	d.Register(func(ctx context.Context, evt Event) error {
		sequence = append(sequence, "first:"+string(evt.Type))
		return nil
	})
	d.Register(func(ctx context.Context, evt Event) error {
		sequence = append(sequence, "second:"+evt.Metadata["label"].(string))
		return errors.New("second handler failed")
	})

	evt := Event{
		ID:         "evt-1",
		Type:       EventUserProvisioned,
		OccurredAt: time.Now(),
		Metadata:   map[string]any{"label": "ok"},
	}

	err := d.Emit(context.Background(), evt)
	if err == nil {
		t.Fatalf("expected aggregated error")
	}
	if !strings.Contains(err.Error(), "second handler failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sequence) != 2 {
		t.Fatalf("expected two handlers to run, got %d", len(sequence))
	}
	if sequence[0] != "first:"+string(EventUserProvisioned) {
		t.Fatalf("unexpected first handler record %q", sequence[0])
	}
	if sequence[1] != "second:ok" {
		t.Fatalf("unexpected second handler record %q", sequence[1])
	}
}

func TestNewScriptHandlerRunsCommand(t *testing.T) {
	// Ensure default marshaler is JSON for the helper process.
	MarshalEvent = JSONMarshaler

	expectID := "evt-script"
	expectType := EventUserUpdated
	handler := NewScriptHandler(ScriptConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcessScriptHandler", "--", expectID, string(expectType)},
		Env: map[string]string{
			"GO_WANT_HELPER_PROCESS": "1",
			"HOOK_EXPECT_ID":         expectID,
			"HOOK_EXPECT_TYPE":       string(expectType),
		},
		Timeout: time.Second,
	})

	evt := Event{
		ID:         expectID,
		Type:       expectType,
		OccurredAt: time.Now(),
		TenantID:   "tenant-123",
		UserID:     "42",
		ActorID:    "42",
		Metadata: map[string]any{
			"role": "consumer",
		},
	}

	if err := handler(context.Background(), evt); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
}

func TestHelperProcessScriptHandler(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	headless := json.NewDecoder(os.Stdin)
	var payload Event
	if err := headless.Decode(&payload); err != nil {
		io.WriteString(os.Stderr, "decode error: "+err.Error())
		os.Exit(2)
	}
	if payload.ID != os.Getenv("HOOK_EXPECT_ID") {
		io.WriteString(os.Stderr, "unexpected id")
		os.Exit(3)
	}
	if string(payload.Type) != os.Getenv("HOOK_EXPECT_TYPE") {
		io.WriteString(os.Stderr, "unexpected type")
		os.Exit(4)
	}
	os.Exit(0)
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{Enabled: true}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected validation error when enabled without script path")
	}

	cfg.ScriptPath = "/tmp/hook.sh"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	h := cfg.BuildScriptHandler()
	if h == nil {
		t.Fatalf("expected handler when config enabled")
	}

	disabled := Config{}
	if handler := disabled.BuildScriptHandler(); handler != nil {
		t.Fatalf("expected nil handler when config disabled")
	}
}
