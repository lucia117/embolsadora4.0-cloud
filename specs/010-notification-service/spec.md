---
id: 010
title: "Servicio de notificaciones: consulta y gestión de estado"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-04-10
repos: [embolsadora4.0-cloud, embolsadora-frontend]
origin: speckit
issues: []
prs: [27]
adrs: []
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 010 — Servicio de notificaciones: consulta y gestión de estado

> **Procedencia.** `origen: original` viene de la spec de speckit del 2026-04-10, en
> [`design.md`](design.md), con su número `FR-NNN`/`SC-NNN`. **Verificado contra `develop`
> el 2026-09-29.**

> ⚠️ **Nada crea notificaciones.** La spec dice: "La creación de notificaciones será
> responsabilidad de un worker/trigger interno futuro; para validación se sembrará datos
> directamente en la BD" (design, *Assumptions*). Ese worker no existe: ningún código
> inserta en `notifications`. Es la otra mitad del objetivo #5 no cumplido; ver
> [008](../008-alarm-rules/spec.md) y la
> [bitácora de alcance](../../docs/_process/bitacora-de-alcance.md#objetivo-específico-5-alertas-automáticas).

## Contexto y problema

El panel de notificaciones del frontend necesita listar las alertas del tenant, contar
las no leídas y marcarlas como vistas o cerradas.

## Alcance

**Entra:** `GET /notifications`, `GET /notifications/count`, `GET /notifications/:id`,
`POST /notifications/:id/ack` y `POST /notifications/:id/close`.

**No entra:** crear notificaciones.

## Requisitos

- **RF-001** `origen: original` (FR-001) — El listado DEBE estar paginado (`limit` y
  `offset`) y ordenado por fecha descendente, con filtros por `status` y `severity`.
- **RF-002** `origen: original` (FR-002) — `GET /notifications/count` DEBE devolver la
  cantidad de notificaciones `unread` del tenant.
- **RF-003** `origen: original` (FR-003, FR-007) — El detalle DEBE estar restringido al
  tenant; otro tenant o un ID inexistente responden 404.
- **RF-004** `origen: original` (FR-004, FR-005) — `ack` DEBE pasar a `acknowledged` con
  `acknowledged_at`; `close` DEBE pasar a `closed` con `closed_at` desde cualquier estado.
- **RF-006** `origen: original` (FR-006) — `ack` y `close` DEBEN ser idempotentes. **Derivado:**
  `ack` sobre una notificación ya `acknowledged` o `closed` la devuelve sin cambios.
  → `internal/repo/pg/notifications/repository.go` (`Ack`, `Close`).
- **RF-008** `origen: original` (FR-008) — Todos los endpoints exigen JWT. **Derivado:**
  no exigen un permiso adicional.

## Criterios de éxito

| CE | Criterio | Evidencia |
|---|---|---|
| **CE-003** | Las notificaciones de un tenant son invisibles para otros | `internal/repo/pg/notifications/repository_test.go` |
| **CE-004** | `ack` y `close` repetidos dan el mismo resultado, sin errores | `repository_test.go`, `internal/app/notifications/service_test.go` |

- SC-001 (rendimiento) no tiene medición.
- SC-002 y SC-005 (pacts e integración con el frontend) no se verifican en este repo.

## Riesgos y preguntas abiertas

- **Sin productor**, la feature solo funciona con datos sembrados a mano.
- FR-009 de la spec original (que `GET /alarm-rules` satisfaga su pact) pertenece a
  [008](../008-alarm-rules/spec.md).
