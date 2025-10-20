package pg

import (
    "context"

    "github.com/tu-org/embolsadora-api/internal/domain"
    "github.com/tu-org/embolsadora-api/internal/platform"
)

// UsersRepo provides access to users storage.
type UsersRepo struct{
    // TODO: hold pgx pool/conn here when wiring real DB.
}

// List lists users for the tenant in context with keyset pagination.
// TODO: Implement keyset pagination (by created_at, id), proper WHERE on tenant_id, and indexes.
func (r *UsersRepo) List(ctx context.Context /* cursor string, limit int */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: SELECT with WHERE tenant_id=?, ORDER BY created_at DESC, id DESC, LIMIT ?, and next_cursor computation.
    return nil
}

// Create creates a new user scoped to the tenant in context.
// TODO: INSERT with tenant_id partitioning/indexing strategy.
func (r *UsersRepo) Create(ctx context.Context /* user fields */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: INSERT ... RETURNING id
    return nil
}

// Update updates a user scoped to the tenant in context.
// TODO: UPDATE with WHERE tenant_id AND id filters; optimistic locking if needed.
func (r *UsersRepo) Update(ctx context.Context /* user id/fields */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: UPDATE ... WHERE tenant_id=? AND id=?
    return nil
}

// Delete deletes a user scoped to the tenant in context.
// TODO: Soft-delete vs hard-delete policy; proper indexing.
func (r *UsersRepo) Delete(ctx context.Context /* user id */) error {
    if platform.TenantID(ctx) == "" {
        return domain.ErrForbidden
    }
    // TODO: DELETE ... WHERE tenant_id=? AND id=?
    return nil
}
