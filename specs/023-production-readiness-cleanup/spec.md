---
id: 023
title: "Production readiness cleanup"
tier: change
status: done
veredicto: A
owner: Federico Degiovanni
date: 2026-08-19
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: superpowers
issues: [96]
prs: [69]
adrs: [CLOUD-ADR-015]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 023 — Production readiness cleanup

> **Procedencia.** `origen: original` viene del diseño consolidado del 2026-08-19, en
> [`design.md`](design.md), organizado en sub-proyectos A a G. Esta spec cubre solo lo que
> toca a este repo; los sub-proyectos D y F son del frontend. **Verificado contra `develop`
> el 2026-09-29.**

## Contexto y problema

Antes de dar el MVP por listo para producción quedaban 15 pendientes, entre ellos dos
huecos cross-tenant en el backend, RBAC faltante en edge devices, casos borde de datos y
un 5xx intermitente sin explicar. El diseño los agrupó en sub-proyectos y decidió cuáles
se arreglaban, cuáles se investigaban y cuáles se difieren con una decisión documentada.

## Alcance

**Entra (backend):** sub-proyectos A, B (investigación), C, E y G (decisiones).

**No entra:** D y F, que son del frontend.

## Requisitos

- **RF-001** `origen: original` (sub-proyecto A) — Las mutaciones y lecturas de usuario
  (`DeleteUser`, `GetUserWithRoles`, `UpdateUser` y `UpdateUserStatus`) DEBEN recibir el
  flag `crossTenant`, igual que `GetUser`. Así un operador de plataforma puede operar
  sobre usuarios de otro tenant, y el resto sigue acotado a su tenant.
  → `internal/app/users/service.go`, `internal/repo/pg/users/postgres.go`.
- **RF-002** `origen: original` (sub-proyecto C) — Cada ruta de edge devices DEBE exigir
  su permiso con `RBACCheck` (ver RF-020 de [003](../003-edge-device-management/spec.md)).
  → `internal/api/handler/edge_devices/routes.go`.
- **RF-003** `origen: original` (sub-proyecto E, #7) — Al elegir la membresía más
  reciente, un `assigned_at` nulo NO DEBE ganar el desempate.
  → `ORDER BY … t.assigned_at DESC NULLS LAST` en `PostgresRepository.GetByID`.
- **RF-004** `origen: original` (sub-proyecto E, #8) — `platform_admin` NO DEBE poder
  asignarse fuera del tenant plataforma, y tiene que haber un test que lo confirme.
  → `TestTriggerRechazaInsertRawDePlatformAdminEnTenantNoPlataforma`.
- **RF-005** `origen: original` (sub-proyecto B) — Investigar los 5xx del BFF pese a
  escrituras exitosas. **Resultado:** no es un bug de código. La causa es el cold start de
  Cloud Run (sin `minScale`) combinado con el timeout del `fetch` en Vercel. Quedó como
  pendiente de infraestructura, sin fix.
- **RF-006** `origen: original` (sub-proyecto G) — Se difieren con una decisión
  documentada:
  - #9, dividir `_manage` en `_create`, `_update` y `_delete`;
  - #15, rate limit real en `GET /api/v1/public/tenants/:idOrSubdomain`.

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-001** | Un `super_admin` parado en el tenant A puede borrar, editar, leer con roles y cambiar el estado de un usuario del tenant B; un `admin` no | `internal/repo/pg/users/{delete,update,update_status,with_roles}_cross_tenant_test.go` |
| **CE-002** | Cada ruta de edge devices exige su permiso | `internal/api/handler/edge_devices/routes_rbac_test.go` |
| **CE-003** | El trigger rechaza `platform_admin` fuera del tenant plataforma | `internal/repo/pg/user_roles/repository_test.go` |

## Riesgos y preguntas abiertas

- **Cold start de Cloud Run** (RF-005): sigue sin resolverse. Configurar `minScale`
  depende de costo y de la salida de la fase MVP (ver `docs/operations.md`).
- **Endpoint público de tenants sin rate limit** (RF-006, #15). El diseño decía que no
  tenía ningún middleware; hoy recibe los globales (`RequestID`, `Logger`, `CORS`, desde
  `cmd/api/main.go`), pero sigue sin límite. Retomarlo al salir de la fase MVP.
