---
id: 001
title: "Asignación de roles a usuarios por tenant"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-02-27
repos: [embolsadora4.0-cloud]
origin: speckit
issues: []
prs: [11, 13, 47]
adrs: [CLOUD-ADR-015]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 001 — Asignación de roles a usuarios por tenant

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-02-27, en
> inglés, conservada en [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. La
> traducción al español que existía (`spec.es.md`) se eliminó el 2026-09-29 por duplicada.
> `origen: derivado` se reconstruyó del código. **Verificado contra `develop` el
> 2026-09-29.**

## Contexto y problema

Un usuario opera dentro de un tenant con un rol. Hacía falta gestionar esa relación
(`user_tenant_roles`): asignar, listar, cambiar, revocar y asignar en lote, conservando
el historial y con la garantía de un solo rol activo por usuario y tenant.

## Alcance

**Entra:**

- `POST /api/v1/user-roles`, `GET /api/v1/user-roles`, `PUT /api/v1/user-roles/:id`,
  `DELETE /api/v1/user-roles/:id` y `POST /api/v1/user-roles/bulk`.
- `GET /api/v1/users/:id/roles`.

**No entra:**

- Crear usuario y rol a la vez: está en [013](../013-user-create-with-role/spec.md).
- El estado de la membresía expuesto como estado del usuario: está en
  [007](../007-user-roles-status/spec.md).
- Los datos de usuario y nombre del rol en el listado: están en
  [019](../019-tenant-user-roles-enrichment/spec.md).

## Requisitos

- **RF-001** `origen: original` (FR-001) — Un administrador DEBE poder asignar exactamente
  un rol a un usuario en un tenant. Si ya tiene un rol activo ahí, responde 409.
  → índice único parcial `idx_utr_active_unique (user_id, tenant_id) WHERE status = 'active'`,
  `domain.ErrUserAlreadyHasActiveRole`, `assign_user_role.go`.
- **RF-002** `origen: original` (FR-002, FR-003) — Un administrador DEBE poder listar las
  asignaciones del tenant con usuario, rol, estado, quién asignó y fechas, y filtrarlas por
  `status`.
  → `list_user_roles`, `userRoleRepository.FindByTenant`.
- **RF-004** `origen: original` (FR-004) — Un administrador DEBE poder cambiar el rol de
  una asignación sin crear un registro nuevo.
  → `PUT /user-roles/:id`, `userRoleRepository.Update`.
- **RF-005** `origen: original` (FR-005) — Revocar una asignación DEBE marcarla
  `revoked`; el registro nunca se borra físicamente.
  → `RevokeQuery` (`UPDATE … SET status = 'revoked'`, acotado por tenant).
- **RF-006** `origen: original` (FR-006) — La asignación en lote DEBE ser atómica: si
  algún usuario ya tiene un rol activo en el tenant, se rechaza todo sin cambios parciales.
  → `userRoleRepository.BulkCreate` (una transacción).
- **RF-007** `origen: original` (FR-007) — `GET /users/:id/roles` DEBE devolver las
  asignaciones del usuario con nombre de tenant y de rol. **Derivado:** un operador de
  plataforma (rol global) ve todos los tenants; el resto, solo el tenant actual. Exige
  `perm_users_view`.
  → `userRoleRepository.FindByUser` (parámetro `crossTenant`), `internal/api/router.go`.
- **RF-008** `origen: original` (FR-008) — Toda asignación DEBE registrar en
  `assigned_by` al usuario autenticado que la hizo.
  → `platform.UserID(ctx)` en los handlers de alta y lote.
- **RF-009** `origen: original` (FR-009), **mecanismo cambiado** — Las escrituras DEBEN
  requerir autenticación. **Derivado:** además exigen `perm_users_manage`, y el listado
  `perm_users_view`.
  → `internal/api/router.go`.
- **RF-010** `origen: original` (FR-010) — Las entradas inválidas DEBEN responder con un
  error descriptivo: 400 si el usuario o el rol no existen (FK), 409 si ya hay un rol
  activo.
  → `mapForeignKeyViolation`, `resolveActiveUniqueViolation`.
- **RF-011** `origen: derivado` — Los roles de plataforma (`super_admin`,
  `tenant_manager`, `platform_admin`, y `admin`/`operario` desde `000010`) solo pueden
  asignarse dentro del tenant plataforma. La regla la impone la base con el trigger
  `trg_enforce_platform_role_tenant` y la función `tenant_can_use_role`, y la repite la
  aplicación.
  → migraciones `000004` y `000010`, `checkRoleAllowedForTenant`,
  `TestTriggerRechaza*` en `repository_test.go`.
- **RF-012** `origen: derivado` — Las asignaciones de roles globales DEBEN quedar ocultas
  para quien no es `super_admin` (cloaking), y no pueden revocarse ni suspenderse desde
  fuera.
  → `includeGlobal` en `resources.go`, `internal/repo/pg/user_roles/cloaking_test.go`.

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-002** | No puede haber dos asignaciones activas para el mismo usuario y tenant | original (SC-002) | índice `idx_utr_active_unique` (garantía de la base) |
| **CE-003** | Revocar nunca borra el registro | original (SC-003) | `RevokeQuery`; `revoke_user_role/usecase_test.go` |
| **CE-005** | Todo acceso sin autenticación o sin permiso se rechaza | original (SC-005) | `JWTAuth` y `RBACCheck` en el grupo |
| **CE-006** | El filtro por estado devuelve solo coincidencias | original (SC-006) | `FindByTenant` con `status`; `list_user_roles/usecase_test.go` |
| **CE-007** | Toda request inválida recibe un error con mensaje | original (SC-007) | handlers de `user_roles` |
| **CE-008** | Un rol de plataforma no puede asignarse fuera del tenant plataforma, ni por SQL directo | derivado | `TestCreateRechazaAdminEnTenantNoPlataforma`, `TestTriggerRechaza*` |

SC-001 (asignación en menos de 30 s) y SC-004 (lote de 100 en menos de 5 s) no tienen
medición y no se trasladan como cumplidos.

## Decisiones

| Decisión | Por qué | Fuente |
|---|---|---|
| Un solo rol activo por usuario y tenant, garantizado por un índice parcial | La base impide los duplicados aunque haya carreras | design (FR-001) |
| Revocación lógica | Conservar el historial de quién tuvo qué rol | design (FR-005) |
| Lote atómico | Evitar estados parciales difíciles de corregir | design (FR-006) |
| Roles de plataforma restringidos por trigger | Una regla de seguridad no puede depender solo del código de aplicación | derivado (`000004`, `000010`) |

## Riesgos y preguntas abiertas

- **Invitaciones pendientes con roles de plataforma en tenants cliente:** el trigger las
  rechaza al activarlas. Ver la nota de `000010` en `migrations/README.md`.
- **Latencia** (SC-001, SC-004 originales): sin verificar.
