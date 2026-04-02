-- Migration 006: Make name and tenant_id nullable on users
-- After Supabase auth integration (004), users are auto-provisioned on first login
-- without a name or tenant assignment. Both fields get populated later.

ALTER TABLE users
  ALTER COLUMN name DROP NOT NULL,
  ALTER COLUMN tenant_id DROP NOT NULL;
