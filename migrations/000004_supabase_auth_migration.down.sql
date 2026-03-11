-- Rollback migration 004: Restore old auth columns
-- NOTE: sessions and password_reset_tokens are NOT recreated (data intentionally dropped)

ALTER TABLE users
  DROP COLUMN IF EXISTS supabase_user_id,
  DROP COLUMN IF EXISTS auth_provider,
  DROP COLUMN IF EXISTS email_verified_at,
  DROP COLUMN IF EXISTS last_login_at,
  DROP COLUMN IF EXISTS password_change_required,
  ADD COLUMN IF NOT EXISTS password_hash TEXT;

DROP INDEX IF EXISTS idx_users_supabase_user_id;
