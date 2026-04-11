# Feature Specification: Permissions Management API

**Feature Branch**: `011-permissions-management`  
**Created**: 2026-04-10  
**Status**: Draft  
**Input**: User description: "Permissions Management API — CRUD de permisos con soporte para permisos de sistema (inmutables) y permisos custom (creables por el admin del tenant), expuesto via REST para que el frontend pueda listar, crear, editar y eliminar permisos al configurar roles"

## User Scenarios & Testing *(mandatory)*

### User Story 1 — Listar el catálogo de permisos disponibles (Priority: P1)

Un administrador accede a la pantalla de configuración de roles en el frontend. Necesita ver qué permisos existen para poder asignarlos a un rol. El sistema devuelve la lista completa de permisos disponibles para ese tenant: tanto los permisos de sistema predefinidos por el producto como los permisos custom que el propio tenant haya creado anteriormente.

**Why this priority**: Es el punto de entrada de toda la funcionalidad. Sin esta lista, el frontend no puede mostrar nada en la pantalla de gestión de roles. Además es el endpoint más usado: se consulta cada vez que se abre el panel de roles.

**Independent Test**: Se puede probar enviando un `GET /api/v1/permissions` con un JWT válido. El sistema debe devolver al menos los permisos de sistema predefinidos aunque no exista ningún permiso custom creado aún.

**Acceptance Scenarios**:

1. **Dado** que existen permisos de sistema cargados y el usuario tiene un JWT válido, **cuando** hace `GET /api/v1/permissions`, **entonces** el sistema responde 200 con un array que incluye todos los permisos de sistema con su `id`, `name`, `section`, `description` e `isSystemPermission: true`.
2. **Dado** que el tenant tiene además permisos custom creados, **cuando** hace `GET /api/v1/permissions`, **entonces** la respuesta incluye tanto los permisos de sistema como los custom, diferenciados por `isSystemPermission`.
3. **Dado** que el usuario no incluye un token de autenticación, **cuando** hace `GET /api/v1/permissions`, **entonces** el sistema responde 401 con mensaje de error.

---

### User Story 2 — Crear un permiso custom (Priority: P2)

Un administrador identifica que su empresa necesita un permiso específico que no existe en el catálogo de sistema (por ejemplo, "Exportar datos de producción"). Puede crear un permiso custom con nombre, sección y descripción. Este permiso luego puede asignarse a los roles del tenant.

**Why this priority**: Habilita la extensibilidad del RBAC sin necesidad de cambios en el código. Es el segundo endpoint más crítico para el flujo de configuración de roles.

**Independent Test**: Luego de crear un permiso custom via `POST /api/v1/permissions`, se puede verificar que aparece en el listado y puede recuperarse por ID.

**Acceptance Scenarios**:

1. **Dado** que el admin envía nombre válido (≥ 3 caracteres), sección y descripción, **cuando** hace `POST /api/v1/permissions`, **entonces** el sistema responde 201 con el permiso creado incluyendo su `id` generado, `isSystemPermission: false`, `createdAt` y `updatedAt`.
2. **Dado** que el admin envía un nombre con menos de 3 caracteres, **cuando** hace `POST /api/v1/permissions`, **entonces** el sistema responde 400 con un error de validación indicando el campo `name` y el motivo.
3. **Dado** que el admin envía `section` vacío o ausente, **cuando** hace `POST /api/v1/permissions`, **entonces** el sistema responde 400 con error de validación indicando el campo faltante.

---

### User Story 3 — Consultar un permiso por ID (Priority: P2)

El frontend necesita mostrar el detalle de un permiso específico al editar un rol o al mostrar los permisos asignados. El usuario puede obtener la información completa de cualquier permiso (de sistema o custom) mediante su identificador.

**Why this priority**: Complementa el listado para los flujos de edición y visualización de detalle.

**Independent Test**: Se puede probar con `GET /api/v1/permissions/{id}` usando un ID de permiso de sistema conocido (ej: `perm_dashboard`) y verificar que devuelve los datos correctos.

**Acceptance Scenarios**:

1. **Dado** que el permiso con ese ID existe, **cuando** hace `GET /api/v1/permissions/{id}`, **entonces** el sistema responde 200 con todos los campos del permiso.
2. **Dado** que el ID no corresponde a ningún permiso, **cuando** hace `GET /api/v1/permissions/{id}`, **entonces** el sistema responde 404 con mensaje de error.

---

### User Story 4 — Actualizar un permiso custom (Priority: P3)

Un administrador quiere corregir el nombre o descripción de un permiso custom que creó anteriormente. Puede modificar el nombre, sección y descripción de permisos custom. Los permisos de sistema no pueden ser modificados.

**Why this priority**: Necesario para el ciclo de vida completo de los permisos custom, pero menos urgente que crear o listar.

**Independent Test**: Se puede probar creando un permiso custom y luego actualizándolo via `PUT /api/v1/permissions/{id}`. La respuesta debe reflejar los nuevos valores y un `updatedAt` más reciente.

**Acceptance Scenarios**:

1. **Dado** que el permiso es custom y los datos son válidos, **cuando** hace `PUT /api/v1/permissions/{id}`, **entonces** el sistema responde 200 con los datos actualizados y el `updatedAt` renovado.
2. **Dado** que el permiso es de sistema (`isSystemPermission: true`), **cuando** intenta modificarlo via `PUT /api/v1/permissions/{id}`, **entonces** el sistema responde 403 con mensaje "Cannot modify system permissions".
3. **Dado** que el ID no existe, **cuando** hace `PUT /api/v1/permissions/{id}`, **entonces** el sistema responde 404.

---

### User Story 5 — Eliminar un permiso custom (Priority: P3)

Un administrador quiere eliminar un permiso custom que ya no es necesario. El sistema permite eliminar solo permisos custom. Los permisos de sistema están protegidos contra eliminación.

**Why this priority**: Cierra el ciclo CRUD de permisos custom. Los permisos de sistema nunca pueden eliminarse.

**Independent Test**: Se puede probar creando un permiso custom y luego eliminándolo via `DELETE /api/v1/permissions/{id}`. Verificar que ya no aparece en el listado.

**Acceptance Scenarios**:

1. **Dado** que el permiso es custom, **cuando** hace `DELETE /api/v1/permissions/{id}`, **entonces** el sistema responde 200 con `{ "success": true }` y el permiso desaparece del listado.
2. **Dado** que el permiso es de sistema, **cuando** intenta eliminarlo via `DELETE /api/v1/permissions/{id}`, **entonces** el sistema responde 403 con mensaje "Cannot delete system permissions".
3. **Dado** que el ID no existe, **cuando** hace `DELETE /api/v1/permissions/{id}`, **entonces** el sistema responde 404.

---

### Edge Cases

- ¿Qué pasa si se intenta crear un permiso custom con el mismo nombre que uno ya existente en el tenant? Los nombres duplicados son permitidos dado que el identificador único es el ID generado; el sistema no impone unicidad por nombre.
- ¿Qué pasa si se elimina un permiso custom que está asignado a uno o más roles? El sistema no verifica dependencias en esta fase; la eliminación procede. La consistencia entre permisos y roles es responsabilidad del servicio de roles.
- ¿Qué pasa si el body de `PUT` está vacío? Si todos los campos editables están ausentes, se devuelve 400.
- ¿Qué ocurre si los permisos de sistema no están cargados en la base de datos? El sistema debe garantizar que los permisos de sistema se cargan via seed/migración al inicializar. Son de solo lectura para la API.
- ¿Qué pasa si se envía un ID de permiso de sistema en `DELETE` o `PUT`? El sistema responde 403 independientemente de si el permiso existe o no en la BD.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE exponer `GET /api/v1/permissions` que retorne la lista completa de permisos (sistema + custom del tenant) para usuarios autenticados.
- **FR-002**: El sistema DEBE diferenciar permisos de sistema (`isSystemPermission: true`) de permisos custom (`isSystemPermission: false`) en todas las respuestas.
- **FR-003**: El sistema DEBE permitir crear permisos custom mediante `POST /api/v1/permissions`, validando que `name` tenga al menos 3 caracteres y que `section` y `description` estén presentes y no vacíos.
- **FR-004**: El sistema DEBE rechazar la modificación de permisos de sistema (`PUT` sobre un permiso con `isSystemPermission: true`) con HTTP 403.
- **FR-005**: El sistema DEBE rechazar la eliminación de permisos de sistema (`DELETE` sobre un permiso con `isSystemPermission: true`) con HTTP 403.
- **FR-006**: El sistema DEBE permitir actualizar nombre, sección y descripción de permisos custom existentes mediante `PUT /api/v1/permissions/{id}`.
- **FR-007**: El sistema DEBE eliminar permisos custom de forma permanente (no soft-delete) al invocar `DELETE /api/v1/permissions/{id}`.
- **FR-008**: El sistema DEBE retornar 401 para cualquier endpoint accedido sin token de autenticación válido.
- **FR-009**: El sistema DEBE retornar 404 cuando se consulta, modifica o elimina un permiso con ID inexistente.
- **FR-010**: Los permisos de sistema deben estar disponibles desde el inicio del sistema, cargados via migración de base de datos (seed inmutable).
- **FR-011**: Los IDs de permisos custom deben ser únicos y generados automáticamente por el sistema al crear.
- **FR-012**: Los permisos custom DEBEN estar aislados por tenant: un tenant no puede ver ni modificar los permisos custom de otro tenant. Los permisos de sistema son globales y visibles para todos los tenants.
- **FR-013**: Solo usuarios con rol admin pueden crear, modificar y eliminar permisos custom. Cualquier usuario autenticado puede consultar el catálogo.

### Key Entities

- **Permission**: Representa un permiso del sistema de control de acceso. Tiene un identificador único, nombre legible, sección funcional a la que pertenece (dashboard, logs, reports, maintenance, analytics, users, tenants, settings, all-tenants), descripción, e indicador que distingue si es de sistema o custom. Los permisos custom tienen además timestamps de creación y actualización.
- **SystemPermission**: Subconjunto inmutable de Permission, definido por el producto. No puede ser creado, modificado ni eliminado a través de la API. Se carga via seed en la base de datos durante la inicialización del sistema. Es compartido entre todos los tenants.
- **CustomPermission**: Subconjunto mutable de Permission, creado por administradores de un tenant específico para necesidades de acceso no cubiertas por los permisos de sistema. Pertenece exclusivamente al tenant que lo creó.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un administrador puede consultar el catálogo completo de permisos en menos de 500ms desde que envía la solicitud.
- **SC-002**: El sistema protege el 100% de los permisos de sistema contra modificación o eliminación accidental, devolviendo siempre 403 al intentarlo.
- **SC-003**: El listado de permisos devuelve al menos los 17 permisos de sistema predefinidos en cualquier estado del sistema.
- **SC-004**: Las 10 interacciones del contrato Pact `permissions-service-api.json` pasan exitosamente contra el backend implementado.
- **SC-005**: Todos los endpoints requieren autenticación; el 100% de las solicitudes sin JWT válido son rechazadas con 401.
- **SC-006**: Los permisos custom de un tenant no son visibles para otros tenants en ninguna circunstancia.

## Assumptions

- Los permisos de sistema son los 17 definidos en el contrato Pact: `perm_dashboard`, `perm_alerts`, `perm_reports`, `perm_users`, `perm_tenants`, `perm_settings`, `perm_maintenance`, `perm_analytics`, `perm_all_tenants`, `perm_logs_view`, `perm_logs_export`, `perm_logs_admin`, `perm_edge_devices_view`, `perm_edge_devices_manage`, `perm_edge_devices_check`, `perm_reports_view`, `perm_reports_manage`.
- El aislamiento multi-tenant aplica solo a permisos custom; los permisos de sistema son globales.
- No se implementa verificación de dependencias al eliminar un permiso custom asignado a roles: esto queda como responsabilidad del servicio de roles en una iteración posterior.
- Los nombres de permisos custom dentro de un mismo tenant pueden repetirse; no hay restricción de unicidad por nombre.
- La sección (`section`) es un campo de texto libre, no un enum cerrado, para permitir extensibilidad.
- El contrato Pact usa `/api/permissions` como prefijo; el backend expone `/api/v1/permissions`, consistente con el resto de la API. El frontend debe tener configurado el basePath correcto.
