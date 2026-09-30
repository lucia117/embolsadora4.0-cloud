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
supersedes: [005b]
traducido: 2026-09-29
last_reviewed: 2026-09-29
---

# 022 — Endpoint de ingesta del cloud (Edge Pi → Cloud)

> **Procedencia.** Los requisitos `origen: original` están escritos en el diseño aprobado
> del 2026-07-30, que vive en `embolsadora-edge` (ver `spec_externa`); se cita la sección
> (`§N`) o la decisión (`D-N`). Los `origen: derivado` se reconstruyeron el 2026-09-29
> leyendo el código del PR #56 y los fixes posteriores. **Cada requisito se verificó contra
> el código de `develop`** el 2026-09-29. El plan de implementación está en
> [`plan.md`](plan.md).

## Contexto y problema

El Edge Pi Service de cada planta lee mediciones de una InfluxDB local, las mapea contra
el catálogo AAS, las guarda en un outbox y las envía en batches a
`POST /api/v1/consumers/events`, con reintentos, backoff e idempotencia. Cuando se
escribió el diseño, el Edge ya estaba enviando tráfico real, y del lado del cloud el
handler respondía 501: no había tabla de API keys, ni almacenamiento de mediciones, ni
MongoDB.

La restricción que domina todo el diseño es que **el contrato HTTP ya estaba congelado**
del lado del Edge (`embolsadora-edge/specs/002-forwarder-influx/contracts/outbound-events.openapi.yaml`),
y el Edge reacciona distinto a cada respuesta (`embolsadora-edge/internal/app/forwarder/errors.go`):

| Respuesta del cloud | Qué hace el Edge |
|---|---|
| `errors[].code = DUPLICATE` | Marca el evento ACKED |
| `INVALID_SCHEMA` o `VALIDATION_FAILED` | Marca el evento **DEAD, para siempre** |
| Cualquier otro código | Reintenta con backoff |
| HTTP 400 | **Manda el batch entero a DEAD** |
| HTTP 403, 429 o 5xx | Reintenta (el 429 respetando `Retry-After`) |

Por eso cualquier respuesta equivocada produce **pérdida silenciosa de datos**: el Edge
borra de su outbox eventos que el cloud nunca guardó. Los requisitos RF-015 a RF-018 (las
invariantes I-1 a I-4 del diseño) existen para impedirlo.

## Alcance

**Entra:**
- El endpoint `POST /api/v1/consumers/events` completo.
- Autenticación por API key: tabla, hash, rotación y revocación (migración `000014`), y
  los endpoints del ABM para gestionarlas.
- Rate limit por API key.
- Persistencia idempotente en MongoDB, con los índices que necesitan los lectores.
- Provisioning de MongoDB: driver, configuración y servicio de desarrollo.

**No entra** (§2):
- Endpoints de lectura para el frontend. Los resuelve la spec [024](../024-dashboard-metrics-query/spec.md).
- **Motor de evaluación de reglas de alarma.** El diseño lo excluye explícitamente; ver la
  [bitácora de alcance](../../docs/_process/bitacora-de-alcance.md#objetivo-específico-5-alertas-automáticas).
- `POST /api/v1/consumers/heartbeat`: sigue respondiendo 501.
- Agregaciones, rollups, downsampling y retención con TTL.
- Caché de idempotencia por batch en Redis (D-6).

## Requisitos

### Contrato y autenticación

- **RF-001** `origen: original` (§3.1) — El sistema DEBE aceptar
  `POST /api/v1/consumers/events` con body `{"events":[...]}` de entre 1 y 1000 eventos.
  Un body sin `events`, con `events` vacío o con más de `INGEST_MAX_EVENTS` elementos se
  rechaza con 400.
  → `internal/consumers/events_handler.go`, `internal/consumers/router.go`.
- **RF-002** `origen: original` (§6.1, D-3, D-4) — El sistema DEBE autenticar cada
  request con el header `X-Api-Key` en formato `emb_<key_id>_<secreto>`: busca por
  `key_id`, compara `sha256(secreto)` en tiempo constante, y exige que la key no esté
  revocada ni vencida y que el device esté `ACTIVE`. Cualquier falla responde **403,
  nunca 401**. El secreto en claro no se guarda nunca.
  → `internal/domain/apikeys/keygen.go` (`subtle.ConstantTimeCompare`),
  `internal/security/apikeys_authenticator.go`, `internal/consumers/middleware/middleware.go`,
  `migrations/000014_edge_device_api_keys.up.sql`.
- **RF-003** `origen: original` (D-10, §6.2) — El sistema DEBE tomar `tenantId` y
  `deviceId` de la API key resuelta, nunca del body. Un evento cuyo `machineId` no es el
  del device de la key se rechaza con `VALIDATION_FAILED` y no se escribe.
  → `internal/app/ingest/validate.go` (comparación con `dev.MachineID`).
- **RF-004** `origen: original` (§6.1) — El sistema DEBE cachear en Redis la key resuelta
  (`APIKEY_CACHE_TTL`, 60 s por defecto) e invalidar esa entrada al revocar la key.
  → `security.APIKeyAuthenticator.lookup`, `security.InvalidateAPIKeyCache`, llamado
  desde `internal/app/edge_devices/service.go`.
- **RF-005** `origen: original` (§6.1, §8) — El sistema DEBE actualizar
  `edge_device_api_keys.last_used_at` como máximo una vez por minuto por key, y
  `edge_devices.last_seen_at` de forma best-effort después de persistir el batch, sin
  afectar la respuesta.
  → `APIKeyAuthenticator.touch` (`SetNX` con TTL de un minuto),
  `ingest.Service.recordActivity`.
- **RF-006** `origen: original` (§8) — El sistema DEBE limitar la tasa con un token
  bucket por `key_id` en Redis (`INGEST_RATE_LIMIT_RPS` = 200, `INGEST_RATE_LIMIT_BURST`
  = 1000) y responder 429 con `Retry-After` al exceder el límite.
  → `internal/consumers/ratelimit.go`, `consumers/middleware.RateLimit`.
- **RF-007** `origen: derivado` — Sin Redis (no configurado o inalcanzable), el rate limit
  y la caché de keys DEBEN fallar abiertos: la ingesta sigue funcionando sin límite.
  → `consumers.NewRateLimiter` (`rdb == nil`), test `TestRateLimiterFailsOpenWithoutRedis`.
  El diseño no lo menciona; es la política general del repo (ver
  [`docs/architecture.md`](../../docs/architecture.md#degradación-controlada)).
- **RF-008** `origen: original` (§8, §9.2) — El sistema DEBE leer el body con un tope
  duro (`INGEST_MAX_BODY_BYTES`, 4 MiB) antes de parsearlo, y responder 400 si lo supera.
  El tope es de 4 MiB y no de 2 porque el Edge corta sus batches en 2 MiB exactos.
  → `http.MaxBytesReader` en `events_handler.go`.

### Validación

- **RF-009** `origen: original` (§8) — El sistema DEBE validar el sobre de cada evento por
  separado, sin abortar el batch. Un evento es `INVALID_SCHEMA` si le falta `eventId`,
  `machineId`, `ts`, `kind`, `schemaVersion` o `payload`; si `ts` no es RFC3339; si
  `schemaVersion` no es un entero ≥ 1; si `payload` no es un objeto; o si `seq` viene y no
  es un entero ≥ 0.
  → `ingest.ValidateEvent`.
- **RF-010** `origen: derivado` — **Diverge del diseño.** Un evento con `kind` fuera de
  `{metric, alarm, heartbeat}`, o con `schemaVersion` mayor a `MaxSchemaVersion` (hoy 1),
  DEBE **aceptarse y persistirse**, incrementando `ingest_version_skew_total{reason}` y
  dejando un log `warn`. El diseño (§8, §9) pedía rechazarlos con `INVALID_SCHEMA` y
  `VALIDATION_FAILED`. Se cambió porque rechazar manda el evento a DEAD para siempre la
  primera vez que el Edge hable una versión nueva antes que el cloud desplegado; aceptar
  de más es reversible y rechazar de más no.
  → `internal/domain/ingest/measurement.go` (comentarios de `Kinds` y
  `MaxSchemaVersion`), `ingest.Service.IngestBatch`.
- **RF-011** `origen: original` (D-8) — El sistema NO DEBE validar el contenido de
  `payload`: se guarda tal cual llega, incluido `aasPath` si viene (D-7).
  → `ingest.ValidateEvent`.

### Persistencia e idempotencia

- **RF-012** `origen: original` (§6.2, D-1) — El sistema DEBE persistir cada evento
  válido en la colección `measurements` de MongoDB con `eventId`, `tenantId`, `deviceId`,
  `machineId`, `ts`, `seq`, `kind`, `schemaVersion`, `payload` y `receivedAt` (hora del
  servidor).
  → `internal/domain/ingest/measurement.go`, `internal/repo/mongo/measurements/`.
- **RF-013** `origen: original` con corrección `derivado` (D-5) — La idempotencia DEBE
  garantizarla un índice único en MongoDB, no lógica de aplicación, y un duplicado se
  reporta como `DUPLICATE` dentro de un 200. **Corrección:** el índice es
  `(tenantId, eventId)` (`uq_tenant_eventId`) y no `eventId` solo, como decía el diseño,
  porque `eventId` no incluye el tenant y dos tenants con el mismo `machineId` colisionaban.
  → `measurements.Repository.EnsureIndexes`, que además borra el índice viejo `uq_eventId`.
- **RF-014** `origen: original` (D-5) — La escritura DEBE ser síncrona, con
  `insertMany(ordered:false)`: `accepted` es un hecho consumado antes de responder, porque
  el Edge avanza su watermark con esa respuesta.
  → `measurements.Repository.InsertMany`.
- **RF-025** `origen: original` (§6.3) — El sistema DEBE crear al arrancar los índices
  que sirven a los lectores: `(tenantId, machineId, payload.aasPath, ts desc)` sparse y
  `(tenantId, ts desc)`.
  → `EnsureIndexes` (`ix_tenant_machine_path_ts`, `ix_tenant_ts`).

### Invariantes de respuesta

- **RF-015** `origen: original` (I-1) — Un error de infraestructura NUNCA DEBE reportarse
  como error de payload. Si Mongo falla, la respuesta es `STORAGE_UNAVAILABLE` o HTTP 500,
  nunca `INVALID_SCHEMA`, `VALIDATION_FAILED` ni 400. Aplica también a la autenticación:
  si falla el lookup de la key por infraestructura, responde 500 y no 403.
  → `ingest.Service.IngestBatch`, `consumers/middleware.APIKeyAuth`.
- **RF-016** `origen: original` (I-2) — Los problemas de eventos individuales DEBEN
  viajar como `200` + `errors[]`; el 400 queda reservado a un sobre malformado.
- **RF-017** `origen: original` (I-3) — `errors[].index` DEBE ser la posición, con base 0,
  en el array `events` original.
  → `origIndex` en `ingest.Service.IngestBatch`.
- **RF-018** `origen: original` (I-4) — En toda respuesta 200 DEBE cumplirse
  `accepted + rejected == len(events)`.
- **RF-019** `origen: original` (§9) — Si falla la escritura de algunos eventos dentro de
  un `insertMany` parcialmente exitoso, esos eventos DEBEN reportarse como
  `STORAGE_UNAVAILABLE`. Si falla la escritura entera, la respuesta DEBE ser HTTP 500.
- **RF-020** `origen: derivado` — Cada `InsertMany` DEBE estar acotado por
  `MONGO_TIMEOUT`, de modo que un primario de Mongo colgado (no caído) termine en 500 y no
  en un request colgado.
  → `mongoTimeout` en `ingest.Service`.
- **RF-021** `origen: derivado` — Si Mongo es inalcanzable al arrancar, el proceso NO
  DEBE caerse: la ingesta responde 500 en cada request y el resto de la API sigue. Si Mongo
  conecta pero falla la creación del índice único, el proceso DEBE terminar.
  → `connectMeasurementsRepo` en `internal/routes/url_mappings.go`,
  `measurements.Unavailable`.
- **RF-022** `origen: original` (§3.1) — La respuesta 200 DEBE ser
  `{"data":{"accepted":n,"rejected":m,"errors":[{"index","code","message"}]}}`, **sin**
  el envelope `{"success":...}` del resto de la API.
  → `internal/consumers/dto/events.go`.
- **RF-023** `origen: original` (D-6) — El header `Idempotency-Key` DEBE aceptarse y
  loguearse, pero no decide nada.
  → `events_handler.go`.

### Gestión de API keys (ABM)

- **RF-024** `origen: original` (§2, §6.1) — El ABM DEBE permitir crear, listar y
  revocar API keys de un edge device. El valor completo de la key se muestra una sola vez,
  al crearla, y un device puede tener varias keys activas para rotar sin downtime.
  → `POST|GET /api/v1/tenants/:tenantId/edge-devices/:deviceId/api-keys`,
  `DELETE …/api-keys/:keyId` (`internal/api/handler/edge_devices/routes.go`).

### No funcionales

- **RNF-001** `origen: original` (§10) — El sistema DEBE exponer métricas Prometheus de la
  ingesta (`ingest_batches_total`, `ingest_events_accepted_total`,
  `ingest_events_rejected_total`, `ingest_auth_total`, `ingest_rate_limited_total`,
  `ingest_batch_duration_seconds`, `ingest_mongo_up`), y `GET /health` DEBE incluir el
  estado de Mongo.
  → `internal/telemetry/metrics.go`, `/health` en `url_mappings.go`.
- **RNF-002** `origen: original` (D-9) — Las mediciones se retienen indefinidamente, sin
  índice TTL.
- **RNF-003** `origen: original` (§12, SC-008) — Un batch de 1000 eventos DEBE procesarse
  con latencia p95 menor a 500 ms (ver CE-008).

## Criterios de éxito

Son los SC-001 a SC-009 del diseño, con la misma numeración. Todos tienen un test que los
verifica. Los de `ingest_integration_test.go` necesitan `MONGO_URI` y `DATABASE_URL`.

| CE | Criterio | Test |
|---|---|---|
| **CE-001** | Un batch de 108 eventos reales responde `accepted: 108, rejected: 0` y deja 108 documentos | `consumers.TestSC001CleanBatch` |
| **CE-002** | Reenviar el mismo batch responde `accepted: 0, rejected: 108`, todos `DUPLICATE`, y sigue habiendo 108 documentos | `consumers.TestSC002ReplayIsIdempotent` |
| **CE-003** | `[válido, válido, sin ts, válido, ya existente]` responde `accepted: 3, rejected: 2`, con índices 2 y 4 | `consumers.TestSC003MixedBatchReportsOriginalIndices`, `ingest/service_test.go` |
| **CE-004** | Con Mongo caído, ninguna respuesta contiene `INVALID_SCHEMA`, `VALIDATION_FAILED` ni 400; al volver, el reintento persiste sin duplicar | `consumers.TestSC004StorageDownNeverReportsPayloadErrors`, `TestSC004RetryAfterRecoveryPersistsWithoutDuplicates` |
| **CE-005** | Una key revocada devuelve 403 | `security/apikeys_authenticator_test.go` (caso "key revocada"), `TestAPIKeyAuthInvalidKeyReturns403` |
| **CE-006** | Superar el rate limit devuelve 429 con `Retry-After` | `middleware.TestSC006RateLimitReturns429WithRetryAfter` |
| **CE-007** | Un batch con `machineId` ajeno responde `VALIDATION_FAILED` y escribe cero documentos | `consumers.TestSC007ForeignMachineIDWritesNothing` |
| **CE-008** | 1000 eventos con p95 < 500 ms | `consumers.TestSC008ThousandEventsLatency` |
| **CE-009** | "Último valor de una propiedad" resuelve por `IXSCAN`, sin `COLLSCAN` | `consumers.TestSC009LatestValueQueryUsesIndex` |

## Decisiones

Las decisiones difíciles de revertir están en ADRs:

- [CLOUD-ADR-017](../../docs/adr/CLOUD-ADR-017-mediciones-en-mongodb.md): mediciones en
  MongoDB, identidad en Postgres, idempotencia por índice único y sin TTL (D-1, D-2, D-5,
  D-9).
- [CLOUD-ADR-018](../../docs/adr/CLOUD-ADR-018-ingesta-http-batch.md): contrato, códigos,
  invariantes, límites y rate limit (D-3, D-4, D-6, D-8).

Decisiones de implementación que no llegan a ADR:

| Decisión | Alternativa descartada | Por qué |
|---|---|---|
| SHA-256 para el hash de keys (D-4) | bcrypt o argon2 | Las keys son secretos aleatorios de alta entropía; la lentitud deliberada de bcrypt es inviable a 200 rps |
| `aasPath` dentro de `payload` (D-7) | Campo de primer nivel en el sobre | `payload` es libre en el contrato: no obliga a versionar el schema ni a coordinar el deploy |
| Aceptar `kind` y `schemaVersion` desconocidos (RF-010) | Rechazarlos, como pedía el diseño | Rechazar es irreversible del lado del Edge |
| `last_used_at` diferido a uno por minuto (RF-005) | UPDATE en cada request | A 200 rps serían 200 UPDATE por segundo sobre la misma fila |

## Riesgos y preguntas abiertas

- **Crecimiento sin límite de `measurements`** (RNF-002). Agregar un TTL o rollups después
  no es destructivo, pero hoy nadie lo monitorea.
- **`Idempotency-Key` no tiene efecto** (RF-023). Si aparecen reintentos por timeout que
  generan trabajo redundante contra Mongo, el diseño prevé una caché de respuestas en
  Redis (§13).
- **Heartbeat** sigue respondiendo 501. El Edge no lo usa hoy (`KindHeartbeat` está
  reservado), pero está en el contrato.
- **Sin motor de evaluación**, las mediciones con `kind: alarm` se guardan pero no generan
  notificaciones. Ver el objetivo #5 en la bitácora de alcance.
- **Referencia a casos de uso del hub** (`UC-xx`): pendiente hasta que exista
  `embolsadora-docs`.

## Reemplaza

A [`005b-plc-events`](../005b-plc-events/spec.md) (veredicto B). Ver la
[bitácora de alcance](../../docs/_process/bitacora-de-alcance.md#005b-plc-events).
