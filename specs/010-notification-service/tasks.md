# Tasks: Notification Service API (010)

**Input**: Design documents from `specs/010-notification-service/`
**Branch**: `010-notification-service`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## Format: `[ID] [P?] [Story?] Description`

- **[P]**: Puede ejecutarse en paralelo con otras tareas [P] del mismo bloque (archivos distintos, sin dependencias incompletas)
- **[Story]**: A qué user story corresponde (US1–US4)
- Todas las tareas incluyen ruta de archivo exacta

---

## Phase 1: Setup

**Purpose**: Estructura de directorios para la feature

- [X] T001 Crear estructura de directorios: `internal/app/notifications/`, `internal/repo/pg/notifications/`, `internal/api/handler/notifications/dto/`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Infraestructura compartida que deben completar TODAS las user stories

**⚠️ CRÍTICO**: Ninguna user story puede comenzar hasta completar esta fase

- [X] T002 [P] Crear migración `migrations/000016_create_notifications_table.up.sql`: tabla `notifications` (id UUID PK DEFAULT gen_random_uuid(), tenant_id UUID NOT NULL, title TEXT NOT NULL, message TEXT NOT NULL, severity VARCHAR(20) NOT NULL CHECK IN (info/warning/critical/error), status VARCHAR(20) NOT NULL DEFAULT 'unread' CHECK IN (unread/acknowledged/closed), alarm_rule_id UUID, machine_id UUID, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), acknowledged_at TIMESTAMPTZ, closed_at TIMESTAMPTZ) + índices: idx_notifications_tenant_list ON notifications(tenant_id, created_at DESC), idx_notifications_tenant_status ON notifications(tenant_id, status, created_at DESC), idx_notifications_tenant_severity ON notifications(tenant_id, severity, created_at DESC). Sin FK constraints en alarm_rule_id ni machine_id (referencias históricas). Sin soft-delete ni updated_at.

- [X] T003 [P] Crear migración `migrations/000016_create_notifications_table.down.sql`: DROP TABLE IF EXISTS notifications;

- [X] T004 [P] Crear `internal/domain/notifications.go`: type `NotificationStatus string` con constantes StatusUnread="unread", StatusAcknowledged="acknowledged", StatusClosed="closed" + type `NotificationSeverity string` con constantes SeverityInfo="info", SeverityWarning="warning", SeverityCritical="critical", SeverityError="error" + struct `Notification` (ID, TenantID uuid.UUID; Title, Message string; Severity NotificationSeverity; Status NotificationStatus; AlarmRuleID *uuid.UUID; MachineID *uuid.UUID; CreatedAt time.Time; AcknowledgedAt *time.Time; ClosedAt *time.Time) + var `ErrNotificationNotFound = errors.New("notificación no encontrada")`

- [X] T005 Crear `internal/repo/pg/notifications/repository.go`: struct `ListParams` (Status *string, Severity *string, Limit int, Offset int) + interface `Repository` con métodos List(ctx, tenantID uuid.UUID, params ListParams) ([]*domain.Notification, int, error), CountUnread(ctx, tenantID uuid.UUID) (int, error), GetByID(ctx, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error), Ack(ctx, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error), Close(ctx, id uuid.UUID, tenantID uuid.UUID) (*domain.Notification, error) + struct `PostgresRepository` con pool *pgxpool.Pool + constructor `New(pool *pgxpool.Pool) *PostgresRepository` + implementación de todos los métodos: List usa WHERE tenant_id=$1 AND ($2::text IS NULL OR status=$2) AND ($3::text IS NULL OR severity=$3) ORDER BY created_at DESC LIMIT $4 OFFSET $5 + COUNT(*) para total; CountUnread usa SELECT COUNT(*) WHERE tenant_id=$1 AND status='unread'; GetByID busca por id AND tenant_id y mapea pgx.ErrNoRows a ErrNotificationNotFound; Ack hace UPDATE SET status='acknowledged', acknowledged_at=CASE WHEN acknowledged_at IS NULL THEN NOW() ELSE acknowledged_at END WHERE id=$1 AND tenant_id=$2 RETURNING * (idempotente: si ya está acknowledged/closed, retorna sin error); Close hace UPDATE SET status='closed', closed_at=CASE WHEN closed_at IS NULL THEN NOW() ELSE closed_at END WHERE id=$1 AND tenant_id=$2 RETURNING * (idempotente)

- [X] T006 [P] Crear `internal/api/handler/notifications/dto/request.go`: struct `ListNotificationsParams` con campos query tag (Status string `form:"status"`, Severity string `form:"severity"`, Limit int `form:"limit" default:"20"`, Offset int `form:"offset" default:"0"`)

- [X] T007 [P] Crear `internal/api/handler/notifications/dto/response.go`: struct `NotificationResponse` (ID, TenantID, AlarmRuleID *string, MachineID *string, Title, Message, Severity, Status string, CreatedAt time.Time, AcknowledgedAt *time.Time, ClosedAt *time.Time — todos con json tags snake_case) + función `FromDomain(n *domain.Notification) NotificationResponse` + struct `NotificationListResponse` (Data []NotificationResponse `json:"data"`, Total int `json:"total"`, Limit int `json:"limit"`, Offset int `json:"offset"`) + struct `NotificationCountResponse` (Unread int `json:"unread"`)

- [X] T008 Crear `internal/app/notifications/service.go`: struct `Service` con repo `Repository` interface + logger `*zap.Logger` + constructor `New(repo Repository, logger *zap.Logger) *Service` — dejar métodos vacíos con TODO para completar en fases siguientes

- [X] T009 [P] Crear `internal/telemetry/notification_metrics.go`: `var NotificationOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{Name:"notification_operations_total", Help:"Total de operaciones en el servicio de notificaciones"}, []string{"operation","tenant"})` + `var NotificationOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{Name:"notification_operation_duration_seconds",...}, []string{"operation"})` — siguiendo el patrón de log_metrics.go

**Checkpoint**: Fundación lista — las user stories pueden comenzar

---

## Phase 3: User Story 1 — Consultar notificaciones del tenant (Priority: P1) 🎯 MVP

**Goal**: `GET /api/v1/notifications` con paginación/filtros, `GET /api/v1/notifications/count` y caso 401

**Independent Test**: Ejecutar Pacts 1–3 del quickstart.md (GET lista → 200, GET sin auth → 401, GET count → 200 con conteo correcto)

- [X] T010 [US1] Implementar `service.List` y `service.CountUnread` en `internal/app/notifications/service.go`: List llama repo.List con params, loguea con zap (tenant_id, filtros aplicados, total retornado), instrumenta NotificationOperationDuration; CountUnread llama repo.CountUnread y loguea

- [X] T011 [P] [US1] Crear `internal/api/handler/notifications/list_notifications.go`: handler `ListNotifications` que extrae tenantID del contexto (platform.TenantID), parsea query params en `ListNotificationsParams` (defaults: Limit=20, Offset=0, clamp Limit max=100), llama `service.List`, retorna `NotificationListResponse` con 200

- [X] T012 [P] [US1] Crear `internal/api/handler/notifications/count_notifications.go`: handler `CountNotifications` que extrae tenantID del contexto, llama `service.CountUnread`, retorna `NotificationCountResponse` con 200

- [X] T013 [US1] Crear `internal/api/handler/notifications/routes.go`: función `RegisterRoutes(g *gin.RouterGroup, service *notifications.Service)` registrando rutas en ESTE ORDEN EXACTO (crítico para Gin — rutas estáticas antes de parámetros): g.GET("/notifications/count", CountNotifications(service)), g.GET("/notifications", ListNotifications(service)), g.GET("/notifications/:id", placeholder con TODO), g.POST("/notifications/:id/ack", placeholder con TODO), g.POST("/notifications/:id/close", placeholder con TODO)

- [X] T014 [US1] Agregar wiring de notifications en `internal/routes/url_mappings.go`: importar `notificationsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"`, `notificationsApp "github.com/tu-org/embolsadora-api/internal/app/notifications"`, `notificationsHandler "github.com/tu-org/embolsadora-api/internal/api/handler/notifications"` + instanciar `nRepo := notificationsRepo.New(db)`, `nService := notificationsApp.New(nRepo, logger)` + llamar `notificationsHandler.RegisterRoutes(v1, nService)` después del bloque de alarm-rules (línea ~180)

**Checkpoint**: `GET /api/v1/notifications` y `GET /api/v1/notifications/count` funcionales. Pacts 1–3 pasan.

---

## Phase 4: User Story 2 — Ver detalle de una notificación (Priority: P2)

**Goal**: `GET /api/v1/notifications/:id` (200 y 404)

**Independent Test**: Ejecutar Pacts 4–5 del quickstart.md (GET por ID existente → 200 con todos los campos, ID inexistente o de otro tenant → 404)

- [X] T015 [US2] Implementar `service.Get` en `internal/app/notifications/service.go`: llama repo.GetByID, mapea ErrNotificationNotFound si repo retorna ese error, loguea con zap (notification_id, tenant_id), instrumenta NotificationOperationsTotal con label operation="get"

- [X] T016 [US2] Crear `internal/api/handler/notifications/get_notification.go`: handler `GetNotification` que extrae `:id` como uuid.UUID (retorna 400 si UUID inválido), extrae tenantID del contexto, llama `service.Get`, retorna `NotificationResponse` (200) o `{"error":"notificación no encontrada","code":"NOT_FOUND"}` (404) si ErrNotificationNotFound + actualizar `routes.go` reemplazando placeholder de GET /:id con GetNotification(service)

**Checkpoint**: `GET /api/v1/notifications/:id` funcional. Pacts 4–5 pasan.

---

## Phase 5: User Story 3 — Acusar recibo de una notificación (Priority: P3)

**Goal**: `POST /api/v1/notifications/:id/ack` (200 idempotente, 404)

**Independent Test**: Ejecutar Pact 6 del quickstart.md (POST ack en notificación unread → 200 con status=acknowledged + acknowledged_at seteado; segunda llamada → 200 idempotente con mismo acknowledged_at)

- [X] T017 [US3] Implementar `service.Ack` en `internal/app/notifications/service.go`: llama repo.Ack, mapea ErrNotificationNotFound, loguea (notification_id, tenant_id, resultado), instrumenta NotificationOperationsTotal con label operation="ack"

- [X] T018 [US3] Crear `internal/api/handler/notifications/ack_notification.go`: handler `AckNotification` que extrae `:id` como UUID, extrae tenantID, llama `service.Ack`, retorna `NotificationResponse` (200) o 404 si ErrNotificationNotFound + actualizar `routes.go` reemplazando placeholder de POST /:id/ack con AckNotification(service)

**Checkpoint**: `POST /api/v1/notifications/:id/ack` funcional e idempotente. Pact 6 pasa.

---

## Phase 6: User Story 4 — Cerrar una notificación (Priority: P4)

**Goal**: `POST /api/v1/notifications/:id/close` (200 idempotente, 404)

**Independent Test**: Ejecutar Pact 7 del quickstart.md (POST close → 200 con status=closed + closed_at seteado; segunda llamada → 200 idempotente; verificar que count de unread decrementó)

- [X] T019 [US4] Implementar `service.Close` en `internal/app/notifications/service.go`: llama repo.Close, mapea ErrNotificationNotFound, loguea (notification_id, tenant_id, resultado), instrumenta NotificationOperationsTotal con label operation="close"

- [X] T020 [US4] Crear `internal/api/handler/notifications/close_notification.go`: handler `CloseNotification` que extrae `:id` como UUID, extrae tenantID, llama `service.Close`, retorna `NotificationResponse` (200) o 404 si ErrNotificationNotFound + actualizar `routes.go` reemplazando placeholder de POST /:id/close con CloseNotification(service) — routes.go queda completo

**Checkpoint**: `POST /api/v1/notifications/:id/close` funcional e idempotente. Pact 7 pasa.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentación, integración final y validación completa

- [X] T021 [P] Agregar carpeta `Notifications` en `postman/Embolsadora-API-Complete.postman_collection.json` con los 8 requests del quickstart.md: GET /notifications (con y sin auth), GET /notifications/count, GET /notifications/:id (200 y 404), POST /notifications/:id/ack (primera y segunda llamada idempotente), POST /notifications/:id/close, GET /alarm-rules (verificación Pact 8). Variables de collection: baseUrl, jwt_token, tenant_id, notification_id.

- [X] T022 [P] Actualizar `docs/openapi.yaml`: agregar paths `/notifications`, `/notifications/count`, `/notifications/{id}`, `/notifications/{id}/ack`, `/notifications/{id}/close` con todos los query params, responses y schemas de `specs/010-notification-service/contracts/notification-service-api.openapi.yaml`

- [ ] T023 Ejecutar validación manual con `specs/010-notification-service/quickstart.md`: (pendiente — requiere servidor local con BD) verificar los 8 Pacts contra servidor local, confirmar que todos retornan los status codes esperados, verificar aislamiento multi-tenant

---

## Dependencies & Execution Order

### Dependencias entre fases

- **Phase 1 (Setup)**: Sin dependencias — puede iniciar de inmediato
- **Phase 2 (Foundational)**: Depende de Phase 1
  - T002–T004 (migración + domain): independientes entre sí [P]
  - T005 (repository): depende de T004 (domain)
  - T006–T007 (DTOs): dependen de T004 [P entre sí]
  - T008 (service skeleton): depende de T005
  - T009 (metrics): independiente [P]
- **Phase 3 (US1)**: Depende de Phase 2 completa
  - T010 (service.List+CountUnread): depende de T008
  - T011+T012 (handlers list+count): dependen de T007, [P entre sí]
  - T013 (routes.go): depende de T011+T012
  - T014 (wiring): depende de T013
- **Phase 4 (US2)**: Depende de Phase 3 (routes.go y wiring ya registrados)
- **Phase 5 (US3)**: Depende de Phase 3 (routes.go con placeholder de ack)
- **Phase 6 (US4)**: Depende de Phase 3 (routes.go con placeholder de close)
- **Phase 7 (Polish)**: Depende de todas las fases anteriores

### Dependencias entre User Stories

- **US1 (P1)**: Bloquea el resto — establece el router wiring y service base
- **US2, US3, US4**: Pueden implementarse en paralelo una vez US1 esté completo (cada una modifica un archivo distinto: get_notification.go, ack_notification.go, close_notification.go)

### Paralelos dentro de cada fase

- Phase 2: T002+T003+T004 [P], T006+T007 [P], T009 [P]
- Phase 3: T011+T012 [P]
- Phase 7: T021+T022 [P]

---

## Parallel Example: Phase 2

```text
Batch A (sin dependencias):
  T002 - Migración .up.sql
  T003 - Migración .down.sql
  T004 - domain/notifications.go
  T009 - telemetry/notification_metrics.go

Batch B (depende de T004):
  T005 - repo/pg/notifications/repository.go
  T006 - handler/notifications/dto/request.go
  T007 - handler/notifications/dto/response.go

Batch C (depende de T005):
  T008 - app/notifications/service.go skeleton
```

## Parallel Example: US2 + US3 + US4 (en paralelo tras US1)

```text
Developer A:
  T015 - service.Get
  T016 - get_notification.go

Developer B:
  T017 - service.Ack
  T018 - ack_notification.go

Developer C:
  T019 - service.Close
  T020 - close_notification.go
```

---

## Implementation Strategy

### MVP (solo US1 — Pacts 1–3)

1. Phase 1: Setup (T001)
2. Phase 2: Foundational (T002–T009)
3. Phase 3: US1 (T010–T014)
4. **STOP y VALIDAR**: Ejecutar Pacts 1–3 del quickstart.md
5. Si pasan → continuar con US2–US4

### Incremental por User Story

1. Setup + Foundational → base lista
2. US1 → GET /notifications + GET /count funcionales → validar Pacts 1–3
3. US2 → GET /notifications/:id → validar Pacts 4–5
4. US3 → POST /ack → validar Pact 6
5. US4 → POST /close → validar Pact 7
6. Polish → Postman + OpenAPI + validación completa (Pact 8 = verificación de 008)

---

## Notes

- Orden de rutas en `routes.go` es CRÍTICO: `/notifications/count` antes que `/notifications/:id` para evitar que Gin interprete "count" como un UUID
- Las operaciones Ack y Close son idempotentes por diseño SQL (CASE en acknowledged_at/closed_at)
- No hay endpoint de creación de notificaciones en este Pact — el seeding de prueba se hace directamente en la BD
- Pact 8 (GET /alarm-rules) no requiere implementación nueva — solo verificar en quickstart que responde 200
- Pattern de repo: seguir `internal/repo/pg/alarm_rules/repository.go`
- Pattern de service: seguir `internal/app/alarm_rules/service.go`
- Pattern de handler: seguir `internal/api/handler/alarm_rules/`
- Pattern de wiring: seguir el bloque de alarm-rules en `internal/routes/url_mappings.go` (~línea 174)
