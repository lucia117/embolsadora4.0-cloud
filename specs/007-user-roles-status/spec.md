---
id: 007
title: "Extensión de gestión de usuarios: roles, estado y pendientes"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-03
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [24]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 007 — Extensión de gestión de usuarios: roles, estado y pendientes

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-04-03,
> conservada en [`design.md`](design.md). Esa spec ya numeraba como `RF-NNN`/`CE-NNN`;
> acá se conserva esa numeración. `origen: derivado` se reconstruyó del código.
> **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

El frontend necesitaba ver el rol de un usuario en una sola consulta, cambiar su estado
desde el detalle y listar invitaciones sin activar. Eran cuatro interacciones del pact
`user-service-api-roles-extension`, que extienden la API de usuarios de
[002a](../002a-user-management/spec.md).

## Alcance

**Entra:** `GET /api/v1/users/:id?include=roles`, `PATCH /api/v1/users/:id/status` y
`GET /api/v1/users/pending`.

**No entra:** el alta con rol ([013](../013-user-create-with-role/spec.md)) ni la gestión
de asignaciones ([001](../001-user-role-assignments/spec.md)).

## Requisitos

- **RF-001** `origen: original` — `GET /users/:id?include=roles` DEBE devolver el usuario
  con sus roles asignados.
  → `Handler.GetUser`, `Service.GetUserWithRoles`, `PostgresRepository.GetByIDWithRoles`.
- **RF-002** `origen: original` — Sin `include`, la respuesta DEBE ser la misma de antes
  (compatibilidad hacia atrás).
- **RF-010** `origen: original` — Cada rol en `roles` DEBE traer `id`, `name` y
  `permissions`.
  → `dto.RoleInfo`.
- **RF-003** `origen: original`, **implementación a precisar** — Un administrador DEBE
  poder cambiar el estado de un usuario a `active`, `inactive` o `suspended`; otro valor
  responde 400 `INVALID_STATUS`. **Derivado:** el estado se guarda en la **membresía del
  tenant** (`user_tenant_roles.status`), no en `users.status`, y `inactive` se guarda como
  `revoked`.
  → `Service.UpdateUserStatus`, `userRoleRepository.UpdateStatus`.
- **RF-004** `origen: original` — Solo quien tenga permiso de administración DEBE poder
  cambiar estados. **Derivado:** hoy lo exige `perm_users_manage`.
  → `internal/api/router.go`.
- **RF-005** `origen: original` — El usuario DEBE pertenecer al tenant de quien cambia el
  estado. **Derivado:** un operador de plataforma puede cambiarlo en cualquier tenant, y
  la mutación se aplica sobre el tenant real del usuario, no sobre el de la request.
  → `Service.UpdateUserStatus` (comentario sobre `current.TenantID`),
  `internal/repo/pg/users/update_status_cross_tenant_test.go`.
- **RF-006** `origen: original` — Un administrador NO DEBE poder desactivarse a sí mismo.
  **Derivado:** el bloqueo aplica a **cualquier** cambio de estado sobre uno mismo,
  incluido `active`.
  → `domainUsers.ErrCannotDeactivateSelf`.
- **RF-007** `origen: original` y **RF-008** `origen: original` — `GET /users/pending`
  DEBE devolver los usuarios del tenant con una membresía `pending`, y solo quien tenga
  permiso de lectura de usuarios puede usarlo. **Derivado:** hoy lo exige
  `perm_users_view`.
  → `PostgresRepository.ListPendingByTenant` (`utr.status = 'pending'`), `router.go`.
- **RF-009** `origen: original` — Un usuario tiene como máximo un rol activo por tenant.
  → índice `idx_utr_active_unique` (ver [001](../001-user-role-assignments/spec.md)).

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | El rol de un usuario se obtiene en una sola consulta | `?include=roles`; `internal/repo/pg/users/with_roles_cross_tenant_test.go` |
| **CE-004** | Los pendientes se listan en una sola consulta, sin filtros | `GET /users/pending` |
| **CE-005** | Los 5 endpoints previos de usuarios no cambian su comportamiento | RF-002; tests de [002a](../002a-user-management/spec.md) |

- CE-002 (los 4 pacts satisfechos) no se verifica en este repo: los pacts no corren en su
  CI.
- CE-003 (cambio de estado en menos de 30 s desde la pantalla) es un criterio de UX del
  frontend, sin medición.

## Decisiones

| Decisión | Por qué | Fuente |
|---|---|---|
| `include=roles` como parámetro opcional | Compatibilidad hacia atrás con consumidores existentes | design (RF-002) |
| El estado vive en la membresía, no en el usuario | Un usuario puede tener estados distintos en cada tenant | derivado |

## Riesgos y preguntas abiertas

- **Dos vocabularios de estado.** `users.status` (`invited`, `active`, `revoked`,
  `disabled`, usado por `JWTAuth` para bloquear el acceso) y
  `user_tenant_roles.status` (`active`, `pending`, `revoked`, `suspended`, el que cambia
  este endpoint) son cosas distintas, y el nombre `PATCH /users/:id/status` sugiere lo
  primero. Suspender acá quita el acceso al tenant, no a la cuenta.
- **`inactive` y `revoked`** son el mismo valor en la base: no se pueden distinguir
  después.
