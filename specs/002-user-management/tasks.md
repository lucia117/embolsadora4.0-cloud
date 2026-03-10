# Tasks: User Management API Implementation

**Feature**: 002-user-management
**Branch**: `002-user-management`
**Total Estimated Effort**: 14-20 hours
**Total Tasks**: 42

---

## Overview & Execution Strategy

### User Stories (from spec.md)

| Priority | Story | Description |
|----------|-------|-------------|
| **P1** | US1 | List Users for Tenant (paginated) |
| **P1** | US3 | Create New User |
| **P2** | US2 | View Specific User Profile |
| **P2** | US4 | Update Existing User |
| **P3** | US5 | Remove User (soft delete) |

### Independent Testability

Each user story can be implemented and tested independently once the foundational layer (Phase 2) is complete. Suggested MVP scope: **US1 + US3 only** (P1 features).

### Parallelization Opportunities

After Phase 2 completion:
- US1, US3 can be implemented in parallel (both depend only on foundation)
- US2, US4, US5 can start once US1 handler is complete (for patterns/reference)

---

## Phase 1: Setup & Migrations

### Goal
Initialize database schema and project structure.

### Independent Test Criteria
- [ ] Migration applies cleanly to test database
- [ ] Migration can be rolled back without errors
- [ ] User table exists with correct schema (columns, constraints, indexes)

---

- [ ] T001 Create PostgreSQL migration for users table in `migrations/000X_create_users_table.sql`
- [ ] T002 Define user table with columns: id (UUID), tenant_id (FK), first_name, last_name, email, role, image, created_at, updated_at, deleted_at
- [ ] T003 Add constraints: PRIMARY KEY (id), UNIQUE (tenant_id, email), FK tenant_id → tenants(id)
- [ ] T004 Add indexes: (tenant_id, deleted_at), (tenant_id, email) in `migrations/000X_create_users_table.sql`
- [ ] T005 Test migration with `migrate -path migrations -database postgres://... up` on test database
- [ ] T006 Verify rollback: `migrate -path migrations -database postgres://... down` completes without error

---

## Phase 2: Foundation (Blocking Prerequisites)

### Goal
Implement core layers (domain, repository, service) that all user stories depend on.

### Independent Test Criteria
- [ ] Repository interface defined and PostgreSQL implementation created
- [ ] Service layer implements all CRUD operations
- [ ] Domain model validates all business rules
- [ ] All tests pass (≥80% coverage)

---

### Subtask Group: Domain Layer

- [ ] T007 Create domain/users/user.go with User struct (id, tenantID, firstName, lastName, email, role, image, createdAt, updatedAt, deletedAt)
- [ ] T008 Implement User validation: email format, firstName/lastName max 100 chars, role enum ('admin', 'user')
- [ ] T009 Create domain/users/command.go with CreateUserCommand and UpdateUserCommand structs
- [ ] T010 Add validation methods to commands (field presence, immutability checks for update)
- [ ] T011 Create domain/users/errors.go with domain error types (ErrEmailTaken, ErrNotFound, ErrInvalidRole, etc.)
- [ ] T012 Write unit tests for domain models in `internal/domain/users/user_test.go` (coverage ≥80%)

### Subtask Group: Repository Layer

- [ ] T013 Create repo/pg/users/repository.go interface with methods: ListByTenant, GetByID, Create, Update, Delete
- [ ] T014 Create repo/pg/users/postgres.go implementing repository.go
- [ ] T015 [P] Implement ListByTenant(ctx, tenantID, limit, offset) with pagination and soft-delete filtering (WHERE deleted_at IS NULL)
- [ ] T016 [P] Implement GetByID(ctx, tenantID, userID) returning 404 if soft-deleted (check deleted_at IS NULL)
- [ ] T017 [P] Implement Create(ctx, user) with email uniqueness check (tenant_id, email) UNIQUE constraint → 409 on violation
- [ ] T018 [P] Implement Update(ctx, user) with immutability checks (error if email or tenantID changed)
- [ ] T019 [P] Implement Delete(ctx, tenantID, userID) soft-delete: UPDATE users SET deleted_at = NOW() WHERE id = ? AND tenant_id = ?
- [ ] T020 Write integration tests in `internal/repo/pg/users/postgres_test.go` with test PostgreSQL container (coverage ≥80%)
- [ ] T021 Test multi-tenant isolation: verify queries scoped to tenant_id, cross-tenant queries return empty

### Subtask Group: Service Layer

- [ ] T022 Create app/users/service.go with UserService type and all use case methods
- [ ] T023 Implement ListUsers(ctx, tenantID, limit, offset) → calls repository.ListByTenant, returns paginated results + total count
- [ ] T024 Implement GetUser(ctx, tenantID, userID) → calls repository.GetByID, returns User or error
- [ ] T025 Implement CreateUser(ctx, tenantID, cmd) → validates command, calls repository.Create, returns User or domain error
- [ ] T026 Implement UpdateUser(ctx, tenantID, userID, cmd) → validates command, calls repository.Update, returns User or domain error
- [ ] T027 Implement DeleteUser(ctx, tenantID, userID) → calls repository.Delete, returns error only
- [ ] T028 Add logging via zap.Logger (info for mutations, debug for queries, error for failures)
- [ ] T029 Write unit tests in `internal/app/users/service_test.go` with mocked repository (coverage ≥80%)

---

## Phase 3: [US1] List Users for Tenant

### Goal
Implement the List Users endpoint with pagination and soft-delete filtering.

### User Story
An administrator accesses the list of all users belonging to their tenant to review who has access to the platform. Users are fetched with pagination (limit + offset).

### Independent Test Criteria
- [ ] GET /api/v1/users returns 200 with paginated user array
- [ ] Pagination works: limit/offset query params respected, metadata returned (total, count, limit, offset)
- [ ] Soft-deleted users are excluded from results
- [ ] Missing X-Tenant-ID header returns 400 Bad Request
- [ ] Cross-tenant access returns 403 Forbidden

---

- [ ] T030 Create api/handler/users/dto/list.go with ListUsersRequest, ListUsersResponse, PaginationMetadata structs
- [ ] T031 Implement ListUsersResponse marshaling to JSON with proper field names (camelCase)
- [ ] T032 Create api/handler/users/handler.go with NewHandler(service *UserService) constructor
- [ ] T033 Implement handler.ListUsers(c *gin.Context) to:
  - Extract X-Tenant-ID header (return 400 if missing)
  - Parse limit/offset query params (validate: limit 1-100, offset ≥ 0, use defaults: limit=20, offset=0)
  - Call service.ListUsers(ctx, tenantID, limit, offset)
  - Map domain User objects to ListUserResponse DTOs
  - Return 200 with pagination metadata
- [ ] T034 Register route GET /api/v1/users in internal/api/router.go: `r.GET("/users", handler.ListUsers)`
- [ ] T035 Implement middleware to extract X-Tenant-ID and validate tenant ownership (verify JWT tenant claim matches)
- [ ] T036 Add Prometheus metrics: counter users_list_total{tenant_id,status}, histogram users_request_duration_seconds{endpoint=list,status}
- [ ] T037 Write integration test in `internal/api/handler/users/handler_test.go`:
  - Setup test database with sample users
  - GET /api/v1/users with valid X-Tenant-ID → 200 with paginated results
  - Verify soft-deleted users excluded
  - Test pagination: limit/offset combinations
  - Missing X-Tenant-ID → 400
  - Cross-tenant tenant ID → 403

---

## Phase 4: [US3] Create New User

### Goal
Implement the Create User endpoint with email uniqueness per tenant and RBAC enforcement.

### User Story
An administrator creates a new user account and assigns them a role within the tenant. The system ensures email uniqueness is scoped per tenant.

### Independent Test Criteria
- [ ] POST /api/v1/users with valid payload returns 201 with created User
- [ ] Duplicate email in same tenant returns 409 Conflict
- [ ] Same email in different tenant allowed (per-tenant uniqueness)
- [ ] Missing required fields returns 400 Bad Request
- [ ] Non-admin user returns 403 Forbidden
- [ ] Missing X-Tenant-ID header returns 400

---

- [ ] T038 Create api/handler/users/dto/create.go with CreateUserRequest (firstName, lastName, email, role, image) and CreateUserResponse
- [ ] T039 Add validation to CreateUserRequest in handler (email format, required fields)
- [ ] T040 Implement handler.CreateUser(c *gin.Context) to:
  - Extract X-Tenant-ID header (return 400 if missing)
  - Parse JSON body into CreateUserRequest
  - Validate RBAC: only admin role (return 403 if non-admin)
  - Call service.CreateUser(ctx, tenantID, command)
  - Handle domain errors: ErrEmailTaken → 409 Conflict, ErrInvalidRole → 400
  - Return 201 with created User DTO
- [ ] T041 Register route POST /api/v1/users in router with admin RBAC middleware: `r.POST("/users", middleware.RequireRole("admin"), handler.CreateUser)`
- [ ] T042 Add Prometheus metrics: counter users_create_total{tenant_id,status}, histogram users_request_duration_seconds{endpoint=create,status}
- [ ] T043 Write integration test in handler_test.go:
  - POST valid user → 201 with ID, timestamps, deleted_at=null
  - Duplicate email same tenant → 409
  - Same email different tenant → 201 (allowed)
  - Missing required field → 400
  - Non-admin user → 403
  - Missing X-Tenant-ID → 400

---

## Phase 5: [US2] View Specific User Profile

### Goal
Implement the Get User endpoint with soft-delete filtering and tenant isolation.

### User Story
An administrator retrieves the full profile of a specific user by their unique ID to review their details.

### Independent Test Criteria
- [ ] GET /api/v1/users/:id returns 200 with user profile
- [ ] Soft-deleted user returns 404 Not Found
- [ ] User not found returns 404
- [ ] Cross-tenant access returns 403 Forbidden
- [ ] Missing X-Tenant-ID returns 400

---

- [ ] T044 Create api/handler/users/dto/get.go with GetUserRequest (path param userId) and GetUserResponse
- [ ] T045 Implement handler.GetUser(c *gin.Context) to:
  - Extract X-Tenant-ID header (return 400 if missing)
  - Parse userId from path param (validate UUID format)
  - Call service.GetUser(ctx, tenantID, userID)
  - Handle errors: not found → 404, tenant mismatch → 403
  - Return 200 with User DTO
- [ ] T046 Register route GET /api/v1/users/:id in router: `r.GET("/users/:id", handler.GetUser)`
- [ ] T047 Add Prometheus metrics: counter users_get_total{tenant_id,status}, histogram users_request_duration_seconds{endpoint=get,status}
- [ ] T048 Write integration test in handler_test.go:
  - GET existing user → 200 with full profile
  - GET non-existent user → 404
  - GET soft-deleted user → 404
  - GET from different tenant → 403
  - Invalid UUID format → 400
  - Missing X-Tenant-ID → 400

---

## Phase 6: [US4] Update Existing User

### Goal
Implement the Update User endpoint with immutable field protection and RBAC enforcement.

### User Story
An administrator partially updates a user's profile (e.g., name or role) without replacing the entire record.

### Independent Test Criteria
- [ ] PATCH /api/v1/users/:id with valid payload returns 200 with updated User
- [ ] Attempting to update email returns 400 Bad Request
- [ ] Attempting to update tenantId returns 400 Bad Request
- [ ] Invalid role value returns 400
- [ ] Non-admin user returns 403
- [ ] User not found returns 404
- [ ] Missing X-Tenant-ID returns 400

---

- [ ] T049 Create api/handler/users/dto/update.go with UpdateUserRequest (firstName, lastName, role, image - all optional) and UpdateUserResponse
- [ ] T050 Add validation to UpdateUserRequest: reject if email or tenantId present
- [ ] T051 Implement handler.UpdateUser(c *gin.Context) to:
  - Extract X-Tenant-ID header (return 400 if missing)
  - Parse userId from path param
  - Parse JSON body into UpdateUserRequest
  - Validate RBAC: only admin role (return 403 if non-admin)
  - Call service.UpdateUser(ctx, tenantID, userID, command)
  - Handle errors: immutable field attempted → 400, not found → 404, invalid role → 400
  - Return 200 with updated User DTO
- [ ] T052 Register route PATCH /api/v1/users/:id in router with admin RBAC: `r.PATCH("/users/:id", middleware.RequireRole("admin"), handler.UpdateUser)`
- [ ] T053 Add Prometheus metrics: counter users_update_total{tenant_id,status}, histogram users_request_duration_seconds{endpoint=update,status}
- [ ] T054 Write integration test in handler_test.go:
  - PATCH valid fields → 200 with updated user, updatedAt changed
  - PATCH email field → 400 (immutable)
  - PATCH tenantId field → 400 (immutable)
  - PATCH invalid role → 400
  - PATCH non-existent user → 404
  - Non-admin user PATCH → 403
  - Missing X-Tenant-ID → 400

---

## Phase 7: [US5] Remove User (Soft Delete)

### Goal
Implement the Delete User endpoint with soft-delete logic and RBAC enforcement.

### User Story
An administrator removes a user from the platform, revoking their access. Deletion is soft (logical) — the user record is marked as deleted but retained in the database for audit and compliance purposes.

### Independent Test Criteria
- [ ] DELETE /api/v1/users/:id returns 204 No Content
- [ ] Soft-deleted user no longer appears in list queries
- [ ] Soft-deleted user returns 404 on direct access
- [ ] User not found returns 404
- [ ] Non-admin user returns 403
- [ ] Missing X-Tenant-ID returns 400

---

- [ ] T055 Create api/handler/users/dto/delete.go with DeleteUserRequest (path param userId) - no response body needed
- [ ] T056 Implement handler.DeleteUser(c *gin.Context) to:
  - Extract X-Tenant-ID header (return 400 if missing)
  - Parse userId from path param
  - Validate RBAC: only admin role (return 403 if non-admin)
  - Call service.DeleteUser(ctx, tenantID, userID)
  - Handle errors: not found → 404, tenant mismatch → 403
  - Return 204 No Content
- [ ] T057 Register route DELETE /api/v1/users/:id in router with admin RBAC: `r.DELETE("/users/:id", middleware.RequireRole("admin"), handler.DeleteUser)`
- [ ] T058 Add Prometheus metrics: counter users_delete_total{tenant_id,status}, histogram users_request_duration_seconds{endpoint=delete,status}
- [ ] T059 Write integration test in handler_test.go:
  - DELETE user → 204 No Content
  - Verify user no longer in list query
  - Verify GET user → 404
  - DELETE non-existent user → 404
  - Non-admin DELETE → 403
  - Missing X-Tenant-ID → 400

---

## Phase 8: Polish & Cross-Cutting Concerns

### Goal
Add observability, documentation, and finalize implementation.

### Independent Test Criteria
- [ ] All endpoints have structured Zap logging (info/warn/error levels)
- [ ] All endpoints have Prometheus metrics
- [ ] OpenAPI documentation is up-to-date
- [ ] All error responses follow standard error schema
- [ ] End-to-end quickstart scenarios pass

---

- [ ] T060 Add structured logging to all handlers using zap.Logger: info for mutations, debug for queries, error for failures
- [ ] T061 Create telemetry/metrics/users.go to register Prometheus counters and histograms:
  - users_list_total{tenant_id,status}
  - users_create_total{tenant_id,status}
  - users_get_total{tenant_id,status}
  - users_update_total{tenant_id,status}
  - users_delete_total{tenant_id,status}
  - users_request_duration_seconds{endpoint,status}
  - users_active_count{tenant_id} (gauge: count where deleted_at IS NULL)
- [ ] T062 Update docs/openapi.yaml with complete User Management API spec (5 endpoints, all parameters, responses, error codes)
- [ ] T063 Implement standard error response format in api/handler/errors.go:
  - Structure: {error: "CODE", message: "...", status: HTTP_CODE}
  - Map all domain errors to error codes: USER_NOT_FOUND, DUPLICATE_EMAIL, VALIDATION_ERROR, ACCESS_DENIED, MISSING_HEADER, etc.
- [ ] T064 Add X-Request-ID header to all responses for request tracing
- [ ] T065 Run full integration test suite:
  - All 5 user stories pass quickstart scenarios (see plan/quickstart.md)
  - Pagination works end-to-end
  - Soft delete filtering works end-to-end
  - Multi-tenant isolation verified
  - RBAC enforcement verified
- [ ] T066 Verify test coverage ≥80% for all layers (domain, repo, service, handler)
- [ ] T067 Review Constitution v1.1.0 compliance:
  - Clean architecture maintained (transport → app → domain ← repo)
  - Tenant isolation enforced (X-Tenant-ID header, DB scoping)
  - RBAC enforced (admin-only writes)
  - Observability in place (Zap + Prometheus)
  - Tests passing (integration + unit)
- [ ] T068 Create ADR-00X-user-management.md documenting design decisions (if required by Constitution)

---

## Task Dependencies & Parallelization

### Critical Path (Sequential)
```
T001-T006 (Migrations)
    ↓
T007-T029 (Foundation: Domain, Repo, Service)
    ↓
├─ T030-T037 [US1] List Users (3-4h) ─┐
├─ T038-T043 [US3] Create User (3-4h) ├─→ T060-T068 Polish (2-3h)
├─ T044-T048 [US2] Get User (1-2h) ────┤
├─ T049-T054 [US4] Update User (2-3h) ─┤
└─ T055-T059 [US5] Delete User (1-2h) ─┘
```

### Parallelization Opportunities (After T029)

**Batch 1** (can start in parallel after foundation):
- US1 (T030-T037) and US3 (T038-T043) — both independent, same layer

**Batch 2** (can start after Batch 1):
- US2 (T044-T048), US4 (T049-T054), US5 (T055-T059) — all reference US1 patterns

**Polish** (final, sequential):
- T060-T068 — depends on all endpoints working

### Suggested MVP Scope (First Delivery)

- **Phases 1-4 only** (T001-T043): Database setup + Foundation + US1 + US3
- **Duration**: 8-10 hours
- **Delivers**: List users + Create users (P1 features)
- **Test Coverage**: ≥80% for completed stories
- **Skip for MVP**: US2, US4, US5, full observability
- **Add in Next Sprint**: US2, US4, US5, full observability (Phase 5-8)

---

## Notes

- All paths are absolute and relative to `embolsadora-api/` root
- Test integration requires test PostgreSQL container (use `testcontainers-go`)
- All handlers must include tenant isolation validation (X-Tenant-ID header + JWT claim check)
- Error handling must distinguish between domain errors (business logic) and HTTP errors (transport layer)
- Logging must use zap.Logger for structured logs (no PII in logs)
- Metrics must use Prometheus client library (already in go.mod)
- Code must follow Constitution v1.1.0 (Clean Architecture, multi-tenant isolation, observability)

