# Feature Specification: POST /users con Asignación de Rol Inicial

**Feature Branch**: `develop`  
**Created**: 2026-04-11  
**Status**: Aprobado  
**Input**: Completar la interacción Pact pendiente `user-service-api-roles-extension` — POST /api/v1/users con rol inicial activo

---

## Escenarios de Usuario y Testing

### Historia de Usuario 1 — Crear usuario con rol asignado desde el inicio (Prioridad: P1)

Un administrador necesita crear un usuario nuevo en el tenant y asignarle un rol en una sola operación. Hoy el endpoint crea el usuario pero no crea la asignación en `user_tenant_roles`, dejando al usuario sin acceso operativo hasta que se haga una segunda llamada a `POST /user-roles`.

**Por qué esta prioridad**: Es el único Pact pendiente de `user-service-api-roles-extension`. Sin esto, el frontend no puede completar el flujo de alta de usuario.

**Test independiente**: Puede verificarse haciendo `POST /api/v1/users` con un `role` válido y comprobando que el usuario creado tiene una entrada activa en `user_tenant_roles` con `status = 'active'`.

**Escenarios de aceptación**:

1. **Dado** un admin autenticado con un tenant válido, **Cuando** hace `POST /api/v1/users` con `firstName`, `lastName`, `email`, `role` válido, **Entonces** retorna 201 con el usuario creado y queda registrado un UTR activo.
2. **Dado** un `role` que no existe en la tabla `roles`, **Cuando** se crea el usuario, **Entonces** retorna 400 con código `INVALID_ROLE`.
3. **Dado** un email ya usado en el tenant, **Cuando** se intenta crear otro usuario con el mismo email, **Entonces** retorna 409 con código `EMAIL_TAKEN`.
4. **Dado** que la creación del usuario falla a mitad de la transacción, **Cuando** el INSERT en `user_tenant_roles` falla, **Entonces** el usuario tampoco queda creado (rollback).
5. **Dado** un request sin token JWT, **Cuando** se intenta crear el usuario, **Entonces** retorna 401 `UNAUTHORIZED`.

---

### Casos Borde

- ¿Qué pasa si el `role` tiene formato UUID y pertenece a otro tenant? La FK lo rechaza → 400 INVALID_ROLE.
- ¿Puede pasarse un `role` de sistema (`"admin"`) junto con un `role` custom de UUID? Sí — la validación es por FK, no por tipo.
- ¿El usuario ya existente en `users` (otro tenant) puede recibir el mismo `POST`? El email es único por tenant (constraint compuesto), no globalmente. Si el email ya existe en ese tenant → 409. Si es otro tenant → 201 con un nuevo UTR.

---

## Requisitos

### Requisitos Funcionales

- **RF-001**: El sistema DEBE crear el usuario y su UTR activo en una única transacción atómica.
- **RF-002**: El UTR creado DEBE tener `status = 'active'` y `assigned_at = NOW()`.
- **RF-003**: El campo `role` DEBE aceptar cualquier `roles.id` válido (roles del sistema o custom del tenant).
- **RF-004**: Si el `role` no existe en la tabla `roles`, el sistema DEBE retornar 400 con código `INVALID_ROLE`.
- **RF-005**: El `assigned_by` del UTR DEBE ser el UUID del admin autenticado (extraído del JWT).
- **RF-006**: El response DEBE ser idéntico al actual `UserResponse` (backward compatible — sin campo `roles`).
- **RF-007**: Si falla cualquier parte de la transacción, NINGÚN registro debe persistir (atomicidad total).

### Entidades Clave

- **User**: Registro en tabla `users` (sin cambios en schema).
- **UserTenantRole (UTR)**: Entrada en `user_tenant_roles` con `status='active'`, `role_id`, `assigned_by`, `assigned_at`.

---

## Criterios de Éxito

### Resultados Medibles

- **SC-001**: `POST /api/v1/users` con `role` válido retorna 201 y crea exactamente 1 fila en `users` + 1 fila en `user_tenant_roles` con `status='active'`.
- **SC-002**: `POST /api/v1/users` con `role` inválido retorna 400 sin crear ninguna fila.
- **SC-003**: La interacción Pact `user-service-api-roles-extension — POST /users con rol inicial` pasa al 100%.
- **SC-004**: Los 5 endpoints existentes de `POST/GET/PATCH/DELETE /users` no presentan regresiones.
