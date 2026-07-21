# Tenant-ownership scoping on Tenant CRUD endpoints

**Date**: 2026-07-20
**Status**: Approved, pending implementation plan

## Context

Integration testing against the frontend (`embolsadora-frontend`) surfaced a set of
bugs across `/tenants`, `/users`, `/roles`, `/permissions`, `/settings` and
`/account` (tracked separately in `~/Develop/UTN/issues-20-07-2026.md`). While
investigating one of those bugs — and a related finding from the `embolsadora-frontend`
PR #46 code review about a possibly-incorrect `x-tenant-id` header on tenant
requests — this spec surfaced a deeper, unrelated backend authorization gap that
needs to be fixed on its own, ahead of the page-by-page bug fixes.

## Problem

`GET/PATCH/DELETE /api/v1/tenants/:tenantId` and `GET /api/v1/tenants` authorize
purely by permission name (`tenants:read` / `tenants:write` via
`internal/security/rbac.go`), with no check that the tenant being read or
mutated belongs to the acting user.

Concretely:
- `get_tenant.go` (`internal/api/handler/tenants/get_tenant/get_tenant.go:24`)
  parses the `:tenantId` path param and fetches that tenant directly — the use
  case (`internal/api/usecases/tenants/get_tenant/get_tenant.go:24`) never
  receives or checks the actor's own tenant.
- `update_tenant.go` and `delete_tenant.go` follow the same pattern.
- `get_all_tenants.go` returns every tenant in the database unconditionally.
- The `admin` role (tenant-scoped, non-global) has `tenants:read` in
  `rolePermissions` (`internal/security/rbac.go:36`) — so any tenant's plain
  admin can read (and, if the role map ever grants `tenants:write` to a
  tenant-scoped role, mutate) **any other tenant's record** by UUID.

Roles that legitimately act cross-tenant today — `super_admin`, `tenant_manager`,
and the effective `platform_admin` role granted to MRG-tenant admins by
`TenantFromHeader` (see ADR-015) — are unaffected by this problem; the gap only
matters for tenant-scoped roles (`admin` and friends) reaching outside their own
tenant.

## Non-goals

- The `x-tenant-id` header derivation in `embolsadora-frontend/src/proxy.ts`
  (flagged in PR #46's review) is not addressed here. Static reading of the
  proxy code was inconclusive on whether it actually misroutes the header in
  practice for the plain `/api/tenants/:id` case; it needs live verification.
  It is not blocking: once this spec's backend fix lands, the worst case of a
  wrong header value is a 403, never a cross-tenant data leak.
- The conceptual overlap between "platform tenant management" (this CRUD,
  meant for platform operators) and "self-service tenant settings" (what the
  frontend `/settings` page should really be calling) is left for the
  `/settings` + `/account` spec.

## Design

Reuse the context `TenantFromHeader` middleware already populates on every
`/api/v1` request: `platform.TenantID(ctx)` (the actor's validated tenant UUID)
and `security.RoleFromContext(ctx)` (the actor's effective role, already
upgraded to `platform_admin` for MRG-tenant admins per ADR-015).

1. **`internal/security/rbac.go`** — add a small allowlist and helper:
   ```go
   var crossTenantRoles = map[string]bool{"super_admin": true, "tenant_manager": true, "platform_admin": true}

   func IsCrossTenantRole(roleName string) bool { return crossTenantRoles[roleName] }
   ```

2. **`GetTenant` / `UpdateTenant` / `DeleteTenant` handlers** — immediately
   after parsing `:tenantId` from the path, before invoking the use case:
   ```go
   role := security.RoleFromContext(c.Request.Context())
   if !security.IsCrossTenantRole(role) && platform.TenantID(c.Request.Context()) != id.String() {
       c.JSON(http.StatusForbidden, tenantserrors.ErrorResponse{
           Error: "FORBIDDEN", Message: "No tenés acceso a este tenant", Status: http.StatusForbidden,
       })
       return
   }
   ```
   Returns a generic 403 regardless of whether the target tenant exists,
   matching the existing enumeration-avoidance convention in
   `resolve_tenant_path.go` ("same 403 to avoid enumeration").

3. **`GetAllTenants` (handler + use case)** — if the actor's role is not a
   cross-tenant role, the use case returns a single-element list containing
   only the actor's own tenant (`repo.FindByID(actorTenantID)`) instead of
   `repo.FindAll()`. This keeps the endpoint usable for tenant-scoped
   self-service callers without exposing other tenants, and avoids an
   all-or-nothing 403 that could break an existing legitimate caller.

4. Cross-tenant roles (`super_admin`, `tenant_manager`, `platform_admin`)
   keep today's unrestricted behavior — this is intentional per ADR-015 and
   out of scope to change.

## Testing

Add unit/integration coverage (pattern already exists in
`update_tenant_test.go`) for:
- Tenant-scoped `admin` of tenant A gets 403 on `GET/PATCH/DELETE
  /tenants/{B}`.
- Tenant-scoped `admin` of tenant A can still `GET/PATCH` `/tenants/{A}`.
- `super_admin`, `tenant_manager`, and `platform_admin` (MRG admin acting
  cross-tenant) remain unrestricted on all four endpoints.
- `GET /tenants` for a tenant-scoped `admin` returns exactly one tenant (their
  own); for cross-tenant roles it returns the full list as today.
