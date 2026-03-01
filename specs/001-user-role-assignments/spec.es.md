# Especificación de Funcionalidad: Gestión de Asignación de Roles de Usuario

**Rama de funcionalidad**: `001-user-role-assignments`
**Creado**: 2026-02-27
**Estado**: Borrador
**Entrada**: Descripción del usuario: "quiero implementar los cambios que estuvimos relevando"

## Escenarios de Usuario y Pruebas *(obligatorio)*

<!--
  IMPORTANTE: Las historias de usuario deben estar PRIORIZADAS como recorridos de usuario ordenados por importancia.
  Cada historia/recorrido debe ser PROBABLE DE PRUEBA DE FORMA INDEPENDIENTE: es decir, si implementas solo UNA,
  igualmente deberías tener un MVP (Producto Mínimo Viable) que entregue valor.

  Asigna prioridades (P1, P2, P3, etc.) a cada historia, donde P1 es la más crítica.
  Piensa cada historia como un recorte funcional independiente que pueda:
  - Desarrollarse de forma independiente
  - Probarse de forma independiente
  - Desplegarse de forma independiente
  - Demostrarse a usuarios de forma independiente
-->

### Historia de Usuario 1 - Asignar un rol a un usuario (Prioridad: P1)

Como administrador, necesito asignar un rol específico a un usuario dentro del tenant de mi organización para que el usuario obtenga el nivel de acceso apropiado a la plataforma.

**Por qué esta prioridad**: La asignación de roles es la operación fundacional. Sin esto, no es posible gestionar roles. Todas las demás historias dependen primero de la existencia de asignaciones.

**Prueba Independiente**: Un administrador puede asignar un rol a un usuario y, de inmediato, el sistema refleja que el usuario posee ese rol en ese tenant.

**Escenarios de Aceptación**:

1. **Dado** que un administrador está autenticado y existe un usuario en el sistema sin rol en este tenant, **Cuando** el administrador asigna un rol (p. ej., "Operario") a ese usuario, **Entonces** el sistema confirma la asignación y el estado del usuario pasa inmediatamente a "activo".
2. **Dado** que un usuario ya tiene una asignación de rol activa en este tenant, **Cuando** un administrador intenta asignar otro rol al mismo usuario, **Entonces** el sistema rechaza la solicitud con un mensaje claro que indique que el usuario ya tiene un rol activo y que debe actualizarse en su lugar.
3. **Dado** que un administrador provee datos inválidos o incompletos (falta `user ID`, `tenant ID` o `role ID`), **Cuando** se intenta la asignación, **Entonces** el sistema la rechaza con un error de validación informativo.

---

### Historia de Usuario 2 - Ver asignaciones de roles para un tenant (Prioridad: P1)

Como administrador, necesito ver todas las asignaciones usuario-rol dentro del tenant de mi organización, opcionalmente filtradas por estado, para poder auditar y gestionar quién tiene acceso y con qué nivel.

**Por qué esta prioridad**: La visibilidad sobre las asignaciones existentes es crítica para supervisión y gestión. Los administradores no pueden gestionar roles eficazmente si no pueden listarlas.

**Prueba Independiente**: Un administrador puede recuperar la lista completa de asignaciones de roles para su tenant y puede filtrar la lista por estado (p. ej., solo pendientes, solo activas, solo revocadas).

**Escenarios de Aceptación**:

1. **Dado** que un tenant tiene múltiples asignaciones usuario-rol en distintos estados, **Cuando** un administrador solicita la lista completa, **Entonces** se devuelven todas las asignaciones con sus detalles completos (identificador de usuario, rol, estado, quién asignó y cuándo).
2. **Dado** que un tenant tiene una mezcla de asignaciones activas, pendientes y revocadas, **Cuando** el administrador filtra por estado "pendiente", **Entonces** se devuelven solo las asignaciones pendientes.
3. **Dado** que un tenant aún no tiene asignaciones, **Cuando** el administrador solicita la lista, **Entonces** se devuelve una lista vacía sin error.

---

### Historia de Usuario 3 - Revocar una asignación de rol (Prioridad: P2)

Como administrador, necesito revocar la asignación de rol de un usuario dentro de mi tenant para que pierda acceso cuando deja la organización o cambia de responsabilidades, preservando al mismo tiempo una traza de auditoría histórica.

**Por qué esta prioridad**: La revocación de acceso es crítica para la seguridad. Preservar el historial asegura responsabilidad y auditabilidad.

**Prueba Independiente**: Un administrador puede revocar una asignación existente y el sistema la marca como revocada (sin eliminarla), de modo que el historial de auditoría permanezca intacto.

**Escenarios de Aceptación**:

1. **Dado** que un usuario tiene una asignación de rol activa, **Cuando** un administrador la revoca, **Entonces** el estado de la asignación cambia a "revocada", se preservan los datos originales y el usuario deja de aparecer como asignado activamente.
2. **Dado** que un administrador intenta revocar una asignación inexistente, **Entonces** el sistema devuelve un error claro de "no encontrado".
3. **Dado** que una asignación ya fue revocada, **Cuando** el administrador intenta revocarla nuevamente, **Entonces** el sistema lo maneja de forma elegante, sin error ni corrupción de datos.

---

### Historia de Usuario 4 - Actualizar una asignación de rol (Prioridad: P2)

Como administrador, necesito cambiar el rol de un usuario dentro de mi tenant (p. ej., promover de Operario a Admin) sin necesidad de revocar y reasignar, para que las transiciones de rol sean fluidas y se preserve el registro de asignación.

**Por qué esta prioridad**: Las transiciones de rol son una necesidad operativa común. Forzar un flujo de revocación + reasignación rompería continuidad y generaría fricción innecesaria.

**Prueba Independiente**: Un administrador puede actualizar el rol en una asignación existente y el cambio se refleja inmediatamente sin crear registros duplicados.

**Escenarios de Aceptación**:

1. **Dado** que un usuario tiene una asignación activa con rol "Operario", **Cuando** el administrador cambia el rol a "Admin", **Entonces** se actualiza el mismo registro de asignación con el nuevo rol y la marca de tiempo de actualización.
2. **Dado** que un administrador intenta actualizar una asignación inexistente, **Entonces** el sistema devuelve un error claro de "no encontrado".

---

### Historia de Usuario 5 - Asignación masiva de roles a múltiples usuarios (Prioridad: P3)

Como administrador, necesito asignar el mismo rol a múltiples usuarios de una sola vez (p. ej., durante el onboarding de un equipo), para no tener que realizar asignaciones individuales por cada usuario.

**Por qué esta prioridad**: Las operaciones masivas mejoran la eficiencia administrativa en equipos grandes, pero no son críticas para la funcionalidad inicial; la asignación individual cubre el caso de uso principal.

**Prueba Independiente**: Un administrador puede seleccionar múltiples usuarios y un rol, enviar una asignación masiva, y todos los usuarios reciben ese rol en una sola operación. Si existe cualquier conflicto, toda la operación se rechaza de forma limpia.

**Escenarios de Aceptación**:

1. **Dado** que existen múltiples usuarios sin roles activos en un tenant, **Cuando** un administrador asigna masivamente un rol a todos ellos, **Entonces** se crean todas las asignaciones y la respuesta confirma el total de asignaciones exitosas.
2. **Dado** que uno de los usuarios seleccionados ya tiene un rol activo en el tenant, **Cuando** se intenta la asignación masiva, **Entonces** toda la operación se rechaza con un mensaje claro de conflicto y no se aplican cambios parciales.
3. **Dado** que se provee una lista vacía de usuarios, **Cuando** se intenta la asignación masiva, **Entonces** el sistema la rechaza con un error de validación.

---

### Historia de Usuario 6 - Ver los roles de un usuario en todos los tenants (Prioridad: P3)

Como administrador de plataforma (MRG Admin), necesito ver todos los roles que un usuario específico tiene en cada tenant del sistema, para tener una visión completa de sus accesos a nivel plataforma.

**Por qué esta prioridad**: La visibilidad entre tenants es una funcionalidad de administración avanzada para el rol MRG Admin. Los administradores estándar de tenant no la requieren; es una capacidad de gobernanza de plataforma.

**Prueba Independiente**: Un administrador de plataforma puede buscar cualquier usuario por su identificador y recibir una lista consolidada de todas sus asignaciones de rol en todos los tenants, incluyendo nombres de tenant y rol.

**Escenarios de Aceptación**:

1. **Dado** que un usuario tiene asignaciones activas en dos tenants distintos, **Cuando** un administrador de plataforma solicita los roles de ese usuario, **Entonces** se devuelven ambas asignaciones con nombre de tenant, nombre de rol y estado actual.
2. **Dado** que un usuario no tiene asignaciones en ningún tenant, **Cuando** se solicitan sus roles, **Entonces** se devuelve una lista vacía sin error.

---

### Casos límite

- ¿Qué ocurre cuando un `userId` o `tenantId` referencia una entidad que no existe en el sistema?
- ¿Cómo maneja el sistema solicitudes concurrentes de asignación para la misma combinación usuario+tenant (condición de carrera)?
- ¿Qué ocurre cuando la lista de asignación masiva contiene IDs de usuario duplicados?
- ¿Las asignaciones revocadas se incluyen en la respuesta de listado por defecto o solo cuando se filtran explícitamente?
- ¿Qué ocurre cuando un administrador intenta actualizar el rol de una asignación ya revocada?
- ¿Qué ocurre cuando el ID de rol provisto no corresponde a un rol válido en el sistema?

## Requisitos *(obligatorio)*

### Requisitos Funcionales

- **RF-001**: El sistema DEBE permitir que un administrador autorizado asigne exactamente un rol a un usuario dentro de un tenant dado; si el usuario ya tiene un rol activo en ese tenant, el sistema DEBE rechazar la solicitud con estado de conflicto.
- **RF-002**: El sistema DEBE permitir que un administrador autorizado liste todas las asignaciones usuario-rol de un tenant, devolviendo todos los detalles relevantes (identificador de usuario, rol, estado, quién asignó y marcas de tiempo).
- **RF-003**: El sistema DEBE permitir a los administradores filtrar la lista de asignaciones por estado (activo, pendiente o revocado).
- **RF-004**: El sistema DEBE permitir que un administrador autorizado actualice el rol de una asignación existente sin crear un nuevo registro.
- **RF-005**: El sistema DEBE permitir que un administrador autorizado revoque una asignación de rol; el registro de asignación DEBE preservarse con estado "revocado" (el registro nunca se elimina físicamente).
- **RF-006**: El sistema DEBE permitir que un administrador autorizado asigne el mismo rol a múltiples usuarios en una sola operación (asignación masiva); si cualquiera de los usuarios seleccionados ya tiene un rol activo en el tenant objetivo, toda la operación DEBE rechazarse sin aplicar cambios parciales.
- **RF-007**: El sistema DEBE permitir que un administrador de plataforma recupere todas las asignaciones de rol de un usuario específico en todos los tenants, incluyendo el nombre legible de cada tenant y rol.
- **RF-008**: Todas las operaciones de asignación de roles DEBEN registrar automáticamente la identidad del actor autenticado que ejecutó la acción.
- **RF-009**: Todas las operaciones de escritura DEBEN requerir autenticación; las solicitudes no autenticadas DEBEN rechazarse.
- **RF-010**: El sistema DEBE validar todas las entradas requeridas y devolver mensajes de error claros y descriptivos para cualquier dato inválido o faltante.

### Entidades Clave *(incluir si la funcionalidad involucra datos)*

- **Rol**: Un nivel de permisos nominal dentro de la plataforma (p. ej., Admin, Operario, Cliente Admin, Cliente Operario). Cada rol tiene un identificador único y un nombre legible. Los roles están predefinidos por la plataforma.
- **Asignación Usuario-Tenant-Rol (UTR)**: Representa la relación tripartita entre un usuario, un tenant y un rol. Rastrea el estado actual de la asignación (activo, pendiente o revocado), la identidad de quien realizó la asignación y cuándo la asignación fue creada, activada y modificada por última vez.

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **CE-001**: Un administrador puede completar una asignación individual de rol en menos de 30 segundos desde que inicia la solicitud hasta que recibe la confirmación.
- **CE-002**: La restricción de rol activo único se cumple en el 100% de los intentos de asignación; no pueden existir asignaciones activas duplicadas para la misma combinación usuario+tenant.
- **CE-003**: Las asignaciones revocadas siempre se preservan; la tasa de pérdida de datos por revocación es 0%.
- **CE-004**: Una asignación masiva de hasta 100 usuarios se completa en menos de 5 segundos en condiciones normales de operación.
- **CE-005**: Todas las solicitudes no autorizadas o no autenticadas se rechazan; 0% de accesos no protegidos tiene éxito.
- **CE-006**: Los filtros por estado devuelven exclusivamente resultados coincidentes; 100% de precisión de filtro sin contaminación entre estados.
- **CE-007**: El sistema devuelve un mensaje de error significativo en el 100% de las solicitudes inválidas o malformadas, sin fallos silenciosos ni errores no controlados.
