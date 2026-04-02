-- Rollback: Drop users table and related objects
DROP TRIGGER IF EXISTS trigger_update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_users_updated_at();
DROP INDEX IF EXISTS idx_users_deleted_at;
DROP INDEX IF EXISTS idx_users_tenant_email;
DROP INDEX IF EXISTS idx_users_tenant_deleted;
DROP TABLE IF EXISTS users;
