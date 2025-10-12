package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// EventType names the lifecycle transitions Tokligence Gateway exports.
// Downstream systems (RAG stacks, provisioning scripts, audit sinks) can
// subscribe to these events to mirror identity state.
type EventType string

const (
	// EventUserProvisioned is emitted after a new user account is created.
	EventUserProvisioned EventType = "gateway.user.provisioned"
	// EventUserUpdated is emitted when profile metadata changes.
	EventUserUpdated EventType = "gateway.user.updated"
	// EventUserDeleted is emitted when a user is removed or deactivated.
	EventUserDeleted EventType = "gateway.user.deleted"
	// EventAPIKeyIssued is emitted when a new API key is minted.
	EventAPIKeyIssued EventType = "gateway.api_key.issued"
	// EventAPIKeyRevoked is emitted when an API key is revoked or rotated.
	EventAPIKeyRevoked EventType = "gateway.api_key.revoked"
)

// Event envelopes the concrete payload we broadcast to hook listeners.
type Event struct {
	ID         string         // globally unique event identifier
	Type       EventType      // lifecycle transition identifier
	OccurredAt time.Time      // timestamp of emission
	TenantID   string         // optional organisation/tenant scope
	UserID     string         // user associated with the event
	ActorID    string         // initiator (user/service/internal job)
	Metadata   map[string]any // extensible JSON-friendly payload
}

// Handler reacts to an Event. Implementations should be idempotent.
type Handler func(context.Context, Event) error

// Dispatcher coordinates handler registration and event fan-out.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers []Handler
}

// Register adds a new handler. Handlers fire sequentially in registration
// order so operators can reason about side effects.
func (d *Dispatcher) Register(h Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, h)
}

// Emit delivers an event to all registered handlers. Errors are aggregated so
// callers can surface each failure in logs or telemetry.
func (d *Dispatcher) Emit(ctx context.Context, event Event) error {
	d.mu.RLock()
	handlers := append([]Handler(nil), d.handlers...)
	d.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// ScriptConfig describes how to invoke an external command when events fire.
// This lets operators bridge gateway identities into vector databases like
// Weaviate, Dgraph, or Milvus without waiting for native integrations.
type ScriptConfig struct {
	Command string            // required executable (absolute or PATH lookup)
	Args    []string          // static arguments passed to the executable
	Env     map[string]string // optional environment overrides
	Timeout time.Duration     // optional max execution time
}

// MarshalEvent converts an Event into the wire format presented to scripts.
// Packages embedding the dispatcher can override this variable to swap JSON
// for other encodings or to inject additional metadata.
var MarshalEvent = JSONMarshaler

// NewScriptHandler returns a Handler that pipes the marshalled event to a
// configured executable via STDIN. It is a bridge for the CLI/config layer.
func NewScriptHandler(cfg ScriptConfig) Handler {
	return func(parentCtx context.Context, evt Event) error {
		if cfg.Command == "" {
			return fmt.Errorf("hooks: command not configured")
		}

		payload, err := MarshalEvent(evt)
		if err != nil {
			return fmt.Errorf("hooks: marshal event: %w", err)
		}

		ctx := parentCtx
		if cfg.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(parentCtx, cfg.Timeout)
			defer cancel()
		}

		cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
		if len(cfg.Env) > 0 {
			env := cmd.Environ()
			for key, val := range cfg.Env {
				env = append(env, fmt.Sprintf("%s=%s", key, val))
			}
			cmd.Env = env
		}

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("hooks: stdin pipe: %w", err)
		}

		go func() {
			defer stdin.Close()
			_, _ = stdin.Write(payload)
		}()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hooks: command failed: %w", err)
		}

		return nil
	}
}

// JSONMarshaler serialises the event into a stable JSON envelope. It is
// provided as a reference implementation for callers that want a plug-and-play
// payload without writing their own MarshalEvent override.
func JSONMarshaler(evt Event) ([]byte, error) {
	envelope := struct {
		ID         string         `json:"id"`
		Type       EventType      `json:"type"`
		OccurredAt time.Time      `json:"occurred_at"`
		TenantID   string         `json:"tenant_id"`
		UserID     string         `json:"user_id"`
		ActorID    string         `json:"actor_id"`
		Metadata   map[string]any `json:"metadata"`
	}{
		ID:         evt.ID,
		Type:       evt.Type,
		OccurredAt: evt.OccurredAt,
		TenantID:   evt.TenantID,
		UserID:     evt.UserID,
		ActorID:    evt.ActorID,
		Metadata:   evt.Metadata,
	}
	return json.Marshal(envelope)
}
