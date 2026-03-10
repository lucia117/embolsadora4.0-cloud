-- ============================================================================
-- SEED DATA: Users, Roles, Sessions, and Password Reset Tokens
-- ============================================================================
-- Purpose: Load test/demo data for development and testing
-- Usage: psql -U embolsadora_user -d embolsadora_dev -f seeds/seed_users_and_sessions.sql
-- ============================================================================

-- ============================================================================
-- 1. INSERT USERS
-- ============================================================================

-- User 1: Juan García (Admin at Demo Tenant)
INSERT INTO users (id, tenant_id, first_name, last_name, email, role, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  '550e8400-e29b-41d4-a716-446655440001',
  'Juan',
  'García',
  'juan.garcia@demo.com',
  'admin',
  NOW(),
  NOW()
);

-- User 2: María Rodríguez (User at Demo Tenant)
INSERT INTO users (id, tenant_id, first_name, last_name, email, role, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  '550e8400-e29b-41d4-a716-446655440001',
  'María',
  'Rodríguez',
  'maria.rodriguez@demo.com',
  'user',
  NOW(),
  NOW()
);

-- User 3: Carlos López (Admin at Tech Solutions)
INSERT INTO users (id, tenant_id, first_name, last_name, email, role, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  '550e8400-e29b-41d4-a716-446655440002',
  'Carlos',
  'López',
  'carlos.lopez@techsolutions.com',
  'admin',
  NOW(),
  NOW()
);

-- ============================================================================
-- 2. INSERT USER TENANT ROLES
-- ============================================================================

-- Role Assignment 1: Juan as admin in Demo Tenant
INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at, created_at, updated_at)
SELECT
  gen_random_uuid(),
  id,
  '550e8400-e29b-41d4-a716-446655440001',
  'admin',
  'active',
  NOW(),
  NOW(),
  NOW()
FROM users WHERE email = 'juan.garcia@demo.com';

-- Role Assignment 2: María as operario in Demo Tenant
INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at, created_at, updated_at)
SELECT
  gen_random_uuid(),
  id,
  '550e8400-e29b-41d4-a716-446655440001',
  'operario',
  'active',
  NOW(),
  NOW(),
  NOW()
FROM users WHERE email = 'maria.rodriguez@demo.com';

-- Role Assignment 3: Carlos as admin in Tech Solutions
INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at, created_at, updated_at)
SELECT
  gen_random_uuid(),
  id,
  '550e8400-e29b-41d4-a716-446655440002',
  'admin',
  'active',
  NOW(),
  NOW(),
  NOW()
FROM users WHERE email = 'carlos.lopez@techsolutions.com';

-- ============================================================================
-- 3. INSERT SESSIONS
-- ============================================================================

-- Session 1: Juan García (valid 24 hours)
INSERT INTO sessions (token, user_id, expires_at, created_at)
SELECT
  'token_' || gen_random_uuid()::text,
  id,
  NOW() + INTERVAL '24 hours',
  NOW()
FROM users WHERE email = 'juan.garcia@demo.com';

-- Session 2: María Rodríguez (valid 24 hours)
INSERT INTO sessions (token, user_id, expires_at, created_at)
SELECT
  'token_' || gen_random_uuid()::text,
  id,
  NOW() + INTERVAL '24 hours',
  NOW()
FROM users WHERE email = 'maria.rodriguez@demo.com';

-- Session 3: Carlos López (valid 24 hours)
INSERT INTO sessions (token, user_id, expires_at, created_at)
SELECT
  'token_' || gen_random_uuid()::text,
  id,
  NOW() + INTERVAL '24 hours',
  NOW()
FROM users WHERE email = 'carlos.lopez@techsolutions.com';

-- ============================================================================
-- 4. INSERT PASSWORD RESET TOKENS
-- ============================================================================

-- Reset Token 1: Juan García (valid 1 hour)
INSERT INTO password_reset_tokens (id, user_id, token, expires_at, created_at)
SELECT
  gen_random_uuid(),
  id,
  'reset_' || md5(random()::text),
  NOW() + INTERVAL '1 hour',
  NOW()
FROM users WHERE email = 'juan.garcia@demo.com';

-- Reset Token 2: María Rodríguez (valid 1 hour)
INSERT INTO password_reset_tokens (id, user_id, token, expires_at, created_at)
SELECT
  gen_random_uuid(),
  id,
  'reset_' || md5(random()::text),
  NOW() + INTERVAL '1 hour',
  NOW()
FROM users WHERE email = 'maria.rodriguez@demo.com';

-- Reset Token 3: Carlos López (valid 1 hour)
INSERT INTO password_reset_tokens (id, user_id, token, expires_at, created_at)
SELECT
  gen_random_uuid(),
  id,
  'reset_' || md5(random()::text),
  NOW() + INTERVAL '1 hour',
  NOW()
FROM users WHERE email = 'carlos.lopez@techsolutions.com';

-- ============================================================================
-- VERIFICATION QUERY
-- ============================================================================
-- Run this to verify all data was inserted:
--
-- SELECT 'users' as tabla, COUNT(*) as registros FROM users
-- UNION ALL
-- SELECT 'user_tenant_roles', COUNT(*) FROM user_tenant_roles
-- UNION ALL
-- SELECT 'sessions', COUNT(*) FROM sessions
-- UNION ALL
-- SELECT 'password_reset_tokens', COUNT(*) FROM password_reset_tokens;
-- ============================================================================
