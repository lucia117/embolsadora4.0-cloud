# Especificación de Feature: API de Gestión de Roles

**Rama**: `006-roles-management`
**Fecha de creación**: 2026-04-03
**Estado**: Borrador
**Descripción original**: "Implementar API de Gestión de Roles: los tenants pueden crear, listar, actualizar y eliminar configuraciones de roles personalizados. Los roles del sistema (admin, operario, cliente_admin, cliente_operario) son de solo lectura y no pueden eliminarse. Los roles tienen nombre, descripción y una lista de permisos. Máximo 3 roles custom por tenant. No se puede eliminar un rol si tiene usuarios asignados."

## Escenarios de Usuario y Pruebas *(obligatorio)*

### Historia de Usuario 1 - Listar Roles Disponibles (Prioridad: P1)

Un administrador de tenant visualiza la lista completa de roles disponibles en su tenant — tanto los roles predefinidos del sistema como los roles personalizados que el tenant haya creado — para entender qué niveles de acceso existen antes de asignarlos a usuarios.

**Por qué esta prioridad**: La lista de roles es el punto de entrada para todas las decisiones relacionadas con roles. Sin visibilidad sobre qué roles existen, los administradores no pueden asignar los niveles de acceso correctos ni saber si hace falta crear un rol personalizado.

**Prueba independiente**: Se puede probar autenticándose como administrador y solicitando la lista de roles para el tenant, verificando que se devuelvan los roles del sistema y los roles personalizados existentes con sus nombres, descripciones y permisos.

**Escenarios de Aceptación**:

1. **Dado que** un tenant no tiene roles personalizados, **cuando** un administrador autenticado solicita la lista de roles, **entonces** el sistema devuelve los 4 roles del sistema (admin, operario, cliente_admin, cliente_operario) con sus metadatos.
2. **Dado que** un tenant tiene 2 roles personalizados además de los del sistema, **cuando** un administrador autenticado solicita la lista, **entonces** el sistema devuelve los 6 roles (4 del sistema + 2 personalizados).
3. **Dado que** la solicitud no está autenticada, **cuando** se pide la lista de roles, **entonces** el sistema devuelve 401 No autorizado.

---

### Historia de Usuario 2 - Crear un Rol Personalizado (Prioridad: P1)

Un administrador de tenant crea un nuevo rol personalizado con nombre, descripción opcional y lista de permisos para modelar un nivel de acceso específico a la estructura de su organización (por ejemplo, un rol "Supervisor" con acceso de solo lectura a dashboards y alertas).

**Por qué esta prioridad**: Los roles personalizados son el valor central de esta feature. Sin la posibilidad de crearlos, los tenants quedan limitados a los 4 roles predefinidos, que pueden no adaptarse a sus necesidades organizacionales.

**Prueba independiente**: Se puede probar enviando un nuevo rol con nombre único y lista de permisos, verificando que la respuesta incluya el rol creado con ID generado por el servidor y timestamps.

**Escenarios de Aceptación**:

1. **Dado que** un tenant tiene menos de 3 roles personalizados, **cuando** un administrador autenticado crea un rol llamado "Supervisor" con una lista de permisos, **entonces** el sistema devuelve 201 con el nuevo rol incluyendo ID generado por el servidor, nombre, descripción, permisos y timestamps.
2. **Dado que** un tenant ya tiene un rol personalizado llamado "Supervisor", **cuando** se envía un nuevo rol con ese mismo nombre, **entonces** el sistema devuelve 409 Conflicto con código de error `DUPLICATE_NAME`.
3. **Dado que** un tenant ya tiene 3 roles personalizados, **cuando** se intenta crear uno nuevo, **entonces** el sistema devuelve 403 Prohibido con código de error `LIMIT_REACHED`.
4. **Dado que** se envía una solicitud válida con lista de permisos vacía, **cuando** se crea el rol, **entonces** el sistema lo acepta y almacena el rol con una lista de permisos vacía.

---

### Historia de Usuario 3 - Ver un Rol Específico (Prioridad: P2)

Un administrador de tenant recupera el detalle completo de un rol por su ID para revisar su configuración antes de asignarlo a un usuario o decidir si actualizarlo.

**Por qué esta prioridad**: Permite inspeccionar un rol antes de asignarlo o modificarlo. Prioridad menor que listar/crear ya que es una operación de soporte.

**Prueba independiente**: Se puede probar solicitando un rol conocido por ID y verificando que se devuelva el perfil completo del rol.

**Escenarios de Aceptación**:

1. **Dado que** existe un rol con ID conocido, **cuando** un usuario autenticado lo solicita por ID, **entonces** el sistema devuelve 200 con el perfil completo: id, nombre, descripción, permisos, isSystemRole, isGlobal, tenantId, createdAt, updatedAt.
2. **Dado que** el ID del rol no existe, **cuando** se lo solicita, **entonces** el sistema devuelve 404 No encontrado con código de error `NOT_FOUND`.

---

### Historia de Usuario 4 - Actualizar un Rol Personalizado (Prioridad: P2)

Un administrador de tenant actualiza el nombre, descripción o lista de permisos de un rol personalizado existente para reflejar cambios en los requisitos de acceso de la organización.

**Por qué esta prioridad**: Permite refinar iterativamente los roles personalizados sin necesidad de eliminarlos y volver a crearlos.

**Prueba independiente**: Se puede probar enviando una actualización con nuevo nombre y/o permisos, verificando que la respuesta refleje los cambios con un timestamp updatedAt actualizado.

**Escenarios de Aceptación**:

1. **Dado que** existe un rol personalizado, **cuando** un administrador autenticado envía un nuevo nombre y/o permisos, **entonces** el sistema devuelve 200 con el rol actualizado y un timestamp updatedAt renovado.
2. **Dado que** un tenant tiene los roles A y B, **cuando** un administrador intenta renombrar el rol A con el nombre del rol B, **entonces** el sistema devuelve 409 Conflicto con código de error `DUPLICATE_NAME`.
3. **Dado que** se intenta actualizar un rol del sistema, **cuando** se envía la actualización, **entonces** el sistema devuelve 403 Prohibido con código de error `SYSTEM_ROLE`.
4. **Dado que** el ID del rol no existe, **cuando** se envía la actualización, **entonces** el sistema devuelve 404 No encontrado.

---

### Historia de Usuario 5 - Eliminar un Rol Personalizado (Prioridad: P2)

Un administrador de tenant elimina un rol personalizado que ya no es necesario para mantener el catálogo de roles limpio y liberar espacio bajo el límite de 3 roles.

**Por qué esta prioridad**: La eliminación es necesaria para gestionar el límite de 3 roles y quitar configuraciones obsoletas. Incluye controles de seguridad para evitar romper asignaciones activas de usuarios.

**Prueba independiente**: Se puede probar eliminando un rol personalizado que no tenga usuarios asignados activos y verificando que desaparece de la lista. Intentar eliminar un rol del sistema o un rol con usuarios asignados debe ser rechazado.

**Escenarios de Aceptación**:

1. **Dado que** existe un rol personalizado sin asignaciones de usuarios activas, **cuando** un administrador autenticado lo elimina por ID, **entonces** el sistema devuelve 200 y el rol ya no aparece en la lista.
2. **Dado que** un rol personalizado tiene una o más asignaciones de usuarios activas, **cuando** se intenta eliminarlo, **entonces** el sistema devuelve 409 Conflicto con código de error `ROLE_HAS_ASSIGNMENTS`, incluyendo la cantidad de usuarios afectados.
3. **Dado que** se intenta eliminar un rol del sistema (por ejemplo, "admin"), **cuando** se envía la solicitud, **entonces** el sistema devuelve 403 Prohibido con código de error `SYSTEM_ROLE`.
4. **Dado que** el ID del rol no existe, **cuando** se intenta eliminarlo, **entonces** el sistema devuelve 404 No encontrado.

---

### Casos Límite

- ¿Qué ocurre si un string de permiso en la lista está vacío o contiene solo espacios? → El sistema rechaza la solicitud con 400 Bad Request.
- ¿Qué ocurre si el nombre del rol supera la longitud máxima? → El sistema rechaza con 400 Bad Request.
- ¿Qué ocurre si el mismo permiso aparece múltiples veces en la lista? → El sistema deduplica los permisos antes de guardar.
- ¿Qué ocurre si un administrador intenta actualizar un rol de otro tenant? → El sistema devuelve 404 (no visible entre tenants).

## Requisitos *(obligatorio)*

### Requisitos Funcionales

- **RF-001**: El sistema DEBE requerir un token JWT Bearer válido en el header Authorization para cada endpoint; las solicitudes sin token válido DEBEN devolver 401.
- **RF-002**: El sistema DEBE delimitar todas las operaciones de roles al tenant identificado por el header `X-Tenant-ID`, garantizando aislamiento estricto de datos entre tenants.
- **RF-003**: El sistema DEBE devolver tanto los roles del sistema como los roles personalizados del tenant cuando se solicita la lista de roles.
- **RF-004**: El sistema DEBE limitar a un máximo de 3 roles personalizados por tenant; los intentos de creación cuando se alcanza el límite DEBEN devolver 403 con código de error `LIMIT_REACHED`.
- **RF-005**: El sistema DEBE garantizar unicidad de nombre por tenant para roles personalizados; la creación o actualización con nombre duplicado DEBE devolver 409 con código de error `DUPLICATE_NAME`.
- **RF-006**: El sistema DEBE asignar un ID generado por el servidor y timestamps (createdAt, updatedAt) a cada rol personalizado en el momento de su creación.
- **RF-007**: El sistema DEBE impedir la eliminación de roles del sistema y DEBE devolver 403 con código de error `SYSTEM_ROLE` cuando se intente.
- **RF-008**: El sistema DEBE impedir la eliminación de cualquier rol con asignaciones de usuarios activas y DEBE devolver 409 con código de error `ROLE_HAS_ASSIGNMENTS`, incluyendo la cantidad de usuarios afectados.
- **RF-009**: El sistema DEBE impedir la actualización de roles del sistema y DEBE devolver 403 con código de error `SYSTEM_ROLE` cuando se intente.
- **RF-010**: El sistema DEBE permitir actualizar el nombre, descripción y permisos de un rol personalizado; updatedAt DEBE renovarse en cada actualización exitosa.
- **RF-011**: El sistema DEBE almacenar los permisos como lista de strings; la lista completa se reemplaza en cada actualización.
- **RF-012**: El sistema DEBE deduplicar las entradas de permisos antes de persistir un rol.
- **RF-013**: El sistema DEBE devolver `{ success: true, data: ... }` en respuestas exitosas y `{ success: false, error: "..." }` en todas las respuestas de error.

### Entidades Clave

- **Rol**: Configuración de acceso con nombre. Atributos clave:
  - ID único (generado por el servidor para roles personalizados; string fijo para roles del sistema)
  - Nombre (modificable para roles personalizados, inmutable para roles del sistema)
  - Descripción (opcional, modificable)
  - Permisos (lista de strings, modificable)
  - Indicador de sistema (señala si es un rol predefinido, inmutable)
  - Indicador global (señala si está disponible en todos los tenants)
  - Tenant de pertenencia (nulo para roles globales/del sistema, establecido para roles personalizados)
  - Timestamps de creación y última modificación

- **Permiso**: Identificador string que representa una acción específica que otorga el rol (por ejemplo, `machines:read`, `users:write`). Los permisos son strings opacos; el servidor los almacena y devuelve sin validar su semántica.

- **Rol del Sistema**: Uno de los 4 roles predefinidos (admin, operario, cliente_admin, cliente_operario). Solo lectura, no se pueden modificar ni eliminar, visibles para todos los tenants.

## Criterios de Éxito *(obligatorio)*

### Resultados Medibles

- **CE-001**: Un administrador puede ver la lista completa de roles disponibles para su tenant en menos de 300ms.
- **CE-002**: Un nuevo rol personalizado puede crearse en menos de 500ms; las violaciones de límite y unicidad de nombre devuelven su respuesta de error en menos de 200ms.
- **CE-003**: Un rol individual puede recuperarse por ID en menos de 200ms.
- **CE-004**: Una actualización de rol completa en menos de 500ms y el updatedAt renovado es inmediatamente visible en consultas posteriores.
- **CE-005**: Una eliminación de rol completa en menos de 300ms; las respuestas de rechazo (rol del sistema, asignaciones activas) devuelven en menos de 200ms.
- **CE-006**: El sistema aísla correctamente los datos de roles personalizados por tenant — ninguna operación puede leer o modificar roles de un tenant diferente.
- **CE-007**: El 100% de las solicitudes sin token válido reciben 401 No autorizado.
- **CE-008**: El límite de 3 roles personalizados se aplica sin falsos positivos — los roles existentes nunca son bloqueados para actualización o lectura por el límite.
- **CE-009**: La unicidad de nombre se aplica por tenant — el mismo nombre de rol puede coexistir en tenants diferentes sin conflicto.
- **CE-010**: Los roles del sistema siempre son visibles y nunca eliminables ni modificables, sin importar quién realice la solicitud.

## Supuestos

- Los permisos son strings opacos (por ejemplo, `machines:read`); la API los almacena y devuelve tal cual sin validar contra un catálogo de permisos conocido.
- El límite de 3 roles personalizados aplica por tenant y no cuenta los roles del sistema.
- Los roles del sistema son globalmente visibles para todos los tenants y están pre-cargados; no cuentan contra el límite de roles personalizados del tenant.
- La unicidad de nombre aplica solo a roles personalizados dentro del mismo tenant; el mismo nombre puede coexistir en tenants diferentes.
- La autenticación la maneja Supabase JWT (consistente con la superficie de API existente).
- Todos los usuarios autenticados dentro de un tenant pueden leer roles; solo los administradores pueden crear, actualizar o eliminar roles personalizados.
- Los IDs de roles personalizados son generados por el servidor. Los IDs de roles del sistema son slugs string fijos.
