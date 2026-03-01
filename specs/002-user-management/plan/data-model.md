# Data Model: User Management

## Entity: User

Represents a platform user with admin or operario role within a tenant.

### Fields

| Field | Type | Nullable | Immutable | Unique | Default | Notes |
|-------|------|----------|-----------|--------|---------|-------|
| `id` | UUID/String | No | Yes | Yes (global) | Server-generated | Unique identifier, server-assigned at creation |
| `tenant_id` | UUID/String | No | Yes | No | From X-Tenant-ID header | Organization the user belongs to; scoped isolation |
| `first_name` | String | No | No | No | | Max 100 characters; required |
| `last_name` | String | No | No | No | | Max 100 characters; required |
| `email` | String | No | Yes | Yes (per tenant) | | Valid email format; unique within tenant, not globally |
| `role` | Enum | No | No | No | `user` | Values: `admin`, `user`, (extensible for future roles) |
| `image` | URL/String | Yes | No | No | NULL | Optional avatar image URL; nullable |
| `created_at` | Timestamp | No | Yes | No | CURRENT_TIMESTAMP | ISO 8601, UTC timezone |
| `updated_at` | Timestamp | No | No | No | CURRENT_TIMESTAMP | ISO 8601, UTC timezone; updated on any field change |
| `deleted_at` | Timestamp | Yes | No | No | NULL | Soft delete: NULL if active, timestamp if deleted |

### Constraints

- **Primary Key**: (id)
- **Unique Constraint**: (tenant_id, email) — email is unique per tenant, not globally
- **Foreign Key**: tenant_id references tenants(id) with ON DELETE CASCADE
- **Index**: (tenant_id, deleted_at) — for efficient listing with soft-delete filtering
- **Index**: (tenant_id, email) — for email uniqueness and lookup
- **Check Constraint**: role IN ('admin', 'user') — enforces valid role values

### Validation Rules

| Field | Rule | Error Code |
|-------|------|-----------|
| `first_name` | Required, max 100 chars, non-empty | 400 VALIDATION_ERROR |
| `last_name` | Required, max 100 chars, non-empty | 400 VALIDATION_ERROR |
| `email` | Required, valid RFC 5322, max 254 chars | 400 VALIDATION_ERROR |
| `role` | Required, must be one of defined enum values | 400 VALIDATION_ERROR |
| `image` | Optional, must be valid URL if provided | 400 VALIDATION_ERROR |
| `tenant_id` | Required (from header), must exist in tenants table | 400 BAD_REQUEST / 403 FORBIDDEN |
| email uniqueness (per tenant) | Must not exist for same tenant_id | 409 CONFLICT |
| immutable fields | Cannot update id, tenant_id, email, created_at | 400 VALIDATION_ERROR |

### State Transitions

```
Active (deleted_at IS NULL)
    ↓
    DELETE request → Soft Deleted (deleted_at IS NOT NULL)
    ↓
    [End of lifecycle; no restore in MVP]

Note: Soft-deleted users do not appear in list queries and return 404 on direct access.
```

### DB Schema (PostgreSQL)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(254) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    image TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE (tenant_id, email),
    INDEX idx_tenant_deleted (tenant_id, deleted_at),
    INDEX idx_tenant_email (tenant_id, email)
);
```

### Relationships

- **Tenant** (1:N): Many users belong to one tenant. Tenant acts as data isolation boundary (no cross-tenant access).
- **Role** (implicit enum): User has one role determining permissions. Role values: `admin` (write/delete/read), `user` (read-only by default, see RBAC matrix).

### Audit Trail

Due to soft delete, deleted users retain complete historical data:
- `created_at` preserved
- `updated_at` shows last modification before deletion
- `deleted_at` shows deletion timestamp
- No hard deletion = full audit trail for compliance

