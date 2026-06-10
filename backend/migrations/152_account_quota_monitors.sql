-- Account quota/balance monitors for upstream relay platforms.
-- Forward-only/idempotent. Monitors reuse account credentials server-side and
-- store only optional override keys encrypted by the application.

CREATE TABLE IF NOT EXISTS account_quota_monitors (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    provider VARCHAR(32) NOT NULL,
    endpoint TEXT NOT NULL DEFAULT '',
    api_key_override_encrypted TEXT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    interval_seconds INTEGER NOT NULL DEFAULT 3600,
    low_balance_threshold NUMERIC(18,6) NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'USD',
    last_balance NUMERIC(18,6) NULL,
    last_quota_total NUMERIC(18,6) NULL,
    last_quota_used NUMERIC(18,6) NULL,
    last_checked_at TIMESTAMPTZ NULL,
    last_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
    last_message TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    CONSTRAINT account_quota_monitors_provider_check CHECK (provider IN ('sub2api', 'newapi', 'custom')),
    CONSTRAINT account_quota_monitors_interval_check CHECK (interval_seconds BETWEEN 60 AND 86400),
    CONSTRAINT account_quota_monitors_status_check CHECK (last_status IN ('unknown', 'ok', 'low_balance', 'error'))
);

CREATE TABLE IF NOT EXISTS account_quota_monitor_history (
    id BIGSERIAL PRIMARY KEY,
    monitor_id BIGINT NOT NULL REFERENCES account_quota_monitors(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    balance NUMERIC(18,6) NULL,
    quota_total NUMERIC(18,6) NULL,
    quota_used NUMERIC(18,6) NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'USD',
    status VARCHAR(32) NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_quota_monitor_history_status_check CHECK (status IN ('ok', 'low_balance', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_account_quota_monitors_enabled
    ON account_quota_monitors(enabled)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_account_quota_monitors_account
    ON account_quota_monitors(account_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_account_quota_monitors_status
    ON account_quota_monitors(last_status)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_account_quota_monitor_history_monitor_checked
    ON account_quota_monitor_history(monitor_id, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_quota_monitor_history_account_checked
    ON account_quota_monitor_history(account_id, checked_at DESC);

COMMENT ON TABLE account_quota_monitors IS 'Upstream account quota/balance monitor configurations';
COMMENT ON TABLE account_quota_monitor_history IS 'Point-in-time upstream quota/balance check results';
