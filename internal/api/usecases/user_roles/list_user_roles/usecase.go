package list_user_roles

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for listing user-role assignments for a tenant.
type UseCase interface {
	Execute(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRole, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new list_user_roles use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute returns all UTR assignments for a tenant, optionally filtered by status.
func (uc *useCase) Execute(ctx context.Context, tenantID uuid.UUID, status *string) ([]domain.UserTenantRole, error) {
	return uc.repo.FindByTenant(ctx, tenantID, status)
}
