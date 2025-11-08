package responses

import (
	"context"

	"github.com/tokligence/tokligence-gateway/internal/adapter"
)

// StreamInit describes the resources required to forward SSE chunks to the client.
type StreamInit struct {
	Channel <-chan adapter.StreamEvent
	Cleanup func()
}

// StreamProvider translates the canonical conversation into provider-specific streams.
type StreamProvider interface {
	Stream(ctx context.Context, conv Conversation) (StreamInit, error)
}
