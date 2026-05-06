
-- Tenant reservado para M.R.G. Equipamientos (plataforma).
-- Los usuarios con rol super_admin deben pertenecer a este tenant.
-- UUID fijo para que pueda referenciarse en seeds y documentación.
INSERT INTO tenants (
    id,
    name,
    company_name,
    subdomain,
    description,
    is_active,
    created_at,
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000000',
    'MRG Plataforma',
    'M.R.G. Equipamientos S.R.L.',
    'mrg-internal',
    'Tenant reservado para administradores de plataforma MRG. No eliminar.',
    true,
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

-- Rol de plataforma: puede crear, editar y eliminar tenants.
-- Solo debe asignarse a empleados de MRG en el tenant mrg-internal.
INSERT INTO roles (id, name, description) VALUES
    ('super_admin', 'Super Admin', 'Administrador de plataforma MRG. Gestión completa de tenants y usuarios.')
ON CONFLICT (id) DO NOTHING;
