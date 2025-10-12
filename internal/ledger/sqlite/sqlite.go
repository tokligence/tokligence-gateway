package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// register sqlite driver
	_ "modernc.org/sqlite"

	"github.com/tokligence/tokligence-gateway/internal/ledger"
)

// Store implements ledger.Store backed by SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite store at the given path.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create ledger directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
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
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	service_id INTEGER NOT NULL DEFAULT 0,
	prompt_tokens INTEGER NOT NULL,
	completion_tokens INTEGER NOT NULL,
	direction TEXT NOT NULL CHECK(direction IN ('consume','supply')),
	memo TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_usage_entries_user_created ON usage_entries(user_id, created_at DESC);
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
	_, err := s.db.ExecContext(ctx, `
INSERT INTO usage_entries(user_id, service_id, prompt_tokens, completion_tokens, direction, memo, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
		entry.UserID,
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
WHERE user_id = ?`, userID)

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
SELECT id, user_id, service_id, prompt_tokens, completion_tokens, direction, memo, created_at
FROM usage_entries
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ledger.Entry
	for rows.Next() {
		var e ledger.Entry
		var direction string
		if err := rows.Scan(&e.ID, &e.UserID, &e.ServiceID, &e.PromptTokens, &e.CompletionTokens, &direction, &e.Memo, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Direction = ledger.Direction(direction)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
