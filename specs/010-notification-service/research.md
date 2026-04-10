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

**Decisión**: Implementar con `UPDATE ... SET status = $1, ... WHERE id = $2 AND tenant_id = $3` sin condición de estado previo. Retornar la notificación en su estado actual siempre que exista.

**Rationale**: Simplifica la lógica del repositorio. La idempotencia queda garantizada porque siempre se aplica el mismo `SET status = 'acknowledged'` independientemente del estado actual. Si la notificación está `closed` y se hace ack, el UPDATE no cambia nada (PostgreSQL aplica el SET pero el valor ya es lo que había), y se retorna el estado actual.

**Alternativas consideradas**:
- `UPDATE ... WHERE status = 'unread'` (solo transiciona si está en estado correcto): descartado porque rompe idempotencia — fallaría si se llama dos veces.
- Verificar estado antes de UPDATE con SELECT: introduce race condition sin beneficio real.

**Nota de implementación**: Para close, el estado `closed` no puede revertirse. Para ack de una notificación `closed`, se hace UPDATE y se retorna el estado actual (closed), ya que el SET `status = 'acknowledged'` no modifica una notificación que ya está `closed` — pero en realidad sí la modificaría. Por lo tanto, la lógica correcta es: si el estado actual ya es `closed`, no aplicar el UPDATE del ack (solo retornar). Si es `acknowledged`, el ack es no-op. Ver implementación en service.

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
