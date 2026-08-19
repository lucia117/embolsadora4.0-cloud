-- ============================================================================
-- Migration 000012: user_tenant_roles_tenant_id_fkey pasa a ON DELETE CASCADE.
-- ============================================================================
-- Era la única FK que referencia tenants(id) sin CASCADE desde la migración
-- 000001 (alarm_rules, dashboard_layouts, edge_devices, permissions, roles,
-- user_invitations y users ya la tenían). Con NO ACTION, borrar un tenant que
-- todavía tiene asignaciones de rol activas fallaba con una violación de FK en
-- vez de arrastrar esas filas — encontrado al intentar borrar el tenant de
-- prueba "Smoke Test B005" en producción (ver handoff 2026-08-19).
-- ============================================================================

ALTER TABLE public.user_tenant_roles
    DROP CONSTRAINT user_tenant_roles_tenant_id_fkey,
    ADD CONSTRAINT user_tenant_roles_tenant_id_fkey
        FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;
