---
id: 017
title: "Scoping de tenant en el CRUD de tenants"
tier: change
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-07-20
repos: [embolsadora4.0-cloud]
origin: superpowers
issues: []
prs: [45]
adrs: [CLOUD-ADR-015, CLOUD-ADR-016]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 017 — Scoping de tenant en el CRUD de tenants

> **Procedencia.** `origen: original` viene del diseño aprobado del 2026-07-20, en
> [`design.md`](design.md), citado por sección. `origen: derivado` se reconstruyó del
> código. **Verificado contra `develop` el 2026-09-29.** Es `tier: change`: corrige un
> hueco de autorización sin agregar funcionalidad.

## Contexto y problema

`GET/PATCH/DELETE /api/v1/tenants/:tenantId` y `GET /api/v1/tenants` autorizaban solo por
el nombre del permiso, sin verificar que el tenant pedido fuera el del usuario. Un `admin`
de un tenant cliente podía **leer cualquier otro tenant por UUID**, y lo habría podido
modificar si algún día su rol recibía el permiso de escritura. El hueco apareció en las
pruebas de integración con el frontend (§Problem).

## Alcance

**Entra:** el chequeo de pertenencia en los cuatro endpoints de tenants.

**No entra** (§Non-goals):
- El header `x-tenant-id` del proxy del frontend.
- La separación entre "gestión de tenants de plataforma" y "configuración propia del
  tenant".

## Requisitos

- **RF-001** `origen: original` (§Design 2) — Un rol no global que pide `GET`, `PATCH` o
  `DELETE` sobre un tenant distinto del suyo DEBE recibir 403 `FORBIDDEN`, exista o no el
  tenant destino (así no se pueden enumerar tenants).
  → `get_tenant.go`, `update_tenant.go`, `delete_tenant.go`
  (`!IsCrossTenantRole && !TenantMatches`).
- **RF-002** `origen: original` (§Design 3) — `GET /tenants` para un rol no global DEBE
  devolver una lista con un solo elemento, su propio tenant, en vez de todos.
  → `get_all_tenants` (usecase con `scopeToTenantID`).
- **RF-003** `origen: original` (§Design 4) — Los roles cross-tenant (`super_admin`,
  `tenant_manager`, `platform_admin`) DEBEN mantener el acceso sin restricción
  (CLOUD-ADR-015).
- **RF-004** `origen: original` (§Design 1), **mecanismo cambiado** — El diseño definía
  los roles cross-tenant con una lista fija en Go. **Derivado (PR #62):**
  `IsCrossTenantRole` lee `is_global` del rol efectivo, cargado de la base. El efecto es
  el mismo, porque los tres roles tienen `is_global = TRUE` (migración `000011`).
  → `security.IsCrossTenantRole`.

## Criterios de éxito

Vienen de la sección Testing del diseño.

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | Un `admin` del tenant A recibe 403 en `GET`, `PATCH` y `DELETE /tenants/{B}` | `delete_tenant_test.go` (`…ForeignTenant_NonGlobalRole_Forbidden`), tests análogos en `get_tenant` y `update_tenant` |
| **CE-002** | Un `admin` del tenant A puede leer y modificar `/tenants/{A}` | `…OwnTenant_NonGlobalRole_Allowed` |
| **CE-003** | Los roles cross-tenant no tienen restricción | `…CrossTenantRole_Allowed`, `TestGetAllTenants_CrossTenantRole_ReturnsFullList` |
| **CE-004** | `GET /tenants` para un `admin` devuelve solo su tenant | `TestGetAllTenants_NonGlobalRole_ReturnsOnlyOwnTenant` |

## Decisiones

| Decisión | Alternativa descartada | Por qué |
|---|---|---|
| 403 genérico, exista o no el tenant | 404 si no existe | Evitar enumerar tenants, igual que `resolve_tenant_path.go` |
| `GET /tenants` devuelve el propio tenant | 403 total | No romper a un consumidor legítimo que lista para autoservicio |
| Reusar el contexto de `TenantFromHeader` | Una consulta nueva | El tenant y el rol ya están validados en el contexto |

## Riesgos y preguntas abiertas

- El chequeo vive en cada handler, no en un middleware. Un endpoint de tenants nuevo tiene
  que acordarse de repetirlo.
