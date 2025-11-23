package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
)

// APIKeyMapper handles API key to priority mapping with database backend and caching
type APIKeyMapper struct {
	db              *sql.DB
	mappings        []*PriorityMapping // Cached mappings (in-memory)
	defaultPriority PriorityTier
	enabled         bool
	cacheTTL        time.Duration
	lastReload      time.Time
	mu              sync.RWMutex // Protects mappings and lastReload
}

// NewAPIKeyMapper creates a new API key mapper with PostgreSQL backend
func NewAPIKeyMapper(db *sql.DB, defaultPriority PriorityTier, enabled bool, cacheTTL time.Duration) (*APIKeyMapper, error) {
	mapper := &APIKeyMapper{
		db:              db,
		mappings:        make([]*PriorityMapping, 0),
		defaultPriority: defaultPriority,
		enabled:         enabled,
		cacheTTL:        cacheTTL,
	}

	// Create tables if not exist (idempotent)
	if err := mapper.initializeTables(); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	// Load mappings from database if enabled
	if enabled {
		if err := mapper.Reload(); err != nil {
			return nil, fmt.Errorf("failed to load mappings: %w", err)
		}
	}

	return mapper, nil
}

// initializeTables creates required database tables (idempotent)
func (m *APIKeyMapper) initializeTables() error {
	// Tables are created via migration 002_api_key_priority.sql
	// This is a no-op, just for interface compatibility
	return nil
}

// IsEnabled returns whether API key priority mapping is enabled
func (m *APIKeyMapper) IsEnabled() bool {
	return m.enabled
}

// GetPriority returns the priority for a given API key
func (m *APIKeyMapper) GetPriority(apiKey string) PriorityTier {
	if !m.enabled {
		return m.defaultPriority
	}

	// Check if cache needs reload (TTL expired)
	if time.Since(m.lastReload) > m.cacheTTL {
		if err := m.Reload(); err != nil {
			log.Printf("[WARN] APIKeyMapper: Failed to reload cache: %v (continuing with stale cache)", err)
			// Continue with stale cache (graceful degradation)
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try each mapping in order (first match wins)
	for _, mapping := range m.mappings {
		if !mapping.Enabled {
			continue
		}
		if mapping.matchFunc(apiKey) {
			return mapping.Priority
		}
	}

	// No match, return default
	return m.defaultPriority
}

// Reload reloads mappings from database (with soft delete filter)
func (m *APIKeyMapper) Reload() error {
	ctx := context.Background()

	query := `
		SELECT id, pattern, priority, match_type,
		       tenant_id, tenant_name, tenant_type, description, enabled
		FROM api_key_priority_mappings
		WHERE deleted_at IS NULL
		ORDER BY priority ASC, id ASC
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query mappings: %w", err)
	}
	defer rows.Close()

	var newMappings []*PriorityMapping

	for rows.Next() {
		var model PriorityMappingModel
		var tenantID, tenantName, tenantType, description sql.NullString

		if err := rows.Scan(
			&model.ID, &model.Pattern, &model.Priority, &model.MatchType,
			&tenantID, &tenantName, &tenantType, &description, &model.Enabled,
		); err != nil {
			log.Printf("[WARN] APIKeyMapper: Failed to scan row: %v", err)
			continue
		}

		// Handle NULL values
		model.TenantID = tenantID.String
		model.TenantName = tenantName.String
		model.TenantType = tenantType.String
		model.Description = description.String

		mapping := &PriorityMapping{
			ID:          model.ID,
			Pattern:     model.Pattern,
			Priority:    PriorityTier(model.Priority),
			MatchType:   ParseMatchType(model.MatchType),
			TenantID:    model.TenantID,
			TenantName:  model.TenantName,
			TenantType:  model.TenantType,
			Description: model.Description,
			Enabled:     model.Enabled,
		}

		// Compile pattern into match function (one-time cost during reload)
		if err := mapping.compile(); err != nil {
			log.Printf("[WARN] APIKeyMapper: Failed to compile pattern %q: %v", model.Pattern, err)
			continue
		}

		newMappings = append(newMappings, mapping)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	// Atomic cache swap (mutex-protected)
	m.mu.Lock()
	m.mappings = newMappings
	m.lastReload = time.Now()
	m.mu.Unlock()

	log.Printf("[INFO] APIKeyMapper: Reloaded %d mappings from database (tenants=%d internal, %d external)",
		len(newMappings),
		countTenantsByType(newMappings, "internal"),
		countTenantsByType(newMappings, "external"))

	return nil
}

// countTenantsByType counts mappings by tenant type
func countTenantsByType(mappings []*PriorityMapping, tenantType string) int {
	count := 0
	for _, m := range mappings {
		if m.TenantType == tenantType {
			count++
		}
	}
	return count
}

// AddMapping adds a new pattern-to-priority mapping to database (returns UUID)
func (m *APIKeyMapper) AddMapping(ctx context.Context, pattern string, priority PriorityTier, matchType MatchType,
	tenantID, tenantName, tenantType, description, createdBy string) (string, error) {

	query := `
		INSERT INTO api_key_priority_mappings
		(pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING id
	`

	var id string
	err := m.db.QueryRowContext(ctx, query,
		pattern, int(priority), matchType.String(),
		nullString(tenantID), nullString(tenantName), nullString(tenantType),
		nullString(description), createdBy,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("failed to insert mapping: %w", err)
	}

	log.Printf("[INFO] APIKeyMapper: Added mapping for tenant '%s' (%s): pattern=%s priority=P%d uuid=%s",
		tenantID, tenantType, pattern, priority, id)

	// Reload cache to pick up new mapping
	if err := m.Reload(); err != nil {
		log.Printf("[WARN] APIKeyMapper: Failed to reload after add: %v", err)
	}

	return id, nil
}

// UpdateMapping updates an existing mapping (UUID param)
func (m *APIKeyMapper) UpdateMapping(ctx context.Context, id string, priority PriorityTier, description string, enabled bool, updatedBy string) error {
	query := `
		UPDATE api_key_priority_mappings
		SET priority = $1, description = $2, enabled = $3, updated_at = NOW(), updated_by = $4
		WHERE id = $5 AND deleted_at IS NULL
	`

	result, err := m.db.ExecContext(ctx, query, int(priority), nullString(description), enabled, updatedBy, id)
	if err != nil {
		return fmt.Errorf("failed to update mapping: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("record not found or already deleted")
	}

	log.Printf("[INFO] APIKeyMapper: Updated mapping uuid=%s priority=P%d enabled=%v", id, priority, enabled)

	// Reload cache
	if err := m.Reload(); err != nil {
		log.Printf("[WARN] APIKeyMapper: Failed to reload after update: %v", err)
	}

	return nil
}

// DeleteMapping soft deletes a mapping (UUID param)
func (m *APIKeyMapper) DeleteMapping(ctx context.Context, id string, deletedBy string) error {
	query := `
		UPDATE api_key_priority_mappings
		SET deleted_at = NOW(), updated_at = NOW(), updated_by = $1
		WHERE id = $2 AND deleted_at IS NULL
	`

	result, err := m.db.ExecContext(ctx, query, deletedBy, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete mapping: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("record not found or already deleted")
	}

	log.Printf("[INFO] APIKeyMapper: Soft deleted mapping uuid=%s", id)

	// Reload cache
	if err := m.Reload(); err != nil {
		log.Printf("[WARN] APIKeyMapper: Failed to reload after delete: %v", err)
	}

	return nil
}

// ListMappings returns all active mappings (for admin UI)
func (m *APIKeyMapper) ListMappings(ctx context.Context) ([]*PriorityMappingModel, error) {
	query := `
		SELECT id, pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description,
		       enabled, created_at, updated_at, created_by, updated_by
		FROM api_key_priority_mappings
		WHERE deleted_at IS NULL
		ORDER BY priority ASC, created_at DESC
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*PriorityMappingModel

	for rows.Next() {
		var m PriorityMappingModel
		var tenantID, tenantName, tenantType, description, createdBy, updatedBy sql.NullString

		if err := rows.Scan(
			&m.ID, &m.Pattern, &m.Priority, &m.MatchType,
			&tenantID, &tenantName, &tenantType, &description,
			&m.Enabled, &m.CreatedAt, &m.UpdatedAt, &createdBy, &updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		m.TenantID = tenantID.String
		m.TenantName = tenantName.String
		m.TenantType = tenantType.String
		m.Description = description.String
		m.CreatedBy = createdBy.String
		m.UpdatedBy = updatedBy.String

		mappings = append(mappings, &m)
	}

	return mappings, rows.Err()
}

// compile compiles a pattern into an efficient match function
func (pm *PriorityMapping) compile() error {
	pattern := pm.Pattern

	// Use match type from database
	switch pm.MatchType {
	case MatchExact:
		pm.matchFunc = func(key string) bool {
			return key == pattern
		}
		return nil

	case MatchPrefix:
		prefix := strings.TrimSuffix(pattern, "*")
		pm.matchFunc = func(key string) bool {
			return strings.HasPrefix(key, prefix)
		}
		return nil

	case MatchSuffix:
		suffix := strings.TrimPrefix(pattern, "*")
		pm.matchFunc = func(key string) bool {
			return strings.HasSuffix(key, suffix)
		}
		return nil

	case MatchContains:
		substr := strings.Trim(pattern, "*")
		pm.matchFunc = func(key string) bool {
			return strings.Contains(key, substr)
		}
		return nil

	case MatchRegex:
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
		pm.matchFunc = func(key string) bool {
			return regex.MatchString(key)
		}
		return nil

	default:
		return fmt.Errorf("unknown match type: %v", pm.MatchType)
	}
}

// nullString converts empty string to sql.NullString
func nullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}
