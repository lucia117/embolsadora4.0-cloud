package dashboard_layouts

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence contract for dashboard layouts.
type Repository interface {
	// List returns all active layouts for the given (tenant, user), ordered by creation date.
	List(ctx context.Context, tenantID, userID uuid.UUID) ([]*DashboardLayout, error)

	// GetByID returns a single active layout by ID within the (tenant, user) scope.
	// Returns ErrLayoutNotFound if not found.
	GetByID(ctx context.Context, tenantID, userID, layoutID uuid.UUID) (*DashboardLayout, error)

	// CountByTenantUser returns the number of active layouts for the (tenant, user) pair.
	CountByTenantUser(ctx context.Context, tenantID, userID uuid.UUID) (int, error)

	// Create persists a new layout.
	Create(ctx context.Context, layout *DashboardLayout) error

	// Update replaces the name and widgets of an existing layout and refreshes updated_at.
	// Returns ErrLayoutNotFound if the layout does not exist.
	Update(ctx context.Context, layout *DashboardLayout) error

	// SoftDelete marks a layout as deleted by setting deleted_at.
	// Returns ErrLayoutNotFound if the layout does not exist.
	SoftDelete(ctx context.Context, tenantID, userID, layoutID uuid.UUID) error
}
