# Data Model: Edge Device Management

## Entity 1: EdgeDevice

Represents a physical edge computing unit (Raspberry Pi + PLC) registered within a tenant.

### Fields

| Field | Type | Nullable | Immutable | Unique | Default | Notes |
|-------|------|----------|-----------|--------|---------|-------|
| `id` | UUID | No | Yes | Yes (global) | gen_random_uuid() | Server-assigned UUID at creation |
| `tenant_id` | UUID | No | Yes | No | From path (resolved from subdomain) | FK → tenants(id); data isolation boundary |
| `name` | VARCHAR(255) | No | No | No | — | Human-readable device label; required |
| `description` | TEXT | Yes | No | No | NULL | Optional operational notes |
| `machine_id` | VARCHAR(100) | No | Yes | Yes (per tenant) | — | Physical machine identifier; unique per tenant; immutable |
| `edge_type` | VARCHAR(50) | No | Yes | No | — | Hardware profile enum: `RASPBERRY_PLC` |
| `raspberry_base_url` | TEXT | No | No | No | — | HTTP base URL to reach the Raspberry Pi service |
| `plc_address` | VARCHAR(255) | Yes | No | No | NULL | Optional PLC IP address |
| `status` | VARCHAR(20) | No | No | No | `ACTIVE` | Device state: `ACTIVE` or `DISABLED` |
| `last_seen_at` | TIMESTAMPTZ | Yes | No | No | NULL | Timestamp of last successful connectivity |
| `last_health_check_at` | TIMESTAMPTZ | Yes | No | No | NULL | Timestamp of last health check |
| `last_health_status` | VARCHAR(20) | No | No | No | `UNKNOWN` | Latest health result: `OK`, `DEGRADED`, `ERROR`, `UNKNOWN` |
| `last_health_summary` | TEXT | Yes | No | No | NULL | Human-readable summary of last health check |
| `created_at` | TIMESTAMPTZ | No | Yes | No | CURRENT_TIMESTAMP | ISO 8601 UTC; set at creation |
| `updated_at` | TIMESTAMPTZ | No | No | No | CURRENT_TIMESTAMP | ISO 8601 UTC; updated on any field change |

### Constraints

- **Primary Key**: `id`
- **Unique Constraint**: `(tenant_id, machine_id)` — machineId unique per tenant
- **Foreign Key**: `tenant_id` → `tenants(id)` ON DELETE CASCADE
- **Check**: `status IN ('ACTIVE', 'DISABLED')`
- **Check**: `edge_type IN ('RASPBERRY_PLC')` — extensible for future hardware types
- **Check**: `last_health_status IN ('OK', 'DEGRADED', 'ERROR', 'UNKNOWN')`

### Validation Rules

| Field | Rule | Error |
|-------|------|-------|
| `name` | Required, max 255 chars, non-empty | 400 VALIDATION_ERROR |
| `machine_id` | Required, max 100 chars, alphanum+dash+underscore | 400 VALIDATION_ERROR |
| `edge_type` | Required, must be in enum values | 400 VALIDATION_ERROR |
| `raspberry_base_url` | Required, valid HTTP/HTTPS URL | 400 VALIDATION_ERROR |
| `plc_address` | Optional; if provided, valid IP or hostname | 400 VALIDATION_ERROR |
| `machine_id` uniqueness | Must not exist for same tenant | 409 CONFLICT |

### State Transitions

```
Registration
    ↓
  ACTIVE (default)
    ↓ disable
  DISABLED
    ↓ enable
  ACTIVE

Notes:
- Status checks and health checks are only allowed in ACTIVE state.
- Telemetry is only available in ACTIVE state.
- Enable on ACTIVE → 200, no change (idempotent).
- Disable on DISABLED → 200, no change (idempotent).
```

### DB Schema (PostgreSQL)

```sql
CREATE TABLE edge_devices (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    machine_id          VARCHAR(100) NOT NULL,
    edge_type           VARCHAR(50) NOT NULL CHECK (edge_type IN ('RASPBERRY_PLC')),
    raspberry_base_url  TEXT NOT NULL,
    plc_address         VARCHAR(255),
    status              VARCHAR(20) NOT NULL DEFAULT 'ACTIVE'
                            CHECK (status IN ('ACTIVE', 'DISABLED')),
    last_seen_at        TIMESTAMPTZ,
    last_health_check_at TIMESTAMPTZ,
    last_health_status  VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN'
                            CHECK (last_health_status IN ('OK', 'DEGRADED', 'ERROR', 'UNKNOWN')),
    last_health_summary TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_edge_devices_tenant_machine UNIQUE (tenant_id, machine_id)
);

-- Indexes
CREATE INDEX idx_edge_devices_tenant_id ON edge_devices (tenant_id);
CREATE INDEX idx_edge_devices_tenant_status ON edge_devices (tenant_id, status);

-- Auto-update trigger for updated_at
CREATE OR REPLACE FUNCTION update_edge_devices_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_edge_devices_updated_at
    BEFORE UPDATE ON edge_devices
    FOR EACH ROW EXECUTE FUNCTION update_edge_devices_updated_at();
```

---

## Entity 2: DeviceEvent

Immutable record of a triggered check (status or health check) performed against an edge device. Persisted for audit and history purposes.

### Fields

| Field | Type | Nullable | Notes |
|-------|------|----------|-------|
| `id` | UUID | No | Server-assigned UUID |
| `device_id` | UUID | No | FK → edge_devices(id) ON DELETE CASCADE |
| `tenant_id` | UUID | No | Denormalized for efficient tenant-scoped queries |
| `check_type` | VARCHAR(20) | No | `STATUS` or `HEALTH_CHECK` |
| `checked_at` | TIMESTAMPTZ | No | Timestamp when check was executed |
| `overall_status` | VARCHAR(20) | No | `OK`, `DEGRADED`, `ERROR`, `UNKNOWN` |
| `summary` | TEXT | Yes | Human-readable result summary |
| `details` | JSONB | Yes | Type-specific payload (version for STATUS; metrics for HEALTH_CHECK) |
| `user_id` | UUID | No | User who triggered the check |
| `user_email` | VARCHAR(254) | No | Denormalized email at time of check |

### Constraints

- **Primary Key**: `id`
- **Foreign Key**: `device_id` → `edge_devices(id)` ON DELETE CASCADE
- **Check**: `check_type IN ('STATUS', 'HEALTH_CHECK')`
- **Check**: `overall_status IN ('OK', 'DEGRADED', 'ERROR', 'UNKNOWN')`
- Records are insert-only (never updated or deleted by the application)

### DB Schema (PostgreSQL)

```sql
CREATE TABLE device_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id       UUID NOT NULL REFERENCES edge_devices(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL,
    check_type      VARCHAR(20) NOT NULL CHECK (check_type IN ('STATUS', 'HEALTH_CHECK')),
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    overall_status  VARCHAR(20) NOT NULL
                        CHECK (overall_status IN ('OK', 'DEGRADED', 'ERROR', 'UNKNOWN')),
    summary         TEXT,
    details         JSONB,
    user_id         UUID NOT NULL,
    user_email      VARCHAR(254) NOT NULL
);

-- Indexes
CREATE INDEX idx_device_events_device_id ON device_events (device_id);
CREATE INDEX idx_device_events_tenant_id ON device_events (tenant_id);
CREATE INDEX idx_device_events_device_checked_at ON device_events (device_id, checked_at DESC);
```

---

## Relationships

```
tenants (1) ──── (N) edge_devices
edge_devices (1) ──── (N) device_events
users (1) ──── (N) device_events [via user_id + user_email]
```

---

## Migration File

**File**: `migrations/0005_create_edge_devices_tables.up.sql`

Contains both `edge_devices` and `device_events` tables in a single migration.

**Rollback**: `migrations/0005_create_edge_devices_tables.down.sql`

```sql
DROP TABLE IF EXISTS device_events;
DROP TABLE IF EXISTS edge_devices;
DROP FUNCTION IF EXISTS update_edge_devices_updated_at();
```
