package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tokligence/tokligence-gateway/internal/ledger"
)

// Store implements ledger.Store backed by PostgreSQL.
type Store struct {
	db *sql.DB
}

// New opens a PostgreSQL-backed ledger store using the provided DSN and connection pool settings.
func New(dsn string, maxOpen, maxIdle, lifetimeMinutes, idleTimeMinutes int) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres db: %w", err)
	}

	// Configure connection pool for high concurrency
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	}
	if lifetimeMinutes > 0 {
		db.SetConnMaxLifetime(time.Duration(lifetimeMinutes) * time.Minute)
	}
	if idleTimeMinutes > 0 {
		db.SetConnMaxIdleTime(time.Duration(idleTimeMinutes) * time.Minute)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initSchema() error {
	const schema = `
CREATE TABLE IF NOT EXISTS usage_entries (
	id SERIAL PRIMARY KEY,
	uuid UUID NOT NULL DEFAULT gen_random_uuid(),
	user_id BIGINT NOT NULL,
	api_key_id BIGINT,
	service_id BIGINT NOT NULL DEFAULT 0,
	prompt_tokens BIGINT NOT NULL,
	completion_tokens BIGINT NOT NULL,
	direction TEXT NOT NULL CHECK(direction IN ('consume','supply')),
	memo TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_usage_entries_user_created ON usage_entries(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_entries_api_key_created ON usage_entries(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_entries_deleted_at ON usage_entries(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_entries_uuid ON usage_entries(uuid);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// Close releases underlying database resources.
func (s *Store) Close() error {
	return s.db.Close()
}

// Record inserts a new usage entry.
func (s *Store) Record(ctx context.Context, entry ledger.Entry) error {
	if entry.UserID == 0 {
		return errors.New("ledger record requires user id")
	}
	if entry.Direction != ledger.DirectionConsume && entry.Direction != ledger.DirectionSupply {
		return fmt.Errorf("invalid direction %q", entry.Direction)
	}
	created := entry.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	var apiKey interface{}
	if entry.APIKeyID != nil {
		apiKey = *entry.APIKeyID
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO usage_entries(user_id, api_key_id, service_id, prompt_tokens, completion_tokens, direction, memo, created_at)
VALUES($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.UserID,
		apiKey,
		entry.ServiceID,
		entry.PromptTokens,
		entry.CompletionTokens,
		string(entry.Direction),
		entry.Memo,
		created,
	)
	return err
}

// Summary returns aggregated usage for the given user.
func (s *Store) Summary(ctx context.Context, userID int64) (ledger.Summary, error) {
	if userID == 0 {
		return ledger.Summary{}, errors.New("user id required")
	}
	row := s.db.QueryRowContext(ctx, `
SELECT
	COALESCE(SUM(CASE WHEN direction='consume' THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS consumed,
	COALESCE(SUM(CASE WHEN direction='supply' THEN prompt_tokens + completion_tokens ELSE 0 END), 0) AS supplied
FROM usage_entries
WHERE user_id = $1 AND deleted_at IS NULL`, userID)

	var consumed, supplied sql.NullInt64
	if err := row.Scan(&consumed, &supplied); err != nil {
		return ledger.Summary{}, err
	}
	summary := ledger.Summary{
		ConsumedTokens: consumed.Int64,
		SuppliedTokens: supplied.Int64,
	}
	summary.NetTokens = summary.SuppliedTokens - summary.ConsumedTokens
	return summary, nil
}

// ListRecent returns the latest entries for a user.
func (s *Store) ListRecent(ctx context.Context, userID int64, limit int) ([]ledger.Entry, error) {
	if userID == 0 {
		return nil, errors.New("user id required")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_id, api_key_id, service_id, prompt_tokens, completion_tokens, direction, memo, created_at
FROM usage_entries
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ledger.Entry
	for rows.Next() {
		var e ledger.Entry
		var direction string
		var apiKey sql.NullInt64
		if err := rows.Scan(&e.ID, &e.UserID, &apiKey, &e.ServiceID, &e.PromptTokens, &e.CompletionTokens, &direction, &e.Memo, &e.CreatedAt); err != nil {
			return nil, err
		}
		if apiKey.Valid {
			id := apiKey.Int64
			e.APIKeyID = &id
		}
		e.Direction = ledger.Direction(direction)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
