package pg

import (
    "context"

    "github.com/tu-org/embolsadora-api/internal/domain"
    "github.com/tu-org/embolsadora-api/internal/platform"
)

// TenantsRepo provides access to tenants storage.
type TenantsRepo struct{
    // TODO: hold pgx pool/conn here when wiring real DB.
}

// List lists tenants visible to the current tenant context (may be restricted).
// TODO: Define access policy; implement keyset pagination and proper indexes.
func (r *TenantsRepo) List(ctx context.Context /* cursor string, limit int */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: SELECT ... ORDER BY created_at DESC, id DESC LIMIT ?
    return nil
}

// Create creates a new tenant (likely privileged operation).
func (r *TenantsRepo) Create(ctx context.Context /* fields */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: INSERT ... RETURNING id
    return nil
}

// Update updates a tenant (likely privileged operation).
func (r *TenantsRepo) Update(ctx context.Context /* id/fields */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: UPDATE ... WHERE id=?
    return nil
}

// Delete deletes a tenant (likely privileged operation).
func (r *TenantsRepo) Delete(ctx context.Context /* id */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: DELETE ... WHERE id=?
    return nil
}
