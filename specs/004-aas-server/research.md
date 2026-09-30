# Research: NoSQL + Asset Administration Shell para Embolsadora 4.0

> Última actualización: 2026-03-24 | Estado: Draft v2
> Revisión: incorpora Edge Device Management (Spec 003) y ADR-003

---

## 1. Estado Actual del Proyecto

### Stack tecnológico

| Componente | Tecnología | Estado |
|---|---|---|
| Runtime | Go 1.24 + Gin | Activo |
| Base de datos relacional | PostgreSQL 16 (pgx/v5) | Activo |
| Cache / Rate limit | Redis 7 (go-redis/v8) | Parcialmente implementado |
| Logging | Uber Zap | Activo |
| Métricas | Prometheus | Activo |
| Autenticación | Supabase Auth (JWT RS256 JWKS) | Activo |
| Admin API | Supabase Admin REST | Activo |

### Arquitectura actual

```
cmd/api/main.go
    └── internal/
         ├── api/                  → Handlers (HTTP transport)
         │   ├── handler/
         │   │   ├── edge_devices/ → 10 handlers (esqueleto completo)
         │   │   ├── invitations/  → implementado
         │   │   ├── me/           → implementado
         │   │   └── auth/         → implementado
         │   ├── usecases/         → auth, me, invitation, password
         │   └── middleware/       → JWT, RBAC, Tenant, CORS
         ├── app/
         │   ├── edge_devices/service.go
         │   └── users/service.go
         ├── consumers/            → Superficie IoT (/api/v1/consumers)
         │   ├── events_handler.go → STUB (501 not implemented)
         │   ├── heartbeat_handler.go → STUB (501 not implemented)
         │   └── router.go         → rutas definidas
         ├── domain/
         │   ├── users.go, invitation.go, errors.go, tenants.go
         │   ├── edge_devices/     → EdgeDevice, DeviceEvent, TelemetrySnapshot ← NUEVO
         │   ├── machines.go       → TODO stub ← punto de entrada para AAS
         │   └── alerts.go, events.go
         ├── repo/
         │   ├── pg/
         │   │   ├── users/, invitations/, tenants/, user_roles/ → implementados
         │   │   ├── edge_devices/repository.go → implementado (JSONB para Details)
         │   │   ├── machines_repo.go → stub vacío
         │   │   └── events_repo.go → stub
         │   └── redis/            → rate limit, idempotency (stubs)
         ├── platform/
         │   ├── edgeclient/       → cliente HTTP a Raspberry Pi / PLC ← NUEVO
         │   └── supabase/         → AdminClient (invite, password reset)
         ├── security/             → JWT, RBAC, API Keys
         └── telemetry/            → Prometheus metrics
```

### Tres superficies HTTP

```
/api/v1                         → ABM (JWT + RBAC) — users, tenants, roles, invitations
/api/tenants/:tenantId/edge-devices → Edge Device Management (JWT, tenant from path)
/api/v1/consumers               → IoT ingest (API Key + Rate limit + Idempotency) — STUBS
```

### Dominio EdgeDevice (nuevo — clave para AAS)

```go
// internal/domain/edge_devices/edge_device.go
type EdgeDevice struct {
    ID                uuid.UUID
    TenantID          uuid.UUID
    Name              string
    Description       *string
    MachineID         string      // ← puente con el AAS (globalAssetId)
    EdgeType          string      // "RASPBERRY_PLC"
    RaspberryBaseURL  string      // endpoint HTTP del dispositivo físico
    PLCAddress        *string
    Status            string      // "ACTIVE" | "DISABLED"
    LastSeenAt        *time.Time
    LastHealthStatus  string      // "OK" | "DEGRADED" | "ERROR" | "UNKNOWN"
    LastHealthSummary *string
}

type TelemetrySnapshot struct {
    CapturedAt         time.Time
    CPU                *CPUTelemetry    // UsagePercent
    RAM                *RAMTelemetry    // UsedPercent, UsedMb, TotalMb
    Disk               *DiskTelemetry   // UsedPercent, UsedGb, TotalGb
    TemperatureCelsius *float64         // sensor de temperatura del dispositivo
    UptimeSeconds      *int64
    PLC                *PLCSnapshot     // Reachable, LatencyMs, LastHeartbeatAt
}

type DeviceEvent struct {
    ID            uuid.UUID
    DeviceID      uuid.UUID
    TenantID      uuid.UUID
    CheckType     string                  // "STATUS" | "HEALTH_CHECK"
    CheckedAt     time.Time
    OverallStatus string
    Details       map[string]interface{}  // persistido como JSONB en PostgreSQL
}
```

### Decisión arquitectónica ya tomada (ADR-003)

**PostgreSQL 16 + JSONB + particionamiento mensual** para `machine_events`:
- Columnas: `tenant_id`, `machine_id`, `event_id` (PK), `ts`, `seq`, `kind`, `payload` (JSONB)
- Índices: `(tenant_id, ts DESC)`, `(machine_id, ts DESC)`
- Retención online: 90 días
- ADR-003 menciona explícitamente TimescaleDB como **evolución futura posible**

### Brecha identificada

| Área | Estado actual | Qué falta |
|---|---|---|
| Estructura de la máquina | `machines.go` es un TODO stub | Modelo digital (AAS) persistido |
| Telemetría histórica | `TelemetrySnapshot` es transiente (no persiste) | Time-series store |
| Consumer events | Handlers devuelven 501 | Implementar ingestión + routing a stores |
| MachineID linkage | Campo existe en EdgeDevice | Conectar a entidad AAS |

---

## 2. Análisis de Opciones NoSQL

### 2.1 Comparativa general

| Base de datos | Modelo de datos | Driver Go | Multi-tenant | Series de tiempo | Caso de uso ideal en este proyecto |
|---|---|---|---|---|---|
| **MongoDB** | Documento (JSON/BSON) | Oficial (`mongo-driver/v2`) | ✅ Campo `tenantId` | ✅ Bueno | AAS, configuración de máquina, submodelos |
| **ArangoDB** | Multi-modelo (doc + grafo + KV) | Comunidad (`arangodb/go-driver`) | ✅ | ✅ | AAS + relaciones jerárquicas |
| **InfluxDB** | Series de tiempo | Oficial (`influxdb-client-go`) | Parcial (via org/bucket) | ✅✅ Excelente | Lecturas de sensores en tiempo real |
| **TimescaleDB** | Extensión de PostgreSQL | via `pgx/v5` (mismo driver actual) | ✅ | ✅✅ Excelente | Time-series reutilizando infra existente |
| **Cassandra / ScyllaDB** | Columnar distribuido | `gocql` | ✅ via keyspace | ✅✅ | Escrituras masivas 100K+/seg |

### 2.2 Recomendación para este proyecto

**Estrategia de dos capas NoSQL:**

```
PostgreSQL (existente)   → Usuarios, tenants, roles, sesiones, edge_devices, device_events (JSONB)
                           [ADR-003: machine_events con JSONB + particionamiento mensual]
MongoDB                  → AAS: estructura digital de la máquina, submodelos, configuración
TimescaleDB              → Time-series: TelemetrySnapshot histórico, lecturas de sensores
```

#### ¿Por qué MongoDB para AAS?

1. **Eclipse BaSyx** (implementación de referencia del estándar AAS) usa MongoDB como backend de producción
2. El metamodelo AAS se serializa en JSON → MongoDB almacena BSON sin transformación
3. Driver Go oficial v2 con soporte activo: `go.mongodb.org/mongo-driver/v2`
4. Multi-tenant simple: índice compuesto `{ tenantId, assetId }`
5. `machines.go` es un TODO stub — el momento ideal para definir las entidades AAS usando este store
6. MCP server oficial disponible para integración con Claude Code

#### ¿Por qué TimescaleDB sobre InfluxDB para time-series?

| Criterio | TimescaleDB | InfluxDB |
|---|---|---|
| Driver Go existente | Mismo `pgx/v5` ya en uso | Nuevo cliente a integrar |
| Nueva infraestructura | No (extensión de PostgreSQL) | Sí (nuevo servicio Docker) |
| SQL estándar + JOINs | ✅ JOIN con edge_devices | ❌ Solo InfluxQL/Flux |
| Antecedente en el proyecto | ✅ ADR-003 lo menciona explícitamente | ❌ |
| Curva de aprendizaje | Mínima | Media |

> Si el volumen supera 100K eventos/seg o se necesita replicación geográfica, escalar a Cassandra/ScyllaDB.

---

## 3. Asset Administration Shell (AAS)

### 3.1 ¿Qué es el AAS?

El estándar **IEC 63278-1:2023** define el Asset Administration Shell como la representación digital estandarizada de un activo industrial — el "pasaporte digital" de la máquina en el contexto de Industry 4.0.

```
Asset                    → La máquina física (embolsadora)
AssetAdministrationShell → Su gemelo digital (modelo persistido en MongoDB)
Submodel                 → Aspecto específico del asset (técnico, operacional, mantenimiento)
SubmodelElement          → Unidad de dato dentro de un submodelo
  ├── Property           → Valor escalar (número, string, booleano, fecha)
  ├── SubmodelElementCollection → Agrupación de elementos
  ├── File               → Referencia a documento/archivo
  ├── Operation          → Función ejecutable
  └── ReferenceElement   → Puntero a otro elemento
```

### 3.2 Vínculo EdgeDevice ↔ AAS

El campo `MachineID` del `EdgeDevice` es el **puente conceptual** entre la capa de conectividad y el gemelo digital:

```
┌──────────────────────┐          ┌──────────────────────────────────┐
│     EdgeDevice        │          │   AssetAdministrationShell        │
│  (PostgreSQL)         │          │   (MongoDB)                       │
│                       │          │                                   │
│  ID: uuid             │          │  _id: "urn:embolsadora:..."       │
│  TenantID: uuid       │◄────────►│  tenantId: uuid                   │
│  MachineID: string    │══════════│  assetInformation.globalAssetId   │
│  EdgeType: RASPBERRY  │          │  assetType: BaggingMachine        │
│  RaspberryBaseURL     │          │                                   │
│  PLCAddress           │          │  submodels[]:                     │
│  LastHealthStatus     │          │   - TechnicalData                 │
│  LastSeenAt           │          │   - OperationalData               │
└──────────────────────┘          │   - EdgeInfrastructure ← NUEVO    │
          │                        │   - Maintenance                   │
          │ TelemetrySnapshot       └──────────────────────────────────┘
          │ (transiente hoy)                     │
          ▼                                      ▼
┌──────────────────────┐          ┌──────────────────────────────────┐
│  /api/v1/consumers   │          │  Submodel: OperationalData        │
│  events_handler.go   │─────────►│  (actualización en tiempo real)   │
│  heartbeat_handler   │          └──────────────────────────────────┘
└──────────────────────┘
          │
          ▼
┌──────────────────────┐
│  TimescaleDB          │
│  telemetry_readings  │
│  (historial)          │
└──────────────────────┘
```

**Flujo de datos:**
1. El `EdgeDevice` se conecta vía HTTP (RaspberryBaseURL) para hacer status/health checks
2. `TelemetrySnapshot` se captura en cada health check (transiente, sin persistir hoy)
3. El consumer `/api/v1/consumers/events` recibirá eventos del edge en batch
4. Esos eventos actualizan el submodelo `OperationalData` en MongoDB (AAS) y se guardan como time-series en TimescaleDB

### 3.3 Submodelos estándar (IDTA) relevantes

| ID Submodelo | Descripción | Aplicación en Embolsadora |
|---|---|---|
| `IDTA-02006-2-0` | Nameplate V2 | Placa de identificación de la máquina |
| `IDTA-02014-1-0` | Maintenance V2 | Plan de mantenimiento preventivo |
| `IDTA-02010-1-0` | ServiceRequests | Solicitudes de servicio/mantenimiento |
| Custom | TechnicalData | Especificaciones físicas (pesos, velocidades, materiales) |
| Custom | OperationalData | Datos en tiempo real (estaciones, sensores, alarmas) |
| Custom | EdgeInfrastructure | Estado del hardware edge (CPU, RAM, PLC) mapeado desde TelemetrySnapshot |

### 3.4 Estructura AAS de una Embolsadora en MongoDB

#### Colección `asset_administration_shells`

```json
{
  "_id": "urn:embolsadora:tenant-abc:machine-001",
  "tenantId": "tenant-abc",
  "modelType": "AssetAdministrationShell",
  "id": "urn:embolsadora:machine-001",
  "assetInformation": {
    "assetKind": "Instance",
    "globalAssetId": "urn:embolsadora:machine-001",
    "assetType": "BaggingMachine"
  },
  "administration": { "version": "1", "revision": "0" },
  "submodels": [
    { "type": "ExternalReference", "keys": [{ "type": "Submodel", "value": "urn:embolsadora:sm:technical:001" }] },
    { "type": "ExternalReference", "keys": [{ "type": "Submodel", "value": "urn:embolsadora:sm:operational:001" }] },
    { "type": "ExternalReference", "keys": [{ "type": "Submodel", "value": "urn:embolsadora:sm:edge-infra:001" }] },
    { "type": "ExternalReference", "keys": [{ "type": "Submodel", "value": "urn:embolsadora:sm:maintenance:001" }] }
  ],
  "createdAt": "2026-01-10T00:00:00Z",
  "updatedAt": "2026-03-24T00:00:00Z"
}
```

#### Colección `submodels` — Datos Técnicos

```json
{
  "_id": "urn:embolsadora:sm:technical:001",
  "tenantId": "tenant-abc",
  "shellId": "urn:embolsadora:machine-001",
  "modelType": "Submodel",
  "idShort": "TechnicalData",
  "semanticId": {
    "type": "ExternalReference",
    "keys": [{ "type": "GlobalReference", "value": "urn:idta:submodel:TechnicalData:1.2" }]
  },
  "submodelElements": [
    { "modelType": "Property", "idShort": "MaxBagWeight",      "valueType": "xs:float",  "value": "50.0", "unit": "kg" },
    { "modelType": "Property", "idShort": "MinBagWeight",      "valueType": "xs:float",  "value": "1.0",  "unit": "kg" },
    { "modelType": "Property", "idShort": "MaxProductionRate", "valueType": "xs:int",    "value": "1200", "unit": "bolsas/hora" },
    { "modelType": "Property", "idShort": "ConveyorLength",    "valueType": "xs:float",  "value": "3.5",  "unit": "m" },
    { "modelType": "Property", "idShort": "FillMaterial",      "valueType": "xs:string", "value": "granulado" }
  ]
}
```

#### Colección `submodels` — Datos Operativos con jerarquía de estaciones

```json
{
  "_id": "urn:embolsadora:sm:operational:001",
  "tenantId": "tenant-abc",
  "shellId": "urn:embolsadora:machine-001",
  "modelType": "Submodel",
  "idShort": "OperationalData",
  "submodelElements": [
    { "modelType": "Property", "idShort": "CurrentBagWeight",   "valueType": "xs:float",  "value": "25.3",  "unit": "kg" },
    { "modelType": "Property", "idShort": "ProductionRateLive", "valueType": "xs:int",    "value": "980",   "unit": "bolsas/hora" },
    { "modelType": "Property", "idShort": "FillLevel",          "valueType": "xs:float",  "value": "72.5",  "unit": "%" },
    { "modelType": "Property", "idShort": "AlarmState",         "valueType": "xs:string", "value": "OK",
      "allowedValues": ["OK", "WARNING", "ALARM", "FAULT"] },
    {
      "modelType": "SubmodelElementCollection",
      "idShort": "Stations",
      "value": [
        {
          "modelType": "SubmodelElementCollection",
          "idShort": "FillingStation",
          "value": [
            { "modelType": "Property", "idShort": "ValveOpen",    "valueType": "xs:boolean", "value": "true" },
            { "modelType": "Property", "idShort": "FlowRate",     "valueType": "xs:float",   "value": "12.4", "unit": "kg/s" },
            { "modelType": "Property", "idShort": "TargetWeight", "valueType": "xs:float",   "value": "25.0", "unit": "kg" }
          ]
        },
        {
          "modelType": "SubmodelElementCollection",
          "idShort": "SealingStation",
          "value": [
            { "modelType": "Property", "idShort": "Temperature", "valueType": "xs:float",   "value": "180.0", "unit": "°C" },
            { "modelType": "Property", "idShort": "Pressure",    "valueType": "xs:float",   "value": "3.2",   "unit": "bar" },
            { "modelType": "Property", "idShort": "SealOK",      "valueType": "xs:boolean", "value": "true" }
          ]
        },
        {
          "modelType": "SubmodelElementCollection",
          "idShort": "ConveyorStation",
          "value": [
            { "modelType": "Property", "idShort": "Speed",   "valueType": "xs:float",   "value": "0.8",  "unit": "m/s" },
            { "modelType": "Property", "idShort": "Running", "valueType": "xs:boolean", "value": "true" }
          ]
        },
        {
          "modelType": "SubmodelElementCollection",
          "idShort": "WeighingStation",
          "value": [
            { "modelType": "Property", "idShort": "LastWeight",  "valueType": "xs:float",   "value": "25.2", "unit": "kg" },
            { "modelType": "Property", "idShort": "Tolerance",   "valueType": "xs:float",   "value": "0.5",  "unit": "kg" },
            { "modelType": "Property", "idShort": "WithinSpec",  "valueType": "xs:boolean", "value": "true" }
          ]
        }
      ]
    }
  ]
}
```

#### Colección `submodels` — Infraestructura Edge (mapeado desde TelemetrySnapshot)

```json
{
  "_id": "urn:embolsadora:sm:edge-infra:001",
  "tenantId": "tenant-abc",
  "shellId": "urn:embolsadora:machine-001",
  "modelType": "Submodel",
  "idShort": "EdgeInfrastructure",
  "submodelElements": [
    { "modelType": "Property", "idShort": "EdgeDeviceID",    "valueType": "xs:string", "value": "uuid-del-edge-device" },
    { "modelType": "Property", "idShort": "LastSeenAt",      "valueType": "xs:dateTime" },
    { "modelType": "Property", "idShort": "UptimeSeconds",   "valueType": "xs:long" },
    { "modelType": "Property", "idShort": "TemperatureCelsius", "valueType": "xs:float", "unit": "°C" },
    {
      "modelType": "SubmodelElementCollection",
      "idShort": "CPU",
      "value": [
        { "modelType": "Property", "idShort": "UsagePercent", "valueType": "xs:float", "unit": "%" }
      ]
    },
    {
      "modelType": "SubmodelElementCollection",
      "idShort": "RAM",
      "value": [
        { "modelType": "Property", "idShort": "UsedPercent", "valueType": "xs:float", "unit": "%" },
        { "modelType": "Property", "idShort": "UsedMb",      "valueType": "xs:float", "unit": "MB" },
        { "modelType": "Property", "idShort": "TotalMb",     "valueType": "xs:float", "unit": "MB" }
      ]
    },
    {
      "modelType": "SubmodelElementCollection",
      "idShort": "PLC",
      "value": [
        { "modelType": "Property", "idShort": "Reachable",        "valueType": "xs:boolean" },
        { "modelType": "Property", "idShort": "LatencyMs",        "valueType": "xs:int", "unit": "ms" },
        { "modelType": "Property", "idShort": "LastHeartbeatAt",  "valueType": "xs:dateTime" }
      ]
    }
  ]
}
```

#### Colección `submodels` — Mantenimiento

```json
{
  "_id": "urn:embolsadora:sm:maintenance:001",
  "tenantId": "tenant-abc",
  "shellId": "urn:embolsadora:machine-001",
  "modelType": "Submodel",
  "idShort": "Maintenance",
  "submodelElements": [
    { "modelType": "Property", "idShort": "LastMaintenanceDate", "valueType": "xs:date", "value": "2026-02-15" },
    { "modelType": "Property", "idShort": "NextMaintenanceDate", "valueType": "xs:date", "value": "2026-05-15" },
    { "modelType": "Property", "idShort": "OperatingHours",      "valueType": "xs:int",  "value": "4521" },
    { "modelType": "Property", "idShort": "MaintenanceCycle",    "valueType": "xs:int",  "value": "2000", "unit": "horas" },
    {
      "modelType": "SubmodelElementCollection",
      "idShort": "ServiceHistory",
      "value": [
        {
          "modelType": "SubmodelElementCollection",
          "idShort": "Service001",
          "value": [
            { "modelType": "Property", "idShort": "Date",        "valueType": "xs:date",   "value": "2026-02-15" },
            { "modelType": "Property", "idShort": "Type",        "valueType": "xs:string", "value": "Preventivo" },
            { "modelType": "Property", "idShort": "Technician",  "valueType": "xs:string", "value": "Juan Pérez" },
            { "modelType": "Property", "idShort": "Description", "valueType": "xs:string", "value": "Lubricación de rodamientos y ajuste de sensores" }
          ]
        }
      ]
    }
  ]
}
```

---

## 4. Librería Go para AAS

### aas-core3.0-golang (oficial)

```
github.com/aas-core-works/aas-core3.0-golang
```

- Implementa el metamodelo AAS **v3.0** completo (alineado con IEC 63278-1:2023)
- Serialización/deserialización **JSON y XML**
- Todos los tipos del metamodelo como interfaces Go
- Validación y verificación incluida
- Mantenido por `aas-core-works` (organización de referencia del estándar)

```bash
# via Docker (Go no está instalado en macOS host — ver CLAUDE.md)
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go get github.com/aas-core-works/aas-core3.0-golang && go mod tidy"
```

### Driver MongoDB para Go (v2 — actual)

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go get go.mongodb.org/mongo-driver/v2/mongo && go mod tidy"
```

### go-aas-proxy

- Proxy AAS sobre backends RDBMS y NoSQL
- Referencia: `hiroyoshii.github.io/go-aas-proxy`

---

## 5. MCPs disponibles para NoSQL

### MongoDB MCP (oficial de MongoDB Inc.)

| Atributo | Detalle |
|---|---|
| Repositorio | [mongodb-js/mongodb-mcp-server](https://github.com/mongodb-js/mongodb-mcp-server) |
| Mantenedor | MongoDB Inc. (oficial) |
| Compatibilidad | Atlas, Community, Enterprise |
| Clientes soportados | Claude Code, Cursor, VS Code Copilot, Windsurf |

**Instalación en Claude Code:**

```bash
claude mcp add mongodb -- npx -y @mongodb-js/mongodb-mcp-server \
  --connectionString "mongodb://localhost:27017"
```

**Capacidades:**
- Listar bases de datos y colecciones
- Ejecutar queries y aggregations en lenguaje natural
- Crear y modificar índices
- Insertar, actualizar y eliminar documentos
- Inspeccionar schemas

### ArangoDB MCP (comunidad)

| Atributo | Detalle |
|---|---|
| PulseMCP | [pulsemcp.com — ArangoDB by Lucas DE ANGELIS](https://www.pulsemcp.com/servers/lucas-deangelis-arangodb) |
| Glama | [glama.ai — mcp-server-arangodb by ravenwits](https://glama.ai/mcp/servers/ravenwits/mcp-server-arangodb) |

---

## 6. Plan de Integración Propuesto

### Estructura de repositorios

```
internal/
├── repo/
│   ├── pg/              (existente — users, tenants, user_roles, edge_devices, device_events)
│   │   └── machines_repo.go → REEMPLAZAR con mongo/aas/
│   ├── redis/           (existente — rate limit, idempotency)
│   ├── mongo/           (NUEVO)
│   │   ├── aas/         → AssetAdministrationShell repository
│   │   └── submodel/    → Submodel repository
│   └── tsdb/            (NUEVO — opcional, o via TimescaleDB extension en postgres)
│       └── telemetry/   → TelemetrySnapshot histórico
```

### Vínculo con entidades existentes

```go
// internal/domain/machines.go (hoy TODO stub → definir como)
type Machine struct {
    ID          uuid.UUID
    TenantID    uuid.UUID
    EdgeDeviceID uuid.UUID   // FK a EdgeDevice
    MachineID   string       // = EdgeDevice.MachineID = AAS.globalAssetId
    AASShellID  string       // urn:embolsadora:tenant:machine-xxx
    Name        string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### Flujo de actualización del AAS desde eventos IoT

```
POST /api/v1/consumers/events  (batch, hasta 1000 eventos)
    ↓
consumers/events_handler.go (a implementar)
    ├── repo/mongo/submodel/  → UPDATE Property "value" en OperationalData
    ├── repo/tsdb/telemetry/  → INSERT time-series row
    └── repo/pg/events_repo.go → INSERT en machine_events (ADR-003, JSONB)
```

### Índices MongoDB recomendados

```javascript
// asset_administration_shells
db.asset_administration_shells.createIndex(
  { tenantId: 1, "assetInformation.globalAssetId": 1 },
  { unique: true }
)
db.asset_administration_shells.createIndex({ tenantId: 1, updatedAt: -1 })

// submodels
db.submodels.createIndex({ tenantId: 1, shellId: 1, idShort: 1 }, { unique: true })
db.submodels.createIndex({ tenantId: 1, shellId: 1 })
```

### Variables de entorno a agregar

```env
MONGO_URI=mongodb://user:pass@localhost:27017
MONGO_DB=embolsadora_dev
# TimescaleDB usa DATABASE_URL existente (misma instancia PostgreSQL con extensión habilitada)
```

### docker-compose additions

```yaml
mongo:
  image: mongo:7
  environment:
    MONGO_INITDB_ROOT_USERNAME: embolsadora_user
    MONGO_INITDB_ROOT_PASSWORD: embolsadora_password
    MONGO_INITDB_DATABASE: embolsadora_dev
  ports:
    - "27017:27017"
  volumes:
    - mongo_data:/data/db
  networks:
    - embolsadora_network
```

---

## 7. Decisiones Arquitectónicas Pendientes (ADRs a crear)

| # | Decisión | Opciones | Recomendación | Antecedente |
|---|---|---|---|---|
| ADR-005 | Adoptar NoSQL para AAS | MongoDB vs ArangoDB | **MongoDB** — mayor ecosistema Go + BaSyx lo usa | — |
| ADR-006 | Time-series para telemetría | TimescaleDB vs InfluxDB vs Cassandra | **TimescaleDB** — reutiliza pgx/v5 + ADR-003 lo menciona | ADR-003 |
| ADR-007 | Librería AAS en Go | aas-core3.0-golang vs custom | **aas-core3.0-golang** — oficial del estándar | — |
| ADR-008 | MCP para desarrollo | MongoDB MCP vs ArangoDB MCP | **MongoDB MCP** — oficial, mejor mantenido | — |

> **Referencia**: ADR-003 (PostgreSQL JSONB + particionamiento) ya está aceptado y es el store
> para `machine_events`. TimescaleDB complementaría ese store para `telemetry_readings`
> con ventanas de tiempo y funciones de agregación nativas.

---

## 8. Referencias

| Recurso | URL |
|---|---|
| aas-core3.0-golang (GitHub) | https://github.com/aas-core-works/aas-core3.0-golang |
| aas-core3.0-golang (pkg.go.dev) | https://pkg.go.dev/github.com/aas-core-works/aas-core3.0-golang |
| awesome-aas | https://github.com/aas-core-works/awesome-aas |
| MongoDB MCP Server (oficial) | https://www.mongodb.com/products/tools/mcp-server |
| mongodb-mcp-server (GitHub) | https://github.com/mongodb-js/mongodb-mcp-server |
| ArangoDB MCP - PulseMCP | https://www.pulsemcp.com/servers/lucas-deangelis-arangodb |
| ArangoDB MCP - Glama | https://glama.ai/mcp/servers/ravenwits/mcp-server-arangodb |
| Eclipse BaSyx - MongoDB Storage | https://wiki.basyx.org/en/latest/content/user_documentation/basyx_components/v2/aas_registry/features/mongodb-storage.html |
| IEC 63278-1:2023 (AAS Spec) | https://webstore.iec.ch/en/publication/65628 |
| IDTA Metamodel Spec v3.0 | https://industrialdigitaltwin.org/wp-content/uploads/2023/06/IDTA-01001-3-0_SpecificationAssetAdministrationShell_Part1_Metamodel.pdf |
| ArangoDB vs MongoDB 2025 | https://salahudinmalik.com/posts/arangodb-vs-mongodb/ |
| go-aas-proxy | https://hiroyoshii.github.io/go-aas-proxy/ |
| TimescaleDB docs | https://docs.timescale.com |
