# Research: Notification Service API (010)

**Feature**: Notification Service API  
**Date**: 2026-04-10  
**Status**: Completo — sin incógnitas abiertas

---

## Decisión 1: Patrón de arquitectura

**Decisión**: Seguir el patrón hexagonal establecido por features anteriores (006-roles, 007-user-roles-status, 008-alarm-rules).

**Rationale**: Consistencia con el codebase existente. El patrón `migration → domain → repo/pg → app/service → handler+dto → wiring en url_mappings.go` está probado y bien comprendido por el equipo. No hay razón para desviarse.

**Alternativas consideradas**:
- Patrón usecase (como tenants/user_roles): descartado porque las features recientes usan el patrón `app/service` más simple y directo.

---

## Decisión 2: Número de migración

**Decisión**: `000016_create_notifications_table`

**Rationale**: La última migración aplicada es `000015_create_log_entries_table`. La siguiente disponible es 000016.

---

## Decisión 3: Almacenamiento de estado de notificaciones

**Decisión**: Columnas `status VARCHAR(20)`, `acknowledged_at TIMESTAMPTZ NULL`, `closed_at TIMESTAMPTZ NULL` en la tabla `notifications`.

**Rationale**: El estado es un enum simple (unread/acknowledged/closed) con timestamps de transición. No justifica una tabla de historial de estados en MVP. Facilita queries eficientes por `status` e índice compuesto `(tenant_id, status, created_at DESC)`.

**Alternativas consideradas**:
- Tabla de historial de transiciones: excesivo para MVP, puede agregarse en features futuras si se necesita auditoría de cambios de estado.

---

## Decisión 4: RBAC para operaciones de estado (ack/close)

**Decisión**: Cualquier usuario autenticado del tenant puede hacer ack y close (sin RBAC adicional, igual que GET).

**Rationale**: Las notificaciones son un recurso del operador, no de administración. El operador que trabaja con la máquina debe poder marcar sus propias alertas sin necesitar permisos de administrador. Este es el mismo patrón que el log-service (ninguna operación de lectura/query requiere RBAC adicional).

**Alternativas consideradas**:
- Solo admin puede cerrar notificaciones: descartado, sería una restricción operativa innecesaria.

---

## Decisión 5: Idempotencia de ack/close

**Decisión**: Implementar con expresiones `CASE WHEN` en el SQL del UPDATE para aplicar transiciones de estado condicionalmente sin romper el estado actual si ya está en un estado terminal.

**Rationale**: Evita race conditions y simplifica el repositorio al eliminar la necesidad de SELECT previo. Las reglas de transición:
- `ack`: `unread → acknowledged`. Si ya está `acknowledged` o `closed`, el UPDATE no modifica el estado ni el timestamp (CASE no aplica el SET).
- `close`: `any → closed`. El `closed_at` solo se escribe si el estado actual NO es `closed` (CASE protege el timestamp original).

**Implementación en repo**:
```sql
-- Ack (idempotente: solo transiciona desde 'unread')
UPDATE notifications
SET status          = CASE WHEN status = 'unread' THEN 'acknowledged' ELSE status END,
    acknowledged_at = CASE WHEN status = 'unread' THEN NOW() ELSE acknowledged_at END
WHERE id = $1 AND tenant_id = $2

-- Close (idempotente: closed_at preserva el timestamp original)
UPDATE notifications
SET status    = 'closed',
    closed_at = CASE WHEN status != 'closed' THEN NOW() ELSE closed_at END
WHERE id = $1 AND tenant_id = $2
```

**Alternativas consideradas**:
- `UPDATE ... WHERE status = 'unread'` (solo transiciona si está en estado correcto): descartado porque con `0 rows affected` no podríamos distinguir entre "no existe" y "ya estaba acknowledged".
- Verificar estado antes de UPDATE con SELECT: introduce race condition sin beneficio real.

---

## Decisión 6: Creación de notificaciones (fuera de alcance)

**Decisión**: No incluir endpoint `POST /notifications` en esta feature.

**Rationale**: Los 6 Pacts del frontend no incluyen creación. Las notificaciones serán generadas por un worker/trigger interno cuando se dispare una alarma (feature futura). Para las pruebas de esta feature se usará seed de datos directamente en la BD.

---

## Decisión 7: Paginación

**Decisión**: Paginación por `limit`/`offset` con valores por defecto limit=20, máximo=100.

**Rationale**: Las notificaciones son un feed acotado (diferente a logs que pueden ser millones). Offset es más simple de implementar y suficiente para el volumen esperado. La paginación por cursor se reserva para colecciones de alto volumen (como logs).

---

## Decisión 8: Filtrado

**Decisión**: Query params opcionales: `status` (unread/acknowledged/closed) y `severity` (info/warning/critical/error).

**Rationale**: Son los filtros más comunes en el UI de un panel de notificaciones: "mostrar solo las no leídas" o "mostrar solo las críticas". Más filtros pueden agregarse en iteraciones futuras.

---

## Dependencias verificadas

| Dependencia | Estado | Notas |
|---|---|---|
| `008-alarm-rules` — `GET /alarm-rules` | ✅ Implementado | Satisface Pact #6 sin código adicional |
| `internal/domain/alarm_rules.go` | ✅ Existe | Referencia para pattern de domain |
| `internal/app/alarm_rules/service.go` | ✅ Existe | Referencia para pattern de service |
| `internal/api/handler/alarm_rules/` | ✅ Existe | Referencia para pattern de handler |
| `internal/routes/url_mappings.go` | ✅ Existe | Punto de wiring para nueva feature |
| Migración 000016 | ✅ Disponible | 000015 es la última |
| `google/uuid` | ✅ En go.mod | Sin nueva dependencia necesaria |
