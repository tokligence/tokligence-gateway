-- Phase 1: API Key to Priority Mapping
-- Migration: 002_api_key_priority.sql
-- Description: Create tables for API key to priority mapping with UUID and soft delete support

BEGIN;

-- Enable pgcrypto extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- Table: api_key_priority_mappings
-- Purpose: Map API key patterns to priority queues (P0-P9)
-- ============================================================================

CREATE TABLE IF NOT EXISTS api_key_priority_mappings (
    -- UUID Primary Key (NOT int/serial!)
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Pattern matching
    pattern TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL CHECK(priority >= 0 AND priority <= 9),
    match_type TEXT NOT NULL CHECK(match_type IN ('exact', 'prefix', 'suffix', 'contains', 'regex')),

    -- Multi-tenant metadata
    tenant_id TEXT,
    tenant_name TEXT,
    tenant_type TEXT CHECK(tenant_type IN ('internal', 'external')),
    description TEXT,

    -- Status
    enabled BOOLEAN NOT NULL DEFAULT TRUE,

    -- Standard audit fields (required for all tables)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete: NULL = active, NOT NULL = deleted
    created_by TEXT,
    updated_by TEXT
);

-- Indexes for fast lookup (with partial index for active records)
CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_pattern
    ON api_key_priority_mappings(pattern) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_enabled
    ON api_key_priority_mappings(enabled) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_tenant_id
    ON api_key_priority_mappings(tenant_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_priority
    ON api_key_priority_mappings(priority) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_api_key_priority_mappings_deleted_at
    ON api_key_priority_mappings(deleted_at);

-- ============================================================================
-- Table: api_key_priority_config
-- Purpose: Global configuration for API key priority feature
-- ============================================================================

CREATE TABLE IF NOT EXISTS api_key_priority_config (
    -- UUID Primary Key
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Config key-value
    key TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,

    -- Standard audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by TEXT,
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_api_key_priority_config_key
    ON api_key_priority_config(key) WHERE deleted_at IS NULL;

-- Insert default configuration
INSERT INTO api_key_priority_config (key, value, description, created_by) VALUES
('enabled', 'false', 'Enable/disable API key priority mapping (false for Personal Edition)', 'system'),
('default_priority', '7', 'Default priority for unmapped keys (P7 = Standard tier)', 'system'),
('cache_ttl_sec', '300', 'Cache TTL for mappings in seconds (5 minutes)', 'system')
ON CONFLICT (key) DO NOTHING;

COMMIT;

-- ============================================================================
-- Example data (commented out, for reference only)
-- Uncomment and modify for production use
-- ============================================================================

-- -- Internal departments (P0-P3) - Production in P0 queue
-- INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by) VALUES
-- ('tok_prod*', 0, 'prefix', 'dept-prod', 'Production Team', 'internal', 'Production workloads - Highest priority (P0 queue)', 'admin'),
-- ('tok_ml*', 1, 'prefix', 'dept-ml', 'ML Research Team', 'internal', 'ML research and training (P1 queue)', 'admin'),
-- ('tok_analytics*', 2, 'prefix', 'dept-analytics', 'Analytics Team', 'internal', 'Business analytics (P2 queue)', 'admin'),
-- ('tok_dev*', 3, 'prefix', 'dept-dev', 'Development Team', 'internal', 'Development and testing (P3 queue)', 'admin');
--
-- -- External customers (P5-P9)
-- INSERT INTO api_key_priority_mappings (pattern, priority, match_type, tenant_id, tenant_name, tenant_type, description, created_by) VALUES
-- ('tok_ext_ent*', 5, 'prefix', 'ext-enterprise', 'Enterprise Customers', 'external', 'Enterprise tier (P5 queue)', 'admin'),
-- ('tok_ext_prem*', 6, 'prefix', 'ext-premium', 'Premium Customers', 'external', 'Premium tier (P6 queue)', 'admin'),
-- ('tok_ext_std*', 7, 'prefix', 'ext-standard', 'Standard Customers', 'external', 'Standard tier (P7 queue)', 'admin'),
-- ('tok_ext_free*', 9, 'prefix', 'ext-free', 'Free Tier Users', 'external', 'Free tier (P9 queue)', 'admin');
