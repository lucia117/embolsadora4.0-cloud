# Implementation Plan: Alarm Rules Service API

**Branch**: `008-alarm-rules` | **Date**: 2026-04-06 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/008-alarm-rules/spec.md`

## Summary

CRUD completo de reglas de alarma para monitoreo industrial multi-tenant. Implementa los 10 contratos Pact de `alarm-rules-service-api` (GET list, GET by ID, GET 404, POST 201, POST 400, PATCH 200, PATCH 404, DELETE 200, DELETE 404, GET 401). Sigue el patrón hexagonal establecido por features anteriores (006-roles, 007-user-roles-status): migración SQL → domain types → repo pg → app service → handler + dto → registro en router.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), Prometheus (metrics), uuid (google/uuid)  
**Storage**: PostgreSQL — tabla `alarm_rules` (migración 000014)  
**Testing**: testify + uber/mock; Postman collection para Pact interactions  
**Target Platform**: Linux server (Docker / Cloud Run)  
**Project Type**: Web service — Superficie ABM (`/api/v1/`)  
**Performance Goals**: P95 < 300ms por operación CRUD  
**Constraints**: Aislamiento multi-tenant obligatorio; JWT Bearer + X-Tenant-ID header; RBAC write operations  
**Scale/Scope**: Pocas reglas por tenant (< 100); sin paginación en MVP

## Constitution Check

| Gate | Estado | Notas |
|---|---|---|
| Arquitectura hexagonal | ✅ | transport → app → domain ← repo |
| Aislamiento multi-tenant | ✅ | tenant_id en todas las queries; X-Tenant-ID header |
| Autenticación JWT + RBAC | ✅ | Superficie ABM; writes requieren `alarm-rules:write` o `users:write` |
| Logging estructurado (Zap) | ✅ | Incluido en service layer |
| Métricas Prometheus | ✅ | Contador de operaciones + histograma de latencia |
| OpenAPI actualizado | ✅ | contracts/alarm-rules-service-api.openapi.yaml generado |
| Sin lógica entre superficies | ✅ | Solo Superficie ABM; sin tocar `/consumers/` |

## Project Structure

### Documentation (this feature)

```text
specs/008-alarm-rules/
├── plan.md              ← este archivo
├── research.md          ← decisiones técnicas
├── data-model.md        ← modelo AlarmRule
├── quickstart.md        ← curls de validación
├── contracts/
│   └── alarm-rules-service-api.openapi.yaml
└── tasks.md             ← generado por /speckit.tasks
```

### Source Code

```text
migrations/
├── 000014_create_alarm_rules_table.up.sql
└── 000014_create_alarm_rules_table.down.sql

internal/
├── domain/
│   └── alarm_rules.go              ← AlarmRule struct, errores de dominio
├── repo/pg/
│   └── alarm_rules/
│       └── repository.go           ← Repository interface + PostgresRepository
├── app/
│   └── alarm_rules/
│       └── service.go              ← Service con lógica de negocio
└── api/
    └── handler/
        └── alarm_rules/
            ├── routes.go
            ├── list_alarm_rules.go
            ├── create_alarm_rule.go
            ├── get_alarm_rule.go
            ├── update_alarm_rule.go
            ├── delete_alarm_rule.go
            └── dto/
                ├── request.go
                └── response.go

internal/api/router.go              ← registro de rutas (modificar)
postman/
└── Embolsadora-API-Complete.postman_collection.json  ← agregar carpeta Alarm Rules
```

## Implementation Phases

### Fase 1 — Base de datos (Migración 000014)

Crear tabla `alarm_rules` con:
- `id` UUID PK
- `tenant_id` UUID FK NOT NULL (aislamiento multi-tenant)
- `name` TEXT NOT NULL
- `description` TEXT NULL
- `metric` TEXT NOT NULL — nombre de la métrica del edge device
- `operator` TEXT NOT NULL — `gt`, `lt`, `gte`, `lte`, `eq`
- `threshold` NUMERIC NOT NULL — valor umbral
- `severity` TEXT NOT NULL — `info`, `warning`, `critical`
- `enabled` BOOLEAN NOT NULL DEFAULT TRUE
- `created_at` TIMESTAMPTZ NOT NULL DEFAULT NOW()
- `updated_at` TIMESTAMPTZ NOT NULL DEFAULT NOW()

Índices: `(tenant_id)`, `(tenant_id, enabled)`.  
Trigger `updated_at` (mismo patrón que otras tablas).  
Sin soft-delete en MVP (eliminación permanente).

### Fase 2 — Domain

`internal/domain/alarm_rules.go`:
- Struct `AlarmRule` con todos los campos
- Errores: `ErrAlarmRuleNotFound`
- Constantes: `ValidOperators`, `ValidSeverities`
- Función `ValidateOperator(op string) bool`
- Función `ValidateSeverity(s string) bool`

### Fase 3 — Repositorio

`internal/repo/pg/alarm_rules/repository.go`:
- Interface `Repository`: `List`, `GetByID`, `Create`, `Update`, `Delete`
- `PostgresRepository` que implementa la interface
- Todas las queries incluyen `tenant_id` para aislamiento
- `GetByID` verifica que el `tenant_id` coincida → devuelve `ErrAlarmRuleNotFound` si no

### Fase 4 — Servicio de aplicación

`internal/app/alarm_rules/service.go`:
- `Service` con `repo Repository` + `logger *zap.Logger`
- Métodos: `ListAlarmRules`, `GetAlarmRule`, `CreateAlarmRule`, `UpdateAlarmRule`, `DeleteAlarmRule`
- Validación de `operator` y `severity` en Create/Update
- Logging estructurado en cada operación

### Fase 5 — Handlers HTTP

`internal/api/handler/alarm_rules/`:
- `dto/request.go`: `CreateAlarmRuleRequest`, `UpdateAlarmRuleRequest` con json tags y validación
- `dto/response.go`: `AlarmRuleResponse`, función `FromDomain`
- Un archivo por handler: list, get, create, update, delete
- `routes.go`: `RegisterRoutes(readGroup, writeGroup *gin.RouterGroup, service *appAlarmRules.Service)`

Mapeo de errores de dominio → HTTP:
- `ErrAlarmRuleNotFound` → 404
- Validación → 400
- Error interno → 500

### Fase 6 — Registro en router

`internal/api/router.go`: inyectar `alarm_rules.NewPostgresRepository` + `alarm_rules.NewService` y llamar `RegisterRoutes`.

### Fase 7 — Postman

Agregar carpeta `Alarm Rules` en `Embolsadora-API-Complete.postman_collection.json` con los 10 requests Pact.

## Complexity Tracking

Sin violaciones a la Constitución. No se requiere tabla de justificación.
