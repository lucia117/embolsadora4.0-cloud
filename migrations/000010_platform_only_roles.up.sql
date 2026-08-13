-- ============================================================================
-- Migration 000010: extiende tenant_can_use_role (migración 000004) para que
-- también trate admin/operario como platform-only, no solo is_global=true.
-- ============================================================================
-- admin/operario son is_global=FALSE (a diferencia de super_admin/tenant_manager),
-- así que tenant_can_use_role(tenant_id, is_global) los dejaba pasar en cualquier
-- tenant. Un admin de un tenant cliente podía ver y asignar el rol "admin" a
-- usuarios de su propio tenant — ese rol no da acceso cross-tenant (EffectiveRole
-- solo lo promueve a platform_admin dentro del tenant plataforma), pero sí
-- permisos de más (tenants:read, machines:write) que cliente_admin no tiene.
--
-- Se mantiene una sola función como fuente de verdad (mismo objetivo que la
-- migración 000004): en vez de agregar un segundo check en Go, esta función pasa
-- a recibir el role_id en vez de is_global, y resuelve is_global internamente.
-- ============================================================================

CREATE OR REPLACE FUNCTION tenant_can_use_role(p_tenant_id uuid, p_role_id text) RETURNS boolean AS $$
    SELECT
        (
            NOT COALESCE((SELECT is_global FROM roles WHERE id = p_role_id), FALSE)
            AND p_role_id NOT IN ('admin', 'operario')
        )
        OR EXISTS (
            SELECT 1 FROM tenants t WHERE t.id = p_tenant_id AND t.is_platform_tenant = TRUE
        );
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION enforce_platform_role_tenant() RETURNS trigger AS $$
BEGIN
    IF NEW.role_id IS NULL THEN
        RETURN NEW;
    END IF;

    IF NOT tenant_can_use_role(NEW.tenant_id, NEW.role_id) THEN
        RAISE EXCEPTION 'role "%" is reserved for the platform tenant and cannot be assigned in tenant %', NEW.role_id, NEW.tenant_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
