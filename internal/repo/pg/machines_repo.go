package pg

import (
    "context"

    "github.com/tu-org/embolsadora-api/internal/domain"
    "github.com/tu-org/embolsadora-api/internal/platform"
)

// MachinesRepo provides access to machines storage.
type MachinesRepo struct{
    // TODO: hold pgx pool/conn here when wiring real DB.
}

// List lists machines for the tenant in context with keyset pagination.
// TODO: Implement keyset pagination (by created_at, id), WHERE tenant_id, and necessary indexes.
func (r *MachinesRepo) List(ctx context.Context /* cursor string, limit int */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: SELECT ... WHERE tenant_id=? ORDER BY created_at DESC, id DESC LIMIT ?
    return nil
}

// Create creates a new machine scoped to the tenant in context.
func (r *MachinesRepo) Create(ctx context.Context /* fields */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: INSERT ... RETURNING id
    return nil
}

// Update updates a machine scoped to the tenant in context.
func (r *MachinesRepo) Update(ctx context.Context /* id/fields */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: UPDATE ... WHERE tenant_id=? AND id=?
    return nil
}

// Delete deletes a machine scoped to the tenant in context.
func (r *MachinesRepo) Delete(ctx context.Context /* id */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: DELETE ... WHERE tenant_id=? AND id=?
    return nil
}
