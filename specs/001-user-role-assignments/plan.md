# Implementation Plan: User Role Assignment Management

**Branch**: `001-user-role-assignments` | **Date**: 2026-02-27 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-user-role-assignments/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command.

## Summary

Implement user-tenant-role (UTR) assignment management endpoints for the ABM surface (`/api/v1`), satisfying the consumer-driven Pact v2 contract between `embolsadora-frontend-bff` and `user-role-service-api`. The implementation introduces a `roles` catalog table, a `user_tenant_roles` table with soft-delete semantics and a partial unique index enforcing one active role per user+tenant, and 6 new HTTP endpoints following the exact same hexagonal architecture already used for tenant management.

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: Gin (HTTP), pgx/v5 (PostgreSQL), Zap (logging), Prometheus (metrics), testify + uber/mock (testing)
**Storage**: PostgreSQL — new tables `roles` + `user_tenant_roles`; Redis (no new usage required for this feature)
**Testing**: testify + uber/mock; integration tests against Dockerized PostgreSQL
**Target Platform**: Linux server (Docker / Cloud Run)
**Project Type**: Web service — monolith modular, Hexagonal / Clean Architecture
**Performance Goals**: Single assignment < 30s (user-facing); bulk 100 users < 5s
**Constraints**: One active role per user+tenant enforced at DB level (partial unique index); soft-delete only; all queries scoped to `tenant_id`; JWT Bearer auth on all endpoints
**Scale/Scope**: ABM surface; same scale as existing tenant endpoints

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Status | Notes |
|------|--------|-------|
| Hexagonal layers respected (transport → app → domain ← repo) | ✅ PASS | New handlers, use cases, repo follow same pattern as `tenants/` |
| ABM surface auth: JWT Bearer + RBAC | ✅ PASS | All 6 routes registered under the existing `v1` group with `apimw.JWTAuth()` + `apimw.TenantFromJWT()` |
| Multi-tenant isolation: all queries include `tenant_id` | ✅ PASS | `FindByTenant` always scopes by `tenant_id`; cross-tenant `GET /users/:id/roles` is explicitly a platform-admin endpoint (RBAC TODO noted) |
| Observability: Zap structured logging + Prometheus metrics | ✅ PASS | Each handler and use case will include Zap log calls; new Prometheus counters for assignment operations |
| Testing: integration tests required for new external integrations | ✅ PASS | New DB schema → migration tests + handler integration tests planned |
| No cross-surface contamination (ABM ↔ Ingesta) | ✅ PASS | All new routes exclusively on `/api/v1`; no consumer/IoT surface touched |
| Backward compatibility: no breaking changes to existing contracts | ✅ PASS | New endpoints only; existing routes and schemas unchanged |

**Gate result: ALL PASS — proceed to Phase 0.**

## Project Structure

### Documentation (this feature)

```text
specs/001-user-role-assignments/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── user-role-service-api.md
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
migrations/
├── 000003_create_roles_and_user_tenant_roles.up.sql   (NEW)
└── 000003_create_roles_and_user_tenant_roles.down.sql (NEW)

internal/domain/
├── user_roles.go         (NEW — UserTenantRole, UserRoleStatus, UserRoleWithContext)
└── errors.go             (MODIFIED — add ErrUserAlreadyHasActiveRole)

internal/repo/pg/user_roles/
├── repository.go         (NEW — UserRoleRepository interface + pgx impl)
└── resources.go          (NEW — SQL query constants)

internal/api/usecases/user_roles/
├── list_user_roles/usecase.go          (NEW)
├── assign_user_role/usecase.go         (NEW)
├── update_user_role/usecase.go         (NEW)
├── revoke_user_role/usecase.go         (NEW)
├── bulk_assign_user_roles/usecase.go   (NEW)
└── get_user_roles/usecase.go           (NEW)

internal/api/handler/user_roles/
├── list_user_roles/
│   ├── list_user_roles.go
│   └── models/response.go
├── assign_user_role/
│   ├── assign_user_role.go
│   ├── models/request.go
│   └── models/response.go
├── update_user_role/
│   ├── update_user_role.go
│   ├── models/request.go
│   └── models/response.go
├── revoke_user_role/
│   ├── revoke_user_role.go
│   └── models/response.go
├── bulk_assign_user_roles/
│   ├── bulk_assign_user_roles.go
│   ├── models/request.go
│   └── models/response.go
└── get_user_roles/
    ├── get_user_roles.go
    └── models/response.go

internal/api/router.go          (MODIFIED — add UserRoleRepo to Deps, register 6 routes)
internal/routes/url_mappings.go (MODIFIED — instantiate UserRoleRepository, pass to Deps)
```

**Structure Decision**: Single-project web service layout. Follows the established `tenants/` pattern exactly. Each use case is a separate package (fine-grained, one responsibility per package).

## Design Principles

> These principles are **NON-NEGOTIABLE** and must be respected in every file of the implementation.

### 1. Follow the tenant pattern exactly

The `tenants/` module is the **canonical reference model** for this entire feature. Before creating any new file, read its tenant equivalent and replicate the structure.

| New component | Tenant reference |
|---------------|-----------------|
| `handler/user_roles/assign_user_role/assign_user_role.go` | `handler/tenants/create_tenant/create_tenant.go` |
| `handler/user_roles/*/models/request.go` | `handler/tenants/create_tenant/models/request.go` |
| `handler/user_roles/*/models/response.go` | `handler/tenants/create_tenant/models/response.go` |
| `usecases/user_roles/assign_user_role/usecase.go` | `usecases/tenants/create_tenant/usecase.go` |
| `usecases/user_roles/update_user_role/usecase.go` | `usecases/tenants/update_tenant/usecase.go` |
| `repo/pg/user_roles/repository.go` | `repo/pg/tenants/repository.go` |
| `repo/pg/user_roles/resources.go` | `repo/pg/tenants/resources.go` |

### 2. Code conventions

- **Package names**: one package per use case, named after the directory (e.g., `package assign_user_role`)
- **Constructor pattern**: `NewXxxHandler(uc UseCase) *XxxHandler` / `NewUseCase(repo Repository) UseCase`
- **UseCase as interface**: define the `UseCase` interface in the same file as the implementation (tenant pattern)
- **Error mapping in handlers**: domain errors caught explicitly with `errors.Is()`; remainder via `httperr.WriteError(c, apperrors.NewInternalServerError(...))`
- **SQL in resources.go**: all queries as `const XxxQuery = ...`; never inlined in the repository
- **Nullable fields**: use pointers (`*string`, `*uuid.UUID`, `*time.Time`) for nullable fields; `derefString()` helper for nullable strings
- **JSON tags**: camelCase on all request/response DTOs

### 3. Response envelope (specific to this feature)

Unlike the tenants module, this feature uses the Pact-defined envelope:

```go
// Success — use gin.H directly in each handler
c.JSON(http.StatusOK, gin.H{"success": true, "data": response})

// Business error (409, 404) — catch before httperr
c.JSON(http.StatusConflict, gin.H{"success": false, "error": err.Error()})

// Internal error — delegate to the existing system
httperr.WriteError(c, apperrors.NewInternalServerError("message"))
```

### 4. Wiring in router.go and url_mappings.go

- Add `UserRoleRepo userrolesrepo.UserRoleRepository` to the `Deps` struct in `router.go`
- Instantiate the repo in `url_mappings.go` the same way as `tenantsRepository.NewTenantRepository(db)`
- Register `POST /user-roles/bulk` **before** `PUT /user-roles/:id` and `DELETE /user-roles/:id` to avoid Gin routing conflicts

### 5. Do not front-load work from other phases

- Implement in dependency order: migration → domain → repo → usecases → handlers → router
- Do not implement real RBAC (project has it as TODO); leave comment `// TODO: RBAC check — platform admin only`

## Complexity Tracking

> No Constitution Check violations — this section is not applicable.
