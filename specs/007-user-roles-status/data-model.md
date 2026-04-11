# Modelo de Datos: Extensión de Gestión de Usuarios (007)

**Fecha**: 2026-04-03  
**Feature**: `007-user-roles-status`

---

## Entidades Involucradas

### 1. User (existente — solo lectura extendida)

Tabla: `users`

| Campo | Tipo | Notas |
|---|---|---|
| id | UUID PK | Identificador interno |
| tenant_id | UUID FK | Tenant de pertenencia (nullable para usuarios Supabase sin tenant aún) |
| first_name | TEXT | Nombre |
| last_name | TEXT | Apellido |
| email | TEXT UNIQUE | Email único por tenant (constraint compuesto) |
| role | VARCHAR | 'admin' \| 'user' — rol legacy en tabla users |
| status | VARCHAR(20) | 'invited' \| 'active' \| 'revoked' \| 'disabled' |
| supabase_user_id | TEXT UNIQUE | ID de Supabase Auth |
| deleted_at | TIMESTAMPTZ | Soft delete |
| created_at | TIMESTAMPTZ | — |
| updated_at | TIMESTAMPTZ | — |

**Sin cambios de schema en esta tabla.**

---

### 2. UserTenantRole (existente — schema modificado)

Tabla: `user_tenant_roles`

| Campo | Tipo | Notas |
|---|---|---|
| id | UUID PK | — |
| user_id | UUID FK → users.id | — |
| tenant_id | UUID FK → tenants.id | — |
| role_id | VARCHAR(50) FK → roles.id | Nullable: assignments pendientes no tienen rol aún |
| status | VARCHAR(20) | **Extendido**: 'active' \| 'pending' \| 'revoked' \| **'suspended'** (nuevo) |
| assigned_by | UUID FK → users.id | Nullable |
| assigned_at | TIMESTAMPTZ | Nullable |
| created_at | TIMESTAMPTZ | — |
| updated_at | TIMESTAMPTZ | — |

**Cambio de schema**: extensión del CHECK constraint para incluir `'suspended'`.

**Índices existentes**:
- `idx_utr_active_unique`: UNIQUE ON (user_id, tenant_id) WHERE status = 'active' — garantiza máximo 1 rol activo por user+tenant

**Nueva migración**: `000013_add_suspended_status_to_user_tenant_roles.up.sql`

---

### 3. Role (existente — solo lectura)

Tabla: `roles`

| Campo | Tipo | Notas |
|---|---|---|
| id | VARCHAR(50) PK | 'admin', 'operario', 'custom_XXXXXX', etc. |
| name | TEXT | Nombre del rol |
| description | TEXT | Nullable |
| permissions | JSONB | Array de strings |
| is_system_role | BOOLEAN | true para roles del sistema |
| is_global | BOOLEAN | true para roles compartidos entre tenants |
| tenant_id | UUID FK → tenants.id | Nullable para roles del sistema |
| deleted_at | TIMESTAMPTZ | Soft delete |

**Sin cambios de schema.**

---

## Nuevas Queries (sin cambio de schema)

### Query 1: GetUserByIDWithRoles

```sql
SELECT 
    u.id, u.tenant_id, u.first_name, u.last_name, u.email, u.role, u.status,
    u.image, u.created_at, u.updated_at, u.deleted_at,
    utr.id      AS utr_id,
    utr.role_id AS utr_role_id,
    utr.status  AS utr_status,
    r.id        AS role_id,
    r.name      AS role_name,
    r.permissions AS role_permissions
FROM users u
LEFT JOIN user_tenant_roles utr 
    ON utr.user_id = u.id 
    AND utr.tenant_id = u.tenant_id 
    AND utr.status = 'active'
LEFT JOIN roles r 
    ON r.id = utr.role_id 
    AND r.deleted_at IS NULL
WHERE u.id = $1 
  AND u.tenant_id = $2 
  AND u.deleted_at IS NULL
```

### Query 2: ListPendingUsersByTenant

```sql
SELECT 
    u.id, u.tenant_id, u.first_name, u.last_name, u.email, u.role, u.status,
    u.image, u.created_at, u.updated_at
FROM users u
JOIN user_tenant_roles utr 
    ON utr.user_id = u.id 
    AND utr.tenant_id = $1 
    AND utr.status = 'pending'
WHERE u.deleted_at IS NULL
ORDER BY utr.created_at DESC
```

### Query 3: UpdateUserTenantRoleStatus

```sql
UPDATE user_tenant_roles
SET status = $1, updated_at = NOW()
WHERE user_id = $2 AND tenant_id = $3 AND status != 'pending'
RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
```

**Nota**: La condición `status != 'pending'` impide cambiar el estado de un usuario que nunca completó la activación via PATCH /status (eso se maneja via invitaciones, no status).

---

## Transiciones de Estado (UserTenantRole)

```
pending  ──(aceptar invitación)──►  active
active   ──(admin inactive)──────►  revoked
active   ──(admin suspend)────────►  suspended
revoked  ──(admin reactivate)─────►  active
suspended ──(admin reactivate)────►  active
```

**Regla**: Un admin no puede cambiar su propia entrada de `active` a otro estado (RF-006).

---

## Tipos de Dominio Nuevos/Modificados

### `internal/domain/user_roles.go` — Nuevo constante

```go
const (
    UserRoleStatusActive    UserRoleStatus = "active"
    UserRoleStatusPending   UserRoleStatus = "pending"
    UserRoleStatusRevoked   UserRoleStatus = "revoked"
    UserRoleStatusSuspended UserRoleStatus = "suspended"  // nuevo
)
```

### `internal/domain/users/user.go` — Nuevo tipo compuesto

```go
// UserWithRoles es el usuario con su rol activo en el tenant.
// Solo se usa cuando se solicita include=roles.
type UserWithRoles struct {
    User  // embed
    Roles []AssignedRole
}

type AssignedRole struct {
    ID          string
    Name        string
    Permissions []string
}
```
