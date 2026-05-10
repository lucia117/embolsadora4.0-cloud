-- ============================================================================
-- Migration 000002: Essential seeds for production bootstrap
-- ============================================================================
-- Loads the data the application needs to authenticate and authorize on a
-- fresh deployment:
--   1. System permissions catalog (17 permissions, is_system_permission=TRUE)
--   2. Global roles (super_admin, tenant_manager) used by MRG operators
--   3. The MRG platform tenant (id 11b36b85-033d-4bb3-9e31-4c92161887c0)
--
-- The admin user(s) are NOT seeded here: their UUID comes from Supabase Auth
-- and the auth middleware auto-provisions the row in `users` on first login
-- (see internal/api/usecases/auth_usecase.go::ProvisionUser). After creating
-- the admin in Supabase, grant the super_admin role inside the MRG tenant via
-- `POST /api/v1/invitations` (see specs/014-consolidate-migrations/quickstart.md
-- Paso 5) or by inserting directly into user_tenant_roles with the resolved UUID.
--
-- All inserts are idempotent (ON CONFLICT DO NOTHING) so this migration is
-- safe to re-run after a partial rollback.
-- ============================================================================

-- 1. System permissions (matches legacy 000017 seed)
INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_dashboard',           'View Dashboard',          'dashboard',    'Access to main dashboard and widgets',                                TRUE, NULL),
    ('perm_alerts',              'View Alerts',             'alerts',       'Access to alerts and notification center',                            TRUE, NULL),
    ('perm_reports',             'View Reports',            'reports',      'Access to reports and analytics',                                     TRUE, NULL),
    ('perm_users',               'Manage Users',            'users',        'Create, edit and delete users',                                       TRUE, NULL),
    ('perm_tenants',             'Manage Tenants',          'tenants',      'Access to tenant management',                                         TRUE, NULL),
    ('perm_settings',            'Manage Settings',         'settings',     'Access to system settings',                                           TRUE, NULL),
    ('perm_maintenance',         'View Maintenance',        'maintenance',  'Access to maintenance module',                                        TRUE, NULL),
    ('perm_analytics',           'View Analytics',          'analytics',    'Access to analytics dashboards',                                      TRUE, NULL),
    ('perm_all_tenants',         'Access All Tenants',      'all-tenants',  'Cross-tenant access (Super Admin only)',                              TRUE, NULL),
    ('perm_logs_view',           'View Logs',               'logs',         'Access to log viewer',                                                TRUE, NULL),
    ('perm_logs_export',         'Export Logs',             'logs',         'Export log data to file',                                             TRUE, NULL),
    ('perm_logs_admin',          'Manage Log Settings',     'logs',         'Manage log retention and configuration',                              TRUE, NULL),
    ('perm_edge_devices_view',   'View Edge Devices',       'maintenance',  'View edge device list and status',                                    TRUE, NULL),
    ('perm_edge_devices_manage', 'Manage Edge Devices',     'maintenance',  'Create, edit, enable and disable edge devices',                       TRUE, NULL),
    ('perm_edge_devices_check',  'Run Edge Checks',         'maintenance',  'Execute status and health checks on edge devices',                    TRUE, NULL),
    ('perm_reports_view',        'View Reports',            'reports',      'Access to report history and download',                               TRUE, NULL),
    ('perm_reports_manage',      'Manage Reports',          'reports',      'Generate reports, manage schedules and retention settings',           TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

-- 2. Roles catalog
--    Globals (is_global=TRUE) are used by MRG operators to act cross-tenant.
--    Tenant-scoped roles (is_global=FALSE, tenant_id=NULL) are reusable
--    archetypes that any tenant can assign to its users.
INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions) VALUES
    ('super_admin',       'Super Admin',       'Full system access. Multi-tenant. Can create and manage any tenant.',                            TRUE, TRUE,  NULL, '[]'::jsonb),
    ('tenant_manager',    'Tenant Manager',    'Multi-tenant support role for MRG team. Can access any tenant with limited write permissions.',  TRUE, TRUE,  NULL, '[]'::jsonb),
    ('admin',             'Admin',             'Tenant administrator. Manages users and configuration within a single tenant.',                   TRUE, FALSE, NULL, '[]'::jsonb),
    ('operario',          'Operario',          'Day-to-day operator within a tenant.',                                                            TRUE, FALSE, NULL, '[]'::jsonb),
    ('cliente_admin',     'Cliente Admin',     'External client administrator with limited management capabilities.',                             TRUE, FALSE, NULL, '[]'::jsonb),
    ('cliente_operario',  'Cliente Operario',  'External client operator with read-only/operational access.',                                     TRUE, FALSE, NULL, '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- 3. MRG platform tenant (house tenant). UUID is fixed so it can be
-- referenced from scripts/seed_mrg_users.sql and operator runbooks.
INSERT INTO tenants (
    id,
    name,
    company_name,
    subdomain,
    description,
    is_active,
    primary_color,
    secondary_color,
    accent_color,
    text_color,
    background_color,
    logo_url,
    favicon_url,
    street,
    city,
    state,
    postal_code,
    country,
    created_at,
    updated_at
) VALUES (
    '11b36b85-033d-4bb3-9e31-4c92161887c0',
    'MRG SRL',
    'M.R.G. Equipamientos S.R.L.',
    'mrgsrl',
    'Tenant interno de MRG — administradores del sistema y soporte a clientes.',
    true,
    '#1d4ed8',
    '#0f3a99',
    '#f97316',
    '#0f172a',
    '#ffffff',
    '/tenants/mrgsrl/logo.png',
    '/tenants/mrgsrl/favicon.ico',
    'Av. Hipólito Yrigoyen 6470',
    'Lanús',
    'Buenos Aires',
    'B1824',
    'Argentina',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;
