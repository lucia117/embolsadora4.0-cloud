-- ============================================================================
-- Migration 000007: Translate system role descriptions to Spanish
-- ============================================================================
-- 000002_seed_essentials seeded the 6 system roles with English descriptions,
-- and 000005 later translated the permission catalog + seeded role permissions
-- but left the role descriptions in English. This migration translates them.
--
-- Applied as UPDATEs (not edits to 000002) because 000002 already ran against
-- the live database with ON CONFLICT DO NOTHING, so re-running it would not
-- update already-seeded rows. System roles have a single row each (tenant_id
-- NULL) and are surfaced in every tenant's role list, so one UPDATE per role
-- id updates the description everywhere.
--
-- All statements are UPDATEs by primary key, so this migration is idempotent
-- and safe to re-run.
-- ============================================================================

UPDATE roles SET description = 'Acceso total al sistema. Multi-tenant. Puede crear y administrar cualquier tenant.' WHERE id = 'super_admin';
UPDATE roles SET description = 'Rol de soporte multi-tenant para el equipo de MRG. Accede a cualquier tenant con permisos de escritura limitados.' WHERE id = 'tenant_manager';
UPDATE roles SET description = 'Administrador del tenant. Gestiona usuarios y configuración dentro de un único tenant.' WHERE id = 'admin';
UPDATE roles SET description = 'Operario del día a día dentro de un tenant.' WHERE id = 'operario';
UPDATE roles SET description = 'Administrador de cliente externo con capacidades de gestión limitadas.' WHERE id = 'cliente_admin';
UPDATE roles SET description = 'Operario de cliente externo con acceso operativo / de solo lectura.' WHERE id = 'cliente_operario';
