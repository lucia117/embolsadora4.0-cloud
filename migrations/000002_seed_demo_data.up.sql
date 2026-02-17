-- Insert demo tenant with complete schema (matches contract)
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
    '550e8400-e29b-41d4-a716-446655440001',
    'Demo Tenant',
    'Demo Company',
    'demo',
    'Demo tenant for testing purposes',
    true,
    '#3b82f6',
    '#6366f1',
    '#8b5cf6',
    '#1f2937',
    '#ffffff',
    '/logos/demo-logo.png',
    '/favicon.ico',
    '123 Main St',
    'Buenos Aires',
    'Buenos Aires',
    'C1001',
    'Argentina',
    '2025-10-17 00:00:00+00:00',
    '2025-10-17 00:00:00+00:00'
)
ON CONFLICT (id) DO NOTHING;

-- Insert additional demo tenant
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
    '550e8400-e29b-41d4-a716-446655440002',
    'Tech Solutions',
    'Tech Solutions Inc.',
    'techsolutions',
    'Technology solutions company',
    true,
    '#10b981',
    '#059669',
    '#047857',
    '#111827',
    '#f9fafb',
    '/logos/tech-logo.png',
    '/favicon.ico',
    '456 Innovation Ave',
    'San Francisco',
    'California',
    '94105',
    'USA',
    '2025-10-17 01:00:00+00:00',
    '2025-10-17 01:00:00+00:00'
)
ON CONFLICT (id) DO NOTHING;

-- Insert demo users for each tenant
-- Email: admin@demo.com, Password: password
INSERT INTO users (
    id, 
    email, 
    name, 
    password_hash, 
    image, 
    tenant_id, 
    status, 
    created_at, 
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000001',
    'admin@demo.com',
    'Admin User',
    '$2a$10$Wyy3EkQva6DjiC73sH/HLOjzDS2OnXgXcVecykwMn6b4GCl7lugLi',
    'https://example.com/avatar.jpg',
    '550e8400-e29b-41d4-a716-446655440001',
    'active',
    '2025-10-17 00:00:00+00:00',
    '2025-10-17 00:00:00+00:00'
)
ON CONFLICT (email) DO NOTHING;

-- Email: admin@techsolutions.com, Password: password
INSERT INTO users (
    id, 
    email, 
    name, 
    password_hash, 
    image, 
    tenant_id, 
    status, 
    created_at, 
    updated_at
) VALUES (
    '00000000-0000-0000-0000-000000000002',
    'admin@techsolutions.com',
    'Tech Admin',
    '$2a$10$Wyy3EkQva6DjiC73sH/HLOjzDS2OnXgXcVecykwMn6b4GCl7lugLi',
    'https://example.com/tech-avatar.jpg',
    '550e8400-e29b-41d4-a716-446655440002',
    'active',
    '2025-10-17 01:00:00+00:00',
    '2025-10-17 01:00:00+00:00'
)
ON CONFLICT (email) DO NOTHING;
