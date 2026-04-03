-- Migration 011: Add user_id to dashboard_layouts
-- Dashboard layouts are now scoped per (tenant, user) because a user can belong
-- to multiple tenants and needs independent layouts in each one.
-- The max-3 limit now applies per (tenant, user) pair.

-- Safe to use NOT NULL without backfill: this migration runs after 000009 which creates
-- the table, and no production data exists at this point. In a fresh DB migrations run
-- sequentially, so the table is always empty when this column is added.
ALTER TABLE dashboard_layouts
  ADD COLUMN user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE;

-- Index for queries scoped by (tenant, user)
CREATE INDEX idx_dashboard_layouts_tenant_user
    ON dashboard_layouts (tenant_id, user_id)
    WHERE deleted_at IS NULL;

-- Unique name per (tenant, user) — replaces the previous per-tenant constraint
DROP INDEX IF EXISTS idx_dashboard_layouts_tenant_name_active;
CREATE UNIQUE INDEX idx_dashboard_layouts_tenant_user_name_active
    ON dashboard_layouts (tenant_id, user_id, name)
    WHERE deleted_at IS NULL;
