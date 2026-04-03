# Tasks: Extensión de Gestión de Usuarios (007)

**Input**: Design documents from `specs/007-user-roles-status/`
**Branch**: `007-user-roles-status`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Organization**: Tasks grouped by user story — cada historia es un incremento independientemente testeable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Puede ejecutarse en paralelo (archivo distinto, sin dependencias incompletas)
- **[Story]**: Historia de usuario correspondiente (US1, US2, US3)

---

## Phase 1: Setup (Migración de Schema)

**Purpose**: Extender el CHECK constraint de `user_tenant_roles.status` para soportar `suspended`.

- [ ] T001 Crear migración `migrations/000013_add_suspended_status_to_utr.up.sql` con `ALTER TABLE user_tenant_roles DROP CONSTRAINT IF EXISTS user_tenant_roles_status_check, ADD CONSTRAINT user_tenant_roles_status_check CHECK (status IN ('active', 'pending', 'revoked', 'suspended'))`
- [ ] T002 Crear migración `migrations/000013_add_suspended_status_to_utr.down.sql` que restaura el constraint original con `CHECK (status IN ('active', 'pending', 'revoked'))`

**Checkpoint**: Aplicar con `migrate -path migrations/ -database $DATABASE_URL up 1`

---

## Phase 2: Foundational (Tipos de Dominio y Wiring Base)

**Purpose**: Tipos de dominio y extensión del Service que bloquean las 3 historias.

**⚠️ CRÍTICO**: Completar antes de iniciar cualquier historia.

- [ ] T003 [P] Agregar constante `UserRoleStatusSuspended UserRoleStatus = "suspended"` en `internal/domain/user_roles.go`
- [ ] T004 [P] Agregar tipos `UserWithRoles` y `AssignedRole` en `internal/domain/users/user.go`:
  ```go
  type UserWithRoles struct {
      User
      Roles []AssignedRole
  }
  type AssignedRole struct {
      ID          string
      Name        string
      Permissions []string
  }
  ```
- [ ] T005 [P] Agregar constantes de error `ErrCannotDeactivateSelf` y `ErrInvalidStatus` en `internal/domain/users/errors.go` (o en el archivo de errores de dominio existente)
- [ ] T006 Extender `Service` en `internal/app/users/service.go` para inyectar `UserRoleRepository`: agregar campo `userRoleRepo userRolesRepo.UserRoleRepository` al struct y actualizar `NewService(repo users.Repository, userRoleRepo userRolesRepo.UserRoleRepository, logger *zap.Logger) *Service`
- [ ] T007 Actualizar el sitio de construcción del Service en `internal/api/router.go` (función `RegisterAdminRoutes`) para pasar `deps.UserRoleRepo` al `NewService` actualizado

**Checkpoint**: `go build ./...` debe compilar sin errores antes de continuar.

---

## Phase 3: User Story 1 — Ver usuario con roles (Prioridad: P1) 🎯 MVP

**Goal**: `GET /users/:id?include=roles` retorna el usuario con su rol activo en el tenant. Sin el parámetro, el comportamiento es idéntico al actual.

**Independent Test**: `curl "$BASE_URL/users/$USER_ID?include=roles" -H "Authorization: Bearer $JWT" -H "X-Tenant-ID: $TENANT_ID"` retorna 200 con campo `roles` array.

### Implementación US1

- [ ] T008 [P] [US1] Agregar método `GetByIDWithRoles` a la interface `Repository` en `internal/repo/pg/users/repository.go`:
  ```go
  GetByIDWithRoles(ctx context.Context, tenantID, userID string) (*domainUsers.UserWithRoles, error)
  ```
- [ ] T009 [P] [US1] Agregar constante SQL `getUserByIDWithRolesQuery` en `internal/repo/pg/users/postgres.go` (o crear `internal/repo/pg/users/queries.go`):
  ```sql
  SELECT u.id, u.tenant_id, u.first_name, u.last_name, u.email, u.role, u.status,
         u.image, u.created_at, u.updated_at, u.deleted_at,
         r.id AS role_id, r.name AS role_name, r.permissions AS role_permissions
  FROM users u
  LEFT JOIN user_tenant_roles utr
      ON utr.user_id = u.id AND utr.tenant_id = u.tenant_id AND utr.status = 'active'
  LEFT JOIN roles r
      ON r.id = utr.role_id AND r.deleted_at IS NULL
  WHERE u.id = $1 AND u.tenant_id = $2 AND u.deleted_at IS NULL
  ```
- [ ] T010 [US1] Implementar `GetByIDWithRoles` en `internal/repo/pg/users/postgres.go`: ejecutar la query, scanear los campos de usuario y los campos opcionales del rol (`role_id`, `role_name`, `role_permissions` pueden ser NULL si no hay UTR activo), construir `UserWithRoles` con `Roles: []AssignedRole{}` si el LEFT JOIN no trajo rol
- [ ] T011 [US1] Implementar `Service.GetUserWithRoles` en `internal/app/users/service.go`: llamar a `repo.GetByIDWithRoles`, mapear errores de dominio (ErrNotFound → 404), loguear con Zap
- [ ] T012 [P] [US1] Agregar DTOs `UserWithRolesResponse` y `RoleInfo` en `internal/api/handler/users/dto/` (nuevo archivo `include_roles.go`):
  ```go
  type RoleInfo struct {
      ID          string   `json:"id"`
      Name        string   `json:"name"`
      Permissions []string `json:"permissions"`
  }
  type UserWithRolesResponse struct {
      dto.UserResponse           // embed response existente
      Roles []RoleInfo `json:"roles"`
  }
  ```
- [ ] T013 [US1] Modificar `Handler.GetUser` en `internal/api/handler/users/handler.go`: si `c.Query("include") == "roles"`, llamar a `service.GetUserWithRoles` y retornar `UserWithRolesResponse`; de lo contrario mantener el flujo actual con `service.GetUser`

**Checkpoint**: `curl "$BASE_URL/users/$USER_ID?include=roles"` retorna 200 con campo `roles`. `curl "$BASE_URL/users/$USER_ID"` retorna 200 sin campo `roles` (backward-compat).

---

## Phase 4: User Story 2 — Cambiar estado de usuario (Prioridad: P2)

**Goal**: `PATCH /users/:id/status` cambia el estado de participación del usuario en el tenant (active/inactive/suspended). Solo admins. Admin no puede desactivarse a sí mismo.

**Independent Test**: `curl -X PATCH "$BASE_URL/users/$USER_ID/status" -d '{"status":"inactive"}'` retorna 200. `curl -X PATCH "$BASE_URL/users/$ADMIN_ID/status" -d '{"status":"inactive"}'` retorna 400.

### Implementación US2

- [ ] T014 [P] [US2] Agregar constante SQL `updateUTRStatusQuery` en `internal/repo/pg/user_roles/resources.go`:
  ```sql
  UPDATE user_tenant_roles
  SET status = $1, updated_at = NOW()
  WHERE user_id = $2 AND tenant_id = $3 AND status != 'pending'
  RETURNING id, user_id, tenant_id, role_id, status, assigned_by, assigned_at, created_at, updated_at
  ```
- [ ] T015 [P] [US2] Agregar DTO `UpdateUserStatusRequest` en `internal/api/handler/users/dto/` (nuevo archivo `status.go`):
  ```go
  type UpdateUserStatusRequest struct {
      Status string `json:"status" binding:"required"`
  }
  ```
- [ ] T016 [US2] Agregar método `UpdateStatus` a la interface `UserRoleRepository` en `internal/repo/pg/user_roles/repository.go`:
  ```go
  UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus) (*domain.UserTenantRole, error)
  ```
- [ ] T017 [US2] Implementar `UpdateStatus` en `internal/repo/pg/user_roles/repository.go`: ejecutar `updateUTRStatusQuery`, retornar `ErrNotFound` si no hay filas afectadas (RETURNING vacío)
- [ ] T018 [US2] Implementar `Service.UpdateUserStatus` en `internal/app/users/service.go`:
  - Validar `status` in {active, inactive, suspended} → retornar `ErrInvalidStatus` si inválido
  - Comparar `callerID == userID` → retornar `ErrCannotDeactivateSelf` si iguales
  - Verificar que el usuario pertenece al tenant via `repo.GetByID` → retornar `ErrNotFound` si no
  - Mapear status: `"active"` → `UserRoleStatusActive`, `"inactive"` → `UserRoleStatusRevoked`, `"suspended"` → `UserRoleStatusSuspended`
  - Llamar `userRoleRepo.UpdateStatus(ctx, userUUID, tenantUUID, mappedStatus)`
  - Retornar el usuario actualizado via `repo.GetByID`
- [ ] T019 [US2] Implementar `Handler.UpdateUserStatus` en `internal/api/handler/users/handler.go`: extraer `tenant_id` del contexto, `id` del path, `callerID` del JWT (`platform.UserID(ctx)` o `c.GetString("user_id")`), bind `UpdateUserStatusRequest`, llamar `service.UpdateUserStatus`, mapear errores a HTTP (400 para ErrInvalidStatus/ErrCannotDeactivateSelf, 403 para falta de permisos, 404 para ErrNotFound)
- [ ] T020 [US2] Registrar `PATCH /users/:id/status` en `internal/api/router.go` dentro del bloque `userRoutes`, con `middleware.RequireRole("admin")`:
  ```go
  userRoutes.PATCH("/users/:id/status", middleware.RequireRole("admin"), uh.UpdateUserStatus)
  ```

**Checkpoint**: `curl -X PATCH "$BASE_URL/users/$USER_ID/status" -d '{"status":"inactive"}'` retorna 200. Auto-deactivación retorna 400 con `CANNOT_DEACTIVATE_SELF`. Estado inválido retorna 400 con `INVALID_STATUS`.

---

## Phase 5: User Story 3 — Listar usuarios pendientes (Prioridad: P2)

**Goal**: `GET /users/pending` retorna usuarios con `user_tenant_roles.status = 'pending'` en el tenant. Solo admins.

**Independent Test**: `curl "$BASE_URL/users/pending"` retorna 200 con `{"data": [...], "total": N}`. Sin usuarios pendientes retorna `{"data": [], "total": 0}`.

### Implementación US3

- [ ] T021 [P] [US3] Agregar constante SQL `listPendingByTenantQuery` en `internal/repo/pg/users/postgres.go` (o `queries.go`):
  ```sql
  SELECT u.id, u.tenant_id, u.first_name, u.last_name, u.email, u.role, u.status,
         u.image, u.created_at, u.updated_at, u.deleted_at
  FROM users u
  JOIN user_tenant_roles utr
      ON utr.user_id = u.id AND utr.tenant_id = $1 AND utr.status = 'pending'
  WHERE u.deleted_at IS NULL
  ORDER BY utr.created_at DESC
  ```
- [ ] T022 [P] [US3] Agregar método `ListPendingByTenant` a la interface `Repository` en `internal/repo/pg/users/repository.go`:
  ```go
  ListPendingByTenant(ctx context.Context, tenantID string) ([]*users.User, error)
  ```
- [ ] T023 [US3] Implementar `ListPendingByTenant` en `internal/repo/pg/users/postgres.go`: ejecutar la query, scanear filas con el mismo patrón que `ListByTenant`, retornar slice vacío (no nil) si no hay filas
- [ ] T024 [US3] Implementar `Service.ListPendingUsers` en `internal/app/users/service.go`: llamar a `repo.ListPendingByTenant(ctx, tenantID)`, loguear resultado con Zap
- [ ] T025 [US3] Implementar `Handler.ListPendingUsers` en `internal/api/handler/users/handler.go`: extraer `tenant_id` del contexto, llamar `service.ListPendingUsers`, retornar `{"data": [...], "total": len(users)}` (reusar `dto.UserResponse` para cada item del slice)
- [ ] T026 [US3] Registrar `GET /users/pending` en `internal/api/router.go` **ANTES** de `GET /users/:id` para evitar conflicto Gin, con `middleware.RequireRole("admin")`:
  ```go
  userRoutes.GET("/users/pending", middleware.RequireRole("admin"), uh.ListPendingUsers)
  // ... luego GET /users/:id como ya está
  ```

**Checkpoint**: `curl "$BASE_URL/users/pending"` retorna 200 con la estructura `{"data":[], "total":0}`.

---

## Phase 6: Polish & Validación

**Purpose**: Documentación, colección Postman y actualización de contratos Pact.

- [ ] T027 [P] Agregar carpeta "Users — Extension (007)" a `postman/Embolsadora-API-Complete.postman_collection.json` con 7 requests que cubren los escenarios del `quickstart.md` (GET /users/:id?include=roles, GET /users/:id sin include, PATCH /users/:id/status → active/inactive/400/400-self, GET /users/pending)
- [ ] T028 [P] Actualizar `PACTS_ANALYSIS.md`: en la sección `user-service-api-roles-extension` marcar ✅ las 3 interacciones implementadas (GET include=roles, PATCH status, GET pending); mantener ❌ la de `POST /users` con rol inicial (fuera del alcance de 007)
- [ ] T029 Ejecutar los curls de `specs/007-user-roles-status/quickstart.md` contra el servidor local y completar el checklist Pact de 4 interacciones

---

## Dependencies & Execution Order

### Dependencias entre Fases

```
Phase 1 (Migration)
    │
    ▼
Phase 2 (Foundational — Domain + Service wiring)
    │
    ├──► Phase 3 (US1 — include=roles)    ← puede ir en paralelo con US2 y US3
    ├──► Phase 4 (US2 — PATCH status)     ← puede ir en paralelo con US1 y US3
    └──► Phase 5 (US3 — GET pending)      ← puede ir en paralelo con US1 y US2
              │
              ▼
         Phase 6 (Polish)
```

### Dependencias Internas por Historia

| Historia | Orden interno |
|---|---|
| US1 | T008, T009 (paralelos) → T010 → T011 → T012 (paralelo) → T013 |
| US2 | T014, T015 (paralelos) → T016 → T017 → T018 → T019 → T020 |
| US3 | T021, T022 (paralelos) → T023 → T024 → T025 → T026 |

---

## Parallel Opportunities

```bash
# Phase 2: domain tasks
T003  # UserRoleStatusSuspended
T004  # UserWithRoles types
T005  # error constants

# Phase 3 (US1): repo interface + SQL
T008  # interface method
T009  # SQL constant
T012  # DTOs

# Phase 4 (US2): SQL + DTO
T014  # SQL constant
T015  # DTO

# Phase 5 (US3): SQL + interface
T021  # SQL constant
T022  # interface method
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1: Migration files
2. Phase 2: Foundational (T003-T007)
3. Phase 3: US1 completo (T008-T013)
4. **VALIDAR**: `GET /users/:id?include=roles` funciona end-to-end
5. Satisface el contrato Pact más importante (P1)

### Incremental Delivery

1. Setup + Foundational → compilación limpia
2. US1 → GET include=roles ✅ → 1er Pact satisfecho
3. US2 → PATCH status ✅ → 2do Pact satisfecho
4. US3 → GET pending ✅ → 3er Pact satisfecho
5. Polish → Postman + docs actualizados

---

## Notes

- `GET /users/pending` DEBE registrarse antes de `GET /users/:id` en router.go (Gin evalúa literales antes que wildcards)
- `PATCH /users/:id/status` necesita extraer el caller ID del JWT para el guard RF-006
- `GetByIDWithRoles` usa LEFT JOIN — si no hay UTR activo, `Roles` debe ser `[]AssignedRole{}` (no nil) para que el JSON serialice como `[]` y no `null`
- El `Service.NewService` cambia de firma — verificar que `RegisterAdminRoutes` en `router.go` (T007) se actualice antes de que los nuevos handlers compilen
- Commit sugerido por fase para facilitar review del PR
