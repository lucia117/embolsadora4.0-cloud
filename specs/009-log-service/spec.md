# Feature Specification: Log Service API

**Feature Branch**: `009-log-service`  
**Created**: 2026-04-07  
**Status**: Draft  
**Input**: API de logs de eventos operacionales para la plataforma embolsadora. 14 interacciones Pact definidas en PACTS_ANALYSIS.md.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consultar historial de eventos (Priority: P1)

Un operador o administrador necesita ver el historial de eventos del sistema para diagnosticar problemas, auditar acciones y monitorear el estado de la plataforma. Puede filtrar por tipo de evento, severidad, máquina y rango de fechas, y también buscar por texto libre dentro de los mensajes.

**Why this priority**: Es el núcleo del servicio. Sin consulta de logs, el resto de funcionalidades no tiene valor. Habilita la observabilidad activa del sistema.

**Independent Test**: Se puede testear completamente ejecutando `GET /api/v1/logs` con distintas combinaciones de filtros y verificando que los resultados son correctos, paginados y restringidos al tenant del solicitante.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado con tenant válido, **When** consulta `GET /api/v1/logs` sin filtros, **Then** recibe la página más reciente de logs del tenant ordenados por timestamp descendente con cursor de paginación.
2. **Given** un usuario autenticado, **When** consulta con filtros `event_type`, `severity`, `machine_id` y rango de fechas (`from`/`to`), **Then** recibe solo los logs que cumplen todos los criterios activos.
3. **Given** un usuario autenticado, **When** consulta con parámetro `q` (texto libre), **Then** recibe logs cuyo mensaje o metadata contiene el texto buscado.
4. **Given** un usuario autenticado, **When** consulta con `cursor` obtenido de una respuesta anterior, **Then** recibe la siguiente página de resultados sin duplicados ni saltos.
5. **Given** un usuario autenticado con tenant sin logs, **When** consulta `GET /api/v1/logs`, **Then** recibe respuesta exitosa con lista vacía (`data: []`).
6. **Given** un usuario sin token, **When** consulta `GET /api/v1/logs`, **Then** recibe `401 UNAUTHORIZED`.

---

### User Story 2 - Ver detalle y contexto de un evento (Priority: P2)

Un operador necesita inspeccionar un evento específico y ver los eventos que ocurrieron antes y después de él para entender la secuencia que llevó a un problema.

**Why this priority**: Complementa la consulta de listado. El contexto temporal de un evento es fundamental para diagnóstico de fallas.

**Independent Test**: Se puede testear obteniendo un `id` de log del listado y haciendo `GET /api/v1/logs/{id}` seguido de `GET /api/v1/logs/{id}/context`. Entrega valor inmediato para diagnóstico.

**Acceptance Scenarios**:

1. **Given** un log existente del tenant, **When** solicita `GET /api/v1/logs/{id}`, **Then** recibe todos los campos del evento (id, timestamp, severity, event_type, source, message, metadata, machine_id).
2. **Given** un id inexistente o de otro tenant, **When** solicita `GET /api/v1/logs/{id}`, **Then** recibe `404 NOT_FOUND`.
3. **Given** un log existente, **When** solicita `GET /api/v1/logs/{id}/context`, **Then** recibe una ventana de N eventos anteriores y N eventos posteriores alrededor del log, ordenados cronológicamente.

---

### User Story 3 - Exportar logs (Priority: P3)

Un administrador necesita exportar el historial de logs filtrado para análisis offline, reportes o auditorías externas.

**Why this priority**: Necesario para auditoría y compliance, pero no bloquea las operaciones diarias.

**Independent Test**: Se puede testear ejecutando `GET /api/v1/logs/export` con filtros y verificando que se descarga un archivo con el contenido correcto.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado, **When** solicita `GET /api/v1/logs/export` con filtros aplicados, **Then** recibe un archivo descargable (CSV o JSON) con los logs que coinciden.
2. **Given** que la exportación supera el límite máximo de registros, **When** solicita `GET /api/v1/logs/export`, **Then** recibe respuesta con los primeros N registros e indicador de truncamiento (`truncated: true`, `total_available: X`).

---

### User Story 4 - Streaming de eventos en tiempo real (Priority: P4)

Un operador quiere ver los nuevos eventos del sistema en tiempo real sin necesidad de recargar la página, para monitoreo continuo de la operación.

**Why this priority**: Mejora significativamente la experiencia de monitoreo en tiempo real pero requiere infraestructura SSE; puede deferirse al no ser bloqueante para las otras funcionalidades.

**Independent Test**: Se puede testear abriendo `GET /api/v1/logs/stream` y verificando que cada nuevo evento insertado llega como mensaje SSE al cliente conectado.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado, **When** conecta a `GET /api/v1/logs/stream`, **Then** recibe una conexión SSE que permanece abierta y envía cada nuevo evento del tenant en formato `data: {...}`.
2. **Given** una conexión SSE activa, **When** no hay nuevos eventos por 30 segundos, **Then** el servidor envía un heartbeat para mantener la conexión viva.

---

### User Story 5 - Gestionar política de retención (Priority: P5)

Un administrador necesita configurar cuántos días se conservan los logs del tenant para controlar el almacenamiento y cumplir con políticas de retención de datos.

**Why this priority**: Operacionalmente importante a largo plazo pero no urgente para el MVP; los logs pueden acumularse con la retención por defecto inicialmente.

**Independent Test**: Se puede testear consultando `GET /api/v1/logs/retention` y luego actualizando con `PATCH /api/v1/logs/retention` y verificando el cambio.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado, **When** consulta `GET /api/v1/logs/retention`, **Then** recibe la política de retención actual del tenant (días configurados, fecha de próxima purga).
2. **Given** un administrador autenticado, **When** envía `PATCH /api/v1/logs/retention` con un valor de días válido, **Then** la política se actualiza y los logs más antiguos que el nuevo límite serán eliminados en la próxima purga.
3. **Given** un usuario sin permiso de administración, **When** intenta `PATCH /api/v1/logs/retention`, **Then** recibe `403 FORBIDDEN`.

---

### Edge Cases

- ¿Qué pasa si se filtra con un `machine_id` que no pertenece al tenant? → Respuesta vacía (sin error, aislamiento multi-tenant).
- ¿Qué pasa si `from` > `to` en el filtro de fechas? → `400 BAD_REQUEST` con mensaje descriptivo.
- ¿Qué pasa si el cursor es inválido o expirado? → `400 BAD_REQUEST`.
- ¿Qué pasa si `GET /api/v1/logs/{id}/context` se llama sobre un log que es el primero del tenant? → Retorna solo los N eventos posteriores (sin previos).
- ¿Qué pasa si se exportan 0 resultados? → Archivo vacío con cabeceras.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir consultar logs filtrados por `event_type`, `severity`, `machine_id`, rango de fechas (`from`/`to`) y búsqueda de texto libre (`q`), con cualquier combinación de filtros.
- **FR-002**: El sistema DEBE implementar paginación por cursor (no offset) para listas de logs, garantizando consistencia ante inserciones concurrentes.
- **FR-003**: El sistema DEBE aislar los logs por tenant — ningún usuario puede ver logs de otro tenant.
- **FR-004**: El sistema DEBE permitir obtener el detalle completo de un log por ID.
- **FR-005**: El sistema DEBE permitir obtener una ventana de logs contiguos alrededor de un evento dado (contexto temporal).
- **FR-006**: El sistema DEBE soportar exportación de logs filtrados en formato descargable, con truncamiento y aviso cuando se supera el límite máximo.
- **FR-007**: El sistema DEBE exponer un endpoint SSE para streaming de nuevos eventos en tiempo real, restringido por tenant.
- **FR-008**: El sistema DEBE permitir consultar y actualizar la política de retención de logs por tenant.
- **FR-009**: El sistema DEBE requerir autenticación JWT válida en todos los endpoints.
- **FR-010**: La operación de actualización de retención DEBE requerir permiso de administrador.

### Key Entities

- **LogEntry**: Representa un evento del sistema. Atributos clave: `id` (UUID), `tenant_id`, `timestamp`, `severity` (info/warning/critical/error), `event_type` (alarm_triggered/device_state_changed/user_action/system), `source_id` (UUID del dispositivo o usuario que originó el evento), `machine_id` (nullable), `message` (texto), `metadata` (datos adicionales del evento).
- **RetentionPolicy**: Configuración de retención por tenant. Atributos: `tenant_id`, `retention_days` (default: 90), `next_purge_at`.

---

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un operador puede consultar el historial de eventos con cualquier combinación de filtros y recibir resultados en menos de 2 segundos para volúmenes de hasta 1 millón de logs por tenant.
- **SC-002**: La paginación por cursor garantiza que no aparecen duplicados ni saltos al navegar entre páginas, incluso con inserciones concurrentes.
- **SC-003**: El 100% de los 14 contratos Pact del `log-service-api` son satisfechos por la implementación.
- **SC-004**: Los logs de un tenant son completamente invisibles para usuarios de otros tenants (aislamiento verificable).
- **SC-005**: La exportación de logs con hasta 10.000 registros se completa en menos de 10 segundos.
- **SC-006**: El streaming SSE entrega nuevos eventos al cliente conectado en menos de 1 segundo desde su ingesta.

---

## Assumptions

- Los logs son generados por otros servicios/workers y almacenados en la BD; este servicio es de **consulta** (lectura + configuración de retención), no de ingesta.
- El formato de exportación por defecto es JSON; CSV como alternativa si el Pact lo especifica.
- La paginación por cursor usa `created_at + id` como clave compuesta para garantizar orden estable.
- El tamaño de ventana para `/context` es de 10 eventos antes y 10 después (configurable por parámetro `window_size`).
- El límite máximo de exportación es 50.000 registros por request.
- La retención por defecto es 90 días si no se ha configurado explícitamente.
- El SSE usa el header `Accept: text/event-stream` estándar.
- Los tipos de evento (`event_type`) son un enum cerrado definido en dominio; valores iniciales: `alarm_triggered`, `alarm_resolved`, `device_state_changed`, `device_connected`, `device_disconnected`, `user_action`, `system`.
