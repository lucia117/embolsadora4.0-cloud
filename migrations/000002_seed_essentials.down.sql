-- Reverts 000002_seed_essentials. FK-safe order: drop role assignments
-- referencing seeded data first, then the tenant, roles, and permissions.

DELETE FROM user_tenant_roles
 WHERE tenant_id = '11b36b85-033d-4bb3-9e31-4c92161887c0'
   AND role_id   IN ('super_admin', 'tenant_manager', 'admin', 'operario', 'cliente_admin', 'cliente_operario');

DELETE FROM tenants WHERE id = '11b36b85-033d-4bb3-9e31-4c92161887c0';

DELETE FROM roles WHERE id IN ('super_admin', 'tenant_manager', 'admin', 'operario', 'cliente_admin', 'cliente_operario');

DELETE FROM permissions WHERE id IN (
    'perm_dashboard', 'perm_alerts', 'perm_reports', 'perm_users',
    'perm_tenants', 'perm_settings', 'perm_maintenance', 'perm_analytics',
    'perm_all_tenants', 'perm_logs_view', 'perm_logs_export', 'perm_logs_admin',
    'perm_edge_devices_view', 'perm_edge_devices_manage', 'perm_edge_devices_check',
    'perm_reports_view', 'perm_reports_manage'
);
