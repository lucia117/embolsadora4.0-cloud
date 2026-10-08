# Research: MongoDB Infrastructure Layer

**Feature**: 006-mongo-infra  
**Date**: 2026-04-02  
**Status**: Complete — no NEEDS CLARIFICATION items remain

---

## 1. Driver MongoDB para Go

**Decision**: `go.mongodb.org/mongo-driver/v2/mongo`

**Rationale**: Es el driver oficial de MongoDB Inc., en su versión v2 (API estabilizada en 2024). El research doc (`docs/nosql-aas-research.md`) lo identifica explícitamente. Compatible con Go 1.24+.

**Cambios de v1 → v2 relevantes para este proyecto**:
- `bson.ObjectID` reemplaza a `primitive.ObjectID`
- `mongo.Connect()` ya no requiere llamada separada a `Disconnect()` en el init; se usa `client.Disconnect(ctx)` en defer
- `mongo.NewClient()` + `client.Connect()` fue unificado en `mongo.Connect()`
- Los errores de "documento no encontrado" se verifican con `errors.Is(err, mongo.ErrNoDocuments)`
- `bson.D`, `bson.M` siguen igual

**Alternatives considered**: v1 (descartado — v2 es la versión activa; v1 dejó de recibir features nuevas)

---

## 2. Patrón de conexión opcional (como Redis)

**Decision**: Wiring en `main.go` siguiendo el patrón Redis existente — si `MONGO_URI` está vacío, log warning y continuar sin cliente.

**Rationale**: El código de `main.go` ya tiene este patrón documentado para Redis (líneas 49-60). MongoDB debe seguir exactamente la misma estructura para coherencia.

```go
// Patrón a replicar (Redis, main.go:49-60):
var redisClient *redis.Client
if cfg.Redis.URL != "" {
    opt, err := redis.ParseURL(cfg.Redis.URL)
    if err != nil {
        log.Printf("Invalid REDIS_URL, rate limiting disabled: %v", err)
    } else {
        redisClient = redis.NewClient(opt)
        if err := redisClient.Ping(context.Background()).Err(); err != nil {
            log.Printf("Redis unreachable, rate limiting disabled: %v", err)
            redisClient = nil
        }
    }
}

// Equivalente para MongoDB:
var mongoClient *mongo.Client
if cfg.Mongo.URI != "" {
    mongoClient, err = platform_mongo.Connect(context.Background(), cfg.Mongo)
    if err != nil {
        log.Printf("WARN mongo disabled — connection failed: %v", err)
    } else {
        defer mongoClient.Disconnect(context.Background())
        log.Println("MongoDB connection established")
    }
} else {
    log.Println("WARN mongo disabled — MONGO_URI not set")
}
```

---

## 3. Estructura de colecciones e índices

**Decision**: Dos colecciones: `asset_administration_shells` y `submodels`. Índices únicos compuestos con `tenantId`.

**Rationale**: Diseñados en el research doc (sección 6) y confirmados por el metamodelo AAS — el `globalAssetId` es único por tenant para shells; `idShort` es único por tenant+shellId para submodelos.

**Índices**:
```javascript
// asset_administration_shells
{ tenantId: 1, "assetInformation.globalAssetId": 1 }  // unique: true
{ tenantId: 1, updatedAt: -1 }

// submodels  
{ tenantId: 1, shellId: 1, idShort: 1 }  // unique: true
{ tenantId: 1, shellId: 1 }
```

**Creación idempotente**: `collection.Indexes().CreateOne()` con `mongo.IndexModel` — si el índice ya existe con la misma definición, MongoDB no lo recrea (idempotente por diseño).

---

## 4. Patrón de métricas Prometheus para repositorios

**Decision**: `promauto.NewHistogramVec` para latencia + `promauto.NewCounterVec` para errores, en `internal/telemetry/mongo_metrics.go`, siguiendo `auth_metrics.go`.

**Rationale**: El proyecto usa `promauto` (registro automático en el registry global de Prometheus). El patrón es consistente en el codebase.

```go
// Métricas a registrar:
MongoOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "mongo_operation_duration_seconds",
    Help:    "Latency of MongoDB repository operations",
    Buckets: prometheus.DefBuckets,
}, []string{"collection", "operation"})  // labels: "shells"/"submodels", "create"/"get"/"update"/"delete"/"list"

MongoOperationErrors = promauto.NewCounterVec(prometheus.CounterOpts{
    Name: "mongo_operation_errors_total",
    Help: "Total number of MongoDB operation errors",
}, []string{"collection", "operation"})
```

**Uso en repo**: `defer` con `time.Since(start)` al inicio de cada método.

---

## 5. Aislamiento de tests de integración

**Decision**: Base de datos efímera por test con nombre `test_<uuid>`, eliminada en `t.Cleanup()`.

**Rationale**: Acordado en clarificación Q5. Es el patrón estándar para repositorios MongoDB en Go — evita estado compartido entre tests en paralelo.

```go
func setupTestDB(t *testing.T, client *mongo.Client) *mongo.Database {
    t.Helper()
    dbName := "test_" + uuid.New().String()
    db := client.Database(dbName)
    t.Cleanup(func() {
        if err := db.Drop(context.Background()); err != nil {
            t.Logf("cleanup: failed to drop test DB %s: %v", dbName, err)
        }
    })
    return db
}
```

**Prerequisito de tests**: Variable de entorno `MONGO_TEST_URI` (default: `mongodb://localhost:27017`). Los tests se saltean si no hay conexión disponible (usando `t.Skip()`).

---

## 6. Traducción de errores del driver a errores de dominio

**Decision**: Capa de traducción en cada repositorio; exponer solo `domain/errors.go` types.

**Rationale**: FR-009. El driver v2 usa `mongo.ErrNoDocuments` para "not found" y errores de duplicate key con código `11000` (`mongo.IsDuplicateKeyError(err)`).

```go
// Mapeo en repositorios:
errors.Is(err, mongo.ErrNoDocuments)  →  domain.ErrNotFound
mongo.IsDuplicateKeyError(err)        →  domain.ErrConflict
```

**Alternatives considered**: Wrapping de errores del driver directamente — descartado porque viola FR-009 y acoplaría las capas superiores al driver.

---

## 7. docker-compose para desarrollo local

**Decision**: Servicio `mongo:7` sin autenticación (acordado en clarificación Q4).

```yaml
mongo:
  image: mongo:7
  ports:
    - "27017:27017"
  volumes:
    - mongo_data:/data/db
  networks:
    - embolsadora_network
```

`MONGO_URI=mongodb://localhost:27017` en `.env`.  
`MONGO_DB=embolsadora_dev` en `.env`.

---

## 8. Librería AAS (aas-core3.0-golang)

**Decision**: **No incluir en esta feature**. Los tipos de dominio (`AssetAdministrationShell`, `Submodel`) se definen como structs Go propios en `internal/domain/aas/`.

**Rationale**: La librería `aas-core3.0-golang` es relevante para la feature AAS Server completa (004-aas-server), donde se necesita serialización/deserialización estándar y validación del metamodelo. Para la capa de infraestructura MongoDB (esta feature), los tipos propios son suficientes y evitan una dependencia prematura.

**Alternatives considered**: Incluir `aas-core3.0-golang` desde ahora — descartado para mantener el scope mínimo de esta feature.
