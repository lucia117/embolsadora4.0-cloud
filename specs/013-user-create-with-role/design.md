---
id: 013
spec: ./spec.md
title: "Diseño superpowers de la misma feature (duplica la spec de speckit)"
origin: superpowers
former_file: docs/superpowers/specs/2026-04-11-create-user-with-initial-role-design.md
last_reviewed: 2026-09-29
---

# Design: POST /users con asignación de rol inicial

**Fecha**: 2026-04-11  
**Feature**: Completar `user-service-api-roles-extension` — 1 interacción Pact pendiente  
**Branch**: a crear desde `develop`

---

## Contexto

El endpoint `POST /api/v1/users` ya existe y crea el registro en la tabla `users`. El contrato Pact `user-service-api-roles-extension` espera que la creación de un usuario también genere una entrada activa en `user_tenant_roles` (UTR), de modo que el usuario quede asignado a un rol en el tenant desde el momento de su creación.

Actualmente esa asignación no ocurre: el usuario se crea huérfano (sin UTR) y queda inaccesible para operaciones que requieren rol activo.

---

## Decisiones de Diseño

### D1: El campo `role` es el `role_id` de la tabla `roles`

El campo `role` en `CreateUserRequest` ya existe como `string`. La tabla `roles` usa `VARCHAR(50)` como PK (`"admin"`, `"operario"`, etc.; UUIDs para roles custom). No se agrega un campo nuevo — se reutiliza `role` con validación ampliada.

**Cambio**: la validación pasa de `oneof=admin user` a solo `required`. El backend no valida existencia en BD (la FK en `user_tenant_roles.role_id → roles.id` lo hace a nivel base de datos).

### D2: UTR con status `active` desde la creación

La asignación directa por un admin no pasa por el flujo de invitaciones. El UTR se crea con `status = 'active'` y `assigned_at = NOW()`.

### D3: Transacción en la capa repo (no en el service)

Se agrega `CreateWithRole` a la interfaz del users repo. La implementación abre una `pgx.Tx`, ejecuta ambos INSERTs, hace commit. Si falla cualquiera, rollback automático. El service no sabe de SQL ni de transacciones.

**Alternativa descartada**: dos operaciones separadas en el service (Opción A). Descartada porque si el segundo INSERT falla, el usuario queda sin rol y sin forma automática de recuperación.

### D4: `assignedBy` viene del JWT

El handler extrae el UUID del admin autenticado via `platform.UserID(c.Request.Context())` y lo pasa al `CreateUserCommand`. El mismo patrón ya se usa en `UpdateUserStatus`.

### D5: Response sin cambios

El response de `POST /users` sigue siendo `UserResponse` (sin campo `roles`). El Pact solo valida que el usuario fue creado; la inclusión de roles en el response corresponde a `GET /users/:id?include=roles` (ya implementado).

---

## Cambios por Capa

### 1. DTO — `internal/api/handler/users/dto/create.go`

```go
type CreateUserRequest struct {
    FirstName string  `json:"firstName" binding:"required,max=100"`
    LastName  string  `json:"lastName"  binding:"required,max=100"`
    Email     string  `json:"email"     binding:"required,email"`
    Role      string  `json:"role"      binding:"required"`        // era: oneof=admin user
    Image     *string `json:"image"`
}
```

### 2. Domain — `internal/domain/users/commands.go`

```go
type CreateUserCommand struct {
    TenantID   string
    FirstName  string
    LastName   string
    Email      string
    Role       string
    Image      *string
    AssignedBy string  // UUID del admin — nuevo campo
}
```

### 3. Repo interface — `internal/repo/pg/users/users_repo.go`

Nuevo método en la interfaz `Repository`:

```go
// CreateWithRole crea el usuario y su UTR activo en una sola transacción.
CreateWithRole(ctx context.Context, user *domainUsers.User, utr *domain.UserTenantRole) (*domainUsers.User, error)
```

Implementación: `pgx.Tx` con INSERT en `users` + INSERT en `user_tenant_roles`. Rollback si cualquiera falla. Manejo de errores FK para `role_id` inválido → `domain.ErrInvalidRoleID`.

### 4. Service — `internal/app/users/service.go`

`CreateUser` construye el UTR y llama `repo.CreateWithRole`:

```go
utr := &domain.UserTenantRole{
    ID:         uuid.New(),
    UserID:     uuid.MustParse(created.ID),  // después del INSERT en users
    TenantID:   uuid.MustParse(tenantID),
    RoleID:     &cmd.Role,
    Status:     domain.UserRoleStatusActive,
    AssignedBy: &assignedByUUID,
    AssignedAt: &now,
}
```

Nuevo error a manejar: `domain.ErrInvalidRoleID` → 400 en el handler.

### 5. Handler — `internal/api/handler/users/handler.go`

```go
callerUUID := platform.UserID(c.Request.Context())
if callerUUID == nil {
    // 401
}
cmd := &domainUsers.CreateUserCommand{
    ...,
    AssignedBy: callerUUID.String(),
}
```

### 6. Error mapping — `internal/api/handler/users/errors.go`

Agregar caso para `domain.ErrInvalidRoleID` → HTTP 400, código `"INVALID_ROLE"`.

---

## Flujo Completo

```
POST /api/v1/users
  Header: X-Tenant-ID, Authorization: Bearer <jwt>
  Body: { firstName, lastName, email, role, image? }

Handler:
  1. Extrae tenant_id del contexto (middleware)
  2. Extrae caller UUID del JWT (platform.UserID)
  3. Bind + valida DTO
  4. Construye CreateUserCommand con AssignedBy

Service.CreateUser:
  1. Valida command
  2. Construye domain.User
  3. Construye domain.UserTenantRole (status=active, assigned_at=now)
  4. Llama repo.CreateWithRole(ctx, user, utr)

Repo.CreateWithRole (tx):
  BEGIN
  INSERT INTO users → devuelve user con ID generado
  INSERT INTO user_tenant_roles → con user_id del paso anterior
  COMMIT (o ROLLBACK si falla)

Handler:
  5. Responde 201 con UserResponse
```

---

## Casos de Error

| Condición | Error | HTTP |
|---|---|---|
| `role` vacío o ausente | VALIDATION_ERROR | 400 |
| `role` no existe en tabla `roles` | INVALID_ROLE | 400 |
| Email duplicado en tenant | EMAIL_TAKEN | 409 |
| Admin no autenticado | UNAUTHORIZED | 401 |
| Usuario ya tiene UTR activo (raro, pero posible) | CONFLICT | 409 |

---

## Scope

- No hay migración nueva
- No cambia el response shape
- No afecta otros endpoints
- Backward compatible: el campo `role` ya existía en el request
