package revoke_user_role

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for revoking a user-role assignment.
type UseCase interface {
	Execute(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new revoke_user_role use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute soft-deletes a UTR assignment by setting its status to 'revoked'.
// Returns ErrAssignmentNotFound if the assignment does not exist.
func (uc *useCase) Execute(ctx context.Context, id uuid.UUID) (*domain.UserTenantRole, error) {
	result, err := uc.repo.Revoke(ctx, id)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, domain.ErrAssignmentNotFound
	}
	return result, nil
}
