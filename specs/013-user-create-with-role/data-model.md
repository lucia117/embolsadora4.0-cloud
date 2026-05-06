# Data Model: POST /users con Asignación de Rol Inicial

**Fecha**: 2026-04-11

---

## Entidades Involucradas (sin cambios de schema)

### `users` (existente)

| Columna | Tipo | Notas |
|---|---|---|
| id | UUID PK | Generado en repo |
| tenant_id | UUID FK → tenants | |
| first_name | VARCHAR(100) | |
| last_name | VARCHAR(100) | |
| email | VARCHAR(255) | Único por tenant |
| role | VARCHAR(50) | Campo denormalizado (igual al role_id asignado) |
| image | TEXT nullable | |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |
| deleted_at | TIMESTAMPTZ nullable | Soft delete |

### `user_tenant_roles` (existente)

| Columna | Tipo | Notas |
|---|---|---|
| id | UUID PK | Generado en repo |
| user_id | UUID FK → users | |
| tenant_id | UUID FK → tenants | |
| role_id | VARCHAR(50) FK → roles nullable | Seteado desde el inicio (no pending) |
| status | VARCHAR(20) | `'active'` en creación directa |
| assigned_by | UUID FK → users nullable | UUID del admin autenticado |
| assigned_at | TIMESTAMPTZ nullable | NOW() |
| created_at | TIMESTAMPTZ | |
| updated_at | TIMESTAMPTZ | |

**Constraint relevante**: `idx_utr_active_unique` — índice único parcial sobre `(user_id, tenant_id) WHERE status = 'active'`. Garantiza máximo 1 rol activo por usuario por tenant.

---

## Domain Objects Modificados

### `CreateUserCommand` (extendido)

```go
type CreateUserCommand struct {
    TenantID   string  // UUID del tenant (del header X-Tenant-ID)
    FirstName  string
    LastName   string
    Email      string
    Role       string  // roles.id — validación por FK
    Image      *string
    AssignedBy string  // UUID del admin — NUEVO
}
```

---

## Flujo de Datos

```
HTTP Request
  ↓
Handler: extrae tenant_id (middleware), caller_id (JWT)
  ↓
CreateUserCommand{..., AssignedBy: callerID}
  ↓
Service.CreateUser: construye User + UserTenantRole
  ↓
Repo.CreateWithRole(ctx, user, utr)
  ↓ BEGIN TX
  INSERT INTO users → user con ID
  INSERT INTO user_tenant_roles (user_id, tenant_id, role_id, status='active', assigned_by, assigned_at)
  COMMIT
  ↓
Service retorna *User
  ↓
Handler responde 201 UserResponse
```

---

## Errores de Dominio (sin cambios, solo nuevo mapeo HTTP)

| Error | Origen | HTTP | Código |
|---|---|---|---|
| `ErrEmailTaken` | Constraint unique (tenant_id, email) | 409 | `EMAIL_TAKEN` |
| `ErrInvalidRoleID` | FK violation role_id → roles | 400 | `INVALID_ROLE` |
| `ErrUserAlreadyHasActiveRole` | Índice único UTR activo | 409 | `CONFLICT` |
| `ErrValidation` | Validación del command | 400 | `VALIDATION_ERROR` |
