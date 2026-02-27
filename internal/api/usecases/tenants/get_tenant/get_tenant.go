package get_tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var (
	ErrTenantNotFound = errors.New("tenant not found")
)

type UseCase struct {
	repo tenants.TenantRepository
}

func NewUseCase(repo tenants.TenantRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Execute(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	tenant, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}
