---
id: 022
title: "Endpoint de ingesta del cloud (Edge Pi → Cloud)"
tier: feature
status: done
veredicto: A
owner: Lucia Scharff
date: 2026-07-30
repos: [embolsadora4.0-cloud, embolsadora-edge]
origin: superpowers
issues: []
prs: [56]
adrs: [CLOUD-ADR-017, CLOUD-ADR-018]
spec_externa: embolsadora-edge/docs/superpowers/specs/2026-07-30-cloud-ingest-endpoint-design.md
last_reviewed: 2026-09-29
---

# 022 — Endpoint de ingesta del cloud (Edge Pi → Cloud)

> El diseño de esta feature (2026-07-30, estado "Aprobado") vive en el repo
> `embolsadora-edge`, porque lo escribió el equipo que es dueño del contrato. Esta spec no
> copia ese diseño: lo resume y lo enlaza. La traducción al formato unificado, con
> `RF-NNN` verificados contra el código, está pendiente (fase 3c del harness).

## Contexto y problema

El Edge Pi Service de cada planta ya enviaba batches de mediciones a
`POST /api/v1/consumers/events` contra un handler que respondía 501. El contrato HTTP
estaba congelado del lado del Edge
(`embolsadora-edge/specs/002-forwarder-influx/contracts/outbound-events.openapi.yaml`),
y el Edge reacciona distinto a cada código de respuesta: algunos marcan los eventos como
ACKED, otros como DEAD para siempre y otros los reintenta. Una respuesta equivocada
produce pérdida silenciosa de datos.

## Alcance

- **Entra:**
  - Autenticación por API key y la tabla `edge_device_api_keys` (migración `000014`).
  - Validación del sobre de cada evento.
  - Persistencia idempotente en MongoDB.
  - Rate limit por key, métricas y health check de Mongo.
- **No entra:**
  - Validar el contenido de `payload`.
  - Retención o TTL.
  - Caché de idempotencia por batch.
  - La consulta de las mediciones, que la resuelve la spec [024](../024-dashboard-metrics-query/spec.md).

## Decisiones

Registradas como ADR:
- [CLOUD-ADR-017](../../docs/adr/CLOUD-ADR-017-mediciones-en-mongodb.md): mediciones en MongoDB e idempotencia por `(tenantId, eventId)`.
- [CLOUD-ADR-018](../../docs/adr/CLOUD-ADR-018-ingesta-http-batch.md): contrato, códigos de error, invariantes I-1 a I-4 y límites.

El detalle ejecutable (restricciones globales, invariantes y tareas) está en
[`plan.md`](plan.md), sección "Global Constraints".

## Reemplaza

A [`005b-plc-events`](../005b-plc-events/spec.md) (veredicto B). Ver la
[bitácora de alcance](../../docs/_process/bitacora-de-alcance.md#005b-plc-events).
