package postgres_v2

// schema contains all v2 table definitions and migration SQL.
const schemaV2 = `
-- ==============================================================================
-- Tokligence User System v2 Schema
-- PostgreSQL version
-- ==============================================================================

-- Enable UUID extension if not exists
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ==============================================================================
-- Gateways
-- ==============================================================================
CREATE TABLE IF NOT EXISTS gateways (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alias            VARCHAR(255) NOT NULL,
    owner_user_id    UUID NOT NULL REFERENCES users(id),
    provider_enabled BOOLEAN NOT NULL DEFAULT false,
    consumer_enabled BOOLEAN NOT NULL DEFAULT false,
    metadata         JSONB DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_gateways_owner ON gateways(owner_user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_gateways_deleted_at ON gateways(deleted_at) WHERE deleted_at IS NOT NULL;

-- ==============================================================================
-- Gateway Memberships
-- ==============================================================================
CREATE TABLE IF NOT EXISTS gateway_memberships (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    gateway_id UUID NOT NULL REFERENCES gateways(id),
    role       VARCHAR(20) NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (user_id, gateway_id)
);

CREATE INDEX IF NOT EXISTS idx_gateway_memberships_gateway ON gateway_memberships(gateway_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_gateway_memberships_user ON gateway_memberships(user_id) WHERE deleted_at IS NULL;

-- ==============================================================================
-- Budgets (must be created before org_units and principals that reference it)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS budgets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id      UUID NOT NULL REFERENCES gateways(id),
    name            VARCHAR(255),
    max_budget      DECIMAL(15, 4),
    budget_duration VARCHAR(20) NOT NULL DEFAULT 'monthly' CHECK (budget_duration IN ('daily', 'weekly', 'monthly', 'total')),
    tpm_limit       BIGINT,
    rpm_limit       BIGINT,
    soft_limit      DECIMAL(5, 2),
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_budgets_gateway ON budgets(gateway_id) WHERE deleted_at IS NULL;

-- ==============================================================================
-- Organization Units (Flexible Hierarchy)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS org_units (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id     UUID NOT NULL REFERENCES gateways(id),
    parent_id      UUID REFERENCES org_units(id),
    path           TEXT NOT NULL,
    depth          INT NOT NULL DEFAULT 0,
    name           VARCHAR(255) NOT NULL,
    slug           VARCHAR(100) NOT NULL,
    unit_type      VARCHAR(20) NOT NULL DEFAULT 'team' CHECK (unit_type IN ('department', 'team', 'group', 'project')),
    budget_id      UUID REFERENCES budgets(id),
    allowed_models TEXT[] DEFAULT '{}',
    metadata       JSONB DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ,
    UNIQUE (gateway_id, path)
);

CREATE INDEX IF NOT EXISTS idx_org_units_gateway ON org_units(gateway_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_org_units_parent ON org_units(parent_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_org_units_path ON org_units(gateway_id, path) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_org_units_path_prefix ON org_units USING btree (gateway_id, path text_pattern_ops) WHERE deleted_at IS NULL;

-- ==============================================================================
-- Principals (Unified Consumer Identity)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS principals (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id       UUID NOT NULL REFERENCES gateways(id),
    principal_type   VARCHAR(20) NOT NULL CHECK (principal_type IN ('user', 'service', 'environment')),
    user_id          UUID REFERENCES users(id),
    service_name     VARCHAR(255),
    environment_name VARCHAR(50),
    display_name     VARCHAR(255) NOT NULL,
    budget_id        UUID REFERENCES budgets(id),
    allowed_models   TEXT[],
    metadata         JSONB DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,
    -- Ensure appropriate field is set based on type
    CONSTRAINT principals_type_check CHECK (
        (principal_type = 'user' AND user_id IS NOT NULL AND service_name IS NULL AND environment_name IS NULL) OR
        (principal_type = 'service' AND user_id IS NULL AND service_name IS NOT NULL AND environment_name IS NULL) OR
        (principal_type = 'environment' AND user_id IS NULL AND service_name IS NULL AND environment_name IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_principals_gateway_user ON principals(gateway_id, user_id) WHERE deleted_at IS NULL AND user_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_principals_gateway_service ON principals(gateway_id, service_name) WHERE deleted_at IS NULL AND service_name IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_principals_gateway_env ON principals(gateway_id, environment_name) WHERE deleted_at IS NULL AND environment_name IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_principals_gateway ON principals(gateway_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_principals_type ON principals(gateway_id, principal_type) WHERE deleted_at IS NULL;

-- ==============================================================================
-- Organization Memberships (Principal <-> OrgUnit)
-- ==============================================================================
CREATE TABLE IF NOT EXISTS org_memberships (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id UUID NOT NULL REFERENCES principals(id),
    org_unit_id  UUID NOT NULL REFERENCES org_units(id),
    role         VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member', 'viewer')),
    budget_id    UUID REFERENCES budgets(id),
    is_primary   BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,
    UNIQUE (principal_id, org_unit_id)
);

CREATE INDEX IF NOT EXISTS idx_org_memberships_principal ON org_memberships(principal_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_org_memberships_org_unit ON org_memberships(org_unit_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_org_memberships_primary ON org_memberships(principal_id) WHERE deleted_at IS NULL AND is_primary = true;

-- ==============================================================================
-- API Keys v2
-- ==============================================================================
CREATE TABLE IF NOT EXISTS api_keys_v2 (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway_id     UUID NOT NULL REFERENCES gateways(id),
    principal_id   UUID NOT NULL REFERENCES principals(id),
    org_unit_id    UUID REFERENCES org_units(id),
    key_hash       TEXT NOT NULL UNIQUE,
    key_prefix     VARCHAR(20) NOT NULL,
    key_name       VARCHAR(255) NOT NULL,
    budget_id      UUID REFERENCES budgets(id),
    allowed_models TEXT[] DEFAULT '{}',
    allowed_ips    TEXT[] DEFAULT '{}',
    scopes         TEXT[] DEFAULT '{}',
    expires_at     TIMESTAMPTZ,
    last_used_at   TIMESTAMPTZ,
    total_spend    DECIMAL(15, 4) NOT NULL DEFAULT 0,
    blocked        BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_v2_gateway ON api_keys_v2(gateway_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_v2_principal ON api_keys_v2(principal_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_v2_org_unit ON api_keys_v2(org_unit_id) WHERE deleted_at IS NULL AND org_unit_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_v2_prefix ON api_keys_v2(key_prefix) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_v2_hash ON api_keys_v2(key_hash) WHERE deleted_at IS NULL;

-- ==============================================================================
-- Functions for OrgUnit Path Management
-- ==============================================================================

-- Function to move an org unit to a new parent
CREATE OR REPLACE FUNCTION move_org_unit(p_unit_id UUID, p_new_parent_id UUID)
RETURNS VOID AS $$
DECLARE
    v_old_path TEXT;
    v_new_path TEXT;
    v_new_depth INT;
    v_gateway_id UUID;
    v_slug TEXT;
    v_parent_path TEXT;
BEGIN
    -- Get current unit info
    SELECT path, gateway_id, slug INTO v_old_path, v_gateway_id, v_slug
    FROM org_units
    WHERE id = p_unit_id AND deleted_at IS NULL;

    IF v_old_path IS NULL THEN
        RAISE EXCEPTION 'OrgUnit not found: %', p_unit_id;
    END IF;

    -- Calculate new path and depth
    IF p_new_parent_id IS NULL THEN
        -- Moving to root
        v_new_path := '/' || v_slug;
        v_new_depth := 0;
    ELSE
        -- Moving under a parent
        SELECT path, depth INTO v_parent_path, v_new_depth
        FROM org_units
        WHERE id = p_new_parent_id AND deleted_at IS NULL AND gateway_id = v_gateway_id;

        IF v_parent_path IS NULL THEN
            RAISE EXCEPTION 'Parent OrgUnit not found or different gateway: %', p_new_parent_id;
        END IF;

        v_new_path := v_parent_path || '/' || v_slug;
        v_new_depth := v_new_depth + 1;
    END IF;

    -- Check for circular reference
    IF p_new_parent_id IS NOT NULL AND EXISTS (
        SELECT 1 FROM org_units
        WHERE id = p_new_parent_id
        AND path LIKE v_old_path || '/%'
        AND deleted_at IS NULL
    ) THEN
        RAISE EXCEPTION 'Cannot move org unit under its own descendant';
    END IF;

    -- Update the unit itself
    UPDATE org_units
    SET parent_id = p_new_parent_id,
        path = v_new_path,
        depth = v_new_depth,
        updated_at = NOW()
    WHERE id = p_unit_id;

    -- Update all descendants
    UPDATE org_units
    SET path = v_new_path || substring(path FROM length(v_old_path) + 1),
        depth = v_new_depth + (depth - (SELECT depth FROM org_units WHERE id = p_unit_id)),
        updated_at = NOW()
    WHERE path LIKE v_old_path || '/%'
    AND gateway_id = v_gateway_id
    AND deleted_at IS NULL;
END;
$$ LANGUAGE plpgsql;

-- ==============================================================================
-- Trigger for updated_at
-- ==============================================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to all v2 tables
DO $$
DECLARE
    tbl TEXT;
BEGIN
    FOR tbl IN SELECT unnest(ARRAY['gateways', 'gateway_memberships', 'budgets', 'org_units', 'principals', 'org_memberships', 'api_keys_v2'])
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS update_%s_updated_at ON %s', tbl, tbl);
        EXECUTE format('CREATE TRIGGER update_%s_updated_at BEFORE UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION update_updated_at_column()', tbl, tbl);
    END LOOP;
END $$;
`

// migrationV2 checks and adds columns for upgrade from v1
const migrationV2 = `
-- Add uuid column to users if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'uuid') THEN
        ALTER TABLE users ADD COLUMN uuid UUID DEFAULT gen_random_uuid();
        UPDATE users SET uuid = gen_random_uuid() WHERE uuid IS NULL;
        ALTER TABLE users ALTER COLUMN uuid SET NOT NULL;
        CREATE UNIQUE INDEX IF NOT EXISTS idx_users_uuid ON users(uuid);
    END IF;
END $$;

-- Add deleted_at to users if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'deleted_at') THEN
        ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;
    END IF;
END $$;
`
