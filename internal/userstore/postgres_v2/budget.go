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
// Budget Management
// ==============================================================================

func (s *Store) CreateBudget(ctx context.Context, gatewayID uuid.UUID, params userstore.CreateBudgetParams) (*userstore.Budget, error) {
	var b userstore.Budget
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO budgets (gateway_id, name, max_budget, budget_duration, tpm_limit, rpm_limit, soft_limit, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, gateway_id, name, max_budget, budget_duration, tpm_limit, rpm_limit, soft_limit, metadata, created_at, updated_at
	`, gatewayID, params.Name, params.MaxBudget, params.BudgetDuration,
		params.TPMLimit, params.RPMLimit, params.SoftLimit, params.Metadata).Scan(
		&b.ID, &b.GatewayID, &b.Name, &b.MaxBudget, &b.BudgetDuration,
		&b.TPMLimit, &b.RPMLimit, &b.SoftLimit, &b.Metadata, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create budget: %w", err)
	}
	return &b, nil
}

func (s *Store) GetBudget(ctx context.Context, id uuid.UUID) (*userstore.Budget, error) {
	var b userstore.Budget
	err := s.db.QueryRowContext(ctx, `
		SELECT id, gateway_id, name, max_budget, budget_duration, tpm_limit, rpm_limit, soft_limit, metadata, created_at, updated_at, deleted_at
		FROM budgets
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&b.ID, &b.GatewayID, &b.Name, &b.MaxBudget, &b.BudgetDuration,
		&b.TPMLimit, &b.RPMLimit, &b.SoftLimit, &b.Metadata, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get budget: %w", err)
	}
	return &b, nil
}

func (s *Store) ListBudgets(ctx context.Context, gatewayID uuid.UUID) ([]userstore.Budget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, gateway_id, name, max_budget, budget_duration, tpm_limit, rpm_limit, soft_limit, metadata, created_at, updated_at
		FROM budgets
		WHERE gateway_id = $1 AND deleted_at IS NULL
		ORDER BY name
	`, gatewayID)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	var budgets []userstore.Budget
	for rows.Next() {
		var b userstore.Budget
		if err := rows.Scan(
			&b.ID, &b.GatewayID, &b.Name, &b.MaxBudget, &b.BudgetDuration,
			&b.TPMLimit, &b.RPMLimit, &b.SoftLimit, &b.Metadata, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		budgets = append(budgets, b)
	}
	return budgets, rows.Err()
}

func (s *Store) UpdateBudget(ctx context.Context, id uuid.UUID, updates userstore.BudgetUpdate) (*userstore.Budget, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *updates.Name)
		argIdx++
	}
	if updates.MaxBudget != nil {
		sets = append(sets, fmt.Sprintf("max_budget = $%d", argIdx))
		args = append(args, *updates.MaxBudget)
		argIdx++
	}
	if updates.BudgetDuration != nil {
		sets = append(sets, fmt.Sprintf("budget_duration = $%d", argIdx))
		args = append(args, *updates.BudgetDuration)
		argIdx++
	}
	if updates.TPMLimit != nil {
		sets = append(sets, fmt.Sprintf("tpm_limit = $%d", argIdx))
		args = append(args, *updates.TPMLimit)
		argIdx++
	}
	if updates.RPMLimit != nil {
		sets = append(sets, fmt.Sprintf("rpm_limit = $%d", argIdx))
		args = append(args, *updates.RPMLimit)
		argIdx++
	}
	if updates.SoftLimit != nil {
		sets = append(sets, fmt.Sprintf("soft_limit = $%d", argIdx))
		args = append(args, *updates.SoftLimit)
		argIdx++
	}
	if updates.Metadata != nil {
		sets = append(sets, fmt.Sprintf("metadata = $%d", argIdx))
		args = append(args, *updates.Metadata)
		argIdx++
	}

	if len(sets) == 0 {
		return s.GetBudget(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE budgets SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, gateway_id, name, max_budget, budget_duration, tpm_limit, rpm_limit, soft_limit, metadata, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	var b userstore.Budget
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&b.ID, &b.GatewayID, &b.Name, &b.MaxBudget, &b.BudgetDuration,
		&b.TPMLimit, &b.RPMLimit, &b.SoftLimit, &b.Metadata, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update budget: %w", err)
	}
	return &b, nil
}

func (s *Store) DeleteBudget(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE budgets SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

// ResolveBudget resolves the effective budget for a principal following the inheritance chain:
// 1. Principal's own budget
// 2. OrgMembership budget (primary membership first)
// 3. Direct OrgUnit budget
// 4. Parent OrgUnit budget (recursively)
// 5. Gateway default (no explicit budget = unlimited)
func (s *Store) ResolveBudget(ctx context.Context, principalID uuid.UUID) (*userstore.BudgetInheritance, error) {
	result := &userstore.BudgetInheritance{
		Chain: make([]userstore.BudgetSource, 0),
	}

	// Get principal info
	var p userstore.Principal
	err := s.db.QueryRowContext(ctx, `
		SELECT id, gateway_id, display_name, budget_id
		FROM principals
		WHERE id = $1 AND deleted_at IS NULL
	`, principalID).Scan(&p.ID, &p.GatewayID, &p.DisplayName, &p.BudgetID)
	if err != nil {
		return nil, fmt.Errorf("get principal: %w", err)
	}

	// 1. Check Principal's own budget
	var principalBudget *userstore.Budget
	if p.BudgetID != nil {
		principalBudget, err = s.GetBudget(ctx, *p.BudgetID)
		if err != nil {
			return nil, fmt.Errorf("get principal budget: %w", err)
		}
	}
	result.Chain = append(result.Chain, userstore.BudgetSource{
		Type:   "principal",
		ID:     p.ID,
		Name:   p.DisplayName,
		Budget: principalBudget,
	})
	if principalBudget != nil {
		result.EffectiveBudget = principalBudget
		result.Source = "principal"
		return result, nil
	}

	// 2. Check primary OrgMembership budget
	var membershipID uuid.UUID
	var membershipBudgetID *uuid.UUID
	var orgUnitID uuid.UUID
	var membershipName string
	err = s.db.QueryRowContext(ctx, `
		SELECT m.id, m.budget_id, m.org_unit_id, ou.name
		FROM org_memberships m
		JOIN org_units ou ON m.org_unit_id = ou.id
		WHERE m.principal_id = $1 AND m.deleted_at IS NULL AND m.is_primary = true AND ou.deleted_at IS NULL
		LIMIT 1
	`, principalID).Scan(&membershipID, &membershipBudgetID, &orgUnitID, &membershipName)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get primary membership: %w", err)
	}

	if err == nil {
		var membershipBudget *userstore.Budget
		if membershipBudgetID != nil {
			membershipBudget, err = s.GetBudget(ctx, *membershipBudgetID)
			if err != nil {
				return nil, fmt.Errorf("get membership budget: %w", err)
			}
		}
		result.Chain = append(result.Chain, userstore.BudgetSource{
			Type:   "membership",
			ID:     membershipID,
			Name:   "Membership: " + membershipName,
			Budget: membershipBudget,
		})
		if membershipBudget != nil {
			result.EffectiveBudget = membershipBudget
			result.Source = "membership"
			return result, nil
		}

		// 3 & 4. Walk up OrgUnit hierarchy
		currentOrgUnitID := orgUnitID
		for {
			var ou userstore.OrgUnit
			err = s.db.QueryRowContext(ctx, `
				SELECT id, parent_id, name, budget_id
				FROM org_units
				WHERE id = $1 AND deleted_at IS NULL
			`, currentOrgUnitID).Scan(&ou.ID, &ou.ParentID, &ou.Name, &ou.BudgetID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					break
				}
				return nil, fmt.Errorf("get org unit: %w", err)
			}

			var ouBudget *userstore.Budget
			if ou.BudgetID != nil {
				ouBudget, err = s.GetBudget(ctx, *ou.BudgetID)
				if err != nil {
					return nil, fmt.Errorf("get org unit budget: %w", err)
				}
			}
			result.Chain = append(result.Chain, userstore.BudgetSource{
				Type:   "orgunit",
				ID:     ou.ID,
				Name:   ou.Name,
				Budget: ouBudget,
			})
			if ouBudget != nil {
				result.EffectiveBudget = ouBudget
				result.Source = "orgunit:" + ou.Name
				return result, nil
			}

			if ou.ParentID == nil {
				break
			}
			currentOrgUnitID = *ou.ParentID
		}
	}

	// 5. Gateway default (no budget = unlimited)
	result.Chain = append(result.Chain, userstore.BudgetSource{
		Type:   "gateway",
		ID:     p.GatewayID,
		Name:   "Gateway Default",
		Budget: nil, // No limit
	})
	result.Source = "gateway"
	// EffectiveBudget remains nil (unlimited)

	return result, nil
}
