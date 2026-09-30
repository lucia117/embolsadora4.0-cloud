---
title: Bitácora de alcance — embolsadora4.0-cloud
status: vigente
owner: Lucia Scharff
last_reviewed: 2026-09-29
---

# Bitácora de alcance

Registro de lo que se especificó y no se construyó, o se construyó distinto (veredictos B
y C de la reconciliación de specs), y de los objetivos que quedaron sin cumplir. Destino
final: `academico/bitacora-de-alcance.md` en el hub `embolsadora-docs`. Vive acá hasta
que el hub exista.

Cada entrada dice qué se pidió, qué hay hoy, la evidencia y la decisión.

## 004-aas-server

- **Veredicto:** C, nunca implementado y descartado. `status: abandoned`.
- **Qué se pidió (2026-03-24):** un AAS Server en el cloud conforme a IDTA-01002-3-1 e
  IEC 63278-1:2023, alimentado por una Processing API que lee InfluxDB.
- **Qué hay:** nada de AAS en el cloud. El AAS vive en el historian (FA³ST). El cloud
  solo transporta `aasPath` dentro de `payload`.
- **Evidencia:** `proyecto-embolsadora/README.md`, `internal/app/ingest/validate.go`.
  La spec estaba en `.gitignore` y solo existía en la rama
  `origin/docs/nosql-aas-plc-specs`; se recuperó el 2026-09-29.
- **Decisión:** [CLOUD-ADR-019](../adr/CLOUD-ADR-019-aas-fuera-del-cloud.md). Razón del
  cambio no documentada.

## 005b-plc-events

- **Veredicto:** B, implementado distinto. `status: superseded`.
- **Qué se pidió (2026-03-24):** ingesta de eventos de PLC (`ALARM`, `MEASUREMENT`,
  `STATE`) en MongoDB por
  `POST /api/tenants/:tenantId/edge-devices/:deviceId/plc-events`, con JWT. El batch se
  rechazaba completo ante cualquier evento inválido, la deduplicación era por
  `externalId` opcional, y había tags editables, consulta filtrada y un endpoint
  `/latest`.
- **Qué hay:** `POST /api/v1/consumers/events` (PR #56) con API key en vez de JWT.
  Rechazo por evento (`200` + `errors[]`) en lugar de rechazar el batch, deduplicación
  por `(tenantId, eventId)`, sin tags y sin tipos de evento cerrados (`kind` libre). La
  consulta la resuelve la API de métricas de dashboards (`/api/v1/dashboards/metrics`),
  no un endpoint bajo edge-devices.
- **Coincide:** MongoDB como store y el máximo de 1000 eventos por batch.
- **Evidencia:** `internal/consumers/`, `internal/repo/mongo/`, diseño de la ingesta
  (`embolsadora-edge/docs/superpowers/specs/2026-07-30-cloud-ingest-endpoint-design.md`).
- **Decisión:** [CLOUD-ADR-017](../adr/CLOUD-ADR-017-mediciones-en-mongodb.md) y
  [CLOUD-ADR-018](../adr/CLOUD-ADR-018-ingesta-http-batch.md). El rediseño lo impuso el
  contrato congelado del Edge Pi Service, que ya estaba enviando.

## 015-aas-shells

- **Veredicto:** C, nunca implementado y descartado. `status: abandoned`.
- **Qué se pidió (2026-04-02):** capa de infraestructura MongoDB y CRUD de Asset
  Administration Shells y Submodels (`/api/v1/aas/shells`, más un endpoint de consumo
  para el Edge).
- **Qué hay:** la infraestructura Mongo existe, con otro diseño, desde el PR #56. El CRUD
  de AAS no existe.
- **Evidencia:** PR #30 cerrado sin mergear (2026-09-01); spec rescatada en el PR #76 e
  incorporada acá el 2026-09-29.
- **Decisión:** [CLOUD-ADR-019](../adr/CLOUD-ADR-019-aas-fuera-del-cloud.md).

## Objetivo específico #5: alertas automáticas

- **Qué se pidió:** el anteproyecto fija como objetivo específico #5 *"configurar un
  sistema de alertas automáticas ante condiciones anómalas o fallas operativas"*.
- **Qué hay:** el CRUD de reglas de alarma (spec `008-alarm-rules`, PR #25) y la consulta
  y gestión de estado de notificaciones (spec `010-notification-service`, PR #27).
  **No hay motor de evaluación**: ninguna función de `internal/` evalúa reglas contra
  mediciones, y nada inserta filas en `notifications`.
- **Por qué las specs 008 y 010 tienen veredicto A:** las dos dejaron el motor fuera de
  alcance de forma explícita. En 008, *Assumptions*: "no generan notificaciones
  directamente (…) feature futura". En 010, *Assumptions*: "La creación de notificaciones
  será responsabilidad de un worker/trigger interno futuro". Esa feature futura nunca se
  especificó.
- **Decisión (2026-09-29, Lucia Scharff):** **objetivo no cumplido.** El motor de
  evaluación no se implementa antes de la entrega.

## Logs y notificaciones sin productor

- **Qué se pidió:** [009](../../specs/009-log-service/spec.md) y
  [010](../../specs/010-notification-service/spec.md) definieron servicios de **consulta**
  y dejaron la generación a "otros servicios/workers" y a "un worker/trigger interno
  futuro".
- **Qué hay:** existe la interfaz `logwriter.LogWriter`, implementada por
  `logs.Service.Write`, pero ningún componente la llama. Ningún código inserta en
  `notifications`. En producción, las dos tablas solo tienen datos cargados a mano o por
  seed.
- **Además:** la política de retención de logs se guarda pero ningún proceso borra los
  logs vencidos.
- **Relación:** es el mismo hueco que el objetivo #5. Un motor de eventos que evalúe las
  mediciones de la ingesta sería el productor natural de ambos.
- **Decisión:** ninguna explícita. Queda registrado como parte del objetivo #5 no cumplido.

