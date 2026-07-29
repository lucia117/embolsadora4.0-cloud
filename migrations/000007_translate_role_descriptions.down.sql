-- ============================================================================
-- Down migration 000007: restore English system role descriptions
-- ============================================================================
-- Reverts the descriptions to the original English text seeded by 000002.
-- ============================================================================

UPDATE roles SET description = 'Full system access. Multi-tenant. Can create and manage any tenant.' WHERE id = 'super_admin';
UPDATE roles SET description = 'Multi-tenant support role for MRG team. Can access any tenant with limited write permissions.' WHERE id = 'tenant_manager';
UPDATE roles SET description = 'Tenant administrator. Manages users and configuration within a single tenant.' WHERE id = 'admin';
UPDATE roles SET description = 'Day-to-day operator within a tenant.' WHERE id = 'operario';
UPDATE roles SET description = 'External client administrator with limited management capabilities.' WHERE id = 'cliente_admin';
UPDATE roles SET description = 'External client operator with read-only/operational access.' WHERE id = 'cliente_operario';
