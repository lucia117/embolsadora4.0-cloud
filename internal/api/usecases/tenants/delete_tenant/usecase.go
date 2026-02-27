package delete_tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var ErrTenantNotFound = errors.New("tenant not found")

// UseCase defines the interface for tenant deletion use case
type UseCase interface {
	Delete(ctx context.Context, id uuid.UUID) error
}

type useCase struct {
	repo tenants.TenantRepository
}

// NewUseCase creates a new instance of the tenant deletion use case
func NewUseCase(repo tenants.TenantRepository) UseCase {
	return &useCase{
		repo: repo,
	}
}

// Delete deletes an existing tenant
func (uc *useCase) Delete(ctx context.Context, id uuid.UUID) error {
	err := uc.repo.Delete(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return ErrTenantNotFound
	}
	return err
}
