-- ============================================================================
-- Migration 000004: Single source of truth for the platform-role tenant rule
-- ============================================================================
-- "A role with is_global = TRUE (super_admin, tenant_manager) may only be
-- used within the MRG platform tenant (tenants.is_platform_tenant = TRUE)"
-- was independently implemented in two places: roles.List's WHERE clause
-- (internal/repo/pg/roles/repository.go) and checkRoleAllowedForTenant, a
-- Go-side pre-check (internal/repo/pg/user_roles/repository.go). Two
-- independent implementations of the same rule can silently drift if one is
-- updated and the other isn't.
--
-- tenant_can_use_role() below is the single canonical definition. Both
-- repositories and the enforcement trigger call it, so there is exactly one
-- place to change if this rule ever changes.
--
-- The trigger itself closes a separate gap: checkRoleAllowedForTenant only
-- runs on the one Go code path that goes through that repository — a direct
-- SQL insert/update (a data-fix script, a migration, an admin psql session,
-- a future second repository implementation) would bypass it entirely and
-- silently reinstate the privilege-escalation bug that check exists to
-- prevent. The trigger makes the DB itself refuse to store an invalid
-- combination no matter what wrote it.
-- ============================================================================

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

CREATE TRIGGER trg_enforce_platform_role_tenant
    BEFORE INSERT OR UPDATE OF role_id, tenant_id ON user_tenant_roles
    FOR EACH ROW
    EXECUTE FUNCTION enforce_platform_role_tenant();
