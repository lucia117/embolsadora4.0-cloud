# Implementation Plan: Permissions Management API

**Branch**: `011-permissions-management` | **Date**: 2026-04-10 | **Spec**: [spec.md](./spec.md)  
**Input**: Feature specification from `/specs/011-permissions-management/spec.md`

## Summary

Implementar la API REST de gestión de permisos que expone un catálogo de permisos a través de 5 endpoints (`GET /permissions`, `POST /permissions`, `GET /permissions/:id`, `PUT /permissions/:id`, `DELETE /permissions/:id`). El sistema soporta dos tipos: permisos de sistema (17, inmutables, globales, cargados via migración) y permisos custom (creados por tenant admin, aislados por tenant, hard-delete). La implementación sigue la arquitectura hexagonal del proyecto: migración → domain → repo → service → handlers → wiring en url_mappings.go.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), Prometheus (métricas), testify + uber/mock (testing)  
**Storage**: PostgreSQL — tabla `permissions` con seed de 17 permisos de sistema via migración `000017`  
**Testing**: testify, uber/mock; Postman collection con 10 interacciones Pact  
**Target Platform**: Linux server (Docker), Supabase Auth (JWT RS256)  
**Project Type**: Web service — monolito modular, superficie ABM `/api/v1`  
**Performance Goals**: Listado de permisos < 500ms P95 (SC-001)  
**Constraints**: Aislamiento multi-tenant obligatorio (Constitución §II); logging estructurado obligatorio (Constitución §III); JWT + RBAC requerido (Constitución §II)  
**Scale/Scope**: ~17 permisos de sistema + permisos custom por tenant (volumen bajo); endpoint de alta frecuencia (consultado en cada apertura del panel de roles)

## Constitution Check

*GATE: Debe pasar antes de Phase 0. Re-verificado post-diseño Phase 1.*

| Principio | Estado | Evidencia |
|-----------|--------|-----------|
| **I. Arquitectura Hexagonal** | ✅ | Estructura: `domain/ → repo/ → app/ → handler/` — igual que roles, alarm_rules |
| **II. Aislamiento de tenants** | ✅ | Query: `WHERE (tenant_id = $1 OR is_system_permission = TRUE)` — mismo patrón que roles |
| **II. Auth JWT + RBAC** | ✅ | Middleware chain estándar: JWTAuth + TenantFromHeader + RBACCheck para mutaciones |
| **III. Observabilidad** | ✅ | Zap logging en service layer; Prometheus counters en handler layer |
| **IV. Testing por contrato** | ✅ | 10 interacciones Pact en quickstart.md; Postman collection en Phase 5 |
| **V. Versionado semántico** | ✅ | Nuevo endpoint, sin breaking changes — incremento MINOR |

**Resultado**: Sin violaciones. No requiere Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/011-permissions-management/
├── plan.md              ✅ Este archivo
├── spec.md              ✅ Especificación funcional
├── research.md          ✅ Decisiones técnicas resueltas
├── data-model.md        ✅ Schema SQL y entidad Go
├── quickstart.md        ✅ Comandos curl para verificar los 10 Pacts
├── contracts/
│   └── permissions-service-api.openapi.yaml  ✅ Contrato OpenAPI 3.0.3
├── checklists/
│   └── requirements.md  ✅ Checklist de calidad (todo en verde)
└── tasks.md             ⏳ Generado por /speckit.tasks
```

### Source Code (repository root)

```text
internal/
├── domain/
│   └── permissions.go                      — Permission struct, errores de dominio
├── app/
│   └── permissions/
│       └── service.go                      — Service: List, Get, Create, Update, Delete
├── repo/pg/
│   └── permissions/
│       └── repository.go                   — Repository interface + PostgresRepository
└── api/
    └── handler/
        └── permissions/
            └── handler.go                  — HTTP handlers + DTOs (ListPermissions, GetPermission,
                                              CreatePermission, UpdatePermission, DeletePermission)

migrations/
├── 000017_create_permissions_table.up.sql  — CREATE TABLE + seed 17 permisos de sistema
└── 000017_create_permissions_table.down.sql

routes/
└── url_mappings.go                         — Wiring: permissionsRepo → permissionsApp → permissionsHandler
```

**Structure Decision**: Un solo proyecto (monolito modular existente). Patrón handler único por feature (igual que `roles/`, `alarm_rules/`, `notifications/`).

## Phases

### Phase 1 — Database (migración + seed)

**Objetivo**: Crear la tabla `permissions` con los 17 permisos de sistema listos para uso.

**Tareas**:
- T01: Crear `000017_create_permissions_table.up.sql` (CREATE TABLE + índices + trigger updated_at + seed 17 permisos de sistema)
- T02: Crear `000017_create_permissions_table.down.sql` (DROP TABLE)
- T03: Aplicar migración manual: `migrate -path migrations/ -database $DATABASE_URL up 1`

**Criterio de aceptación**: `SELECT COUNT(*) FROM permissions WHERE is_system_permission = TRUE` retorna 17.

---

### Phase 2 — Domain

**Objetivo**: Definir la entidad `Permission` y los errores de dominio.

**Tareas**:
- T04: Crear `internal/domain/permissions.go` con struct `Permission` y errores `ErrPermissionNotFound`, `ErrPermissionIsSystem`, `ErrPermissionValidationFailed`

**Criterio de aceptación**: `go build ./internal/domain/...` sin errores.

---

### Phase 3 — Repository

**Objetivo**: Implementar acceso a BD para permisos siguiendo el patrón de `roles/repository.go`.

**Tareas**:
- T05: Crear `internal/repo/pg/permissions/repository.go` con:
  - Interface `Repository` (List, GetByID, Create, Update, Delete)
  - `PostgresRepository` con `pgxpool.Pool`
  - `List(ctx, tenantID)`: query `WHERE (tenant_id = $1 OR is_system_permission = TRUE)` ordenada por `is_system_permission DESC, name ASC`
  - `GetByID(ctx, id)`: busca por id ignorando tenant (permisos de sistema no tienen tenant_id)
  - `Create(ctx, p)`: INSERT con ID provisto (UUID generado en service)
  - `Update(ctx, p)`: UPDATE con verificación de `is_system_permission = FALSE`
  - `Delete(ctx, id)`: DELETE WHERE id = $1 AND is_system_permission = FALSE

**Criterio de aceptación**: Interface satisfecha por PostgresRepository; `go build ./internal/repo/...` sin errores.

---

### Phase 4 — Service (App Layer)

**Objetivo**: Implementar la lógica de negocio en la capa de aplicación.

**Tareas**:
- T06: Crear `internal/app/permissions/service.go` con `Service` y métodos:
  - `ListPermissions(ctx, tenantID)` → `[]*domain.Permission`
  - `GetPermission(ctx, id)` → `*domain.Permission`
  - `CreatePermission(ctx, tenantID, name, section, description)` → `*domain.Permission` (genera UUID, valida nombre ≥ 3 chars, llama repo.Create)
  - `UpdatePermission(ctx, id, name, section, description)` → `*domain.Permission` (GetByID primero, verifica !IsSystemPermission, llama repo.Update)
  - `DeletePermission(ctx, id)` → error (GetByID primero, verifica !IsSystemPermission, llama repo.Delete)

**Criterio de aceptación**: `go build ./internal/app/...` sin errores; lógica de guards (`ErrPermissionIsSystem`) correcta.

---

### Phase 5 — HTTP Handlers

**Objetivo**: Implementar los 5 handlers HTTP con DTOs, mapeo de errores y logging.

**Tareas**:
- T07: Crear `internal/api/handler/permissions/handler.go` con:
  - `Handler` struct con `service` y `logger`
  - `NewHandler(service, logger)` constructor
  - `ListPermissions(c *gin.Context)` → GET `/permissions`
  - `GetPermission(c *gin.Context)` → GET `/permissions/:id`
  - `CreatePermission(c *gin.Context)` → POST `/permissions`
  - `UpdatePermission(c *gin.Context)` → PUT `/permissions/:id`
  - `DeletePermission(c *gin.Context)` → DELETE `/permissions/:id`
  - DTOs: `PermissionResponse`, `CreatePermissionRequest`, `UpdatePermissionRequest`, `ValidationErrorResponse`
  - Mapeo de errores: `ErrPermissionNotFound` → 404, `ErrPermissionIsSystem` → 403, `ErrPermissionValidationFailed` → 400

**Criterio de aceptación**: `go build ./internal/api/...` sin errores; respuestas JSON coinciden con el contrato OpenAPI.

---

### Phase 6 — Wiring (url_mappings.go)

**Objetivo**: Registrar el handler en el router con el middleware apropiado.

**Tareas**:
- T08: Actualizar `internal/routes/url_mappings.go`:
  - Agregar imports: `permissionsHandler`, `permissionsApp`, `permissionsRepo`
  - Instanciar `permissionsRepo.NewPostgresRepository(db)`
  - Instanciar `permissionsApp.NewService(permissionsRepo, logger)`
  - Instanciar `permissionsHandler.NewHandler(permissionsApp, logger)`
  - Registrar rutas en el grupo `/api/v1` protegido:
    - `GET /permissions` — solo JWTAuth + TenantFromHeader
    - `GET /permissions/:id` — solo JWTAuth + TenantFromHeader
    - `POST /permissions` + RBACCheck("admin")
    - `PUT /permissions/:id` + RBACCheck("admin")
    - `DELETE /permissions/:id` + RBACCheck("admin")

**Criterio de aceptación**: `go build ./...` sin errores; `curl GET /api/v1/permissions` retorna 200.

---

### Phase 7 — Testing (Postman + Verificación Pact)

**Objetivo**: Crear la Postman collection y verificar las 10 interacciones Pact.

**Tareas**:
- T09: Crear `postman/Permissions-API.postman_collection.json` con los 10 Pact scenarios del quickstart.md
- T10: Ejecutar manualmente los curl del quickstart.md y verificar los 10 resultados esperados
- T11: Actualizar `PACTS_ANALYSIS.md` marcando `permissions-service-api` como ✅ completado

**Criterio de aceptación**: Las 10 interacciones Pact pasan. `PACTS_ANALYSIS.md` actualizado.

---

### Phase 8 — Observabilidad

**Objetivo**: Instrumentar con Prometheus y Zap (requerido por Constitución §III).

**Tareas**:
- T12: Agregar contadores Prometheus en handlers:
  - `permissions_requests_total{method, endpoint, status}`
  - `permissions_list_duration_seconds` (histograma)
- T13: Verificar que todos los métodos del service loguean con Zap (info en operaciones exitosas, error en fallos)

**Criterio de aceptación**: `/metrics` expone los contadores después de llamar los endpoints.

## Dependency Order

```
T01 (migración) → T02 (down)
T03 (aplicar migración) [manual]
T04 (domain) → T05 (repo) → T06 (service) → T07 (handlers) → T08 (wiring)
T09 (Postman) ← T08 (wiring funcional)
T10 (verificación) ← T08 + T03
T11 (PACTS_ANALYSIS) ← T10
T12, T13 (observabilidad) ← T07
```

## Risks & Mitigations

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|-----------|
| IDs de permisos de sistema no coinciden con Pact | Baja | Seed de migración copia exactamente los IDs del contrato Pact |
| Query multi-tenant devuelve permisos de otro tenant | Media | Test de aislamiento en quickstart.md (paso 6); query con `tenant_id = $1 OR is_system_permission = TRUE` |
| Número de migración colisiona con otra feature | Baja | `000017` verificado como siguiente disponible (último: `000016`) |
| Permisos de sistema modificables via SQL directo | N/A | Fuera de scope MVP; la API los protege con 403 |
