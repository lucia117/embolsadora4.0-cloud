# Tareas: API de Gestión de Roles

**Input**: Documentos de diseño en `/specs/006-roles-management/`
**Prerequisitos**: plan.md, spec.md, data-model.md, contracts/, research.md, quickstart.md

**Organización**: Las tareas están agrupadas por historia de usuario para permitir implementación y prueba independiente de cada una.

## Formato: `[ID] [P?] [Story?] Descripción con ruta de archivo`

- **[P]**: Puede ejecutarse en paralelo (archivos distintos, sin dependencias incompletas)
- **[Story]**: Historia de usuario a la que pertenece la tarea (US1–US5)
- Las rutas de archivo son absolutas desde la raíz del repositorio

---

## Fase 1: Fundacional (Prerequisitos Bloqueantes)

**Propósito**: Migración de BD, dominio, repositorio, servicio y DTOs — todo lo que cualquier historia de usuario necesita antes de poder implementar su handler.

**⚠️ CRÍTICO**: Ninguna historia de usuario puede comenzar hasta que esta fase esté completa.

- [X] T001 Crear migración `migrations/000012_extend_roles_table.up.sql` — agregar columnas `is_system_role`, `is_global`, `tenant_id`, `permissions`, `updated_at`, `deleted_at` a la tabla `roles`; UPDATE para marcar los 4 roles del sistema; índices `idx_roles_tenant_active` e `idx_roles_tenant_name_active` (ver data-model.md para el SQL exacto)
- [X] T002 Crear migración `migrations/000012_extend_roles_table.down.sql` — revertir columnas e índices agregados en T001
- [X] T003 [P] Crear entidad de dominio `internal/domain/roles.go` — struct `Role` con campos ID, Name, Description, Permissions, IsSystemRole, IsGlobal, TenantID, CreatedAt, UpdatedAt, DeletedAt; constante `MaxCustomRolesPerTenant = 3`; errores `ErrRoleNotFound`, `ErrRoleIsSystemRole`, `ErrRoleHasAssignments`, `ErrRoleDuplicateName`, `ErrRoleLimitReached`
- [X] T004 [P] Crear repositorio PostgreSQL `internal/repo/pg/roles/repository.go` — interfaz `Repository` con métodos List, GetByID, CountCustomByTenant, Create, Update, SoftDelete, CountActiveAssignments; implementación `PostgresRepository` con pool pgx/v5; función `scanRole` auxiliar (ver data-model.md para queries SQL)
- [X] T005 Crear servicio de aplicación `internal/app/roles/service.go` — struct `Service` con repo y logger Zap; métodos ListRoles, GetRole, CreateRole (genera ID `custom_<6 hex>`, deduplica permisos, verifica límite), UpdateRole (rechaza roles del sistema), DeleteRole (rechaza sistema + asignaciones activas) — depende de T003 y T004
- [X] T006 [P] Crear DTO de request `internal/api/handler/roles/dto/request.go` — structs `CreateRoleRequest` (name required max:100, description max:500, permissions []string) y `UpdateRoleRequest` (mismos campos)
- [X] T007 [P] Crear DTO de response `internal/api/handler/roles/dto/response.go` — struct `RoleResponse` con todos los campos camelCase (id, name, description, permissions, isSystemRole, isGlobal, tenantId, createdAt, updatedAt); función `FromDomain(role *domain.Role) RoleResponse`

**Checkpoint**: Fase fundacional completa — se puede iniciar cualquier historia de usuario.

---

## Fase 2: Historia de Usuario 1 — Listar Roles (Prioridad: P1) 🎯 MVP

**Meta**: Un administrador autenticado puede ver todos los roles disponibles para su tenant (4 del sistema + custom propios).

**Prueba independiente**: GET `/api/v1/roles` con JWT válido y `X-Tenant-ID` devuelve los 4 roles del sistema más cualquier role custom existente en el tenant.

- [X] T008 [US1] Crear handler `internal/api/handler/roles/list_roles.go` — `ListRoles(service)` gin.HandlerFunc: extrae tenantID del contexto via `platform.TenantID`, llama `service.ListRoles`, devuelve `{"success": true, "data": [...]}` con 200
- [X] T009 [US1] Crear archivo de rutas `internal/api/handler/roles/routes.go` — `RegisterRoutes(g *gin.RouterGroup, service *appRoles.Service)` con `GET /roles` → ListRoles
- [X] T010 [US1] Registrar en `internal/routes/url_mappings.go` — instanciar `rolesRepo.NewPostgresRepository(db)`, `rolesApp.NewService(rRepo, logger)`; llamar `rolesHandler.RegisterRoutes(v1, rService)` en el grupo v1
- [X] T011 [US1] Aplicar migration 000012 con `migrate -path migrations/ -database "..." up 1` y compilar con Docker (`go build ./...`); verificar que GET /roles devuelve los 4 roles del sistema

**Checkpoint**: US1 completa — listado de roles funcional y verificado.

---

## Fase 3: Historia de Usuario 2 — Crear Rol Personalizado (Prioridad: P1)

**Meta**: Un administrador puede crear un nuevo rol con nombre, descripción y permisos, respetando el límite de 3 y unicidad de nombre.

**Prueba independiente**: POST `/api/v1/roles` con body válido devuelve 201 con el rol creado; nombre duplicado devuelve 409; cuarto rol devuelve 409.

- [X] T012 [US2] Crear handler `internal/api/handler/roles/create_role.go` — `CreateRole(service)` gin.HandlerFunc: parsea `CreateRoleRequest` con binding (400 si inválido), extrae tenantID del contexto, llama `service.CreateRole`, mapea errores de dominio a HTTP, devuelve 201 con `{"success": true, "data": role}`
- [X] T013 [US2] Agregar ruta `POST /roles` en `internal/api/handler/roles/routes.go`

**Checkpoint**: US2 completa — creación de roles funcional con todas las validaciones.

---

## Fase 4: Historia de Usuario 3 — Ver Rol por ID (Prioridad: P2)

**Meta**: Un usuario autenticado puede obtener el detalle completo de un rol por su ID.

**Prueba independiente**: GET `/api/v1/roles/:id` con ID válido devuelve 200; ID inexistente devuelve 404.

- [X] T014 [US3] Crear handler `internal/api/handler/roles/get_role.go` — `GetRole(service)` gin.HandlerFunc: extrae `id` del path param, llama `service.GetRole`, mapea `ErrRoleNotFound`→404, devuelve 200 con `{"success": true, "data": role}`
- [X] T015 [US3] Agregar ruta `GET /roles/:id` en `internal/api/handler/roles/routes.go`

**Checkpoint**: US3 completa — consulta individual de rol funcional.

---

## Fase 5: Historia de Usuario 4 — Actualizar Rol Personalizado (Prioridad: P2)

**Meta**: Un administrador puede modificar nombre, descripción y permisos de un rol custom; los roles del sistema no pueden modificarse.

**Prueba independiente**: PUT `/api/v1/roles/:id` con body válido devuelve 200; intentar actualizar rol del sistema devuelve 403; nombre duplicado devuelve 409; ID inexistente devuelve 404.

- [X] T016 [US4] Crear handler `internal/api/handler/roles/update_role.go` — `UpdateRole(service)` gin.HandlerFunc: parsea `UpdateRoleRequest`, extrae path param `id`, llama `service.UpdateRole`, mapea errores de dominio, devuelve 200 con rol actualizado
- [X] T017 [US4] Agregar ruta `PUT /roles/:id` en `internal/api/handler/roles/routes.go`

**Checkpoint**: US4 completa — actualización de roles funcional con protección de roles del sistema.

---

## Fase 6: Historia de Usuario 5 — Eliminar Rol Personalizado (Prioridad: P2)

**Meta**: Un administrador puede eliminar un rol custom sin asignaciones; roles del sistema y roles con usuarios asignados son rechazados con el código de error correcto.

**Prueba independiente**: DELETE `/api/v1/roles/:id` sin asignaciones → 200; rol del sistema → 403; rol con usuarios → 409 + `usersAffected`; ID inexistente → 404.

- [X] T018 [US5] Crear handler `internal/api/handler/roles/delete_role.go` — `DeleteRole(service)` gin.HandlerFunc: extrae path param `id`, llama `service.DeleteRole`, mapea `ErrRoleIsSystemRole`→403, `ErrRoleHasAssignments`→409 con campo `usersAffected`, `ErrRoleNotFound`→404; devuelve 200 `{"success": true}` en éxito
- [X] T019 [US5] Agregar ruta `DELETE /roles/:id` en `internal/api/handler/roles/routes.go`

**Checkpoint**: US5 completa — eliminación de roles funcional con todas las salvaguardas.

---

## Fase 7: Polish y Cierre

**Propósito**: Verificación final, documentación y commit.

- [X] T020 Compilar el proyecto completo con Docker `go build ./...` — sin errores de compilación
- [X] T021 [P] Actualizar `PACTS_ANALYSIS.md` — sección `role-service-api` con 7/7 interacciones marcadas ✅
- [ ] T022 Validar los 7 escenarios Pact descritos en `specs/006-roles-management/quickstart.md` contra el servidor corriendo localmente
- [ ] T023 Commit y push de todos los archivos creados/modificados en esta feature

---

## Dependencias y Orden de Ejecución

### Dependencias entre Fases

- **Fase 1 (Fundacional)**: Sin dependencias externas — empezar aquí
- **Fase 2 (US1)**: Depende de Fase 1 completa — BLOQUEA el registro de rutas
- **Fases 3–6 (US2–US5)**: Dependen de Fase 1; se pueden hacer en cualquier orden después de US1 (las rutas ya están registradas)
- **Fase 7 (Polish)**: Depende de todas las historias deseadas completas

### Dependencias dentro de la Fase 1

```
T001 (migration up)  ──┐
T002 (migration down)  │ independientes entre sí
T003 (domain)        ──┤
T004 (repo)          ──┘
          ↓
T005 (service) — depende de T003 + T004
          ↓
T006 [P] (dto request)  ┐ independientes entre sí
T007 [P] (dto response) ┘ — dependen de T003
```

### Oportunidades de Paralelismo

- T003, T004 pueden ejecutarse juntos (archivos distintos)
- T006, T007 pueden ejecutarse juntos (archivos distintos)
- T008 (list handler) y T012 (create handler) pueden ejecutarse juntos si Fase 1 está completa
- T014, T016, T018 (handlers get/update/delete) pueden ejecutarse juntos

---

## Estrategia de Implementación

### MVP Mínimo (US1 solamente)

1. Completar Fase 1 (T001–T007)
2. Completar Fase 2/US1 (T008–T011)
3. **PARAR Y VALIDAR**: GET /roles devuelve los 4 roles del sistema
4. El admin ya puede ver qué roles existen — valor inmediato

### Entrega Incremental (recomendada)

1. Fase 1 + US1 → validar listado → commit
2. US2 → validar creación → commit
3. US3 + US4 + US5 en paralelo → validar escenarios de error → commit final
4. Polish → commit + push

---

## Notas

- [P] = archivos distintos, sin dependencias incompletas — pueden lanzarse en paralelo
- [USn] = trazabilidad hacia la historia de usuario en spec.md
- El RBAC estático en `security/rbac.go` **no se modifica** — los nuevos roles custom no afectan la autorización en tiempo de request (scope de feature futura)
- La migration 000012 extiende la tabla existente — no reemplaza datos pre-existentes
- La generación de ID usa `crypto/rand` (no `math/rand`) para garantizar aleatoriedad
- Usar `MSYS_NO_PATHCONV=1` antes de comandos Docker en Windows/Git Bash
