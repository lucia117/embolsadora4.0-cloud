-- Drop indexes
DROP INDEX IF EXISTS idx_utr_status;
DROP INDEX IF EXISTS idx_utr_user_id;
DROP INDEX IF EXISTS idx_utr_tenant_id;
DROP INDEX IF EXISTS idx_utr_active_unique;

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS user_tenant_roles;
DROP TABLE IF EXISTS roles;
