package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	domain "github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	"go.uber.org/zap"
)

// Service handles user-related business logic
type Service struct {
	repo         usersRepo.Repository
	userRoleRepo userRolesRepo.UserRoleRepository
	logger       *zap.Logger
}

// NewService creates a new user service
func NewService(repo usersRepo.Repository, userRoleRepo userRolesRepo.UserRoleRepository, logger *zap.Logger) *Service {
	return &Service{
		repo:         repo,
		userRoleRepo: userRoleRepo,
		logger:       logger,
	}
}

// ListUsers retrieves paginated users for a tenant
func (s *Service) ListUsers(ctx context.Context, tenantID string, limit, offset int) ([]*domainUsers.User, int64, error) {
	s.logger.Debug("listing users", zap.String("tenant_id", tenantID), zap.Int("limit", limit), zap.Int("offset", offset))

	users, total, err := s.repo.ListByTenant(ctx, tenantID, limit, offset)
	if err != nil {
		s.logger.Error("failed to list users", zap.String("tenant_id", tenantID), zap.Error(err))
		return nil, 0, err
	}

	s.logger.Debug("users listed", zap.String("tenant_id", tenantID), zap.Int64("total", total), zap.Int("count", len(users)))
	return users, total, nil
}

// GetUser retrieves a single user by ID
func (s *Service) GetUser(ctx context.Context, tenantID, userID string) (*domainUsers.User, error) {
	s.logger.Debug("getting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	user, err := s.repo.GetByID(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Debug("user retrieved", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
	return user, nil
}

// GetUserWithRoles retrieves a user and their active role assignment in the tenant.
func (s *Service) GetUserWithRoles(ctx context.Context, tenantID, userID string) (*domainUsers.UserWithRoles, error) {
	s.logger.Debug("getting user with roles", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	uwr, err := s.repo.GetByIDWithRoles(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user with roles", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Debug("user with roles retrieved", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Int("role_count", len(uwr.Roles)))
	return uwr, nil
}

// CreateUser creates a new user in a tenant
func (s *Service) CreateUser(ctx context.Context, tenantID string, cmd *domainUsers.CreateUserCommand) (*domainUsers.User, error) {
	if err := cmd.Validate(); err != nil {
		s.logger.Warn("invalid create user command", zap.String("tenant_id", tenantID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domainUsers.ErrValidation, err)
	}

	// Create domain object
	user := &domainUsers.User{
		TenantID:  tenantID,
		FirstName: cmd.FirstName,
		LastName:  cmd.LastName,
		Email:     cmd.Email,
		Role:      cmd.Role,
		Image:     cmd.Image,
	}

	s.logger.Debug("creating user", zap.String("tenant_id", tenantID), zap.String("email", cmd.Email))

	created, err := s.repo.Create(ctx, user)
	if err != nil {
		if errors.Is(err, domainUsers.ErrEmailTaken) {
			s.logger.Warn("email already taken", zap.String("tenant_id", tenantID), zap.String("email", cmd.Email))
			return nil, err
		}
		s.logger.Error("failed to create user", zap.String("tenant_id", tenantID), zap.String("email", cmd.Email), zap.Error(err))
		return nil, err
	}

	s.logger.Info("user created", zap.String("tenant_id", tenantID), zap.String("user_id", created.ID), zap.String("email", cmd.Email))
	return created, nil
}

// UpdateUser updates user fields (name, role, image)
func (s *Service) UpdateUser(ctx context.Context, tenantID, userID string, cmd *domainUsers.UpdateUserCommand) (*domainUsers.User, error) {
	if err := cmd.Validate(); err != nil {
		s.logger.Warn("invalid update user command", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domainUsers.ErrValidation, err)
	}

	s.logger.Debug("updating user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	// Get current user
	current, err := s.repo.GetByID(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for update", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user for update", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	// Apply updates (only updatable fields)
	if cmd.FirstName != nil {
		current.FirstName = *cmd.FirstName
	}
	if cmd.LastName != nil {
		current.LastName = *cmd.LastName
	}
	if cmd.Role != nil {
		current.Role = *cmd.Role
	}
	if cmd.Image != nil {
		current.Image = cmd.Image
	}

	updated, err := s.repo.Update(ctx, current)
	if err != nil {
		s.logger.Error("failed to update user", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Info("user updated", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
	return updated, nil
}

// DeleteUser soft-deletes a user
func (s *Service) DeleteUser(ctx context.Context, tenantID, userID string) error {
	s.logger.Debug("deleting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	err := s.repo.Delete(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return err
		}
		s.logger.Error("failed to delete user", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return err
	}

	s.logger.Info("user soft-deleted", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
	return nil
}

// ListPendingUsers returns users with a pending role assignment in the tenant.
func (s *Service) ListPendingUsers(ctx context.Context, tenantID string) ([]*domainUsers.User, error) {
	s.logger.Debug("listing pending users", zap.String("tenant_id", tenantID))

	users, err := s.repo.ListPendingByTenant(ctx, tenantID)
	if err != nil {
		s.logger.Error("failed to list pending users", zap.String("tenant_id", tenantID), zap.Error(err))
		return nil, err
	}

	s.logger.Debug("pending users listed", zap.String("tenant_id", tenantID), zap.Int("count", len(users)))
	return users, nil
}

// UpdateUserStatus changes the UTR status for a user in the tenant.
// callerID is the ID of the authenticated admin making the request.
// Allowed status values: "active", "inactive", "suspended".
func (s *Service) UpdateUserStatus(ctx context.Context, tenantID, userID, callerID, status string) (*domainUsers.User, error) {
	// Guard: admin cannot deactivate themselves
	if userID == callerID {
		return nil, domainUsers.ErrCannotDeactivateSelf
	}

	// Validate allowed status values
	var utrStatus domain.UserRoleStatus
	switch status {
	case "active":
		utrStatus = domain.UserRoleStatusActive
	case "inactive":
		utrStatus = domain.UserRoleStatusRevoked
	case "suspended":
		utrStatus = domain.UserRoleStatusSuspended
	default:
		return nil, domainUsers.ErrInvalidStatus
	}

	// Verify user belongs to this tenant (existence check only)
	if _, err := s.repo.GetByID(ctx, tenantID, userID); err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for status update", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user for status update", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	_, err = s.userRoleRepo.UpdateStatus(ctx, userUUID, tenantUUID, utrStatus)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveAssignment) {
			s.logger.Warn("no active assignment found for status update",
				zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to update user status",
			zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Info("user status updated",
		zap.String("tenant_id", tenantID),
		zap.String("user_id", userID),
		zap.String("status", status))

	// Re-fetch to return the latest state (updatedAt reflects the mutation)
	updated, err := s.repo.GetByID(ctx, tenantID, userID)
	if err != nil {
		s.logger.Error("failed to re-fetch user after status update",
			zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}
	return updated, nil
}
