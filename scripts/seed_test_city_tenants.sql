-- ============================================================================
-- Seed: test city tenants (Córdoba, Mendoza, Rosario)
-- ============================================================================
-- NON-PRODUCTION SCRIPT.
-- Loads three fictional client tenants and their admin/operator users for
-- UAT and demos. Do NOT run against production: SC-006 of the
-- consolidate-migrations spec requires that prod contains only the MRG
-- tenant.
--
-- Step 1 — apply tenants (no Supabase dependency):
--   psql "$DATABASE_URL" -f scripts/seed_test_city_tenants.sql
--
-- Step 2 — create the 6 test users in Supabase Auth and capture their
-- UUIDs. Then assign them inside the tenants by re-running this script
-- with the user-section variables set, e.g.:
--   psql "$DATABASE_URL" \
--        -v cordoba_admin="'<uuid>'" -v cordoba_op="'<uuid>'" \
--        -v mendoza_admin="'<uuid>'" -v mendoza_op="'<uuid>'" \
--        -v rosario_admin="'<uuid>'" -v rosario_op="'<uuid>'" \
--        -v with_users=1 \
--        -f scripts/seed_test_city_tenants.sql
--
-- The user/role section is gated by `:with_users` so the bare run only
-- inserts tenants and stays safe to execute on environments without the
-- Supabase UUIDs at hand.
-- ============================================================================

\set ON_ERROR_STOP on

-- 1. Test tenants (idempotent)
INSERT INTO tenants (
    id, name, company_name, subdomain, description, is_active,
    primary_color, secondary_color, accent_color, text_color, background_color,
    logo_url, favicon_url,
    street, city, state, postal_code, country,
    created_at, updated_at
) VALUES
    (
        '2bff133c-1dba-4a21-a691-0ef7a2fd77ad',
        'Córdoba SA', 'Embolsadora Córdoba SA', 'cordoba',
        'Tenant de prueba — cliente ficticio en Córdoba.',
        true,
        '#15803d', '#166534', '#84cc16', '#0f172a', '#ffffff',
        '/tenants/cordoba/logo.png', '/tenants/cordoba/favicon.ico',
        'Av. Colón 1234', 'Córdoba', 'Córdoba', 'X5000', 'Argentina',
        NOW(), NOW()
    ),
    (
        'c4d4943a-dc0a-4d72-b26b-6c1e60cb8fcd',
        'Mendoza SA', 'Embolsadora Mendoza SA', 'mendoza',
        'Tenant de prueba — cliente ficticio en Mendoza.',
        true,
        '#b91c1c', '#7f1d1d', '#f59e0b', '#0f172a', '#ffffff',
        '/tenants/mendoza/logo.png', '/tenants/mendoza/favicon.ico',
        'Av. San Martín 567', 'Mendoza', 'Mendoza', 'M5500', 'Argentina',
        NOW(), NOW()
    ),
    (
        '75529c15-fcbb-449e-a7f9-8694d1d1e5cc',
        'Rosario SA', 'Embolsadora Rosario SA', 'rosario',
        'Tenant de prueba — cliente ficticio en Rosario.',
        true,
        '#0284c7', '#075985', '#38bdf8', '#0f172a', '#ffffff',
        '/tenants/rosario/logo.png', '/tenants/rosario/favicon.ico',
        'Bv. Oroño 890', 'Rosario', 'Santa Fe', 'S2000', 'Argentina',
        NOW(), NOW()
    )
ON CONFLICT (id) DO NOTHING;

-- 2. Optional user + role section (skipped unless :with_users is set)
\if :{?with_users}

\set cordoba_tid '''2bff133c-1dba-4a21-a691-0ef7a2fd77ad'''
\set mendoza_tid '''c4d4943a-dc0a-4d72-b26b-6c1e60cb8fcd'''
\set rosario_tid '''75529c15-fcbb-449e-a7f9-8694d1d1e5cc'''

INSERT INTO users (id, email, name, tenant_id, status, created_at, updated_at)
VALUES
    (:'cordoba_admin', 'federicoadegiovanni+cordobaadmin@gmail.com', 'Admin Córdoba',    :cordoba_tid, 'active', NOW(), NOW()),
    (:'cordoba_op',    'federicoadegiovanni+cordobaop@gmail.com',    'Operario Córdoba', :cordoba_tid, 'active', NOW(), NOW()),
    (:'mendoza_admin', 'federicoadegiovanni+mendozaadmin@gmail.com', 'Admin Mendoza',    :mendoza_tid, 'active', NOW(), NOW()),
    (:'mendoza_op',    'federicoadegiovanni+mendozaop@gmail.com',    'Operario Mendoza', :mendoza_tid, 'active', NOW(), NOW()),
    (:'rosario_admin', 'federicoadegiovanni+rosarioadmin@gmail.com', 'Admin Rosario',    :rosario_tid, 'active', NOW(), NOW()),
    (:'rosario_op',    'federicoadegiovanni+rosarioop@gmail.com',    'Operario Rosario', :rosario_tid, 'active', NOW(), NOW())
ON CONFLICT (tenant_id, email) DO UPDATE SET updated_at = NOW();

INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at, created_at, updated_at)
VALUES
    (gen_random_uuid(), :'cordoba_admin', :cordoba_tid, 'admin',    'active', NOW(), NOW(), NOW()),
    (gen_random_uuid(), :'cordoba_op',    :cordoba_tid, 'operario', 'active', NOW(), NOW(), NOW()),
    (gen_random_uuid(), :'mendoza_admin', :mendoza_tid, 'admin',    'active', NOW(), NOW(), NOW()),
    (gen_random_uuid(), :'mendoza_op',    :mendoza_tid, 'operario', 'active', NOW(), NOW(), NOW()),
    (gen_random_uuid(), :'rosario_admin', :rosario_tid, 'admin',    'active', NOW(), NOW(), NOW()),
    (gen_random_uuid(), :'rosario_op',    :rosario_tid, 'operario', 'active', NOW(), NOW(), NOW())
ON CONFLICT DO NOTHING;

SELECT t.subdomain, u.email, utr.role_id, utr.status
  FROM users u
  JOIN user_tenant_roles utr ON utr.user_id = u.id
  JOIN tenants t ON t.id = utr.tenant_id
 WHERE t.id IN (
     '2bff133c-1dba-4a21-a691-0ef7a2fd77ad',
     'c4d4943a-dc0a-4d72-b26b-6c1e60cb8fcd',
     '75529c15-fcbb-449e-a7f9-8694d1d1e5cc'
 )
 ORDER BY t.subdomain, utr.role_id;

\endif
