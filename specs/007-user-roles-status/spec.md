# Especificación de Feature: Extensión de Gestión de Usuarios

**Feature Branch**: `007-user-roles-status`
**Creado**: 2026-04-03
**Estado**: Draft
**Input**: Extend User Management API: add GET /users/:id?include=roles to return user with their assigned roles, add PATCH /users/:id/status to change user status (active/inactive/suspended), and add GET /users/pending to list users pending activation. Max 1 role per user per tenant.

---

## Escenarios de Usuario y Testing

### Historia de Usuario 1 — Ver usuario con sus roles asignados (Prioridad: P1)

Un administrador necesita ver el detalle completo de un usuario incluyendo el rol que tiene asignado en el tenant. Hoy el endpoint de detalle de usuario devuelve los datos básicos pero no incluye información sobre su rol, lo que obliga al frontend a hacer una segunda consulta.

**Por qué esta prioridad**: Es la base del ABM de usuarios — sin ver el rol asignado, la pantalla de detalle de usuario está incompleta. Satisface el contrato Pact `user-service-api-roles-extension`.

**Test independiente**: Puede verificarse solicitando el detalle de un usuario con el parámetro `include=roles` y comprobando que el response incluye el campo `roles` con el rol activo del usuario en ese tenant.

**Escenarios de aceptación**:

1. **Dado** un usuario con rol activo en el tenant, **Cuando** se solicita su detalle con `include=roles`, **Entonces** el response incluye los datos del usuario más un array `roles` con el rol asignado (id, nombre, permisos).
2. **Dado** un usuario solicitado sin el parámetro `include=roles`, **Cuando** se hace la consulta, **Entonces** el response es igual que antes sin el campo `roles` (compatibilidad hacia atrás).
3. **Dado** un ID de usuario inexistente, **Cuando** se solicita su detalle, **Entonces** retorna error de no encontrado.
4. **Dado** un usuario de otro tenant, **Cuando** se intenta acceder, **Entonces** retorna error de no encontrado (aislamiento multi-tenant).
5. **Dado** un usuario sin rol asignado en el tenant, **Cuando** se solicita con `include=roles`, **Entonces** retorna `roles: []`.

---

### Historia de Usuario 2 — Cambiar estado de un usuario (Prioridad: P2)

Un administrador necesita poder activar, desactivar o suspender usuarios del tenant sin eliminarlos. Esto permite gestionar el acceso sin perder el historial del usuario.

**Por qué esta prioridad**: Es necesario para el ciclo de vida completo de usuarios — un operario que deja de trabajar puede desactivarse sin borrar su historial.

**Test independiente**: Puede verificarse cambiando el estado de un usuario y comprobando que el cambio se refleja en el response y que el sistema respeta el nuevo estado.

**Escenarios de aceptación**:

1. **Dado** un usuario activo, **Cuando** un admin cambia su estado a `inactive`, **Entonces** el usuario queda inactivo y retorna 200 con el usuario actualizado.
2. **Dado** un usuario inactivo, **Cuando** un admin lo reactiva con estado `active`, **Entonces** el usuario puede volver a operar normalmente.
3. **Dado** un estado inválido en el request, **Cuando** se intenta el cambio, **Entonces** retorna error de validación 400.
4. **Dado** un usuario de otro tenant, **Cuando** se intenta cambiar su estado, **Entonces** retorna error de no encontrado.
5. **Dado** un usuario sin permiso de administrador, **Cuando** intenta cambiar el estado de otro usuario, **Entonces** retorna error de autorización 403.

---

### Historia de Usuario 3 — Listar usuarios pendientes de activación (Prioridad: P2)

Un administrador necesita ver qué usuarios fueron invitados pero todavía no completaron su activación. Esto permite hacer seguimiento de invitaciones enviadas.

**Por qué esta prioridad**: Permite al admin identificar qué invitados no activaron su cuenta y decidir si reenviar la invitación o revocarla.

**Test independiente**: Puede verificarse consultando la lista de usuarios pendientes y comprobando que devuelve solo usuarios que no completaron su activación en el tenant.

**Escenarios de aceptación**:

1. **Dado** un tenant con usuarios pendientes, **Cuando** un admin consulta la lista de pendientes, **Entonces** retorna los usuarios que no completaron su activación.
2. **Dado** un tenant sin usuarios pendientes, **Cuando** se consulta la lista, **Entonces** retorna lista vacía con 200.
3. **Dado** un usuario sin permiso de administrador, **Cuando** intenta acceder a la lista, **Entonces** retorna error de autorización 403.

---

### Casos borde

- ¿Qué pasa si un usuario tiene múltiples entradas históricas en sus asignaciones de rol? Solo se incluye el rol con estado activo (máximo 1 por tenant).
- ¿Puede un admin desactivarse a sí mismo? No — el sistema debe impedirlo para evitar quedarse sin acceso.
- ¿Qué pasa si se solicita `include=roles` para un usuario eliminado (soft delete)? Retorna error de no encontrado.

---

## Requisitos

### Requisitos Funcionales

- **RF-001**: El sistema DEBE permitir obtener el detalle de un usuario incluyendo sus roles asignados cuando se solicita con el parámetro `include=roles`.
- **RF-002**: El sistema DEBE mantener compatibilidad hacia atrás: el endpoint de detalle de usuario sin parámetros retorna el mismo response que antes.
- **RF-003**: El sistema DEBE permitir a un administrador cambiar el estado de un usuario a `active`, `inactive` o `suspended`.
- **RF-004**: Solo usuarios con permiso de administración DEBEN poder cambiar el estado de otros usuarios.
- **RF-005**: El sistema DEBE validar que el usuario pertenece al tenant del administrador antes de permitir el cambio de estado.
- **RF-006**: El sistema DEBE impedir que un administrador se desactive a sí mismo.
- **RF-007**: El sistema DEBE retornar la lista de usuarios con estado pendiente de activación para el tenant.
- **RF-008**: Solo administradores DEBEN poder listar usuarios pendientes.
- **RF-009**: Un usuario DEBE tener como máximo 1 rol activo por tenant.
- **RF-010**: El campo `roles` en el response DEBE incluir id, nombre y permisos del rol asignado.

### Entidades Clave

- **Usuario**: Persona con cuenta en el sistema. Tiene estado (`active`, `inactive`, `suspended`, `pending`), email, nombre y pertenece a uno o más tenants a través de asignaciones de rol.
- **Asignación de Rol**: Relación entre usuario, tenant y rol. Un usuario tiene máximo 1 asignación activa por tenant.
- **Rol**: Conjunto de permisos. Puede ser del sistema o personalizado del tenant. Tiene id, nombre y lista de permisos.

---

## Criterios de Éxito

### Resultados Medibles

- **CE-001**: Un administrador puede ver el rol de un usuario en 1 sola consulta (actualmente requiere 2: detalle + roles).
- **CE-002**: Los 4 contratos Pact de `user-service-api-roles-extension` quedan satisfechos al 100%.
- **CE-003**: Un administrador puede cambiar el estado de un usuario en menos de 30 segundos desde la pantalla de detalle.
- **CE-004**: La lista de usuarios pendientes permite identificar invitaciones sin activar en 1 sola consulta sin filtros adicionales.
- **CE-005**: Ningún cambio rompe el comportamiento existente de los 5 endpoints de usuarios ya implementados.

---

## Suposiciones

- El estado de un usuario se almacena en la tabla `users` como campo `status` (ya existe en el dominio actual).
- Los usuarios "pendientes" son aquellos cuya asignación en `user_tenant_roles` tiene `status = 'pending'` para el tenant consultado.
- El parámetro `include=roles` es opcional y no afecta el comportamiento por defecto del endpoint.
- La autenticación y autorización siguen el mismo patrón que el resto de la API.

---

## Dependencias

- `001-user-role-assignments`: tabla de asignaciones de rol ya existe con campo `status`.
- `002-user-management`: handlers base de usuarios ya implementados (5 endpoints CRUD).
- `006-roles-management`: tabla `roles` extendida con permisos, disponible para el join.
