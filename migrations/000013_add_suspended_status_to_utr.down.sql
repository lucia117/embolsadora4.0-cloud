-- Migration 000013 down: Restore original status CHECK constraint
-- Remove 'suspended' from allowed values
ALTER TABLE user_tenant_roles
    DROP CONSTRAINT IF EXISTS user_tenant_roles_status_check,
    ADD CONSTRAINT user_tenant_roles_status_check
        CHECK (status IN ('active', 'pending', 'revoked'));
