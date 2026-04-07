# Data Model: Log Service API (009)

**Feature**: `009-log-service`  
**Date**: 2026-04-07

---

## Entidades

### LogEntry

Representa un evento del sistema. Registro inmutable (no se actualiza ni hace soft-delete).

| Campo | Tipo | Constraints | Descripción |
|-------|------|-------------|-------------|
| `id` | UUID | PK, DEFAULT gen_random_uuid() | Identificador único |
| `tenant_id` | UUID | NOT NULL, FK → tenants(id) | Aislamiento multi-tenant |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Timestamp del evento |
| `severity` | VARCHAR(20) | NOT NULL, CHECK enum | info / warning / critical / error |
| `event_type` | VARCHAR(50) | NOT NULL, CHECK enum | Tipo de evento (ver abajo) |
| `source_id` | UUID | NULLABLE | ID del dispositivo, usuario o proceso que originó el evento |
| `machine_id` | UUID | NULLABLE, FK → machines(id) | Máquina relacionada (si aplica) |
| `message` | TEXT | NOT NULL | Descripción legible del evento |
| `metadata` | JSONB | NULLABLE, DEFAULT '{}' | Datos adicionales del evento (sin estructura fija) |

**Enum `severity`**: `info`, `warning`, `critical`, `error`

**Enum `event_type`**:
- `alarm_triggered` — Regla de alarma disparada
- `alarm_resolved` — Alarma resuelta
- `device_connected` — Dispositivo conectado al sistema
- `device_disconnected` — Dispositivo desconectado
- `device_state_changed` — Cambio de estado en dispositivo
- `user_action` — Acción administrativa de usuario
- `system` — Evento interno del sistema

**Invariantes**:
- Los registros son inmutables: no hay UPDATE ni DELETE individual.
- La purga masiva por retención es la única forma de eliminación.
- `tenant_id` nunca es null.

---

### RetentionPolicy

Configuración de retención de logs por tenant. Un registro por tenant, upsert en primera configuración.

| Campo | Tipo | Constraints | Descripción |
|-------|------|-------------|-------------|
| `tenant_id` | UUID | PK, FK → tenants(id) | Un registro por tenant |
| `retention_days` | INT | NOT NULL, DEFAULT 90, CHECK > 0 | Días de retención |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Última modificación |
| `next_purge_at` | TIMESTAMPTZ | NOT NULL | Próxima ejecución de purga programada |

**Defaults**: `retention_days = 90`, `next_purge_at = NOW() + INTERVAL '1 day'`

---

## Esquema SQL

```sql
-- Migration: 000015_create_log_entries_table.up.sql

-- Función trigger reutilizada (si no existe)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Tabla principal de logs
CREATE TABLE log_entries (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    severity    VARCHAR(20) NOT NULL CHECK (severity IN ('info', 'warning', 'critical', 'error')),
    event_type  VARCHAR(50) NOT NULL CHECK (event_type IN (
                    'alarm_triggered', 'alarm_resolved',
                    'device_connected', 'device_disconnected', 'device_state_changed',
                    'user_action', 'system'
                )),
    source_id   UUID,
    machine_id  UUID,
    message     TEXT        NOT NULL,
    metadata    JSONB       NOT NULL DEFAULT '{}'
);

-- Tabla de retención por tenant
CREATE TABLE log_retention_policies (
    tenant_id       UUID        PRIMARY KEY,
    retention_days  INT         NOT NULL DEFAULT 90 CHECK (retention_days > 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_purge_at   TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '1 day'
);

CREATE TRIGGER update_log_retention_policies_updated_at
    BEFORE UPDATE ON log_retention_policies
    FOR EACH ROW EXECUTE PROCEDURE update_updated_at_column();

-- Índices de performance
-- Paginación keyset por tenant
CREATE INDEX idx_log_entries_tenant_cursor
    ON log_entries(tenant_id, created_at DESC, id DESC);

-- Filtro por machine_id
CREATE INDEX idx_log_entries_machine
    ON log_entries(tenant_id, machine_id)
    WHERE machine_id IS NOT NULL;

-- Filtro por severity
CREATE INDEX idx_log_entries_severity
    ON log_entries(tenant_id, severity, created_at DESC);

-- Filtro por event_type
CREATE INDEX idx_log_entries_event_type
    ON log_entries(tenant_id, event_type, created_at DESC);

-- Full-text search en message
CREATE INDEX idx_log_entries_message_fts
    ON log_entries USING gin(to_tsvector('spanish', message));
```

---

## Rollback SQL

```sql
-- Migration: 000015_create_log_entries_table.down.sql
DROP TABLE IF EXISTS log_retention_policies;
DROP TABLE IF EXISTS log_entries;
```

---

## Relaciones

```
tenants (1) ──── (N) log_entries
tenants (1) ──── (0..1) log_retention_policies
```

No se usa FK explícita hacia `tenants` para evitar overhead en inserciones de alto volumen; el aislamiento es responsabilidad del repositorio.

---

## Estructura de código

```
internal/
  domain/
    logs.go                          # LogEntry, RetentionPolicy, enums, errores
  platform/
    logwriter/
      writer.go                      # Interface LogWriter (write path interno)
  app/
    logs/
      service.go                     # ReadService: List, Get, GetContext, Export, GetRetention, UpdateRetention, Subscribe
  repo/pg/
    logs/
      repository.go                  # Interface + PostgreSQL impl
  telemetry/
    log_metrics.go                   # Contadores Prometheus
  api/
    handler/
      logs/
        dto/
          request.go                 # Parámetros de query + body
          response.go                # LogResponse, RetentionResponse, ExportResponse
        list_logs.go
        get_log.go
        get_log_context.go
        stream_logs.go
        export_logs.go
        get_retention.go
        update_retention.go
        routes.go
migrations/
  000015_create_log_entries_table.up.sql
  000015_create_log_entries_table.down.sql
```
