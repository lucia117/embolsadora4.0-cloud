-- Reverts 000002_seed_essentials. FK-safe order: drop role assignments
-- referencing seeded data first, then the tenant, roles, and permissions.

DELETE FROM user_tenant_roles
 WHERE tenant_id = '11b36b85-033d-4bb3-9e31-4c92161887c0'
   AND role_id   IN ('super_admin', 'tenant_manager', 'admin', 'operario', 'cliente_admin', 'cliente_operario');

DELETE FROM tenants WHERE id = '11b36b85-033d-4bb3-9e31-4c92161887c0';

DELETE FROM roles WHERE id IN ('super_admin', 'tenant_manager', 'admin', 'operario', 'cliente_admin', 'cliente_operario');

DELETE FROM permissions WHERE is_system_permission = TRUE AND tenant_id IS NULL;
