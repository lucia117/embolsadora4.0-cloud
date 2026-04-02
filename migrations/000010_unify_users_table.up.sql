-- Migration 010: Unify users table
-- The users table was created by the tenant user management feature (first_name, last_name, role).
-- The auth middleware requires supabase_user_id, status, and related columns for provisioning.
-- This migration adds the missing auth columns and relaxes NOT NULL constraints that
-- prevent auto-provisioning (tenant_id, first_name, last_name are populated after first login).

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS supabase_user_id         TEXT UNIQUE,
  ADD COLUMN IF NOT EXISTS name                     TEXT,
  ADD COLUMN IF NOT EXISTS status                   VARCHAR(20) NOT NULL DEFAULT 'invited',
  ADD COLUMN IF NOT EXISTS auth_provider            TEXT,
  ADD COLUMN IF NOT EXISTS email_verified_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_login_at            TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS password_change_required BOOLEAN NOT NULL DEFAULT FALSE,
  ALTER COLUMN tenant_id  DROP NOT NULL,
  ALTER COLUMN first_name DROP NOT NULL,
  ALTER COLUMN last_name  DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_supabase_user_id ON users(supabase_user_id);
