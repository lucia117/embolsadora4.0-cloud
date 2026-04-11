# Data Model: Dashboard Layouts API

**Feature**: 005-dashboard-layouts
**Date**: 2026-03-24

---

## Entity: DashboardLayout

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | `UUID` | PK, DEFAULT `gen_random_uuid()` | Server-generated unique identifier |
| `tenant_id` | `UUID` | NOT NULL, FK → `tenants(id)` | Tenant affiliation (immutable) |
| `name` | `VARCHAR(255)` | NOT NULL | Layout display name (unique per active tenant) |
| `widgets` | `JSONB` | NOT NULL, DEFAULT `'[]'` | Array of widget objects |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Creation timestamp (immutable) |
| `updated_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT `now()` | Auto-updated on every modification |
| `deleted_at` | `TIMESTAMPTZ` | NULLABLE | Soft delete timestamp; NULL = active |

### Indexes

```sql
-- Primary key
PRIMARY KEY (id)

-- Tenant-scoped list queries (main access pattern)
CREATE INDEX idx_dashboard_layouts_tenant_id
  ON dashboard_layouts (tenant_id)
  WHERE deleted_at IS NULL;

-- Unique name per tenant (excludes soft-deleted rows)
CREATE UNIQUE INDEX idx_dashboard_layouts_tenant_name_active
  ON dashboard_layouts (tenant_id, name)
  WHERE deleted_at IS NULL;
```

### Triggers

```sql
-- Auto-update updated_at on row modification
CREATE TRIGGER trg_dashboard_layouts_updated_at
  BEFORE UPDATE ON dashboard_layouts
  FOR EACH ROW EXECUTE FUNCTION update_dashboard_layouts_updated_at();
```

---

## Widget Object (JSONB schema)

Widgets are stored as a JSONB array inside `dashboard_layouts.widgets`. Each element has the following structure:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Client-generated unique ID within the layout |
| `type` | string | Yes | Widget type (e.g., `machine-status`, `bag-counter`) |
| `name` | string | Yes | Internal widget identifier name |
| `title` | string | Yes | Display title shown in the UI |
| `description` | string | Yes | Short description of what the widget shows |
| `category` | string | Yes | Widget category (e.g., `overview`, `production`) |
| `icon` | string | Yes | Icon identifier (e.g., `Activity`, `Package`) |
| `position.x` | integer | Yes | Grid column position (0-based) |
| `position.y` | integer | Yes | Grid row position (0-based) |
| `position.w` | integer | Yes | Width in grid units |
| `position.h` | integer | Yes | Height in grid units |
| `position.i` | string | Yes | Grid item key (matches widget `id`) |

**Example**:
```json
[
  {
    "id": "machine-status-1708000000001",
    "type": "machine-status",
    "name": "Estado de Máquinas",
    "title": "Estado de Máquinas",
    "description": "Muestra el estado general de las máquinas",
    "category": "overview",
    "icon": "Activity",
    "position": { "x": 0, "y": 0, "w": 6, "h": 3, "i": "machine-status-1708000000001" }
  }
]
```

---

## Business Rules

1. **Max 3 layouts per tenant**: `COUNT(*) WHERE tenant_id = ? AND deleted_at IS NULL` must be `< 3` before INSERT.
2. **Unique name per tenant**: Enforced via partial unique index `(tenant_id, name) WHERE deleted_at IS NULL`.
3. **Cannot delete last layout**: `COUNT(*) WHERE tenant_id = ? AND deleted_at IS NULL` must be `> 1` before soft-delete.
4. **Full widget replacement**: On UPDATE, the entire `widgets` JSONB array is replaced.
5. **Soft delete only**: Rows are never physically deleted; `deleted_at` is set instead.

---

## Migration

**File**: `migrations/0006_create_dashboard_layouts_table.up.sql`

```sql
CREATE TABLE dashboard_layouts (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name       VARCHAR(255) NOT NULL,
  widgets    JSONB NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_dashboard_layouts_tenant_id
  ON dashboard_layouts (tenant_id)
  WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX idx_dashboard_layouts_tenant_name_active
  ON dashboard_layouts (tenant_id, name)
  WHERE deleted_at IS NULL;

CREATE TRIGGER set_dashboard_layouts_updated_at
  BEFORE UPDATE ON dashboard_layouts
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**File**: `migrations/0006_create_dashboard_layouts_table.down.sql`

```sql
DROP TABLE IF EXISTS dashboard_layouts;
```
