-- Migration 018: Drop role CHECK constraint from users table
-- The CHECK (role IN ('admin', 'user')) is a leftover from before the roles catalog
-- was introduced. Role existence is enforced via user_tenant_roles.role_id → roles.id (FK).
-- The users.role column is denormalized for display purposes only.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
