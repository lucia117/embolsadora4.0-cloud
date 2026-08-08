package bulk_assign_user_roles

import (
	"context"
	"time"

	"github.com/google/uuid"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// BulkAssignRequest holds the data needed to bulk-assign the same role to multiple users.
type BulkAssignRequest struct {
	UserIDs    []uuid.UUID
	TenantID   uuid.UUID
	RoleID     string
	AssignedBy *uuid.UUID
	// IncludeGlobal lo calcula el handler con security.CanSeePlatformInternals.
	IncludeGlobal bool
}

// BulkAssignResult holds the result of a bulk assignment operation.
// The operation is all-or-nothing: BulkCreate runs in a single transaction,
// so a non-nil error here means nothing was persisted.
type BulkAssignResult struct {
	Assigned    int
	Assignments []domain.UserTenantRole
}

// UseCase defines the interface for bulk-assigning roles.
type UseCase interface {
	Execute(ctx context.Context, req BulkAssignRequest) (*BulkAssignResult, error)
}

type useCase struct {
	repo     userrolesrepo.UserRoleRepository
	roleRepo rolesRepo.Repository
}

// NewUseCase creates a new bulk_assign_user_roles use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository, roleRepo rolesRepo.Repository) UseCase {
	return &useCase{repo: repo, roleRepo: roleRepo}
}

// Execute bulk-assigns the same role to multiple users in an all-or-nothing transaction.
// Returns ErrUserAlreadyHasActiveRole if any user already has an active role in the tenant.
//
// La validación del rol corre una sola vez, antes de abrir la transacción: el batch
// entero comparte tenant y rol (ver BulkAssignRequest). Sin ella, POST /user-roles/bulk
// era la misma escalada que POST /user-roles pero en lote.
func (uc *useCase) Execute(ctx context.Context, req BulkAssignRequest) (*BulkAssignResult, error) {
	if err := appRoles.EnsureAssignable(ctx, uc.roleRepo, req.RoleID, req.TenantID, req.IncludeGlobal); err != nil {
		return nil, err
	}

	now := time.Now()
	utrs := make([]domain.UserTenantRole, 0, len(req.UserIDs))

	for _, userID := range req.UserIDs {
		utr := domain.UserTenantRole{
			ID:         uuid.New(),
			UserID:     userID,
			TenantID:   req.TenantID,
			RoleID:     &req.RoleID,
			Status:     domain.UserRoleStatusActive,
			AssignedBy: req.AssignedBy,
			AssignedAt: &now,
		}
		utrs = append(utrs, utr)
	}

	assignments, err := uc.repo.BulkCreate(ctx, utrs, req.IncludeGlobal)
	if err != nil {
		return nil, err
	}

	return &BulkAssignResult{
		Assigned:    len(assignments),
		Assignments: assignments,
	}, nil
}
