-- ============================================================================
-- Migration 000005: Translate permission catalog + seed role permissions
-- ============================================================================
-- 000002_seed_essentials seeded the 17 system permissions with English
-- name/description, and all 6 system roles with permissions: '[]'::jsonb.
-- This migration is a data-only fix, applied as UPDATEs (not edits to 000002)
-- because 000002 has already run against the live database and its INSERTs
-- use ON CONFLICT DO NOTHING, so re-running it with edited values would not
-- change already-seeded rows.
--
-- All statements are UPDATEs against existing rows by primary key, so this
-- migration is idempotent and safe to re-run.
-- ============================================================================

-- 1. Translate the 17 system permissions to Spanish (name + description).
UPDATE permissions SET name = 'Ver Panel', description = 'Acceso al panel principal y widgets' WHERE id = 'perm_dashboard';
UPDATE permissions SET name = 'Ver Alertas', description = 'Acceso al centro de alertas y notificaciones' WHERE id = 'perm_alerts';
UPDATE permissions SET name = 'Ver Reportes', description = 'Acceso a reportes y analítica' WHERE id = 'perm_reports';
UPDATE permissions SET name = 'Gestionar Usuarios', description = 'Crear, editar y eliminar usuarios' WHERE id = 'perm_users';
UPDATE permissions SET name = 'Gestionar Tenants', description = 'Acceso a la gestión de tenants' WHERE id = 'perm_tenants';
UPDATE permissions SET name = 'Gestionar Configuración', description = 'Acceso a la configuración del sistema' WHERE id = 'perm_settings';
UPDATE permissions SET name = 'Ver Mantenimiento', description = 'Acceso al módulo de mantenimiento' WHERE id = 'perm_maintenance';
UPDATE permissions SET name = 'Ver Analítica', description = 'Acceso a paneles de analítica avanzada' WHERE id = 'perm_analytics';
UPDATE permissions SET name = 'Acceso a Todos los Tenants', description = 'Acceso cross-tenant (solo Super Admin)' WHERE id = 'perm_all_tenants';
UPDATE permissions SET name = 'Ver Logs', description = 'Acceso al visor de logs' WHERE id = 'perm_logs_view';
UPDATE permissions SET name = 'Exportar Logs', description = 'Exportar datos de logs a archivo' WHERE id = 'perm_logs_export';
UPDATE permissions SET name = 'Gestionar Configuración de Logs', description = 'Gestionar retención y configuración de logs' WHERE id = 'perm_logs_admin';
UPDATE permissions SET name = 'Ver Dispositivos Edge', description = 'Ver el listado y estado de dispositivos edge' WHERE id = 'perm_edge_devices_view';
UPDATE permissions SET name = 'Gestionar Dispositivos Edge', description = 'Crear, editar, habilitar y deshabilitar dispositivos edge' WHERE id = 'perm_edge_devices_manage';
UPDATE permissions SET name = 'Ejecutar Chequeos Edge', description = 'Ejecutar chequeos de estado y salud en dispositivos edge' WHERE id = 'perm_edge_devices_check';
UPDATE permissions SET name = 'Ver Reportes', description = 'Acceso al historial de reportes y descargas' WHERE id = 'perm_reports_view';
UPDATE permissions SET name = 'Gestionar Reportes', description = 'Generar reportes, gestionar programaciones y retención' WHERE id = 'perm_reports_manage';

-- 2. Populate roles.permissions for the 6 system roles (perm_* catalog IDs).
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports","perm_users","perm_tenants","perm_settings","perm_maintenance","perm_analytics","perm_all_tenants","perm_logs_view","perm_logs_export","perm_logs_admin","perm_edge_devices_view","perm_edge_devices_manage","perm_edge_devices_check","perm_reports_view","perm_reports_manage"]'::jsonb WHERE id = 'super_admin';
UPDATE roles SET permissions = '["perm_all_tenants","perm_dashboard","perm_alerts","perm_reports","perm_reports_view","perm_users","perm_edge_devices_view","perm_edge_devices_check"]'::jsonb WHERE id = 'tenant_manager';
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports","perm_reports_view","perm_reports_manage","perm_users","perm_tenants","perm_settings","perm_maintenance","perm_analytics","perm_edge_devices_view","perm_edge_devices_manage"]'::jsonb WHERE id = 'admin';
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports_view","perm_edge_devices_view","perm_edge_devices_check"]'::jsonb WHERE id = 'operario';
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports_view","perm_users","perm_edge_devices_view"]'::jsonb WHERE id = 'cliente_admin';
UPDATE roles SET permissions = '["perm_dashboard","perm_edge_devices_view"]'::jsonb WHERE id = 'cliente_operario';
