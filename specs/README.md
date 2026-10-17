---
title: Índice de specs — embolsadora4.0-cloud
status: vigente
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Specs

Índice curado a mano. Cada spec vive en `specs/NNN-slug/`, con `spec.md` (qué y por qué) y
`plan.md` (cómo, en `tier: feature`). Una referencia desde otro repo se escribe
`embolsadora4.0-cloud#NNN`.

**Próximo número libre: `026`.**

## Estados y veredictos

- `status`: `draft` · `approved` · `in-progress` · `done` · `partial` · `abandoned` · `superseded`.
- `veredicto` (reconciliación contra el código, 2026-09-29):
  - **A**: implementado como se especificó.
  - **B**: implementado distinto.
  - **C**: nunca implementado y descartado.
  - **D**: nunca implementado, todavía querido.
  - **E**: basura, eliminada.

Los B y C tienen entrada en la [bitácora de alcance](../docs/_process/bitacora-de-alcance.md).
Un **A** significa que existe el componente que la spec pedía (handler, ruta, tabla). No
significa que se verificaron todos sus requisitos: eso se hace en la pasada de traducción
al formato unificado.

## Índice

| # | Spec | Estado | Veredicto | Fecha | PRs | ADRs |
|---|---|---|---|---|---|---|
| 001 | [User Role Assignment Management](001-user-role-assignments/spec.md) | done | A | 2026-02-27 | #11, #13 | — |
| 002a | [User Management API](002a-user-management/spec.md) | done | A | 2026-03-01 | #15 | — |
| 002b | [Supabase Auth — Backend](002b-supabase-auth-backend/spec.md) | done | A | 2026-03-06 | #16 | CLOUD-ADR-005 |
| 003 | [Edge Device Management API](003-edge-device-management/spec.md) | done | A | 2026-03-09 | #17 | CLOUD-ADR-016 |
| 004 | [AAS Server](004-aas-server/spec.md) | ✗ abandoned | C | 2026-03-24 | — | CLOUD-ADR-019 |
| 005a | [Dashboard Layouts API](005a-dashboard-layouts/spec.md) | done | A | 2026-03-24 | #20 | — |
| 005b | [PLC Events Ingestion & Query API](005b-plc-events/spec.md) | ↻ superseded → 022 | B | 2026-03-24 | (#56) | CLOUD-ADR-017, 018 |
| 006 | [API de Gestión de Roles](006-roles-management/spec.md) | done | A | 2026-04-03 | #22 | — |
| 007 | [Extensión de Gestión de Usuarios](007-user-roles-status/spec.md) | done | A | 2026-04-03 | #24 | — |
| 008 | [Reglas de alarma (CRUD)](008-alarm-rules/spec.md) | done | A¹ | 2026-04-06 | #25 | — |
| 009 | [Servicio de logs](009-log-service/spec.md) | done | A¹ | 2026-04-07 | #26 | — |
| 010 | [Servicio de notificaciones](010-notification-service/spec.md) | done | A¹ | 2026-04-10 | #27 | — |
| 011 | [Permissions Management API](011-permissions-management/spec.md) | done | A | 2026-04-10 | #28, #62 | — |
| 013 | [POST /users con asignación de rol inicial](013-user-create-with-role/spec.md) | done | A | 2026-04-11 | #29 | — |
| 014 | [Consolidación de migraciones](014-consolidate-migrations/spec.md) | done | A | 2026-05-08 | #36 | CLOUD-ADR-014 |
| 015 | [AAS Shells (infraestructura MongoDB + CRUD AAS)](015-aas-shells/spec.md) | ✗ abandoned | C | 2026-04-02 | #30 (cerrado), #76 | CLOUD-ADR-019 |
| 016 | [Estandarización de errores y respuestas (P1)](016-api-standardization/spec.md) | done | A² | 2026-04-16 | #32 | — |
| 017 | [Scoping de tenant en el CRUD de tenants](017-tenant-ownership-scoping/spec.md) | done | A | 2026-07-20 | #45 | CLOUD-ADR-015 |
| 018 | [Seed de roles y permisos (traducción)](018-roles-permissions-seed-data/spec.md) | done | A | 2026-07-21 | #46 | — |
| 019 | [Enriquecimiento de user-roles del tenant](019-tenant-user-roles-enrichment/spec.md) | done | A | 2026-07-21 | #47 | — |
| 020 | [Campos de configuración del tenant](020-tenant-settings-fields/spec.md) | done | A | 2026-07-23 | #48 | — |
| 021 | [Mails de autenticación: plantillas propias y URL por instancia](021-auth-emails/spec.md) | done | A | 2026-07-29 | #53 | CLOUD-ADR-005 |
| 022 | [Endpoint de ingesta del cloud (Edge Pi → Cloud)](022-cloud-ingest-endpoint/spec.md) | done | A | 2026-07-30 | #56 | CLOUD-ADR-017, 018 |
| 023 | [Production readiness cleanup](023-production-readiness-cleanup/spec.md) | done | A | 2026-08-19 | #69 | CLOUD-ADR-015 |
| 024 | [Dashboard Metrics Query API](024-dashboard-metrics-query/spec.md) | done | A | 2026-09-07 | #78 | CLOUD-ADR-017 |
| 025 | [Endurecimiento de roles custom (UpdateRole y guarda de rol propio)](025-endurecimiento-roles-custom/spec.md) | draft | D | 2026-10-07 | (#106, #108, #109) | — |

² La P1 estandarizó los módulos de su alcance (alarm_rules, roles, dashboard_layouts,
notifications, logs, permissions, tenants, users). `edge_devices`, `user_roles`,
`dashboards` y el middleware todavía usan el envelope `{"success":...}`: hoy conviven dos
convenciones de respuesta.

¹ Las specs 008, 009 y 010 se cumplen, pero **nada produce los datos que consultan**:
no hay motor que evalúe reglas, ni código que escriba logs o notificaciones. Las tres
lo dejaron fuera de alcance. El objetivo específico #5 (alertas automáticas) no se cumple; ver la
[bitácora](../docs/_process/bitacora-de-alcance.md#objetivo-específico-5-alertas-automáticas).

## Notas

- **Colisiones de numeración resueltas el 2026-09-29.** `002-user-management` y
  `002-supabase-auth-backend` pasaron a `002a` y `002b`; `005-dashboard-layouts` y
  `005-plc-events` a `005a` y `005b`. El sufijo sigue el orden de creación, con
  desempate alfabético.
- **012 no existe.** No hay registro de que se haya usado.
- **`specs/develop/`** era un duplicado de 013 (veredicto E). Se eliminó el 2026-09-29.
- **004 y 005b** estaban en `.gitignore` y solo existían en la rama
  `origin/docs/nosql-aas-plc-specs`. Se recuperaron con `docs/nosql-aas-research.md`,
  ahora `004-aas-server/research.md`.
- **015** es la spec rescatada del PR #30 en el PR #76. Toma el número `015` porque es el
  que usaba el #76 y el que le corresponde por fecha.
- **001** tenía `spec.md` y `spec.es.md` (duplicado bilingüe). Al traducirla (2026-09-29)
  se conservó el original en inglés como `design.md` y se eliminó la copia en español.
- **016 a 024** vienen de `docs/superpowers/` (diseño → `spec.md`, plan → `plan.md`),
  incorporados el 2026-09-29 y numerados por la fecha del diseño. Su veredicto A es por
  presencia en el código, igual que el resto. Aún no tienen `RF-NNN`.
- **022** no tiene diseño en este repo: el diseño aprobado vive en `embolsadora-edge`
  (`spec_externa` en el front-matter). Su `spec.md` lo resume y enlaza.

## Progreso de la traducción al formato unificado (fase 3c)

Una spec traducida tiene `traducido:` en el front-matter y `RF-NNN`/`CE-NNN` verificados
contra el código, con procedencia `original` o `derivado`. Si algún requisito original no
se cumple, figura en `rf_incumplidos`. El documento de origen se
conserva al lado (`design.md`) o en otro repo (`spec_externa`).

| Estado | Specs |
|---|---|
| Traducidas | todas las vigentes (22): 001, 002a, 002b, 003, 005a, 006, 007, 008, 009, 010, 011, 013, 014, 016, 017, 018, 019, 020, 021, 022, 023, 024 |
| No requieren traducción (veredicto C o B reemplazada; el cuerpo se conserva como evidencia) | 004, 005b, 015 |

## Material asociado y archivado

| Archivo original | Destino | Por qué |
|---|---|---|
| `2026-04-11-create-user-with-initial-role-design.md` | [`013/design.md`](013-user-create-with-role/design.md) | Diseño superpowers de la misma feature que 013 |
| `2026-08-17-rbac-dynamic-permissions-backend.md` | [`011/plan-rbac-dynamic-permissions.md`](011-permissions-management/plan-rbac-dynamic-permissions.md) | Extiende el catálogo de permisos (PR #62); el diseño vive en `embolsadora-frontend` |
| `2026-07-29-invitation-email-followups.md`, `2026-07-29-tokenhash-migration-report.md`, `2026-07-30-invited-user-password-report.md`, `2026-07-31-pr53-review-fixes.md` | [`021-auth-emails/`](021-auth-emails/spec.md) | Continúan la feature de mails de auth |
| Planes de UAT (B-002/004/006), B-005, post-RBAC, OpenAPI, Postman y cobertura de tests | [`docs/_process/plans/`](../docs/_process/README.md) | Fixes y tareas sin diseño propio en este repo |

## Rutas anteriores

`docs/superpowers/` ya no existe. Las migraciones aplicadas y los planes históricos todavía
citan las rutas viejas; así se resuelven:

| Ruta anterior (`docs/superpowers/…`) | Ruta actual |
|---|---|
| `specs/2026-04-11-create-user-with-initial-role-design.md` | `specs/013-user-create-with-role/design.md` |
| `specs/2026-04-16-p1-api-standardization-design.md`, `plans/2026-04-11-p1-error-response-standardization.md` | `specs/016-api-standardization/` |
| `2026-07-20-tenant-ownership-scoping*` (diseño y plan) | `specs/017-tenant-ownership-scoping/` |
| `2026-07-21-roles-permissions-seed-data*` | `specs/018-roles-permissions-seed-data/` |
| `2026-07-21-tenant-user-roles-enrichment*` | `specs/019-tenant-user-roles-enrichment/` |
| `2026-07-23-tenant-settings-fields*` | `specs/020-tenant-settings-fields/` |
| `2026-07-29-invitation-email*`, `plans/2026-07-29-tokenhash-migration-report.md`, `plans/2026-07-30-invited-user-password-report.md`, `plans/2026-07-31-pr53-review-fixes.md` | `specs/021-auth-emails/` |
| `plans/2026-08-05-cloud-ingest-endpoint.md` | `specs/022-cloud-ingest-endpoint/plan.md` |
| `2026-08-19-production-readiness-cleanup*` | `specs/023-production-readiness-cleanup/` |
| `specs/2026-09-07-dashboard-metrics-query-design.md`, `plans/2026-09-11-dashboard-metrics-query-api.md` | `specs/024-dashboard-metrics-query/` |
| `plans/2026-08-17-rbac-dynamic-permissions-backend.md` | `specs/011-permissions-management/plan-rbac-dynamic-permissions.md` |
| `plans/2026-08-13…`, `2026-08-14…`, `2026-08-18…` y `2026-09-19…` a `2026-09-22…` | `docs/_process/plans/` (mismo nombre) |
| `specs/2026-08-04-platform-operator-rbac-design.md`, `specs/2026-08-17-rbac-dynamic-permissions-design.md` | Nunca estuvieron en este repo: viven en `embolsadora-frontend/docs/superpowers/specs/` |
