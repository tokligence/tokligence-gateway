package postgres_v2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// User Management (V2)
// ==============================================================================

func (s *Store) CreateUserV2(ctx context.Context, params userstore.CreateUserParams) (*userstore.UserV2, error) {
	authProvider := params.AuthProvider
	if authProvider == "" {
		authProvider = "local"
	}

	var u userstore.UserV2
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, role, display_name, avatar_url, status, auth_provider, external_id, metadata)
		VALUES ($1, $2, $3, $4, 'active', $5, $6, $7)
		RETURNING id, email, role, display_name, avatar_url, status, auth_provider, external_id, last_login_at, metadata, created_at, updated_at
	`, params.Email, params.Role, params.DisplayName, params.AvatarURL, authProvider, params.ExternalID, params.Metadata).Scan(
		&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AvatarURL, &u.Status,
		&u.AuthProvider, &u.ExternalID, &u.LastLoginAt, &u.Metadata, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserV2(ctx context.Context, id uuid.UUID) (*userstore.UserV2, error) {
	var u userstore.UserV2
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, role, display_name, avatar_url, status, auth_provider, external_id, last_login_at, metadata, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AvatarURL, &u.Status,
		&u.AuthProvider, &u.ExternalID, &u.LastLoginAt, &u.Metadata, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*userstore.UserV2, error) {
	var u userstore.UserV2
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, role, display_name, avatar_url, status, auth_provider, external_id, last_login_at, metadata, created_at, updated_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(
		&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AvatarURL, &u.Status,
		&u.AuthProvider, &u.ExternalID, &u.LastLoginAt, &u.Metadata, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (s *Store) ListUsersV2(ctx context.Context, filter userstore.UserFilter) ([]userstore.UserV2, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, "deleted_at IS NULL")

	if filter.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *filter.Role)
		argIdx++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(email ILIKE $%d OR display_name ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT id, email, role, display_name, avatar_url, status, auth_provider, external_id, last_login_at, metadata, created_at, updated_at
		FROM users
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(conditions, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []userstore.UserV2
	for rows.Next() {
		var u userstore.UserV2
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AvatarURL, &u.Status,
			&u.AuthProvider, &u.ExternalID, &u.LastLoginAt, &u.Metadata, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) UpdateUserV2(ctx context.Context, id uuid.UUID, updates userstore.UserUpdate) (*userstore.UserV2, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.DisplayName != nil {
		sets = append(sets, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *updates.DisplayName)
		argIdx++
	}
	if updates.AvatarURL != nil {
		sets = append(sets, fmt.Sprintf("avatar_url = $%d", argIdx))
		args = append(args, *updates.AvatarURL)
		argIdx++
	}
	if updates.Role != nil {
		sets = append(sets, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *updates.Role)
		argIdx++
	}
	if updates.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *updates.Status)
		argIdx++
	}
	if updates.Metadata != nil {
		sets = append(sets, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, *updates.Metadata)
		argIdx++
	}

	if len(sets) == 0 {
		return s.GetUserV2(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE users SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, email, role, display_name, avatar_url, status, auth_provider, external_id, last_login_at, metadata, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	var u userstore.UserV2
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.AvatarURL, &u.Status,
		&u.AuthProvider, &u.ExternalID, &u.LastLoginAt, &u.Metadata, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return &u, nil
}

func (s *Store) DeleteUserV2(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

func (s *Store) EnsureRootAdminV2(ctx context.Context, email string) (*userstore.UserV2, error) {
	// Try to find existing user
	existing, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Update to root_admin if not already
		if existing.Role != userstore.UserRoleV2RootAdmin {
			role := userstore.UserRoleV2RootAdmin
			return s.UpdateUserV2(ctx, existing.ID, userstore.UserUpdate{Role: &role})
		}
		return existing, nil
	}

	// Create new root admin
	return s.CreateUserV2(ctx, userstore.CreateUserParams{
		Email:       email,
		Role:        userstore.UserRoleV2RootAdmin,
		DisplayName: "Root Admin",
	})
}

// UpdateUserLastLogin updates the last login timestamp for a user.
func (s *Store) UpdateUserLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}
