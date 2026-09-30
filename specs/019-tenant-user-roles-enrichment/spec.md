---
id: 019
title: "Enriquecimiento de GET /user-roles con nombre de rol y datos del usuario"
tier: change
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-07-21
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: []
prs: [47]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 019 — Enriquecimiento de GET /user-roles con nombre de rol y datos del usuario

> **Procedencia.** `origen: original` viene del diseño aprobado del 2026-07-21, en
> [`design.md`](design.md). **Verificado contra `develop` el 2026-09-29.**

## Contexto y problema

La pestaña "Usuarios y roles" de un tenant en el frontend mostraba el `roleId` crudo y, a
veces, un placeholder `Usuario a1b2c3d4…` en lugar del usuario. La causa estaba en el
backend:
- `GET /user-roles?tenantId=` devolvía solo `roleId`.
- Para completar los datos, el frontend hacía un segundo fetch a `GET /users?tenantId=`,
  que filtraba por `users.tenant_id`.
- Los usuarios auto-provisionados por Supabase tienen `users.tenant_id` nulo y pertenecen
  al tenant solo por `user_tenant_roles`, así que nunca aparecían.

El diseño establece que **`user_tenant_roles` es la fuente de verdad de la pertenencia**,
no `users.tenant_id` (§Problem).

## Alcance

**Entra:** la consulta, el tipo de dominio y la respuesta de `list_user_roles`.

**No entra:**
- `GET /users`.
- Los demás endpoints de `user_roles`.
- Cambios en el frontend.

## Requisitos

- **RF-001** `origen: original` (§Design 1) — La consulta de asignaciones por tenant DEBE
  hacer `JOIN users` (siempre resoluble, por la FK `NOT NULL`) y `LEFT JOIN roles` (el rol
  puede ser nulo en asignaciones pendientes).
  → `FindByTenantQuery` en `internal/repo/pg/user_roles/resources.go`.
- **RF-002** `origen: original` (§Design 2) — El dominio DEBE exponer
  `UserTenantRoleDetail`, con `RoleName`, `UserEmail`, `UserName`, `UserFirstName` y
  `UserLastName`.
  → `internal/domain/user_roles.go`.
- **RF-003** `origen: original` (§Design 3) — La respuesta DEBE ser **aditiva**: conserva
  todos los campos previos y suma `roleName` y `user: {email, name, firstName, lastName}`.
  → `list_user_roles/models/response.go`.

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | El listado resuelve usuario y rol en una sola consulta, incluidos los usuarios con `users.tenant_id` nulo | `internal/repo/pg/user_roles/repository_test.go` (`TestFindByTenant_ResolvesUserAndRoleAcrossJoin`) |
| **CE-002** | Ningún campo previo cambia | RF-003; `list_user_roles/usecase_test.go` |

## Decisiones

| Decisión | Alternativa descartada | Por qué |
|---|---|---|
| Enriquecer `GET /user-roles` | Arreglar `GET /users?tenantId=` | `GET /users` tiene su propio problema de pertenencia; este camino ya usaba la relación correcta |
| Respuesta aditiva | Reemplazar `roleId` | Cero cambios incompatibles para los consumidores existentes |

## Riesgos y preguntas abiertas

- El problema de fondo, que `users.tenant_id` no es la fuente de verdad de la pertenencia,
  sigue presente en otros caminos. `GET /users` lo resuelve hoy con un `OR` (ver RF-019 de
  [002a](../002a-user-management/spec.md)).
