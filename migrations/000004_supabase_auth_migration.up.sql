-- Migration 004: Supabase Auth integration
-- Modifies users table for Supabase Auth; removes old auth tables

-- Add new columns to users table
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS supabase_user_id         TEXT UNIQUE,
  ADD COLUMN IF NOT EXISTS auth_provider            TEXT,
  ADD COLUMN IF NOT EXISTS email_verified_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_login_at            TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS password_change_required BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_supabase_user_id ON users(supabase_user_id);

-- Remove old password column
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;

-- Remove old auth tables (data is intentionally lost)
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS password_reset_tokens CASCADE;
