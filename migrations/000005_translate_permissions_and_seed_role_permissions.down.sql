-- Reverts 000005: restore English permission catalog text and clear
-- roles.permissions back to the empty state left by 000002.

-- 1. Revert roles.permissions to empty.
UPDATE roles SET permissions = '[]'::jsonb WHERE id IN ('super_admin', 'tenant_manager', 'admin', 'operario', 'cliente_admin', 'cliente_operario');

-- 2. Revert permission catalog to English (values as seeded by 000002).
UPDATE permissions SET name = 'View Dashboard', description = 'Access to main dashboard and widgets' WHERE id = 'perm_dashboard';
UPDATE permissions SET name = 'View Alerts', description = 'Access to alerts and notification center' WHERE id = 'perm_alerts';
UPDATE permissions SET name = 'View Reports', description = 'Access to reports and analytics' WHERE id = 'perm_reports';
UPDATE permissions SET name = 'Manage Users', description = 'Create, edit and delete users' WHERE id = 'perm_users';
UPDATE permissions SET name = 'Manage Tenants', description = 'Access to tenant management' WHERE id = 'perm_tenants';
UPDATE permissions SET name = 'Manage Settings', description = 'Access to system settings' WHERE id = 'perm_settings';
UPDATE permissions SET name = 'View Maintenance', description = 'Access to maintenance module' WHERE id = 'perm_maintenance';
UPDATE permissions SET name = 'View Analytics', description = 'Access to analytics dashboards' WHERE id = 'perm_analytics';
UPDATE permissions SET name = 'Access All Tenants', description = 'Cross-tenant access (Super Admin only)' WHERE id = 'perm_all_tenants';
UPDATE permissions SET name = 'View Logs', description = 'Access to log viewer' WHERE id = 'perm_logs_view';
UPDATE permissions SET name = 'Export Logs', description = 'Export log data to file' WHERE id = 'perm_logs_export';
UPDATE permissions SET name = 'Manage Log Settings', description = 'Manage log retention and configuration' WHERE id = 'perm_logs_admin';
UPDATE permissions SET name = 'View Edge Devices', description = 'View edge device list and status' WHERE id = 'perm_edge_devices_view';
UPDATE permissions SET name = 'Manage Edge Devices', description = 'Create, edit, enable and disable edge devices' WHERE id = 'perm_edge_devices_manage';
UPDATE permissions SET name = 'Run Edge Checks', description = 'Execute status and health checks on edge devices' WHERE id = 'perm_edge_devices_check';
UPDATE permissions SET name = 'View Reports', description = 'Access to report history and download' WHERE id = 'perm_reports_view';
UPDATE permissions SET name = 'Manage Reports', description = 'Generate reports, manage schedules and retention settings' WHERE id = 'perm_reports_manage';
