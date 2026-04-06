# Research: Extensión de Gestión de Usuarios (007)

**Fecha**: 2026-04-03  
**Feature**: `007-user-roles-status`  
**Spec**: [spec.md](spec.md)

---

## Decisión 1: Definición de "usuarios pendientes"

**Pregunta**: ¿Qué significa "pendiente de activación" en el contexto de esta API?

**Decisión**: Usuarios con `user_tenant_roles.status = 'pending'` para el tenant actual.

**Rationale**: Cuando un admin invita a un usuario al tenant via `POST /invitations`, se crea un registro en `user_tenant_roles` con `status = 'pending'`. Al aceptar la invitación y autenticarse por primera vez, ese status pasa a `'active'`. Los usuarios "pendientes" son exactamente los que fueron invitados pero aún no completaron este ciclo.

**Alternativa descartada**: Usar `users.status = 'invited'`. Descartada porque `users.status` es global (no por tenant), y un usuario puede estar activo en un tenant pero pendiente en otro. El campo UTR es el correcto para aislamiento multi-tenant.

---

## Decisión 2: Implementación de `include=roles`

**Pregunta**: ¿JOIN en una query o segunda query separada?

**Decisión**: JOIN único en el repositorio de usuarios → nuevo método `GetByIDWithRoles`.

**Rationale**: El repositorio `UserRoleRepository.FindByUser` ya existe pero retorna datos de todos los tenants. Para `include=roles` necesitamos:
1. Los datos del usuario (de la tabla `users`)
2. El UTR activo para este tenant específico (`user_tenant_roles WHERE tenant_id=X AND status='active'`)
3. El rol completo (JOIN con `roles`)

Un JOIN único es más eficiente que tres queries separadas. El método vive en el repositorio de usuarios porque el objeto raíz es el usuario.

**Alternativa descartada**: Reusar `UserRoleRepository.FindByUser` + filtrar por tenant en la capa de servicio. Descartada por ineficiencia (trae datos de todos los tenants para filtrar en memoria).

---

## Decisión 3: Endpoint `PATCH /users/:id/status` — ¿qué tabla actualizar?

**Pregunta**: ¿Actualizar `users.status` (global) o `user_tenant_roles.status` (per-tenant)?

**Decisión**: Actualizar `user_tenant_roles.status` para el par (user_id, tenant_id) activo.

**Rationale**: La spec dice "del tenant", lo que implica operación per-tenant. Si actualizáramos `users.status`, un admin del tenant A podría desactivar globalmente a un usuario que también pertenece al tenant B, violando el principio de aislamiento multi-tenant (Constitución II).

**Mapeo de estados**:
| Estado (spec) | Estado UTR | Semántica |
|---|---|---|
| `active` | `active` | El usuario puede operar normalmente en el tenant |
| `inactive` | `revoked` | El usuario fue desactivado por el admin del tenant |
| `suspended` | `suspended` (nuevo) | El usuario está suspendido temporalmente |

**Alternativa descartada**: Mapear `inactive` → `revoked` y no agregar `suspended`. Descartada porque perderíamos la distinción entre "revocado permanentemente" (acción del admin) y "suspendido temporalmente" (estado reversible). El Pact especifica los 3 estados.

---

## Decisión 4: Migración para el estado `suspended`

**Decisión**: Nueva migración que extiende el CHECK constraint de `user_tenant_roles.status`.

**Rationale**: La tabla actual tiene `CHECK (status IN ('active', 'pending', 'revoked'))`. Para soportar `suspended`, necesitamos extenderlo. Esta es una migración minor (additive), backward-compatible.

**SQL**:
```sql
ALTER TABLE user_tenant_roles 
  DROP CONSTRAINT IF EXISTS user_tenant_roles_status_check,
  ADD CONSTRAINT user_tenant_roles_status_check 
    CHECK (status IN ('active', 'pending', 'revoked', 'suspended'));
```

---

## Decisión 5: Protección anti-auto-desactivación

**Decisión**: En el handler de `PATCH /users/:id/status`, comparar el ID del usuario autenticado (extraído del JWT via `platform.UserID(ctx)`) con el `:id` del path. Si son iguales, retornar 400.

**Rationale**: RF-006 establece que un admin no puede desactivarse a sí mismo. Sin este guard, el último admin de un tenant podría quedar bloqueado.

**Nota de implementación**: `platform.UserID(ctx)` retorna el UUID del usuario en la tabla `users` (no el supabase_user_id). El `:id` del path también es el UUID interno.

---

## Decisión 6: Registro de rutas — conflicto `/users/pending` vs `/users/:id`

**Decisión**: Registrar `GET /users/pending` ANTES de `GET /users/:id` en `RegisterAdminRoutes`.

**Rationale**: En Gin, las rutas se evalúan en orden de registro. Si `/users/:id` está primero, la ruta `/users/pending` se interpretaría como `id = "pending"`, causando un 404 o error de UUID inválido. Al registrar el literal primero, Gin lo prioriza sobre el wildcard.

---

## Decisión 7: Estilo de handlers

**Decisión**: Receiver methods en el struct `Handler` existente (`internal/api/handler/users/`).

**Rationale**: Los handlers de usuarios usan `(h *Handler) MethodName(c *gin.Context)`. Mantener consistencia con el patrón actual. No crear nuevos archivos de handler separados salvo que la lógica sea demasiado extensa (>150 LOC).

**Nuevos métodos**: `GetUserWithRoles`, `ListPendingUsers`, `UpdateUserStatus`.

---

## Decisión 8: Response de `GET /users/:id?include=roles`

**Decisión**: El campo `roles` en el response es un array (puede ser vacío si el usuario no tiene rol asignado).

**Rationale**: El Pact de `user-service-api-roles-extension` espera `include=roles` con el rol activo. Si no hay UTR activo para este tenant, el array es vacío `[]`. Backward-compatible: sin el parámetro, el response es idéntico al actual (sin campo `roles`).

**Estructura del elemento `roles`**:
```json
{
  "id": "admin",
  "name": "Administrador",
  "permissions": ["users:read", "users:write", ...]
}
```

---

## Resumen de Impacto en el Esquema

| Cambio | Tabla | Tipo |
|---|---|---|
| Agregar status `suspended` | `user_tenant_roles` | CHECK constraint extension |
| Sin cambios | `users` | — |
| Sin cambios | `roles` | — |

La migración es **additive** (MINOR en semver). No rompe compatibilidad con registros existentes.
