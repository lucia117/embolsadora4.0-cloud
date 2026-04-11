# Tasks: Permissions Management API

**Input**: Design documents from `/specs/011-permissions-management/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅

**Tests**: No incluidos (no solicitados en la spec). Verificación via quickstart.md y Postman.

**Organización**: Tareas agrupadas por user story para habilitar implementación y testing independiente de cada historia.

## Formato: `[ID] [P?] [Story] Descripción con file path`

- **[P]**: Se puede ejecutar en paralelo (archivos distintos, sin dependencias incompletas)
- **[Story]**: A qué user story pertenece la tarea (US1–US5)
- Paths relativos desde la raíz del repositorio

---

## Phase 1: Setup — Migración de base de datos

**Propósito**: Crear la tabla `permissions` con los 17 permisos de sistema listos. Es prerequisito bloqueante para todo lo demás.

- [x] T001 Crear `migrations/000017_create_permissions_table.up.sql` con CREATE TABLE, CHECK constraints, índices, trigger `update_permissions_updated_at` y seed de los 17 permisos de sistema (ver `data-model.md` para SQL completo)
- [x] T002 [P] Crear `migrations/000017_create_permissions_table.down.sql` con `DROP TABLE IF EXISTS permissions CASCADE`
- [ ] T003 ⚠️ PASO MANUAL: Aplicar migración: `migrate -path migrations/ -database $DATABASE_URL up 1` — verificar con `SELECT COUNT(*) FROM permissions WHERE is_system_permission = TRUE` (debe retornar 17)

**Checkpoint**: Tabla `permissions` creada con 17 permisos de sistema. Base lista para el desarrollo.

---

## Phase 2: Foundational — Domain + Repository

**Propósito**: Infraestructura de dominio y acceso a BD que bloquea todas las user stories.

⚠️ **CRÍTICO**: Ninguna user story puede comenzar hasta completar esta fase.

- [x] T004 Crear `internal/domain/permissions.go` con struct `Permission` (ID, Name, Section, Description, IsSystemPermission, TenantID *uuid.UUID, CreatedAt, UpdatedAt) y errores de dominio: `ErrPermissionNotFound`, `ErrPermissionIsSystem`, `ErrPermissionValidationFailed`
- [x] T005 Crear `internal/repo/pg/permissions/repository.go` con:
  - Interface `Repository` con métodos: `List(ctx, tenantID) ([]*domain.Permission, error)`, `GetByID(ctx, id) (*domain.Permission, error)`, `Create(ctx, p) error`, `Update(ctx, p) error`, `Delete(ctx, id) error`
  - Struct `PostgresRepository` con `pool *pgxpool.Pool`
  - `NewPostgresRepository(pool) *PostgresRepository`
  - `List`: query `WHERE (tenant_id = $1 OR is_system_permission = TRUE)` ordenada `is_system_permission DESC, name ASC`; retorna `[]` vacío si no hay resultados
  - `GetByID`: busca por id sin filtrar por tenant (permisos de sistema tienen `tenant_id = NULL`); retorna `ErrPermissionNotFound` si no existe
  - `Create`: INSERT con id provisto (no `gen_random_uuid()` en BD; el ID viene del service)
  - `Update`: UPDATE `name, section, description, updated_at` WHERE id = $1; verifica que el registro existe post-update; retorna `ErrPermissionNotFound` si no existe
  - `Delete`: DELETE WHERE id = $1 AND is_system_permission = FALSE; retorna `ErrPermissionIsSystem` si el registro existe pero `is_system_permission = TRUE`; retorna `ErrPermissionNotFound` si no existe

**Checkpoint**: Domain + Repository compilando. Fundación lista para las user stories.

---

## Phase 3: User Story 1 — Listar permisos (Priority: P1) 🎯 MVP

**Goal**: `GET /api/v1/permissions` retorna el catálogo completo (sistema + custom del tenant) para usuarios autenticados.

**Independent Test**: `curl -X GET "$BASE_URL/permissions" -H "Authorization: Bearer $JWT" -H "X-Tenant-ID: $TENANT_ID"` → 200 con array de ≥ 17 items con `isSystemPermission: true`. Sin auth → 401.

- [x] T006 [US1] Crear `internal/app/permissions/service.go` con struct `Service` (campos: `repo Repository`, `logger *zap.Logger`), constructor `NewService(repo, logger)` y método `ListPermissions(ctx, tenantID uuid.UUID) ([]*domain.Permission, error)` — llama `repo.List` y loguea error con Zap si falla
- [x] T007 [US1] Crear `internal/api/handler/permissions/handler.go` con:
  - Struct `Handler` (campos: `service *permissions.Service`, `logger *zap.Logger`) y constructor `NewHandler`
  - DTO `PermissionResponse` (campos JSON: `id`, `name`, `section`, `description`, `isSystemPermission`, `createdAt` omitempty, `updatedAt` omitempty)
  - Función `toPermissionResponse(p *domain.Permission) PermissionResponse`
  - Handler `ListPermissions(c *gin.Context)`: extrae tenant_id del contexto via `platform.TenantID`, llama service, responde 200 con array JSON; si el array es nil retorna `[]` vacío
- [x] T008 [US1] Actualizar `internal/routes/url_mappings.go`:
  - Agregar imports para `permissionsHandler`, `permissionsApp`, `permissionsRepo`
  - Instanciar `permissionsRepo.NewPostgresRepository(db)`
  - Instanciar `permissionsApp.NewService(permissionsRepo, logger)`
  - Instanciar `permissionsHandler.NewHandler(permissionsApp, logger)`
  - Registrar `GET /permissions` en el grupo protegido `/api/v1` (con JWTAuth + TenantFromHeader, sin RBACCheck extra — cualquier usuario autenticado puede listar)
- [ ] T009 [US1] Verificar US1 con quickstart.md paso 1: listar permisos retorna ≥ 17 items, `perm_dashboard` existe con `isSystemPermission: true`, sin auth retorna 401

**Checkpoint**: `GET /api/v1/permissions` completamente funcional. MVP operativo.

---

## Phase 4: User Story 2 — Crear permiso custom (Priority: P2)

**Goal**: `POST /api/v1/permissions` permite a un admin crear permisos custom. Valida nombre ≥ 3 chars y sección + descripción presentes. Retorna 201 con el permiso creado.

**Independent Test**: `POST` con body válido → 201 con `isSystemPermission: false` y UUID en `id`. `POST` con nombre de 2 chars → 400 con `errors[0].path = "name"`.

- [x] T010 [US2] Agregar método `CreatePermission(ctx, tenantID uuid.UUID, name, section, description string) (*domain.Permission, error)` al service en `internal/app/permissions/service.go`:
  - Validar `len(name) < 3` → retornar `ErrPermissionValidationFailed` con campo "name"
  - Validar `section == ""` → retornar `ErrPermissionValidationFailed` con campo "section"
  - Validar `description == ""` → retornar `ErrPermissionValidationFailed` con campo "description"
  - Generar UUID con `uuid.New().String()` como ID del permiso
  - Construir `domain.Permission` con `IsSystemPermission: false`, `TenantID: &tenantID`
  - Llamar `repo.Create`; loguear resultado con Zap
- [x] T011 [US2] Agregar a `internal/api/handler/permissions/handler.go`:
  - DTO `CreatePermissionRequest` (campos: `Name`, `Section`, `Description` con json tags)
  - DTO `ValidationErrorResponse` (campos: `Error string`, `Errors []FieldError`) y `FieldError` (campos: `Path`, `Message`)
  - Handler `CreatePermission(c *gin.Context)`: bind JSON, llama service, responde 201; mapea `ErrPermissionValidationFailed` → 400 con `ValidationErrorResponse`; mapea 403 si se intenta crear permiso de sistema (no aplica aquí pero documentar el mapeo base de errores)
- [x] T012 [US2] Registrar `POST /permissions` con `RBACCheck("permissions:write")` o equivalente en `internal/routes/url_mappings.go` (usar el mismo mecanismo RBAC que otros endpoints admin del proyecto)
- [ ] T013 [US2] Verificar US2 con quickstart.md paso 3: crear permiso válido → 201 con UUID, nombre corto → 400, sección ausente → 400

**Checkpoint**: `POST /api/v1/permissions` funcional. Admin puede crear permisos custom.

---

## Phase 5: User Story 3 — Consultar permiso por ID (Priority: P2)

**Goal**: `GET /api/v1/permissions/:id` retorna el detalle de un permiso específico (de sistema o custom). 404 si no existe.

**Independent Test**: `GET /permissions/perm_dashboard` → 200 con datos del permiso. `GET /permissions/nonexistent-perm` → 404.

- [x] T014 [US3] Agregar método `GetPermission(ctx context.Context, id string) (*domain.Permission, error)` al service en `internal/app/permissions/service.go`: llama `repo.GetByID`; no loguea `ErrPermissionNotFound` (es flujo normal); loguea otros errores
- [x] T015 [US3] Agregar handler `GetPermission(c *gin.Context)` a `internal/api/handler/permissions/handler.go`: extrae `id` de path param, llama service, responde 200; mapea `ErrPermissionNotFound` → 404 `{"error": "Permission not found"}`
- [x] T016 [US3] Registrar `GET /permissions/:id` en `internal/routes/url_mappings.go` (solo JWTAuth + TenantFromHeader, sin RBAC extra — cualquier usuario autenticado puede consultar)
- [ ] T017 [US3] Verificar US3 con quickstart.md paso 2: `perm_dashboard` retorna 200, `nonexistent-perm` retorna 404

**Checkpoint**: `GET /api/v1/permissions/:id` funcional. Listado + detalle completos.

---

## Phase 6: User Story 4 — Actualizar permiso custom (Priority: P3)

**Goal**: `PUT /api/v1/permissions/:id` actualiza nombre, sección y descripción de permisos custom. Retorna 403 si se intenta modificar un permiso de sistema.

**Independent Test**: `PUT` sobre permiso custom → 200 con datos actualizados. `PUT` sobre `perm_dashboard` → 403 `{"error": "Cannot modify system permissions"}`.

- [x] T018 [US4] Agregar método `UpdatePermission(ctx context.Context, id, name, section, description string) (*domain.Permission, error)` al service en `internal/app/permissions/service.go`:
  - Llamar `repo.GetByID` primero; retornar `ErrPermissionNotFound` si no existe
  - Si `p.IsSystemPermission == true` → retornar `ErrPermissionIsSystem`
  - Validar `name`, `section`, `description` (mismas reglas que Create)
  - Actualizar campos en el objeto y llamar `repo.Update`; loguear con Zap
- [x] T019 [US4] Agregar a `internal/api/handler/permissions/handler.go`:
  - DTO `UpdatePermissionRequest` (campos: `Name`, `Section`, `Description` con json tags)
  - Handler `UpdatePermission(c *gin.Context)`: bind JSON, extrae `id` de path, llama service, responde 200; mapea `ErrPermissionIsSystem` → 403 `{"error": "Cannot modify system permissions"}`; mapea `ErrPermissionNotFound` → 404; mapea `ErrPermissionValidationFailed` → 400
- [x] T020 [US4] Registrar `PUT /permissions/:id` con RBAC admin en `internal/routes/url_mappings.go`
- [ ] T021 [US4] Verificar US4 con quickstart.md paso 4: actualizar permiso custom → 200 con `updatedAt` renovado, intentar modificar `perm_dashboard` → 403, ID inexistente → 404

**Checkpoint**: `PUT /api/v1/permissions/:id` funcional. CRUD de permisos custom completo excepto delete.

---

## Phase 7: User Story 5 — Eliminar permiso custom (Priority: P3)

**Goal**: `DELETE /api/v1/permissions/:id` elimina permanentemente permisos custom. Retorna 403 para permisos de sistema.

**Independent Test**: `DELETE` sobre permiso custom → 200 `{"success": true}`. Verificar que ya no aparece en el listado. `DELETE` sobre `perm_dashboard` → 403.

- [x] T022 [US5] Agregar método `DeletePermission(ctx context.Context, id string) error` al service en `internal/app/permissions/service.go`:
  - Llamar `repo.GetByID` primero; retornar `ErrPermissionNotFound` si no existe
  - Si `p.IsSystemPermission == true` → retornar `ErrPermissionIsSystem`
  - Llamar `repo.Delete`; loguear con Zap
- [x] T023 [US5] Agregar handler `DeletePermission(c *gin.Context)` a `internal/api/handler/permissions/handler.go`: extrae `id` de path, llama service, responde 200 `{"success": true}`; mapea `ErrPermissionIsSystem` → 403 `{"error": "Cannot delete system permissions"}`; mapea `ErrPermissionNotFound` → 404
- [x] T024 [US5] Registrar `DELETE /permissions/:id` con RBAC admin en `internal/routes/url_mappings.go`
- [ ] T025 [US5] Verificar US5 con quickstart.md paso 5: eliminar permiso custom → 200, ya no aparece en GET, intentar eliminar `perm_dashboard` → 403

**Checkpoint**: CRUD completo. Los 5 endpoints están operativos. Todas las user stories implementadas.

---

## Phase 8: Polish — Observabilidad, Postman y cierre Pact

**Propósito**: Completar observabilidad (requisito no-negociable de la Constitución §III), documentación Postman y cierre del ciclo Pact.

- [x] T026 [P] Agregar contadores Prometheus en `internal/api/handler/permissions/handler.go`:
  - Registrar en `init()`: `permissions_requests_total` counter vec con labels `{method, status}` y `permissions_list_duration_seconds` histograma
  - Instrumentar `ListPermissions` con el histograma de latencia
  - Instrumentar todos los handlers con el counter de requests
- [x] T027 [P] Crear `postman/Permissions-API.postman_collection.json` con los 10 Pact scenarios del `quickstart.md`: lista con sistema, create válido, create nombre corto → 400, get por id, get inexistente → 404, update custom, update sistema → 403, delete custom, delete sistema → 403, get sin auth → 401
- [ ] T028 Ejecutar el checklist completo del `quickstart.md` (los 10 scenarios de verificación Pact) contra el servidor local y confirmar que todos pasan
- [ ] T029 Verificar aislamiento multi-tenant con quickstart.md paso 6: permiso custom de tenant A no visible para tenant B
- [x] T030 Actualizar `PACTS_ANALYSIS.md`: marcar `permissions-service-api` como ✅ completado (10 interacciones), actualizar métricas de cobertura (de ~54% a ~61%)

**Checkpoint final**: Los 10 Pact scenarios pasan, observabilidad activa, `PACTS_ANALYSIS.md` actualizado. Feature lista para PR.

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (migración)      → sin dependencias — comenzar aquí
Phase 2 (domain + repo)  → depende de Phase 1 (necesita compilar contra domain)
Phase 3 (US1)            → depende de Phase 2 — PRIMER MVP entregable
Phase 4 (US2)            → depende de Phase 2; independiente de Phase 3
Phase 5 (US3)            → depende de Phase 2; independiente de Phase 3, 4
Phase 6 (US4)            → depende de Phase 2; independiente de Phase 3, 4, 5
Phase 7 (US5)            → depende de Phase 2; independiente de Phase 3, 4, 5, 6
Phase 8 (Polish)         → depende de Phases 3–7 completas
```

### User Story Dependencies

- **US1 (P1 — Listar)**: Solo necesita Phase 2. No depende de ninguna otra US.
- **US2 (P2 — Crear)**: Solo necesita Phase 2. Independiente de US1 (aunque para probar crear hay que listar).
- **US3 (P2 — Get por ID)**: Solo necesita Phase 2. Independiente de US1 y US2.
- **US4 (P3 — Actualizar)**: Solo necesita Phase 2. Guard `IsSystemPermission` reutiliza repo.GetByID (ya implementado).
- **US5 (P3 — Eliminar)**: Solo necesita Phase 2. Mismo patrón que US4.

### Dentro de cada User Story

```
Service method → Handler → Wiring → Verificación
```

### Oportunidades de paralelismo

- **T001 y T002** (migration up/down): archivos distintos, 100% paralelas
- **Phase 3–7**: Las US son independientes entre sí en términos de archivos de negocio. Sin embargo, todas modifican `service.go`, `handler.go` y `url_mappings.go` → **secuencial dentro del mismo archivo**, paralelas si se distribuyen entre agentes distintos con archivos propios
- **T026 y T027** (Prometheus + Postman): archivos distintos, paralelas
- **T028–T030** (verificaciones): secuenciales (T028 debe pasar antes de T029 y T030)

---

## Parallel Example: Phase 1

```bash
# T001 y T002 pueden ejecutarse simultáneamente:
Task T001: "Crear migrations/000017_create_permissions_table.up.sql con CREATE TABLE + seed"
Task T002: "Crear migrations/000017_create_permissions_table.down.sql con DROP TABLE"
```

## Parallel Example: Phase 8

```bash
# T026 y T027 pueden ejecutarse simultáneamente:
Task T026: "Agregar Prometheus counters en internal/api/handler/permissions/handler.go"
Task T027: "Crear postman/Permissions-API.postman_collection.json con 10 Pact scenarios"
```

---

## Implementation Strategy

### MVP First (User Story 1 solamente)

1. Completar Phase 1: Migración (T001–T003)
2. Completar Phase 2: Domain + Repository (T004–T005)
3. Completar Phase 3: US1 — Listar permisos (T006–T009)
4. **PARAR Y VALIDAR**: `GET /api/v1/permissions` retorna los 17 permisos de sistema + custom del tenant
5. El frontend ya puede consultar el catálogo para renderizar la pantalla de roles

### Entrega Incremental

| Fase | Entrega | Valor |
|------|---------|-------|
| Phase 1 + 2 + 3 | Listado de permisos | Frontend puede renderizar catálogo |
| + Phase 4 | Crear permisos custom | Admin puede extender el RBAC |
| + Phase 5 | Get por ID | Frontend puede mostrar detalle |
| + Phase 6 | Actualizar custom | Admin puede corregir errores |
| + Phase 7 | Eliminar custom | Ciclo CRUD completo |
| + Phase 8 | Polish + Pact | Feature lista para merge |

---

## Notes

- **T003 es un paso manual** — no puede ejecutarse por el agente sin DATABASE_URL configurada en el entorno
- Las migraciones siguen numeración estricta: `000017` es el siguiente disponible después de `000016_create_notifications_table`
- El ID de permisos custom se genera en el **service** (no en la BD): `uuid.New().String()` — esto permite que los IDs de permisos de sistema sean strings como `perm_dashboard` (no UUIDs)
- La query de listado usa `tenant_id = $1 OR is_system_permission = TRUE` — mismo patrón que `roles/repository.go`
- `repo.GetByID` no filtra por tenant: permite acceder a permisos de sistema (tenant_id NULL) sin pasar tenant_id
- El mecanismo RBAC para mutaciones usa el mismo patrón que `roles/` y `alarm_rules/` en el proyecto
- Commits recomendados: uno por fase completada (migración, domain+repo, cada US, polish)
