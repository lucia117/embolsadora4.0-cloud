# Data Model: Permissions Management API

**Feature**: 011-permissions-management  
**Date**: 2026-04-10

---

## Entidades

### Permission

Representa un permiso del sistema de control de acceso.

| Campo | Tipo | Nulable | Descripción |
|-------|------|---------|-------------|
| `id` | `TEXT` | No | Identificador único. Para permisos de sistema: `perm_<nombre>` (ej: `perm_dashboard`). Para permisos custom: UUID v4 |
| `name` | `TEXT` | No | Nombre legible del permiso. Mínimo 3 caracteres |
| `section` | `TEXT` | No | Sección funcional de la aplicación (dashboard, logs, reports, maintenance, analytics, users, tenants, settings, all-tenants) |
| `description` | `TEXT` | No | Descripción del permiso |
| `is_system_permission` | `BOOLEAN` | No | `TRUE` para permisos predefinidos del producto. `FALSE` para permisos custom creados por tenants |
| `tenant_id` | `UUID` | Sí | `NULL` para permisos de sistema (globales). UUID del tenant propietario para permisos custom |
| `created_at` | `TIMESTAMPTZ` | No | Timestamp de creación (gestionado por la BD) |
| `updated_at` | `TIMESTAMPTZ` | No | Timestamp de última actualización (actualizado via trigger) |

**Restricciones**:
- `tenant_id IS NULL` cuando `is_system_permission = TRUE`
- `tenant_id IS NOT NULL` cuando `is_system_permission = FALSE`
- `name` mínimo 3 caracteres (validado en capa de servicio)
- Los permisos de sistema no tienen soft-delete (son inmutables)
- Los permisos custom son eliminados permanentemente (no hay columna `deleted_at`)

---

## Schema SQL

### Tabla `permissions`

```sql
CREATE TABLE permissions (
    id                   TEXT            PRIMARY KEY,
    name                 TEXT            NOT NULL CHECK (char_length(name) >= 3),
    section              TEXT            NOT NULL CHECK (char_length(section) > 0),
    description          TEXT            NOT NULL CHECK (char_length(description) > 0),
    is_system_permission BOOLEAN         NOT NULL DEFAULT FALSE,
    tenant_id            UUID            REFERENCES tenants(id) ON DELETE CASCADE,
    created_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_system_perm_no_tenant
        CHECK (NOT (is_system_permission = TRUE AND tenant_id IS NOT NULL)),
    CONSTRAINT chk_custom_perm_has_tenant
        CHECK (NOT (is_system_permission = FALSE AND tenant_id IS NULL))
);
```

**Índices**:
```sql
-- Listado por tenant (query principal)
CREATE INDEX idx_permissions_tenant_id ON permissions(tenant_id)
    WHERE tenant_id IS NOT NULL;

-- Listado de permisos de sistema (always-read)
CREATE INDEX idx_permissions_system ON permissions(is_system_permission)
    WHERE is_system_permission = TRUE;
```

**Trigger updated_at**:
```sql
CREATE OR REPLACE FUNCTION update_permissions_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_permissions_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION update_permissions_updated_at();
```

---

## Seed de permisos de sistema

Los 17 permisos de sistema se insertan en la misma migración `000017`:

```sql
INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_dashboard',          'View Dashboard',          'dashboard',    'Access to main dashboard and widgets',                        TRUE, NULL),
    ('perm_alerts',             'View Alerts',             'alerts',       'Access to alerts and notification center',                    TRUE, NULL),
    ('perm_reports',            'View Reports',            'reports',      'Access to reports and analytics',                             TRUE, NULL),
    ('perm_users',              'Manage Users',            'users',        'Create, edit and delete users',                               TRUE, NULL),
    ('perm_tenants',            'Manage Tenants',          'tenants',      'Access to tenant management',                                 TRUE, NULL),
    ('perm_settings',           'Manage Settings',         'settings',     'Access to system settings',                                   TRUE, NULL),
    ('perm_maintenance',        'View Maintenance',        'maintenance',  'Access to maintenance module',                                TRUE, NULL),
    ('perm_analytics',          'View Analytics',          'analytics',    'Access to analytics dashboards',                              TRUE, NULL),
    ('perm_all_tenants',        'Access All Tenants',      'all-tenants',  'Cross-tenant access (Super Admin only)',                       TRUE, NULL),
    ('perm_logs_view',          'View Logs',               'logs',         'Access to log viewer',                                        TRUE, NULL),
    ('perm_logs_export',        'Export Logs',             'logs',         'Export log data to file',                                     TRUE, NULL),
    ('perm_logs_admin',         'Manage Log Settings',     'logs',         'Manage log retention and configuration',                      TRUE, NULL),
    ('perm_edge_devices_view',  'View Edge Devices',       'maintenance',  'View edge device list and status',                            TRUE, NULL),
    ('perm_edge_devices_manage','Manage Edge Devices',     'maintenance',  'Create, edit, enable and disable edge devices',               TRUE, NULL),
    ('perm_edge_devices_check', 'Run Edge Checks',         'maintenance',  'Execute status and health checks on edge devices',            TRUE, NULL),
    ('perm_reports_view',       'View Reports',            'reports',      'Access to report history and download',                       TRUE, NULL),
    ('perm_reports_manage',     'Manage Reports',          'reports',      'Generate reports, manage schedules and retention settings',   TRUE, NULL)
ON CONFLICT (id) DO NOTHING;
```

---

## Query principal — Listado

```sql
SELECT id, name, section, description, is_system_permission, tenant_id, created_at, updated_at
FROM permissions
WHERE (tenant_id = $1 OR is_system_permission = TRUE)
ORDER BY is_system_permission DESC, name ASC
```

Esta query retorna primero los permisos de sistema (ordenados por nombre), luego los permisos custom del tenant (ordenados por nombre).

---

## Relaciones

```
permissions
  ├── tenant_id → tenants.id (NULL para permisos de sistema)
  └── [futuro] roles.permissions[] → permissions.id (referenciado por roles, no FK directa)
```

**Nota**: Los roles almacenan permisos como `TEXT[]` en la columna `permissions`. No hay FK directa entre `roles.permissions` y `permissions.id`. Esta desconexión es intencional en el MVP; la validación de existencia de permisos al asignar roles es un scope futuro del role-service.

---

## Entidad de dominio Go

```go
// internal/domain/permissions.go

type Permission struct {
    ID                 string
    Name               string
    Section            string
    Description        string
    IsSystemPermission bool
    TenantID           *uuid.UUID  // nil para permisos de sistema
    CreatedAt          time.Time
    UpdatedAt          time.Time
}
```

**Errores de dominio**:
```go
var (
    ErrPermissionNotFound        = errors.New("permiso no encontrado")
    ErrPermissionIsSystem        = errors.New("los permisos del sistema no pueden modificarse ni eliminarse")
    ErrPermissionValidationFailed = errors.New("datos del permiso inválidos")
)
```
