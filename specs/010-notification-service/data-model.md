# Data Model: Notification Service API (010)

**Feature**: Notification Service API  
**Date**: 2026-04-10

---

## Entidad: Notification

Representa un evento que requiere atención del operador, generado cuando una regla de alarma se dispara.

### Tabla: `notifications`

| Columna | Tipo | Restricciones | Descripción |
|---|---|---|---|
| `id` | UUID | PK DEFAULT gen_random_uuid() | Identificador único |
| `tenant_id` | UUID | NOT NULL | Aislamiento multi-tenant |
| `title` | TEXT | NOT NULL | Texto breve del evento (ej: "Temperatura crítica detectada") |
| `message` | TEXT | NOT NULL | Descripción detallada del evento |
| `severity` | VARCHAR(20) | NOT NULL CHECK IN (info/warning/critical/error) | Nivel de criticidad |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'unread' CHECK IN (unread/acknowledged/closed) | Estado de atención |
| `alarm_rule_id` | UUID | NULL | Referencia a la regla que disparó la notificación (histórica) |
| `machine_id` | UUID | NULL | Máquina involucrada (nullable si es evento de sistema) |
| `created_at` | TIMESTAMPTZ | NOT NULL DEFAULT NOW() | Timestamp de creación |
| `acknowledged_at` | TIMESTAMPTZ | NULL | Timestamp del acuse de recibo |
| `closed_at` | TIMESTAMPTZ | NULL | Timestamp de cierre |

### Índices

| Índice | Columnas | Tipo | Propósito |
|---|---|---|---|
| `idx_notifications_tenant_list` | `(tenant_id, created_at DESC)` | BTREE | Listado paginado por tenant |
| `idx_notifications_tenant_status` | `(tenant_id, status, created_at DESC)` | BTREE | Filtrado por status (conteo de unread) |
| `idx_notifications_tenant_severity` | `(tenant_id, severity, created_at DESC)` | BTREE | Filtrado por severidad |

### Notas de esquema

- Sin soft-delete: las notificaciones `closed` permanecen en el historial indefinidamente.
- `alarm_rule_id` es una referencia histórica sin FK constraint (la regla puede ser eliminada; la notificación permanece).
- `machine_id` es una referencia histórica sin FK constraint por la misma razón.
- Trigger `updated_at` NO necesario (no hay `updated_at` — los timestamps de transición son campos específicos).

---

## Máquina de estados: Notification.status

```
         POST /{id}/ack          POST /{id}/close
unread ──────────────────► acknowledged ──────────────► closed
  │                                                         ▲
  │                     POST /{id}/close                    │
  └─────────────────────────────────────────────────────────┘
```

**Reglas de transición**:
- `unread → acknowledged`: via `POST /notifications/{id}/ack`
- `unread → closed`: via `POST /notifications/{id}/close` (sin requerir ack previo)
- `acknowledged → closed`: via `POST /notifications/{id}/close`
- `closed → *`: no hay transición posible (estado terminal)
- Idempotencia: aplicar ack/close sobre estado ya alcanzado retorna éxito sin modificar datos

---

## Go Structs (domain)

```go
// internal/domain/notifications.go

type NotificationStatus string

const (
    StatusUnread       NotificationStatus = "unread"
    StatusAcknowledged NotificationStatus = "acknowledged"
    StatusClosed       NotificationStatus = "closed"
)

type NotificationSeverity string

const (
    SeverityInfo     NotificationSeverity = "info"
    SeverityWarning  NotificationSeverity = "warning"
    SeverityCritical NotificationSeverity = "critical"
    SeverityError    NotificationSeverity = "error"
)

type Notification struct {
    ID             uuid.UUID
    TenantID       uuid.UUID
    Title          string
    Message        string
    Severity       NotificationSeverity
    Status         NotificationStatus
    AlarmRuleID    *uuid.UUID  // nullable
    MachineID      *uuid.UUID  // nullable
    CreatedAt      time.Time
    AcknowledgedAt *time.Time  // nullable
    ClosedAt       *time.Time  // nullable
}

var ErrNotificationNotFound = errors.New("notificación no encontrada")
```

---

## Consultas clave

### Listar notificaciones (con filtros opcionales)

```sql
SELECT id, tenant_id, title, message, severity, status,
       alarm_rule_id, machine_id, created_at, acknowledged_at, closed_at
FROM notifications
WHERE tenant_id = $1
  AND ($2::text IS NULL OR status = $2)
  AND ($3::text IS NULL OR severity = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;
```

### Contar no leídas

```sql
SELECT COUNT(*) FROM notifications
WHERE tenant_id = $1 AND status = 'unread';
```

### Obtener por ID (con verificación de tenant)

```sql
SELECT ... FROM notifications
WHERE id = $1 AND tenant_id = $2;
```

### Acknowledge (idempotente)

```sql
UPDATE notifications
SET status = 'acknowledged',
    acknowledged_at = CASE WHEN status = 'unread' THEN NOW() ELSE acknowledged_at END
WHERE id = $1 AND tenant_id = $2
RETURNING *;
```

### Close (idempotente)

```sql
UPDATE notifications
SET status = CASE WHEN status = 'closed' THEN 'closed' ELSE 'closed' END,
    closed_at = CASE WHEN status != 'closed' THEN NOW() ELSE closed_at END
WHERE id = $1 AND tenant_id = $2
RETURNING *;
```

> Nota: la lógica de idempotencia (no sobreescribir `acknowledged_at`/`closed_at` si ya está seteado) se implementa con CASE en SQL o en la capa de servicio.
