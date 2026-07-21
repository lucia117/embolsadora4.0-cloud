# Roles & Permissions seed data fix (permissions + translation)

**Date**: 2026-07-21
**Status**: Approved, pending implementation plan

## Context

Integration testing against the frontend (tracked in `~/Develop/UTN/issues-20-07-2026.md`)
found that the `/roles` and `/permissions` pages show all 6 seeded system roles
with 0 permissions, and that permission names/descriptions are in English. Most
of the other `/roles` and `/permissions` page issues (tenants column in the
tables, header tenant selector, custom-role routing bug, no tenant field when
creating a permission) live entirely in `embolsadora-frontend` and are out of
scope for this backend-only cycle — this spec covers only what
`embolsadora4.0-cloud` owns: the seed data itself.

## Problem

`migrations/000002_seed_essentials.up.sql` seeds the 6 system roles
(`super_admin`, `tenant_manager`, `admin`, `operario`, `cliente_admin`,
`cliente_operario`) with `permissions: '[]'::jsonb`, and seeds the 17 system
permissions with English `name`/`description`.

- `embolsadora-frontend`'s `/roles` and `/permissions` pages render
  `roles.permissions` and `permissions.name`/`description` directly from the
  backend (confirmed: `src/app/s/[tenantId]/(dashboard)/permissions/page.tsx`
  maps `p.name`/`p.description` straight from the `GET /api/v1/permissions`
  response) — so both bugs are pure data problems, not frontend bugs.
- This `permissions` jsonb column is a separate vocabulary from
  `internal/security/rbac.go`'s `rolePermissions` map (`"resource:action"`
  strings like `"tenants:read"`, used for actual backend authorization). The
  `roles.permissions` column instead uses the `perm_*` catalog IDs from the
  `permissions` table, matching what `embolsadora-frontend/src/lib/permissions.ts`
  and `role-registry.ts` expect for UI gating and display. This spec only
  touches the UI-facing catalog; `rbac.go`'s enforcement map is untouched and
  unaffected.

## Non-goals

- `internal/security/rbac.go`'s `rolePermissions` map (backend authorization
  enforcement) — not touched, not affected by this fix.
- Any `embolsadora-frontend` change (tenants column in roles/permissions
  tables, header tenant selector, custom-role view/edit routing bug, tenant
  field on permission creation) — separate spec, different repo.
- Renaming/restructuring the `permissions` table or its 17 system IDs.

## Design

### 1. New migration, not an edit to `000002`

`000002_seed_essentials` has already been applied against the live database
(Supabase Postgres, migrated from the old Koyeb-hosted Postgres on
2026-07-16 — see `~/.claude/.../memory/embolsadora-stack.md`). Its inserts use
`ON CONFLICT DO NOTHING`, so editing the file's literal values would only
affect a fresh, never-migrated database — it would silently do nothing against
the already-seeded Supabase database. Per the project's own convention
(`migrations/README.md`: "Crear una nueva migración"), this needs a new
migration: `000005_translate_permissions_and_seed_role_permissions`, using
idempotent `UPDATE` statements (safe to re-run) with a `.down.sql` that
reverts both changes.

### 2. Permission-to-role mapping

Populate `roles.permissions` for the 6 system roles using the `perm_*` catalog
IDs, mirroring `embolsadora-frontend/src/lib/role-registry.ts`'s
`PREDEFINED_ROLES` fallback data where a role has a direct equivalent there
(`super_admin`, `tenant_manager`, `admin`, `operario`). `cliente_admin` and
`cliente_operario` have no frontend fallback equivalent, so their sets mirror
their existing `rbac.go` `rolePermissions` entries instead (read-mostly,
narrower than `admin`/`operario`):

| Role | Permissions |
|---|---|
| `super_admin` | all 17: `perm_dashboard`, `perm_alerts`, `perm_reports`, `perm_users`, `perm_tenants`, `perm_settings`, `perm_maintenance`, `perm_analytics`, `perm_all_tenants`, `perm_logs_view`, `perm_logs_export`, `perm_logs_admin`, `perm_edge_devices_view`, `perm_edge_devices_manage`, `perm_edge_devices_check`, `perm_reports_view`, `perm_reports_manage` |
| `tenant_manager` | `perm_all_tenants`, `perm_dashboard`, `perm_alerts`, `perm_reports`, `perm_reports_view`, `perm_users`, `perm_edge_devices_view`, `perm_edge_devices_check` |
| `admin` | `perm_dashboard`, `perm_alerts`, `perm_reports`, `perm_reports_view`, `perm_reports_manage`, `perm_users`, `perm_tenants`, `perm_settings`, `perm_maintenance`, `perm_analytics`, `perm_edge_devices_view`, `perm_edge_devices_manage` |
| `operario` | `perm_dashboard`, `perm_alerts`, `perm_reports_view`, `perm_edge_devices_view`, `perm_edge_devices_check` |
| `cliente_admin` | `perm_dashboard`, `perm_alerts`, `perm_reports_view`, `perm_users`, `perm_edge_devices_view` |
| `cliente_operario` | `perm_dashboard`, `perm_edge_devices_view` |

### 3. Translation

Translate all 17 `permissions.name`/`description` rows to Spanish:

| id | name (es) | description (es) |
|---|---|---|
| `perm_dashboard` | Ver Panel | Acceso al panel principal y widgets |
| `perm_alerts` | Ver Alertas | Acceso al centro de alertas y notificaciones |
| `perm_reports` | Ver Reportes | Acceso a reportes y analítica |
| `perm_users` | Gestionar Usuarios | Crear, editar y eliminar usuarios |
| `perm_tenants` | Gestionar Tenants | Acceso a la gestión de tenants |
| `perm_settings` | Gestionar Configuración | Acceso a la configuración del sistema |
| `perm_maintenance` | Ver Mantenimiento | Acceso al módulo de mantenimiento |
| `perm_analytics` | Ver Analítica | Acceso a paneles de analítica avanzada |
| `perm_all_tenants` | Acceso a Todos los Tenants | Acceso cross-tenant (solo Super Admin) |
| `perm_logs_view` | Ver Logs | Acceso al visor de logs |
| `perm_logs_export` | Exportar Logs | Exportar datos de logs a archivo |
| `perm_logs_admin` | Gestionar Configuración de Logs | Gestionar retención y configuración de logs |
| `perm_edge_devices_view` | Ver Dispositivos Edge | Ver el listado y estado de dispositivos edge |
| `perm_edge_devices_manage` | Gestionar Dispositivos Edge | Crear, editar, habilitar y deshabilitar dispositivos edge |
| `perm_edge_devices_check` | Ejecutar Chequeos Edge | Ejecutar chequeos de estado y salud en dispositivos edge |
| `perm_reports_view` | Ver Reportes | Acceso al historial de reportes y descargas |
| `perm_reports_manage` | Gestionar Reportes | Generar reportes, gestionar programaciones y retención |

### Down migration

Reverts both changes: sets `permissions` back to `'[]'::jsonb` for the 6
roles, and restores the original English `name`/`description` for the 17
permissions (values as currently seeded in `000002_seed_essentials.up.sql`).

## Testing

- Add a focused test in `internal/platform/dbmigrate/runner_test.go` that
  runs all migrations against a test database and then asserts: each of the
  6 system roles has a non-empty `permissions` array matching the table
  above, and each of the 17 permissions has the translated Spanish
  `name`/`description`.
- Verify `down` then `up` round-trips cleanly (existing pattern used for the
  other migrations in this project).
- No Go application code changes, so no handler/usecase tests are needed —
  this is a data-only migration.
