-- Revierte 000008: reinserta perm_all_tenants y lo devuelve a los roles globales.

INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_all_tenants', 'Acceso a Todos los Tenants', 'all-tenants', 'Acceso cross-tenant (solo Super Admin)', TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

UPDATE roles
SET permissions = permissions || '["perm_all_tenants"]'::jsonb,
    updated_at  = NOW()
WHERE id IN ('super_admin', 'tenant_manager')
  AND NOT (permissions @> '["perm_all_tenants"]'::jsonb);
