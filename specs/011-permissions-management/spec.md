---
id: 011
title: "Gestión de permisos y permisos dinámicos"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-10
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: [93]
prs: [28, 62]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 011 — Gestión de permisos y permisos dinámicos

> **Procedencia.** `origen: original` viene de dos documentos:
>
> - la spec de speckit del 2026-04-10 (CRUD del catálogo), conservada en
>   [`design.md`](design.md), con su número `FR-NNN`;
> - el plan de permisos dinámicos del 2026-08-17 (PR #62), en
>   [`plan-rbac-dynamic-permissions.md`](plan-rbac-dynamic-permissions.md), cuyo diseño
>   vive en `embolsadora-frontend/docs/superpowers/specs/2026-08-17-rbac-dynamic-permissions-design.md`.
>
> `origen: derivado` se reconstruyó del código. **Verificado contra `develop` el
> 2026-09-29.**

## Contexto y problema

La spec original creó el catálogo de permisos (`/api/v1/permissions`): permisos del
sistema, sembrados e inmutables, más permisos custom por tenant. Pero el backend
**autorizaba con otro vocabulario**: un mapa fijo en Go de strings `recurso:acción`. El
catálogo `perm_*` que se guardaba en `roles.permissions` solo lo usaba el frontend para
mostrar u ocultar cosas. Por eso un rol custom (spec [006](../006-roles-management/spec.md))
no otorgaba ningún acceso real (bug B-004 de UAT).

El plan de permisos dinámicos (PR #62) unificó los dos mundos: la autorización pasó a leer
`roles.permissions` de la base en cada request.

## Alcance

**Entra:**

- El CRUD del catálogo de permisos.
- El modelo de autorización por permisos leídos de la base.
- El catálogo fino `perm_*_view` / `perm_*_manage`.

**No entra:**

- El CRUD de roles, que está en [006](../006-roles-management/spec.md).
- La traducción del catálogo, que está en [018](../018-roles-permissions-seed-data/spec.md).

## Requisitos

### Catálogo (spec original)

- **RF-001** `origen: original` (FR-001, FR-002) — `GET /api/v1/permissions` DEBE
  devolver los permisos del sistema y los custom del tenant, distinguidos por
  `isSystemPermission`. Cualquier usuario autenticado puede consultarlo.
  → `permissionsRepo` (`WHERE tenant_id = $1 OR is_system_permission = TRUE`).
- **RF-003** `origen: original` (FR-003) — `POST /api/v1/permissions` DEBE crear un
  permiso custom, validando que `name` tenga al menos 3 caracteres y que `section` y
  `description` no estén vacíos.
  → `internal/app/permissions/service.go`.
- **RF-004** `origen: original` (FR-004, FR-005) — Modificar o borrar un permiso del
  sistema DEBE responder 403.
  → `domain.ErrPermissionIsSystem`, `internal/api/handler/permissions/handler.go`.
- **RF-006** `origen: original` (FR-006) — `PUT /api/v1/permissions/:id` DEBE permitir
  actualizar nombre, sección y descripción de un permiso custom.
- **RF-007** `origen: original` (FR-007) — Borrar un permiso custom DEBE ser un borrado
  físico.
  → `DELETE FROM permissions WHERE … is_system_permission = FALSE AND tenant_id = $2`.
- **RF-009** `origen: original` (FR-009) — Un ID inexistente DEBE responder 404.
- **RF-010** `origen: original` (FR-010) — Los permisos del sistema DEBEN sembrarse por
  migración. **Derivado:** el catálogo creció con `000011` (`perm_users_view`,
  `perm_users_manage`, `perm_tenants_view`, `perm_tenants_manage`, en reemplazo de
  `perm_users` y `perm_tenants`), `000013` (`perm_edge_devices_create`) y `000015`
  (`perm_metrics_view`).
- **RF-011** `origen: original` (FR-011) — Los IDs de los permisos custom DEBEN generarse
  en el servidor y ser únicos.
- **RF-012** `origen: original` (FR-012) — Los permisos custom DEBEN estar aislados por
  tenant; los del sistema son visibles para todos.
- **RF-013** `origen: original` (FR-013), **mecanismo cambiado** — Solo un administrador
  DEBE poder crear, modificar o borrar permisos custom. Hoy lo exige `perm_users_manage`.
  → `permissionsWriteGroup` en `internal/routes/url_mappings.go`.

### Autorización dinámica (plan del 2026-08-17)

- **RF-014** `origen: original` (plan, Goal) — `security.Can()` DEBE autorizar leyendo
  los permisos del rol efectivo desde `roles.permissions`, no desde un mapa fijo en Go.
  Así, un rol custom otorga los permisos que tiene cargados.
  → `loadRolePermissions` en `TenantFromHeader`, `security.RoleContext`,
  `security.Can`.
- **RF-015** `origen: original` (plan, Architecture) — `platform_admin` DEBE ser una fila
  real de `roles`, con `is_global = TRUE`, en lugar de un rol calculado en memoria.
  `IsCrossTenantRole` se deriva de `is_global`.
  → migración `000011`, `security.IsCrossTenantRole`.
- **RF-016** `origen: derivado` — `GET /api/v1/me` DEBE devolver los permisos leídos en
  vivo de la base, con el mismo catálogo que usa la autorización.
  → `internal/api/usecases/me_usecase.go`.
- **RF-017** `origen: derivado` — Sin rol en el contexto, `Can()` DEBE denegar
  (fail-closed).

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-002** | Los permisos del sistema nunca se modifican ni se borran por API | original (SC-002) | `ErrPermissionIsSystem`; `internal/app/permissions/service_test.go` |
| **CE-003** | El listado siempre incluye los permisos del sistema | original (SC-003) | seed por migración; `internal/repo/pg/permissions/repository_test.go` |
| **CE-005** | Sin JWT, 401 | original (SC-005) | `JWTAuth` |
| **CE-006** | Los permisos custom de un tenant no son visibles para otro | original (SC-006) | filtro por `tenant_id` en el repo |
| **CE-007** | Un rol custom con un permiso asignado pasa `RBACCheck` para ese permiso | derivado (plan) | `internal/security/e2e_permissions_test.go` (`TestCustomRolePermissionsAreEnforced`) |
| **CE-008** | La resiembra de `000011` deja `roles.permissions` exactamente como la tabla de mapeo del diseño | derivado (plan) | `internal/repo/pg/roles/seed_test.go` (`TestSeedPermissionsMatchDesign`, `TestSeedPermissionsCatalogCleanedUp`) |

- SC-001 (latencia) no tiene medición.
- SC-004 (10 interacciones del pact `permissions-service-api`) no se verifica en este
  repo. Además, `docs/_process/PACTS_ANALYSIS.md` se contradice sobre ese pact (10/10 y
  0/10).
- SC-003 hablaba de "al menos 17 permisos del sistema". Después de `000011` el catálogo
  cambió de forma, así que el número dejó de ser un criterio útil.

## Decisiones

| Decisión | Alternativa descartada | Por qué | Fuente |
|---|---|---|---|
| Autorización leída de la base en cada request | Mantener el mapa fijo en Go | Un rol custom tiene que otorgar acceso real (B-004) | plan-rbac-dynamic-permissions |
| Permisos finos `_view` / `_manage` | `perm_users` / `perm_tenants` gruesos | Separar lectura de escritura | migración `000011` |
| Borrado físico de permisos custom | Borrado lógico | Decisión de la spec original | design (FR-007) |

> **Candidato a ADR:** leer los permisos de la base en cada request es una decisión de
> seguridad difícil de revertir. Está listado en
> [`docs/adr/index.md`](../../docs/adr/index.md#candidatos-a-adr).

## Riesgos y preguntas abiertas

- **Orden de deploy:** cambiar ids del catálogo obliga a coordinar con el frontend (ver
  el incidente del 2026-08-18 en `docs/operations.md`).
- **Una consulta extra por request** para cargar los permisos del rol, sin caché.
- **Borrar un permiso custom lo quita de `roles.permissions`** de los roles del mismo
  tenant, en la misma transacción (issue #93).
