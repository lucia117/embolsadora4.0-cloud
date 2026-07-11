-- ============================================================================
-- Migration 000003: Flag the MRG platform (house) tenant explicitly
-- ============================================================================
-- Global roles (super_admin, tenant_manager — roles.is_global = TRUE) are
-- meant to be used only by MRG staff operating inside the MRG platform
-- tenant, not by regular client tenants. Until now there was no column to
-- identify which tenant that is; it was only a documented convention (the
-- fixed UUID 11b36b85-033d-4bb3-9e31-4c92161887c0). This adds that as real,
-- queryable state so the roles catalog and role-assignment endpoints can
-- enforce it instead of trusting every caller to know the convention.
-- ============================================================================

ALTER TABLE tenants ADD COLUMN is_platform_tenant boolean NOT NULL DEFAULT false;

UPDATE tenants SET is_platform_tenant = true
WHERE id = '11b36b85-033d-4bb3-9e31-4c92161887c0';
