package postgres_v2

import (
	"context"
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

// ==============================================================================
// API Key v2 Management
// ==============================================================================

func (s *Store) CreateAPIKeyV2(ctx context.Context, params userstore.CreateAPIKeyV2Params) (*userstore.APIKeyV2, string, error) {
	token, hash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}

	var k userstore.APIKeyV2
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO api_keys_v2 (gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at, last_used_at, total_spend, blocked, created_at, updated_at
	`, params.GatewayID, params.PrincipalID, params.OrgUnitID, hash, prefix, params.KeyName,
		params.BudgetID, pq.Array(params.AllowedModels), pq.Array(params.AllowedIPs),
		pq.Array(params.Scopes), params.ExpiresAt).Scan(
		&k.ID, &k.GatewayID, &k.PrincipalID, &k.OrgUnitID, &k.KeyHash, &k.KeyPrefix, &k.KeyName,
		&k.BudgetID, pq.Array(&k.AllowedModels), pq.Array(&k.AllowedIPs), pq.Array(&k.Scopes),
		&k.ExpiresAt, &k.LastUsedAt, &k.TotalSpend, &k.Blocked, &k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create api key: %w", err)
	}

	return &k, token, nil
}

func (s *Store) GetAPIKeyV2(ctx context.Context, id uuid.UUID) (*userstore.APIKeyV2, error) {
	var k userstore.APIKeyV2
	err := s.db.QueryRowContext(ctx, `
		SELECT id, gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at, last_used_at, total_spend, blocked, created_at, updated_at, deleted_at
		FROM api_keys_v2
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(
		&k.ID, &k.GatewayID, &k.PrincipalID, &k.OrgUnitID, &k.KeyHash, &k.KeyPrefix, &k.KeyName,
		&k.BudgetID, pq.Array(&k.AllowedModels), pq.Array(&k.AllowedIPs), pq.Array(&k.Scopes),
		&k.ExpiresAt, &k.LastUsedAt, &k.TotalSpend, &k.Blocked, &k.CreatedAt, &k.UpdatedAt, &k.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return &k, nil
}

func (s *Store) ListAPIKeysV2(ctx context.Context, gatewayID uuid.UUID, filter userstore.APIKeyFilter) ([]userstore.APIKeyV2, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("gateway_id = $%d", argIdx))
	args = append(args, gatewayID)
	argIdx++

	conditions = append(conditions, "deleted_at IS NULL")

	if filter.PrincipalID != nil {
		conditions = append(conditions, fmt.Sprintf("principal_id = $%d", argIdx))
		args = append(args, *filter.PrincipalID)
		argIdx++
	}

	if filter.OrgUnitID != nil {
		conditions = append(conditions, fmt.Sprintf("org_unit_id = $%d", argIdx))
		args = append(args, *filter.OrgUnitID)
		argIdx++
	}

	if filter.Blocked != nil {
		conditions = append(conditions, fmt.Sprintf("blocked = $%d", argIdx))
		args = append(args, *filter.Blocked)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT id, gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at, last_used_at, total_spend, blocked, created_at, updated_at
		FROM api_keys_v2
		WHERE %s
		ORDER BY created_at DESC
	`, strings.Join(conditions, " AND "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []userstore.APIKeyV2
	for rows.Next() {
		var k userstore.APIKeyV2
		if err := rows.Scan(
			&k.ID, &k.GatewayID, &k.PrincipalID, &k.OrgUnitID, &k.KeyHash, &k.KeyPrefix, &k.KeyName,
			&k.BudgetID, pq.Array(&k.AllowedModels), pq.Array(&k.AllowedIPs), pq.Array(&k.Scopes),
			&k.ExpiresAt, &k.LastUsedAt, &k.TotalSpend, &k.Blocked, &k.CreatedAt, &k.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Store) UpdateAPIKeyV2(ctx context.Context, id uuid.UUID, updates userstore.APIKeyV2Update) (*userstore.APIKeyV2, error) {
	var sets []string
	var args []interface{}
	argIdx := 1

	if updates.KeyName != nil {
		sets = append(sets, fmt.Sprintf("key_name = $%d", argIdx))
		args = append(args, *updates.KeyName)
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
	if updates.AllowedIPs != nil {
		sets = append(sets, fmt.Sprintf("allowed_ips = $%d", argIdx))
		args = append(args, pq.Array(*updates.AllowedIPs))
		argIdx++
	}
	if updates.ExpiresAt != nil {
		sets = append(sets, fmt.Sprintf("expires_at = $%d", argIdx))
		args = append(args, *updates.ExpiresAt)
		argIdx++
	}
	if updates.Blocked != nil {
		sets = append(sets, fmt.Sprintf("blocked = $%d", argIdx))
		args = append(args, *updates.Blocked)
		argIdx++
	}

	if len(sets) == 0 {
		return s.GetAPIKeyV2(ctx, id)
	}

	args = append(args, id)
	query := fmt.Sprintf(`
		UPDATE api_keys_v2 SET %s
		WHERE id = $%d AND deleted_at IS NULL
		RETURNING id, gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at, last_used_at, total_spend, blocked, created_at, updated_at
	`, strings.Join(sets, ", "), argIdx)

	var k userstore.APIKeyV2
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&k.ID, &k.GatewayID, &k.PrincipalID, &k.OrgUnitID, &k.KeyHash, &k.KeyPrefix, &k.KeyName,
		&k.BudgetID, pq.Array(&k.AllowedModels), pq.Array(&k.AllowedIPs), pq.Array(&k.Scopes),
		&k.ExpiresAt, &k.LastUsedAt, &k.TotalSpend, &k.Blocked, &k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update api key: %w", err)
	}
	return &k, nil
}

func (s *Store) RotateAPIKeyV2(ctx context.Context, id uuid.UUID, gracePeriod time.Duration) (*userstore.APIKeyV2, string, error) {
	// Get existing key info
	oldKey, err := s.GetAPIKeyV2(ctx, id)
	if err != nil {
		return nil, "", fmt.Errorf("get old key: %w", err)
	}
	if oldKey == nil {
		return nil, "", fmt.Errorf("key not found")
	}

	// Generate new key
	token, hash, prefix, err := generateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if gracePeriod > 0 {
		// Keep old key active for grace period
		expiry := time.Now().Add(gracePeriod)
		_, err = tx.ExecContext(ctx, `
			UPDATE api_keys_v2 SET expires_at = $2, updated_at = NOW()
			WHERE id = $1
		`, id, expiry)
		if err != nil {
			return nil, "", fmt.Errorf("set grace period: %w", err)
		}
	} else {
		// Immediately revoke old key
		_, err = tx.ExecContext(ctx, `
			UPDATE api_keys_v2 SET deleted_at = NOW() WHERE id = $1
		`, id)
		if err != nil {
			return nil, "", fmt.Errorf("revoke old key: %w", err)
		}
	}

	// Create new key with same settings
	var newKey userstore.APIKeyV2
	err = tx.QueryRowContext(ctx, `
		INSERT INTO api_keys_v2 (gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, gateway_id, principal_id, org_unit_id, key_hash, key_prefix, key_name, budget_id, allowed_models, allowed_ips, scopes, expires_at, last_used_at, total_spend, blocked, created_at, updated_at
	`, oldKey.GatewayID, oldKey.PrincipalID, oldKey.OrgUnitID, hash, prefix, oldKey.KeyName,
		oldKey.BudgetID, pq.Array(oldKey.AllowedModels), pq.Array(oldKey.AllowedIPs),
		pq.Array(oldKey.Scopes), oldKey.ExpiresAt).Scan(
		&newKey.ID, &newKey.GatewayID, &newKey.PrincipalID, &newKey.OrgUnitID, &newKey.KeyHash,
		&newKey.KeyPrefix, &newKey.KeyName, &newKey.BudgetID, pq.Array(&newKey.AllowedModels),
		pq.Array(&newKey.AllowedIPs), pq.Array(&newKey.Scopes), &newKey.ExpiresAt,
		&newKey.LastUsedAt, &newKey.TotalSpend, &newKey.Blocked, &newKey.CreatedAt, &newKey.UpdatedAt,
	)
	if err != nil {
		return nil, "", fmt.Errorf("create new key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}

	return &newKey, token, nil
}

func (s *Store) RevokeAPIKeyV2(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_keys_v2 SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL
	`, id)
	return err
}

func (s *Store) LookupAPIKeyV2(ctx context.Context, token string) (*userstore.APIKeyV2, *userstore.Principal, *userstore.Gateway, error) {
	// Hash the token for lookup
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	var k userstore.APIKeyV2
	var p userstore.Principal
	var g userstore.Gateway

	err := s.db.QueryRowContext(ctx, `
		SELECT
			k.id, k.gateway_id, k.principal_id, k.org_unit_id, k.key_hash, k.key_prefix, k.key_name,
			k.budget_id, k.allowed_models, k.allowed_ips, k.scopes, k.expires_at, k.last_used_at,
			k.total_spend, k.blocked, k.created_at, k.updated_at,
			p.id, p.gateway_id, p.principal_type, p.user_id, p.service_name, p.environment_name,
			p.display_name, p.budget_id, p.allowed_models, p.metadata, p.created_at, p.updated_at,
			g.id, g.alias, g.owner_user_id, g.provider_enabled, g.consumer_enabled, g.metadata, g.created_at, g.updated_at
		FROM api_keys_v2 k
		JOIN principals p ON k.principal_id = p.id
		JOIN gateways g ON k.gateway_id = g.id
		WHERE k.key_hash = $1
			AND k.deleted_at IS NULL
			AND k.blocked = false
			AND (k.expires_at IS NULL OR k.expires_at > NOW())
			AND p.deleted_at IS NULL
			AND g.deleted_at IS NULL
	`, hash).Scan(
		&k.ID, &k.GatewayID, &k.PrincipalID, &k.OrgUnitID, &k.KeyHash, &k.KeyPrefix, &k.KeyName,
		&k.BudgetID, pq.Array(&k.AllowedModels), pq.Array(&k.AllowedIPs), pq.Array(&k.Scopes),
		&k.ExpiresAt, &k.LastUsedAt, &k.TotalSpend, &k.Blocked, &k.CreatedAt, &k.UpdatedAt,
		&p.ID, &p.GatewayID, &p.PrincipalType, &p.UserID, &p.ServiceName, &p.EnvironmentName,
		&p.DisplayName, &p.BudgetID, pq.Array(&p.AllowedModels), &p.Metadata, &p.CreatedAt, &p.UpdatedAt,
		&g.ID, &g.Alias, &g.OwnerUserID, &g.ProviderEnabled, &g.ConsumerEnabled, &g.Metadata, &g.CreatedAt, &g.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("lookup api key: %w", err)
	}

	return &k, &p, &g, nil
}

func (s *Store) RecordAPIKeyUsage(ctx context.Context, keyID uuid.UUID, spend float64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE api_keys_v2
		SET last_used_at = NOW(), total_spend = total_spend + $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, keyID, spend)
	return err
}
