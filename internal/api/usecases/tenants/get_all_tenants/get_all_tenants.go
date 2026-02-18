package get_all_tenants

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

type UseCase struct {
	repo tenants.TenantRepository
}

func NewUseCase(repo tenants.TenantRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context) ([]domain.Tenant, error) {
	tenants, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	return tenants, nil
}
