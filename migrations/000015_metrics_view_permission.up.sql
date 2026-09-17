-- ============================================================================
-- Migration 000015: permiso perm_metrics_view (Dashboard Metrics Query API)
-- ============================================================================
-- Ver docs/superpowers/specs/2026-09-07-dashboard-metrics-query-design.md.
-- Se seedea dinamicamente a cualquier rol que hoy tenga perm_dashboard o
-- perm_analytics (containment JSONB), en vez de enumerar roles a mano: es el
-- mismo criterio que la spec declara ("roles que hoy tienen
-- perm_dashboard/perm_analytics") y no se desincroniza si un rol nuevo gana
-- alguno de esos dos permisos entre esta migracion y el reseed manual.
-- ============================================================================

INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_metrics_view', 'Ver Métricas de Dashboard', 'dashboard', 'Consultar métricas agregadas/crudas de measurements para los widgets de dashboard', TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

UPDATE roles
SET permissions = permissions || '["perm_metrics_view"]'::jsonb,
    updated_at = NOW()
WHERE (permissions @> '["perm_dashboard"]'::jsonb OR permissions @> '["perm_analytics"]'::jsonb)
  AND NOT (permissions @> '["perm_metrics_view"]'::jsonb);
