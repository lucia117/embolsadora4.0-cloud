package create_tenant

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

// UseCase defines the interface for tenant creation use case
type UseCase interface {
	Create(ctx context.Context, tenant *domain.Tenant) error
}

type useCase struct {
	repo tenants.TenantRepository
}

// NewUseCase creates a new instance of the tenant creation use case
func NewUseCase(repo tenants.TenantRepository) UseCase {
	return &useCase{
		repo: repo,
	}
}

// Create creates a new tenant
func (uc *useCase) Create(ctx context.Context, tenant *domain.Tenant) error {
	return uc.repo.Create(ctx, tenant)
}
