-- Remove demo data
DELETE FROM users WHERE email = 'user@example.com';
DELETE FROM tenants WHERE id = 'demo';
