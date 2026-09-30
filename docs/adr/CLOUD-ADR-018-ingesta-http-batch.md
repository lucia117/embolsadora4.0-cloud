---
id: CLOUD-ADR-018
title: "Ingesta HTTP batch con contrato congelado del Edge, límites y rate limit por API key"
status: propuesta
date: 2026-09-29
owner: Lucia Scharff
last_reviewed: 2026-09-29
supersedes: [CLOUD-ADR-004]
superseded_by: []
---

# CLOUD-ADR-018 — Ingesta HTTP batch con contrato congelado del Edge, límites y rate limit por API key

> ADR retrospectivo. Reemplaza por completo a
> [CLOUD-ADR-004](CLOUD-ADR-004-ingesta-http-batch.md): repite lo que sigue vigente y
> corrige lo que se implementó distinto. Fuente: diseño de la ingesta
> (`embolsadora-edge/docs/superpowers/specs/2026-07-30-cloud-ingest-endpoint-design.md`),
> PR #56.

## Contexto

CLOUD-ADR-004 (2025-10-17) fijó la ingesta como HTTP batch con idempotencia y rate limit.
La forma general se mantuvo, pero tres puntos se implementaron distinto:

| CLOUD-ADR-004 | Implementado | Por qué |
|---|---|---|
| Tamaño máximo 2 MB | Tope de lectura de **4 MiB** (`INGEST_MAX_BODY_BYTES`) | El Edge corta sus batches en `BATCH_MAX_BYTES=2097152` exactos. Rechazar desde 2 MiB mandaría a DEAD hasta 1000 eventos válidos por un byte de diferencia (§9.2 del diseño) |
| `Idempotency-Key` por batch con TTL de 10 min | Se acepta y se loguea, pero **no decide nada** | El índice único ya da la garantía; una caché en Redis sería una segunda fuente de verdad (D-6) |
| `eventId` único global | Único **por tenant**: índice `(tenantId, eventId)` | `eventId` no incluye el tenant; ver [CLOUD-ADR-017](CLOUD-ADR-017-mediciones-en-mongodb.md) |

Además, cuando se diseñó la ingesta el Edge Pi Service **ya estaba enviando** con un
contrato HTTP congelado (`embolsadora-edge/specs/002-forwarder-influx/contracts/outbound-events.openapi.yaml`),
y reacciona de forma distinta a cada código de respuesta.

## Decisión

- **Protocolo:** `POST /api/v1/consumers/events`, body `{"events":[...]}` con entre 1 y
  1000 eventos. Autenticación con `X-Api-Key` (SHA-256 con prefijo `key_id`: D-3, D-4).
- **Límites:** 1000 eventos por batch como regla de negocio (`INGEST_MAX_EVENTS`);
  4 MiB solo contra abuso.
- **Rate limit:** token bucket por API key en Redis, 200 rps sostenidos y ráfaga de 1000
  (`INGEST_RATE_LIMIT_RPS`, `INGEST_RATE_LIMIT_BURST`). 429 con `Retry-After`, que el
  Edge respeta. Sin Redis, el límite falla abierto.
- **Respuesta:** `{"data":{"accepted":n,"rejected":m,"errors":[{"index","code","message"}]}}`,
  **sin** el envelope `{"success":...}` del resto de la API.
- **Códigos de error fijos:** `DUPLICATE` (el Edge marca ACKED), `INVALID_SCHEMA` y
  `VALIDATION_FAILED` (DEAD para siempre), `STORAGE_UNAVAILABLE` (se reintenta).
- **Invariantes:**
  - I-1: un error de infraestructura nunca se reporta como error de payload.
  - I-2: los errores de eventos individuales van por `200` + `errors[]`; el 400 queda
    reservado a un sobre malformado.
  - I-3: `index` es la posición en el array original, con base 0.
  - I-4: en toda respuesta 200, `accepted + rejected == len(events)`.
- **Validación:** del sobre del evento, nunca del contenido de `payload` (D-8).
- **Escritura síncrona:** `accepted` es un hecho consumado antes de responder, porque el
  Edge avanza su watermark con esa respuesta (D-5).
- **Orden:** no garantizado entre batches; se reconstruye con `ts` y `seq` (vigente
  desde CLOUD-ADR-004).

## Alternativas consideradas

Las de CLOUD-ADR-004 siguen valiendo: un broker (Kafka, MQTT) desde el inicio es
innecesario hasta validar la carga, y HTTP de un evento por request tiene demasiado
overhead. El diseño de la ingesta agrega:

1. **mTLS u OAuth2 en lugar de API key.** Descartado: el contrato ya estaba congelado en
   `X-Api-Key` y el cambio no daba una ganancia proporcional sobre HTTPS (D-3).
2. **Ingesta asíncrona** (encolar y responder). Descartada: `accepted` pasaría a ser una
   promesa y el Edge borraría datos que todavía no se guardaron (D-5).

## Consecuencias

- Cambiar la forma de la respuesta o un código de error puede provocar **pérdida
  silenciosa de datos** en el Edge. Todo cambio en esta superficie es `contract-change`
  y se coordina con `embolsadora-edge`.
- Las invariantes I-1 a I-4 tienen que estar cubiertas por tests; están en
  `internal/consumers` e `internal/app/ingest`.
- `Idempotency-Key` queda en el contrato sin efecto. Si en algún momento se lo usa, es
  un ADR nuevo.
