package ledger

import (
	"context"
	"time"
)

// Direction indicates whether tokens were consumed or supplied via the gateway.
type Direction string

const (
	DirectionConsume Direction = "consume"
	DirectionSupply  Direction = "supply"
)

// Entry represents a single usage record written to the local ledger.
type Entry struct {
	ID               int64     `json:"id"`
	UserID           int64     `json:"user_id"`
	APIKeyID         *int64    `json:"api_key_id,omitempty"`
	ServiceID        int64     `json:"service_id"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	Direction        Direction `json:"direction"`
	Memo             string    `json:"memo"`
	CreatedAt        time.Time `json:"created_at"`
}

// Summary aggregates token usage for a user.
type Summary struct {
	ConsumedTokens int64 `json:"consumed_tokens"`
	SuppliedTokens int64 `json:"supplied_tokens"`
	NetTokens      int64 `json:"net_tokens"`
}

// Store defines persistence behaviour for the ledger.
type Store interface {
	Record(ctx context.Context, entry Entry) error
	Summary(ctx context.Context, userID int64) (Summary, error)
	ListRecent(ctx context.Context, userID int64, limit int) ([]Entry, error)
	Close() error
}
