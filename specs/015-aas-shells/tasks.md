# Tasks: MongoDB Infrastructure Layer

**Input**: Design documents from `specs/006-mongo-infra/`
**Branch**: `006-mongo-infra`
**Prerequisites**: plan.md ✅ | spec.md ✅ | research.md ✅ | data-model.md ✅ | quickstart.md ✅

**Tests**: Los tests de integración de repositorios MongoDB (T008, T009, T012–T015, T017, T018, T020, T021, T025) están diferidos — se abordarán en una iteración posterior dedicada a testing. La implementación funcional está completa y validada manualmente vía Bruno.

**Organization**: Tareas agrupadas por User Story para habilitar implementación y verificación independiente de cada historia.

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Puede correr en paralelo (archivos diferentes, sin dependencias incompletas)
- **[Story]**: A qué user story pertenece (US1–US4)
- Todos los comandos Go se ejecutan vía Docker (Go no instalado en host macOS — ver CLAUDE.md)

---

## Phase 1: Setup (Infraestructura compartida)

**Purpose**: Agregar dependencia MongoDB y levantar el servicio en docker-compose.

- [x] T001 Agregar dependencia `go.mongodb.org/mongo-driver/v2/mongo` ejecutando: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go get go.mongodb.org/mongo-driver/v2/mongo && go mod tidy"`
- [x] T002 Agregar servicio `mongo:7` sin autenticación a `docker-compose.yml` (puerto 27017, volumen `mongo_data`, red `embolsadora_network`)

---

## Phase 2: Fundacional (Prerequisitos bloqueantes)

**Purpose**: Config, cliente MongoDB, métricas y tipos de dominio. Ninguna User Story puede empezar sin esta fase completa.

**⚠️ CRITICAL**: No comenzar ninguna User Story hasta completar T003–T007.

- [x] T003 Agregar `MongoConfig{URI, DB string}` a `internal/config/config.go` y campo `Mongo MongoConfig` en la struct `Config`; leer `MONGO_URI` y `MONGO_DB` con `getEnv()` (sin `require()` — son opcionales)
- [x] T004 Crear `internal/platform/mongo/client.go` con funciones `Connect(ctx, cfg MongoConfig) (*mongo.Client, error)` y `Ping(ctx, client)` siguiendo el patrón de `pgxpool.New()` en `main.go`; retornar error descriptivo si el ping falla
- [x] T005 [P] Crear `internal/telemetry/mongo_metrics.go` con `MongoOperationDuration` (`promauto.NewHistogramVec`, labels: `collection`, `operation`) y `MongoOperationErrors` (`promauto.NewCounterVec`, labels: `collection`, `operation`), siguiendo el patrón de `auth_metrics.go`
- [x] T006 [P] Crear `internal/domain/aas/shell.go` con tipos `AssetAdministrationShell`, `Administration`, `SubmodelRef`, `ShellUpdate` e interfaz `ShellRepository` con métodos `Create`, `GetByID`, `Update`, `Delete`, `ListByTenant(ctx, tenantID, limit, offset int) ([]*AAS, int64, error)` — ver `data-model.md` para firmas exactas
- [x] T007 [P] Crear `internal/domain/aas/submodel.go` con tipos `Submodel`, `SubmodelElement`, `SemanticReference`, `SemanticKey` e interfaz `SubmodelRepository` con métodos `Create`, `GetByID`, `ListByShell(ctx, tenantID, shellID, limit, offset int) ([]*Submodel, int64, error)`, `UpsertElement`, `Delete` — ver `data-model.md` para firmas exactas

**Checkpoint**: Config, cliente, métricas y tipos de dominio listos — US1 puede comenzar.

---

## Phase 3: US1 — Conexión controlada de MongoDB (Priority: P1) 🎯 MVP

**Goal**: El servidor arranca con o sin MongoDB; cierra limpiamente la conexión en shutdown; emite WARN si MONGO_URI no está definido.

**Independent Test**: Arrancar el servidor con `MONGO_URI` definido → verificar log "MongoDB connection established". Arrancar sin `MONGO_URI` → verificar log "WARN mongo disabled" y que el servidor sigue respondiendo en `/ping`.

### Tests — US1

> ⏸ **DIFERIDO** — Tests de integración pendientes para iteración posterior.

- [ ] T008 [P] [US1] Crear `internal/platform/mongo/client_test.go` con test de integración que verifica `Connect()` retorna cliente funcional cuando `MONGO_TEST_URI` está definido y `t.Skip()` si no lo está
- [ ] T009 [P] [US1] Agregar test en `internal/platform/mongo/client_test.go` que verifica que `Connect()` con URI inválido retorna error descriptivo (no panic)

### Implementación — US1

- [x] T010 [US1] Actualizar `cmd/api/main.go`: agregar bloque opcional de wiring MongoDB siguiendo el patrón Redis existente (líneas 49–60); si `cfg.Mongo.URI != ""` → llamar `platform_mongo.Connect()`; si falla → `log.Printf("WARN mongo disabled — connection failed: %v", err)`; si `cfg.Mongo.URI == ""` → `log.Println("WARN mongo disabled — MONGO_URI not set")`; agregar `defer mongoClient.Disconnect()` si el cliente es no-nil
- [x] T011 [US1] Actualizar firma de `internal/routes/url_mappings.go` `RegisterURLMappings()` para aceptar `mongoClient *mongo.Client` como parámetro adicional (puede ser nil); actualizar la llamada en `main.go`

**Checkpoint**: Servidor arranca con y sin MONGO_URI. `go build ./...` pasa. US1 completa y testeable.

---

## Phase 4: US2 — CRUD de Asset Administration Shell (Priority: P1)

**Goal**: Repositorio completo para crear, leer, actualizar, eliminar y listar shells AAS en MongoDB, con aislamiento multi-tenant, índices y métricas Prometheus.

**Independent Test**: Test de integración en `internal/repo/mongo/aas/repository_test.go` — crear shell, leer por ID, actualizar campo, borrar; verificar aislamiento con dos tenantIDs distintos; verificar error `ErrConflict` al duplicar `globalAssetId` dentro del mismo tenant.

### Tests — US2

> ⏸ **DIFERIDO** — Tests de integración pendientes para iteración posterior.

- [ ] T012 [P] [US2] Crear `internal/repo/mongo/aas/repository_test.go` con función `setupTestDB(t, client)` que crea DB `test_<uuid>` y la borra en `t.Cleanup(db.Drop)`; agregar test de `Create`: verifica persistencia y campos `createdAt`/`updatedAt`; verificar que `GetByID` retorna el shell creado
- [ ] T013 [P] [US2] Agregar en `repository_test.go`: test de `Update` (solo campos provistos se modifican; `updatedAt` cambia); test de `Delete` (documento ya no existe después)
- [ ] T014 [P] [US2] Agregar en `repository_test.go`: test de `ListByTenant` con paginación (`limit=2, offset=0` sobre 3 shells → 2 resultados, total=3); verificar que el total count es correcto
- [ ] T015 [P] [US2] Agregar en `repository_test.go`: test de aislamiento multi-tenant — dos tenants con mismo `globalAssetId`; verificar que `GetByID` de cada tenant solo retorna sus propios documentos; verificar `ErrConflict` al duplicar `globalAssetId` dentro del mismo tenant

### Implementación — US2

- [x] T016 [US2] Crear `internal/repo/mongo/aas/repository.go` con `MongoShellRepository` que implementa `domain/aas.ShellRepository`: `ensureIndexes()` (índice único `{tenantId,globalAssetId}` + índice `{tenantId,updatedAt}`), `Create`, `GetByID`, `Update`, `Delete`, `ListByTenant`; mapear `mongo.ErrNoDocuments` → `domain.ErrNotFound` y `mongo.IsDuplicateKeyError` → `domain.ErrConflict`; instrumentar cada método con `telemetry.MongoOperationDuration` y `telemetry.MongoOperationErrors`

**Checkpoint**: CRUD de shells funcional con tests verdes. `go test ./internal/repo/mongo/aas/... -v` pasa. US2 completa e independientemente testeable.

---

## Phase 5: US3 — CRUD de Submodelos (Priority: P2)

**Goal**: Repositorio de submodelos vinculados a un shell padre por `shellId`, con `UpsertElement` atómico para actualizar elementos individuales sin tocar el resto del submodelo.

**Independent Test**: Test de integración en `internal/repo/mongo/submodel/repository_test.go` — crear submodelo con `shellId` de prueba, listar por shell, hacer `UpsertElement` sobre un Property, verificar que solo ese elemento cambia, borrar y verificar que `GetByID` retorna nil.

### Tests — US3

> ⏸ **DIFERIDO** — Tests de integración pendientes para iteración posterior.

- [ ] T017 [P] [US3] Crear `internal/repo/mongo/submodel/repository_test.go` con `setupTestDB` (reutilizar patrón de US2); test de `Create` y `GetByID`; test de `ListByShell` con paginación (2 submodelos, limit=1 → 1 resultado, total=2)
- [ ] T018 [P] [US3] Agregar en `repository_test.go`: test de `UpsertElement` — submodelo con 2 Properties; hacer upsert en una Property; verificar que solo esa Property cambió de valor y la otra permanece intacta; test de `Delete` con `GetByID` post-delete retornando nil

### Implementación — US3

- [x] T019 [US3] Crear `internal/repo/mongo/submodel/repository.go` con `MongoSubmodelRepository` que implementa `domain/aas.SubmodelRepository`: `ensureIndexes()` (índice único `{tenantId,shellId,idShort}` + índice `{tenantId,shellId}`), `Create`, `GetByID`, `ListByShell`, `Delete`; para `UpsertElement` usar operación atómica de MongoDB (`$set` sobre el array de elementos por `idShort`); mapear errores a `domain.ErrNotFound`/`ErrConflict`; instrumentar con métricas Prometheus

**Checkpoint**: CRUD de submodelos funcional con tests verdes. `go test ./internal/repo/mongo/submodel/... -v` pasa. US3 completa e independientemente testeable.

---

## Phase 6: US4 — Healthcheck de MongoDB (Priority: P3)

**Goal**: El endpoint `/ping` incluye el estado de MongoDB (`"ok"` o `"degraded"`) sin retornar 5xx cuando MongoDB no está disponible.

**Independent Test**: `curl localhost:8080/ping` con MongoDB corriendo → sección `mongo.status: "ok"`. Detener MongoDB → sección `mongo.status: "degraded"` con HTTP 200.

### Tests — US4

> ⏸ **DIFERIDO** — Tests de integración pendientes para iteración posterior.

- [ ] T020 [P] [US4] Crear test de integración en `internal/routes/health_test.go` (o archivo apropiado según la ubicación del handler `/ping`): verificar que con cliente MongoDB no-nil y MongoDB accesible la respuesta incluye `"mongo":{"status":"ok"}`
- [ ] T021 [P] [US4] Agregar test en el mismo archivo: con `mongoClient` nil (no configurado) la respuesta incluye `"mongo":{"status":"disabled"}` con HTTP 200; con cliente no-nil pero MongoDB caído incluye `"mongo":{"status":"degraded"}` con HTTP 200

### Implementación — US4

- [x] T022 [US4] Localizar el handler del endpoint `/ping` en `internal/routes/url_mappings.go` (línea 43–45 actual) y extraerlo a una función `healthHandler(db *pgxpool.Pool, redisClient *redis.Client, mongoClient *mongo.Client)` que retorna JSON con secciones `postgres`, `redis` y `mongo`; hacer ping con timeout de 1s a cada servicio; retornar siempre HTTP 200 (falla open)

**Checkpoint**: Healthcheck completo. `curl /ping` retorna estado de los tres stores. US4 completa.

---

## Phase 7: Polish & Verificación transversal

**Purpose**: Verificación end-to-end, variables de entorno y documentación.

- [x] T023 Ejecutar `go build ./...` vía Docker y corregir cualquier error de compilación: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go build ./..."`
- [x] T024 [P] Agregar `MONGO_URI`, `MONGO_DB` y `MONGO_TEST_URI` a `.env.example` con valores de desarrollo y comentarios explicativos
- [ ] T025 [P] ⏸ **DIFERIDO** — Ejecutar suite completa de tests de integración de repositorios MongoDB vía Docker: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e MONGO_TEST_URI=mongodb://host.docker.internal:27017 golang:1.24-alpine sh -c "go test ./internal/repo/mongo/... ./internal/platform/mongo/... -v -count=1"`
- [x] T026 Verificar comportamiento de arranque: (a) con `MONGO_URI` definido → log "MongoDB connection established" + `/ping` con `mongo.status: ok`; (b) sin `MONGO_URI` → log "WARN mongo disabled" + servidor responde en todos los endpoints no-Mongo

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sin dependencias — empezar de inmediato
- **Foundational (Phase 2)**: Depende de Phase 1 — **bloquea todas las User Stories**
- **US1 (Phase 3)**: Depende de Phase 2 completa
- **US2 (Phase 4)**: Depende de Phase 2 + Phase 3 (necesita mongoClient en routes)
- **US3 (Phase 5)**: Depende de Phase 2 (puede ir en paralelo con US2 tras Phase 3 si hay capacidad)
- **US4 (Phase 6)**: Depende de Phase 3 (necesita healthHandler con mongoClient)
- **Polish (Phase 7)**: Depende de todas las fases anteriores

### User Story Dependencies

- **US1 (P1)**: Puede empezar al completar Phase 2 — sin dependencias en otras stories
- **US2 (P1)**: Requiere US1 completa (usa `mongoClient` establecido en routes)
- **US3 (P2)**: Puede ir en paralelo con US2 — comparte fundación, no comparte archivos de implementación
- **US4 (P3)**: Requiere US1 completa (healthHandler necesita el cliente)

### Within Each User Story

- Tests de integración ANTES de implementación (escribir tests, verificar que fallan, implementar)
- Tests marcados [P] dentro de la misma story pueden escribirse en paralelo
- `ensureIndexes()` siempre primero en cada repositorio
- Mapeo de errores siempre incluido en la implementación (no como paso posterior)

---

## Parallel Opportunities

### Phase 2 (Foundacional) — tras T003+T004:
```
T005: telemetry/mongo_metrics.go   ← en paralelo
T006: domain/aas/shell.go          ← en paralelo
T007: domain/aas/submodel.go       ← en paralelo
```

### Phase 4 (US2) — tests pueden escribirse antes de implementar:
```
T012: test Create + GetByID         ← en paralelo (mismo archivo, secciones distintas)
T013: test Update + Delete          ← en paralelo
T014: test ListByTenant             ← en paralelo
T015: test aislamiento multi-tenant ← en paralelo
```
Luego T016 (implementación) una vez los tests estén escritos.

### Phase 5 (US3) — puede ir en paralelo con US2 si hay dos desarrolladores:
```
Developer A: T012–T016 (US2 shell repo)
Developer B: T017–T019 (US3 submodel repo)
```

---

## Implementation Strategy

### MVP First (US1 + US2 únicamente)

1. Completar Phase 1: Setup (T001–T002)
2. Completar Phase 2: Foundacional (T003–T007)
3. Completar Phase 3: US1 — conexión (T008–T011)
4. Completar Phase 4: US2 — shell CRUD (T012–T016)
5. **STOP y VALIDAR**: `go test ./internal/repo/mongo/aas/... -v` verde + servidor arranca con/sin Mongo
6. El proyecto ya tiene MongoDB listo para ser consumido por features futuras

### Incremental Delivery

1. Setup + Fundacional → fundación lista
2. US1 → conexión MongoDB opcional funcionando
3. US2 → CRUD de shells completo (demo-able)
4. US3 → CRUD de submodelos completo
5. US4 → healthcheck completo → Polish

### Parallel Team Strategy (2 desarrolladores)

1. Ambos completan Phase 1 + Phase 2 juntos
2. Tras completar US1 (T008–T011):
   - Dev A: US2 (T012–T016) — shell repository
   - Dev B: US3 + US4 (T017–T022) — submodel repository + healthcheck
3. Polish (T023–T026) juntos al final

---

## Notes

- `[P]` = archivos diferentes, sin dependencias incompletas entre sí
- `[USn]` mapea cada tarea a la user story del spec para trazabilidad
- Todos los comandos Go requieren contenedor Docker (ver CLAUDE.md)
- `MONGO_TEST_URI` para tests de integración; si no está definido → `t.Skip()` (tests no fallan en CI sin MongoDB)
- Cada repositorio debe llamar `ensureIndexes()` en su constructor (idempotente)
- `domain.ErrNotFound` y `domain.ErrConflict` deben existir en `internal/domain/errors.go` — verificar antes de T016/T019
- Hacer commit tras cada fase o checkpoint para facilitar rollback
