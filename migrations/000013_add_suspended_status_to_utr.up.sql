-- Migration 000013: Add 'suspended' status to user_tenant_roles
-- Extends the status CHECK constraint to support temporary suspension of users
ALTER TABLE user_tenant_roles
    DROP CONSTRAINT IF EXISTS user_tenant_roles_status_check,
    ADD CONSTRAINT user_tenant_roles_status_check
        CHECK (status IN ('active', 'pending', 'revoked', 'suspended'));
