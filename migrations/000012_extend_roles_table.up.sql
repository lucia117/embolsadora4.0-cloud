-- Migration 012: Extender tabla roles para soportar gestión dinámica
-- La tabla roles ya existe (migration 003) con 4 roles del sistema pre-cargados.
-- Esta migración agrega las columnas necesarias para CRUD de roles personalizados
-- y actualiza los roles del sistema con sus metadatos.

ALTER TABLE roles
  ADD COLUMN IF NOT EXISTS is_system_role BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS is_global      BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS tenant_id      UUID REFERENCES tenants(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS permissions    JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS deleted_at     TIMESTAMPTZ;

-- Marcar los 4 roles del sistema como system + global y poblar sus permisos
UPDATE roles SET
  is_system_role = TRUE,
  is_global      = TRUE,
  permissions    = '["users:read","users:write","invitations:write","machines:read","machines:write","tenants:read"]'
WHERE id = 'admin';

UPDATE roles SET
  is_system_role = TRUE,
  is_global      = TRUE,
  permissions    = '["machines:read","machines:write"]'
WHERE id = 'operario';

UPDATE roles SET
  is_system_role = TRUE,
  is_global      = TRUE,
  permissions    = '["users:read","invitations:write","machines:read"]'
WHERE id = 'cliente_admin';

UPDATE roles SET
  is_system_role = TRUE,
  is_global      = TRUE,
  permissions    = '["machines:read"]'
WHERE id = 'cliente_operario';

-- Índice para queries por tenant (roles custom activos)
CREATE INDEX IF NOT EXISTS idx_roles_tenant_active
  ON roles (tenant_id)
  WHERE deleted_at IS NULL;

-- Unicidad de nombre por tenant para roles custom (excluye roles del sistema y eliminados)
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_tenant_name_active
  ON roles (tenant_id, name)
  WHERE deleted_at IS NULL AND is_system_role = FALSE;
