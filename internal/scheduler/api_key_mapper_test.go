package scheduler

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewAPIKeyMapper tests initialization with PostgreSQL backend
func TestNewAPIKeyMapper(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, mapper)
	assert.True(t, mapper.IsEnabled())
	assert.Equal(t, PriorityStandard, mapper.defaultPriority)
	assert.Equal(t, 5*time.Minute, mapper.cacheTTL)
}

// TestNewAPIKeyMapperDisabled tests disabled mode (Personal Edition)
func TestNewAPIKeyMapperDisabled(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, false, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, mapper)
	assert.False(t, mapper.IsEnabled())

	// When disabled, should always return default priority
	priority := mapper.GetPriority("tok_prod_12345")
	assert.Equal(t, PriorityStandard, priority)
}

// TestAddMapping_UUID tests that AddMapping returns UUID (not int)
func TestAddMapping_UUID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()
	id, err := mapper.AddMapping(ctx,
		"tok_test*", PriorityCritical, MatchPrefix,
		"dept-test", "Test Department", "internal",
		"Test mapping", "test-user",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, id)
	// UUID format check (8-4-4-4-12 hex characters)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}

// TestGetPriority_PatternMatching tests all match types
func TestGetPriority_PatternMatching(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add test mappings
	tests := []struct {
		pattern   string
		matchType MatchType
		priority  PriorityTier
		testKey   string
		expected  PriorityTier
	}{
		// Exact match
		{"tok_exact_key", MatchExact, PriorityCritical, "tok_exact_key", PriorityCritical},
		{"tok_exact_other", MatchExact, PriorityCritical, "tok_exact_key_not", PriorityStandard}, // No match

		// Prefix match
		{"tok_prod*", MatchPrefix, PriorityUrgent, "tok_prod_12345", PriorityUrgent},
		{"tok_stage*", MatchPrefix, PriorityUrgent, "tok_dev_12345", PriorityStandard}, // No match

		// Suffix match
		{"*_premium", MatchSuffix, PriorityHigh, "user_premium", PriorityHigh},
		{"*_vip", MatchSuffix, PriorityHigh, "premium_user", PriorityStandard}, // No match

		// Contains match
		{"*ml*", MatchContains, PriorityNormal, "tok_ml_team", PriorityNormal},
		{"*nlp*", MatchContains, PriorityNormal, "tok_analytics", PriorityStandard}, // No match

		// Regex match
		{"^tok_dev_[0-9]+$", MatchRegex, PriorityLow, "tok_dev_123", PriorityLow},
		{"^tok_stage_[0-9]+$", MatchRegex, PriorityLow, "tok_dev_abc", PriorityStandard}, // No match
	}

	for i, tt := range tests {
		// Add mapping
		_, err := mapper.AddMapping(ctx,
			tt.pattern, tt.priority, tt.matchType,
			"", "", "", "", "test-user",
		)
		require.NoError(t, err, "Test case %d failed to add mapping", i)

		// Reload to pick up new mapping
		err = mapper.Reload()
		require.NoError(t, err)

		// Test pattern matching
		priority := mapper.GetPriority(tt.testKey)
		assert.Equal(t, tt.expected, priority, "Test case %d: pattern=%s, testKey=%s", i, tt.pattern, tt.testKey)
	}
}

// TestSoftDelete tests that deleted mappings are filtered out
func TestSoftDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add mapping
	id, err := mapper.AddMapping(ctx,
		"tok_deleted*", PriorityCritical, MatchPrefix,
		"dept-test", "Test", "internal", "Test soft delete", "test-user",
	)
	require.NoError(t, err)

	// Verify mapping works
	priority := mapper.GetPriority("tok_deleted_123")
	assert.Equal(t, PriorityCritical, priority)

	// Soft delete the mapping
	err = mapper.DeleteMapping(ctx, id, "test-user")
	require.NoError(t, err)

	// Verify mapping no longer works (returns default)
	priority = mapper.GetPriority("tok_deleted_123")
	assert.Equal(t, PriorityStandard, priority)

	// Verify soft delete in database (deleted_at IS NOT NULL)
	var deletedAt sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT deleted_at FROM api_key_priority_mappings WHERE id = $1`, id).Scan(&deletedAt)
	require.NoError(t, err)
	assert.True(t, deletedAt.Valid, "deleted_at should not be NULL")
	assert.False(t, deletedAt.Time.IsZero(), "deleted_at should have a timestamp")
}

// TestUpdateMapping tests UUID-based updates
func TestUpdateMapping(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add mapping
	id, err := mapper.AddMapping(ctx,
		"tok_update*", PriorityCritical, MatchPrefix,
		"dept-test", "Test", "internal", "Test update", "test-user",
	)
	require.NoError(t, err)

	// Verify initial priority
	priority := mapper.GetPriority("tok_update_123")
	assert.Equal(t, PriorityCritical, priority)

	// Update mapping to different priority
	err = mapper.UpdateMapping(ctx, id, PriorityLow, "Updated description", true, "test-user")
	require.NoError(t, err)

	// Verify updated priority
	priority = mapper.GetPriority("tok_update_123")
	assert.Equal(t, PriorityLow, priority)

	// Disable mapping
	err = mapper.UpdateMapping(ctx, id, PriorityLow, "Disabled", false, "test-user")
	require.NoError(t, err)

	// Verify disabled mapping returns default
	priority = mapper.GetPriority("tok_update_123")
	assert.Equal(t, PriorityStandard, priority)
}

// TestListMappings tests that soft-deleted mappings are excluded
func TestListMappings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add 3 mappings
	id1, err := mapper.AddMapping(ctx, "tok_active1*", PriorityCritical, MatchPrefix, "", "", "", "", "test-user")
	require.NoError(t, err)
	id2, err := mapper.AddMapping(ctx, "tok_active2*", PriorityUrgent, MatchPrefix, "", "", "", "", "test-user")
	require.NoError(t, err)
	id3, err := mapper.AddMapping(ctx, "tok_deleted*", PriorityHigh, MatchPrefix, "", "", "", "", "test-user")
	require.NoError(t, err)

	// Soft delete one mapping
	err = mapper.DeleteMapping(ctx, id3, "test-user")
	require.NoError(t, err)

	// List mappings (should only return active ones)
	mappings, err := mapper.ListMappings(ctx)
	require.NoError(t, err)

	// Should only have 2 active mappings
	assert.Len(t, mappings, 2)

	// Verify UUIDs are present
	ids := make(map[string]bool)
	for _, m := range mappings {
		ids[m.ID] = true
	}
	assert.True(t, ids[id1])
	assert.True(t, ids[id2])
	assert.False(t, ids[id3]) // Deleted mapping should not appear
}

// TestCacheTTL tests that cache reloads after TTL expires
func TestCacheTTL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Short TTL for testing (1 second)
	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 1*time.Second)
	require.NoError(t, err)

	ctx := context.Background()

	// Add initial mapping
	_, err = mapper.AddMapping(ctx, "tok_ttl1*", PriorityCritical, MatchPrefix, "", "", "", "", "test-user")
	require.NoError(t, err)

	// Verify mapping works
	priority := mapper.GetPriority("tok_ttl1_123")
	assert.Equal(t, PriorityCritical, priority)

	// Add second mapping directly to database (bypassing cache)
	_, err = db.ExecContext(ctx, `
		INSERT INTO api_key_priority_mappings (pattern, priority, match_type, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $4)
	`, "tok_ttl2*", int(PriorityUrgent), "prefix", "test-user")
	require.NoError(t, err)

	// Should still return default (cache not reloaded yet)
	priority = mapper.GetPriority("tok_ttl2_123")
	assert.Equal(t, PriorityStandard, priority)

	// Wait for TTL to expire
	time.Sleep(1500 * time.Millisecond)

	// Next GetPriority should trigger cache reload
	priority = mapper.GetPriority("tok_ttl2_123")
	assert.Equal(t, PriorityUrgent, priority, "Cache should have reloaded after TTL")
}

// TestManualReload tests explicit cache reload
func TestManualReload(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Long TTL (won't expire during test)
	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 10*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add mapping directly to database (bypassing cache)
	_, err = db.ExecContext(ctx, `
		INSERT INTO api_key_priority_mappings (pattern, priority, match_type, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $4)
	`, "tok_manual*", int(PriorityHigh), "prefix", "test-user")
	require.NoError(t, err)

	// Should return default (cache not reloaded)
	priority := mapper.GetPriority("tok_manual_123")
	assert.Equal(t, PriorityStandard, priority)

	// Manual reload
	err = mapper.Reload()
	require.NoError(t, err)

	// Should now return correct priority
	priority = mapper.GetPriority("tok_manual_123")
	assert.Equal(t, PriorityHigh, priority)
}

// TestMultiTenantMetadata tests that tenant fields are stored and retrieved
func TestMultiTenantMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add mapping with full tenant metadata
	id, err := mapper.AddMapping(ctx,
		"tok_tenant*", PriorityCritical, MatchPrefix,
		"dept-prod", "Production Team", "internal",
		"Production workloads", "admin-user",
	)
	require.NoError(t, err)

	// List and verify metadata
	mappings, err := mapper.ListMappings(ctx)
	require.NoError(t, err)

	var found *PriorityMappingModel
	for _, m := range mappings {
		if m.ID == id {
			found = m
			break
		}
	}

	require.NotNil(t, found, "Mapping not found")
	assert.Equal(t, "dept-prod", found.TenantID)
	assert.Equal(t, "Production Team", found.TenantName)
	assert.Equal(t, "internal", found.TenantType)
	assert.Equal(t, "Production workloads", found.Description)
	assert.Equal(t, "admin-user", found.CreatedBy)
}

// TestPriorityOrdering tests that mappings are evaluated in priority order
func TestPriorityOrdering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	mapper, err := NewAPIKeyMapper(db, PriorityStandard, true, 5*time.Minute)
	require.NoError(t, err)

	ctx := context.Background()

	// Add overlapping patterns with different priorities
	// More specific pattern should win if it has higher priority
	_, err = mapper.AddMapping(ctx, "tok_*", PriorityLow, MatchPrefix, "", "", "", "Catch-all", "test-user")
	require.NoError(t, err)
	_, err = mapper.AddMapping(ctx, "tok_prod*", PriorityCritical, MatchPrefix, "", "", "", "Production", "test-user")
	require.NoError(t, err)

	// Reload to pick up mappings
	err = mapper.Reload()
	require.NoError(t, err)

	// "tok_prod_123" matches both patterns, but "tok_prod*" has higher priority (P0)
	// Since Reload() orders by priority ASC, P0 comes first and should match first
	priority := mapper.GetPriority("tok_prod_123")
	assert.Equal(t, PriorityCritical, priority, "Higher priority pattern should match first")

	// "tok_dev_123" only matches "tok_*" (P8)
	priority = mapper.GetPriority("tok_dev_123")
	assert.Equal(t, PriorityLow, priority)
}

// setupTestDB creates an in-memory test database with schema
func setupTestDB(t *testing.T) *sql.DB {
	// Use PostgreSQL test database (requires TEST_DATABASE_URL env var)
	dbURL := getTestDatabaseURL()
	db, err := sql.Open("postgres", dbURL)
	require.NoError(t, err)

	// Create schema
	_, err = db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";

		DROP TABLE IF EXISTS api_key_priority_mappings;
		DROP TABLE IF EXISTS api_key_priority_config;

		CREATE TABLE api_key_priority_mappings (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			pattern TEXT NOT NULL UNIQUE,
			priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
			match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),
			tenant_id TEXT,
			tenant_name TEXT,
			tenant_type TEXT CHECK(tenant_type IN ('internal', 'external')),
			description TEXT,
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by TEXT,
			updated_by TEXT
		);

		CREATE INDEX idx_api_key_priority_mappings_pattern
			ON api_key_priority_mappings(pattern) WHERE deleted_at IS NULL;
		CREATE INDEX idx_api_key_priority_mappings_enabled
			ON api_key_priority_mappings(enabled) WHERE deleted_at IS NULL;
		CREATE INDEX idx_api_key_priority_mappings_priority
			ON api_key_priority_mappings(priority) WHERE deleted_at IS NULL;

		CREATE TABLE api_key_priority_config (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ,
			created_by TEXT,
			updated_by TEXT
		);

		INSERT INTO api_key_priority_config (key, value, description, created_by) VALUES
		('enabled', 'true', 'Enable API key priority mapping', 'system'),
		('default_priority', '7', 'Default priority', 'system'),
		('cache_ttl_sec', '300', 'Cache TTL in seconds', 'system');
	`)
	require.NoError(t, err)

	return db
}

// getTestDatabaseURL returns the test database URL from environment or default
func getTestDatabaseURL() string {
	// Default: local PostgreSQL test database
	// Override with TEST_DATABASE_URL environment variable
	dbURL := "postgres://postgres:postgres@localhost:5432/tokligence_test?sslmode=disable"
	// In CI/CD, set TEST_DATABASE_URL to point to test database
	return dbURL
}
