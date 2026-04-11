-- Migration: 000017_create_permissions_table.up.sql
-- Feature: 011-permissions-management

CREATE TABLE permissions (
    id                   TEXT            PRIMARY KEY,
    name                 TEXT            NOT NULL CHECK (char_length(name) >= 3),
    section              TEXT            NOT NULL CHECK (char_length(section) > 0),
    description          TEXT            NOT NULL CHECK (char_length(description) > 0),
    is_system_permission BOOLEAN         NOT NULL DEFAULT FALSE,
    tenant_id            UUID            REFERENCES tenants(id) ON DELETE CASCADE,
    created_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_system_perm_no_tenant
        CHECK (NOT (is_system_permission = TRUE AND tenant_id IS NOT NULL)),
    CONSTRAINT chk_custom_perm_has_tenant
        CHECK (NOT (is_system_permission = FALSE AND tenant_id IS NULL))
);

CREATE INDEX idx_permissions_tenant_id ON permissions(tenant_id)
    WHERE tenant_id IS NOT NULL;

CREATE INDEX idx_permissions_system ON permissions(is_system_permission)
    WHERE is_system_permission = TRUE;

CREATE OR REPLACE FUNCTION update_permissions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_permissions_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION update_permissions_updated_at();

-- Seed: 17 permisos de sistema (inmutables, globales)
INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_dashboard',           'View Dashboard',          'dashboard',    'Access to main dashboard and widgets',                                TRUE, NULL),
    ('perm_alerts',              'View Alerts',             'alerts',       'Access to alerts and notification center',                            TRUE, NULL),
    ('perm_reports',             'View Reports',            'reports',      'Access to reports and analytics',                                     TRUE, NULL),
    ('perm_users',               'Manage Users',            'users',        'Create, edit and delete users',                                       TRUE, NULL),
    ('perm_tenants',             'Manage Tenants',          'tenants',      'Access to tenant management',                                         TRUE, NULL),
    ('perm_settings',            'Manage Settings',         'settings',     'Access to system settings',                                           TRUE, NULL),
    ('perm_maintenance',         'View Maintenance',        'maintenance',  'Access to maintenance module',                                        TRUE, NULL),
    ('perm_analytics',           'View Analytics',          'analytics',    'Access to analytics dashboards',                                      TRUE, NULL),
    ('perm_all_tenants',         'Access All Tenants',      'all-tenants',  'Cross-tenant access (Super Admin only)',                               TRUE, NULL),
    ('perm_logs_view',           'View Logs',               'logs',         'Access to log viewer',                                                TRUE, NULL),
    ('perm_logs_export',         'Export Logs',             'logs',         'Export log data to file',                                             TRUE, NULL),
    ('perm_logs_admin',          'Manage Log Settings',     'logs',         'Manage log retention and configuration',                              TRUE, NULL),
    ('perm_edge_devices_view',   'View Edge Devices',       'maintenance',  'View edge device list and status',                                    TRUE, NULL),
    ('perm_edge_devices_manage', 'Manage Edge Devices',     'maintenance',  'Create, edit, enable and disable edge devices',                       TRUE, NULL),
    ('perm_edge_devices_check',  'Run Edge Checks',         'maintenance',  'Execute status and health checks on edge devices',                    TRUE, NULL),
    ('perm_reports_view',        'View Reports',            'reports',      'Access to report history and download',                               TRUE, NULL),
    ('perm_reports_manage',      'Manage Reports',          'reports',      'Generate reports, manage schedules and retention settings',           TRUE, NULL)
ON CONFLICT (id) DO NOTHING;
