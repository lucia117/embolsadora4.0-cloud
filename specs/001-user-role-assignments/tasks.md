# Tasks: User Role Assignment Management

**Input**: Design documents from `/specs/001-user-role-assignments/`
**Prerequisites**: plan.md ✅ | spec.md ✅ | research.md ✅ | data-model.md ✅ | contracts/ ✅

**Tests**: Not included — not explicitly requested in the feature specification.

**Organization**: Tasks grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies between them)
- **[Story]**: User story this task belongs to ([US1]–[US6])

---

## Phase 1: Setup (Migration)

**Purpose**: Create database schema — required before any code references the new tables.

- [x] T001 [P] Create migration UP file `migrations/000003_create_roles_and_user_tenant_roles.up.sql` — roles catalog + user_tenant_roles table + partial unique index + seed data
- [x] T002 [P] Create migration DOWN file `migrations/000003_create_roles_and_user_tenant_roles.down.sql` — drop indexes then tables in reverse order

**Checkpoint**: Run `migrate up` and verify `roles` and `user_tenant_roles` tables exist in the database

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Domain types, errors, repository — shared infrastructure that MUST be complete before any user story can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T003 [P] Create domain types in `internal/domain/user_roles.go` — `UserRoleStatus` type + constants (`active`/`pending`/`revoked`), `UserTenantRole` struct (all fields with nullable pointers), `UserRoleWithContext` struct (for cross-tenant view with TenantName + RoleName)
- [ ] T004 [P] Add domain errors to `internal/domain/errors.go` — `ErrUserAlreadyHasActiveRole` and `ErrAssignmentNotFound`
- [ ] T005 [P] Create SQL query constants in `internal/repo/pg/user_roles/resources.go` — `FindByTenantQuery` (with optional status filter via LEFT JOIN roles), `CreateQuery`, `FindByIDQuery`, `UpdateQuery`, `RevokeQuery`, `FindByUserQuery` (JOIN tenants + roles for roleName/tenantName)
- [ ] T006 Create `UserRoleRepository` interface + full pgx implementation in `internal/repo/pg/user_roles/repository.go` — implement all 7 methods: `FindByTenant`, `FindByID`, `Create`, `Update`, `Revoke`, `BulkCreate` (transactional), `FindByUser`; catch pgconn PgError code `23505` in `Create` and `BulkCreate` → return `domain.ErrUserAlreadyHasActiveRole`; use nullable pointer helpers matching the tenant repo pattern
- [ ] T007 Wire repository into router: add `UserRoleRepo userrolesrepo.UserRoleRepository` to `Deps` struct in `internal/api/router.go`; instantiate `userRolesRepository.NewUserRoleRepository(db)` and pass it to `api.Deps` in `internal/routes/url_mappings.go`

**Checkpoint**: `go build ./...` passes — all new types compile, router compiles with new Deps field

---

## Phase 3: User Story 1 — Assign a Role to a User (Priority: P1) 🎯 MVP

**Goal**: An administrator can assign a role to a user. Returns 201 on success, 409 if the user already has an active role in that tenant.

**Independent Test**: `POST /api/v1/user-roles` with valid body → 201 with `{"success":true,"data":{...}}`; repeat same request → 409 with `{"success":false,"error":"..."}`.

- [ ] T008 Create assign_user_role use case in `internal/api/usecases/user_roles/assign_user_role/usecase.go` — define `UseCase` interface with `Execute(ctx, AssignRequest) (*domain.UserTenantRole, error)`; implementation sets `Status=active`, `AssignedAt=now()`, `AssignedBy` from context; delegates to `repo.Create`; maps `ErrUserAlreadyHasActiveRole` through (don't wrap it)
- [ ] T009 [P] Create request DTO in `internal/api/handler/user_roles/assign_user_role/models/request.go` — `AssignUserRoleRequest{UserID, TenantID, RoleID string}` with `binding:"required"` tags; `Parse(c)` helper following tenant pattern
- [ ] T010 [P] Create response DTO in `internal/api/handler/user_roles/assign_user_role/models/response.go` — `UserRoleResponse` struct with all UTR fields in camelCase JSON; `FromDomain(*domain.UserTenantRole) *UserRoleResponse`
- [ ] T011 Create handler in `internal/api/handler/user_roles/assign_user_role/assign_user_role.go` — `NewAssignUserRoleHandler(uc UseCase)`; parse request via `models.Parse`; call use case; if `errors.Is(err, domain.ErrUserAlreadyHasActiveRole)` → `c.JSON(409, gin.H{"success":false,"error":err.Error()})`; on success → `c.JSON(201, gin.H{"success":true,"data":models.FromDomain(result)})`
- [ ] T012 Register route in `internal/api/router.go`: wire `ucAssignUserRole.NewUseCase(deps.UserRoleRepo)` + `assignUserRole.NewAssignUserRoleHandler(...)` and add `g.POST("/user-roles", handler.Handle)`

**Checkpoint**: US1 fully functional — POST /api/v1/user-roles returns 201 on first call, 409 on duplicate active assignment

---

## Phase 4: User Story 2 — View Role Assignments for a Tenant (Priority: P1)

**Goal**: An administrator can list all UTR assignments for a tenant, with optional `?status=` filter.

**Independent Test**: `GET /api/v1/user-roles?tenantId=<uuid>` → 200 with `{"success":true,"data":[...]}` (empty array when no assignments); add `&status=pending` → only pending rows returned.

- [ ] T013 Create list_user_roles use case in `internal/api/usecases/user_roles/list_user_roles/usecase.go` — define `UseCase` interface with `Execute(ctx, tenantID uuid.UUID, status *string) ([]domain.UserTenantRole, error)`; implementation delegates to `repo.FindByTenant`
- [ ] T014 [P] Create response DTO in `internal/api/handler/user_roles/list_user_roles/models/response.go` — `UserRoleResponse` (same shape as assign response); `FromDomain([]domain.UserTenantRole) []UserRoleResponse`
- [ ] T015 Create handler in `internal/api/handler/user_roles/list_user_roles/list_user_roles.go` — parse `tenantId` query param as UUID (400 if missing/invalid); parse optional `status` query param; call use case; respond `c.JSON(200, gin.H{"success":true,"data":results})`
- [ ] T016 Register route in `internal/api/router.go`: wire list use case + handler and add `g.GET("/user-roles", handler.Handle)`

**Checkpoint**: US1 + US2 both functional — GET and POST /api/v1/user-roles work independently

---

## Phase 5: User Story 3 — Revoke a Role Assignment (Priority: P2)

**Goal**: An administrator can revoke a UTR assignment by ID. Record is soft-deleted (status → `revoked`), never physically removed.

**Independent Test**: `DELETE /api/v1/user-roles/:id` → 200 with `{"success":true,"data":{"id":"...","status":"revoked"}}`; verify record still exists in DB with status `revoked`.

- [ ] T017 Create revoke_user_role use case in `internal/api/usecases/user_roles/revoke_user_role/usecase.go` — define `UseCase` interface with `Execute(ctx, id uuid.UUID) (*domain.UserTenantRole, error)`; implementation delegates to `repo.Revoke`; maps nil return → `ErrAssignmentNotFound`
- [ ] T018 [P] Create response DTO in `internal/api/handler/user_roles/revoke_user_role/models/response.go` — `RevokeResponse{ID string, Status string}`; `FromDomain(*domain.UserTenantRole) RevokeResponse`
- [ ] T019 Create handler in `internal/api/handler/user_roles/revoke_user_role/revoke_user_role.go` — parse `:id` as UUID; call use case; if `ErrAssignmentNotFound` → `c.JSON(404, gin.H{"success":false,"error":err.Error()})`; on success → `c.JSON(200, gin.H{"success":true,"data":models.FromDomain(result)})`
- [ ] T020 Register route in `internal/api/router.go`: wire revoke use case + handler and add `g.DELETE("/user-roles/:id", handler.Handle)`

**Checkpoint**: US3 functional — DELETE /api/v1/user-roles/:id soft-deletes and returns revoked status

---

## Phase 6: User Story 4 — Update a Role Assignment (Priority: P2)

**Goal**: An administrator can change the `roleId` on an existing assignment without revoking and reassigning.

**Independent Test**: `PUT /api/v1/user-roles/:id` with `{"roleId":"operario"}` → 200 with updated UTR; same `:id` that doesn't exist → 404.

- [ ] T021 Create update_user_role use case in `internal/api/usecases/user_roles/update_user_role/usecase.go` — define `UseCase` interface with `Execute(ctx, id uuid.UUID, roleID string) (*domain.UserTenantRole, error)`; implementation: `FindByID` → if nil → `ErrAssignmentNotFound`; update `RoleID`, `UpdatedAt`; call `repo.Update`; return updated entity
- [ ] T022 [P] Create request DTO in `internal/api/handler/user_roles/update_user_role/models/request.go` — `UpdateUserRoleRequest{RoleID string \`json:"roleId" binding:"required"\``}`
- [ ] T023 [P] Create response DTO in `internal/api/handler/user_roles/update_user_role/models/response.go` — same `UserRoleResponse` shape; `FromDomain`
- [ ] T024 Create handler in `internal/api/handler/user_roles/update_user_role/update_user_role.go` — parse `:id` as UUID; bind JSON; call use case; handle `ErrAssignmentNotFound` → 404; on success → `c.JSON(200, gin.H{"success":true,"data":...})`
- [ ] T025 Register route in `internal/api/router.go`: wire update use case + handler and add `g.PUT("/user-roles/:id", handler.Handle)`

**Checkpoint**: US3 + US4 both functional — PUT and DELETE /api/v1/user-roles/:id work independently

---

## Phase 7: User Story 5 — Bulk Assign Roles to Multiple Users (Priority: P3)

**Goal**: An administrator can assign the same role to N users in a single all-or-nothing operation.

**Independent Test**: `POST /api/v1/user-roles/bulk` with 3 valid users → 201 with `{"assigned":3,"failed":0,"assignments":[...]}`. Include a user with existing active role → 409, no partial changes in DB.

- [ ] T026 Create bulk_assign_user_roles use case in `internal/api/usecases/user_roles/bulk_assign_user_roles/usecase.go` — define `BulkAssignRequest{UserIDs []uuid.UUID, TenantID uuid.UUID, RoleID string}` and `BulkAssignResult{Assigned int, Failed int, Assignments []domain.UserTenantRole}`; `Execute` builds UTR slice then calls `repo.BulkCreate` in transaction; catches `ErrUserAlreadyHasActiveRole` from repo → propagates as-is for 409; on success returns result with `Failed:0`
- [ ] T027 [P] Create request DTO in `internal/api/handler/user_roles/bulk_assign_user_roles/models/request.go` — `BulkAssignRequest{UserIDs []string \`json:"userIds" binding:"required,min=1"\``, TenantID, RoleID string}`; `Parse(c)` validates UUIDs
- [ ] T028 [P] Create response DTO in `internal/api/handler/user_roles/bulk_assign_user_roles/models/response.go` — `BulkAssignResponse{Assigned int, Failed int, Assignments []AssignmentSummary}`; `AssignmentSummary{ID, UserID, RoleID, Status string}`; `FromDomain`
- [ ] T029 Create handler in `internal/api/handler/user_roles/bulk_assign_user_roles/bulk_assign_user_roles.go` — parse request; call use case; if `ErrUserAlreadyHasActiveRole` → 409; on success → `c.JSON(201, gin.H{"success":true,"data":models.FromDomain(result)})`
- [ ] T030 Register route in `internal/api/router.go`: wire bulk use case + handler and add `g.POST("/user-roles/bulk", handler.Handle)` — **CRITICAL: register this BEFORE** `g.PUT("/user-roles/:id", ...)` and `g.DELETE("/user-roles/:id", ...)` to avoid Gin routing conflict

**Checkpoint**: US5 functional — POST /api/v1/user-roles/bulk creates all or nothing; verify transaction rollback on conflict

---

## Phase 8: User Story 6 — View User's Roles Across All Tenants (Priority: P3)

**Goal**: A platform administrator can retrieve all role assignments for a given user across every tenant, including tenant name and role name.

**Independent Test**: `GET /api/v1/users/:userId/roles` for a user with assignments in 2 tenants → 200 with `{"success":true,"data":[{tenantId,tenantName,roleId,roleName,status},...]}`; user with no assignments → empty array.

- [ ] T031 Create get_user_roles use case in `internal/api/usecases/user_roles/get_user_roles/usecase.go` — define `UseCase` interface with `Execute(ctx, userID uuid.UUID) ([]domain.UserRoleWithContext, error)`; implementation delegates to `repo.FindByUser` (which JOINs tenants + roles tables); add `// TODO: RBAC check — platform admin only`
- [ ] T032 [P] Create response DTO in `internal/api/handler/user_roles/get_user_roles/models/response.go` — `UserRoleContextResponse{TenantID, TenantName, RoleID, RoleName, Status string}`; `FromDomain([]domain.UserRoleWithContext) []UserRoleContextResponse`
- [ ] T033 Create handler in `internal/api/handler/user_roles/get_user_roles/get_user_roles.go` — parse `:userId` as UUID; call use case; respond `c.JSON(200, gin.H{"success":true,"data":results})`
- [ ] T034 Register route in `internal/api/router.go`: wire get_user_roles use case + handler and add `g.GET("/users/:userId/roles", handler.Handle)`

**Checkpoint**: All 6 user stories functional — full Pact contract coverage achieved

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Final validation that all stories work together and the build is clean.

- [ ] T035 [P] Run `go build ./...` from repo root and fix any compilation errors
- [ ] T036 [P] Verify Gin routing order in `internal/api/router.go`: confirm `POST /user-roles/bulk` appears before `PUT /user-roles/:id` and `DELETE /user-roles/:id`
- [ ] T037 Run migration against local PostgreSQL and verify schema: `roles` table seeded with 4 rows, `user_tenant_roles` table exists, partial unique index `idx_utr_active_unique` present
- [ ] T038 Smoke-test all 6 Pact interactions manually using Postman or curl against local server — verify request/response shapes match `specs/001-user-role-assignments/contracts/user-role-service-api.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Migration)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **Phase 3–8 (User Stories)**: All depend on Phase 2 completion; can then proceed in priority order or in parallel
- **Phase 9 (Polish)**: Depends on all desired user story phases being complete

### User Story Dependencies

| Story | Depends on | Blocks |
|-------|-----------|--------|
| US1 — Assign (P1) | Phase 2 | Nothing |
| US2 — List (P1) | Phase 2 | Nothing |
| US3 — Revoke (P2) | Phase 2 | Nothing |
| US4 — Update (P2) | Phase 2 | Nothing |
| US5 — Bulk Assign (P3) | Phase 2 | Nothing |
| US6 — Cross-tenant View (P3) | Phase 2 | Nothing |

> All user stories are independent after Phase 2. No story blocks another.

### Within Each User Story

1. Use case (depends on domain types + repo from Phase 2)
2. Request + response models (parallel, depend only on domain types)
3. Handler (depends on use case + models)
4. Route registration (depends on handler)

### Parallel Opportunities

- T001 + T002 (migration files): parallel
- T003 + T004 + T005 (domain types, errors, SQL): parallel
- T008 + T009 + T010 (US1 usecase + request model + response model): usecase parallel with models
- T013 + T014 (US2 usecase + response model): parallel
- T017 + T018 (US3 usecase + response model): parallel
- T021 + T022 + T023 (US4 usecase + models): usecase parallel with models
- T026 + T027 + T028 (US5 usecase + models): usecase parallel with models
- T031 + T032 (US6 usecase + response model): parallel
- US1 + US2 (both P1): can be developed in parallel after Phase 2
- US3 + US4 (both P2): can be developed in parallel after Phase 2
- US5 + US6 (both P3): can be developed in parallel after Phase 2

---

## Parallel Example: US1 + US2 (both P1, after Phase 2)

```
# Parallel — different packages, no cross-dependency:
Task A: T008 assign_user_role use case
Task B: T013 list_user_roles use case

# Parallel within US1:
Task C: T009 assign request model
Task D: T010 assign response model

# Sequential:
Task E: T011 assign handler     (depends on A, C, D)
Task F: T012 register POST route (depends on E)

Task G: T014 list response model
Task H: T015 list handler       (depends on B, G)
Task I: T016 register GET route  (depends on H)
```

---

## Implementation Strategy

### MVP First (US1 Only — Assign Role)

1. Phase 1: Migration
2. Phase 2: Foundational (domain + repo + wiring)
3. Phase 3: US1 — Assign role
4. **STOP and VALIDATE**: POST /api/v1/user-roles returns 201/409 correctly
5. Deploy/demo if ready

### Incremental Delivery

1. Phase 1 + 2 → Foundation ready
2. Phase 3 → US1 ✅ (assign)
3. Phase 4 → US2 ✅ (list) — adds visibility
4. Phase 5 → US3 ✅ (revoke) — adds security
5. Phase 6 → US4 ✅ (update) — adds flexibility
6. Phase 7 → US5 ✅ (bulk) — adds efficiency
7. Phase 8 → US6 ✅ (cross-tenant) — completes Pact contract
8. Phase 9 → Polish

### Parallel Team Strategy (2 developers)

After Phase 2:
- Developer A: US1 (assign) → US3 (revoke) → US5 (bulk)
- Developer B: US2 (list) → US4 (update) → US6 (cross-tenant)

---

## Notes

- No test tasks generated — not requested in spec
- `[P]` = different files, no mutual dependencies
- `[Story]` label maps each task to its user story for traceability
- Tenant module is the canonical reference for all patterns (see Design Principles in plan.md)
- Register `POST /user-roles/bulk` BEFORE `PUT/DELETE /user-roles/:id` in router.go (T030 note)
- `// TODO: RBAC check — platform admin only` required in US6 use case (T031)
- Commit after each checkpoint to enable clean rollback per story
