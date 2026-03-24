-- Create edge_devices table
CREATE TABLE edge_devices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    machine_id          VARCHAR(100) NOT NULL,
    edge_type           VARCHAR(50) NOT NULL CHECK (edge_type IN ('RASPBERRY_PLC')),
    raspberry_base_url  TEXT NOT NULL,
    plc_address         VARCHAR(255),
    status              VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
                            CHECK (status IN ('ACTIVE', 'DISABLED')),
    last_seen_at        TIMESTAMPTZ,
    last_health_check_at TIMESTAMPTZ,
    last_health_status  VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN'
                            CHECK (last_health_status IN ('OK', 'DEGRADED', 'ERROR', 'UNKNOWN')),
    last_health_summary TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_edge_devices_tenant_machine UNIQUE (tenant_id, machine_id)
);

-- Indexes for edge_devices
CREATE INDEX idx_edge_devices_tenant_id ON edge_devices (tenant_id);
CREATE INDEX idx_edge_devices_tenant_status ON edge_devices (tenant_id, status);

-- Auto-update trigger for updated_at
CREATE OR REPLACE FUNCTION update_edge_devices_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_edge_devices_updated_at
    BEFORE UPDATE ON edge_devices
    FOR EACH ROW EXECUTE FUNCTION update_edge_devices_updated_at();

-- Create device_events table
CREATE TABLE device_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id       UUID NOT NULL REFERENCES edge_devices(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL,
    check_type      VARCHAR(20) NOT NULL CHECK (check_type IN ('STATUS', 'HEALTH_CHECK')),
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    overall_status  VARCHAR(20) NOT NULL
                        CHECK (overall_status IN ('OK', 'DEGRADED', 'ERROR', 'UNKNOWN')),
    summary         TEXT,
    details         JSONB,
    user_id         UUID NOT NULL,
    user_email      VARCHAR(254) NOT NULL
);

-- Indexes for device_events
CREATE INDEX idx_device_events_device_id ON device_events (device_id);
CREATE INDEX idx_device_events_tenant_id ON device_events (tenant_id);
CREATE INDEX idx_device_events_device_checked_at ON device_events (device_id, checked_at DESC);
