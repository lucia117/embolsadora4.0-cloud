# Data Model: User Role Assignment Management

**Branch**: `001-user-role-assignments` | **Date**: 2026-02-27

---

## Entities

### 1. Role (Catalog)

A predefined, platform-managed permission level. Roles are seeded at migration time and are not user-created.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `id` | `VARCHAR(50)` | PRIMARY KEY | Short identifier (e.g., `admin`, `operario`) |
| `name` | `VARCHAR(255)` | NOT NULL | Human-readable display name (e.g., "Admin") |
| `description` | `TEXT` | nullable | Optional description of the role's purpose |
| `created_at` | `TIMESTAMPTZ` | DEFAULT NOW() | Record creation timestamp |

**Predefined rows** (seeded in migration):

| id | name |
|----|------|
| `admin` | Admin |
| `operario` | Operario |
| `cliente_admin` | Cliente Admin |
| `cliente_operario` | Cliente Operario |

**Go domain type**:
```go
// internal/domain/user_roles.go
type Role struct {
    ID          string
    Name        string
    Description string
    CreatedAt   time.Time
}
```

---

### 2. UserTenantRole (UTR) — Aggregate Root

Represents the assignment of a Role to a User within a Tenant. This is the core entity of this feature.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| `id` | `UUID` | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier for the assignment |
| `user_id` | `UUID` | NOT NULL, FK → users(id) | The user receiving the role |
| `tenant_id` | `UUID` | NOT NULL, FK → tenants(id) | The tenant context |
| `role_id` | `VARCHAR(50)` | FK → roles(id), nullable | The assigned role; nullable when status=pending |
| `status` | `VARCHAR(20)` | NOT NULL, DEFAULT 'pending', CHECK (active\|pending\|revoked) | Lifecycle state of the assignment |
| `assigned_by` | `UUID` | FK → users(id), nullable | ID of the user who performed the assignment |
| `assigned_at` | `TIMESTAMPTZ` | nullable | When the role was actively assigned |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | Record creation timestamp |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT NOW() | Last modification timestamp |

**Indexes**:
```sql
-- Enforces: one active role per user+tenant
CREATE UNIQUE INDEX idx_utr_active_unique
    ON user_tenant_roles (user_id, tenant_id)
    WHERE status = 'active';

CREATE INDEX idx_utr_tenant_id ON user_tenant_roles (tenant_id);
CREATE INDEX idx_utr_user_id   ON user_tenant_roles (user_id);
CREATE INDEX idx_utr_status    ON user_tenant_roles (status);
```

**Go domain types**:
```go
// internal/domain/user_roles.go

type UserRoleStatus string

const (
    UserRoleStatusActive  UserRoleStatus = "active"
    UserRoleStatusPending UserRoleStatus = "pending"
    UserRoleStatusRevoked UserRoleStatus = "revoked"
)

type UserTenantRole struct {
    ID         uuid.UUID
    UserID     uuid.UUID
    TenantID   uuid.UUID
    RoleID     *string        // nullable (pending state has no role yet)
    Status     UserRoleStatus
    AssignedBy *uuid.UUID     // nullable
    AssignedAt *time.Time     // nullable
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

// Used by GET /users/:userId/roles — JOIN with tenants + roles tables
type UserRoleWithContext struct {
    TenantID   uuid.UUID
    TenantName string
    RoleID     string
    RoleName   string
    Status     UserRoleStatus
}
```

**Domain errors** (additions to `internal/domain/errors.go`):
```go
var ErrUserAlreadyHasActiveRole = errors.New("user already has an active role in this tenant. Use PUT to update.")
var ErrAssignmentNotFound       = errors.New("user-role assignment not found")
```

---

## State Machine: UserTenantRole.status

```
             assign (POST /user-roles)
                    │
                    ▼
              ┌─────────────┐
              │   pending   │  ← initial state when status unset
              └──────┬──────┘
                     │  assign with roleId / promote
                     ▼
              ┌─────────────┐
              │   active    │  ← normal operating state
              └──────┬──────┘
                     │  revoke (DELETE /user-roles/:id)
                     ▼
              ┌─────────────┐
              │   revoked   │  ← terminal state (record preserved)
              └─────────────┘

Notes:
- active → active is blocked (409 Conflict)
- revoked → any is NOT supported (terminal)
- PUT /user-roles/:id updates roleId while keeping status=active
```

---

## Database Schema (Migration 000003)

### UP

```sql
-- Roles catalog
CREATE TABLE roles (
    id          VARCHAR(50)  PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO roles (id, name) VALUES
    ('admin',            'Admin'),
    ('operario',         'Operario'),
    ('cliente_admin',    'Cliente Admin'),
    ('cliente_operario', 'Cliente Operario');

-- User-Tenant-Role assignments
CREATE TABLE user_tenant_roles (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL REFERENCES users(id),
    tenant_id   UUID         NOT NULL REFERENCES tenants(id),
    role_id     VARCHAR(50)  REFERENCES roles(id),
    status      VARCHAR(20)  NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('active', 'pending', 'revoked')),
    assigned_by UUID         REFERENCES users(id),
    assigned_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- One active role per user per tenant
CREATE UNIQUE INDEX idx_utr_active_unique
    ON user_tenant_roles (user_id, tenant_id)
    WHERE status = 'active';

CREATE INDEX idx_utr_tenant_id ON user_tenant_roles (tenant_id);
CREATE INDEX idx_utr_user_id   ON user_tenant_roles (user_id);
CREATE INDEX idx_utr_status    ON user_tenant_roles (status);
```

### DOWN

```sql
DROP INDEX  IF EXISTS idx_utr_status;
DROP INDEX  IF EXISTS idx_utr_user_id;
DROP INDEX  IF EXISTS idx_utr_tenant_id;
DROP INDEX  IF EXISTS idx_utr_active_unique;
DROP TABLE  IF EXISTS user_tenant_roles;
DROP TABLE  IF EXISTS roles;
```

---

## Repository Interface

```go
// internal/repo/pg/user_roles/repository.go

type UserRoleRepository interface {
    // Query
    FindByTenant(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRole, error)
    FindByID(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error)
    FindByUser(ctx context.Context, userID uuid.UUID) ([]domain.UserRoleWithContext, error)

    // Mutations
    Create(ctx context.Context, utr *domain.UserTenantRole) error
    Update(ctx context.Context, utr *domain.UserTenantRole) error
    Revoke(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error)
    BulkCreate(ctx context.Context, utrs []domain.UserTenantRole) error // transactional
}
```

---

## Validation Rules

| Field | Rule |
|-------|------|
| `userId` (request) | Required; must be a valid UUID |
| `tenantId` (request) | Required; must be a valid UUID |
| `roleId` (request) | Required for assign/update/bulk; must match a known role ID |
| `userIds` (bulk request) | Required; non-empty list; each element must be a valid UUID |
| `status` (query filter) | Optional; if present, must be one of: `active`, `pending`, `revoked` |
| Uniqueness | At most one `status = 'active'` row per `(user_id, tenant_id)` — enforced by DB partial unique index |
