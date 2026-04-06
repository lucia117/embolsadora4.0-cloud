# Plan de Implementación: Extensión de Gestión de Usuarios

**Branch**: `007-user-roles-status` | **Fecha**: 2026-04-03 | **Spec**: [spec.md](spec.md)  
**Input**: Spec de feature `007-user-roles-status`

---

## Resumen

Extender la API de gestión de usuarios con 3 endpoints que satisfacen los contratos Pact de `user-service-api-roles-extension`:

1. `GET /users/:id?include=roles` — retorna el usuario con su rol activo en el tenant (JOIN con UTR + roles)
2. `PATCH /users/:id/status` — cambia el estado de participación del usuario en el tenant (active/inactive/suspended)
3. `GET /users/pending` — lista usuarios con UTR.status = 'pending' para el tenant

---

## Contexto Técnico

**Lenguaje/Versión**: Go 1.24+  
**Framework HTTP**: Gin  
**ORM/Driver**: pgx/v5 (queries SQL manuales)  
**Base de datos**: PostgreSQL (migraciones en `migrations/`)  
**Logging**: Zap estructurado  
**Testing**: testify + colección Postman  
**Tipo de proyecto**: Web service (monolito modular hexagonal)

---

## Constitution Check

| Gate | Estado | Justificación |
|---|---|---|
| Arquitectura hexagonal | ✅ PASS | Flujo: handler → service → repo/domain |
| Aislamiento multi-tenant | ✅ PASS | Todas las queries incluyen tenant_id; ExtractTenantID middleware |
| Seguridad / RBAC | ✅ PASS | PATCH y GET pending requieren RequireRole("admin") |
| Observabilidad | ✅ PASS | Logs Zap en service layer, mismo patrón que handlers existentes |
| Backward compatibility | ✅ PASS | GET /users/:id sin parámetro es idéntico al actual |
| Contrato OpenAPI | ✅ PASS | Contrato generado en contracts/ |
| Tests de integración | ⚠️ PENDIENTE | Validación manual via Postman/quickstart.md (igual que features previas) |

---

## Estructura del Proyecto

### Documentación

```text
specs/007-user-roles-status/
├── plan.md          ← este archivo
├── research.md      ← decisiones técnicas
├── data-model.md    ← schema y queries
├── quickstart.md    ← curls de validación
├── contracts/
│   └── user-service-api-extension.openapi.yaml
└── tasks.md         ← generado por /speckit.tasks
```

### Código a crear/modificar

```text
migrations/
└── 000013_add_suspended_status_to_utr.up.sql    [NUEVO]
└── 000013_add_suspended_status_to_utr.down.sql  [NUEVO]

internal/domain/
└── user_roles.go                    [MODIFICAR: + UserRoleStatusSuspended]
└── users/
    └── user.go                      [MODIFICAR: + UserWithRoles, AssignedRole]

internal/repo/pg/users/
├── repository.go                    [MODIFICAR: + GetByIDWithRoles]
├── postgres.go                      [MODIFICAR: + implementación GetByIDWithRoles]
└── queries.go (o resources.go)      [MODIFICAR: + SQL GetUserWithRoles]

internal/repo/pg/user_roles/
├── repository.go                    [MODIFICAR: + ListPendingByTenant, UpdateStatus]
├── repository_impl.go               [MODIFICAR: + implementaciones]
└── resources.go                     [MODIFICAR: + SQLs nuevos]

internal/app/users/
└── service.go                       [MODIFICAR: + GetUserWithRoles, ListPendingUsers, UpdateUserStatus]

internal/api/handler/users/
├── handler.go                       [MODIFICAR: + GetUserWithRoles, ListPendingUsers, UpdateUserStatus]
└── dto/                             [MODIFICAR: + UpdateStatusRequest, UserWithRolesResponse]

internal/api/router.go               [MODIFICAR: + 3 rutas nuevas]

postman/
└── Embolsadora-API-Complete.postman_collection.json  [MODIFICAR: + carpeta Users Extension]

PACTS_ANALYSIS.md                    [MODIFICAR: marcar 3/4 interactions ✅]
```

---

## Fases de Implementación

### Fase 1 — Migración

Agregar `suspended` al CHECK constraint de `user_tenant_roles.status`.

```sql
-- 000013_add_suspended_status_to_utr.up.sql
ALTER TABLE user_tenant_roles
  DROP CONSTRAINT IF EXISTS user_tenant_roles_status_check,
  ADD CONSTRAINT user_tenant_roles_status_check
    CHECK (status IN ('active', 'pending', 'revoked', 'suspended'));
```

```sql
-- 000013_add_suspended_status_to_utr.down.sql
ALTER TABLE user_tenant_roles
  DROP CONSTRAINT IF EXISTS user_tenant_roles_status_check,
  ADD CONSTRAINT user_tenant_roles_status_check
    CHECK (status IN ('active', 'pending', 'revoked'));
```

### Fase 2 — Dominio

**`internal/domain/user_roles.go`**:
```go
UserRoleStatusSuspended UserRoleStatus = "suspended"
```

**`internal/domain/users/user.go`**:
```go
type UserWithRoles struct {
    User
    Roles []AssignedRole
}

type AssignedRole struct {
    ID          string
    Name        string
    Permissions []string
}
```

### Fase 3 — Repositorios

**`internal/repo/pg/users/repository.go`** — extender interface:
```go
GetByIDWithRoles(ctx context.Context, tenantID, userID string) (*users.UserWithRoles, error)
```

**`internal/repo/pg/users/postgres.go`** — implementación con JOIN:
```sql
SELECT u.*, utr.role_id AS utr_role_id, r.id AS role_id, r.name AS role_name, r.permissions AS role_permissions
FROM users u
LEFT JOIN user_tenant_roles utr ON utr.user_id = u.id AND utr.tenant_id = u.tenant_id AND utr.status = 'active'
LEFT JOIN roles r ON r.id = utr.role_id AND r.deleted_at IS NULL
WHERE u.id = $1 AND u.tenant_id = $2 AND u.deleted_at IS NULL
```

**`internal/repo/pg/user_roles/repository.go`** — extender interface:
```go
ListPendingByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.UserTenantRole, error)
UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus) error
```

### Fase 4 — Servicio

**`internal/app/users/service.go`** — 3 métodos nuevos:

```go
func (s *Service) GetUserWithRoles(ctx context.Context, tenantID, userID string) (*users.UserWithRoles, error)
func (s *Service) ListPendingUsers(ctx context.Context, tenantID string) ([]*users.User, error)
func (s *Service) UpdateUserStatus(ctx context.Context, tenantID, userID, callerID, status string) (*users.User, error)
```

`UpdateUserStatus` valida:
1. `userID == callerID` → error `ErrCannotDeactivateSelf`
2. `status` in {active, inactive, suspended} → error `ErrInvalidStatus`
3. Lookup UTR por (userID, tenantID), mapear status → UTR status, llamar `userRoleRepo.UpdateStatus`

Necesita inyectar `UserRoleRepository` en el `Service`. Actualmente el service solo tiene `UserRepository`. Agregar dependencia.

### Fase 5 — Handlers y DTOs

**DTOs nuevos** (`internal/api/handler/users/dto/`):

```go
type UpdateUserStatusRequest struct {
    Status string `json:"status" binding:"required"`
}

// Extender UserResponse para incluir roles opcionales
type UserWithRolesResponse struct {
    UserResponse
    Roles []RoleInfo `json:"roles"`
}

type RoleInfo struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Permissions []string `json:"permissions"`
}
```

**Métodos nuevos en Handler** (`internal/api/handler/users/handler.go`):

```go
func (h *Handler) GetUserWithRoles(c *gin.Context)  // GET /users/:id?include=roles
func (h *Handler) ListPendingUsers(c *gin.Context)   // GET /users/pending
func (h *Handler) UpdateUserStatus(c *gin.Context)   // PATCH /users/:id/status
```

`GetUserWithRoles` extiende el handler `GetUser` existente — si `c.Query("include") == "roles"`, llama a `service.GetUserWithRoles`, si no, llama al `service.GetUser` de siempre.

### Fase 6 — Rutas

**`internal/api/router.go`** — en `RegisterAdminRoutes`, dentro del bloque `userRoutes`:

```go
// IMPORTANTE: /users/pending debe ir ANTES de /users/:id para evitar conflicto
userRoutes.GET("/users/pending", middleware.RequireRole("admin"), uh.ListPendingUsers)
userRoutes.GET("/users", uh.ListUsers)
userRoutes.GET("/users/:id", uh.GetUser)          // GetUser detecta ?include=roles internamente
userRoutes.POST("/users", middleware.RequireRole("admin"), uh.CreateUser)
userRoutes.PATCH("/users/:id", middleware.RequireRole("admin"), uh.UpdateUser)
userRoutes.PATCH("/users/:id/status", middleware.RequireRole("admin"), uh.UpdateUserStatus)
userRoutes.DELETE("/users/:id", middleware.RequireRole("admin"), uh.DeleteUser)
```

### Fase 7 — Postman + PACTS_ANALYSIS

Agregar carpeta "Users — Extension (007)" a la colección Postman con 7 requests (escenarios del quickstart.md).

Actualizar `PACTS_ANALYSIS.md`: marcar 3/4 interactions de `user-service-api-roles-extension` como ✅.

---

## Dependencias entre Fases

```
Fase 1 (migración)
    │
    ▼
Fase 2 (dominio)
    │
    ├──► Fase 3 repos (users)
    │         │
    │         └──► Fase 4 service ──► Fase 5 handlers ──► Fase 6 rutas
    │
    └──► Fase 3 repos (user_roles)
```

Fases 3 (users) y 3 (user_roles) pueden ejecutarse en paralelo.  
Fases 7 (Postman + docs) son independientes y pueden ejecutarse al final.

---

## Verificación End-to-End

1. Aplicar migración 000013: `migrate -path migrations/ -database $DATABASE_URL up 1`
2. Compilar: `docker run ... go build ./...`
3. Ejecutar los curls de [quickstart.md](quickstart.md) en orden
4. Verificar que los 4 escenarios del checklist Pact pasan
5. Verificar que `GET /users/:id` sin `include` sigue retornando el mismo response (backward-compat)
