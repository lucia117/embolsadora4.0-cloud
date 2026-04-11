# Implementation Plan: POST /users con Asignación de Rol Inicial

**Branch**: `013-user-create-with-role` | **Fecha**: 2026-04-11 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification de `/specs/develop/spec.md`

---

## Summary

Completar la última interacción Pact pendiente de `user-service-api-roles-extension`: `POST /api/v1/users` debe crear el usuario y su UTR activo en una única transacción atómica. El approach técnico es un nuevo método `CreateWithRole` en el users repo que agrupa ambos INSERTs en `pgx.Tx`. Sin migración nueva.

---

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), uuid (google/uuid)  
**Storage**: PostgreSQL — tablas `users` y `user_tenant_roles` (sin cambios de schema)  
**Testing**: testify, uber/mock; colección Postman para smoke test  
**Target Platform**: Linux server (Docker)  
**Project Type**: web-service (hexagonal monolith)  
**Performance Goals**: P95 < 300ms en `POST /users`  
**Constraints**: Sin migración nueva; backward compatible en response shape  
**Scale/Scope**: Feature acotada — 1 Pact interaction, ~5 archivos modificados

---

## Constitution Check

| Principio | Estado | Detalle |
|---|---|---|
| I. Arquitectura Hexagonal | ✅ | Transacción en repo (infra); service orquesta; handler solo HTTP |
| II. Aislamiento de tenants | ✅ | `tenant_id` en todos los INSERTs; extraído de middleware |
| III. Observabilidad | ✅ | Logs Zap en service (info/warn/error); métricas Prometheus en handler |
| IV. Testing dirigido por contrato | ✅ | Quickstart verifica los 5 escenarios; Postman collection actualizado |
| V. Backward compatibility | ✅ | Response shape idéntico (UserResponse sin campo `roles`) |

**GATE: PASSED** — Sin violaciones.

---

## Project Structure

### Documentación (esta feature)

```text
specs/develop/
├── plan.md              ← este archivo
├── spec.md              ← especificación
├── research.md          ← decisiones de diseño
├── data-model.md        ← entidades y flujo de datos
├── quickstart.md        ← escenarios de smoke test
├── contracts/
│   └── user-service-api-create-with-role.openapi.yaml
└── tasks.md             ← generado por /speckit.tasks
```

### Archivos a modificar

```text
internal/
├── api/handler/users/
│   ├── dto/create.go          → quitar oneof=admin user
│   ├── handler.go             → extraer callerID, pasar AssignedBy
│   └── errors.go              → mapear ErrInvalidRoleID → 400
├── app/users/
│   └── service.go             → construir UTR, llamar CreateWithRole
├── domain/users/
│   └── commands.go            → agregar AssignedBy a CreateUserCommand
└── repo/pg/users/
    └── users_repo.go          → interfaz + implementación CreateWithRole

postman/
└── Embolsadora-API-Complete.postman_collection.json  → agregar carpeta POST /users con rol
```

---

## Complexity Tracking

Sin violaciones — no aplica.

---

## Fases de Implementación

### Fase 1 — Domain (sin dependencias)

**Objetivo**: Extender `CreateUserCommand` con `AssignedBy`.

**Tarea F1-T1**: Agregar campo `AssignedBy string` a `CreateUserCommand` en `internal/domain/users/commands.go`.

---

### Fase 2 — Repo (depende de Fase 1)

**Objetivo**: Implementar `CreateWithRole` en el users repo.

**Tarea F2-T1**: Agregar `CreateWithRole` a la interfaz `Repository` en `internal/repo/pg/users/users_repo.go`:

```go
CreateWithRole(ctx context.Context, user *User, utr *domain.UserTenantRole) (*User, error)
```

**Tarea F2-T2**: Implementar `CreateWithRole` en la misma infra:
- `BEGIN` pgx.Tx
- INSERT en `users` (reusar lógica de `Create`)
- INSERT en `user_tenant_roles` (reusar queries de `user_roles` repo)
- Mapear FK violation de `role_id` → `domain.ErrInvalidRoleID`
- Mapear unique violation de UTR → `domain.ErrUserAlreadyHasActiveRole`
- `COMMIT` / `ROLLBACK`

---

### Fase 3 — Service (depende de Fase 2)

**Objetivo**: `CreateUser` construye el UTR y delega a `CreateWithRole`.

**Tarea F3-T1**: Modificar `Service.CreateUser` en `internal/app/users/service.go`:
- Parsear `cmd.AssignedBy` a `uuid.UUID`
- Construir `domain.UserTenantRole` con `Status=active`, `AssignedAt=now`, `RoleID=&cmd.Role`
- Reemplazar `s.repo.Create(ctx, user)` por `s.repo.CreateWithRole(ctx, user, utr)`
- Manejar `domain.ErrInvalidRoleID` con log warn

---

### Fase 4 — Handler + DTO (depende de Fase 3)

**Objetivo**: Extraer `callerID` del JWT y ajustar validación de `role`.

**Tarea F4-T1**: Modificar `dto/create.go`:
- Cambiar `binding:"required,oneof=admin user"` → `binding:"required"` en `Role`

**Tarea F4-T2**: Modificar `handler.go` — `CreateUser`:
- Extraer `callerUUID := platform.UserID(c.Request.Context())`
- Guard 401 si `callerUUID == nil`
- Pasar `AssignedBy: callerUUID.String()` al command

**Tarea F4-T3**: Modificar `errors.go`:
- Agregar case `domain.ErrInvalidRoleID` → 400, código `"INVALID_ROLE"`

---

### Fase 5 — Observabilidad (depende de Fase 4)

**Objetivo**: Asegurar logs y métricas en el nuevo path.

**Tarea F5-T1**: Verificar logs Zap en `Service.CreateUser`:
- `info`: usuario + UTR creados exitosamente (con `user_id`, `tenant_id`, `role_id`)
- `warn`: `ErrInvalidRoleID`
- `error`: fallas inesperadas de repo

**Tarea F5-T2**: Verificar métricas Prometheus en `handler.go` — `CreateUser`:
- Confirmar que el handler tiene instrumentación (ya existe en la capa de middleware/handler).

---

### Fase 6 — Postman + Verificación (depende de Fase 4)

**Objetivo**: Agregar la nueva interacción Pact a la colección y actualizar `PACTS_ANALYSIS.md`.

**Tarea F6-T1**: Agregar request a `postman/Embolsadora-API-Complete.postman_collection.json`:
- Carpeta "Users — Create with Role (Pact)"
- 3 requests: happy path (201), rol inválido (400), email duplicado (409)

**Tarea F6-T2**: Actualizar `PACTS_ANALYSIS.md`:
- Marcar `POST /api/v1/users con rol inicial` como ✅
- Actualizar cobertura estimada (~100% sin contar reports)

---

## Orden de Ejecución

```
F1-T1 → F2-T1 → F2-T2 → F3-T1 → F4-T1 → F4-T2 → F4-T3 → F5-T1 → F5-T2 → F6-T1 → F6-T2
```

Sin paralelismo — cada fase depende de la anterior.

---

## Criterios de Done

- [ ] `POST /api/v1/users` con `role` válido → 201 + 1 fila en `users` + 1 fila en `user_tenant_roles` con `status='active'`
- [ ] `POST /api/v1/users` con `role` inválido → 400 `INVALID_ROLE` sin ninguna fila creada
- [ ] `GET /users/:id?include=roles` muestra el rol asignado en la creación
- [ ] Todos los endpoints existentes de `/users` sin regresiones
- [ ] `PACTS_ANALYSIS.md` actualizado con la interacción marcada ✅
