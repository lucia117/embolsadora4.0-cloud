package get_user_roles

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for retrieving a user's roles across all tenants.
type UseCase interface {
	Execute(ctx context.Context, userID uuid.UUID) ([]domain.UserRoleWithContext, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new get_user_roles use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute retrieves all role assignments for a user across all tenants.
// TODO: RBAC check — platform admin only
func (uc *useCase) Execute(ctx context.Context, userID uuid.UUID) ([]domain.UserRoleWithContext, error) {
	return uc.repo.FindByUser(ctx, userID)
}
