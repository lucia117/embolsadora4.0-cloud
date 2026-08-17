-- Revierte 000011: vuelve a perm_users/perm_tenants sin distinción de acción,
-- borra platform_admin y el catálogo _view/_manage nuevo.
--
-- Dos limitaciones conocidas de este down, aceptadas porque 000011 todavía no
-- shippeó a producción y ninguno de los dos escenarios puede existir hoy:
-- 1. El DELETE de platform_admin (línea ~18) va a fallar por violación de FK
--    si alguna vez se asignó ese rol directamente en user_tenant_roles.
-- 2. La restauración de abajo (CASE) solo mira presencia de "_view" — un
--    hipotético rol custom con "_manage" pero sin "_view" perdería "_manage"
--    en el rollback sin recuperar el permiso grueso equivalente.

INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_users',   'Gestionar Usuarios', 'users',   'Crear, editar y eliminar usuarios', TRUE, NULL),
    ('perm_tenants', 'Gestionar Tenants',  'tenants', 'Acceso a la gestión de tenants',    TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

-- Si el rol tenía _view (siempre presente cuando había _manage, ver up.sql),
-- se le restaura el permiso grueso equivalente.
UPDATE roles
SET permissions = (permissions - 'perm_users_view' - 'perm_users_manage' - 'perm_tenants_view' - 'perm_tenants_manage')
                   || (CASE WHEN permissions @> '["perm_users_view"]'::jsonb THEN '["perm_users"]'::jsonb ELSE '[]'::jsonb END)
                   || (CASE WHEN permissions @> '["perm_tenants_view"]'::jsonb THEN '["perm_tenants"]'::jsonb ELSE '[]'::jsonb END),
    updated_at = NOW()
WHERE id != 'platform_admin';

DELETE FROM roles WHERE id = 'platform_admin';

DELETE FROM permissions WHERE id IN ('perm_users_view', 'perm_users_manage', 'perm_tenants_view', 'perm_tenants_manage');
