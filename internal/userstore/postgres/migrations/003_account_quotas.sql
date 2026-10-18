-- Migration 003: Account Quota Management (Phase 2)
-- Hierarchical quota system with borrowing support

-- Ensure UUID extension is available
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Account quotas table
CREATE TABLE IF NOT EXISTS account_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Hierarchy (account -> team -> environment)
    account_id TEXT NOT NULL,
    team_id TEXT,
    environment TEXT,  -- production, staging, dev, test, uat

    -- Quota configuration
    quota_type TEXT NOT NULL CHECK(quota_type IN ('hard', 'soft', 'reserved', 'burstable')),
    limit_dimension TEXT NOT NULL CHECK(limit_dimension IN ('tokens_per_month', 'tokens_per_day', 'tokens_per_hour', 'usd_per_month', 'tps', 'rpm')),
    limit_value BIGINT NOT NULL CHECK(limit_value > 0),

    -- Borrowing (for burstable quotas)
    allow_borrow BOOLEAN NOT NULL DEFAULT false,
    max_borrow_pct NUMERIC(5,2) DEFAULT 0.0 CHECK(max_borrow_pct >= 0.0 AND max_borrow_pct <= 1.0),

    -- Time window
    window_type TEXT NOT NULL DEFAULT 'monthly' CHECK(window_type IN ('hourly', 'daily', 'monthly', 'custom')),
    window_start TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    window_end TIMESTAMPTZ,

    -- Current usage (periodically synced from Redis)
    used_value BIGINT NOT NULL DEFAULT 0,
    last_sync_at TIMESTAMPTZ,

    -- Alerts
    alert_at_pct NUMERIC(5,2) DEFAULT 0.80 CHECK(alert_at_pct > 0.0 AND alert_at_pct <= 1.0),
    alert_webhook_url TEXT,
    alert_triggered BOOLEAN NOT NULL DEFAULT false,
    last_alert_at TIMESTAMPTZ,

    -- Metadata
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Audit fields (following Phase 1 pattern)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    created_by TEXT,
    updated_by TEXT,

    -- Constraints
    CONSTRAINT unique_active_quota UNIQUE (account_id, team_id, environment, quota_type, limit_dimension, window_start, deleted_at)
);

-- Indexes for active quotas (using partial indexes to exclude soft-deleted)
CREATE INDEX IF NOT EXISTS idx_account_quotas_account
    ON account_quotas(account_id, environment, window_start DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_account_quotas_team
    ON account_quotas(team_id, environment, window_start DESC)
    WHERE deleted_at IS NULL AND team_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_account_quotas_lookup
    ON account_quotas(account_id, quota_type, limit_dimension, window_start DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_account_quotas_alerts
    ON account_quotas(enabled, alert_triggered, window_end)
    WHERE deleted_at IS NULL AND enabled = true;

-- Quota usage history (for analytics)
CREATE TABLE IF NOT EXISTS quota_usage_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    quota_id UUID NOT NULL,
    account_id TEXT NOT NULL,

    -- Snapshot data
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    used_value BIGINT NOT NULL,
    limit_value BIGINT NOT NULL,
    utilization_pct NUMERIC(5,2) NOT NULL,

    -- Request metadata (optional)
    request_id TEXT,
    model TEXT,
    tokens_used BIGINT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quota_usage_history_quota
    ON quota_usage_history(quota_id, snapshot_at DESC);

CREATE INDEX IF NOT EXISTS idx_quota_usage_history_account
    ON quota_usage_history(account_id, snapshot_at DESC);

-- Quota borrowing transactions (for audit trail)
CREATE TABLE IF NOT EXISTS quota_borrowing_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    borrower_quota_id UUID NOT NULL,
    lender_quota_id UUID NOT NULL,

    borrowed_amount BIGINT NOT NULL,
    borrowed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    returned_at TIMESTAMPTZ,

    reason TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quota_borrowing_log_borrower
    ON quota_borrowing_log(borrower_quota_id, borrowed_at DESC);

CREATE INDEX IF NOT EXISTS idx_quota_borrowing_log_lender
    ON quota_borrowing_log(lender_quota_id, borrowed_at DESC);

-- Configuration for quota management
CREATE TABLE IF NOT EXISTS quota_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default configuration
INSERT INTO quota_config (key, value, description) VALUES
    ('redis_sync_interval_sec', '60', 'How often to sync quota usage from Redis to PostgreSQL (seconds)'),
    ('alert_cooldown_sec', '3600', 'Minimum time between alert notifications for same quota (seconds)'),
    ('enable_borrowing', 'true', 'Enable dynamic borrowing between quotas'),
    ('max_borrow_global_pct', '0.3', 'Maximum percentage any quota can borrow globally')
ON CONFLICT (key) DO NOTHING;
