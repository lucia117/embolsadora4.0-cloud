UPDATE roles
SET permissions = permissions - 'perm_metrics_view',
    updated_at = NOW()
WHERE permissions @> '["perm_metrics_view"]'::jsonb;

DELETE FROM permissions WHERE id = 'perm_metrics_view';
