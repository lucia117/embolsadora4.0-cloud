package assign_user_role

import (
	"context"
	"time"

	"github.com/google/uuid"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// AssignRequest holds the data needed to assign a role to a user within a tenant.
type AssignRequest struct {
	UserID     uuid.UUID
	TenantID   uuid.UUID
	RoleID     string
	AssignedBy *uuid.UUID
	// IncludeGlobal lo calcula el handler con security.CanSeePlatformInternals y
	// viaja explícito hasta el repo; nadie lee el contexto acá abajo.
	IncludeGlobal bool
}

// UseCase defines the interface for assigning a role to a user.
type UseCase interface {
	Execute(ctx context.Context, req AssignRequest) (*domain.UserTenantRole, error)
}

type useCase struct {
	repo     userrolesrepo.UserRoleRepository
	roleRepo rolesRepo.Repository
}

// NewUseCase creates a new assign_user_role use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository, roleRepo rolesRepo.Repository) UseCase {
	return &useCase{repo: repo, roleRepo: roleRepo}
}

// Execute assigns a role to a user within a tenant.
// Returns ErrUserAlreadyHasActiveRole if the user already has an active role in that tenant.
//
// El role_id pedido se valida ANTES de crear nada (roles.EnsureAssignable): sin eso un
// platform_admin podía asignarse 'super_admin' a sí mismo. Ver EnsureAssignable.
func (uc *useCase) Execute(ctx context.Context, req AssignRequest) (*domain.UserTenantRole, error) {
	if err := appRoles.EnsureAssignable(ctx, uc.roleRepo, req.RoleID, req.TenantID, req.IncludeGlobal); err != nil {
		return nil, err
	}

	now := time.Now()
	utr := &domain.UserTenantRole{
		ID:         uuid.New(),
		UserID:     req.UserID,
		TenantID:   req.TenantID,
		RoleID:     &req.RoleID,
		Status:     domain.UserRoleStatusActive,
		AssignedBy: req.AssignedBy,
		AssignedAt: &now,
	}
	return uc.repo.Create(ctx, utr, req.IncludeGlobal)
}
