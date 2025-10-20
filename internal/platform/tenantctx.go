package platform

import "context"

// TODO: Define tenant context helpers and propagation.

type tenantKeyType struct{}

var tenantKey = tenantKeyType{}

// WithTenantID returns a new context carrying the given tenant ID.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
    return context.WithValue(ctx, tenantKey, tenantID)
}

// TenantID extracts the tenant ID from context, or empty string if none.
func TenantID(ctx context.Context) string {
    v := ctx.Value(tenantKey)
    if s, ok := v.(string); ok {
        return s
    }
    return ""
}
