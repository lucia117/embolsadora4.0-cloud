-- ============================================================================
-- Migration 000011: permisos dinámicos — catálogo perm_users/tenants view+manage,
-- fila platform_admin, reseed de los 7 roles de sistema
-- ============================================================================
-- Cierra B-004: security.Can() va a leer roles.permissions en vez del mapa Go
-- hardcodeado (ver internal/security/rbac.go, cambia en un commit de Go aparte
-- de esta migración). Ver docs/superpowers/specs/2026-08-17-rbac-dynamic-permissions-design.md
-- en embolsadora-frontend (spec cross-repo) para el diseño completo y la tabla
-- de mapeo de la que sale este reseed.
--
-- 1. Nuevo catálogo perm_users_view/_manage y perm_tenants_view/_manage,
--    reemplazando los permisos gruesos perm_users/perm_tenants (sin
--    distinción de acción — la causa de que un admin de tenant cliente viera
--    la sección Tenants en el menú y recibiera 403 al intentar escribir).
-- 2. Fila nueva platform_admin en roles: hoy es 100% virtual (derivado en
--    runtime por security.EffectiveRole), sin ninguna fila propia.
-- 3. Reseed de roles.permissions para los 7 roles de sistema, replicando
--    exactamente lo que hoy define rolePermissions en rbac.go.
-- ============================================================================

-- 1. Catálogo: agregar los 4 permisos nuevos.
INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_users_view',     'Ver Usuarios',      'users',   'Ver el listado de usuarios y sus roles',          TRUE, NULL),
    ('perm_users_manage',   'Gestionar Usuarios','users',   'Crear, editar, eliminar usuarios e invitaciones', TRUE, NULL),
    ('perm_tenants_view',   'Ver Tenants',       'tenants', 'Ver el listado de tenants',                       TRUE, NULL),
    ('perm_tenants_manage', 'Gestionar Tenants', 'tenants', 'Crear, editar y eliminar tenants',                TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

-- 2. Fila platform_admin: mismo shape que los otros roles is_global=TRUE
--    (super_admin, tenant_manager) — solo visible dentro del tenant
--    plataforma vía tenant_can_use_role (migraciones 000004/000010).
INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions) VALUES
    ('platform_admin', 'Platform Admin', 'Admin de tenant cuya membresía pertenece al tenant plataforma MRG. Mismos permisos que Admin más gestión de tenants.', TRUE, TRUE, NULL, '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- 3. Reseed: sacar los permisos gruesos y agregar los finos, por rol.

--    super_admin: users read+write, tenants read+write -> view + manage de ambos.
UPDATE roles
SET permissions = (permissions - 'perm_users' - 'perm_tenants')
                   || '["perm_users_view","perm_users_manage","perm_tenants_view","perm_tenants_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'super_admin';

--    tenant_manager: users read-only, tenants read-only -> solo view de ambos.
UPDATE roles
SET permissions = (permissions - 'perm_users' - 'perm_tenants')
                   || '["perm_users_view","perm_tenants_view"]'::jsonb,
    updated_at = NOW()
WHERE id = 'tenant_manager';

--    admin: users read+write -> view+manage; tenants read-only -> view SIN manage.
--    Este es el fix real de B-004: hoy perm_tenants sin distinción le daba
--    acceso visual a Tenants aunque el backend rechace la escritura con 403.
UPDATE roles
SET permissions = (permissions - 'perm_users' - 'perm_tenants')
                   || '["perm_users_view","perm_users_manage","perm_tenants_view"]'::jsonb,
    updated_at = NOW()
WHERE id = 'admin';

--    platform_admin (fila nueva, arranca en '[]'): mismo set que admin, pero
--    con tenants manage completo — la única diferencia real vs. admin.
UPDATE roles
SET permissions = '["perm_dashboard","perm_alerts","perm_reports","perm_reports_view","perm_reports_manage","perm_users_view","perm_users_manage","perm_tenants_view","perm_tenants_manage","perm_settings","perm_maintenance","perm_analytics","perm_edge_devices_view","perm_edge_devices_manage","perm_logs_view"]'::jsonb,
    updated_at = NOW()
WHERE id = 'platform_admin';

--    cliente_admin: users read+write -> view+manage. Nunca tuvo tenants.
UPDATE roles
SET permissions = (permissions - 'perm_users')
                   || '["perm_users_view","perm_users_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'cliente_admin';

-- operario y cliente_operario no tenían perm_users ni perm_tenants -> sin cambios.

-- 4. Catálogo: borrar los permisos gruesos ya reemplazados. Sin FKs entrantes
--    (roles.permissions es JSONB, no relacional) — mismo patrón que 000008.
DELETE FROM permissions WHERE id IN ('perm_users', 'perm_tenants');
