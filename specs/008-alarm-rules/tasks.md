# Tasks: Alarm Rules Service API (008)

**Input**: Design documents from `/specs/008-alarm-rules/`  
**Branch**: `008-alarm-rules`  
**Total tasks**: 18  
**Tests**: No incluidos (no solicitados en la spec)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Puede ejecutarse en paralelo (archivos distintos, sin dependencias incompletas)
- **[Story]**: User story a la que pertenece la tarea (US1–US5)

---

## Phase 1: Setup (Estructura de paquetes)

**Purpose**: Crear los directorios de los paquetes nuevos antes de escribir código.

- [x] T001 Crear estructura de directorios: `internal/domain/` (ya existe), `internal/repo/pg/alarm_rules/`, `internal/app/alarm_rules/`, `internal/api/handler/alarm_rules/dto/`

---

## Phase 2: Foundational (Bloqueante para todas las US)

**Purpose**: Migración, domain types, repositorio y servicio — base obligatoria antes de cualquier handler.

**⚠️ CRÍTICO**: Ninguna user story puede comenzar hasta que esta fase esté completa.

- [x] T00X Crear migración up `migrations/000014_create_alarm_rules_table.up.sql` con tabla `alarm_rules`, CHECK constraints en `operator` y `severity`, índices y trigger `updated_at`
- [x] T00X [P] Crear migración down `migrations/000014_create_alarm_rules_table.down.sql` con `DROP TABLE IF EXISTS alarm_rules`
- [x] T00X [P] Crear domain types en `internal/domain/alarm_rules.go`: struct `AlarmRule`, `ErrAlarmRuleNotFound`, constantes `ValidOperators` y `ValidSeverities`, funciones `ValidateOperator` y `ValidateSeverity`
- [x] T00X Crear repositorio en `internal/repo/pg/alarm_rules/repository.go`: interface `Repository` (List, GetByID, Create, Update, Delete) + `PostgresRepository` con todas las queries; todas incluyen `tenant_id` para aislamiento multi-tenant
- [x] T00X Crear servicio en `internal/app/alarm_rules/service.go`: `Service` con `repo Repository` + `logger *zap.Logger`; métodos `ListAlarmRules`, `GetAlarmRule`, `CreateAlarmRule`, `UpdateAlarmRule`, `DeleteAlarmRule`; validación de `operator` y `severity` en Create/Update; logging Zap en cada operación

**Checkpoint**: Compilar con `docker run ... go build ./...` — foundation lista.

---

## Phase 3: User Story 1 — Listar reglas de alarma (Priority: P1) 🎯 MVP

**Goal**: `GET /api/v1/alarm-rules` devuelve la lista de reglas del tenant o `[]` si no hay ninguna. Primer Pact validado.

**Independent Test**: `curl -s "$BASE_URL/alarm-rules" -H "Authorization: Bearer $JWT" -H "X-Tenant-ID: $TENANT_ID"` → 200 con `{"success":true,"data":[]}`.

- [x] T00X [US1] Crear `internal/api/handler/alarm_rules/dto/response.go`: struct `AlarmRuleResponse` con todos los campos de `AlarmRule` + función `FromDomain(r *domain.AlarmRule) AlarmRuleResponse` con conversión de campos a camelCase JSON
- [x] T00X [US1] Crear `internal/api/handler/alarm_rules/list_alarm_rules.go`: handler `ListAlarmRules(service *appAlarmRules.Service) gin.HandlerFunc`; extrae `tenant_id` desde context via `platform.TenantID`; responde `{"success":true,"data":[...]}` en 200
- [x] T0\1 [US1] Crear `internal/api/handler/alarm_rules/routes.go` con `RegisterRoutes(readGroup, writeGroup *gin.RouterGroup, service *appAlarmRules.Service)` registrando `GET /alarm-rules` en `readGroup`
- [x] T0\1 [US1] Actualizar `internal/api/router.go`: inyectar `alarm_rules.NewPostgresRepository(pool)`, `alarm_rules.NewService(repo, logger)`, llamar `alarm_rules.RegisterRoutes(readGroup, writeGroup, service)`

**Checkpoint**: Ejecutar Pacts 1, 2 y 5 del quickstart.md.

---

## Phase 4: User Story 2 — Crear una regla de alarma (Priority: P1)

**Goal**: `POST /api/v1/alarm-rules` crea una regla válida (201) o devuelve 400 con detalle de validación.

**Independent Test**: `curl -X POST "$BASE_URL/alarm-rules" -d '{...válido...}'` → 201 con objeto creado; `curl -X POST` con body incompleto → 400 con `VALIDATION_ERROR`.

- [x] T0\1 [US2] Crear `internal/api/handler/alarm_rules/dto/request.go`: structs `CreateAlarmRuleRequest` (campos requeridos: name, metric, operator, threshold, severity; opcional: description, enabled) y `UpdateAlarmRuleRequest` (todos opcionales via punteros)
- [x] T0\1 [US2] Crear `internal/api/handler/alarm_rules/create_alarm_rule.go`: handler `CreateAlarmRule`; bindea JSON, delega a `service.CreateAlarmRule`, mapea `ErrAlarmRuleNotFound` → 404 y errores de validación de domain → 400 con código `VALIDATION_ERROR`; responde 201
- [x] T0\1 [US2] Agregar `POST /alarm-rules` en `routes.go` dentro de `writeGroup`

**Checkpoint**: Ejecutar Pacts 3 y 4 del quickstart.md.

---

## Phase 5: User Story 3 — Obtener regla por ID (Priority: P2)

**Goal**: `GET /api/v1/alarm-rules/:id` devuelve la regla (200) o 404 si no existe o es de otro tenant.

**Independent Test**: `curl "$BASE_URL/alarm-rules/$RULE_ID"` → 200; `curl "$BASE_URL/alarm-rules/00000000-..."` → 404 con `NOT_FOUND`.

- [x] T0\1 [US3] Crear `internal/api/handler/alarm_rules/get_alarm_rule.go`: handler `GetAlarmRule`; parsea `:id` como UUID, llama `service.GetAlarmRule`, mapea `ErrAlarmRuleNotFound` → 404; responde 200 con objeto
- [x] T0\1 [US3] Agregar `GET /alarm-rules/:id` en `routes.go` dentro de `readGroup`

**Checkpoint**: Ejecutar Pacts 6 y 7 del quickstart.md.

---

## Phase 6: User Story 4 — Modificar regla existente (Priority: P2)

**Goal**: `PATCH /api/v1/alarm-rules/:id` actualiza solo los campos enviados (200) o devuelve 404 / 400.

**Independent Test**: `curl -X PATCH "$BASE_URL/alarm-rules/$RULE_ID" -d '{"threshold":85.0}'` → 200 con `threshold` actualizado y resto sin cambios.

- [x] T0\1 [US4] Crear `internal/api/handler/alarm_rules/update_alarm_rule.go`: handler `UpdateAlarmRule`; bindea `UpdateAlarmRuleRequest` (campos opcionales via punteros), llama `service.UpdateAlarmRule` pasando solo los campos presentes; mapea errores de dominio → 400/404; responde 200
- [x] T0\1 [US4] Agregar `PATCH /alarm-rules/:id` en `routes.go` dentro de `writeGroup`

**Checkpoint**: Ejecutar Pacts 8 y 9 del quickstart.md.

---

## Phase 7: User Story 5 — Eliminar una regla (Priority: P3)

**Goal**: `DELETE /api/v1/alarm-rules/:id` elimina permanentemente (200) o devuelve 404.

**Independent Test**: Crear regla → DELETE → `curl "$BASE_URL/alarm-rules/$RULE_ID"` → 404.

- [x] T0\1 [US5] Crear `internal/api/handler/alarm_rules/delete_alarm_rule.go`: handler `DeleteAlarmRule`; parsea `:id`, llama `service.DeleteAlarmRule`, mapea `ErrAlarmRuleNotFound` → 404; responde `{"success":true}` en 200
- [x] T0\1 [US5] Agregar `DELETE /alarm-rules/:id` en `routes.go` dentro de `writeGroup`

**Checkpoint**: Ejecutar Pacts 10 y 11 del quickstart.md. ✅ Todos los Pacts completos.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Observabilidad, documentación y validación final.

- [ ] T020 [P] Aplicar migración manualmente: `migrate -path migrations/ -database $DATABASE_URL up 1` (migration 000014)
- [ ] T021 [P] Agregar carpeta `Alarm Rules` en `postman/Embolsadora-API-Complete.postman_collection.json` con los 10 requests Pact (GET list, GET list 401, GET by id 200, GET by id 404, POST 201, POST 400, PATCH 200, PATCH 404, DELETE 200, DELETE 404)
- [ ] T022 Ejecutar todos los curls del quickstart.md y marcar checklist de validación Pact

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Sin dependencias — comenzar inmediatamente
- **Foundational (Phase 2)**: Depende de Phase 1 — **bloquea todas las US**
- **US1 (Phase 3)**: Depende de Phase 2 — primera en implementar (habilita testing básico)
- **US2 (Phase 4)**: Depende de Phase 2; puede arrancar en paralelo con US1 (archivos distintos), pero necesita routes.go de US1
- **US3 (Phase 5)**: Depende de Phase 2 y US1 (usa `routes.go`)
- **US4 (Phase 6)**: Depende de Phase 2 y US1 (usa `routes.go`); puede ir en paralelo con US3
- **US5 (Phase 7)**: Depende de Phase 2 y US1 (usa `routes.go`)
- **Polish (Phase 8)**: Depende de todas las US completadas

### User Story Dependencies

- **US1 (P1)**: Solo depende de Foundational — crea la infraestructura de routing
- **US2 (P1)**: Depende de Foundational + `routes.go` de US1
- **US3 (P2)**: Depende de Foundational + `routes.go` de US1 (no depende de US2)
- **US4 (P2)**: Depende de Foundational + `routes.go` de US1 + `dto/request.go` de US2
- **US5 (P3)**: Depende de Foundational + `routes.go` de US1 (no depende de US2, US3, US4)

### Parallel Opportunities

- T002, T003, T004 pueden ir en paralelo (archivos distintos)
- T003 y T004 son paralelas entre sí (independientes)
- Una vez completa la Foundational phase: US3 y US5 pueden desarrollarse en paralelo
- T021 (Postman) puede hacerse en paralelo con T022

---

## Parallel Example: Phase 2 (Foundational)

```text
En paralelo:
  T003 migrations/000014_create_alarm_rules_table.down.sql
  T004 internal/domain/alarm_rules.go

Secuencial (T002 → T005 → T006):
  T002 migration up (necesaria para T005 para testear queries)
  T005 internal/repo/pg/alarm_rules/repository.go
  T006 internal/app/alarm_rules/service.go
```

---

## Implementation Strategy

### MVP (US1 + US2 solamente — Pacts 1–5)

1. Completar Phase 1: Setup
2. Completar Phase 2: Foundational
3. Completar Phase 3: US1 (GET list)
4. Completar Phase 4: US2 (POST create)
5. **STOP & VALIDAR**: curls Pacts 1–5 del quickstart.md
6. Continuar con US3–US5

### Incremental Delivery

1. Setup + Foundational → base lista
2. US1 → GET list funcional (Pacts 1, 2, 5)
3. US2 → POST create funcional (Pacts 3, 4)
4. US3 → GET by id funcional (Pacts 6, 7)
5. US4 → PATCH update funcional (Pacts 8, 9)
6. US5 → DELETE funcional (Pacts 10, 11) → **10/10 Pacts completos**

---

## Notes

- Migración: siguente número disponible es 000014
- Pattern de repo: seguir `internal/repo/pg/roles/repository.go` exactamente
- Pattern de service: seguir `internal/app/roles/service.go`
- Pattern de handler: seguir `internal/api/handler/roles/list_roles.go`
- `UpdateAlarmRuleRequest` usa punteros (`*string`, `*float64`, `*bool`) para distinguir "campo no enviado" de "campo enviado con valor cero"
- RBAC: writes con `users:write` (igual que roles — ver `routes.go` de roles para referencia)
- Eliminación permanente: no soft-delete, usar `DELETE FROM alarm_rules WHERE id = $1 AND tenant_id = $2`
