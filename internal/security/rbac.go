package security

import (
	"context"
	"fmt"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// Role represents a named role.
type Role string

// Permission uses the form "resource:action" (e.g., "users:read").
type Permission string

// rolePermissions maps role names to their allowed permissions.
var rolePermissions = map[string][]string{
	"admin":            {"users:read", "users:write", "invitations:write", "machines:read", "machines:write", "tenants:read"},
	"operario":         {"machines:read", "machines:write"},
	"cliente_admin":    {"users:read", "invitations:write", "machines:read"},
	"cliente_operario": {"machines:read"},
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

	for _, p := range perms {
		if p == perm {
			return nil
		}
	}

	return fmt.Errorf("%w: role %q lacks permission %q", domain.ErrForbidden, roleName, perm)
}
