-- Create dashboard_layouts table
CREATE TABLE dashboard_layouts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    widgets    JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

-- Index for tenant-scoped list queries (main access pattern)
CREATE INDEX idx_dashboard_layouts_tenant_id
    ON dashboard_layouts (tenant_id)
    WHERE deleted_at IS NULL;

-- Unique name per tenant (excludes soft-deleted rows)
CREATE UNIQUE INDEX idx_dashboard_layouts_tenant_name_active
    ON dashboard_layouts (tenant_id, name)
    WHERE deleted_at IS NULL;

-- Auto-update trigger for updated_at
CREATE OR REPLACE FUNCTION update_dashboard_layouts_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_dashboard_layouts_updated_at
    BEFORE UPDATE ON dashboard_layouts
    FOR EACH ROW EXECUTE FUNCTION update_dashboard_layouts_updated_at();
