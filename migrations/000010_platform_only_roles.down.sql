-- Revierte a la definición original de la migración 000004.

CREATE OR REPLACE FUNCTION tenant_can_use_role(p_tenant_id uuid, p_is_global boolean) RETURNS boolean AS $$
    SELECT (NOT p_is_global) OR EXISTS (
        SELECT 1 FROM tenants t WHERE t.id = p_tenant_id AND t.is_platform_tenant = TRUE
    );
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION enforce_platform_role_tenant() RETURNS trigger AS $$
DECLARE
    v_is_global boolean;
BEGIN
    IF NEW.role_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT is_global INTO v_is_global FROM roles WHERE id = NEW.role_id;
    IF v_is_global IS NOT TRUE THEN
        RETURN NEW;
    END IF;

    IF NOT tenant_can_use_role(NEW.tenant_id, TRUE) THEN
        RAISE EXCEPTION 'role "%" is reserved for the platform tenant and cannot be assigned in tenant %', NEW.role_id, NEW.tenant_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
