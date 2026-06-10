-- Add group-level Secret Shield enablement flag.
-- Forward-only/idempotent: defaults existing and future groups to disabled.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS secret_shield_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.secret_shield_enabled IS 'Whether Secret Shield is enabled for this group';
