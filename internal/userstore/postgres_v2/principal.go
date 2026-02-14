package postgres_v2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

// ==============================================================================
// Principal Management
// ==============================================================================

func (s *Store) CreatePrincipal(ctx context.Context, params userstore.CreatePrincipalParams) (*userstore.Principal, error) {
	var p userstore.Principal
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO principals (gateway_id, principal_type, user_id, service_name, environment_name, display_name, budget_id, allowed_models, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, gateway_id, principal_type, user_id, service_name, environment_name, display_name, budget_id, allowed_models, metadata, created_at, updated_at
	`, params.GatewayID, params.PrincipalType, params.UserID, params.ServiceName, params.EnvironmentName,
		params.DisplayName, params.BudgetID, pq.Array(params.AllowedModels), params.Metadata).Scan(
		&p.ID, &p.GatewayID, &p.PrincipalType, &p.UserID, &p.ServiceName, &p.EnvironmentName,
		&p.DisplayName, &p.BudgetID, pq.Array(&p.AllowedModels), &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create principal: %w", err)
	}
	return &p, nil
}

func (s *Store) GetPrincipal(ctx context.Context, id uuid.UUID) (*userstore.Principal, error) {
	var p userstore.Principal
	err := s.db.QueryRowContext(ctx, `
		SELECT id, gateway_id, principal_type, user_id, service_name, environment_name, display_name, budget_id, allowed_models, metadata, created_at, updated_at, deleted_at
		FROM principals
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&p.ID, &p.GatewayID, &p.PrincipalType, &p.UserID, &p.ServiceName, &p.EnvironmentName,
		&p.DisplayName, &p.BudgetID, pq.Array(&p.AllowedModels), &p.Metadata, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get principal: %w", err)
	}
	return &p, nil
}

func (s *Store) GetPrincipalByUserID(ctx context.Context, gatewayID, userID uuid.UUID) (*userstore.Principal, error) {
	var p userstore.Principal
	err := s.db.QueryRowContext(ctx, `
		SELECT id, gateway_id, principal_type, user_id, service_name, environment_name, display_name, budget_id, allowed_models, metadata, created_at, updated_at
		FROM principals
		WHERE gateway_id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, gatewayID, userID).Scan(
		&p.ID, &p.GatewayID, &p.PrincipalType, &p.UserID, &p.ServiceName, &p.EnvironmentName,
		&p.DisplayName, &p.BudgetID, pq.Array(&p.AllowedModels), &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get principal by user: %w", err)
	}
	return &p, nil
}

func (s *Store) ListPrincipals(ctx context.Context, gatewayID uuid.UUID, filter userstore.PrincipalFilter) ([]userstore.Principal, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("gateway_id = $%d", argIdx))
	args = append(args, gatewayID)
	argIdx++

	conditions = append(conditions, "deleted_at IS NULL")

	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("principal_type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}

	if filter.OrgUnitID != nil {
		conditions = append(conditions, fmt.Sprintf(`
			id IN (SELECT principal_id FROM org_memberships WHERE org_unit_id = $%d AND deleted_at IS NULL)
		`, argIdx))
		args = append(args, *filter.OrgUnitID)
		argIdx++
	}

	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("display_name ILIKE $%d", argIdx))
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT id, gateway_id, principal_type, user_id, service_name, environment_name, display_name, budget_id, allowed_models, metadata, created_at, updated_at
		FROM principals
		WHERE %s
		ORDER BY display_name
	`, strings.Join(conditions, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list principals: %w", err)
	}
	defer rows.Close()

	var principals []userstore.Principal
	for rows.Next() {
		var p userstore.Principal
		if err := rows.Scan(
			&p.ID, &p.GatewayID, &p.PrincipalType, &p.UserID, &p.ServiceName, &p.EnvironmentName,
			&p.DisplayName, &p.BudgetID, pq.Array(&p.AllowedModels), &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan principal: %w", err)
		}
		principals = append(principals, p)
	}
	return principals, rows.Err()
}

func (s *Store) UpdatePrincipal(ctx context.Context, id uuid.UUID, updates userstore.PrincipalUpdate) (*userstore.Principal, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.DisplayName != nil {
		sets = append(sets, fmt.Sprintf("display_name = $%d", argIdx))
		args = append(args, *updates.DisplayName)
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
		return s.GetPrincipal(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE principals SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, gateway_id, principal_type, user_id, service_name, environment_name, display_name, budget_id, allowed_models, metadata, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	var p userstore.Principal
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&p.ID, &p.GatewayID, &p.PrincipalType, &p.UserID, &p.ServiceName, &p.EnvironmentName,
		&p.DisplayName, &p.BudgetID, pq.Array(&p.AllowedModels), &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update principal: %w", err)
	}
	return &p, nil
}

func (s *Store) DeletePrincipal(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE principals SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

// ==============================================================================
// OrgMembership Management
// ==============================================================================

func (s *Store) AddOrgMembership(ctx context.Context, params userstore.CreateOrgMembershipParams) (*userstore.OrgMembership, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// If this is primary, unset other primary memberships for this principal
	if params.IsPrimary {
		_, err = tx.ExecContext(ctx, `
			UPDATE org_memberships SET is_primary = false, updated_at = NOW()
			WHERE principal_id = $1 AND deleted_at IS NULL
		`, params.PrincipalID)
		if err != nil {
			return nil, fmt.Errorf("unset primary: %w", err)
		}
	}

	var m userstore.OrgMembership
	err = tx.QueryRowContext(ctx, `
		INSERT INTO org_memberships (principal_id, org_unit_id, role, budget_id, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, principal_id, org_unit_id, role, budget_id, is_primary, created_at, updated_at
	`, params.PrincipalID, params.OrgUnitID, params.Role, params.BudgetID, params.IsPrimary).Scan(
		&m.ID, &m.PrincipalID, &m.OrgUnitID, &m.Role, &m.BudgetID, &m.IsPrimary, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("add org membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &m, nil
}

func (s *Store) ListOrgMemberships(ctx context.Context, principalID uuid.UUID) ([]userstore.OrgMembershipWithOrgUnit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.principal_id, m.org_unit_id, m.role, m.budget_id, m.is_primary, m.created_at, m.updated_at,
		       ou.id, ou.gateway_id, ou.parent_id, ou.path, ou.depth, ou.name, ou.slug, ou.unit_type, ou.budget_id, ou.allowed_models, ou.metadata, ou.created_at, ou.updated_at
		FROM org_memberships m
		JOIN org_units ou ON m.org_unit_id = ou.id
		WHERE m.principal_id = $1 AND m.deleted_at IS NULL AND ou.deleted_at IS NULL
		ORDER BY m.is_primary DESC, ou.path
	`, principalID)
	if err != nil {
		return nil, fmt.Errorf("list org memberships: %w", err)
	}
	defer rows.Close()

	var memberships []userstore.OrgMembershipWithOrgUnit
	for rows.Next() {
		var m userstore.OrgMembershipWithOrgUnit
		if err := rows.Scan(
			&m.ID, &m.PrincipalID, &m.OrgUnitID, &m.Role, &m.BudgetID, &m.IsPrimary, &m.CreatedAt, &m.UpdatedAt,
			&m.OrgUnit.ID, &m.OrgUnit.GatewayID, &m.OrgUnit.ParentID, &m.OrgUnit.Path, &m.OrgUnit.Depth,
			&m.OrgUnit.Name, &m.OrgUnit.Slug, &m.OrgUnit.UnitType, &m.OrgUnit.BudgetID,
			pq.Array(&m.OrgUnit.AllowedModels), &m.OrgUnit.Metadata, &m.OrgUnit.CreatedAt, &m.OrgUnit.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

func (s *Store) ListOrgUnitMembers(ctx context.Context, orgUnitID uuid.UUID) ([]userstore.OrgMembershipWithPrincipal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.principal_id, m.org_unit_id, m.role, m.budget_id, m.is_primary, m.created_at, m.updated_at,
		       p.id, p.gateway_id, p.principal_type, p.user_id, p.service_name, p.environment_name, p.display_name, p.budget_id, p.allowed_models, p.metadata, p.created_at, p.updated_at
		FROM org_memberships m
		JOIN principals p ON m.principal_id = p.id
		WHERE m.org_unit_id = $1 AND m.deleted_at IS NULL AND p.deleted_at IS NULL
		ORDER BY p.display_name
	`, orgUnitID)
	if err != nil {
		return nil, fmt.Errorf("list org unit members: %w", err)
	}
	defer rows.Close()

	var memberships []userstore.OrgMembershipWithPrincipal
	for rows.Next() {
		var m userstore.OrgMembershipWithPrincipal
		if err := rows.Scan(
			&m.ID, &m.PrincipalID, &m.OrgUnitID, &m.Role, &m.BudgetID, &m.IsPrimary, &m.CreatedAt, &m.UpdatedAt,
			&m.Principal.ID, &m.Principal.GatewayID, &m.Principal.PrincipalType, &m.Principal.UserID,
			&m.Principal.ServiceName, &m.Principal.EnvironmentName, &m.Principal.DisplayName,
			&m.Principal.BudgetID, pq.Array(&m.Principal.AllowedModels), &m.Principal.Metadata,
			&m.Principal.CreatedAt, &m.Principal.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

func (s *Store) UpdateOrgMembership(ctx context.Context, membershipID uuid.UUID, updates userstore.OrgMembershipUpdate) error {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.Role != nil {
		sets = append(sets, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *updates.Role)
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
	if updates.IsPrimary != nil {
		sets = append(sets, fmt.Sprintf("is_primary = $%d", argIdx))
		args = append(args, *updates.IsPrimary)
		argIdx++
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, membershipID)
	query := fmt.Sprintf(`
		UPDATE org_memberships SET %s WHERE id = $%d AND deleted_at IS NULL
	`, strings.Join(sets, ", "), argIdx)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *Store) RemoveOrgMembership(ctx context.Context, membershipID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE org_memberships SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, membershipID)
	return err
}

func (s *Store) SetPrimaryMembership(ctx context.Context, principalID, membershipID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Unset all primary memberships for this principal
	_, err = tx.ExecContext(ctx, `
		UPDATE org_memberships SET is_primary = false, updated_at = NOW()
		WHERE principal_id = $1 AND deleted_at IS NULL
	`, principalID)
	if err != nil {
		return fmt.Errorf("unset primary: %w", err)
	}

	// Set the new primary
	_, err = tx.ExecContext(ctx, `
		UPDATE org_memberships SET is_primary = true, updated_at = NOW()
		WHERE id = $1 AND principal_id = $2 AND deleted_at IS NULL
	`, membershipID, principalID)
	if err != nil {
		return fmt.Errorf("set primary: %w", err)
	}

	return tx.Commit()
}
