# Feature Specification: Notification Service API

**Feature Branch**: `010-notification-service`  
**Created**: 2026-04-10  
**Status**: Draft  
**Input**: API de notificaciones para la plataforma embolsadora. 6 interacciones Pact definidas en PACTS_ANALYSIS.md: GET /notifications (listar), GET /notifications/count (conteo sin leer), GET /notifications/{id} (detalle), POST /notifications/{id}/ack (acknowledger), POST /notifications/{id}/close (cerrar), GET /alarm-rules (listar reglas en contexto). Depende de alarm-rules ya implementado en 008.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consultar notificaciones del tenant (Priority: P1)

Un operador o administrador quiere ver el listado de notificaciones generadas por el sistema (p. ej., alarmas disparadas) para estar al tanto de eventos que requieren su atención. Puede ver cuántas tiene sin leer y navegar el historial.

**Why this priority**: Es el núcleo del servicio. Sin poder listar notificaciones y conocer el conteo de no leídas, el panel de alertas del dashboard no tiene funcionalidad. Habilita la observabilidad activa del operador.

**Independent Test**: Se puede testear ejecutando `GET /api/v1/notifications` con un usuario autenticado y verificando que se retornan las notificaciones del tenant con paginación, y `GET /api/v1/notifications/count` para obtener el conteo de no leídas.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado con tenant válido, **When** consulta `GET /api/v1/notifications`, **Then** recibe la lista paginada de notificaciones del tenant ordenadas por fecha descendente, con campos: id, título, mensaje, severidad, estado (unread/acknowledged/closed), referencia a regla y máquina, y timestamps.
2. **Given** un usuario autenticado, **When** consulta `GET /api/v1/notifications/count`, **Then** recibe el número de notificaciones con estado `unread` del tenant.
3. **Given** un usuario autenticado con tenant sin notificaciones, **When** consulta `GET /api/v1/notifications`, **Then** recibe respuesta exitosa con lista vacía (`data: []`, `total: 0`).
4. **Given** un usuario sin token, **When** consulta `GET /api/v1/notifications`, **Then** recibe `401 UNAUTHORIZED`.
5. **Given** un usuario autenticado, **When** consulta notificaciones, **Then** solo ve las de su propio tenant (aislamiento multi-tenant).

---

### User Story 2 - Ver detalle de una notificación (Priority: P2)

Un operador quiere inspeccionar el detalle completo de una notificación específica para entender qué evento la originó, qué máquina está involucrada y cuál fue la condición que disparó la alarma.

**Why this priority**: Complementa la consulta de listado. El detalle provee el contexto necesario para decidir si escalar o resolver el problema.

**Independent Test**: Se puede testear obteniendo un `id` del listado y haciendo `GET /api/v1/notifications/{id}`. Si no existe o es de otro tenant, retorna 404.

**Acceptance Scenarios**:

1. **Given** una notificación existente del tenant, **When** solicita `GET /api/v1/notifications/{id}`, **Then** recibe todos los campos del evento (id, title, message, severity, status, alarm_rule_id, machine_id, created_at, acknowledged_at, closed_at).
2. **Given** un id inexistente o de otro tenant, **When** solicita `GET /api/v1/notifications/{id}`, **Then** recibe `404 NOT_FOUND`.

---

### User Story 3 - Acusar recibo de una notificación (Priority: P3)

Un operador quiere marcar una notificación como "vista" (acknowledged) para indicar que tomó conocimiento del evento. Esto permite al equipo distinguir alertas nuevas de las ya revisadas.

**Why this priority**: Fundamental para el flujo de trabajo operativo. Permite reducir el ruido visual en el panel diferenciando notificaciones nuevas de las ya atendidas.

**Independent Test**: Ejecutar `POST /api/v1/notifications/{id}/ack` y verificar que el estado cambia de `unread` a `acknowledged` y que el conteo de no leídas se decrementa.

**Acceptance Scenarios**:

1. **Given** una notificación con estado `unread`, **When** un usuario autenticado hace `POST /api/v1/notifications/{id}/ack`, **Then** el estado cambia a `acknowledged`, se registra `acknowledged_at` y se retorna la notificación actualizada.
2. **Given** una notificación ya `acknowledged` o `closed`, **When** se intenta hacer ack nuevamente, **Then** el sistema responde con éxito de forma idempotente (sin error, retorna el estado actual sin cambios).
3. **Given** un id inexistente o de otro tenant, **When** se intenta hacer ack, **Then** recibe `404 NOT_FOUND`.

---

### User Story 4 - Cerrar una notificación (Priority: P4)

Un operador quiere cerrar una notificación para indicar que el incidente fue resuelto y ya no requiere acción. Las notificaciones cerradas quedan en el historial pero no contribuyen al conteo de alertas activas.

**Why this priority**: Permite limpiar el panel de notificaciones activas una vez que los incidentes están resueltos, mejorando la gestión operativa.

**Independent Test**: Ejecutar `POST /api/v1/notifications/{id}/close` y verificar que el estado cambia a `closed` y la notificación deja de contabilizarse como no leída.

**Acceptance Scenarios**:

1. **Given** una notificación con estado `unread` o `acknowledged`, **When** un usuario autenticado hace `POST /api/v1/notifications/{id}/close`, **Then** el estado cambia a `closed`, se registra `closed_at` y se retorna la notificación actualizada.
2. **Given** una notificación ya `closed`, **When** se intenta cerrar nuevamente, **Then** el sistema responde con éxito de forma idempotente.
3. **Given** un id inexistente o de otro tenant, **When** se intenta cerrar, **Then** recibe `404 NOT_FOUND`.

---

### Edge Cases

- ¿Qué pasa si se hace ack de una notificación `closed`? → Respuesta exitosa idempotente, estado no cambia.
- ¿Qué pasa si se hace close de una notificación `unread`? → Se cierra directamente sin requerir ack previo.
- ¿Qué pasa si el tenant no tiene notificaciones? → `GET /count` retorna `{"unread": 0}`.
- ¿Qué pasa si `alarm_rule_id` referenciada fue eliminada? → La notificación se retorna igual; el campo permanece como referencia histórica.
- ¿Qué pasa si se pagina con offset mayor al total? → Lista vacía sin error.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir listar las notificaciones del tenant autenticado, paginadas y ordenadas por fecha descendente.
- **FR-002**: El sistema DEBE retornar el conteo de notificaciones no leídas (`unread`) del tenant autenticado.
- **FR-003**: El sistema DEBE permitir obtener el detalle completo de una notificación por ID, restringido al tenant del solicitante.
- **FR-004**: El sistema DEBE permitir marcar una notificación como `acknowledged`, registrando el timestamp del acuse de recibo.
- **FR-005**: El sistema DEBE permitir cerrar una notificación (`closed`), registrando el timestamp de cierre.
- **FR-006**: Las operaciones de ack y close DEBEN ser idempotentes: aplicarlas a una notificación ya en ese estado retorna éxito sin modificar datos.
- **FR-007**: El sistema DEBE aislar notificaciones por tenant — ningún usuario puede ver ni modificar notificaciones de otro tenant.
- **FR-008**: El sistema DEBE requerir autenticación JWT válida en todos los endpoints.
- **FR-009**: El endpoint `GET /api/v1/alarm-rules` ya implementado en feature 008 satisface el Pact correspondiente sin nueva implementación; debe verificarse en validación.

### Key Entities

- **Notification**: Representa un evento que requiere atención del operador. Atributos clave: `id` (UUID), `tenant_id`, `title` (texto breve del evento), `message` (descripción detallada), `severity` (info/warning/critical/error), `status` (unread/acknowledged/closed), `alarm_rule_id` (UUID nullable — regla que disparó la notificación), `machine_id` (UUID nullable — máquina involucrada), `created_at`, `acknowledged_at` (nullable), `closed_at` (nullable).

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un operador puede consultar el listado de notificaciones y el conteo de no leídas en menos de 1 segundo para volúmenes de hasta 10.000 notificaciones por tenant.
- **SC-002**: El 100% de los 6 contratos Pact del `notification-service-api` son satisfechos por la implementación.
- **SC-003**: Las notificaciones de un tenant son completamente invisibles para usuarios de otros tenants (aislamiento verificable).
- **SC-004**: Las operaciones de ack y close son idempotentes: aplicarlas múltiples veces produce el mismo resultado sin errores.
- **SC-005**: El panel de notificaciones del frontend puede listar, ver detalle, obtener conteo de no leídas y realizar ack/close sin errores de integración.

---

## Assumptions

- Las notificaciones son generadas automáticamente por el sistema cuando una alarma se dispara (fuera del alcance de esta feature). Este servicio es de consulta y gestión de estado, no de creación.
- La creación de notificaciones será responsabilidad de un worker/trigger interno futuro; para validación se sembrará datos directamente en la BD.
- `GET /api/v1/alarm-rules` ya está implementado en `008-alarm-rules` y satisface ese Pact sin código adicional.
- La paginación usa `limit` y `offset` (no cursor), ya que el volumen de notificaciones es acotado y no requiere consistencia ante inserciones concurrentes al nivel de los logs.
- El filtrado soporta como mínimo: `status` (unread/acknowledged/closed) y `severity`.
- El tamaño de página por defecto es 20 notificaciones, máximo 100.
- No existe un endpoint de creación de notificaciones en este Pact; si se requiere en el futuro, se agrega en una feature separada.
- Las transiciones de estado siguen el flujo unidireccional: `unread → acknowledged → closed` (no se puede "reabrir" una notificación cerrada).
