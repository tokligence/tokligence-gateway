package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// Store implements userstore.Store backed by SQLite.
type Store struct {
	db *sql.DB
}

// New opens (or creates) a SQLite user store at the supplied path.
func New(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create identity directory: %w", err)
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
CREATE TABLE IF NOT EXISTS organizations (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS teams (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL UNIQUE,
	role TEXT NOT NULL,
	display_name TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	password_hash TEXT,
	organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	key_hash TEXT NOT NULL UNIQUE,
	scopes TEXT,
	expires_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_api_keys_team ON api_keys(team_id);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

// Close releases underlying resources.
func (s *Store) Close() error {
	return s.db.Close()
}

// EnsureRootAdmin guarantees a root admin account exists with the provided email.
func (s *Store) EnsureRootAdmin(ctx context.Context, email string) (*userstore.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		email = "admin@local"
	}

	row := tx.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE role = ? LIMIT 1`, userstore.RoleRootAdmin)
	var existing userstore.User
	var createdAt, updatedAt time.Time
	scanErr := row.Scan(&existing.ID, &existing.Email, &existing.Role, &existing.DisplayName, &existing.Status, &createdAt, &updatedAt)
	if scanErr == nil {
		existing.CreatedAt = createdAt
		existing.UpdatedAt = updatedAt
		if !strings.EqualFold(existing.Email, email) {
			if _, err = tx.ExecContext(ctx, `UPDATE users SET email = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, email, existing.ID); err != nil {
				return nil, err
			}
			existing.Email = email
		}
		existing.Role = userstore.RoleRootAdmin
		if existing.Status == "" {
			existing.Status = userstore.StatusActive
		}
		return &existing, nil
	}
	if scanErr != sql.ErrNoRows {
		return nil, scanErr
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO users(email, role, status) VALUES(?, ?, ?)`, email, userstore.RoleRootAdmin, userstore.StatusActive)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	created := time.Now().UTC()
	return &userstore.User{
		ID:        id,
		Email:     email,
		Role:      userstore.RoleRootAdmin,
		Status:    userstore.StatusActive,
		CreatedAt: created,
		UpdatedAt: created,
	}, nil
}

// FindByEmail returns the user matching the email, if present.
func (s *Store) FindByEmail(ctx context.Context, email string) (*userstore.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	row := s.db.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE email = ? LIMIT 1`, email)
	var u userstore.User
	var createdAt, updatedAt time.Time
	if err := row.Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt
	return &u, nil
}
