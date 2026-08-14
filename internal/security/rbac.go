package security

import (
	"context"
	"fmt"
	"slices"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// Role represents a named role.
type Role string

// Permission uses the form "resource:action" (e.g., "users:read").
type Permission string

// rolePermissions maps role names to their allowed permissions.
var rolePermissions = map[string][]string{
	// Platform-level role: MRG employees who manage the platform (tenants, all users, all machines).
	// Members of the mrg-internal tenant only.
	"super_admin": {
		"tenants:read", "tenants:write",
		"users:read", "users:write",
		"invitations:write",
		"machines:read", "machines:write",
	},
	// Platform-level support role: MRG operators with cross-tenant read access
	// and limited write capabilities. Members of the MRG tenant only.
	"tenant_manager": {
		"tenants:read",
		"users:read",
		"invitations:write",
		"machines:read",
	},
	"admin": {"users:read", "users:write", "invitations:write", "machines:read", "machines:write", "tenants:read"},
	// Effective role (not in the roles catalog): assigned by TenantFromHeader to
	// users whose `admin` membership belongs to the platform tenant (MRG).
	// Same as admin plus tenant management.
	"platform_admin":   {"users:read", "users:write", "invitations:write", "machines:read", "machines:write", "tenants:read", "tenants:write"},
	"operario":         {"machines:read", "machines:write"},
	"cliente_admin":    {"users:read", "users:write", "invitations:write", "machines:read"},
	"cliente_operario": {"machines:read"},
}

// crossTenantRoles lists roles allowed to act on any tenant, not just their own.
// super_admin and tenant_manager are the DB-seeded global roles; platform_admin
// is the effective role TenantFromHeader assigns to admins of the MRG platform
// tenant acting cross-tenant (see ADR-015).
var crossTenantRoles = map[string]bool{
	"super_admin":    true,
	"tenant_manager": true,
	"platform_admin": true,
}

// IsCrossTenantRole reports whether roleName may act on tenants other than its own.
func IsCrossTenantRole(roleName string) bool {
	return crossTenantRoles[roleName]
}

// platformTenantAdminRole es el rol efectivo que toma un `admin` cuya membresía
// pertenece al tenant plataforma de MRG: mismos permisos que admin más
// tenants:write. No existe en la tabla `roles` — es una derivación en runtime.
const platformTenantAdminRole = "platform_admin"

// EffectiveRole traduce el role_id almacenado en user_tenant_roles al rol con el
// que el usuario actúa realmente. Única definición de la regla: la consumen
// TenantFromHeader (para el enforcement) y GetMe (para lo que ve el frontend).
// Si divergieran, el frontend mostraría capacidades que el backend niega.
func EffectiveRole(roleID string, isPlatformTenant bool) string {
	if roleID == "admin" && isPlatformTenant {
		return platformTenantAdminRole
	}
	return roleID
}

// CanSeePlatformInternals reporta si el caller puede ver las internas de
// plataforma: roles globales (super_admin, tenant_manager), sus miembros y las
// invitaciones a esos roles. Solo super_admin — ni tenant_manager ni
// platform_admin, que pertenecen a la misma capa pero no la administran.
//
// Fail-closed: sin rol en contexto devuelve false.
func CanSeePlatformInternals(ctx context.Context) bool {
	return RoleFromContext(ctx) == "super_admin"
}

// roleContextKeyType is an unexported type to store role in context.
type roleContextKeyType struct{}

var roleContextKey = roleContextKeyType{}

// WithRole stores the user's role name in context (set by me_usecase or TenantFromHeader).
func WithRole(ctx context.Context, roleName string) context.Context {
	return context.WithValue(ctx, roleContextKey, roleName)
}

// RoleFromContext extracts the role name from context.
func RoleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(roleContextKey).(string); ok {
		return v
	}
	return ""
}

// PermissionsForRole returns the list of permissions for a given role name.
func PermissionsForRole(roleName string) []string {
	perms, ok := rolePermissions[roleName]
	if !ok {
		return []string{}
	}
	result := make([]string, len(perms))
	copy(result, perms)
	return result
}

// Can checks whether the caller in context has the given permission.
// Returns domain.ErrForbidden if the user lacks the permission.
func Can(ctx context.Context, perm string) error {
	// Get role from context (set by TenantFromHeader after tenant validation)
	roleName := RoleFromContext(ctx)
	if roleName == "" {
		// Fallback: try to derive role from domain user + tenant membership
		// This will be wired fully in Phase 5 (me_usecase)
		_ = platform.TenantID(ctx)
		return domain.ErrForbidden
	}

	perms, ok := rolePermissions[roleName]
	if !ok {
		return fmt.Errorf("%w: unknown role %q", domain.ErrForbidden, roleName)
	}

	if slices.Contains(perms, perm) {
		return nil
	}

	return fmt.Errorf("%w: role %q lacks permission %q", domain.ErrForbidden, roleName, perm)
}
