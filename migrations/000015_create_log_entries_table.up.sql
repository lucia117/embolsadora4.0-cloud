-- Migration: 000004_create_log_entries_table
-- Creates log_entries and log_retention_policies tables

-- Reuse existing trigger function or create if not exists
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Table: log_entries (immutable event records)
CREATE TABLE log_entries (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    severity    VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical', 'error')),
    event_type  VARCHAR(50) NOT NULL CHECK (event_type IN (
                    'alarm_triggered', 'alarm_resolved',
                    'device_connected', 'device_disconnected', 'device_state_changed',
                    'user_action', 'system'
                )),
    source_id   UUID,
    machine_id  UUID,
    message     TEXT        NOT NULL,
    metadata    JSONB       NOT NULL DEFAULT '{}'
);

-- Table: log_retention_policies (one per tenant)
CREATE TABLE log_retention_policies (
    tenant_id       UUID        PRIMARY KEY,
    retention_days  INT         NOT NULL DEFAULT 90 CHECK (retention_days > 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_purge_at   TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '1 day'
);

CREATE TRIGGER update_log_retention_policies_updated_at
    BEFORE UPDATE ON log_retention_policies
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();

-- Index: keyset pagination (tenant + created_at DESC + id DESC)
CREATE INDEX idx_log_entries_tenant_cursor
    ON log_entries(tenant_id, created_at DESC, id DESC);

-- Index: filter by machine_id
CREATE INDEX idx_log_entries_machine
    ON log_entries(tenant_id, machine_id)
    WHERE machine_id IS NOT NULL;

-- Index: filter by severity
CREATE INDEX idx_log_entries_severity
    ON log_entries(tenant_id, severity, created_at DESC);

-- Index: filter by event_type
CREATE INDEX idx_log_entries_event_type
    ON log_entries(tenant_id, event_type, created_at DESC);

-- Index: full-text search on message
CREATE INDEX idx_log_entries_message_fts
    ON log_entries USING gin(to_tsvector('spanish', message));
