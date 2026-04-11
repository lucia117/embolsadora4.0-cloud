# Implementation Plan: MongoDB Infrastructure Layer

**Branch**: `006-mongo-infra` | **Date**: 2026-04-02 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `specs/006-mongo-infra/spec.md`

---

## Summary

Establecer la infraestructura de acceso a MongoDB en el proyecto: conexión opcional (patrón Redis existente), repositorios base para AAS shells y submodelos con CRUD completo, métricas Prometheus por operación, y un patrón de tests de integración con DB efímera por test. No se exponen endpoints HTTP en esta feature; la capa queda lista para ser consumida por las features AAS Server y Consumer Events.

---

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: `go.mongodb.org/mongo-driver/v2/mongo` (nueva), Gin, pgx/v5, Zap, Prometheus/promauto, testify  
**Storage**: PostgreSQL (existente) + MongoDB 7 (nueva, opcional en startup)  
**Testing**: testify + contenedores Docker para integración; DB efímera por test (`test_<uuid>`)  
**Target Platform**: Docker + Docker Compose (dev); Go en contenedor (no instalado en host macOS)  
**Project Type**: web-service (monolito modular hexagonal)  
**Performance Goals**: Latencia de operaciones de repo < 50ms p95 en condiciones normales (DB local)  
**Constraints**: MongoDB es opcional — servidor arranca sin él; aislamiento multi-tenant obligatorio en todas las queries  
**Scale/Scope**: Decenas de shells por tenant; cientos de submodelos por shell; paginación limit/offset desde inicio

---

## Constitution Check

| Principio | Estado | Notas |
|-----------|--------|-------|
| I. Arquitectura Hexagonal | ✅ PASS | Tipos en `domain/aas/`, repos en `repo/mongo/`, cliente en `platform/mongo/` |
| II. Seguridad — aislamiento tenant | ✅ PASS | `tenantID` obligatorio en todas las firmas de repositorio; ninguna query cross-tenant |
| III. Observabilidad | ✅ PASS | FR-011: histograma de latencia + contador de errores en `telemetry/mongo_metrics.go` |
| IV. Testing de integración | ✅ PASS | Tests de repos con DB efímera por test; `t.Skip()` si no hay MongoDB disponible |
| V. Versioning/Compatibilidad | ✅ N/A | No se exponen endpoints HTTP en esta feature |

**Resultado**: Sin violaciones. No se requiere tabla de Complexity Tracking.

---

## Project Structure

### Documentation (this feature)

```text
specs/006-mongo-infra/
├── plan.md              ← Este archivo
├── research.md          ← Phase 0 completada
├── data-model.md        ← Phase 1 completada
├── quickstart.md        ← Phase 1 completada
├── checklists/
│   └── requirements.md
└── tasks.md             ← Generado por /speckit.tasks (próximo paso)
```

### Source Code

```text
# Archivos nuevos
internal/
├── platform/
│   └── mongo/
│       └── client.go                   — Connect(cfg MongoConfig) (*mongo.Client, error)
│                                          Ping(ctx, client) error
├── domain/
│   └── aas/
│       ├── shell.go                    — AssetAdministrationShell, ShellUpdate, SubmodelRef
│       │                                  Administration, ShellRepository interface
│       └── submodel.go                 — Submodel, SubmodelElement, SemanticReference
│                                          SubmodelRepository interface
├── repo/
│   └── mongo/
│       ├── aas/
│       │   └── repository.go           — MongoShellRepository + ensureIndexes()
│       └── submodel/
│           └── repository.go           — MongoSubmodelRepository + ensureIndexes()
└── telemetry/
    └── mongo_metrics.go                — MongoOperationDuration, MongoOperationErrors

# Archivos modificados
internal/config/config.go               — +MongoConfig{URI, DB string}; +campo Mongo MongoConfig en Config
cmd/api/main.go                         — +bloque opcional de wiring MongoDB (patrón Redis)
internal/routes/url_mappings.go         — +parámetro mongoClient *mongo.Client en RegisterURLMappings
docker-compose.yml                      — +servicio mongo:7 sin auth

# Archivos de test nuevos
internal/repo/mongo/aas/repository_test.go
internal/repo/mongo/submodel/repository_test.go
internal/platform/mongo/client_test.go
```

---

## Phase 0: Research

**Status**: Completada — ver [research.md](research.md)

Resoluciones clave:
- Driver: `go.mongodb.org/mongo-driver/v2/mongo` (v2, API estabilizada)
- Wiring: patrón Redis de `main.go` (opcional con WARN)
- Métricas: `promauto.NewHistogramVec` + `promauto.NewCounterVec` en `telemetry/`
- Tests: DB efímera `test_<uuid>` con `t.Cleanup(db.Drop)`
- Errores: `mongo.ErrNoDocuments` → `domain.ErrNotFound`; `IsDuplicateKeyError` → `domain.ErrConflict`
- Librería AAS (`aas-core3.0-golang`): excluida de esta feature; structs Go propios en `domain/aas/`
- docker-compose: `mongo:7` sin autenticación; `MONGO_URI=mongodb://localhost:27017`

---

## Phase 1: Design & Contracts

**Status**: Completada

- **Data model**: [data-model.md](data-model.md) — tipos Go, interfaces de repo, schema BSON, índices, paginación
- **Contracts**: N/A — esta feature no expone endpoints HTTP; sin contratos API externos
- **Quickstart**: [quickstart.md](quickstart.md) — setup local, comandos Docker, troubleshooting

---

## Implementation Order (para /speckit.tasks)

Las tareas deben implementarse en este orden para respetar dependencias:

### Bloque 1 — Fundación (prerequisito de todo)
1. Agregar `MongoConfig` a `internal/config/config.go`
2. Crear `internal/platform/mongo/client.go` (Connect, Disconnect, Ping)
3. Crear `internal/telemetry/mongo_metrics.go` (MongoOperationDuration, MongoOperationErrors)
4. Actualizar `docker-compose.yml` con servicio `mongo:7`
5. Agregar dependencia `go.mongodb.org/mongo-driver/v2/mongo` vía Docker

### Bloque 2 — Dominio
6. Crear `internal/domain/aas/shell.go` (tipos + ShellRepository interface)
7. Crear `internal/domain/aas/submodel.go` (tipos + SubmodelRepository interface)

### Bloque 3 — Repositorios
8. Crear `internal/repo/mongo/aas/repository.go` (MongoShellRepository + índices)
9. Crear `internal/repo/mongo/submodel/repository.go` (MongoSubmodelRepository + índices)
10. Tests: `internal/repo/mongo/aas/repository_test.go`
11. Tests: `internal/repo/mongo/submodel/repository_test.go`

### Bloque 4 — Wiring
12. Actualizar `cmd/api/main.go` (bloque opcional MongoDB, patrón Redis)
13. Actualizar `internal/routes/url_mappings.go` (pasar mongoClient, registrar repos)
14. Actualizar healthcheck (`/ping`) para incluir estado de MongoDB

### Bloque 5 — Verificación
15. Build completo (`go build ./...`)
16. Tests de integración contra MongoDB local (`go test ./internal/repo/mongo/... -v`)
17. Verificar métricas en `/metrics`
18. Verificar comportamiento sin `MONGO_URI` (WARN + servidor sigue funcionando)

---

## Dependency Graph

```
config.MongoConfig
    └── platform/mongo.Connect()
            └── telemetry.mongo_metrics (instrumenta operaciones)
            └── domain/aas.ShellRepository (interface)
                    └── repo/mongo/aas.MongoShellRepository (impl)
            └── domain/aas.SubmodelRepository (interface)
                    └── repo/mongo/submodel.MongoSubmodelRepository (impl)
    └── main.go (wiring opcional)
            └── routes.RegisterURLMappings (recibe mongoClient)
```

---

## Variables de Entorno Nuevas

| Variable | Default | Requerida | Descripción |
|----------|---------|-----------|-------------|
| `MONGO_URI` | `""` | No | URI de conexión MongoDB. Sin valor → WARN + MongoDB deshabilitado |
| `MONGO_DB` | `""` | Sí si MONGO_URI definido | Nombre de la base de datos a usar |
| `MONGO_TEST_URI` | `""` | No (tests) | URI para tests de integración. Sin valor → `t.Skip()` |
