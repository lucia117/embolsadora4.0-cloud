# Tasks: Dashboard Layouts API

**Input**: Design documents from `specs/005-dashboard-layouts/`
**Branch**: `005-dashboard-layouts`
**Total tasks**: 38
**Spec**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Data model**: [plan/data-model.md](./plan/data-model.md) | **Contract**: [contracts/dashboard-service-api.openapi.yaml](./contracts/dashboard-service-api.openapi.yaml)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (US1–US5)
- File paths are relative to repo root

---

## Phase 1: Setup (Directory Structure)

**Purpose**: Create all necessary directories so parallel tasks can proceed without conflicts.

- [x] T001 Create directory structure: `internal/domain/dashboard_layouts/`, `internal/repo/pg/dashboard_layouts/`, `internal/app/dashboard_layouts/`, `internal/api/handler/dashboard_layouts/dto/`

**Checkpoint**: Directory structure ready — all subsequent tasks can target specific files.

---

## Phase 2: Foundational (Migration + Domain + Repository Interface)

**Purpose**: Core infrastructure that MUST be complete before any user story can be implemented. DB schema, domain types, errors, and the repository interface are shared by all 5 user stories.

⚠️ **CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 Create migration `migrations/0006_create_dashboard_layouts_table.up.sql` — table `dashboard_layouts` with columns: `id UUID PK`, `tenant_id UUID FK`, `name VARCHAR(255)`, `widgets JSONB DEFAULT '[]'`, `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`, `deleted_at TIMESTAMPTZ`; partial unique index `(tenant_id, name) WHERE deleted_at IS NULL`; list index `(tenant_id) WHERE deleted_at IS NULL`; trigger `set_dashboard_layouts_updated_at`
- [x] T003 [P] Create migration `migrations/0006_create_dashboard_layouts_table.down.sql` — `DROP TABLE IF EXISTS dashboard_layouts;`
- [x] T004 [P] Create domain aggregate in `internal/domain/dashboard_layouts/dashboard_layout.go` — `DashboardLayout` struct (ID, TenantID, Name, Widgets, CreatedAt, UpdatedAt, DeletedAt) and `Widget` struct (ID, Type, Name, Title, Description, Category, Icon, Position{X,Y,W,H,I})
- [x] T005 [P] Create domain commands in `internal/domain/dashboard_layouts/commands.go` — `CreateLayoutCommand{Name string; Widgets []Widget}` and `UpdateLayoutCommand{Name string; Widgets []Widget}`
- [x] T006 [P] Create domain errors in `internal/domain/dashboard_layouts/errors.go` — sentinel errors: `ErrLayoutNotFound`, `ErrDuplicateName`, `ErrLimitReached`, `ErrCannotDeleteLastLayout`
- [x] T007 Create `Repository` interface in `internal/domain/dashboard_layouts/repository.go` — methods: `List`, `GetByID`, `CountByTenant`, `ExistsByName`, `Create`, `Update`, `SoftDelete`

**Checkpoint**: Foundation ready — T002 migration applied, domain types and repository interface defined. User story work can begin.

---

## Phase 3: Repository Implementation

**Purpose**: PostgreSQL implementation of the repository interface. All user story service operations depend on this layer.

⚠️ Depends on T004, T005, T006, T007 completing first.

- [x] T008 Create `internal/repo/pg/dashboard_layouts/repository.go` — struct `PostgresRepository{db *pgxpool.Pool}` and constructor `NewPostgresRepository(db) *PostgresRepository`
- [x] T009 [P] Implement `List` in `internal/repo/pg/dashboard_layouts/repository.go`
- [x] T010 [P] Implement `GetByID` in `internal/repo/pg/dashboard_layouts/repository.go` — return `ErrLayoutNotFound` on `pgx.ErrNoRows`
- [x] T011 [P] Implement `CountByTenant` in `internal/repo/pg/dashboard_layouts/repository.go`
- [x] T012 [P] Implement `ExistsByName` in `internal/repo/pg/dashboard_layouts/repository.go` — `excludeID` allows self-exclusion for updates
- [x] T013 [P] Implement `Create` in `internal/repo/pg/dashboard_layouts/repository.go` — handle `23505` unique violation as `ErrDuplicateName`
- [x] T014 [P] Implement `Update` in `internal/repo/pg/dashboard_layouts/repository.go` — handle `23505` as `ErrDuplicateName`; `RowsAffected == 0` → `ErrLayoutNotFound`
- [x] T015 [P] Implement `SoftDelete` in `internal/repo/pg/dashboard_layouts/repository.go` — `RowsAffected == 0` → `ErrLayoutNotFound`

**Checkpoint**: All repository methods implemented. Service layer can be built.

---

## Phase 4: User Story 1 — List Dashboard Layouts (Priority: P1) 🎯 MVP

**Goal**: Authenticated users can retrieve all layouts for their tenant.

**Independent Test**: `GET /api/tenants/acme/dashboard-layouts` with valid JWT → 200 with `{ success: true, data: [...], meta: { total: N, limit: 3 } }`. Without JWT → 401.

- [x] T016 Create `internal/app/dashboard_layouts/service.go` — `Service` struct with `repo Repository` and `logger *zap.Logger`; constructor `NewService(repo, logger) *Service`
- [x] T017 [US1] Implement `ListLayouts(ctx, tenantID uuid.UUID)` in `internal/app/dashboard_layouts/service.go`
- [x] T018 [P] [US1] Create `internal/api/handler/dashboard_layouts/dto/dto.go` — `WidgetDTO`, `LayoutDTO`, `ListLayoutsResponse`, `MetaDTO`, `CreateLayoutRequest`, `UpdateLayoutRequest`, `SingleLayoutResponse`, `DeleteLayoutResponse`, `ErrorResponse`, mapping helpers
- [x] T019 [US1] Create `internal/api/handler/dashboard_layouts/list_layouts.go` — handler returning `ListLayoutsResponse` with `meta.limit = 3`
- [x] T020 [US1] Create `internal/api/handler/dashboard_layouts/routes.go` — `RegisterRoutes(group, service)`; register `GET /dashboard-layouts`

**Checkpoint**: `GET /api/tenants/:tenantId/dashboard-layouts` fully functional. US1 independently testable.

---

## Phase 5: User Story 2 — Create a Dashboard Layout (Priority: P1)

**Goal**: Authenticated users can create a new named layout with widgets. Enforces limit (max 3) and name uniqueness per tenant.

**Independent Test**: `POST /api/tenants/acme/dashboard-layouts` with valid body → 200 created layout. Duplicate name → 409. 4th layout attempt → 403.

- [x] T021 [US2] Implement `CreateLayout(ctx, tenantID, cmd)` in `internal/app/dashboard_layouts/service.go` — count check (≥3 → ErrLimitReached), name uniqueness check, then `repo.Create()`
- [x] T022 [US2] Create `internal/api/handler/dashboard_layouts/create_layout.go` — map `ErrLimitReached` → 403, `ErrDuplicateName` → 409
- [x] T023 [US2] Register `POST /dashboard-layouts` in `internal/api/handler/dashboard_layouts/routes.go`

**Checkpoint**: US1 + US2 complete. Tenants can list and create layouts (with business rule enforcement).

---

## Phase 6: User Story 3 — Get a Specific Dashboard Layout (Priority: P2)

**Goal**: Authenticated users can retrieve a single layout by ID.

**Independent Test**: `GET /api/tenants/acme/dashboard-layouts/:id` with existing ID → 200. Non-existent ID → 404.

- [x] T024 [US3] Implement `GetLayout(ctx, tenantID, layoutID)` in `internal/app/dashboard_layouts/service.go`
- [x] T025 [US3] Create `internal/api/handler/dashboard_layouts/get_layout.go` — map `ErrLayoutNotFound` → 404 `"Layout no encontrado"`
- [x] T026 [US3] Register `GET /dashboard-layouts/:layoutId` in `internal/api/handler/dashboard_layouts/routes.go`

**Checkpoint**: US1 + US2 + US3 complete. Full read path (list + get by ID) operational.

---

## Phase 7: User Story 4 — Update a Dashboard Layout (Priority: P2)

**Goal**: Authenticated users can rename a layout and replace its widget list.

**Independent Test**: `PUT /api/tenants/acme/dashboard-layouts/:id` with new name → 200 with refreshed `updatedAt`. Duplicate name → 409. Non-existent ID → 404.

- [x] T027 [US4] Implement `UpdateLayout(ctx, tenantID, layoutID, cmd)` in `internal/app/dashboard_layouts/service.go` — verify exists, name uniqueness (self-exclusion), then `repo.Update()`
- [x] T028 [US4] Create `internal/api/handler/dashboard_layouts/update_layout.go` — map `ErrLayoutNotFound` → 404 `"NOT_FOUND"`, `ErrDuplicateName` → 409 `"DUPLICATE_NAME"`
- [x] T029 [US4] Register `PUT /dashboard-layouts/:layoutId` in `internal/api/handler/dashboard_layouts/routes.go`

**Checkpoint**: US1–US4 complete. Full read + write path operational (except delete).

---

## Phase 8: User Story 5 — Delete a Dashboard Layout (Priority: P2)

**Goal**: Authenticated users can delete a layout. Last layout is protected.

**Independent Test**: `DELETE /api/tenants/acme/dashboard-layouts/:id` with 2+ layouts → 200. With only 1 layout → 400. Non-existent ID → 404.

- [x] T030 [US5] Implement `DeleteLayout(ctx, tenantID, layoutID)` in `internal/app/dashboard_layouts/service.go` — verify exists, count ≤1 → ErrCannotDeleteLastLayout, then `repo.SoftDelete()`
- [x] T031 [US5] Create `internal/api/handler/dashboard_layouts/delete_layout.go` — map `ErrCannotDeleteLastLayout` → 400, `ErrLayoutNotFound` → 404
- [x] T032 [US5] Register `DELETE /dashboard-layouts/:layoutId` in `internal/api/handler/dashboard_layouts/routes.go`

**Checkpoint**: All 5 user stories complete. Full CRUD operational with all 12 Pact interactions covered.

---

## Phase 9: Router Integration

**Purpose**: Wire dependency injection and register dashboard layout routes into the existing `/api/tenants/:tenantId` Gin group.

- [x] T033 Add imports for `dashboardLayoutsApp`, `dashboardLayoutsHandler`, `dashboardLayoutsRepo` in `internal/routes/url_mappings.go`
- [x] T034 Instantiate repository, service, and register routes in `internal/routes/url_mappings.go`

**Checkpoint**: All 5 endpoints live at `/api/tenants/:tenantId/dashboard-layouts`. Manual smoke test with quickstart.md curl commands.

---

## Phase 10: Observability

**Purpose**: Structured logging and Prometheus metrics on all operations (Constitution Principle III).

- [x] T035 Create `internal/telemetry/dashboard_layout_metrics.go` — counter `dashboard_layout_requests_total{operation, status}` and histogram `dashboard_layout_request_duration_seconds{operation}`
- [x] T036 [P] Zap structured logging verified in all service methods — `Info` level with fields `tenant_id`, `layout_id`, `operation`; `Error` level with `error` field

**Checkpoint**: Observability requirements satisfied per Constitution v1.1.0.

---

## Phase 11: Polish & Validation

**Purpose**: Unit tests, integration tests, and Pact provider verification.

- [ ] T037 Write unit tests in `internal/app/dashboard_layouts/service_test.go` using `uber/mock` — mock `Repository` interface; test all 5 operations including all error paths
- [ ] T038 [P] Write integration tests in `tests/integration/dashboard_layouts/` using Docker Postgres — cover: create, duplicate name, limit enforcement, cross-tenant isolation, soft-delete, last-layout deletion, update name self-exclusion

**Checkpoint**: All 12 Pact interactions from `embolsadora-frontend/pacts/dashboard-service-api.json` verified.

---

## Dependency Graph

```
T001 (dirs)
  └── T002–T007 (foundation: migration + domain)
        └── T008–T015 (repository implementation)
              ├── T016–T020 (US1: List)      ← MVP deliverable
              ├── T021–T023 (US2: Create)    ← MVP deliverable
              ├── T024–T026 (US3: Get)
              ├── T027–T029 (US4: Update)
              └── T030–T032 (US5: Delete)
                    └── T033–T034 (router integration)
                          ├── T035–T036 (observability)
                          └── T037–T038 (tests)
```

## Summary

| Phase | Tasks | Status |
|---|---|---|
| Setup | T001 | ✅ |
| Foundational | T002–T007 | ✅ |
| Repository | T008–T015 | ✅ |
| List Layouts (US1 P1) | T016–T020 | ✅ |
| Create Layout (US2 P1) | T021–T023 | ✅ |
| Get Layout (US3 P2) | T024–T026 | ✅ |
| Update Layout (US4 P2) | T027–T029 | ✅ |
| Delete Layout (US5 P2) | T030–T032 | ✅ |
| Router Integration | T033–T034 | ✅ |
| Observability | T035–T036 | ✅ |
| Polish & Tests | T037–T038 | ⏳ pending |
| **Total** | **38 tasks** | **36/38 ✅** |
