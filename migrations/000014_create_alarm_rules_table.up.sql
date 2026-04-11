-- Migration: 000014_create_alarm_rules_table.up.sql
-- Feature: 008-alarm-rules

CREATE TABLE alarm_rules (
    id          UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID            NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT            NOT NULL,
    description TEXT,
    metric      TEXT            NOT NULL,
    operator    TEXT            NOT NULL CHECK (operator IN ('gt', 'lt', 'gte', 'lte', 'eq')),
    threshold   NUMERIC(15, 4)  NOT NULL,
    severity    TEXT            NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    enabled     BOOLEAN         NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alarm_rules_tenant_id      ON alarm_rules(tenant_id);
CREATE INDEX idx_alarm_rules_tenant_enabled ON alarm_rules(tenant_id, enabled);

CREATE OR REPLACE FUNCTION update_alarm_rules_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_alarm_rules_updated_at
    BEFORE UPDATE ON alarm_rules
    FOR EACH ROW EXECUTE FUNCTION update_alarm_rules_updated_at();
