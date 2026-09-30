---
id: 002a
title: "User Management API"
tier: feature
status: done
veredicto: A
rf_incumplidos: [RF-011]
owner: Lucia Scharff
date: 2026-03-01
repos: [embolsadora4.0-cloud]
origin: speckit
issues: [91, 92]
prs: [15]
adrs: [CLOUD-ADR-016]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 002a — User Management API

> **Procedencia.** Los requisitos `origen: original` vienen de la spec de speckit del
> 2026-03-01, conservada en [`design.md`](design.md), con su número `FR-NNN` original. Los
> `origen: derivado` describen cambios que llegaron después (migración a Supabase,
> membresías múltiples, RBAC por permisos) y se reconstruyeron del código.
> **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

El frontend necesitaba un ABM de usuarios por tenant: listar, ver, crear, editar y dar de
baja, con aislamiento entre tenants y control de acceso para las escrituras.

Desde que se escribió la spec cambiaron tres cosas que cambian la lectura de sus
requisitos:

1. **La identidad pasó a Supabase** ([002b](../002b-supabase-auth-backend/spec.md),
   CLOUD-ADR-005). El tenant ya no sale de claims del JWT sino del header `X-Tenant-ID`,
   validado contra membresías ([CLOUD-ADR-016](../../docs/adr/CLOUD-ADR-016-resolucion-de-tenant.md)).
2. **Un usuario puede pertenecer a varios tenants** mediante `user_tenant_roles`. La
   entidad "cada usuario pertenece a exactamente un tenant" de la spec ya no es cierta.
3. **Los permisos son finos** (`perm_users_view`, `perm_users_manage`), no el rol literal
   `admin` ([011](../011-permissions-management/spec.md), migración `000011`).

## Alcance

**Entra:** `GET /api/v1/users`, `GET /api/v1/users/:id`, `POST /api/v1/users`,
`PATCH /api/v1/users/:id` y `DELETE /api/v1/users/:id`.

**No entra** (lo cubren otras specs):

- `?include=roles`, `PATCH /users/:id/status` y `GET /users/pending`, en
  [007](../007-user-roles-status/spec.md).
- La asignación de rol inicial en `POST /users`, en
  [013](../013-user-create-with-role/spec.md).
- `PATCH /users/me` (B-002, PR #58).

## Requisitos

### Tenant y acceso

- **RF-001** `origen: original` (FR-001, FR-002) — Toda operación DEBE estar acotada al
  tenant del header `X-Tenant-ID`. Si el header falta o no es un UUID, responde 400.
  → `TenantFromHeader` (`internal/api/middleware/middleware.go`).
- **RF-003** `origen: original` (FR-003), **mecanismo cambiado** — El acceso a un tenant
  al que el usuario no pertenece DEBE responder 403. La spec lo validaba "vía claims del
  JWT"; hoy se valida contra una membresía activa en `user_tenant_roles`, con la
  excepción de los operadores de plataforma (CLOUD-ADR-015 y CLOUD-ADR-016).
  → `TenantFromHeader`, `resolvePlatformOperator`.
- **RF-016** `origen: original` (FR-016), **mecanismo cambiado** — Solo quien tenga
  permiso de administración DEBE poder crear, editar o borrar usuarios. La spec pedía el
  rol `admin`; hoy lo exige `RBACCheck("perm_users_manage")`, leído del catálogo de
  permisos del rol en la base.
  → `internal/api/router.go`.
- **RF-017** `origen: derivado` — `GET /users` y `GET /users/:id` NO exigen un permiso
  adicional: los puede usar cualquier miembro activo del tenant. La definición de RBAC de
  lectura quedó como pendiente de decisión (ver `docs/_process/DEUDA-TECNICA.md`, "RBAC en
  GET /users").
  → `internal/api/router.go` (rutas sin `RBACCheck`).
- **RF-018** `origen: derivado` — Los usuarios con roles globales (`super_admin`,
  `tenant_manager`) DEBEN quedar ocultos en listados y consultas para todos, salvo para
  `super_admin` (cloaking; fail-closed si no hay rol en contexto).
  → `security.CanSeePlatformInternals`, `internal/repo/pg/users/cloaking_test.go`.

### Listado y consulta

- **RF-004** `origen: original` (FR-004) — `GET /users` DEBE devolver una lista paginada
  con `limit` (20 por defecto) y `offset` (0 por defecto). **Derivado:** `limit` acepta de
  1 a 100; fuera de rango responde 400 `VALIDATION_ERROR`.
  → `Handler.ListUsers`.
- **RF-005** `origen: original` (FR-005) — La respuesta DEBE incluir
  `pagination: {total, count, limit, offset}`.
  → `dto.ListUsersResponse`.
- **RF-019** `origen: derivado` — El listado DEBE incluir a los usuarios cuyo
  `users.tenant_id` es el tenant **y** a los que tienen una membresía activa en él.
  → `PostgresRepository.ListByTenant` (`LEFT JOIN user_tenant_roles`).
- **RF-006** `origen: original` (FR-006) — `GET /users/:id` DEBE devolver el perfil
  completo, y 404 si no existe, está borrado o no pertenece al tenant. **Derivado:** un
  rol cross-tenant (operador de plataforma) puede leer usuarios de cualquier tenant.
  → `Handler.GetUser`, `PostgresRepository.GetByID` (parámetro `crossTenant`).

### Alta, edición y baja

- **RF-007** `origen: original` (FR-007, FR-015) — `POST /users` DEBE exigir `firstName`,
  `lastName` (hasta 100 caracteres), `email` válido y `role`, con `image` opcional, y
  responder 201.
  → `dto.CreateUserRequest`, `Handler.CreateUser`.
- **RF-008** `origen: original` (FR-008, FR-009) — El email DEBE ser único **dentro de un
  tenant** (409 si se repite) y puede repetirse entre tenants distintos.
  → restricción `users_tenant_id_email_key UNIQUE (tenant_id, email)`, mapeo a 409 en
  `internal/api/handler/users/errors.go`.
- **RF-010** `origen: original` (FR-010) — `PATCH /users/:id` DEBE permitir actualizar en
  forma parcial `firstName`, `lastName`, `role` e `image`.
  → `dto.UpdateUserRequest`, `Handler.UpdateUser`.
- **RF-011** `origen: original` (FR-011) — **No se cumple.** La spec pedía responder 400
  si el request intenta cambiar `email` o `tenantId`. Hoy esos campos no están en
  `UpdateUserRequest` y **se ignoran en silencio**: el request responde 200 sin
  cambiarlos. El dato queda protegido, pero el cliente no se entera de que su cambio no
  se aplicó.
  → `dto.UpdateUserRequest` (sin `DisallowUnknownFields`). El error
  `ErrImmutableField` existe y el handler lo traduce a 400 `IMMUTABLE_FIELD`, pero
  **ningún código lo produce**: quedó preparado y sin conectar.
- **RF-012** `origen: original` (FR-012, FR-013) — `DELETE /users/:id` DEBE hacer un
  borrado lógico (`deleted_at`) y responder 204. Los borrados no aparecen en listados y
  responden 404 por ID.
  → `PostgresRepository.Delete` (`UPDATE users SET deleted_at`), filtro
  `deleted_at IS NULL`.
- **RF-014** `origen: original` (FR-014) — Cada usuario DEBE tener un ID generado por el
  servidor, y `createdAt`, `updatedAt` y `deletedAt`.
  → esquema `users` (`000001`), `dto.UserResponse`.

## Criterios de éxito

| CE | Criterio | Origen | Evidencia |
|---|---|---|---|
| **CE-005** | Ninguna operación expone usuarios de otro tenant | original (SC-005) | `internal/repo/pg/users/cross_tenant_test.go`, `update_cross_tenant_test.go`, `delete_cross_tenant_test.go` |
| **CE-006** | Las requests inválidas reciben 4xx con `error` y `message` | original (SC-006) | `Handler.*`; parcial por RF-011 |
| **CE-007** | Todo alta produce ID único, timestamps y `deletedAt` nulo | original (SC-007) | `internal/repo/pg/users/create_test.go` |
| **CE-008** | Los borrados no aparecen en listados y dan 404 por ID | original (SC-008) | `postgres_repo_test.go`, `delete_sidedoor_test.go` |
| **CE-009** | Solo quien tiene `perm_users_manage` escribe; el resto recibe 403 | original (SC-009), mecanismo cambiado | `RBACCheck` en `router.go` |
| **CE-010** | Dos tenants pueden tener usuarios con el mismo email | original (SC-010) | restricción `UNIQUE (tenant_id, email)` |

Los SC-001 a SC-004 de la spec original eran umbrales de latencia (500 ms, 100 ms,
1 s). **No hay tests ni mediciones que los verifiquen**, así que no se trasladan como
criterios cumplidos.

## Decisiones

| Decisión | Por qué | Fuente |
|---|---|---|
| Borrado lógico en vez de físico | Conservar el historial y las referencias | design (FR-012) |
| Email único por tenant, no global | Un mismo email puede operar en varias empresas | design (FR-008, FR-009) |
| Tenant por header validado contra membresías | Migración a Supabase; usuarios con varios tenants | CLOUD-ADR-016 |

## Riesgos y preguntas abiertas

- **RF-011 incumplido.** Hay que decidir si se rechazan los campos desconocidos en
  `PATCH` (400) o si se actualiza la spec para aceptar que se ignoren.
- **RBAC de lectura** (RF-017): pendiente de decisión; candidato a issue.
- **Modelo mixto de pertenencia:** conviven `users.tenant_id` y `user_tenant_roles`, lo
  que obliga a consultas con `OR` (RF-019).
- **Latencia** (SC-001 a SC-004 originales): sin verificar.
