package get_public_tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var ErrTenantNotFound = errors.New("tenant not found")

type UseCase struct {
	repo tenants.TenantRepository
}

func NewUseCase(repo tenants.TenantRepository) *UseCase {
	return &UseCase{repo: repo}
}

// Execute resolves a tenant by its UUID or its subdomain — whichever
// idOrSubdomain parses as. No auth/tenant-membership check: this backs the
// unauthenticated public tenant lookup (invitation callback link, public
// landing page), so it deliberately doesn't distinguish "doesn't exist" from
// "exists but inactive" — both come back as ErrTenantNotFound.
func (uc *UseCase) Execute(ctx context.Context, idOrSubdomain string) (*domain.Tenant, error) {
	var tenant *domain.Tenant
	var err error

	if id, parseErr := uuid.Parse(idOrSubdomain); parseErr == nil {
		tenant, err = uc.repo.FindByID(ctx, id)
	} else {
		tenant, err = uc.repo.FindBySubdomain(ctx, idOrSubdomain)
	}
	if err != nil {
		return nil, err
	}

	if tenant == nil || !tenant.IsActive {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}
