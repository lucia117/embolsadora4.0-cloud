# Implementation Plan: Log Service API

**Branch**: `009-log-service` | **Date**: 2026-04-07 | **Spec**: [spec.md](spec.md)

## Summary

API de logs de eventos operacionales para la plataforma Embolsadora 4.0.
Implementa lectura de logs con filtros combinables y paginación keyset, detalle por ID,
ventana de contexto temporal, streaming SSE en tiempo real, exportación con truncado,
y gestión de política de retención por tenant. Cubre 14 contratos Pact del frontend.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), Prometheus (métricas)  
**Storage**: PostgreSQL — tablas `log_entries` + `log_retention_policies` (migración 000004)  
**Testing**: testify, uber/mock, Postman collection con 14 interacciones Pact  
**Target Platform**: Linux server (Docker)  
**Project Type**: web-service (REST + SSE)  
**Performance Goals**: P95 < 500ms para listados hasta 1M logs/tenant; export 10k logs < 10s  
**Constraints**: Aislamiento multi-tenant obligatorio; logs inmutables; SSE heartbeat cada 30s  
**Scale/Scope**: ~14 endpoints, 2 tablas nuevas, ~500 LOC estimados

## Constitution Check

| Gate | Estado |
|------|--------|
| Arquitectura hexagonal (`transport → app → domain ← repo`) | ✅ |
| Aislamiento de tenants (`tenant_id` en todas las queries) | ✅ |
| JWT + RBAC en superficie ABM | ✅ (`logs:admin` para PATCH /retention) |
| Observabilidad (Zap + Prometheus en `telemetry/log_metrics.go`) | ✅ requerido |
| Tests de contrato (14 Pact interactions) | ✅ requerido |
| OpenAPI actualizado (`docs/openapi.yaml`) | ✅ requerido |

Sin violaciones. Sin ADR requerido (no hay nueva superficie ni cambio de auth).

## Project Structure

### Documentación (esta feature)

```text
specs/009-log-service/
├── plan.md              ← este archivo
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── log-service-api.openapi.yaml
├── checklists/
│   └── requirements.md
└── tasks.md             ← generado por /speckit.tasks
```

### Código fuente

```text
internal/
  domain/
    logs.go                           # LogEntry, RetentionPolicy, enums severity/event_type, errores
  platform/
    logwriter/
      writer.go                       # Interface LogWriter (write path interno — future use)
  app/
    logs/
      service.go                      # List, Get, GetContext, Export, GetRetention, UpdateRetention, Subscribe
  repo/pg/
    logs/
      repository.go                   # Interface Repository + PostgreSQL impl (cursor pagination)
  telemetry/
    log_metrics.go                    # Prometheus: request_count, list_latency, export_count
  api/
    handler/
      logs/
        dto/
          request.go                  # ListLogsParams, ExportLogsParams, UpdateRetentionRequest
          response.go                 # LogResponse, LogListResponse, ExportResponse, ContextResponse, RetentionResponse
        list_logs.go                  # GET /logs
        get_log.go                    # GET /logs/:id
        get_log_context.go            # GET /logs/:id/context
        stream_logs.go                # GET /logs/stream (SSE)
        export_logs.go                # GET /logs/export
        get_retention.go              # GET /logs/retention
        update_retention.go           # PATCH /logs/retention
        routes.go                     # RegisterRoutes — orden crítico: estáticas antes de :id

migrations/
  000004_create_log_entries_table.up.sql
  000004_create_log_entries_table.down.sql

postman/
  Log-Service-API.postman_collection.json   # 14 Pact interactions
```

## Fases de Implementación

### Fase 1 — Base de datos (P1)
- Migración `000004`: tablas `log_entries` + `log_retention_policies` + índices

### Fase 2 — Dominio (P1)
- `internal/domain/logs.go`: entidades, enums, errores
- `internal/platform/logwriter/writer.go`: interface LogWriter

### Fase 3 — Repositorio (P1)
- `internal/repo/pg/logs/repository.go`: List (cursor), Get, GetContext, Export, GetRetention, UpsertRetention
- Paginación keyset: `WHERE (created_at, id) < ($cursor_ts, $cursor_id)`

### Fase 4 — Servicio de aplicación (P1)
- `internal/app/logs/service.go`: orquestación, validación de parámetros, cursor encoding/decoding
- Subscribe/Unsubscribe para SSE (pub/sub en memoria por tenant)

### Fase 5 — Handlers HTTP (P1–P2)
- DTOs: `request.go`, `response.go`
- 7 handlers: list, get, context, stream, export, get_retention, update_retention
- `routes.go`: orden correcto (estáticas antes de `:id`)

### Fase 6 — Integración en router (P1)
- Wiring en `internal/routes/url_mappings.go`
- Middleware: `JWTAuth → TenantFromHeader → [RBACCheck para PATCH /retention]`

### Fase 7 — Observabilidad (P2)
- `internal/telemetry/log_metrics.go`: contadores y latencia
- Instrumentación en handlers

### Fase 8 — Postman + OpenAPI (P2)
- `postman/Log-Service-API.postman_collection.json`: 14 requests Pact
- Actualizar `docs/openapi.yaml` con endpoints de logs

## Decisiones Técnicas Clave

| Decisión | Elección | Referencia |
|----------|----------|-----------|
| Paginación | Cursor keyset `(created_at, id)` en base64 | research.md §1 |
| SSE | Gin `c.Stream()` nativo, sin librerías extra | research.md §2 |
| Write path | Interface `LogWriter`, no endpoint HTTP | research.md §3 |
| Export format | JSON default, CSV opcional | research.md §4 |
| Migración # | `000004` (renumerar si 008 merge primero) | research.md §5 |
| Orden de rutas | Estáticas (retention/stream/export) antes de `:id` | research.md §6 |
| Context query | UNION de N previos + N posteriores | research.md §7 |
