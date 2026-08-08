-- Revierte 000009.
UPDATE roles
SET permissions = permissions - 'perm_logs_view',
    updated_at  = NOW()
WHERE id = 'admin';
