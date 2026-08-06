package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	domain "github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	"go.uber.org/zap"
)

// Service handles user-related business logic
type Service struct {
	repo         usersRepo.Repository
	userRoleRepo userRolesRepo.UserRoleRepository
	roleRepo     rolesRepo.Repository
	logger       *zap.Logger
}

// NewService creates a new user service
func NewService(repo usersRepo.Repository, userRoleRepo userRolesRepo.UserRoleRepository, roleRepo rolesRepo.Repository, logger *zap.Logger) *Service {
	return &Service{
		repo:         repo,
		userRoleRepo: userRoleRepo,
		roleRepo:     roleRepo,
		logger:       logger,
	}
}

// ListUsers retrieves paginated users for a tenant.
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
func (s *Service) ListUsers(ctx context.Context, tenantID string, limit, offset int, includeGlobal bool) ([]*domainUsers.User, int64, error) {
	s.logger.Debug("listing users", zap.String("tenant_id", tenantID), zap.Int("limit", limit), zap.Int("offset", offset))

	users, total, err := s.repo.ListByTenant(ctx, tenantID, limit, offset, includeGlobal)
	if err != nil {
		s.logger.Error("failed to list users", zap.String("tenant_id", tenantID), zap.Error(err))
		return nil, 0, err
	}

	s.logger.Debug("users listed", zap.String("tenant_id", tenantID), zap.Int64("total", total), zap.Int("count", len(users)))
	return users, total, nil
}

// GetUser retrieves a single user by ID.
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
func (s *Service) GetUser(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.User, error) {
	s.logger.Debug("getting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	user, err := s.repo.GetByID(ctx, tenantID, userID, includeGlobal)
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
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
func (s *Service) GetUserWithRoles(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	s.logger.Debug("getting user with roles", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	uwr, err := s.repo.GetByIDWithRoles(ctx, tenantID, userID, includeGlobal)
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

// CreateUser creates a new user in a tenant with an active role assignment.
//
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals y acá sirve
// para una sola cosa, la más importante de este archivo: validar que el rol pedido sea
// asignable por este caller ANTES de crear nada. Sin esa validación, POST /users
// {"role":"super_admin"} desde el tenant plataforma creaba un usuario con UTR activa de
// superadmin — la FK de user_tenant_roles.role_id no lo impide (el rol existe) y
// tenant_can_use_role() tampoco (dentro de MRG devuelve TRUE para is_global). Con
// force-password-change a continuación, eso es tomar la cuenta. Ver EnsureAssignable.
func (s *Service) CreateUser(ctx context.Context, tenantID string, cmd *domainUsers.CreateUserCommand, includeGlobal bool) (*domainUsers.User, error) {
	if err := cmd.Validate(); err != nil {
		s.logger.Warn("invalid create user command", zap.String("tenant_id", tenantID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domainUsers.ErrValidation, err)
	}

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid tenant_id: %v", domainUsers.ErrValidation, err)
	}
	assignedByUUID, err := uuid.Parse(cmd.AssignedBy)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid assigned_by: %v", domainUsers.ErrValidation, err)
	}

	if err := appRoles.EnsureAssignable(ctx, s.roleRepo, cmd.Role, tenantUUID, includeGlobal); err != nil {
		s.logger.Warn("rol no asignable en create user",
			zap.String("tenant_id", tenantID), zap.String("role", cmd.Role), zap.Error(err))
		return nil, err
	}

	user := &domainUsers.User{
		TenantID:  tenantID,
		FirstName: cmd.FirstName,
		LastName:  cmd.LastName,
		Email:     cmd.Email,
		Role:      cmd.Role,
		Image:     cmd.Image,
	}

	now := time.Now().UTC()
	utr := &domain.UserTenantRole{
		ID:         uuid.New(),
		TenantID:   tenantUUID,
		RoleID:     &cmd.Role,
		Status:     domain.UserRoleStatusActive,
		AssignedBy: &assignedByUUID,
		AssignedAt: &now,
	}

	s.logger.Debug("creating user with role", zap.String("tenant_id", tenantID), zap.String("email", cmd.Email), zap.String("role", cmd.Role))

	created, err := s.repo.CreateWithRole(ctx, user, utr)
	if err != nil {
		switch {
		case errors.Is(err, domainUsers.ErrEmailTaken):
			s.logger.Warn("email already taken", zap.String("tenant_id", tenantID), zap.String("email", cmd.Email))
		case errors.Is(err, domain.ErrInvalidRoleID):
			s.logger.Warn("invalid role id on user creation", zap.String("tenant_id", tenantID), zap.String("role", cmd.Role))
		default:
			s.logger.Error("failed to create user", zap.String("tenant_id", tenantID), zap.String("email", cmd.Email), zap.Error(err))
		}
		return nil, err
	}

	s.logger.Info("user created with role",
		zap.String("tenant_id", tenantID),
		zap.String("user_id", created.ID),
		zap.String("email", cmd.Email),
		zap.String("role", cmd.Role))
	return created, nil
}

// UpdateUser updates user fields (name, role, image).
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals: resuelve
// el usuario actual con el mismo scoping que GetUser, para que un usuario oculto dé
// 404 antes de llegar al UPDATE — no 200/403, que confirmarían su existencia.
func (s *Service) UpdateUser(ctx context.Context, tenantID, userID string, includeGlobal bool, cmd *domainUsers.UpdateUserCommand) (*domainUsers.User, error) {
	if err := cmd.Validate(); err != nil {
		s.logger.Warn("invalid update user command", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domainUsers.ErrValidation, err)
	}

	s.logger.Debug("updating user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	// Get current user
	current, err := s.repo.GetByID(ctx, tenantID, userID, includeGlobal)
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
		// users.role es la columna legada (la membresía real vive en
		// user_tenant_roles, que es lo que lee /me), pero los listados la muestran
		// vía COALESCE(utr.role_id, u.role) cuando no hay UTR activa. Escribir
		// 'super_admin' acá no da permisos, y aun así se valida con el mismo lookup
		// cloakeado: dejarla libre permitiría pintar a un usuario como superadmin y,
		// peor, sacarlo del filtro de cloaking de los listados de users.
		tenantUUID, err := uuid.Parse(tenantID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid tenant_id: %v", domainUsers.ErrValidation, err)
		}
		if err := appRoles.EnsureAssignable(ctx, s.roleRepo, *cmd.Role, tenantUUID, includeGlobal); err != nil {
			s.logger.Warn("rol no asignable en update user",
				zap.String("tenant_id", tenantID), zap.String("role", *cmd.Role), zap.Error(err))
			return nil, err
		}
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

// DeleteUser soft-deletes a user.
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
//
// Resuelve el usuario con GetByID (mismo scoping que GetUser/UpdateUser) antes de
// tocar el repo.Delete: repo.Delete no filtra por rol, así que sin este precheck un
// caller no-superadmin podría soft-borrar a un usuario con rol global aunque no
// pudiera verlo — un efecto observable (el usuario oculto deja de poder operar)
// que delata su existencia igual que un 403. Con el precheck, un usuario oculto
// da 404 y el DELETE nunca se ejecuta.
func (s *Service) DeleteUser(ctx context.Context, tenantID, userID string, includeGlobal bool) error {
	s.logger.Debug("deleting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	if _, err := s.repo.GetByID(ctx, tenantID, userID, includeGlobal); err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return err
		}
		s.logger.Error("failed to get user for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return err
	}

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
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
func (s *Service) ListPendingUsers(ctx context.Context, tenantID string, includeGlobal bool) ([]*domainUsers.User, error) {
	s.logger.Debug("listing pending users", zap.String("tenant_id", tenantID))

	users, err := s.repo.ListPendingByTenant(ctx, tenantID, includeGlobal)
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
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals: así un
// platform_admin tampoco puede cambiarle el estado a un superadmin invisible, y
// recibe el mismo 404 coherente con GetUser/ListUsers.
func (s *Service) UpdateUserStatus(ctx context.Context, tenantID, userID, callerID, status string, includeGlobal bool) (*domainUsers.User, error) {
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
	if _, err := s.repo.GetByID(ctx, tenantID, userID, includeGlobal); err != nil {
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

	_, err = s.userRoleRepo.UpdateStatus(ctx, userUUID, tenantUUID, utrStatus, includeGlobal)
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
	updated, err := s.repo.GetByID(ctx, tenantID, userID, includeGlobal)
	if err != nil {
		s.logger.Error("failed to re-fetch user after status update",
			zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}
	return updated, nil
}
