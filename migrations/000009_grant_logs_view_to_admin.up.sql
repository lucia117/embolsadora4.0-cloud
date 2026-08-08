-- ============================================================================
-- Migration 000009: dar perm_logs_view al rol admin
-- ============================================================================
-- 000005 sembró roles.permissions sin perm_logs_view para admin, así que el
-- Centro de Logs quedaba invisible para cualquier administrador de tenant.
-- No se detectó antes porque /me devolvía permisos en vocabulario
-- resource:action y el sidebar nunca matcheaba ningún perm_*.
--
-- Solo lectura: perm_logs_export y perm_logs_admin quedan reservados.
-- ============================================================================

UPDATE roles
SET permissions = permissions || '["perm_logs_view"]'::jsonb,
    updated_at  = NOW()
WHERE id = 'admin'
  AND NOT (permissions @> '["perm_logs_view"]'::jsonb);
