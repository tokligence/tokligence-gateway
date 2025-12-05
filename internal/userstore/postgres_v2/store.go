// Package postgres_v2 provides a PostgreSQL implementation of the StoreV2 interface.
package postgres_v2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// Store implements userstore.StoreV2 for PostgreSQL.
type Store struct {
	db *sql.DB
}

// Config holds connection pool settings.
type Config struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultConfig returns sensible defaults for connection pooling.
func DefaultConfig() Config {
	return Config{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// New creates a new PostgreSQL store with the given DSN.
func New(dsn string, cfg Config) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return s, nil
}

// initSchema creates all v2 tables and runs migrations.
func (s *Store) initSchema() error {
	// Run v1 to v2 migration first
	if _, err := s.db.Exec(migrationV2); err != nil {
		return fmt.Errorf("migration v2 failed: %w", err)
	}

	// Create v2 tables
	if _, err := s.db.Exec(schemaV2); err != nil {
		return fmt.Errorf("schema v2 creation failed: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// ==============================================================================
// Gateway Management
// ==============================================================================

func (s *Store) CreateGateway(ctx context.Context, ownerUserID uuid.UUID, alias string) (*userstore.Gateway, error) {
	var gw userstore.Gateway
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO gateways (owner_user_id, alias)
		VALUES ($1, $2)
		RETURNING id, alias, owner_user_id, provider_enabled, consumer_enabled, metadata, created_at, updated_at
	`, ownerUserID, alias).Scan(
		&gw.ID, &gw.Alias, &gw.OwnerUserID, &gw.ProviderEnabled, &gw.ConsumerEnabled,
		&gw.Metadata, &gw.CreatedAt, &gw.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create gateway: %w", err)
	}

	// Create owner membership
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO gateway_memberships (user_id, gateway_id, role)
		VALUES ($1, $2, 'owner')
	`, ownerUserID, gw.ID)
	if err != nil {
		return nil, fmt.Errorf("create owner membership: %w", err)
	}

	return &gw, nil
}

func (s *Store) GetGateway(ctx context.Context, id uuid.UUID) (*userstore.Gateway, error) {
	var gw userstore.Gateway
	err := s.db.QueryRowContext(ctx, `
		SELECT id, alias, owner_user_id, provider_enabled, consumer_enabled, metadata, created_at, updated_at, deleted_at
		FROM gateways
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&gw.ID, &gw.Alias, &gw.OwnerUserID, &gw.ProviderEnabled, &gw.ConsumerEnabled,
		&gw.Metadata, &gw.CreatedAt, &gw.UpdatedAt, &gw.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get gateway: %w", err)
	}
	return &gw, nil
}

func (s *Store) ListGatewaysForUser(ctx context.Context, userID uuid.UUID) ([]userstore.Gateway, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.alias, g.owner_user_id, g.provider_enabled, g.consumer_enabled, g.metadata, g.created_at, g.updated_at
		FROM gateways g
		JOIN gateway_memberships gm ON g.id = gm.gateway_id
		WHERE gm.user_id = $1 AND g.deleted_at IS NULL AND gm.deleted_at IS NULL
		ORDER BY g.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list gateways: %w", err)
	}
	defer rows.Close()

	var gateways []userstore.Gateway
	for rows.Next() {
		var gw userstore.Gateway
		if err := rows.Scan(
			&gw.ID, &gw.Alias, &gw.OwnerUserID, &gw.ProviderEnabled, &gw.ConsumerEnabled,
			&gw.Metadata, &gw.CreatedAt, &gw.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan gateway: %w", err)
		}
		gateways = append(gateways, gw)
	}
	return gateways, rows.Err()
}

func (s *Store) UpdateGateway(ctx context.Context, id uuid.UUID, updates userstore.GatewayUpdate) (*userstore.Gateway, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.Alias != nil {
		sets = append(sets, fmt.Sprintf("alias = $%d", argIdx))
		args = append(args, *updates.Alias)
		argIdx++
	}
	if updates.ProviderEnabled != nil {
		sets = append(sets, fmt.Sprintf("provider_enabled = $%d", argIdx))
		args = append(args, *updates.ProviderEnabled)
		argIdx++
	}
	if updates.ConsumerEnabled != nil {
		sets = append(sets, fmt.Sprintf("consumer_enabled = $%d", argIdx))
		args = append(args, *updates.ConsumerEnabled)
		argIdx++
	}
	if updates.Metadata != nil {
		sets = append(sets, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, *updates.Metadata)
		argIdx++
	}

	if len(sets) == 0 {
		return s.GetGateway(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE gateways SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, alias, owner_user_id, provider_enabled, consumer_enabled, metadata, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	var gw userstore.Gateway
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&gw.ID, &gw.Alias, &gw.OwnerUserID, &gw.ProviderEnabled, &gw.ConsumerEnabled,
		&gw.Metadata, &gw.CreatedAt, &gw.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update gateway: %w", err)
	}
	return &gw, nil
}

func (s *Store) DeleteGateway(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE gateways SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

// ==============================================================================
// Gateway Membership
// ==============================================================================

func (s *Store) AddGatewayMember(ctx context.Context, gatewayID, userID uuid.UUID, role userstore.GatewayMemberRole) (*userstore.GatewayMembership, error) {
	var m userstore.GatewayMembership
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO gateway_memberships (gateway_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, gateway_id) DO UPDATE SET role = $3, deleted_at = NULL, updated_at = NOW()
		RETURNING id, user_id, gateway_id, role, created_at, updated_at
	`, gatewayID, userID, role).Scan(
		&m.ID, &m.UserID, &m.GatewayID, &m.Role, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add gateway member: %w", err)
	}
	return &m, nil
}

func (s *Store) ListGatewayMembers(ctx context.Context, gatewayID uuid.UUID) ([]userstore.GatewayMembershipWithUser, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT gm.id, gm.user_id, gm.gateway_id, gm.role, gm.created_at, gm.updated_at,
		       u.id, u.email, u.role, u.display_name, u.avatar_url, u.status,
		       u.auth_provider, u.external_id, u.last_login_at, u.metadata, u.created_at, u.updated_at
		FROM gateway_memberships gm
		JOIN users u ON gm.user_id = u.id
		WHERE gm.gateway_id = $1 AND gm.deleted_at IS NULL AND u.deleted_at IS NULL
		ORDER BY gm.created_at
	`, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list gateway members: %w", err)
	}
	defer rows.Close()

	var members []userstore.GatewayMembershipWithUser
	for rows.Next() {
		var m userstore.GatewayMembershipWithUser
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.GatewayID, &m.Role, &m.CreatedAt, &m.UpdatedAt,
			&m.User.ID, &m.User.Email, &m.User.Role, &m.User.DisplayName, &m.User.AvatarURL, &m.User.Status,
			&m.User.AuthProvider, &m.User.ExternalID, &m.User.LastLoginAt, &m.User.Metadata, &m.User.CreatedAt, &m.User.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *Store) UpdateGatewayMember(ctx context.Context, membershipID uuid.UUID, role userstore.GatewayMemberRole) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE gateway_memberships SET role = $2 WHERE id = $1 AND deleted_at IS NULL
	`, membershipID, role)
	return err
}

func (s *Store) RemoveGatewayMember(ctx context.Context, membershipID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE gateway_memberships SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, membershipID)
	return err
}

func (s *Store) GetGatewayMembership(ctx context.Context, gatewayID, userID uuid.UUID) (*userstore.GatewayMembership, error) {
	var m userstore.GatewayMembership
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, gateway_id, role, created_at, updated_at
		FROM gateway_memberships
		WHERE gateway_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, gatewayID, userID).Scan(&m.ID, &m.UserID, &m.GatewayID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get gateway membership: %w", err)
	}
	return &m, nil
}

// ==============================================================================
// OrgUnit Management
// ==============================================================================

func (s *Store) CreateOrgUnit(ctx context.Context, params userstore.CreateOrgUnitParams) (*userstore.OrgUnit, error) {
	var path string
	var depth int

	if params.ParentID != nil {
		var parentPath string
		err := s.db.QueryRowContext(ctx, `
			SELECT path, depth FROM org_units WHERE id = $1 AND deleted_at IS NULL
		`, *params.ParentID).Scan(&parentPath, &depth)
		if err != nil {
			return nil, fmt.Errorf("get parent org unit: %w", err)
		}
		path = parentPath + "/" + params.Slug
		depth = depth + 1
	} else {
		path = "/" + params.Slug
		depth = 0
	}

	var ou userstore.OrgUnit
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO org_units (gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata, created_at, updated_at
	`, params.GatewayID, params.ParentID, path, depth, params.Name, params.Slug, params.UnitType,
		params.BudgetID, pq.Array(params.AllowedModels), params.Metadata).Scan(
		&ou.ID, &ou.GatewayID, &ou.ParentID, &ou.Path, &ou.Depth, &ou.Name, &ou.Slug, &ou.UnitType,
		&ou.BudgetID, pq.Array(&ou.AllowedModels), &ou.Metadata, &ou.CreatedAt, &ou.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create org unit: %w", err)
	}
	return &ou, nil
}

func (s *Store) GetOrgUnit(ctx context.Context, id uuid.UUID) (*userstore.OrgUnit, error) {
	var ou userstore.OrgUnit
	err := s.db.QueryRowContext(ctx, `
		SELECT id, gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata, created_at, updated_at, deleted_at
		FROM org_units
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&ou.ID, &ou.GatewayID, &ou.ParentID, &ou.Path, &ou.Depth, &ou.Name, &ou.Slug, &ou.UnitType,
		&ou.BudgetID, pq.Array(&ou.AllowedModels), &ou.Metadata, &ou.CreatedAt, &ou.UpdatedAt, &ou.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get org unit: %w", err)
	}
	return &ou, nil
}

func (s *Store) GetOrgUnitTree(ctx context.Context, gatewayID uuid.UUID) ([]userstore.OrgUnitWithChildren, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata, created_at, updated_at
		FROM org_units
		WHERE gateway_id = $1 AND deleted_at IS NULL
		ORDER BY path
	`, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("get org unit tree: %w", err)
	}
	defer rows.Close()

	var allUnits []userstore.OrgUnit
	unitMap := make(map[uuid.UUID]*userstore.OrgUnitWithChildren)

	for rows.Next() {
		var ou userstore.OrgUnit
		if err := rows.Scan(
			&ou.ID, &ou.GatewayID, &ou.ParentID, &ou.Path, &ou.Depth, &ou.Name, &ou.Slug, &ou.UnitType,
			&ou.BudgetID, pq.Array(&ou.AllowedModels), &ou.Metadata, &ou.CreatedAt, &ou.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan org unit: %w", err)
		}
		allUnits = append(allUnits, ou)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Build tree structure
	for i := range allUnits {
		unitMap[allUnits[i].ID] = &userstore.OrgUnitWithChildren{
			OrgUnit:  allUnits[i],
			Children: nil,
		}
	}

	var roots []userstore.OrgUnitWithChildren
	for i := range allUnits {
		ou := unitMap[allUnits[i].ID]
		if ou.ParentID == nil {
			roots = append(roots, *ou)
		} else if parent, ok := unitMap[*ou.ParentID]; ok {
			parent.Children = append(parent.Children, *ou)
		}
	}

	return roots, nil
}

func (s *Store) GetOrgUnitChildren(ctx context.Context, parentID uuid.UUID) ([]userstore.OrgUnit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata, created_at, updated_at
		FROM org_units
		WHERE parent_id = $1 AND deleted_at IS NULL
		ORDER BY name
	`, parentID)
	if err != nil {
		return nil, fmt.Errorf("get org unit children: %w", err)
	}
	defer rows.Close()

	var units []userstore.OrgUnit
	for rows.Next() {
		var ou userstore.OrgUnit
		if err := rows.Scan(
			&ou.ID, &ou.GatewayID, &ou.ParentID, &ou.Path, &ou.Depth, &ou.Name, &ou.Slug, &ou.UnitType,
			&ou.BudgetID, pq.Array(&ou.AllowedModels), &ou.Metadata, &ou.CreatedAt, &ou.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan org unit: %w", err)
		}
		units = append(units, ou)
	}
	return units, rows.Err()
}

func (s *Store) GetOrgUnitsByPath(ctx context.Context, gatewayID uuid.UUID, pathPrefix string) ([]userstore.OrgUnit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata, created_at, updated_at
		FROM org_units
		WHERE gateway_id = $1 AND path LIKE $2 AND deleted_at IS NULL
		ORDER BY path
	`, gatewayID, pathPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("get org units by path: %w", err)
	}
	defer rows.Close()

	var units []userstore.OrgUnit
	for rows.Next() {
		var ou userstore.OrgUnit
		if err := rows.Scan(
			&ou.ID, &ou.GatewayID, &ou.ParentID, &ou.Path, &ou.Depth, &ou.Name, &ou.Slug, &ou.UnitType,
			&ou.BudgetID, pq.Array(&ou.AllowedModels), &ou.Metadata, &ou.CreatedAt, &ou.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan org unit: %w", err)
		}
		units = append(units, ou)
	}
	return units, rows.Err()
}

func (s *Store) UpdateOrgUnit(ctx context.Context, id uuid.UUID, updates userstore.OrgUnitUpdate) (*userstore.OrgUnit, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *updates.Name)
		argIdx++
	}
	if updates.Slug != nil {
		sets = append(sets, fmt.Sprintf("slug = $%d", argIdx))
		args = append(args, *updates.Slug)
		argIdx++
	}
	if updates.UnitType != nil {
		sets = append(sets, fmt.Sprintf("unit_type = $%d", argIdx))
		args = append(args, *updates.UnitType)
		argIdx++
	}
	if updates.BudgetID != nil {
		if *updates.BudgetID == uuid.Nil {
			sets = append(sets, "budget_id = NULL")
		} else {
			sets = append(sets, fmt.Sprintf("budget_id = $%d", argIdx))
			args = append(args, *updates.BudgetID)
			argIdx++
		}
	}
	if updates.AllowedModels != nil {
		sets = append(sets, fmt.Sprintf("allowed_models = $%d", argIdx))
		args = append(args, pq.Array(*updates.AllowedModels))
		argIdx++
	}
	if updates.Metadata != nil {
		sets = append(sets, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, *updates.Metadata)
		argIdx++
	}

	if len(sets) == 0 {
		return s.GetOrgUnit(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE org_units SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, gateway_id, parent_id, path, depth, name, slug, unit_type, budget_id, allowed_models, metadata, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	var ou userstore.OrgUnit
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&ou.ID, &ou.GatewayID, &ou.ParentID, &ou.Path, &ou.Depth, &ou.Name, &ou.Slug, &ou.UnitType,
		&ou.BudgetID, pq.Array(&ou.AllowedModels), &ou.Metadata, &ou.CreatedAt, &ou.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update org unit: %w", err)
	}
	return &ou, nil
}

func (s *Store) MoveOrgUnit(ctx context.Context, id uuid.UUID, newParentID *uuid.UUID) (*userstore.OrgUnit, error) {
	_, err := s.db.ExecContext(ctx, `SELECT move_org_unit($1, $2)`, id, newParentID)
	if err != nil {
		return nil, fmt.Errorf("move org unit: %w", err)
	}
	return s.GetOrgUnit(ctx, id)
}

func (s *Store) MergeOrgUnits(ctx context.Context, sourceID, targetID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Move all memberships from source to target
	_, err = tx.ExecContext(ctx, `
		UPDATE org_memberships
		SET org_unit_id = $2, updated_at = NOW()
		WHERE org_unit_id = $1 AND deleted_at IS NULL
		AND principal_id NOT IN (
			SELECT principal_id FROM org_memberships WHERE org_unit_id = $2 AND deleted_at IS NULL
		)
	`, sourceID, targetID)
	if err != nil {
		return fmt.Errorf("move memberships: %w", err)
	}

	// Delete duplicate memberships (principals already in target)
	_, err = tx.ExecContext(ctx, `
		UPDATE org_memberships SET deleted_at = NOW()
		WHERE org_unit_id = $1 AND deleted_at IS NULL
	`, sourceID)
	if err != nil {
		return fmt.Errorf("cleanup memberships: %w", err)
	}

	// Move children to target
	_, err = tx.ExecContext(ctx, `
		UPDATE org_units SET parent_id = $2, updated_at = NOW()
		WHERE parent_id = $1 AND deleted_at IS NULL
	`, sourceID, targetID)
	if err != nil {
		return fmt.Errorf("move children: %w", err)
	}

	// Soft delete source
	_, err = tx.ExecContext(ctx, `
		UPDATE org_units SET deleted_at = NOW() WHERE id = $1
	`, sourceID)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}

	return tx.Commit()
}

func (s *Store) DeleteOrgUnit(ctx context.Context, id uuid.UUID, force bool) error {
	if force {
		// Delete all descendants first
		_, err := s.db.ExecContext(ctx, `
			WITH RECURSIVE descendants AS (
				SELECT id FROM org_units WHERE id = $1
				UNION ALL
				SELECT ou.id FROM org_units ou
				JOIN descendants d ON ou.parent_id = d.id
				WHERE ou.deleted_at IS NULL
			)
			UPDATE org_units SET deleted_at = NOW()
			WHERE id IN (SELECT id FROM descendants)
		`, id)
		return err
	}

	// Check for children
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM org_units WHERE parent_id = $1 AND deleted_at IS NULL
	`, id).Scan(&count)
	if err != nil {
		return fmt.Errorf("check children: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("org unit has %d children, use force=true to delete", count)
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE org_units SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

// ==============================================================================
// Helper Functions
// ==============================================================================

// generateAPIKey creates a new random API key.
func generateAPIKey() (token string, hash string, prefix string, err error) {
	// Generate 32 random bytes
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", "", err
	}

	// Format: tok_<hex>
	token = "tok_" + hex.EncodeToString(tokenBytes)
	prefix = token[:12] // tok_ + 8 hex chars

	// Hash for storage
	h := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(h[:])

	return token, hash, prefix, nil
}
