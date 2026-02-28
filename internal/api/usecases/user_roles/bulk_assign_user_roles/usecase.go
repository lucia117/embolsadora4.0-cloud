package bulk_assign_user_roles

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// BulkAssignRequest holds the data needed to bulk-assign the same role to multiple users.
type BulkAssignRequest struct {
	UserIDs   []uuid.UUID
	TenantID  uuid.UUID
	RoleID    string
	AssignedBy *uuid.UUID
}

// BulkAssignResult holds the result of a bulk assignment operation.
type BulkAssignResult struct {
	Assigned    int
	Failed      int
	Assignments []domain.UserTenantRole
}

// UseCase defines the interface for bulk-assigning roles.
type UseCase interface {
	Execute(ctx context.Context, req BulkAssignRequest) (*BulkAssignResult, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new bulk_assign_user_roles use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute bulk-assigns the same role to multiple users in an all-or-nothing transaction.
// Returns ErrUserAlreadyHasActiveRole if any user already has an active role in the tenant.
func (uc *useCase) Execute(ctx context.Context, req BulkAssignRequest) (*BulkAssignResult, error) {
	now := time.Now()
	utrs := make([]domain.UserTenantRole, 0, len(req.UserIDs))

	for _, userID := range req.UserIDs {
		utr := domain.UserTenantRole{
			UserID:     userID,
			TenantID:   req.TenantID,
			RoleID:     &req.RoleID,
			Status:     domain.UserRoleStatusActive,
			AssignedBy: req.AssignedBy,
			AssignedAt: &now,
		}
		utrs = append(utrs, utr)
	}

	assignments, err := uc.repo.BulkCreate(ctx, utrs)
	if err != nil {
		return nil, err
	}

	return &BulkAssignResult{
		Assigned:    len(assignments),
		Failed:      0,
		Assignments: assignments,
	}, nil
}
