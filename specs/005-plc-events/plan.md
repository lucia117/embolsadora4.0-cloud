# Technical Plan: PLC Events Ingestion & Query API

**Branch**: `005-plc-events` | **Date**: 2026-03-24 | **Spec**: [spec.md](./spec.md)

---

## Summary

Implementar un sistema de ingestión y consulta de eventos de PLC pre-procesados. La Processing API externa envía batches de eventos (alarmas, mediciones, estados) a un endpoint de esta cloud API. Los eventos se almacenan en **MongoDB** y se exponen a través de endpoints de consulta consumidos por los widgets del dashboard del frontend.

---

## Arquitectura

```
Processing API (externo)
 │
 │ POST /api/tenants/:tenantId/edge-devices/:deviceId/plc-events
 │ Authorization: Bearer <JWT>
 │ Body: { events: [...] }  ← batch, máx 1000
 ▼
internal/api/handler/plc_events/ingest.go
 │
 ▼
internal/app/plc_events/service.go
 ├── Validar deviceId pertenece al tenant (query PostgreSQL)
 ├── Deduplicar por externalId (query MongoDB)
 └── Insertar batch en MongoDB (insertMany)
 ▼
MongoDB — colección plc_events
 ▼
GET /api/tenants/:tenantId/edge-devices/:deviceId/plc-events?filters
GET /api/tenants/:tenantId/edge-devices/:deviceId/plc-events/latest
 ▼
Frontend Dashboard — widgets
```

---

## Technical Context

**Language/Version**: Go 1.24 + Gin
**Storage**: MongoDB 7 (`go.mongodb.org/mongo-driver/v2`) — nueva dependencia
**Auth**: JWT Bearer existente (`JWTAuth` middleware)
**Tenant resolution**: `ResolveTenantFromPath` (existente, del módulo edge_devices)
**Logging/Metrics**: Zap + Prometheus (existentes)
**Testing**: testify + uber/mock

---

## API Contract

### Prefijo de rutas

```
/api/tenants/:tenantId/edge-devices/:deviceId/plc-events
```

Reutiliza el grupo `/api/tenants` ya registrado en `url_mappings.go` (creado para edge-devices).

### Endpoints

#### `POST .../plc-events` — Ingestión batch

```
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "events": [
    {
      "eventType": "ALARM",
      "timestamp": "2026-03-24T10:32:00Z",
      "externalId": "plc-001-alarm-4521",        // opcional, para deduplicación
      "tags": { "station": "filling", "line": "L1" },
      "alarm": {
        "code": "E_OVERFLOW",
        "message": "Overflow en estación de llenado",
        "severity": "HIGH",
        "status": "ACTIVE"
      }
    },
    {
      "eventType": "MEASUREMENT",
      "timestamp": "2026-03-24T10:32:01Z",
      "tags": { "station": "weighing" },
      "measurement": {
        "name": "bag_weight",
        "value": 25.3,
        "unit": "kg"
      }
    },
    {
      "eventType": "STATE",
      "timestamp": "2026-03-24T10:32:02Z",
      "tags": { "line": "L1" },
      "state": {
        "name": "machine_mode",
        "previousValue": "IDLE",
        "currentValue": "RUNNING"
      }
    }
  ]
}
```

**Response 201**:
```json
{ "success": true, "data": { "inserted": 3, "deduplicated": 0 } }
```

**Response 400**:
```json
{ "success": false, "error": "event[1].measurement.name is required", "code": "VALIDATION_ERROR" }
```

---

#### `GET .../plc-events` — Consulta con filtros

```
Query params:
  eventType   string    ALARM | MEASUREMENT | STATE
  tags        string    key:value (repetible: ?tags=station:filling&tags=line:L1)
  from        string    ISO 8601 (ej: 2026-03-24T00:00:00Z)
  to          string    ISO 8601
  status      string    ACTIVE | RESOLVED  (solo para ALARM)
  measurement string    nombre de medición (solo para MEASUREMENT)
  limit       int       default 100, max 1000
  cursor      string    cursor de paginación (opaco, base64 del last _id)
```

**Response 200**:
```json
{
  "success": true,
  "data": {
    "events": [
      {
        "id": "...",
        "deviceId": "...",
        "tenantId": "...",
        "eventType": "ALARM",
        "timestamp": "2026-03-24T10:32:00Z",
        "receivedAt": "2026-03-24T10:32:00.123Z",
        "tags": { "station": "filling", "line": "L1" },
        "alarm": {
          "code": "E_OVERFLOW",
          "message": "Overflow en estación de llenado",
          "severity": "HIGH",
          "status": "ACTIVE"
        }
      }
    ],
    "cursor": "base64encodedId",
    "hasMore": true
  }
}
```

---

#### `GET .../plc-events/latest` — Último valor por measurement

```
Query params:
  measurement  string  (opcional) filtrar por nombre de medición
```

**Response 200 — con filtro** (`?measurement=bag_weight`):
```json
{
  "success": true,
  "data": {
    "measurement": "bag_weight",
    "value": 25.3,
    "unit": "kg",
    "timestamp": "2026-03-24T10:32:01Z",
    "receivedAt": "2026-03-24T10:32:01.089Z",
    "tags": { "station": "weighing" }
  }
}
```

**Response 200 — sin filtro** (último de cada measurement):
```json
{
  "success": true,
  "data": {
    "bag_weight":      { "value": 25.3, "unit": "kg",  "timestamp": "..." },
    "fill_level":      { "value": 72.5, "unit": "%",   "timestamp": "..." },
    "conveyor_speed":  { "value": 0.8,  "unit": "m/s", "timestamp": "..." }
  }
}
```

---

#### `PATCH .../plc-events/:eventId/tags` — Agregar tags

```json
{ "tags": { "cause": "sensor-drift", "operator": "jperez" } }
```

**Response 200**:
```json
{ "success": true, "data": { "id": "...", "tags": { "station": "filling", "cause": "sensor-drift", "operator": "jperez" } } }
```

---

## Data Model (MongoDB)

### Colección `plc_events`

```json
{
  "_id": "ObjectId",
  "tenantId": "uuid-string",
  "deviceId": "uuid-string",
  "eventType": "ALARM",
  "timestamp": "2026-03-24T10:32:00.000Z",
  "receivedAt": "2026-03-24T10:32:00.123Z",
  "externalId": "plc-001-alarm-4521",
  "tags": {
    "station": "filling",
    "line": "L1"
  },
  "alarm": {
    "code": "E_OVERFLOW",
    "message": "Overflow en estación de llenado",
    "severity": "HIGH",
    "status": "ACTIVE"
  }
}
```

Para `MEASUREMENT`, el campo `measurement` reemplaza a `alarm`:
```json
{
  "measurement": {
    "name": "bag_weight",
    "value": 25.3,
    "unit": "kg"
  }
}
```

Para `STATE`, el campo `state` reemplaza a `alarm`:
```json
{
  "state": {
    "name": "machine_mode",
    "previousValue": "IDLE",
    "currentValue": "RUNNING"
  }
}
```

### Índices MongoDB

```javascript
// Consulta principal: eventos de un device en orden temporal
db.plc_events.createIndex({ tenantId: 1, deviceId: 1, timestamp: -1 })

// Filtro por tipo + tenant + device
db.plc_events.createIndex({ tenantId: 1, deviceId: 1, eventType: 1, timestamp: -1 })

// Filtro por measurement (para /latest y filtros de medición)
db.plc_events.createIndex(
  { tenantId: 1, deviceId: 1, "measurement.name": 1, timestamp: -1 },
  { sparse: true }
)

// Filtro por status de alarma
db.plc_events.createIndex(
  { tenantId: 1, deviceId: 1, "alarm.status": 1, timestamp: -1 },
  { sparse: true }
)

// Deduplicación por externalId
db.plc_events.createIndex(
  { tenantId: 1, deviceId: 1, externalId: 1 },
  { unique: true, sparse: true }  // sparse: externalId es opcional
)

// TTL: retención 90 días
db.plc_events.createIndex({ receivedAt: 1 }, { expireAfterSeconds: 7776000 })
```

---

## Source Code Layout

```
internal/
├── api/
│   └── handler/
│       └── plc_events/
│           ├── dto/
│           │   └── dto.go              # Request/response structs
│           ├── ingest.go               # POST .../plc-events
│           ├── list_events.go          # GET .../plc-events
│           ├── latest_values.go        # GET .../plc-events/latest
│           ├── patch_tags.go           # PATCH .../plc-events/:eventId/tags
│           └── routes.go              # RegisterPLCEventRoutes(group, service)
├── app/
│   └── plc_events/
│       └── service.go                 # Lógica de negocio + orquestación
├── domain/
│   └── plc_events/
│       ├── event.go                   # PLCEvent, Alarm, Measurement, StateChange
│       ├── filters.go                 # EventFilter struct (query params)
│       ├── errors.go                  # ErrDeviceMismatch, ErrBatchTooLarge, etc.
│       └── repository.go              # Interface Repository
└── repo/
    └── mongo/
        ├── client.go                  # MongoDB client (compartido con futuro AAS)
        └── plc_events/
            └── repository.go          # Implementación MongoDB
```

---

## Fases de implementación

### Fase 1 — MongoDB client + config

**Archivos**:
- `internal/repo/mongo/client.go` — `NewMongoClient(uri, dbName)`
- `internal/config/config.go` — agregar `MongoConfig{ URI, DBName }`
- `cmd/api/main.go` — inicializar cliente MongoDB al startup (fallar silenciosamente si MONGO_URI no está seteado, como Redis)
- `docker-compose.yml` — agregar servicio `mongo:7`

**Variables de entorno**:
```
MONGO_URI=mongodb://embolsadora_user:embolsadora_password@mongo:27017
MONGO_DB=embolsadora_dev
```

**Fail-open**: Si `MONGO_URI` no está configurado, el cliente es nil. Los handlers de PLC events retornan 503 si el cliente es nil (misma estrategia que Redis en rate limiting).

---

### Fase 2 — Domain layer

**`internal/domain/plc_events/event.go`**:

```go
type EventType string
const (
    EventTypeAlarm       EventType = "ALARM"
    EventTypeMeasurement EventType = "MEASUREMENT"
    EventTypeState       EventType = "STATE"
)

type Severity string
const (
    SeverityCritical Severity = "CRITICAL"
    SeverityHigh     Severity = "HIGH"
    SeverityMedium   Severity = "MEDIUM"
    SeverityLow      Severity = "LOW"
)

type AlarmStatus string
const (
    AlarmStatusActive   AlarmStatus = "ACTIVE"
    AlarmStatusResolved AlarmStatus = "RESOLVED"
)

type PLCEvent struct {
    ID         string
    TenantID   string
    DeviceID   string
    EventType  EventType
    Timestamp  time.Time
    ReceivedAt time.Time
    ExternalID *string
    Tags       map[string]string

    // Solo uno de estos estará poblado según EventType
    Alarm       *AlarmPayload
    Measurement *MeasurementPayload
    State       *StatePayload
}

type AlarmPayload struct {
    Code     string
    Message  string
    Severity Severity
    Status   AlarmStatus
}

type MeasurementPayload struct {
    Name  string
    Value *float64
    Unit  string
}

type StatePayload struct {
    Name          string
    PreviousValue string
    CurrentValue  string
}
```

**`internal/domain/plc_events/filters.go`**:

```go
type EventFilter struct {
    EventType   *EventType
    Tags        map[string]string  // AND entre todos los tags
    From        *time.Time
    To          *time.Time
    Status      *AlarmStatus       // solo para ALARM
    Measurement *string            // solo para MEASUREMENT
    Limit       int                // default 100, max 1000
    Cursor      *string            // cursor opaco
}
```

**`internal/domain/plc_events/repository.go`**:

```go
type Repository interface {
    InsertBatch(ctx context.Context, events []*PLCEvent) (inserted int, deduplicated int, err error)
    List(ctx context.Context, tenantID, deviceID string, filter EventFilter) ([]*PLCEvent, string, error)
    LatestByMeasurement(ctx context.Context, tenantID, deviceID string, measurement *string) (map[string]*PLCEvent, error)
    PatchTags(ctx context.Context, tenantID, deviceID, eventID string, tags map[string]string) (*PLCEvent, error)
}
```

---

### Fase 3 — MongoDB repository

**`internal/repo/mongo/plc_events/repository.go`**:

**`InsertBatch`**:
1. Construir slice de documentos BSON
2. Para eventos con `externalId`: `ordered: false` insert con manejo de `E11000` (duplicate key) → contar como deduplicated
3. Para eventos sin `externalId`: insert normal
4. Retornar `(inserted, deduplicated, nil)`

**`List`**:
1. Construir filtro BSON desde `EventFilter`
2. Tags: `{ "tags.station": "filling", "tags.line": "L1" }` (punto-notación en MongoDB)
3. Cursor: decodificar base64 → `_id` del último doc → agregar `{ _id: { $lt: lastId } }` al filtro
4. `Find` con `Sort({ timestamp: -1 })`, `Limit(filter.Limit + 1)` para detectar `hasMore`
5. Si hay `Limit+1` resultados: `hasMore = true`, encodear `_id` del último como cursor

**`LatestByMeasurement`**:
1. Aggregation pipeline:
```
{ $match: { tenantId, deviceId, eventType: "MEASUREMENT", [measurement si aplica] } }
{ $sort:  { "measurement.name": 1, timestamp: -1 } }
{ $group: { _id: "$measurement.name", doc: { $first: "$$ROOT" } } }
```
2. Retornar map `measurement.name → *PLCEvent`

---

### Fase 4 — Application service

**`internal/app/plc_events/service.go`**:

```go
type PLCEventService struct {
    repo      plc_events.Repository
    deviceRepo edge_devices.Repository  // para validar deviceId pertenece al tenant
    logger    *zap.Logger
}
```

**`IngestBatch`**:
1. Validar len(events) > 0 y <= 1000
2. Validar cada evento (tipo requerido, campos del subtipo requeridos)
3. Verificar que `deviceID` pertenece al `tenantID` via `deviceRepo.GetByID`
4. Setear `receivedAt = time.Now()` y `tenantID` a todos los eventos
5. Llamar `repo.InsertBatch`
6. Registrar métricas

---

### Fase 5 — Handlers HTTP

**`internal/api/handler/plc_events/ingest.go`**:
- Bind JSON del body
- Llamar service
- Mapear errores de dominio a HTTP

**Error mapping**:

| Error | HTTP |
|---|---|
| `ErrBatchEmpty` | 400 |
| `ErrBatchTooLarge` | 400 `BATCH_TOO_LARGE` |
| `ErrValidation` | 400 `VALIDATION_ERROR` |
| `ErrDeviceNotFound` | 404 |
| `ErrDeviceMismatch` | 403 |
| MongoDB nil | 503 |

---

### Fase 6 — Router integration

**`internal/routes/url_mappings.go`** — agregar rutas al grupo `/api/tenants` existente:

```go
// Reutiliza el grupo ya creado para edge-devices
// deviceGroup es el grupo /api/tenants/:tenantId/edge-devices/:deviceId
plcEvents.RegisterPLCEventRoutes(deviceGroup, plcEventsDeps)
```

---

### Fase 7 — Widget config contract (coordinación con frontend)

El campo `config` de `WidgetConfig` en el frontend almacenará la configuración del datasource:

```typescript
// Widget config para un gauge de medición
{
  "type": "measurement-gauge",
  "config": {
    "deviceId": "uuid-del-edge-device",
    "measurement": "bag_weight",
    "unit": "kg",
    "min": 0,
    "max": 50,
    "thresholds": [{ "value": 45, "color": "yellow" }, { "value": 49, "color": "red" }]
  }
}

// Widget config para lista de alarmas activas
{
  "type": "alarm-list",
  "config": {
    "deviceId": "uuid-del-edge-device",
    "tags": { "station": "filling" },
    "severity": ["CRITICAL", "HIGH"],
    "limit": 10
  }
}

// Widget config para gráfico de historial
{
  "type": "measurement-chart",
  "config": {
    "deviceId": "uuid-del-edge-device",
    "measurements": ["bag_weight", "fill_level"],
    "timeRange": "1h"
  }
}
```

**Este contrato es orientativo** — el frontend necesita adaptarlo según los widgets existentes en `007-dashboard-widget-refactor`. La estructura de `config` se define entre frontend y backend.

---

### Fase 8 — Observabilidad

**Métricas Prometheus**:
```
plc_events_ingested_total{tenant, device, event_type}   — contador de eventos ingeridos
plc_events_deduplicated_total{tenant, device}           — eventos deduplicados
plc_events_batch_size_histogram                         — distribución de tamaño de batches
plc_events_ingest_duration_seconds                      — latencia de ingestión
plc_events_query_duration_seconds{operation}            — latencia de consultas
plc_events_stale_total                                  — eventos con timestamp > 30 días
```

**Logging** (Zap):
```json
{ "msg": "plc_events_ingested", "tenant_id": "...", "device_id": "...", "inserted": 100, "deduplicated": 0, "duration_ms": 45 }
```

---

### Fase 9 — Tests

**Unit tests** (uber/mock):
- `app/plc_events/service_test.go` — mock repo + mock deviceRepo
- Casos: batch válido, batch vacío, batch too large, deduplicación, device no pertenece al tenant

**Integration tests** (Docker MongoDB):
- `tests/integration/plc_events/` — MongoDB real
- Casos: ingestión y consulta con filtros combinados, deduplicación por externalId, /latest aggregation, cursor pagination

---

## Constitution Check

| Principio | Estado | Nota |
|---|---|---|
| I — Hexagonal | ✅ | `transport → app → domain ← repo/mongo` |
| II — Seguridad / Tenant Isolation | ✅ | JWT + deviceId validado contra tenant en cada operación |
| III — Observabilidad | ✅ | Zap + Prometheus en Fase 8 |
| IV — Integration Tests | ✅ | Docker MongoDB en Fase 9 |
| V — Versioning | ✅ | Nuevos endpoints = MINOR bump |

---

## Decisiones de diseño

| Decisión | Justificación | Trade-off |
|---|---|---|
| **MongoDB para PLC events** | Schema flexible (alarm/measurement/state tienen payloads distintos); índices eficientes para consultas por tags + tiempo | Nueva infra; PostgreSQL JSONB sería alternativa pero menos ergonómica para este patrón |
| **Tags como `map[string]string`** | Máxima flexibilidad para el PLC; el frontend filtra por key:value | No hay schema fijo de tags; depende de la Processing API para consistencia |
| **TTL de 90 días** | Balance entre auditabilidad y costo de storage en MongoDB | Series de alta frecuencia del PLC permanecen en InfluxDB; aquí solo van eventos procesados |
| **Fail-open si MongoDB es nil** | Consistente con estrategia Redis del proyecto | PLC events dejan de funcionar si MongoDB no está disponible (503 explícito) |
| **Batch fail-fast** | Simplifica el manejo de errores en la Processing API; evita inserciones parciales difíciles de rastrear | La Processing API debe reenviar el batch completo si falla |
| **Cursor por `_id`** | Eficiente en MongoDB (índice en `_id`); evita el problema de skip offset en colecciones grandes | El cursor es opaco; no permite saltar a una página arbitraria |
| **Reutilizar grupo `/api/tenants`** | Consistencia con edge-devices; misma auth y resolución de tenant | Los endpoints de PLC quedan acoplados al path de edge-devices |

---

## Artifacts Summary

| Artefacto | Path |
|---|---|
| Especificación funcional | `specs/005-plc-events/spec.md` |
| Plan técnico (este archivo) | `specs/005-plc-events/plan.md` |
| Data model detallado | `specs/005-plc-events/data-model.md` (a generar) |
| Tasks | `specs/005-plc-events/tasks.md` (a generar con `/speckit.tasks`) |
| Widget config contract | Coordinar con `embolsadora-frontend/specs/007-dashboard-widget-refactor` |
