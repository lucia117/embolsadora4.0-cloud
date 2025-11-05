-- Insert demo tenant
INSERT INTO tenants (id, name, company_name, subdomain, created_at, updated_at)
VALUES ('demo', 'demo', 'Demo Company', 'demo', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert demo user
-- Email: user@example.com
-- Password: password (bcrypt hash with cost 10)
INSERT INTO users (id, email, name, password_hash, image, tenant_id, status, created_at, updated_at)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'user@example.com',
    'John Doe',
    '$2a$10$Wyy3EkQva6DjiC73sH/HLOjzDS2OnXgXcVecykwMn6b4GCl7lugLi',
    'https://example.com/avatar.jpg',
    'demo',
    'active',
    NOW(),
    NOW()
)
ON CONFLICT (email) DO NOTHING;
