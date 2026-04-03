# Modelo de Datos: API de Gestión de Roles

**Feature**: `006-roles-management`
**Fecha**: 2026-04-03

---

## Tabla: `roles` (extendida)

La tabla ya existe (migration 000003). La migration 000012 agrega las columnas necesarias.

### Esquema completo post-migración

```sql
CREATE TABLE roles (
    id            VARCHAR(50)  PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    description   TEXT,
    -- Nuevas columnas (migration 000012):
    is_system_role BOOLEAN NOT NULL DEFAULT FALSE,
    is_global      BOOLEAN NOT NULL DEFAULT FALSE,
    tenant_id      UUID REFERENCES tenants(id) ON DELETE CASCADE,
    permissions    JSONB NOT NULL DEFAULT '[]',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ,
    -- Existente:
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Índices

```sql
-- Listado por tenant (incluyendo roles globales)
CREATE INDEX idx_roles_tenant_active
  ON roles (tenant_id)
  WHERE deleted_at IS NULL;

-- Unicidad de nombre por tenant para roles custom
CREATE UNIQUE INDEX idx_roles_tenant_name_active
  ON roles (tenant_id, name)
  WHERE deleted_at IS NULL AND is_system_role = FALSE;
```

### Datos pre-cargados (roles del sistema)

| id | name | is_system_role | is_global | tenant_id | permissions |
|----|------|---------------|-----------|-----------|-------------|
| `admin` | Admin | TRUE | TRUE | NULL | `["users:read","users:write","invitations:write","machines:read","machines:write","tenants:read"]` |
| `operario` | Operario | TRUE | TRUE | NULL | `["machines:read","machines:write"]` |
| `cliente_admin` | Cliente Admin | TRUE | TRUE | NULL | `["users:read","invitations:write","machines:read"]` |
| `cliente_operario` | Cliente Operario | TRUE | TRUE | NULL | `["machines:read"]` |

### Invariantes

| Regla | Dónde se aplica |
|-------|----------------|
| Roles del sistema: `tenant_id` siempre NULL | Datos de seed + validación en servicio |
| Roles custom: `tenant_id` siempre NOT NULL | Lógica de `CreateRole` |
| Máx. 3 roles custom activos por tenant | `CountCustomByTenant` en transacción de `Create` |
| Nombre único por tenant (custom) | Índice parcial + error `ErrRoleDuplicateName` |
| Roles del sistema no modificables ni eliminables | Validación en `UpdateRole` y `DeleteRole` |

---

## Tabla: `user_tenant_roles` (sin cambios)

La relación ya existe y usa `role_id VARCHAR(50)` como FK a `roles.id`. Los nuevos roles custom generados con IDs tipo `custom_3a9f12` serán válidos como `role_id`.

```sql
-- Relevante para CountActiveAssignments:
SELECT COUNT(*) FROM user_tenant_roles
WHERE role_id = $1 AND status = 'active';
```

---

## Entidad de Dominio: `Role`

```go
// internal/domain/roles.go
type Role struct {
    ID           string
    Name         string
    Description  string
    Permissions  []string  // strings opacos, deduplicados
    IsSystemRole bool
    IsGlobal     bool
    TenantID     *uuid.UUID // nil para roles del sistema
    CreatedAt    time.Time
    UpdatedAt    time.Time
    DeletedAt    *time.Time
}

const MaxCustomRolesPerTenant = 3

var (
    ErrRoleNotFound       = errors.New("role not found")
    ErrRoleIsSystemRole   = errors.New("cannot modify or delete system roles")
    ErrRoleHasAssignments = errors.New("role has active user assignments")
    ErrRoleDuplicateName  = errors.New("role name already exists in this tenant")
    ErrRoleLimitReached   = errors.New("maximum custom roles per tenant reached")
)
```

---

## Flujo de Datos por Operación

### GET /api/v1/roles

```
Handler → Service.ListRoles(tenantID) → Repo.List(tenantID)
SQL: WHERE (tenant_id = $1 OR is_global = TRUE) AND deleted_at IS NULL
     ORDER BY is_system_role DESC, name ASC
```

### POST /api/v1/roles

```
Handler → Service.CreateRole(tenantID, name, description, permissions)
  1. Repo.CountCustomByTenant(tenantID) → si >= 3 → ErrRoleLimitReached
  2. Generar ID: "custom_" + hex(rand[3])
  3. Deduplicar permissions
  4. Repo.Create(role)
     → pgErr 23505 → ErrRoleDuplicateName
```

### DELETE /api/v1/roles/:id

```
Handler → Service.DeleteRole(id)
  1. Repo.GetByID(id) → si no existe → ErrRoleNotFound
  2. Si role.IsSystemRole → ErrRoleIsSystemRole
  3. Repo.CountActiveAssignments(id) → si > 0 → ErrRoleHasAssignments
  4. Repo.SoftDelete(id)
```
