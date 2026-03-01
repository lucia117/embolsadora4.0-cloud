package update_user_role

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for updating the role on an existing assignment.
type UseCase interface {
	Execute(ctx context.Context, id uuid.UUID, roleID string) (*domain.UserTenantRole, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new update_user_role use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute updates the roleId on an existing UTR assignment.
// Returns ErrAssignmentNotFound if the assignment does not exist.
func (uc *useCase) Execute(ctx context.Context, id uuid.UUID, roleID string) (*domain.UserTenantRole, error) {
	utr, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if utr == nil {
		return nil, domain.ErrAssignmentNotFound
	}

	utr.RoleID = &roleID
	return uc.repo.Update(ctx, utr)
}
