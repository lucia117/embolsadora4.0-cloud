package get_all_tenants

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

type UseCase struct {
	repo tenants.TenantRepository
}

func NewUseCase(repo tenants.TenantRepository) *UseCase {
	return &UseCase{repo: repo}
}

// Execute returns every tenant when scopeToTenantID is nil (cross-tenant roles),
// or a single-element list containing only that tenant otherwise (non-cross-tenant
// roles must not see other tenants' records via the list endpoint).
func (uc *UseCase) Execute(ctx context.Context, scopeToTenantID *uuid.UUID) ([]domain.Tenant, error) {
	if scopeToTenantID == nil {
		return uc.repo.FindAll(ctx)
	}

	tenant, err := uc.repo.FindByID(ctx, *scopeToTenantID)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return []domain.Tenant{}, nil
	}
	return []domain.Tenant{*tenant}, nil
}
