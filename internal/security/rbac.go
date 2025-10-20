package security

// TODO: Define RBAC roles/permissions scaffolding and middleware wiring.

// Role represents a named role.
type Role string

// Permission uses the form "resource:action" (e.g., "users:list").
type Permission string

// Can checks whether the caller in context has the given permission.
// TODO: Implement extraction from context (tenant, roles), mapping to permissions, and evaluation.
func Can(ctx interface{}, perm string) error {
    // TODO: replace ctx type with context.Context when wiring; return nil if allowed, error otherwise.
    return nil
}
