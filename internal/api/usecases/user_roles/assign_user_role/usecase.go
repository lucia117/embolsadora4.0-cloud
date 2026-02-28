package assign_user_role

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// AssignRequest holds the data needed to assign a role to a user within a tenant.
type AssignRequest struct {
	UserID     uuid.UUID
	TenantID   uuid.UUID
	RoleID     string
	AssignedBy *uuid.UUID
}

// UseCase defines the interface for assigning a role to a user.
type UseCase interface {
	Execute(ctx context.Context, req AssignRequest) (*domain.UserTenantRole, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new assign_user_role use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute assigns a role to a user within a tenant.
// Returns ErrUserAlreadyHasActiveRole if the user already has an active role in that tenant.
func (uc *useCase) Execute(ctx context.Context, req AssignRequest) (*domain.UserTenantRole, error) {
	now := time.Now()
	utr := &domain.UserTenantRole{
		UserID:     req.UserID,
		TenantID:   req.TenantID,
		RoleID:     &req.RoleID,
		Status:     domain.UserRoleStatusActive,
		AssignedBy: req.AssignedBy,
		AssignedAt: &now,
	}
	return uc.repo.Create(ctx, utr)
}
