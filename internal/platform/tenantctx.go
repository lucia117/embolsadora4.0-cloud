package platform

import (
	"context"

	"github.com/google/uuid"
)

type tenantKeyType struct{}
type userKeyType struct{}
type supabaseSubKeyType struct{}
type domainUserKeyType struct{}
type userEmailKeyType struct{}
type tenantUUIDKeyType struct{}

var tenantKey = tenantKeyType{}
var userKey = userKeyType{}
var supabaseSubKey = supabaseSubKeyType{}
var domainUserKey = domainUserKeyType{}
var userEmailKey = userEmailKeyType{}
var tenantUUIDKey = tenantUUIDKeyType{}

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

// WithUserID returns a new context carrying the authenticated user's UUID.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userKey, userID)
}

// UserID extracts the authenticated user's ID from context.
// Returns nil if no user ID is present.
func UserID(ctx context.Context) *uuid.UUID {
	v := ctx.Value(userKey)
	if id, ok := v.(uuid.UUID); ok {
		return &id
	}
	return nil
}

// WithSupabaseSub stores the Supabase JWT subject (sub claim) in context.
func WithSupabaseSub(ctx context.Context, sub string) context.Context {
	return context.WithValue(ctx, supabaseSubKey, sub)
}

// SupabaseSub extracts the Supabase subject from context.
func SupabaseSub(ctx context.Context) string {
	if v, ok := ctx.Value(supabaseSubKey).(string); ok {
		return v
	}
	return ""
}

// WithUserEmail returns a new context carrying the authenticated user's email.
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, userEmailKey, email)
}

// UserEmail extracts the authenticated user's email from context.
// Returns empty string if no email is present.
func UserEmail(ctx context.Context) string {
	v := ctx.Value(userEmailKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// DomainUserValue is a type alias used to store the provisioned domain user in context.
// Using interface{} to avoid import cycles; callers cast to *domain.User.
type DomainUserValue interface{}

// WithDomainUser stores the provisioned domain user in context.
func WithDomainUser(ctx context.Context, user DomainUserValue) context.Context {
	return context.WithValue(ctx, domainUserKey, user)
}

// DomainUser extracts the domain user from context.
func DomainUser(ctx context.Context) DomainUserValue {
	return ctx.Value(domainUserKey)
}

// WithTenantUUID returns a new context carrying the given tenant UUID.
func WithTenantUUID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantUUIDKey, tenantID)
}

// TenantUUID extracts the tenant UUID from context.
// Returns nil if no tenant UUID is present.
func TenantUUID(ctx context.Context) *uuid.UUID {
	v := ctx.Value(tenantUUIDKey)
	if id, ok := v.(uuid.UUID); ok {
		return &id
	}
	return nil
}
