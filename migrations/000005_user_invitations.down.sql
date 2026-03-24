-- Rollback migration 005: Remove user_invitations table

DROP TABLE IF EXISTS user_invitations CASCADE;
