# Tenant User-Roles Enrichment Design

## Context

`embolsadora-frontend`'s `/tenants/[id]` page has a "Usuarios y roles" tab
(`tenant-users-roles-table.tsx`) that lists the users assigned to a tenant via
`GET /api/v1/user-roles?tenantId=`. Integration testing against this backend
(documented in `issues-20-07-2026.md`) found two display bugs on that tab:

1. The role column shows the raw `roleId` (e.g. `admin`) instead of a
   human-readable name.
2. The user column sometimes shows a placeholder like `Usuario a1b2c3d4…`
   instead of the user's real name or email.

## Problem

`GET /api/v1/user-roles?tenantId=` (`list_user_roles`) returns only
`roleId` — no role name. To work around this, the frontend does a second
fetch to `GET /api/v1/users?tenantId=` and joins client-side by `userId`
(`tenant-users-roles-table.tsx:69-98`, already flagged separately as a
sequential N+1 pattern, CR-5 in the PR #46 frontend code review).

That second fetch is also the root cause of bug 2, not just a missing
field: `GET /api/v1/users?tenantId=` filters by the `users.tenant_id`
column. A user who was auto-provisioned via Supabase JWT (`users.tenant_id
IS NULL` — see `ProvisionUser()` in `auth_usecase.go`) and later assigned to
a tenant purely through a `user_tenant_roles` (UTR) row never appears in
that query, even though they have a real, active assignment. The frontend
then can't find a match in its client-side map and falls back to the
`Usuario ${id.slice(0,8)}…` placeholder. The frontend's own source already
carries a comment anticipating this exact fix (`tenant-users-roles-table.tsx:70-72`):
"hasta que el backend incluya a los usuarios aprovisionados en GET /users
... o enriquezca GET /user-roles".

`users.tenant_id` is not the source of truth for tenant membership —
`user_tenant_roles` is. Any query that resolves "which user is this
assignment for" by filtering `users` on `tenant_id` will systematically miss
auto-provisioned users, regardless of which endpoint does it.

## Non-Goals

- No frontend changes. `tenant-users-roles-table.tsx` (stop rendering raw
  `roleId`, consume the new `user` object, drop the second fetch) is a
  separate, already-identified follow-up in `embolsadora-frontend`, tracked
  independently of this spec.
- No change to `GET /api/v1/users?tenantId=`. That endpoint's `tenant_id`
  filtering may have its own bugs (see the still-open `/users` page issues
  in `issues-20-07-2026.md`), but that's a different, unrelated data path —
  out of scope here.
- No change to `assign_user_role`, `update_user_role`, `revoke_user_role`,
  or `bulk_assign_user_roles`. Each has its own private response model
  (confirmed by reading all four `models/response.go` files) and none of
  them share code with `list_user_roles` beyond the `domain.UserTenantRole`
  base struct — this spec touches only `list_user_roles`'s query, domain
  type, and response model.
- No change to `get_user_roles` (`GET /users/:id/roles`, used for a
  different view — a user's roles across tenants). It already returns
  `RoleName` via `domain.UserRoleWithContext` / `FindByUserQuery`; this spec
  reuses that existing JOIN pattern for a different query, it doesn't touch
  the existing one.

## Design

### 1. Query change: JOIN on the real membership relation

`internal/repo/pg/user_roles/resources.go`'s `FindByTenantQuery` and
`FindByTenantWithStatusQuery` currently select only from `user_tenant_roles`.
Add:

- `JOIN users u ON u.id = utr.user_id` — a UTR row's `user_id` has a
  `NOT NULL` foreign key to `users.id` (enforced by
  `user_tenant_roles_user_id_fkey`), so this is always resolvable and never
  drops a row. This replaces the frontend's broken `users.tenant_id`-based
  lookup with the actual membership relation.
- `LEFT JOIN roles r ON r.id = utr.role_id` — `LEFT` because `role_id` is
  nullable (pending assignments have no role yet); this exactly mirrors the
  existing `FindByUserQuery` pattern in the same file.

### 2. New domain type: `UserTenantRoleDetail`

`FindByTenant` currently returns `[]domain.UserTenantRole`. It has exactly
one caller (`list_user_roles`'s usecase — confirmed by repo-wide grep), so
its return type can change without touching any other endpoint.

Add to `internal/domain/user_roles.go`:

```go
// UserTenantRoleDetail is returned by GET /user-roles?tenantId=...
// It embeds UserTenantRole plus role and user display fields resolved via
// JOIN, so callers don't need a second round-trip to render a name.
type UserTenantRoleDetail struct {
	UserTenantRole
	RoleName       string  // "" when RoleID is nil or the role has no name
	UserEmail      string
	UserName       *string
	UserFirstName  *string
	UserLastName   *string
}
```

`FindByTenant`'s signature becomes
`FindByTenant(ctx, tenantID, status) ([]domain.UserTenantRoleDetail, error)`.

### 3. Response shape: additive only

`internal/api/handler/user_roles/list_user_roles/models/response.go`'s
`UserRoleResponse` keeps every existing field unchanged (`id`, `userId`,
`tenantId`, `roleId`, `status`, `assignedBy`, `assignedAt`, `createdAt`,
`updatedAt`) — zero breaking changes for any existing consumer. It gains:

```go
RoleName string       `json:"roleName"`
User     *UserSummary `json:"user"`
```

```go
type UserSummary struct {
	Email     string  `json:"email"`
	Name      *string `json:"name"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}
```

`User` is always non-nil in practice (the `JOIN users` guarantees a match),
but the field is a pointer for consistency with how this codebase already
models optional nested data.

## Testing

There are currently no tests for `list_user_roles` or `FindByTenant` at any
layer (confirmed: no `_test.go` files exist under
`internal/api/usecases/user_roles/list_user_roles/`,
`internal/api/handler/user_roles/list_user_roles/`, or
`internal/repo/pg/user_roles/`).

Add one integration test, following the existing pattern in
`internal/repo/pg/users/users_repo_test.go` (`DATABASE_URL` env var, `t.Skip`
if unset, `testify` assertions, raw-SQL setup/teardown):

`internal/repo/pg/user_roles/repository_test.go` —
`TestFindByTenant_ResolvesUserAndRoleAcrossJoin`:
- Creates a user row with `tenant_id = NULL` (simulating auto-provisioning)
  and a distinct email/first/last name.
- Creates a UTR row assigning that user an existing seeded role (e.g.
  `operario`) in a test tenant.
- Calls `FindByTenant` for that tenant and asserts the returned
  `UserTenantRoleDetail` has the correct `RoleName` (`"Ver Alertas"`-style
  Spanish name from migration 000005, matched against whatever
  `roles.name` actually is for the role used) and the correct `UserEmail`/
  `UserName`/`UserFirstName`/`UserLastName` — proving the fix works
  specifically for the auto-provisioned (`tenant_id IS NULL`) case that was
  broken before.
- Also asserts a pending assignment (`role_id = NULL`) yields
  `RoleName == ""` rather than an error.
- Cleans up both rows after the assertions.
