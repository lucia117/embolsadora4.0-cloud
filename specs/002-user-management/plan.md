# Implementation Plan: User Management API

**Feature**: 002-user-management
**Branch**: `002-user-management`
**Spec**: [spec.md](./spec.md)
**Status**: Ready for Implementation

---

## I. Architecture & Design

### Architectural Alignment

This feature implements **Principle I** (Hexagonal Architecture) and **Principle II** (Aislamiento de Seguridad) from Constitution v1.1.0:

- **Surface**: ABM (`/api/v1/**`) — Administrative user management
- **Authentication**: JWT bearer token + RBAC
- **Tenant Isolation**: X-Tenant-ID header scopes all operations
- **Layer Pattern**: `transport → app → domain ← repo | security | telemetry | platform`

### Key Design Decisions

| Decision | Rationale | Trade-offs |
|----------|-----------|-----------|
| **Tenant Context via Header** | Explicit, stateless, follows SaaS pattern | Visible in logs (acceptable, non-PII) |
| **Email Uniqueness per Tenant** | Flexibility for multi-org SaaS | Not globally unique (OK for MVP) |
| **Pagination (limit+offset)** | Standard, scalable to N users | Not cursor-based (OK for initial version) |
| **Soft Delete** | Audit trail + compliance preservation | Requires deleted_at filtering (minor query cost) |
| **UUID IDs** | Industry standard, globally unique | Larger than int (acceptable tradeoff) |

---

## II. Technical Context

### Constitution Compliance

- ✅ **Clean Architecture**: Hexagonal pattern with separated layers
- ✅ **Tenant Isolation**: `X-Tenant-ID` header enforced, DB queries scoped
- ✅ **RBAC**: Only `admin` role can write; validation in middleware
- ✅ **Observability**: Zap logging + Prometheus metrics required (Phase 1.1)
- ✅ **Testing**: Integration tests required for DB + API contracts
- ✅ **OpenAPI**: Contract defined in contracts/user-service-api.openapi.yaml

### Dependencies

- **Platform**: `platform.TenantID` context function (existing pattern)
- **Security**: JWT middleware (existing, reuse)
- **Database**: PostgreSQL with migrations (existing)
- **Framework**: Gin router (existing)
- **Logging**: Zap (existing)
- **Testing**: testify + uber/mock (existing)

### No New External Dependencies Required

All required libraries already in `go.mod`:
- `github.com/gin-gonic/gin`
- `jackc/pgx/v5`
- `go.uber.org/zap`
- `github.com/prometheus/client_golang`
- `github.com/stretchr/testify`

---

## III. Design Artifacts

### 1. Data Model
**File**: [plan/data-model.md](./plan/data-model.md)

**User Entity**:
- Fields: id (UUID), tenant_id, firstName, lastName, email, role, image, createdAt, updatedAt, deletedAt
- Constraints: (tenant_id, email) UNIQUE, soft delete via deleted_at
- Validation: Email format, max lengths, role enum
- Indexes: (tenant_id, deleted_at), (tenant_id, email)

**PostgreSQL Schema**:
```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    email VARCHAR(254) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    image TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE (tenant_id, email),
    INDEX idx_tenant_deleted (tenant_id, deleted_at),
    INDEX idx_tenant_email (tenant_id, email)
);
```

### 2. API Contract
**File**: [contracts/user-service-api.openapi.yaml](./contracts/user-service-api.openapi.yaml)

**Endpoints** (all require X-Tenant-ID header + JWT):
- `GET /api/v1/users` — List paginated users
- `POST /api/v1/users` — Create user (admin only)
- `GET /api/v1/users/:id` — Get user by ID
- `PATCH /api/v1/users/:id` — Update user (admin only, immutable email/tenantId)
- `DELETE /api/v1/users/:id` — Soft delete user (admin only)

**Status Codes**:
- 200 OK, 201 Created, 204 No Content
- 400 Bad Request (validation, missing header, immutable field)
- 401 Unauthorized (missing/invalid JWT)
- 403 Forbidden (cross-tenant, non-admin write)
- 404 Not Found (user not found or soft-deleted)
- 409 Conflict (duplicate email in same tenant)

### 3. Testing & Validation
**File**: [plan/quickstart.md](./plan/quickstart.md)

Integration test scenarios covering:
- CRUD operations (happy path + error cases)
- Pagination behavior
- Soft delete filtering
- Multi-tenant isolation
- RBAC enforcement
- Immutable field protection

---

## IV. Implementation Phases

### Phase 0: Database Schema & Migrations
**Effort**: 1-2 hours

1. Create migration file: `migrations/000N_create_users_table.sql`
2. Define User table with constraints and indexes
3. Test migration up/down on test container
4. Verify indexes and constraints

**Deliverable**: Migration file + verification tests

### Phase 1.0: Data Layer (Repository)
**Effort**: 3-4 hours

**Location**: `internal/repo/pg/users/`

**Components**:
1. `repository.go` — User repository interface
   - `ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*User, int, error)`
   - `GetByID(ctx context.Context, tenantID, userID string) (*User, error)`
   - `Create(ctx context.Context, user *User) (*User, error)`
   - `Update(ctx context.Context, user *User) (*User, error)`
   - `Delete(ctx context.Context, tenantID, userID string) error`
2. `postgres.go` — PostgreSQL implementation
   - Tenant scoping in all queries (WHERE tenant_id = ?)
   - Soft delete filtering (WHERE deleted_at IS NULL)
   - Pagination with LIMIT/OFFSET
   - Unique constraint handling (409 on duplicate email)

**Key Implementation Details**:
- Reuse `platform.TenantID` context pattern
- All queries must include tenant_id scope
- Use prepared statements (pgx parameterized queries)
- Handle pgx unique constraint error → 409
- Atomic transactions for updates

**Testing**:
- Unit tests with uber/mock
- Integration tests with test Postgres container
- Test soft-delete filtering
- Test multi-tenant isolation

**Deliverables**: Repository interface + Postgres implementation + tests (coverage ≥80%)

### Phase 1.1: Domain & Application Layer
**Effort**: 3-4 hours

**Location**: `internal/domain/users/`, `internal/app/users/`

**Components**:
1. `domain/users/user.go` — Value objects & aggregate
   - User struct with validation
   - NewUser() constructor with validation
   - Role enum with validation
2. `app/users/service.go` — Application service (use cases)
   - ListUsers(ctx, tenantID, limit, offset)
   - GetUser(ctx, tenantID, userID)
   - CreateUser(ctx, tenantID, cmd CreateUserCommand)
   - UpdateUser(ctx, tenantID, userID, cmd UpdateUserCommand)
   - DeleteUser(ctx, tenantID, userID)
3. Business logic:
   - Email format validation
   - Role enum validation
   - Immutable field checks (on update)
   - Pagination validation (limit 1-100, offset ≥ 0)
   - Access control checks (RBAC passed from handler context)

**Key Implementation Details**:
- Service depends on Repository interface (testable)
- Return domain errors (not HTTP errors)
- No HTTP details in domain/app
- Logging via Zap (error/info levels)
- Metrics recording (request count, latency)

**Testing**:
- Unit tests with mocked repository
- Test business rules (validation, immutability, pagination)

**Deliverables**: Domain models + service + tests (coverage ≥80%)

### Phase 1.2: HTTP Transport Layer (Handlers)
**Effort**: 3-4 hours

**Location**: `internal/api/handler/users/`

**Components**:
1. `dto/` — DTOs
   - ListUsersRequest, ListUsersResponse
   - GetUserRequest, GetUserResponse
   - CreateUserRequest, CreateUserResponse
   - UpdateUserRequest, UpdateUserResponse
   - DeleteUserRequest, DeleteUserResponse
2. `handler.go` — HTTP handlers
   - ListUsers(c *gin.Context)
   - GetUser(c *gin.Context)
   - CreateUser(c *gin.Context)
   - UpdateUser(c *gin.Context)
   - DeleteUser(c *gin.Context)
3. Middleware:
   - X-Tenant-ID header validation (extract, validate UUID)
   - RBAC check (admin only for write ops)
   - Error mapping (domain error → HTTP response)

**Key Implementation Details**:
- Use `c.Header("X-Tenant-ID")` to extract tenant
- Set `context.WithValue(ctx, platform.TenantID, tenantID)`
- Middleware layer: `security.AuthJWT()` (existing) + `security.RequireRole("admin")` (reuse if exists)
- Error responses: `{"error": "CODE", "message": "...", "status": 400}`
- Logging: request/response via Zap middleware
- Metrics: counter per endpoint, histogram for latency

**Testing**:
- Integration tests with test server
- Test all status codes (200, 201, 204, 400, 401, 403, 404, 409)
- Test header validation
- Test RBAC enforcement

**Deliverables**: Handlers + DTOs + tests (coverage ≥80%)

### Phase 1.3: Router Registration & Integration
**Effort**: 1-2 hours

**Location**: `internal/api/router.go`

**Changes**:
1. Import `github.com/lucia117/embolsadora4.0-cloud/internal/api/handler/users`
2. Register routes:
   ```go
   abm := r.Group("/api/v1")
   abm.Use(middleware.AuthJWT())  // Existing JWT middleware

   usersHandler := users.NewHandler(userService)
   abm.GET("/users", usersHandler.ListUsers)
   abm.POST("/users", middleware.RequireRole("admin"), usersHandler.CreateUser)
   abm.GET("/users/:id", usersHandler.GetUser)
   abm.PATCH("/users/:id", middleware.RequireRole("admin"), usersHandler.UpdateUser)
   abm.DELETE("/users/:id", middleware.RequireRole("admin"), usersHandler.DeleteUser)
   ```

**Testing**:
- Router integration test
- Verify middleware order (auth → rbac → handler)

**Deliverable**: Updated router with registered routes + integration test

### Phase 2: Observability & Documentation
**Effort**: 2-3 hours

**Components**:
1. **Logging**:
   - Structured logs for each operation
   - Error logs with stack traces
   - Info logs for create/update/delete with user details (no emails)

2. **Metrics**:
   - Counter: `users_list_total{tenant_id,status}`
   - Counter: `users_create_total{tenant_id,status}`
   - Counter: `users_update_total{tenant_id,status}`
   - Counter: `users_delete_total{tenant_id,status}`
   - Histogram: `users_request_duration_seconds{endpoint,status}`
   - Gauge: `users_active{tenant_id}` (count where deleted_at IS NULL)

3. **Documentation**:
   - Update `docs/openapi.yaml` with endpoints
   - Add quickstart guide ([plan/quickstart.md](./plan/quickstart.md) already created)
   - Document error codes and HTTP status mapping

**Deliverable**: Logging + metrics implementation + documentation updates

### Phase 3: ADR (If Required)
If multi-tenant email uniqueness decision or soft delete strategy needs justification, create:
- `docs/adr/ADR-00X-user-email-per-tenant.md`
- `docs/adr/ADR-00X-soft-delete-strategy.md`

**Deliverable**: ADR document (optional)

---

## V. File Structure

```
embolsadora-api/
├── migrations/
│   └── 000N_create_users_table.sql         [NEW] User table migration
├── internal/
│   ├── domain/
│   │   └── users/
│   │       └── user.go                     [NEW] User aggregate + value objects
│   ├── app/
│   │   └── users/
│   │       ├── service.go                  [NEW] Application service
│   │       └── command.go                  [NEW] Commands (CreateUser, UpdateUser)
│   ├── repo/
│   │   └── pg/
│   │       └── users/
│   │           ├── repository.go           [NEW] Interface definition
│   │           └── postgres.go             [NEW] PostgreSQL implementation
│   ├── api/
│   │   ├── router.go                       [MODIFIED] Register user routes
│   │   └── handler/
│   │       └── users/
│   │           ├── handler.go              [NEW] HTTP handlers
│   │           └── dto/
│   │               ├── list.go             [NEW] List request/response DTOs
│   │               ├── get.go              [NEW] Get request/response DTOs
│   │               ├── create.go           [NEW] Create request/response DTOs
│   │               ├── update.go           [NEW] Update request/response DTOs
│   │               └── delete.go           [NEW] Delete request/response DTOs
│   └── telemetry/
│       └── metrics/
│           └── users.go                    [NEW] User metrics registration
├── specs/
│   └── 002-user-management/
│       ├── spec.md                         [DONE] Feature specification
│       ├── plan.md                         [THIS FILE] Implementation plan
│       ├── research.md                     [DONE] Research findings (empty, no clarifications)
│       ├── plan/
│       │   ├── data-model.md               [DONE] DB schema design
│       │   └── quickstart.md               [DONE] Testing guide
│       └── contracts/
│           └── user-service-api.openapi.yaml [DONE] OpenAPI contract
└── docs/
    ├── openapi.yaml                        [MODIFIED] Update with new endpoints
    └── adr/
        └── ADR-00X-...md                   [OPTIONAL] Decisions documentation
```

---

## VI. Success Criteria

### Code Quality
- ✅ No `NEEDS CLARIFICATION` markers in implementation
- ✅ Follows Constitution v1.1.0 (clean architecture, tenant isolation, observability)
- ✅ Test coverage ≥80% for all layers
- ✅ All HTTP status codes match OpenAPI contract

### Functional Testing
- ✅ All 5 user stories pass acceptance scenarios
- ✅ Pagination works (limit + offset)
- ✅ Soft delete filters correctly
- ✅ Multi-tenant isolation enforced (403 on cross-tenant)
- ✅ RBAC enforced (403 on non-admin write)
- ✅ Email uniqueness per tenant (409 on duplicate)
- ✅ Immutable fields protected (email, tenantId)

### Non-Functional
- ✅ List operation <500ms (typical; up to 10K users)
- ✅ Single user retrieval <100ms
- ✅ Create operation <1s (including DB)
- ✅ All responses include X-Request-ID (tracing)
- ✅ Structured logs + Prometheus metrics

### Documentation
- ✅ OpenAPI contract matches implementation
- ✅ Quickstart guide works end-to-end
- ✅ Migration file documented
- ✅ ADR (if required) explains decisions

---

## VII. Estimated Effort

| Phase | Task | Effort | Owner |
|-------|------|--------|-------|
| 0 | Database migration | 1-2 h | Backend |
| 1.0 | Repository + tests | 3-4 h | Backend |
| 1.1 | Domain + service + tests | 3-4 h | Backend |
| 1.2 | Handlers + DTOs + tests | 3-4 h | Backend |
| 1.3 | Router integration | 1-2 h | Backend |
| 2 | Observability | 2-3 h | Backend |
| 3 | ADR (optional) | 0-1 h | Backend |
| **Total** | | **14-20 h** | |

---

## VIII. Next Steps

1. **Branch**: Checkout `002-user-management` ✅ (already done)
2. **Phase 0**: Create migration file
3. **Phase 1.0-1.3**: Implement layers in order
4. **Phase 2**: Add observability
5. **Testing**: Run `go test ./...` and integration tests
6. **Review**: Check constitution compliance
7. **Merge**: Create PR, verify all checks pass

---

## Appendix: Architecture Diagram

```
HTTP Request (POST /api/v1/users)
    ↓
Transport Layer (handler/users/handler.go)
  ├─ Extract X-Tenant-ID header
  ├─ Validate JWT (middleware/auth.go)
  ├─ Check RBAC (middleware/rbac.go)
  ├─ Parse request DTO
  └─ Call application service
    ↓
Application Layer (app/users/service.go)
  ├─ Instantiate command
  ├─ Validate business rules
  └─ Call repository
    ↓
Domain Layer (domain/users/user.go)
  └─ User aggregate + value objects
    ↓
Repository Layer (repo/pg/users/postgres.go)
  ├─ Scope query by tenant_id
  ├─ Execute INSERT query
  └─ Return created user
    ↓
Application Layer (back)
  └─ Record metrics + logs
    ↓
Transport Layer (back)
  ├─ Map domain object → response DTO
  ├─ Return 201 Created + User JSON
    ↓
HTTP Response (201 Created + JSON body)
```

---

**Status**: Ready for implementation phase
**Next**: `/speckit.tasks` to generate actionable tasks
