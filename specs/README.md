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

**Próximo número libre: `016`.**

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
| 005b | [PLC Events Ingestion & Query API](005b-plc-events/spec.md) | ↻ superseded | B | 2026-03-24 | (#56) | CLOUD-ADR-017, 018 |
| 006 | [API de Gestión de Roles](006-roles-management/spec.md) | done | A | 2026-04-03 | #22 | — |
| 007 | [Extensión de Gestión de Usuarios](007-user-roles-status/spec.md) | done | A | 2026-04-03 | #24 | — |
| 008 | [Alarm Rules Service API](008-alarm-rules/spec.md) | done | A¹ | 2026-04-06 | #25 | — |
| 009 | [Log Service API](009-log-service/spec.md) | done | A | 2026-04-07 | #26 | — |
| 010 | [Notification Service API](010-notification-service/spec.md) | done | A¹ | 2026-04-10 | #27 | — |
| 011 | [Permissions Management API](011-permissions-management/spec.md) | done | A | 2026-04-10 | #28, #62 | — |
| 013 | [POST /users con asignación de rol inicial](013-user-create-with-role/spec.md) | done | A | 2026-04-11 | #29 | — |
| 014 | [Consolidación de migraciones](014-consolidate-migrations/spec.md) | done | A | 2026-05-08 | #36 | CLOUD-ADR-014 |
| 015 | [AAS Shells (infraestructura MongoDB + CRUD AAS)](015-aas-shells/spec.md) | ✗ abandoned | C | 2026-04-02 | #30 (cerrado), #76 | CLOUD-ADR-019 |

¹ Las specs 008 y 010 se cumplen, pero **el motor que evalúa reglas y genera
notificaciones nunca se especificó ni se implementó**: las dos lo dejaron fuera de
alcance. El objetivo específico #5 (alertas automáticas) no se cumple; ver la
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
- **001** tiene `spec.md` y `spec.es.md` (duplicado bilingüe). Se unifica en un solo
  idioma en la pasada de traducción.
- **Pendiente:** los diseños y planes de `docs/superpowers/` se incorporan como specs
  `016` en adelante.
