# Data Model: Alarm Rules (008)

## Entidad: AlarmRule

Representa una condición de alerta configurable para una métrica de un edge device del tenant.

### Campos

| Campo | Tipo | Nullable | Descripción |
|---|---|---|---|
| `id` | UUID | NO | Identificador único, generado al crear |
| `tenant_id` | UUID | NO | Tenant al que pertenece la regla (aislamiento multi-tenant) |
| `name` | string | NO | Nombre descriptivo de la regla |
| `description` | string | SÍ | Descripción opcional con más contexto |
| `metric` | string | NO | Nombre de la métrica monitoreada (ej: `temperature`, `pressure`, `bag_count`) |
| `operator` | string | NO | Operador de comparación: `gt`, `lt`, `gte`, `lte`, `eq` |
| `threshold` | decimal | NO | Valor umbral numérico contra el que se compara la métrica |
| `severity` | string | NO | Nivel de severidad: `info`, `warning`, `critical` |
| `enabled` | boolean | NO | Si la regla está activa. Default: `true` |
| `created_at` | timestamp | NO | Fecha de creación (UTC) |
| `updated_at` | timestamp | NO | Fecha de última modificación (UTC) |

### Restricciones de negocio

- `operator` debe ser uno de: `gt`, `lt`, `gte`, `lte`, `eq`
- `severity` debe ser uno de: `info`, `warning`, `critical`
- `name` no puede estar vacío
- `metric` no puede estar vacío
- `threshold` puede ser cualquier número (incluido negativo, para temperaturas bajo cero)
- El nombre **no necesita ser único** dentro del tenant (las reglas se identifican por UUID)
- La eliminación es permanente (no soft-delete)

### Relaciones

- `tenant_id` → referencia a `tenants.id` (FK)
- No tiene relación directa con `users` (las reglas son de tenant, no de usuario)
- Las notificaciones generadas por estas reglas pertenecen a `notification-service-api` (fuera de alcance)

## Schema SQL (migración 000014)

```sql
CREATE TABLE alarm_rules (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT,
    metric      TEXT        NOT NULL,
    operator    TEXT        NOT NULL CHECK (operator IN ('gt', 'lt', 'gte', 'lte', 'eq')),
    threshold   NUMERIC(15,4) NOT NULL,
    severity    TEXT        NOT NULL CHECK (severity IN ('info', 'warning', 'critical')),
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alarm_rules_tenant_id ON alarm_rules(tenant_id);
CREATE INDEX idx_alarm_rules_tenant_enabled ON alarm_rules(tenant_id, enabled);

-- Trigger para updated_at automático
CREATE OR REPLACE FUNCTION update_alarm_rules_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_alarm_rules_updated_at
    BEFORE UPDATE ON alarm_rules
    FOR EACH ROW EXECUTE FUNCTION update_alarm_rules_updated_at();
```

## Representación JSON (response)

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "tenantId": "550e8400-e29b-41d4-a716-446655440001",
  "name": "Temperatura alta",
  "description": "Alerta cuando la temperatura supera el umbral de seguridad",
  "metric": "temperature",
  "operator": "gt",
  "threshold": 80.0,
  "severity": "critical",
  "enabled": true,
  "createdAt": "2026-04-06T10:00:00Z",
  "updatedAt": "2026-04-06T10:00:00Z"
}
```

## Request body (crear)

```json
{
  "name": "Temperatura alta",
  "description": "Alerta cuando la temperatura supera el umbral de seguridad",
  "metric": "temperature",
  "operator": "gt",
  "threshold": 80.0,
  "severity": "critical",
  "enabled": true
}
```

## Request body (actualizar — PATCH, parcial)

```json
{
  "threshold": 85.0,
  "severity": "warning"
}
```
