package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// Store implements userstore.Store backed by Postgres.
type Store struct {
	db *sql.DB
}

// New opens a Postgres-backed user store using the provided DSN.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres db: %w", err)
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
	id SERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS teams (
	id SERIAL PRIMARY KEY,
	organization_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	role TEXT NOT NULL,
	display_name TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	password_hash TEXT,
	organization_id INTEGER REFERENCES organizations(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS api_keys (
	id SERIAL PRIMARY KEY,
	team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
	key_hash TEXT NOT NULL UNIQUE,
	scopes TEXT,
	expires_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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

// EnsureRootAdmin ensures a root admin exists with the provided email.
func (s *Store) EnsureRootAdmin(ctx context.Context, email string) (*userstore.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		email = "admin@local"
	}

	var existing userstore.User
	row := s.db.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE role = $1 LIMIT 1`, userstore.RoleRootAdmin)
	var createdAt, updatedAt time.Time
	err := row.Scan(&existing.ID, &existing.Email, &existing.Role, &existing.DisplayName, &existing.Status, &createdAt, &updatedAt)
	if err == nil {
		existing.CreatedAt = createdAt
		existing.UpdatedAt = updatedAt
		if !strings.EqualFold(existing.Email, email) {
			if _, err := s.db.ExecContext(ctx, `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2`, email, existing.ID); err != nil {
				return nil, err
			}
			existing.Email = email
		}
		if existing.Status == "" {
			existing.Status = userstore.StatusActive
		}
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	query := `INSERT INTO users(email, role, status) VALUES($1, $2, $3) RETURNING id, created_at, updated_at`
	var id int64
	if err := s.db.QueryRowContext(ctx, query, email, userstore.RoleRootAdmin, userstore.StatusActive).Scan(&id, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	return &userstore.User{
		ID:        id,
		Email:     email,
		Role:      userstore.RoleRootAdmin,
		Status:    userstore.StatusActive,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// FindByEmail returns the matching user or nil if not found.
func (s *Store) FindByEmail(ctx context.Context, email string) (*userstore.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	row := s.db.QueryRowContext(ctx, `SELECT id, email, role, display_name, status, created_at, updated_at FROM users WHERE email = $1 LIMIT 1`, email)
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
