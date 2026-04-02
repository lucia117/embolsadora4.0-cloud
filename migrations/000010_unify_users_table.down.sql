-- Revert migration 010
ALTER TABLE users
  DROP COLUMN IF EXISTS supabase_user_id,
  DROP COLUMN IF EXISTS name,
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS auth_provider,
  DROP COLUMN IF EXISTS email_verified_at,
  DROP COLUMN IF EXISTS last_login_at,
  DROP COLUMN IF EXISTS password_change_required,
  ALTER COLUMN tenant_id  SET NOT NULL,
  ALTER COLUMN first_name SET NOT NULL,
  ALTER COLUMN last_name  SET NOT NULL;
