-- Revertir migration 012: eliminar columnas y índices agregados a roles
DROP INDEX IF EXISTS idx_roles_tenant_name_active;
DROP INDEX IF EXISTS idx_roles_tenant_active;

ALTER TABLE roles
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS permissions,
  DROP COLUMN IF EXISTS tenant_id,
  DROP COLUMN IF EXISTS is_global,
  DROP COLUMN IF EXISTS is_system_role;
