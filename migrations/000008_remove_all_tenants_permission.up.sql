-- ============================================================================
-- Migration 000008: eliminar perm_all_tenants del catálogo
-- ============================================================================
-- El acceso cross-tenant pasa a ser una capability derivada del rol efectivo
-- (security.IsCrossTenantRole), no un permiso asignable del catálogo. Un
-- permiso que nadie puede asignar y que ningún código lee no es un permiso:
-- es ruido que además delata la existencia del rol super_admin a los admins
-- de plataforma. Ver docs/superpowers/specs/2026-08-04-platform-operator-rbac-design.md
--
-- La tabla permissions no tiene deleted_at, así que es un DELETE real.
-- ============================================================================

-- 1. Sacarlo de los arrays roles.permissions que lo contengan.
UPDATE roles
SET permissions = permissions - 'perm_all_tenants',
    updated_at  = NOW()
WHERE permissions @> '["perm_all_tenants"]'::jsonb;

-- 2. Borrar la fila del catálogo.
DELETE FROM permissions WHERE id = 'perm_all_tenants';
