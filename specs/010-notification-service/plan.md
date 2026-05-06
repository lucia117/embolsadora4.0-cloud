# Implementation Plan: Notification Service API

**Branch**: `010-notification-service` | **Date**: 2026-04-10 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/010-notification-service/spec.md`

## Summary

API de consulta y gestión de estado de notificaciones para la plataforma embolsadora. Implementa los 5 contratos Pact de `notification-service-api` (GET list, GET count, GET by ID, POST ack, POST close) más verificación del 6° Pact (GET /alarm-rules, ya implementado en 008). Sigue el patrón hexagonal establecido: migración SQL → domain types → repo pg → app service → handler + dto → registro en url_mappings.go.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), Prometheus (metrics), google/uuid  
**Storage**: PostgreSQL — tabla `notifications` (migración 000016)  
**Testing**: testify + uber/mock; Postman collection para Pact interactions  
**Target Platform**: Linux server (Docker / Cloud Run)  
**Project Type**: Web service — Superficie ABM (`/api/v1/`)  
**Performance Goals**: P95 < 300ms por operación; GET /count < 100ms  
**Constraints**: Aislamiento multi-tenant obligatorio; JWT Bearer + X-Tenant-ID header; ack/close idempotentes  
**Scale/Scope**: Hasta ~10.000 notificaciones por tenant; paginación limit/offset

## Constitution Check

| Gate | Estado | Notas |
|---|---|---|
| Arquitectura hexagonal | ✅ | transport → app → domain ← repo |
| Aislamiento multi-tenant | ✅ | tenant_id en todas las queries; X-Tenant-ID header |
| Autenticación JWT | ✅ | Superficie ABM; todos los endpoints requieren JWT |
| RBAC | ✅ | Sin RBAC adicional para ack/close — acceso a cualquier usuario del tenant (operadores y admins) |
| Logging estructurado (Zap) | ✅ | Incluido en service layer |
| Métricas Prometheus | ✅ | Contador de operaciones + histograma de latencia |
| OpenAPI actualizado | ✅ | contracts/notification-service-api.openapi.yaml generado |
| Sin lógica entre superficies | ✅ | Solo Superficie ABM; sin tocar `/consumers/` |

## Project Structure

### Documentation (this feature)

```text
specs/010-notification-service/
├── plan.md              ← este archivo
├── research.md          ← decisiones técnicas
├── data-model.md        ← modelo Notification + state machine
├── quickstart.md        ← curls de validación (8 Pacts)
├── contracts/
│   └── notification-service-api.openapi.yaml
├── checklists/
│   └── requirements.md
└── tasks.md             ← generado por /speckit.tasks
```

### Source Code

```text
migrations/
├── 000016_create_notifications_table.up.sql
└── 000016_create_notifications_table.down.sql

internal/
├── domain/
│   └── notifications.go              ← Notification struct, status/severity types, ErrNotificationNotFound
├── repo/pg/
│   └── notifications/
│       └── repository.go             ← Repository interface + PostgresRepository
├── app/
│   └── notifications/
│       └── service.go                ← Service con List, Count, Get, Ack, Close
└── api/
    └── handler/
        └── notifications/
            ├── routes.go
            ├── list_notifications.go
            ├── count_notifications.go
            ├── get_notification.go
            ├── ack_notification.go
            ├── close_notification.go
            └── dto/
                ├── request.go
                └── response.go

internal/routes/url_mappings.go       ← agregar wiring de notifications
postman/
└── Embolsadora-API-Complete.postman_collection.json  ← agregar carpeta Notifications
```

## Implementation Phases

### Fase 1 — Base de datos (Migración 000016)

Crear tabla `notifications`:

```sql
CREATE TABLE notifications (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      UUID NOT NULL,
  title          TEXT NOT NULL,
  message        TEXT NOT NULL,
  severity       VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical', 'error')),
  status         VARCHAR(20) NOT NULL DEFAULT 'unread' CHECK (status IN ('unread', 'acknowledged', 'closed')),
  alarm_rule_id  UUID,
  machine_id     UUID,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  acknowledged_at TIMESTAMPTZ,
  closed_at      TIMESTAMPTZ
);
```

Índices:
- `idx_notifications_tenant_list ON notifications(tenant_id, created_at DESC)`
- `idx_notifications_tenant_status ON notifications(tenant_id, status, created_at DESC)`
- `idx_notifications_tenant_severity ON notifications(tenant_id, severity, created_at DESC)`

No hay FK constraints sobre `alarm_rule_id` ni `machine_id` (referencias históricas).  
No hay soft-delete ni `updated_at`.

### Fase 2 — Domain

`internal/domain/notifications.go`:
- Type `NotificationStatus string` con constantes: `StatusUnread`, `StatusAcknowledged`, `StatusClosed`
- Type `NotificationSeverity string` con constantes: `SeverityInfo`, `SeverityWarning`, `SeverityCritical`, `SeverityError`
- Struct `Notification` con todos los campos (ver data-model.md)
- Error: `ErrNotificationNotFound = errors.New("notificación no encontrada")`

### Fase 3 — Repositorio

`internal/repo/pg/notifications/repository.go`:
- Interface `Repository`:
  - `List(ctx, tenantID uuid.UUID, params ListParams) ([]*domain.Notification, int, error)` — retorna lista + total
  - `CountUnread(ctx, tenantID uuid.UUID) (int, error)`
  - `GetByID(ctx, id, tenantID uuid.UUID) (*domain.Notification, error)`
  - `Ack(ctx, id, tenantID uuid.UUID) (*domain.Notification, error)` — idempotente
  - `Close(ctx, id, tenantID uuid.UUID) (*domain.Notification, error)` — idempotente
- Struct `ListParams`: `Status *string, Severity *string, Limit int, Offset int`
- `PostgresRepository` implementando la interface con `*pgxpool.Pool`
- Constructor `New(pool *pgxpool.Pool) *PostgresRepository`
- Todas las queries incluyen `tenant_id` en WHERE
- `GetByID` retorna `ErrNotificationNotFound` si no existe o es de otro tenant (pgx.ErrNoRows → ErrNotificationNotFound)
- `Ack`: UPDATE idempotente — si ya está acknowledged/closed, retorna sin error (ver data-model.md para SQL)
- `Close`: UPDATE idempotente — si ya está closed, retorna sin error

### Fase 4 — Servicio de aplicación

`internal/app/notifications/service.go`:
- Struct `Service` con `repo Repository` + `logger *zap.Logger`
- Constructor `New(repo Repository, logger *zap.Logger) *Service`
- Métodos:
  - `List(ctx, tenantID uuid.UUID, params repo.ListParams) ([]*domain.Notification, int, error)`
  - `CountUnread(ctx, tenantID uuid.UUID) (int, error)`
  - `Get(ctx, id, tenantID uuid.UUID) (*domain.Notification, error)`
  - `Ack(ctx, id, tenantID uuid.UUID) (*domain.Notification, error)`
  - `Close(ctx, id, tenantID uuid.UUID) (*domain.Notification, error)`
- Logging estructurado (zap) en cada operación con `tenant_id`, `notification_id`
- Métricas: instrumentar latencia y conteo de operaciones via `internal/telemetry/`

### Fase 5 — Handlers HTTP

`internal/api/handler/notifications/dto/request.go`:
- Struct `ListNotificationsParams` con query tags: `Status string json:"status"`, `Severity string json:"severity"`, `Limit int json:"limit" default:"20"`, `Offset int json:"offset" default:"0"`

`internal/api/handler/notifications/dto/response.go`:
- Struct `NotificationResponse` (todos los campos de Notification con json tags, nullable como *string o *time.Time)
- Función `FromDomain(n *domain.Notification) NotificationResponse`
- Struct `NotificationListResponse`: `Data []NotificationResponse json:"data"`, `Total int json:"total"`, `Limit int json:"limit"`, `Offset int json:"offset"`
- Struct `NotificationCountResponse`: `Unread int json:"unread"`

Handlers (un archivo por handler):
- `list_notifications.go`: parsea query params, llama `service.List`, retorna `NotificationListResponse`
- `count_notifications.go`: llama `service.CountUnread`, retorna `NotificationCountResponse`
- `get_notification.go`: extrae `:id` como UUID, llama `service.Get`, retorna `NotificationResponse` o 404 si `ErrNotificationNotFound`
- `ack_notification.go`: extrae `:id`, llama `service.Ack`, retorna `NotificationResponse` o 404
- `close_notification.go`: extrae `:id`, llama `service.Close`, retorna `NotificationResponse` o 404

Mapeo de errores:
- `ErrNotificationNotFound` → 404 `{ "error": "...", "code": "NOT_FOUND" }`
- Error interno → 500

`routes.go`:
```go
func RegisterRoutes(g *gin.RouterGroup, service *notifications.Service) {
    // CRÍTICO: /notifications/count antes de /notifications/:id
    g.GET("/notifications/count", CountNotifications(service))
    g.GET("/notifications", ListNotifications(service))
    g.GET("/notifications/:id", GetNotification(service))
    g.POST("/notifications/:id/ack", AckNotification(service))
    g.POST("/notifications/:id/close", CloseNotification(service))
}
```

### Fase 6 — Registro en url_mappings.go

En `internal/routes/url_mappings.go`:
1. Importar `notificationsRepo`, `notificationsApp`, `notificationsHandler`
2. Instanciar: `nRepo := notificationsRepo.New(db)`, `nService := notificationsApp.New(nRepo, logger)`
3. Registrar: `notificationsHandler.RegisterRoutes(v1, nService)` — después de alarm-rules

### Fase 7 — Telemetría

En `internal/telemetry/` (o en service directamente siguiendo el patrón de alarm_rules):
- Counter `notification_operations_total` con labels `operation` (list/count/get/ack/close) y `tenant`
- Histogram `notification_operation_duration_seconds`

### Fase 8 — Postman Collection

Agregar carpeta `Notifications` en `Embolsadora-API-Complete.postman_collection.json` con los 8 requests del quickstart.md.

## Complexity Tracking

Sin violaciones a la Constitución. No se requiere tabla de justificación.
