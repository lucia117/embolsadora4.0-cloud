# Tasks: Log Service API (009)

**Input**: Design documents from `specs/009-log-service/`
**Branch**: `009-log-service`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Puede ejecutarse en paralelo con otras tareas [P] del mismo bloque (archivos distintos, sin dependencias incompletas)
- **[Story]**: A qué user story corresponde (US1–US5)
- Todas las tareas incluyen ruta de archivo exacta

---

## Phase 1: Setup

**Purpose**: Estructura de directorios para la feature

- [X] T001 Crear estructura de directorios: `internal/app/logs/`, `internal/repo/pg/logs/`, `internal/api/handler/logs/dto/`, `internal/platform/logwriter/`, `internal/telemetry/` (si no existe)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Infraestructura compartida que deben completar TODAS las user stories

**⚠️ CRÍTICO**: Ninguna user story puede comenzar hasta completar esta fase

- [X] T002 Crear migración `migrations/000015_create_log_entries_table.up.sql`: tabla `log_entries` (id UUID PK DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), severity VARCHAR(20) NOT NULL CHECK IN (info/warning/critical/error), event_type VARCHAR(50) NOT NULL CHECK IN (alarm_triggered/alarm_resolved/device_connected/device_disconnected/device_state_changed/user_action/system), source_id UUID, machine_id UUID, message TEXT NOT NULL, metadata JSONB NOT NULL DEFAULT '{}') + tabla `log_retention_policies` (tenant_id UUID PK, retention_days INT NOT NULL DEFAULT 90 CHECK > 0, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), next_purge_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '1 day') + trigger updated_at en log_retention_policies + índices: idx_log_entries_tenant_cursor ON log_entries(tenant_id, created_at DESC, id DESC), idx_log_entries_machine ON log_entries(tenant_id, machine_id) WHERE machine_id IS NOT NULL, idx_log_entries_severity ON log_entries(tenant_id, severity, created_at DESC), idx_log_entries_event_type ON log_entries(tenant_id, event_type, created_at DESC), idx_log_entries_message_fts USING gin(to_tsvector('spanish', message))

- [X] T003 Crear migración `migrations/000015_create_log_entries_table.down.sql`: DROP TABLE IF EXISTS log_retention_policies; DROP TABLE IF EXISTS log_entries;

- [X] T004 [P] Crear `internal/domain/logs.go`: struct `LogEntry` (ID, TenantID, CreatedAt, Severity, EventType, SourceID *uuid.UUID, MachineID *uuid.UUID, Message, Metadata map[string]any) + struct `RetentionPolicy` (TenantID, RetentionDays int, UpdatedAt, NextPurgeAt) + type `Severity` string con constantes (SeverityInfo/Warning/Critical/Error) + type `EventType` string con constantes (EventTypeAlarmTriggered/AlarmResolved/DeviceConnected/DeviceDisconnected/DeviceStateChanged/UserAction/System) + errores: ErrLogNotFound = errors.New("log not found"), ErrInvalidCursor = errors.New("invalid cursor"), ErrInvalidDateRange = errors.New("from must be before to")

- [X] T005 [P] Crear `internal/platform/logwriter/writer.go`: interface `LogWriter { Write(ctx context.Context, entry *domain.LogEntry) error }` — write path interno para otros servicios del sistema (placeholder para integración futura)

- [X] T006 Crear `internal/repo/pg/logs/repository.go`: interface `Repository` con métodos List(ctx, tenantID, params ListParams) ([]*domain.LogEntry, nextCursor string, total int, error), Get(ctx, id, tenantID uuid.UUID) (*domain.LogEntry, error), GetContext(ctx, id, tenantID uuid.UUID, windowSize int) (before, after []*domain.LogEntry, error), Export(ctx, tenantID uuid.UUID, params ExportParams) ([]*domain.LogEntry, truncated bool, totalAvailable int, error), GetRetention(ctx, tenantID uuid.UUID) (*domain.RetentionPolicy, error), UpsertRetention(ctx, policy *domain.RetentionPolicy) error + struct `ListParams` (EventType, Severity, MachineID *string, From, To *time.Time, Q *string, Cursor string, Limit int) + struct `ExportParams` (mismos filtros sin cursor) + struct `PostgresRepository` con pool *pgxpool.Pool + constructor `NewPostgresRepository` + implementación de todos los métodos: List usa paginación keyset `WHERE (created_at, id) < ($cursor_ts, $cursor_id)` con cursor codificado en base64 JSON; Get busca por id AND tenant_id; GetContext usa UNION de N previos + N posteriores; Export limita a 50000 registros y retorna truncated=true si hay más; GetRetention retorna política o default si no existe; UpsertRetention hace INSERT ON CONFLICT (tenant_id) DO UPDATE

- [X] T007 [P] Crear `internal/api/handler/logs/dto/request.go`: struct `ListLogsParams` con campos json/query tags (EventType, Severity, MachineID, From, To, Q string, Cursor string, Limit int default 50) + struct `ExportLogsParams` (mismos filtros sin cursor + Format string default "json") + struct `UpdateRetentionRequest` (RetentionDays int json:"retention_days" binding:"required,min=1,max=3650") + función `DecodeCursor(s string) (time.Time, uuid.UUID, error)` y `EncodeCursor(t time.Time, id uuid.UUID) string` usando base64 JSON

- [X] T008 [P] Crear `internal/api/handler/logs/dto/response.go`: struct `LogResponse` (todos los campos de LogEntry con json tags, SourceID/MachineID como *string nullable) + función `LogResponseFromDomain(e *domain.LogEntry) LogResponse` + struct `LogListResponse` (Data []LogResponse, NextCursor *string json:"next_cursor", Total int) + struct `LogExportResponse` (Data []LogResponse, Truncated bool, ExportedCount int json:"exported_count", TotalAvailable int json:"total_available") + struct `LogContextResponse` (Before []LogResponse, Anchor LogResponse, After []LogResponse) + struct `RetentionResponse` (TenantID, RetentionDays int, NextPurgeAt, UpdatedAt con json tags)

- [X] T009 Crear `internal/app/logs/service.go`: struct `Service` con repo `Repository` interface + logger `*zap.Logger` + hub `map[uuid.UUID][]chan *domain.LogEntry` + mu `sync.RWMutex` + constructor `New(repo Repository, logger *zap.Logger) *Service` — dejar métodos vacíos con TODO para completar en fases siguientes

- [X] T010 [P] Crear `internal/telemetry/log_metrics.go`: `var LogListTotal = promauto.NewCounterVec(prometheus.CounterOpts{Name:"log_list_total",...}, []string{"tenant"})` + `var LogListLatency = promauto.NewHistogramVec(...)` + `var LogExportTotal = promauto.NewCounterVec(...)` + `var LogStreamConnections = promauto.NewGaugeVec(...)`

**Checkpoint**: Fundación lista — las user stories pueden comenzar

---

## Phase 3: User Story 1 — Consultar historial de eventos (Priority: P1) 🎯 MVP

**Goal**: `GET /api/v1/logs` con filtros, paginación por cursor y caso 401

**Independent Test**: Ejecutar Pacts 1–6 del quickstart.md (sin auth → 401, con filtros → 200, texto → 200, máquina → 200, cursor → 200, sin resultados → 200)

- [X] T011 [US1] Implementar `service.List` en `internal/app/logs/service.go`: decodificar cursor (DecodeCursor), construir ListParams hacia repo, encodear next_cursor si hay más resultados, instrumentar con LogListLatency, loguear con zap

- [X] T012 [US1] Crear `internal/api/handler/logs/list_logs.go`: handler `ListLogs` que parsea query params en `ListLogsParams`, llama `service.List`, retorna `LogListResponse` + manejo de errores (ErrInvalidCursor → 400, ErrInvalidDateRange → 400)

- [X] T013 [US1] Crear `internal/api/handler/logs/routes.go`: función `RegisterRoutes(readGroup, writeGroup *gin.RouterGroup, svc *logs.Service)` registrando rutas en ESTE ORDEN EXACTO (crítico para Gin): readGroup.GET("/logs/retention", ...), readGroup.GET("/logs/stream", ...), readGroup.GET("/logs/export", ...), readGroup.GET("/logs", ...), readGroup.GET("/logs/:id/context", ...), readGroup.GET("/logs/:id", ...) + writeGroup.PATCH("/logs/retention", ...)

- [X] T014 [US1] Agregar logs route wiring en `internal/routes/url_mappings.go`: importar `logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"`, `logsApp "github.com/tu-org/embolsadora-api/internal/app/logs"`, `logsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/logs"` + instanciar `logsRepo.NewPostgresRepository(db)`, `logsApp.New(logsRepo, logger)` + llamar `logsHandler.RegisterRoutes(readGroup, writeGroup, logsService)` dentro del bloque de middleware `JWTAuth → TenantFromHeader`

**Checkpoint**: `GET /api/v1/logs` funcional. Pacts 1–6 pasan.

---

## Phase 4: User Story 2 — Detalle y contexto (Priority: P2)

**Goal**: `GET /api/v1/logs/:id` (200 y 404) + `GET /api/v1/logs/:id/context`

**Independent Test**: Ejecutar Pacts 7–9 del quickstart.md (GET por ID existente → 200, inexistente → 404, context → 200 con before/anchor/after)

- [X] T015 [US2] Implementar `service.Get` y `service.GetContext` en `internal/app/logs/service.go`: Get retorna ErrLogNotFound si repo retorna pgx.ErrNoRows; GetContext arma LogContextResponse con before/anchor/after

- [X] T016 [P] [US2] Crear `internal/api/handler/logs/get_log.go`: handler `GetLog` que extrae `:id` como UUID, llama `service.Get`, retorna LogResponse o 404 si ErrLogNotFound

- [X] T017 [P] [US2] Crear `internal/api/handler/logs/get_log_context.go`: handler `GetLogContext` que extrae `:id` + query param `window_size` (default 10, clamp 1–50), llama `service.GetContext`, retorna LogContextResponse

**Checkpoint**: GET /logs/:id y /logs/:id/context funcionales. Pacts 7–9 pasan.

---

## Phase 5: User Story 3 — Exportar logs (Priority: P3)

**Goal**: `GET /api/v1/logs/export` (normal y truncado)

**Independent Test**: Ejecutar Pacts 13–14 del quickstart.md (export con datos → 200 truncated:false, export con >50k → 200 truncated:true)

- [X] T018 [US3] Implementar `service.Export` en `internal/app/logs/service.go`: llamar repo.Export, construir LogExportResponse con truncated/exported_count/total_available, instrumentar con LogExportTotal

- [X] T019 [US3] Crear `internal/api/handler/logs/export_logs.go`: handler `ExportLogs` que parsea `ExportLogsParams`, llama `service.Export`, retorna LogExportResponse; si format=csv serializar como CSV con header row

**Checkpoint**: GET /logs/export funcional. Pacts 13–14 pasan.

---

## Phase 6: User Story 4 — Streaming SSE (Priority: P4)

**Goal**: `GET /api/v1/logs/stream` con Server-Sent Events

**Independent Test**: Ejecutar Pact 12 del quickstart.md (conexión SSE abierta → recibe eventos nuevos en tiempo real + heartbeat cada 30s)

- [X] T020 [US4] Implementar `service.Subscribe` y `service.Unsubscribe` en `internal/app/logs/service.go`: hub de canales `map[uuid.UUID][]chan *domain.LogEntry` protegido con `sync.RWMutex`; Subscribe crea canal y lo agrega al slice del tenant; Unsubscribe lo elimina y cierra; método interno `Publish(tenantID, entry)` envía a todos los canales del tenant sin bloquear (select con default)

- [X] T021 [US4] Implementar `service.WriteAndPublish` en `internal/app/logs/service.go` (implementa LogWriter interface): persiste en repo Y publica en hub SSE

- [X] T022 [US4] Crear `internal/api/handler/logs/stream_logs.go`: handler `StreamLogs` con headers `Cache-Control: no-cache`, `Connection: keep-alive`; extrae tenantID del contexto; llama `service.Subscribe` + defer `Unsubscribe`; usa `c.Stream(func(w io.Writer) bool { select { case event: c.SSEvent("log", dto.LogResponseFromDomain(event)); return true; case <-time.After(30s): c.SSEvent("heartbeat", ""); return true; case <-ctx.Done(): return false } })`; instrumentar LogStreamConnections (+1 al conectar, -1 al desconectar)

**Checkpoint**: GET /logs/stream funcional con SSE. Pact 12 pasa.

---

## Phase 7: User Story 5 — Gestionar retención (Priority: P5)

**Goal**: `GET /api/v1/logs/retention` + `PATCH /api/v1/logs/retention`

**Independent Test**: Ejecutar Pacts 10–11 del quickstart.md (GET retention → 200 con defaults, PATCH retention → 200 con nuevo valor)

- [X] T023 [US5] Implementar `service.GetRetention` y `service.UpdateRetention` en `internal/app/logs/service.go`: GetRetention retorna política del repo o RetentionPolicy con defaults (90 días) si no existe; UpdateRetention valida retention_days > 0, calcula next_purge_at = NOW() + INTERVAL '1 day', llama repo.UpsertRetention

- [X] T024 [P] [US5] Crear `internal/api/handler/logs/get_retention.go`: handler `GetRetention` que llama `service.GetRetention` y retorna RetentionResponse

- [X] T025 [P] [US5] Crear `internal/api/handler/logs/update_retention.go`: handler `UpdateRetention` que parsea `UpdateRetentionRequest` (binding:"required"), llama `service.UpdateRetention`, retorna RetentionResponse actualizada; mapear errores de validación a 400

**Checkpoint**: GET y PATCH /logs/retention funcionales. Pacts 10–11 pasan.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentación, integración final y validación completa

- [X] T026 [P] Crear `postman/Log-Service-API.postman_collection.json` con 14 requests Pact: 6 variantes de GET /logs (401 sin auth, con filtros, texto, máquina, cursor, sin resultados) + GET /logs/:id (200 y 404) + GET /logs/:id/context + GET /logs/retention + PATCH /logs/retention + GET /logs/stream + GET /logs/export (normal y truncado); incluir variables de collection: `baseUrl`, `jwt_token`, `tenant_id`, `log_id`, `machine_id`

- [X] T027 [P] Actualizar `docs/openapi.yaml`: agregar paths `/logs`, `/logs/{id}`, `/logs/{id}/context`, `/logs/retention`, `/logs/stream`, `/logs/export` con todos los query params, responses y schemas de `specs/009-log-service/contracts/log-service-api.openapi.yaml`

- [X] T028 Ejecutar validación manual con `specs/009-log-service/quickstart.md`: verificar los 14 Pacts contra servidor local, confirmar que todos retornan los status codes esperados

---

## Dependencies & Execution Order

### Dependencias entre fases

- **Phase 1 (Setup)**: Sin dependencias — puede iniciar de inmediato
- **Phase 2 (Foundational)**: Depende de Phase 1
  - T002–T003 (migración): independientes entre sí [P]
  - T004–T005 (domain + logwriter): independientes entre sí [P]
  - T006 (repository): depende de T004 (domain)
  - T007–T008 (DTOs): dependen de T004 [P entre sí]
  - T009 (service skeleton): depende de T006
  - T010 (metrics): independiente [P]
- **Phase 3 (US1)**: Depende de Phase 2 completa
- **Phase 4 (US2)**: Depende de Phase 3 (routes.go y wiring ya registrados)
- **Phase 5 (US3)**: Depende de Phase 2 (service skeleton + DTOs)
- **Phase 6 (US4)**: Depende de Phase 2 (service skeleton)
- **Phase 7 (US5)**: Depende de Phase 2 (service skeleton + DTOs)
- **Phase 8 (Polish)**: Depende de todas las fases anteriores

### Dependencias entre User Stories

- **US1 (P1)**: Bloquea el resto — establece el router wiring y service base
- **US2, US3, US4, US5**: Pueden implementarse en paralelo una vez US1 esté completo

### Paralelos dentro de cada fase

- Phase 2: T002+T003 [P], T004+T005 [P], T007+T008 [P], T010 [P]
- Phase 4: T016+T017 [P]
- Phase 7: T024+T025 [P]
- Phase 8: T026+T027 [P]

---

## Parallel Example: Phase 2

```text
Batch A (sin dependencias):
  T002 - Migración .up.sql
  T003 - Migración .down.sql
  T004 - domain/logs.go
  T005 - logwriter/writer.go
  T010 - telemetry/log_metrics.go

Batch B (depende de T004):
  T006 - repo/pg/logs/repository.go
  T007 - handler/logs/dto/request.go
  T008 - handler/logs/dto/response.go

Batch C (depende de T006):
  T009 - app/logs/service.go skeleton
```

---

## Implementation Strategy

### MVP (solo US1 — Pacts 1–6)

1. Phase 1: Setup (T001)
2. Phase 2: Foundational (T002–T010)
3. Phase 3: US1 (T011–T014)
4. **STOP y VALIDAR**: Ejecutar Pacts 1–6 del quickstart.md
5. Si pasan → continuar con US2–US5

### Incremental por User Story

1. Setup + Foundational → base lista
2. US1 → `GET /logs` funcional → validar Pacts 1–6
3. US2 → `GET /logs/:id` + context → validar Pacts 7–9
4. US3 → export → validar Pacts 13–14
5. US4 → SSE → validar Pact 12
6. US5 → retention → validar Pacts 10–11
7. Polish → Postman + OpenAPI + validación completa

---

## Notes

- Migración 000004 puede necesitar renumerarse a 000015 si PR #25 (008-alarm-rules) se mergea antes a main
- Orden de rutas en `routes.go` es CRÍTICO: `/logs/retention`, `/logs/stream`, `/logs/export` antes que `/logs/:id`
- El SSE (US4) es la historia de mayor complejidad; el hub in-memory se resetea al reiniciar el servidor
- `service.WriteAndPublish` implementa `LogWriter` interface — integración con otros servicios es trabajo futuro
- Pattern de repo: seguir `internal/repo/pg/roles/repository.go`
- Pattern de service: seguir `internal/app/roles/service.go`
- Pattern de handler: seguir `internal/api/handler/alarm_rules/`
