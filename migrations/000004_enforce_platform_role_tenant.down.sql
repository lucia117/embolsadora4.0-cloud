DROP TRIGGER IF EXISTS trg_enforce_platform_role_tenant ON user_tenant_roles;
DROP FUNCTION IF EXISTS enforce_platform_role_tenant();
DROP FUNCTION IF EXISTS tenant_can_use_role(uuid, boolean);
