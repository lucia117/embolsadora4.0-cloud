---
id: 006
title: "API de Gestión de Roles"
tier: feature
status: done
veredicto: A
rf_incumplidos: [RF-013]
owner: Lucia Scharff
date: 2026-04-03
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [22, 32]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 006 — API de Gestión de Roles

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-04-03,
> conservada en [`design.md`](design.md). Esa spec ya numeraba sus requisitos como
> `RF-NNN` y sus criterios como `CE-NNN`; acá se conserva esa numeración. `origen:
> derivado` se reconstruyó del código. **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

Cada tenant necesita roles propios además de los del sistema, con un tope para no
convertir la gestión de permisos en algo inmanejable. Los roles del sistema tienen que
ser visibles y a la vez intocables. Esta spec define el CRUD de roles
(`/api/v1/roles`).

Desde la migración `000011` y el plan de permisos dinámicos (PR #62, en
[011](../011-permissions-management/spec.md)), los permisos de un rol **ya no son strings
decorativos**: `security.Can()` los lee de `roles.permissions` en cada request, así que un
rol custom otorga acceso real.

## Alcance

**Entra:** `GET /roles`, `GET /roles/:id`, `POST /roles`, `PUT /roles/:id` y
`DELETE /roles/:id`.

**No entra:**

- La asignación de roles a usuarios, que está en [001](../001-user-role-assignments/spec.md).
- El catálogo de permisos, que está en [011](../011-permissions-management/spec.md).

## Requisitos

- **RF-001** `origen: original` — Todo endpoint DEBE exigir un JWT válido; sin él
  responde 401.
- **RF-002** `origen: original` — Toda operación DEBE estar acotada al tenant de
  `X-Tenant-ID`.
- **RF-003** `origen: original` — El listado DEBE devolver los roles del sistema y los
  custom del tenant. **Derivado:** los roles globales (`super_admin`, `tenant_manager`,
  `platform_admin`) solo los ve `super_admin` (cloaking). Un rol invisible responde 404,
  no 403, para no confirmar que existe.
  → `rolesRepo.GetByIDForTenant` con `includeGlobal`, `Service.UpdateRole` y
  `Service.DeleteRole`.
- **RF-004** `origen: original` — Un tenant puede tener hasta 3 roles custom. Al
  alcanzar el tope, crear responde 403 `LIMIT_REACHED`.
  → `domain.MaxCustomRolesPerTenant = 3`, `internal/api/handler/roles/errors.go`.
- **RF-005** `origen: original` — El nombre de un rol custom DEBE ser único por tenant;
  un duplicado responde 409 `DUPLICATE_NAME`.
- **RF-006** `origen: original` — Cada rol custom recibe un ID del servidor y
  `createdAt`/`updatedAt`. **Derivado:** el formato del ID es `custom_<6 hex>`.
  → `generateRoleID`.
- **RF-007** `origen: original` y **RF-009** `origen: original` — Borrar o modificar un
  rol del sistema DEBE responder 403 `SYSTEM_ROLE`.
- **RF-008** `origen: original` — Borrar un rol con asignaciones activas DEBE responder
  409 `ROLE_HAS_ASSIGNMENTS` con la cantidad de usuarios afectados (`usersAffected`).
  → `delete_role.go`, `rolesRepo.CountActiveAssignments`.
- **RF-010** `origen: original` y **RF-011** `origen: original` — Un rol custom permite
  actualizar nombre, descripción y permisos (la lista se reemplaza completa), y renueva
  `updatedAt`.
- **RF-012** `origen: original` — Los permisos se deduplican antes de persistir.
  **Derivado:** además se ordenan.
  → `deduplicatePermissions`.
- **RF-013** `origen: original` — **Ya no se cumple, por decisión posterior.** La spec
  pedía el envelope `{success: true, data}` y `{success: false, error}`. La
  estandarización [016](../016-api-standardization/spec.md) (PR #32) lo eliminó en este
  módulo:
  - las respuestas exitosas son el struct directo (un array en el listado);
  - los errores son `{error, message, status}`.
  → `internal/api/handler/roles/*.go`.
- **RF-014** `origen: derivado` — Borrar un rol custom es un borrado lógico y responde
  200 con `{}`.
  → `rolesRepo.SoftDelete`, `delete_role.go`.
- **RF-015** `origen: derivado` — Crear, modificar y borrar roles exige
  `perm_users_manage`; leer no exige un permiso adicional.
  → `internal/routes/url_mappings.go` (`rolesWriteGroup`).

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-006** | Ninguna operación lee ni modifica roles custom de otro tenant | `GetByIDForTenant`; `internal/repo/pg/roles/repository_test.go`, `crud_test.go` |
| **CE-007** | Sin token válido, 401 | `JWTAuth` |
| **CE-008** | El tope de 3 no bloquea leer ni actualizar roles existentes | `Service.CreateRole` (solo cuenta al crear); `internal/app/roles/service_test.go` |
| **CE-009** | El mismo nombre puede existir en tenants distintos | unicidad por tenant en el repo |
| **CE-010** | Los roles del sistema siempre son visibles para quien corresponde, y nunca se borran ni se modifican | `ErrRoleIsSystemRole`; `service_test.go` |

Los CE-001 a CE-005 originales eran umbrales de latencia sin medición: no se trasladan.

## Decisiones

| Decisión | Por qué | Fuente |
|---|---|---|
| Tope de 3 roles custom por tenant | Mantener la gestión simple en el MVP | design (RF-004) |
| Los permisos son strings opacos, sin validar contra el catálogo | El servidor los guarda como vienen | design (entidad Permiso) |
| Roles invisibles responden 404 antes que 403 | No revelar roles de plataforma | derivado |

## Riesgos y preguntas abiertas

- **Permisos sin validar:** un rol custom puede guardar un `perm_*` que no existe en el
  catálogo. Hoy es inofensivo (no otorga nada), pero ahora que los permisos se aplican de
  verdad, conviene validarlos contra `permissions`.
- **RF-013** no rompe al frontend: el BFF (`src/app/api/roles/[id]/route.ts` en
  `embolsadora-frontend`, `origin/develop`) recibe el rol plano y lo envuelve él mismo en
  `{success, data}`. Conviene actualizar el contrato del lado de la spec y no del código.
- Los roles del sistema son 7, no los 4 que listaba la spec: se sumaron `super_admin`,
  `tenant_manager` y `platform_admin` (migraciones `000002` y `000011`).
