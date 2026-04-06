# Feature Specification: Alarm Rules Service API

**Feature Branch**: `008-alarm-rules`  
**Created**: 2026-04-06  
**Status**: Draft  
**Input**: Alarm Rules Service API: CRUD de reglas de alarma para monitoreo industrial. 10 interacciones Pact pendientes: GET/POST/PATCH/DELETE /api/alarm-rules con auth, validación, 404 handling.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consultar reglas de alarma del tenant (Priority: P1)

Un administrador o técnico quiere ver todas las reglas de alarma configuradas para su tenant, para entender qué condiciones están siendo monitoreadas en la máquina.

**Why this priority**: Sin poder listar reglas, el usuario no puede auditar ni gestionar el sistema de alertas. Es el punto de entrada de toda la funcionalidad.

**Independent Test**: Listar reglas devuelve la colección actual (vacía o con datos) sin requerir ninguna regla creada previamente. Entrega valor inmediato como punto de auditoría.

**Acceptance Scenarios**:

1. **Given** un usuario autenticado con tenant activo, **When** solicita la lista de reglas de alarma, **Then** el sistema devuelve todas las reglas del tenant con sus detalles, o una lista vacía si no hay ninguna.
2. **Given** un usuario sin autenticación, **When** solicita la lista, **Then** el sistema rechaza la solicitud con error 401.
3. **Given** un usuario autenticado, **When** solicita la lista, **Then** solo ve reglas de su propio tenant (aislamiento multi-tenant).

---

### User Story 2 - Crear una regla de alarma (Priority: P1)

Un administrador quiere definir una nueva condición de alerta (ej: temperatura > 80°C durante más de 5 minutos) para recibir notificaciones cuando la máquina opere fuera de parámetros seguros.

**Why this priority**: La creación de reglas es el núcleo de la funcionalidad — sin reglas no hay monitoreo activo.

**Independent Test**: Crear una regla y verificar que aparece en el listado. Entrega valor como configuración básica de monitoreo.

**Acceptance Scenarios**:

1. **Given** un administrador autenticado con datos válidos, **When** crea una regla de alarma, **Then** la regla queda registrada y disponible para consulta con código 201.
2. **Given** un administrador con datos incompletos o inválidos, **When** intenta crear una regla, **Then** el sistema rechaza con 400 indicando qué campo falló.
3. **Given** un usuario no autenticado, **When** intenta crear una regla, **Then** el sistema rechaza con 401.

---

### User Story 3 - Obtener el detalle de una regla específica (Priority: P2)

Un técnico quiere consultar el detalle completo de una regla particular para revisar su configuración antes de modificarla.

**Why this priority**: Necesario para flujos de edición y auditoría, pero el listado cubre la mayoría de los casos de consulta.

**Independent Test**: Crear una regla y luego obtenerla por ID; el detalle debe coincidir exactamente con lo ingresado.

**Acceptance Scenarios**:

1. **Given** una regla existente, **When** se solicita por su ID, **Then** el sistema devuelve el detalle completo de esa regla con código 200.
2. **Given** un ID inexistente, **When** se solicita la regla, **Then** el sistema devuelve 404 con mensaje descriptivo.
3. **Given** un ID de regla perteneciente a otro tenant, **When** se solicita, **Then** el sistema devuelve 404 (aislamiento multi-tenant).

---

### User Story 4 - Modificar una regla de alarma existente (Priority: P2)

Un administrador quiere ajustar los parámetros de una regla ya creada (ej: cambiar el umbral de temperatura) sin tener que eliminarla y recrearla.

**Why this priority**: Las condiciones operativas cambian con el tiempo; la edición in-place evita pérdida de historial.

**Independent Test**: Crear una regla, modificar un campo, y verificar que el valor actualizado es el correcto al consultar.

**Acceptance Scenarios**:

1. **Given** una regla existente y datos válidos, **When** se actualiza parcialmente, **Then** solo los campos enviados cambian; el resto permanece igual.
2. **Given** datos inválidos en la actualización, **When** se intenta modificar, **Then** el sistema rechaza con 400.
3. **Given** un ID inexistente, **When** se intenta actualizar, **Then** el sistema devuelve 404.

---

### User Story 5 - Eliminar una regla de alarma (Priority: P3)

Un administrador quiere eliminar una regla que ya no aplica para evitar falsas alarmas o reducir el ruido del sistema.

**Why this priority**: Operación destructiva; el sistema funciona sin ella, pero es necesaria para el ciclo de vida completo de la configuración.

**Independent Test**: Crear una regla, eliminarla, y verificar que ya no aparece en el listado ni puede obtenerse por ID (404).

**Acceptance Scenarios**:

1. **Given** una regla existente, **When** se elimina, **Then** la regla ya no es accesible ni aparece en el listado, con código 200.
2. **Given** un ID inexistente, **When** se intenta eliminar, **Then** el sistema devuelve 404.
3. **Given** un usuario no autenticado, **When** intenta eliminar una regla, **Then** el sistema devuelve 401.

---

### Edge Cases

- ¿Qué ocurre si se envía un cuerpo vacío en la creación? → 400 con detalle de campos requeridos.
- ¿Qué ocurre si el ID no es un UUID válido? → 400 antes de consultar la base de datos.
- ¿Qué ocurre si un usuario intenta acceder a una regla de otro tenant? → 404 (no revelar existencia del recurso).
- ¿Qué ocurre si se modifican campos no editables (ej: ID, tenantId)? → esos campos son ignorados silenciosamente.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir listar todas las reglas de alarma del tenant del usuario autenticado.
- **FR-002**: El sistema DEBE permitir crear una nueva regla de alarma con nombre, descripción, métrica monitoreada, operador de comparación, umbral numérico y severidad.
- **FR-003**: El sistema DEBE permitir obtener el detalle de una regla por su identificador único.
- **FR-004**: El sistema DEBE permitir modificar parcialmente los campos de una regla existente.
- **FR-005**: El sistema DEBE permitir eliminar una regla de alarma existente.
- **FR-006**: El sistema DEBE rechazar todas las operaciones sin autenticación válida con código 401.
- **FR-007**: El sistema DEBE devolver código 404 al acceder, modificar o eliminar una regla con ID inexistente o de otro tenant.
- **FR-008**: El sistema DEBE rechazar creaciones y modificaciones con datos inválidos con código 400, indicando el campo que falló.
- **FR-009**: El sistema DEBE garantizar aislamiento multi-tenant: un usuario solo puede ver y operar reglas de su propio tenant.
- **FR-010**: El sistema DEBE registrar la fecha de creación y última modificación de cada regla.

### Key Entities

- **AlarmRule**: Representa una condición de alerta configurable. Atributos: identificador único, nombre descriptivo, descripción opcional, métrica monitoreada (ej: `temperature`, `pressure`, `bag_count`), operador de comparación (`gt`, `lt`, `gte`, `lte`, `eq`), valor umbral numérico, severidad (`info`, `warning`, `critical`), estado habilitado/deshabilitado, tenant al que pertenece, fecha de creación, fecha de última modificación.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un administrador puede crear, consultar, modificar y eliminar reglas de alarma completando cada operación en una sola interacción con el sistema.
- **SC-002**: El 100% de las operaciones sobre reglas de otro tenant son rechazadas con 404, garantizando el aislamiento de datos entre tenants.
- **SC-003**: El 100% de las solicitudes sin autenticación válida son rechazadas con 401 sin exponer datos del tenant.
- **SC-004**: El 100% de los 10 contratos Pact de `alarm-rules-service-api` son satisfechos por la implementación.
- **SC-005**: Todas las respuestas de error incluyen un código de error estable y un mensaje descriptivo, permitiendo al frontend mostrar retroalimentación útil al usuario.

## Assumptions

- Las reglas de alarma son configuración, no eventos; no generan notificaciones directamente (eso corresponde a `notification-service-api`, feature futura).
- La severidad tiene tres niveles predefinidos: `info`, `warning`, `critical`.
- Los operadores de comparación aceptados son: `gt`, `lt`, `gte`, `lte`, `eq`.
- El listado no requiere paginación en el MVP (los tenants tienen pocas reglas configuradas).
- La autenticación sigue el esquema JWT Bearer existente (Supabase Auth + X-Tenant-ID header).
- No se implementa soft-delete en el MVP; la eliminación es permanente.
- El campo `metric` hace referencia a métricas publicadas por los edge devices del tenant.
- El nombre de la regla no necesita ser único dentro del tenant (las reglas se identifican por ID).
