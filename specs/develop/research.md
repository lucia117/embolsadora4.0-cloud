# Research: POST /users con Asignación de Rol Inicial

**Fecha**: 2026-04-11  
**Feature**: `develop` — Completar Pact `user-service-api-roles-extension`

---

## Decisión 1: Campo `role` — validación por enum vs FK

**Decisión**: Quitar validación `oneof=admin user` del DTO. Solo `required`. La FK `user_tenant_roles.role_id → roles.id` hace la validación de existencia a nivel BD.

**Rationale**: La tabla `roles` ahora tiene roles del sistema (`"admin"`, `"operario"`, `"cliente_admin"`, `"cliente_operario"`) y roles custom con UUID. El enum `oneof=admin user` estaba desactualizado desde la migración 012. Validar por enum en Go requeriría consulta extra a BD o mantener una lista hardcodeada que diverge con el tiempo.

**Alternativa descartada**: Consulta a BD en el handler para validar existencia del rol antes del INSERT. Descartada por latencia extra (round-trip adicional) cuando la FK ya maneja esto atómicamente.

---

## Decisión 2: Ubicación de la transacción — repo vs service

**Decisión**: Nuevo método `CreateWithRole` en la interfaz `Repository` del users repo. La transacción vive en la capa infra.

**Rationale**: La Constitución (Principio I) establece flujo `transport → app → domain ← repo`. El service no debe manejar `pgx.Tx` ni conocer detalles de SQL. El patrón de transacción ya existe en `userRoleRepository.BulkCreate` — se replica en la capa de users.

**Alternativa descartada**: Dos operaciones separadas en el service (crear user, luego crear UTR). Descartada por riesgo de inconsistencia: si el segundo INSERT falla, queda un usuario sin rol sin mecanismo automático de recuperación.

**Alternativa descartada**: Inyectar el pool `pgxpool.Pool` en el service. Descartada por violación de arquitectura hexagonal (infra filtraría a la capa app).

---

## Decisión 3: Status del UTR — `active` vs `pending`

**Decisión**: `status = 'active'` desde la creación.

**Rationale**: La creación directa por un admin es una asignación autoritativa. El flujo `pending → active` es exclusivo de las invitaciones (el usuario acepta y se autentica). En este caso el admin decide el rol, no hay confirmación del usuario requerida.

---

## Decisión 4: `assigned_by` — campo requerido u opcional

**Decisión**: Requerido. Si el JWT no tiene `UserID`, el handler retorna 401 antes de llegar al service.

**Rationale**: El UTR schema requiere saber quién asignó el rol para auditoría. El admin siempre está autenticado con JWT en la superficie ABM. El mismo patrón ya existe en `UpdateUserStatus`.

---

## Decisión 5: Response shape

**Decisión**: `UserResponse` sin cambios (sin campo `roles`).

**Rationale**: El Pact de `POST /users con rol inicial` solo valida que el usuario fue creado. La respuesta con roles corresponde a `GET /users/:id?include=roles` (ya implementado en 007). Incluir `roles` en el response de creación sería un cambio MINOR no solicitado y añadiría complejidad sin beneficio.

---

## Decisión 6: Manejo de `domain.ErrInvalidRoleID`

**Decisión**: Mapear `domain.ErrInvalidRoleID` → HTTP 400, código `"INVALID_ROLE"` en `errors.go`.

**Rationale**: El error ya existe en el codebase (`internal/repo/pg/user_roles/repository.go` lo usa). Solo falta el mapeo en el handler de users.

---

## Impacto en Esquema

| Tabla | Cambio |
|---|---|
| `users` | Ninguno |
| `user_tenant_roles` | Ninguno (inserción nueva, no cambio de schema) |
| `roles` | Ninguno |

**Sin migración nueva.**
