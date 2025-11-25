package postgres_v2

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// Backward Compatibility: Base Store Interface Implementation
// These methods implement the original Store interface for compatibility
// with existing code that uses v1 methods.
// ==============================================================================

func (s *Store) EnsureRootAdmin(ctx context.Context, email string) (*userstore.User, error) {
	// First try to find existing user
	user, err := s.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	// Create new root admin
	var u userstore.User
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, role, display_name, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, uuid, email, role, display_name, status, created_at, updated_at
	`, email, userstore.RoleRootAdmin, email, userstore.StatusActive).Scan(
		&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create root admin: %w", err)
	}
	return &u, nil
}

func (s *Store) FindByEmail(ctx context.Context, email string) (*userstore.User, error) {
	var u userstore.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, uuid, email, role, display_name, status, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(
		&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find by email: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUser(ctx context.Context, id int64) (*userstore.User, error) {
	var u userstore.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, uuid, email, role, display_name, status, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]userstore.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, uuid, email, role, display_name, status, created_at, updated_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []userstore.User
	for rows.Next() {
		var u userstore.User
		if err := rows.Scan(&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, email string, role userstore.Role, displayName string) (*userstore.User, error) {
	var u userstore.User
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, role, display_name, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, uuid, email, role, display_name, status, created_at, updated_at
	`, email, role, displayName, userstore.StatusActive).Scan(
		&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (s *Store) UpdateUser(ctx context.Context, id int64, displayName string, role userstore.Role) (*userstore.User, error) {
	var u userstore.User
	err := s.db.QueryRowContext(ctx, `
		UPDATE users SET display_name = $2, role = $3, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, uuid, email, role, display_name, status, created_at, updated_at
	`, id, displayName, role).Scan(
		&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return &u, nil
}

func (s *Store) SetUserStatus(ctx context.Context, id int64, status userstore.Status) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET status = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, status)
	return err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

func (s *Store) CreateAPIKey(ctx context.Context, userID int64, scopes []string, expiresAt *time.Time) (*userstore.APIKey, string, error) {
	token, hash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}

	var k userstore.APIKey
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key_hash, key_prefix, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, uuid, user_id, key_prefix, scopes, expires_at, created_at, updated_at
	`, userID, hash, prefix, scopes, expiresAt).Scan(
		&k.ID, &k.UUID, &k.UserID, &k.Prefix, &k.Scopes, &k.ExpiresAt, &k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create api key: %w", err)
	}

	return &k, token, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, userID int64) ([]userstore.APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, uuid, user_id, key_prefix, scopes, expires_at, created_at, updated_at
		FROM api_keys
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []userstore.APIKey
	for rows.Next() {
		var k userstore.APIKey
		if err := rows.Scan(&k.ID, &k.UUID, &k.UserID, &k.Prefix, &k.Scopes, &k.ExpiresAt, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_keys SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

func (s *Store) LookupAPIKey(ctx context.Context, token string) (*userstore.APIKey, *userstore.User, error) {
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	var k userstore.APIKey
	var u userstore.User

	err := s.db.QueryRowContext(ctx, `
		SELECT k.id, k.uuid, k.user_id, k.key_prefix, k.scopes, k.expires_at, k.created_at, k.updated_at,
		       u.id, u.uuid, u.email, u.role, u.display_name, u.status, u.created_at, u.updated_at
		FROM api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.key_hash = $1
			AND k.deleted_at IS NULL
			AND (k.expires_at IS NULL OR k.expires_at > NOW())
			AND u.deleted_at IS NULL
	`, hash).Scan(
		&k.ID, &k.UUID, &k.UserID, &k.Prefix, &k.Scopes, &k.ExpiresAt, &k.CreatedAt, &k.UpdatedAt,
		&u.ID, &u.UUID, &u.Email, &u.Role, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("lookup api key: %w", err)
	}

	return &k, &u, nil
}
