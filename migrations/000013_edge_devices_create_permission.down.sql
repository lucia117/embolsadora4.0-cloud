-- Revierte 000013.

--    super_admin: view, manage, check
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_manage","perm_edge_devices_check"]'::jsonb,
    updated_at = NOW()
WHERE id = 'super_admin';

--    platform_admin: view, manage
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'platform_admin';

--    admin: view, manage
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'admin';

--    tenant_manager / operario: view, check
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_check"]'::jsonb,
    updated_at = NOW()
WHERE id IN ('tenant_manager', 'operario');

--    cliente_admin / cliente_operario: view
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view"]'::jsonb,
    updated_at = NOW()
WHERE id IN ('cliente_admin', 'cliente_operario');

UPDATE permissions
SET description = 'Crear, editar, habilitar y deshabilitar dispositivos edge'
WHERE id = 'perm_edge_devices_manage';

DELETE FROM permissions WHERE id = 'perm_edge_devices_create';
