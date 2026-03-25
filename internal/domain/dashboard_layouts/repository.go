package dashboard_layouts

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence contract for dashboard layouts.
type Repository interface {
	// List returns all active layouts for the given tenant, ordered by creation date.
	List(ctx context.Context, tenantID uuid.UUID) ([]*DashboardLayout, error)

	// GetByID returns a single active layout by ID within the tenant.
	// Returns ErrLayoutNotFound if not found.
	GetByID(ctx context.Context, tenantID, layoutID uuid.UUID) (*DashboardLayout, error)

	// CountByTenant returns the number of active layouts for the tenant.
	CountByTenant(ctx context.Context, tenantID uuid.UUID) (int, error)

	// ExistsByName returns true if an active layout with the given name exists in the tenant.
	// excludeID, when non-nil, excludes that layout from the check (used for update self-exclusion).
	ExistsByName(ctx context.Context, tenantID uuid.UUID, name string, excludeID *uuid.UUID) (bool, error)

	// Create persists a new layout.
	Create(ctx context.Context, layout *DashboardLayout) error

	// Update replaces the name and widgets of an existing layout and refreshes updated_at.
	// Returns ErrLayoutNotFound if the layout does not exist.
	Update(ctx context.Context, layout *DashboardLayout) error

	// SoftDelete marks a layout as deleted by setting deleted_at.
	// Returns ErrLayoutNotFound if the layout does not exist.
	SoftDelete(ctx context.Context, tenantID, layoutID uuid.UUID) error
}
