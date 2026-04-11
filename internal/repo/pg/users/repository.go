package users

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/domain/users"
)

// Repository defines user persistence operations
type Repository interface {
	// ListByTenant retrieves paginated users belonging to a tenant (excludes soft-deleted)
	ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*users.User, int64, error)

	// GetByID retrieves a single user by ID (returns ErrNotFound if soft-deleted or not found)
	GetByID(ctx context.Context, tenantID, userID string) (*users.User, error)

	// GetByIDWithRoles retrieves a user with their active role assignment in the tenant.
	// Returns ErrNotFound if user doesn't exist or is soft-deleted.
	// The Roles field is an empty slice if no active UTR is found.
	GetByIDWithRoles(ctx context.Context, tenantID, userID string) (*users.UserWithRoles, error)

	// ListPendingByTenant retrieves users with a pending UTR in the tenant.
	ListPendingByTenant(ctx context.Context, tenantID string) ([]*users.User, error)

	// Create inserts a new user and returns it with server-generated fields
	// Returns ErrEmailTaken if email exists in same tenant
	Create(ctx context.Context, user *users.User) (*users.User, error)

	// CreateWithRole inserts a new user and an active UTR in a single transaction.
	// Returns ErrEmailTaken if email exists in the same tenant.
	// Returns domain.ErrInvalidRoleID if the role_id does not exist in the roles table.
	CreateWithRole(ctx context.Context, user *users.User, utr *domain.UserTenantRole) (*users.User, error)

	// Update modifies user fields (name, role, image only - email/tenantId immutable)
	// Returns ErrNotFound if user doesn't exist or is soft-deleted
	Update(ctx context.Context, user *users.User) (*users.User, error)

	// Delete performs soft delete: sets deleted_at timestamp
	// Returns ErrNotFound if user doesn't exist
	Delete(ctx context.Context, tenantID, userID string) error
}
