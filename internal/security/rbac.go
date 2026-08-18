package security

import (
	"context"
	"fmt"
	"slices"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Role represents a named role.
type Role string

// Permission uses the perm_* catalog ids from the `permissions` table
// (e.g., "perm_users_view"). Antes de esta versión el enforcement usaba un
// vocabulario "resource:action" propio (users:read, tenants:write) definido
// en un mapa Go hardcodeado; ahora es el mismo catálogo perm_* que ya
// consumía el frontend (GET /me), cargado desde roles.permissions.
type Permission string

// RoleContext agrupa todo lo que TenantFromHeader resuelve una vez por
// request sobre el rol efectivo del caller: su nombre, el catálogo perm_*
// que tiene asignado (roles.permissions) y si puede actuar cross-tenant
// (roles.is_global). Can() e IsCrossTenantRole() leen de acá — ningún mapa
// hardcodeado queda en este archivo.
type RoleContext struct {
	Name        string
	Permissions []string
	IsGlobal    bool
}

// platformTenantAdminRole es el rol efectivo que toma un `admin` cuya membresía
// pertenece al tenant plataforma de MRG: mismos permisos que admin más
// tenants:write. Existe como fila real en `roles` desde la migración 000011
// (antes era 100% virtual, derivado solo en runtime).
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

// roleContextKeyType is an unexported type to store RoleContext in context.
type roleContextKeyType struct{}

var roleContextKey = roleContextKeyType{}

// WithRoleContext stores the caller's resolved role (name, permissions,
// is_global) in context. Called once per request by
// middleware.TenantFromHeader after resolving the effective role.
func WithRoleContext(ctx context.Context, rc RoleContext) context.Context {
	return context.WithValue(ctx, roleContextKey, rc)
}

// RoleContextFromContext extracts the RoleContext. Returns the zero value
// (empty name, nil permissions, IsGlobal=false) if none was set — fail-closed.
func RoleContextFromContext(ctx context.Context) RoleContext {
	if rc, ok := ctx.Value(roleContextKey).(RoleContext); ok {
		return rc
	}
	return RoleContext{}
}

// WithRole is a convenience wrapper for callers that only need the role name
// (tests, CanSeePlatformInternals-style checks) without a real permission
// set. Request code should use WithRoleContext instead.
func WithRole(ctx context.Context, roleName string) context.Context {
	return WithRoleContext(ctx, RoleContext{Name: roleName})
}

// RoleFromContext extracts just the role name, for consumers that don't need
// the permission list (logging, telemetry, CanSeePlatformInternals).
func RoleFromContext(ctx context.Context) string {
	return RoleContextFromContext(ctx).Name
}

// IsCrossTenantRole reports whether the caller in context may act on tenants
// other than its own. Backed by roles.is_global, loaded into context by
// middleware.TenantFromHeader — never call this outside a request that went
// through that middleware (GET /me does not; it computes its own
// cross-tenant flag locally, see usecases/me_usecase.go).
func IsCrossTenantRole(ctx context.Context) bool {
	return RoleContextFromContext(ctx).IsGlobal
}

// Can checks whether the caller in context has the given permission.
// Returns domain.ErrForbidden if the user lacks the permission, or if no
// role is set in context at all (fail-closed).
func Can(ctx context.Context, perm string) error {
	rc := RoleContextFromContext(ctx)
	if rc.Name == "" {
		return domain.ErrForbidden
	}
	if slices.Contains(rc.Permissions, perm) {
		return nil
	}
	return fmt.Errorf("%w: role %q lacks permission %q", domain.ErrForbidden, rc.Name, perm)
}
