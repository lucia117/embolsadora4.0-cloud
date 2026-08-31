-- ============================================================================
-- Migration 000013: permiso perm_edge_devices_create + reseed edge de roles
-- ============================================================================
-- Parte del trabajo edge-devices cross-tenant (ver
-- embolsadora-frontend/docs/superpowers/specs/2026-08-29-edge-devices-cross-tenant-design.md).
--
-- 1. Nuevo permiso fino perm_edge_devices_create: gatea SOLO el alta
--    (POST /edge-devices). perm_edge_devices_manage se queda con editar /
--    habilitar / deshabilitar. Objetivo: cliente_admin puede editar pero no
--    crear; solo los roles de plataforma dan de alta el primer device.
-- 2. Reseed de las claves perm_edge_devices_* de cada rol de sistema al estado
--    target de la spec §4a. Solo se tocan esas claves; el resto del array queda.
-- ============================================================================

-- 1. Catálogo.
INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_edge_devices_create', 'Registrar Dispositivos Edge', 'maintenance', 'Dar de alta nuevos dispositivos edge', TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

-- Ajuste de descripción de manage (ya no cubre el alta).
UPDATE permissions
SET description = 'Editar, habilitar y deshabilitar dispositivos edge'
WHERE id = 'perm_edge_devices_manage';

-- 2. Reseed por rol. Patrón: sacar todas las claves edge y agregar el set target.
--    super_admin / platform_admin / tenant_manager: view, check, manage, create
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_check","perm_edge_devices_manage","perm_edge_devices_create"]'::jsonb,
    updated_at = NOW()
WHERE id IN ('super_admin', 'platform_admin', 'tenant_manager');

--    admin: view, check, manage (gana check; NO create)
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_check","perm_edge_devices_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'admin';

--    cliente_admin: view, check, manage (gana check + manage; NO create)
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_check","perm_edge_devices_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'cliente_admin';

--    operario: view, check (sin cambios reales, pero explícito e idempotente)
--    cliente_operario: view, check (gana check)
UPDATE roles
SET permissions = (permissions - 'perm_edge_devices_view' - 'perm_edge_devices_check'
                               - 'perm_edge_devices_manage' - 'perm_edge_devices_create')
                  || '["perm_edge_devices_view","perm_edge_devices_check"]'::jsonb,
    updated_at = NOW()
WHERE id IN ('operario', 'cliente_operario');
