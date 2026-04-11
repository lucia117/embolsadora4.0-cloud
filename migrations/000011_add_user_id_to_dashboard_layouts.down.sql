DROP INDEX IF EXISTS idx_dashboard_layouts_tenant_user_name_active;
DROP INDEX IF EXISTS idx_dashboard_layouts_tenant_user;

CREATE UNIQUE INDEX idx_dashboard_layouts_tenant_name_active
    ON dashboard_layouts (tenant_id, name)
    WHERE deleted_at IS NULL;

ALTER TABLE dashboard_layouts DROP COLUMN IF EXISTS user_id;
