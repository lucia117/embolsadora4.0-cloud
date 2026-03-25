# Technical Plan: AAS Server — Asset Administration Shell

**Branch**: `004-aas-server` | **Date**: 2026-03-24 | **Spec**: [spec.md](./spec.md)
**Standard**: IDTA-01002-3-1 (AAS Part 2 API) | IEC 63278-1:2023

---

## Summary

Implementar un servidor AAS conforme al estándar IDTA dentro del monolito Go existente. El servidor expone cinco interfaces REST (AAS Repository, Submodel Repository, AAS Registry, Submodel Registry, Discovery Service), usa **MongoDB** como store principal, y es el destino de actualizaciones enviadas por la **Processing API** luego de procesar datos de InfluxDB (llenado por el PLC).

---

## Arquitectura del sistema completo

```
┌──────────────────────────────────────────────────────────────────┐
│  Capa de campo (existente, externa)                               │
│                                                                   │
│  PLC ──writes──► InfluxDB                                        │
└────────────────────────────────┬─────────────────────────────────┘
                                 │ lectura periódica
                                 ▼
┌──────────────────────────────────────────────────────────────────┐
│  Processing API  (nuevo — puede ser un servicio separado o        │
│  un worker dentro del monolito)                                  │
│                                                                   │
│  1. Lee datos de InfluxDB (raw telemetry del PLC)                │
│  2. Valida, normaliza, aplica reglas de negocio                  │
│  3. Mapea a AAS Property paths                                   │
│  4. Llama al AAS Server vía PATCH $value                         │
└────────────────────────────────┬─────────────────────────────────┘
                                 │ PATCH /submodels/{id}/
                                 │   submodel-elements/{path}/$value
                                 ▼
┌──────────────────────────────────────────────────────────────────┐
│  AAS Server  (este feature — dentro del monolito Go)             │
│                                                                   │
│  /api/aas/v3/                                                    │
│  ├── shells/              AAS Repository      → MongoDB          │
│  ├── submodels/           Submodel Repository → MongoDB          │
│  ├── registry/            AAS Registry        → MongoDB          │
│  └── lookup/              Discovery Service   → MongoDB          │
│                                                                   │
│  MongoDB collections:                                            │
│  ├── asset_administration_shells                                 │
│  ├── submodels                                                   │
│  ├── shell_descriptors                                           │
│  └── property_change_log                                         │
└──────────────────────────────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────┐
│  Consumidores del AAS (lectura)                                  │
│  Frontend, sistemas externos (MES, SCADA), otros servicios       │
└──────────────────────────────────────────────────────────────────┘
```

---

## Technical Context

**Language/Version**: Go 1.24+
**HTTP Framework**: Gin v1.11
**Storage**: MongoDB 7 (`go.mongodb.org/mongo-driver/v2`)
**AAS SDK**: `github.com/aas-core-works/aas-core3.0-golang` (tipos, validación, serialización)
**Existing Storage**: PostgreSQL (para `EdgeDevice.MachineID` — FK conceptual, no forzada en DB)
**Auth**: Supabase JWT (mismo middleware `JWTAuth` existente)
**Logging**: Uber Zap (existente)
**Metrics**: Prometheus (existente)
**Testing**: testify + uber/mock
**Target**: Linux server (Docker / Cloud Run)

---

## Nuevas dependencias

```bash
# AAS SDK oficial (tipos, serialización JSON/XML, validación)
go get github.com/aas-core-works/aas-core3.0-golang

# MongoDB driver v2
go get go.mongodb.org/mongo-driver/v2/mongo
```

---

## API Contract (IDTA-01002 compliant)

### Prefijo base

```
/api/aas/v3/
```

El tenant se extrae del JWT (no del path), siguiendo el patrón existente del proyecto.

### AAS Repository

| Método | Path | Descripción |
|--------|------|-------------|
| `GET` | `/shells` | Listar todos los AAS del tenant (paginado) |
| `POST` | `/shells` | Crear nuevo AAS |
| `GET` | `/shells/{aasId}` | Obtener AAS por ID (Base64URL) |
| `PUT` | `/shells/{aasId}` | Actualizar AAS |
| `DELETE` | `/shells/{aasId}` | Eliminar AAS |
| `GET` | `/shells/{aasId}/submodel-refs` | Listar referencias de submodelos del AAS |
| `POST` | `/shells/{aasId}/submodel-refs` | Agregar referencia a submodelo |
| `DELETE` | `/shells/{aasId}/submodel-refs/{smId}` | Quitar referencia |

### Submodel Repository

| Método | Path | Descripción |
|--------|------|-------------|
| `GET` | `/submodels` | Listar submodelos del tenant (paginado) |
| `POST` | `/submodels` | Crear submodelo |
| `GET` | `/submodels/{smId}` | Obtener submodelo completo |
| `PUT` | `/submodels/{smId}` | Actualizar submodelo |
| `DELETE` | `/submodels/{smId}` | Eliminar submodelo |
| `GET` | `/submodels/{smId}/submodel-elements` | Listar elementos |
| `GET` | `/submodels/{smId}/submodel-elements/{idShortPath}` | Obtener elemento por path |
| `PATCH` | `/submodels/{smId}/submodel-elements/{idShortPath}/$value` | **Actualizar valor** (usado por Processing API) |
| `GET` | `/submodels/{smId}/submodel-elements/{idShortPath}/$value` | Obtener solo el valor |

### AAS Registry

| Método | Path | Descripción |
|--------|------|-------------|
| `GET` | `/registry/shell-descriptors` | Listar descriptores |
| `POST` | `/registry/shell-descriptors` | Registrar descriptor |
| `GET` | `/registry/shell-descriptors/{aasId}` | Obtener descriptor |
| `PUT` | `/registry/shell-descriptors/{aasId}` | Actualizar descriptor |
| `DELETE` | `/registry/shell-descriptors/{aasId}` | Eliminar descriptor |

### Discovery Service

| Método | Path | Descripción |
|--------|------|-------------|
| `GET` | `/lookup/shells` | Buscar AAS-IDs por `assetIds` (query param, Base64URL) |
| `POST` | `/lookup/shells` | Registrar relación assetId → aasId |
| `DELETE` | `/lookup/shells/{aasId}` | Eliminar relación |

### Endpoint de historial (extensión propia)

| Método | Path | Descripción |
|--------|------|-------------|
| `GET` | `/submodels/{smId}/submodel-elements/{idShortPath}/history` | Historial de cambios de una Property |

---

## Data Model (MongoDB)

### Colección `asset_administration_shells`

```json
{
  "_id": "urn:embolsadora:{tenantId}:{machineId}",
  "tenantId": "uuid-string",
  "modelType": "AssetAdministrationShell",
  "assetInformation": {
    "assetKind": "Instance",
    "globalAssetId": "urn:embolsadora:machine-001",
    "assetType": "BaggingMachine"
  },
  "administration": { "version": "1", "revision": "0" },
  "submodels": [
    { "type": "ExternalReference", "keys": [{ "type": "Submodel", "value": "urn:..." }] }
  ],
  "createdAt": "2026-03-24T00:00:00Z",
  "updatedAt": "2026-03-24T00:00:00Z"
}
```

**Índices**:
```javascript
{ tenantId: 1, "assetInformation.globalAssetId": 1 }  // unique
{ tenantId: 1, updatedAt: -1 }
```

### Colección `submodels`

```json
{
  "_id": "urn:embolsadora:sm:{tenantId}:{machineId}:operational",
  "tenantId": "uuid-string",
  "shellId": "urn:embolsadora:{tenantId}:{machineId}",
  "modelType": "Submodel",
  "idShort": "OperationalData",
  "semanticId": { ... },
  "submodelElements": [ ... ],
  "createdAt": "...",
  "updatedAt": "..."
}
```

**Índices**:
```javascript
{ tenantId: 1, shellId: 1, idShort: 1 }  // unique
{ tenantId: 1, shellId: 1 }
```

### Colección `shell_descriptors`

```json
{
  "_id": "urn:embolsadora:{tenantId}:{machineId}",
  "tenantId": "uuid-string",
  "globalAssetId": "urn:embolsadora:machine-001",
  "endpoints": [
    { "protocolInformation": { "href": "https://api.ejemplo.com/api/aas/v3/shells/{base64Id}" }, "interface": "AAS-3.0" }
  ],
  "createdAt": "...",
  "updatedAt": "..."
}
```

**Índices**:
```javascript
{ tenantId: 1, globalAssetId: 1 }  // unique
```

### Colección `property_change_log`

```json
{
  "_id": "ObjectId",
  "tenantId": "uuid-string",
  "submodelId": "urn:...",
  "propertyPath": "Stations.FillingStation.FlowRate",
  "oldValue": "12.4",
  "newValue": "13.1",
  "valueType": "xs:float",
  "source": "processing-api",
  "changedAt": "2026-03-24T10:32:00Z"
}
```

**Índices**:
```javascript
{ tenantId: 1, submodelId: 1, propertyPath: 1, changedAt: -1 }
{ changedAt: 1 }  // TTL index — expirar después de 90 días
```

---

## Vínculo con entidades existentes

| Concepto AAS | Concepto del proyecto | Store | Nota |
|---|---|---|---|
| `globalAssetId` | `EdgeDevice.MachineID` | PostgreSQL | FK conceptual — no forzada en DB |
| `AssetAdministrationShell` | `machines.go` (TODO stub → implementar aquí) | MongoDB | |
| `Submodel: OperationalData` | Datos del PLC procesados por Processing API | MongoDB | |
| `Submodel: EdgeInfrastructure` | `TelemetrySnapshot` del `EdgeDeviceClient` | MongoDB | |
| `PropertyChangeLog` | — | MongoDB | Nuevo |

---

## Source Code Layout

```
internal/
├── api/
│   ├── handler/
│   │   └── aas/
│   │       ├── dto/
│   │       │   └── dto.go                   # Request/response DTOs (JSON ↔ AAS types)
│   │       ├── shells/
│   │       │   ├── list_shells.go
│   │       │   ├── create_shell.go
│   │       │   ├── get_shell.go
│   │       │   ├── update_shell.go
│   │       │   ├── delete_shell.go
│   │       │   ├── list_submodel_refs.go
│   │       │   └── routes.go
│   │       ├── submodels/
│   │       │   ├── list_submodels.go
│   │       │   ├── create_submodel.go
│   │       │   ├── get_submodel.go
│   │       │   ├── update_submodel.go
│   │       │   ├── delete_submodel.go
│   │       │   ├── get_element_value.go
│   │       │   ├── patch_element_value.go   # Usado por Processing API
│   │       │   ├── get_element_history.go
│   │       │   └── routes.go
│   │       ├── registry/
│   │       │   ├── list_descriptors.go
│   │       │   ├── create_descriptor.go
│   │       │   ├── get_descriptor.go
│   │       │   ├── update_descriptor.go
│   │       │   ├── delete_descriptor.go
│   │       │   └── routes.go
│   │       └── discovery/
│   │           ├── lookup_shells.go
│   │           └── routes.go
├── app/
│   └── aas/
│       └── service.go                       # Orquestación + lógica de negocio AAS
├── domain/
│   └── aas/
│       ├── shell.go                         # AssetAdministrationShell + validación
│       ├── submodel.go                      # Submodel + SubmodelElement types
│       ├── descriptor.go                    # ShellDescriptor
│       ├── property_change.go               # PropertyChangeEvent
│       ├── errors.go                        # ErrShellNotFound, ErrSubmodelNotFound, ErrElementNotFound, ErrDuplicateShell, ErrTypeMismatch
│       └── repository.go                    # Interfaces: ShellRepository, SubmodelRepository, DescriptorRepository, ChangeLogRepository
├── repo/
│   └── mongo/
│       ├── client.go                        # MongoDB client + connection
│       ├── aas/
│       │   └── shell_repo.go                # Implementación ShellRepository
│       ├── submodel/
│       │   └── submodel_repo.go             # Implementación SubmodelRepository
│       ├── registry/
│       │   └── descriptor_repo.go           # Implementación DescriptorRepository
│       └── changelog/
│           └── changelog_repo.go            # Implementación ChangeLogRepository
├── platform/
│   └── aasid/
│       └── encoding.go                      # Base64URL encode/decode para IDs (IDTA)
└── routes/
    └── url_mappings.go                      # Nuevo grupo /api/aas/v3
```

---

## Fases de implementación

### Fase 1 — Infraestructura MongoDB

**Objetivo**: Conectar MongoDB al monolito.

**Archivos**:
- `internal/repo/mongo/client.go` — `NewMongoClient(uri, dbName) (*mongo.Client, error)`
- `internal/config/config.go` — agregar `MongoConfig{ URI, DBName }`
- `cmd/api/main.go` — inicializar cliente MongoDB al startup
- `docker-compose.yml` — agregar servicio `mongo:7`

**Variables de entorno**:
```
MONGO_URI=mongodb://embolsadora_user:embolsadora_password@mongo:27017
MONGO_DB=embolsadora_dev
```

---

### Fase 2 — Dominio AAS

**Objetivo**: Definir tipos del dominio en Go usando `aas-core3.0-golang` como base.

**Archivos**:
- `internal/domain/aas/shell.go`
- `internal/domain/aas/submodel.go`
- `internal/domain/aas/descriptor.go`
- `internal/domain/aas/property_change.go`
- `internal/domain/aas/errors.go`
- `internal/domain/aas/repository.go`

**Interfaces de repositorio**:

```go
type ShellRepository interface {
    List(ctx context.Context, tenantID string, cursor string, limit int) ([]*Shell, string, error)
    GetByID(ctx context.Context, tenantID, shellID string) (*Shell, error)
    Create(ctx context.Context, shell *Shell) error
    Update(ctx context.Context, shell *Shell) error
    Delete(ctx context.Context, tenantID, shellID string) error
    GetByAssetID(ctx context.Context, tenantID, globalAssetID string) (*Shell, error)
}

type SubmodelRepository interface {
    List(ctx context.Context, tenantID string, cursor string, limit int) ([]*Submodel, string, error)
    GetByID(ctx context.Context, tenantID, smID string) (*Submodel, error)
    Create(ctx context.Context, sm *Submodel) error
    Update(ctx context.Context, sm *Submodel) error
    Delete(ctx context.Context, tenantID, smID string) error
    PatchElementValue(ctx context.Context, tenantID, smID, idShortPath, value string) (oldValue string, err error)
    GetElementValue(ctx context.Context, tenantID, smID, idShortPath string) (string, error)
}

type DescriptorRepository interface {
    List(ctx context.Context, tenantID string) ([]*ShellDescriptor, error)
    GetByID(ctx context.Context, tenantID, aasID string) (*ShellDescriptor, error)
    GetByAssetID(ctx context.Context, tenantID, globalAssetID string) ([]*ShellDescriptor, error)
    Create(ctx context.Context, desc *ShellDescriptor) error
    Update(ctx context.Context, desc *ShellDescriptor) error
    Delete(ctx context.Context, tenantID, aasID string) error
}

type ChangeLogRepository interface {
    Append(ctx context.Context, entry *PropertyChangeEvent) error
    List(ctx context.Context, tenantID, smID, idShortPath string) ([]*PropertyChangeEvent, error)
}
```

---

### Fase 3 — Repositorios MongoDB

**Objetivo**: Implementar las interfaces del dominio contra MongoDB.

**Archivos**:
- `internal/repo/mongo/aas/shell_repo.go`
- `internal/repo/mongo/submodel/submodel_repo.go`
- `internal/repo/mongo/registry/descriptor_repo.go`
- `internal/repo/mongo/changelog/changelog_repo.go`
- `internal/platform/aasid/encoding.go` — Base64URL encode/decode

**Detalle `PatchElementValue`**:
1. Cargar el documento del submodelo desde MongoDB
2. Navegar la jerarquía de `submodelElements` siguiendo `idShortPath` (separado por `.`)
3. Validar que el elemento es una `Property` (no `SubmodelElementCollection`)
4. Validar el tipo del nuevo valor contra `valueType`
5. Actualizar `value` y `updatedAt` del elemento y del submodelo raíz
6. Retornar `oldValue` para el log de cambios
7. Operación atómica con `FindOneAndUpdate` de MongoDB

---

### Fase 4 — Servicio de aplicación

**Objetivo**: Orquestar repositorios, aplicar reglas de negocio y escribir el change log.

**Archivo**: `internal/app/aas/service.go`

```go
type AASService struct {
    shellRepo    aas.ShellRepository
    smRepo       aas.SubmodelRepository
    descRepo     aas.DescriptorRepository
    changeLog    aas.ChangeLogRepository
    logger       *zap.Logger
}
```

**Operaciones clave**:

- `PatchElementValue(ctx, tenantID, smID, idShortPath, value, source string)`:
  1. Llamar `smRepo.PatchElementValue` → obtener `oldValue`
  2. Llamar `changeLog.Append` con `oldValue`, `newValue`, `source`, `changedAt: time.Now()`
  3. Retornar 204

- `CreateShell(ctx, tenantID, shell)`:
  1. Validar que no existe otro shell con el mismo `globalAssetId` en el tenant
  2. Generar ID si no fue provisto (URN canónico)
  3. Persistir en MongoDB
  4. Auto-crear `ShellDescriptor` en el Registry con endpoint local

- `DeleteShell(ctx, tenantID, shellID)`:
  1. Verificar existencia
  2. Eliminar shell
  3. Eliminar descriptor del Registry (cascade)
  4. NO eliminar submodelos (pueden ser referenciados por otros shells)

---

### Fase 5 — Handlers HTTP

**Objetivo**: Capa de transporte — parsear requests, llamar servicio, formatear respuesta.

**Consideraciones de formato**:
- IDs en path params: siempre en Base64URL (encode en response, decode en handler)
- Paginación: `?cursor=base64&limit=N` → `{"paging_metadata": {"cursor": "...", "total": N}, "result": [...]}`
- Niveles de contenido: soporte para `?level=deep` (default) y `?level=core`
- Content-Type: `application/json` por defecto; `application/xml` si `Accept: application/xml`

**Error mapping**:

| Domain Error | HTTP |
|---|---|
| `ErrShellNotFound` | 404 |
| `ErrSubmodelNotFound` | 404 |
| `ErrElementNotFound` | 404 |
| `ErrDuplicateShell` | 409 |
| `ErrTypeMismatch` | 400 |
| `ErrElementNotProperty` | 405 Method Not Allowed |
| `ErrUnauthorized` (cross-tenant) | 403 |

---

### Fase 6 — Middleware y routing

**Objetivo**: Registrar el grupo `/api/aas/v3` en el monolito.

**Archivo**: `internal/routes/url_mappings.go`

```go
aasGroup := r.Group(
    "/api/aas/v3",
    apimw.RequestID(),
    apimw.Logger(),
    apimw.CORS(),
    apimw.JWTAuth(verifier, authUC, invUC),
    // Sin TenantFromHeader — el tenant se extrae del JWT (patrón existente)
)
aas.RegisterRoutes(aasGroup, aasDeps)
```

**Tenant isolation**: extraído del JWT via `platform.TenantID(ctx)` (igual que `/api/v1/me`).

---

### Fase 7 — Processing API (worker interno)

**Objetivo**: Leer datos de InfluxDB, procesar y llamar `PATCH $value` en el AAS Server.

**Opciones de implementación**:

| Opción | Pros | Contras |
|---|---|---|
| **Worker goroutine en el monolito** | Simple, sin infra extra, acceso directo al repo MongoDB | Acoplado al monolito, difícil de escalar independientemente |
| **Servicio separado (microservicio)** | Escala independiente, falla aislada | Nueva infra, más complejidad operativa |
| **Scheduled job (cron)** | Predecible, simple | Latencia proporcional al intervalo |

**Recomendación MVP**: Worker goroutine dentro del monolito con intervalo configurable.

**Archivo**: `internal/workers/plc_data_processor.go`

```go
type PLCDataProcessor struct {
    influxClient  influxdb2.Client
    aasService    *aas.AASService
    interval      time.Duration
    mappings      []PLCMapping     // configuración: InfluxDB measurement → AAS idShortPath
    logger        *zap.Logger
}

type PLCMapping struct {
    TenantID     string
    SubmodelID   string
    Measurement  string  // nombre de la serie en InfluxDB
    Field        string  // campo de la medición
    IDShortPath  string  // path en el submodelo AAS destino
    ValueType    string  // "xs:float", "xs:int", etc.
}
```

**Flujo del worker**:
1. Conectar a InfluxDB con `influxdb-client-go`
2. Cada `interval` (default: 5s), ejecutar Flux queries para cada `PLCMapping`
3. Obtener el último valor de cada measurement/field
4. Aplicar transformaciones configuradas (escala, offset, unidades)
5. Llamar `aasService.PatchElementValue(ctx, tenantID, smID, idShortPath, value, "processing-api")`
6. Registrar latencia y errores en Prometheus

**Nueva dependencia**:
```bash
go get github.com/influxdata/influxdb-client-go/v2
```

**Variables de entorno**:
```
INFLUXDB_URL=http://influxdb:8086
INFLUXDB_TOKEN=...
INFLUXDB_ORG=embolsadora
INFLUXDB_BUCKET=plc_data
PLC_PROCESSOR_INTERVAL=5s
```

---

### Fase 8 — Observabilidad

**Métricas Prometheus** (nuevas):
- `aas_requests_total{operation, status}` — contador por operación
- `aas_patch_value_duration_seconds` — histograma de latencia de PATCH $value
- `plc_processor_cycles_total{status}` — ciclos del worker (success/error)
- `plc_processor_lag_seconds` — tiempo entre última lectura InfluxDB y actualización AAS
- `mongo_operation_duration_seconds{collection, operation}` — histograma MongoDB

**Logging** (Zap, estructurado):
- Cada `PatchElementValue`: `{tenant_id, submodel_id, path, old_value, new_value, source, duration_ms}`
- Errores del worker PLC: `{tenant_id, mapping, influx_query, error}`

---

### Fase 9 — Tests

**Unit tests** (uber/mock):
- `app/aas/service_test.go` — mock repos, todos los casos de servicio
- `platform/aasid/encoding_test.go` — round-trip Base64URL

**Integration tests** (Docker MongoDB):
- `tests/integration/aas/` — MongoDB real, verificar CRUD + PatchElementValue con jerarquía anidada

**Contract tests**:
- Verificar conformidad con OpenAPI de IDTA-01002-3-1 usando los schemas oficiales del repositorio `admin-shell-io/aas-specs-api`

---

## Constitution Check

| Principio | Estado | Nota |
|---|---|---|
| I — Hexagonal | ✅ | `transport → app → domain ← repo/mongo` |
| II — Seguridad / Tenant Isolation | ✅ | JWT en todos los endpoints; `tenantId` en cada query MongoDB |
| III — Observabilidad | ✅ | Zap + Prometheus en Fases 8 |
| IV — Integration Tests | ✅ | Docker MongoDB en Fase 9 |
| V — Semantic Versioning | ✅ | Nueva surface `/api/aas/v3` — MINOR bump |

---

## Decisiones de diseño

| Decisión | Justificación | Trade-off |
|---|---|---|
| **MongoDB para AAS** | Mapeo 1:1 JSON ↔ BSON; Eclipse BaSyx lo usa en producción; índices eficientes para consulta por tenant+assetId | Nueva infra vs reutilizar PostgreSQL JSONB |
| **aas-core3.0-golang** | SDK oficial del estándar; validación de tipos y serialización incluida | Dependency externa; versión v3.0 (última especificación) |
| **Worker dentro del monolito (MVP)** | Sin nueva infra; acceso directo a MongoDB | Acoplamiento; a extraer como microservicio en v2 |
| **Tenant del JWT (no del path)** | Consistente con el patrón `/api/v1/me` existente | No permite "act-as" para AAS (se puede agregar después) |
| **Historial con TTL 90 días** | Balance entre auditabilidad y costo de storage | Series de alta frecuencia van a InfluxDB, no aquí |
| **Base64URL en IDs de path** | Obligatorio por IDTA-01002 (los IDs son URNs con caracteres especiales) | Requiere encode/decode en cada handler |
| **PatchElementValue atómico** | MongoDB `FindOneAndUpdate` garantiza consistencia | Documentos grandes pueden ser lentos; partición de submodelos recomendada |

---

## ADRs a crear

| # | Decisión |
|---|---|
| ADR-005 | Adoptar MongoDB para AAS Repository |
| ADR-006 | TimescaleDB para series de tiempo de sensores (PLC data de alta frecuencia) |
| ADR-007 | aas-core3.0-golang como SDK del estándar AAS |
| ADR-008 | Processing API como worker goroutine en el monolito (MVP) |

---

## Artifacts Summary

| Artefacto | Path |
|---|---|
| Especificación funcional | `specs/004-aas-server/spec.md` |
| Plan técnico (este archivo) | `specs/004-aas-server/plan.md` |
| Research NoSQL + AAS | `docs/nosql-aas-research.md` |
| Data model detallado | `specs/004-aas-server/data-model.md` (a generar) |
| OpenAPI contract | `specs/004-aas-server/contracts/aas-server-api.openapi.yaml` (a generar) |
| Tasks | `specs/004-aas-server/tasks.md` (a generar con `/speckit.tasks`) |

---

## Referencias

| Recurso | URL |
|---|---|
| IDTA-01002-3-1-1 AAS Part 2 API | https://industrialdigitaltwin.org/wp-content/uploads/2025/08/IDTA-01002-3-1-1_AAS-Specification_Part2_API.pdf |
| aas-specs-api (OpenAPI oficial) | https://github.com/admin-shell-io/aas-specs-api |
| aas-core3.0-golang | https://github.com/aas-core-works/aas-core3.0-golang |
| Eclipse BaSyx AAS Repository | https://wiki.basyx.org/en/latest/content/user_documentation/basyx_components/v2/aas_repository/index.html |
| Eclipse BaSyx MongoDB Storage | https://wiki.basyx.org/en/latest/content/user_documentation/basyx_components/v2/aas_registry/features/mongodb-storage.html |
| influxdb-client-go | https://github.com/influxdata/influxdb-client-go |
