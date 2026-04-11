# Tasks: POST /users con Asignación de Rol Inicial

**Input**: Design documents de `/specs/013-user-create-with-role/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Organización**: 1 user story (P1) — todas las tareas son para completar la interacción Pact `POST /users con rol inicial`.

## Formato: `[ID] [P?] [Story] Descripción con ruta de archivo`

- **[P]**: Puede ejecutarse en paralelo (archivos distintos, sin dependencias incompletas)
- **[US1]**: Pertenece a la única user story de esta feature

---

## Phase 1: Setup

> Sin tareas — rama `013-user-create-with-role` ya creada, artefactos de spec generados.

---

## Phase 2: Foundational (Prerequisito bloqueante)

**Propósito**: Extender el domain command con el campo `AssignedBy`. Bloquea el resto.

- [X] T001 Agregar campo `AssignedBy string` a `CreateUserCommand` en `internal/domain/users/commands.go`

**Checkpoint**: `CreateUserCommand` compilará con el nuevo campo — el resto de las fases puede comenzar.

---

## Phase 3: User Story 1 — Crear usuario con rol activo asignado (P1) 🎯 MVP

**Goal**: `POST /api/v1/users` crea el usuario y su UTR activo en una única transacción atómica.

**Independent Test**: Ejecutar Escenario 1 de `quickstart.md` — verificar 201 + fila en `user_tenant_roles` con `status='active'`.

### Implementación

- [X] T002 [US1] Agregar método `CreateWithRole(ctx context.Context, user *domainUsers.User, utr *domain.UserTenantRole) (*domainUsers.User, error)` a la interfaz `Repository` en `internal/repo/pg/users/users_repo.go`
- [X] T003 [US1] Implementar `CreateWithRole` en `internal/repo/pg/users/users_repo.go`: abrir `pgx.Tx`, INSERT en `users` (reusar lógica de `Create`), INSERT en `user_tenant_roles` con los campos del UTR, mapear FK violation de `role_id` → `domain.ErrInvalidRoleID`, mapear unique violation del índice UTR activo → `domain.ErrUserAlreadyHasActiveRole`, COMMIT/ROLLBACK
- [X] T004 [US1] Modificar `Service.CreateUser` en `internal/app/users/service.go`: parsear `cmd.AssignedBy` a `uuid.UUID`, construir `domain.UserTenantRole{Status: UserRoleStatusActive, RoleID: &cmd.Role, AssignedBy: &assignedByUUID, AssignedAt: &now}`, reemplazar `s.repo.Create` por `s.repo.CreateWithRole`, agregar manejo de `domain.ErrInvalidRoleID` con log warn
- [X] T005 [P] [US1] Modificar validación de `Role` en `internal/api/handler/users/dto/create.go`: cambiar `binding:"required,oneof=admin user"` → `binding:"required"`
- [X] T006 [US1] Modificar `CreateUser` en `internal/api/handler/users/handler.go`: extraer `callerUUID := platform.UserID(c.Request.Context())`, agregar guard 401 si `callerUUID == nil`, pasar `AssignedBy: callerUUID.String()` al `CreateUserCommand`
- [X] T007 [P] [US1] Agregar case `domain.ErrInvalidRoleID` → HTTP 400 con código `"INVALID_ROLE"` en `internal/api/handler/users/errors.go`

**Checkpoint**: `POST /api/v1/users` con `role` válido → 201 + UTR activo. Con `role` inválido → 400 `INVALID_ROLE`. Rollback verificado con Escenario 3 del quickstart.

---

## Phase 4: Polish & Cross-Cutting Concerns

**Propósito**: Observabilidad, documentación Postman, actualización del tracker de Pacts.

- [X] T008 [P] Verificar logs Zap en `internal/app/users/service.go` — confirmar que `CreateUser` loguea `info` con `user_id`, `tenant_id`, `role_id` al crear exitosamente, y `warn` para `ErrInvalidRoleID`
- [X] T009 [P] Agregar carpeta "Users — Create with Role (013 Pact)" a `postman/Embolsadora-API-Complete.postman_collection.json` con 3 requests: happy path 201 (role válido), 400 INVALID_ROLE (role inexistente), 409 EMAIL_TAKEN (email duplicado)
- [X] T010 [P] Actualizar `PACTS_ANALYSIS.md`: marcar `POST /api/v1/users con rol inicial` como ✅, actualizar cobertura de `user-service-api-roles-extension` a 4/4, actualizar cobertura global

---

## Dependencias y Orden de Ejecución

```
T001
 ├─→ T002 → T003 → T004 → T006 → T009
 └─→ T005 (paralelo con T002-T004)
     T007 (paralelo con T001-T006, solo usa domain errors)
     T008 (después de T004)
     T010 (después de T006)
```

### Orden secuencial mínimo

```
T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009 → T010
```

### Con paralelismo (recomendado)

```
T001
 ├── T002 → T003 → T004 → T006
 ├── T005 (en paralelo desde T001)
 └── T007 (en paralelo desde T001)
     T008, T009, T010 (en paralelo, después de T006)
```

---

## Parallel Example: User Story 1

```
# Tras completar T001, lanzar en paralelo:
Task T005: dto/create.go — cambiar validación de role
Task T007: errors.go — mapear ErrInvalidRoleID

# Tras completar T006, lanzar en paralelo:
Task T008: verificar logs en service.go
Task T009: agregar requests a Postman collection
Task T010: actualizar PACTS_ANALYSIS.md
```

---

## Implementation Strategy

### MVP (única historia, entrega única)

1. Completar Phase 2 (T001) — foundation
2. Completar Phase 3 (T002–T007) — implementación completa
3. **VALIDAR**: ejecutar quickstart.md escenarios 1–5
4. Completar Phase 4 (T008–T010) — polish
5. PR hacia `develop`

---

## Resumen

| Fase | Tareas | Paralelas |
|---|---|---|
| Phase 2 — Foundational | 1 (T001) | 0 |
| Phase 3 — US1 | 6 (T002–T007) | 2 (T005, T007) |
| Phase 4 — Polish | 3 (T008–T010) | 3 |
| **Total** | **10** | **5** |

**Scope MVP**: User Story 1 = toda la feature (es la única historia).  
**Pact cubierto**: `user-service-api-roles-extension — POST /users con rol inicial` ✅
