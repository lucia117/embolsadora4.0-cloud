# Plan de Implementación: API de Gestión de Roles

**Rama**: `006-roles-management` | **Fecha**: 2026-04-03 | **Spec**: [spec.md](./spec.md)

## Resumen

Implementar endpoints REST para gestionar roles por tenant. La tabla `roles` ya existe con 4 roles del sistema pre-cargados. Esta feature extiende ese esquema y agrega los 5 endpoints CRUD que el frontend espera según el contrato Pact (`role-service-api`, 7 interacciones). El RBAC estático en `security/rbac.go` se mantiene tal cual; los nuevos endpoints son para gestión administrativa, no para autorización en tiempo de request.

## Contexto Técnico

**Lenguaje/Versión**: Go 1.24+
**Dependencias principales**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging)
**Almacenamiento**: PostgreSQL — tabla `roles` existente (migration 000003), extendida con migration 000012
**Testing**: testify + uber/mock
**Plataforma destino**: Linux server (Docker / Cloud Run)
**Tipo de proyecto**: Web service — monolito modular hexagonal
**Objetivos de performance**: respuestas < 300ms p95 (sin caché, consultas simples)
**Restricciones**: aislamiento multi-tenant obligatorio vía `X-Tenant-ID` header; autenticación JWT en todos los endpoints

## Verificación de Constitución

| Compuerta | Estado | Observación |
|-----------|--------|-------------|
| I. Arquitectura hexagonal | ✅ | Sigue capas `transport → app → domain ← repo` |
| II. Aislamiento de tenants | ✅ | Todas las queries filtran por `tenant_id`; tenant desde contexto |
| II. JWT + RBAC | ✅ | Endpoints bajo middleware `JWTAuth + TenantFromHeader` existente |
| III. Observabilidad (Zap) | ✅ | Logger estructurado en service y handlers |
| III. Métricas Prometheus | ⚠️ | Diferido — consistente con dashboard layouts y otras features |
| IV. Tests de integración | ⚠️ | Diferido — Postman collection como validación de contrato |
| V. OpenAPI actualizado | ✅ | Contrato generado en `contracts/role-service-api.openapi.yaml` |

## Estructura del Proyecto

### Documentación (esta feature)

```text
specs/006-roles-management/
├── plan.md              ← este archivo
├── research.md          ← decisiones técnicas y alternativas
├── data-model.md        ← esquema de BD y entidades
├── quickstart.md        ← guía de prueba manual con curl
├── contracts/
│   └── role-service-api.openapi.yaml
└── checklists/
    └── requirements.md
```

### Código fuente

```text
migrations/
├── 000012_extend_roles_table.up.sql    ← columnas nuevas + datos del sistema
└── 000012_extend_roles_table.down.sql

internal/
├── domain/
│   └── roles.go                        ← struct Role + errores de dominio
├── repo/pg/roles/
│   └── repository.go                   ← implementación PostgreSQL
├── app/roles/
│   └── service.go                      ← lógica de negocio
└── api/handler/roles/
    ├── dto/
    │   ├── request.go
    │   └── response.go
    ├── list_roles.go
    ├── get_role.go
    ├── create_role.go
    ├── update_role.go
    ├── delete_role.go
    └── routes.go

internal/routes/url_mappings.go         ← registro de rutas (modificar)
```

---

## Decisiones Técnicas Clave

| Decisión | Elección | Alternativa descartada | Motivo |
|----------|----------|------------------------|--------|
| ID de roles custom | `custom_<6 hex chars>` (string) | UUID | La tabla `roles` tiene PK VARCHAR(50); consistente con IDs de sistema ("admin") |
| Almacenamiento de permisos | JSONB `[]` en columna `permissions` | Tabla separada `role_permissions` | Los permisos se leen/escriben siempre como conjunto; sin consultas cross-permiso |
| Unicidad de nombre | Índice parcial `(tenant_id, name) WHERE deleted_at IS NULL` | Constraint UNIQUE | Permite soft-delete y reusar nombres |
| Scoping de lista | Roles del tenant (tenant_id = X) + roles globales (is_global = TRUE) | Solo roles del tenant | El frontend espera ver roles del sistema en la lista |
| Tenant en respuesta | UUID del tenant (string) | Subdomain slug | Consistente con todos los demás endpoints del proyecto |

---

## Fases de Implementación

### Fase 1 — Migración de Base de Datos

**Archivo**: `migrations/000012_extend_roles_table.up.sql`

```sql
ALTER TABLE roles
  ADD COLUMN IF NOT EXISTS is_system_role BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS is_global      BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS tenant_id      UUID REFERENCES tenants(id) ON DELETE CASCADE,
  ADD COLUMN IF NOT EXISTS permissions    JSONB NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS deleted_at     TIMESTAMPTZ;

-- Marcar roles del sistema
UPDATE roles
  SET is_system_role = TRUE, is_global = TRUE
  WHERE id IN ('admin', 'operario', 'cliente_admin', 'cliente_operario');

-- Índice para queries por tenant
CREATE INDEX IF NOT EXISTS idx_roles_tenant_active
  ON roles (tenant_id)
  WHERE deleted_at IS NULL;

-- Nombre único por tenant (custom roles)
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_tenant_name_active
  ON roles (tenant_id, name)
  WHERE deleted_at IS NULL AND is_system_role = FALSE;
```

### Fase 2 — Dominio

**Archivo**: `internal/domain/roles.go`

- Struct `Role` con todos los campos
- Errores: `ErrRoleNotFound`, `ErrRoleIsSystemRole`, `ErrRoleHasAssignments`, `ErrRoleDuplicateName`, `ErrRoleLimitReached`
- Constante `MaxCustomRolesPerTenant = 3`

### Fase 3 — Repositorio PostgreSQL

**Archivo**: `internal/repo/pg/roles/repository.go`

Interface embebida en el mismo paquete (patrón igual a `dashboard_layouts`):

```go
type Repository interface {
    List(ctx, tenantID uuid.UUID) ([]*domain.Role, error)
    GetByID(ctx, id string) (*domain.Role, error)
    CountCustomByTenant(ctx, tenantID uuid.UUID) (int, error)
    Create(ctx, role *domain.Role) error
    Update(ctx, role *domain.Role) error
    SoftDelete(ctx, id string) error
    CountActiveAssignments(ctx, roleID string) (int, error)
}
```

Queries clave:
- **List**: `WHERE (tenant_id = $1 OR is_global = TRUE) AND deleted_at IS NULL ORDER BY is_system_role DESC, name ASC`
- **CountCustomByTenant**: `WHERE tenant_id = $1 AND is_system_role = FALSE AND deleted_at IS NULL`
- **CountActiveAssignments**: `SELECT COUNT(*) FROM user_tenant_roles WHERE role_id = $1 AND status = 'active'`

### Fase 4 — Servicio de Aplicación

**Archivo**: `internal/app/roles/service.go`

Lógica de negocio en `DeleteRole`:
1. `GetByID` → si `IsSystemRole` → `ErrRoleIsSystemRole`
2. `CountActiveAssignments` → si > 0 → `ErrRoleHasAssignments`
3. `SoftDelete`

Lógica de negocio en `CreateRole`:
1. `CountCustomByTenant` → si >= 3 → `ErrRoleLimitReached`
2. Generar ID: `"custom_" + hex(rand 3 bytes)`
3. Deduplicar permisos
4. `Create`

Lógica en `UpdateRole`:
1. `GetByID` → si `IsSystemRole` → `ErrRoleIsSystemRole`
2. Deduplicar permisos
3. `Update`

### Fase 5 — Handlers HTTP

**Directorio**: `internal/api/handler/roles/`

DTOs:
```go
// Request
type CreateRoleRequest struct {
    Name        string   `json:"name" binding:"required,max=100"`
    Description string   `json:"description" binding:"max=500"`
    Permissions []string `json:"permissions"`
}
type UpdateRoleRequest struct {
    Name        string   `json:"name" binding:"required,max=100"`
    Description string   `json:"description" binding:"max=500"`
    Permissions []string `json:"permissions"`
}

// Response
type RoleResponse struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    Description  string    `json:"description"`
    Permissions  []string  `json:"permissions"`
    IsSystemRole bool      `json:"isSystemRole"`
    IsGlobal     bool      `json:"isGlobal"`
    TenantID     *string   `json:"tenantId"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

Envelope: `{"success": true/false, "data": ...}` — consistente con Pact y demás endpoints.

Mapeo de errores de dominio → HTTP:
| Error de dominio | HTTP |
|-----------------|------|
| `ErrRoleNotFound` | 404 |
| `ErrRoleIsSystemRole` | 403 + `SYSTEM_ROLE` |
| `ErrRoleHasAssignments` | 409 + `ROLE_HAS_ASSIGNMENTS` + `usersAffected` |
| `ErrRoleDuplicateName` | 409 + `DUPLICATE_NAME` |
| `ErrRoleLimitReached` | 403 + `LIMIT_REACHED` |

### Fase 6 — Registro de Rutas

**Archivo**: `internal/routes/url_mappings.go`

Agregar al grupo `v1` (ya tiene `JWTAuth + TenantFromHeader + PasswordChangeGuard`):

```go
rolesRepo  := pgRoles.NewPostgresRepository(db)
rolesSvc   := appRoles.NewService(rolesRepo, logger)
rolesH     := handlerRoles.NewHandler(rolesSvc, logger)
rolesH.RegisterRoutes(v1)
```

Rutas:
```
GET    /api/v1/roles         → ListRoles
GET    /api/v1/roles/:id     → GetRole
POST   /api/v1/roles         → CreateRole
PUT    /api/v1/roles/:id     → UpdateRole
DELETE /api/v1/roles/:id     → DeleteRole
```

---

## Interacciones Pact a satisfacer

| # | Método | Path | Status | Handler |
|---|--------|------|--------|---------|
| 1 | GET | `/api/v1/roles` | 200 | `ListRoles` |
| 2 | GET | `/api/v1/roles/{id}` | 200 | `GetRole` |
| 3 | POST | `/api/v1/roles` | 201 | `CreateRole` |
| 4 | PUT | `/api/v1/roles/{id}` | 200 | `UpdateRole` |
| 5 | DELETE | `/api/v1/roles/{id}` | 200 | `DeleteRole` |
| 6 | DELETE | `/api/v1/roles/{id}` | 409 | `DeleteRole` (usuarios asignados) |
| 7 | DELETE | `/api/v1/roles/{id}` | 403 | `DeleteRole` (rol del sistema) |

---

## Verificación

1. **Migración**: `migrate -path migrations/ -database "..." up 1`
2. **Compilación**: `docker run ... golang:1.24-alpine sh -c "go build ./..."`
3. **Pruebas manuales** con `quickstart.md`:
   - GET `/api/v1/roles` → lista 4 roles del sistema
   - POST `/api/v1/roles` → crea rol custom
   - POST con mismo nombre → 409 DUPLICATE_NAME
   - POST con 3 roles existentes → 403 LIMIT_REACHED
   - GET `/api/v1/roles/:id` → detalle del rol
   - GET con ID inexistente → 404 NOT_FOUND
   - PUT `/api/v1/roles/:id` → actualiza nombre/permisos
   - DELETE rol custom sin asignaciones → 200
   - DELETE rol con usuarios asignados → 409 ROLE_HAS_ASSIGNMENTS
   - DELETE rol del sistema ("admin") → 403 SYSTEM_ROLE
