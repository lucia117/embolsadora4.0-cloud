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
// crossTenant es un eje separado, decidido vía security.IsCrossTenantRole: deja
// resolver un usuario de un tenant distinto al de la request, pero no afecta el
// cloaking de includeGlobal — ver el comentario de Repository.GetByID.
func (s *Service) GetUser(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.User, error) {
	s.logger.Debug("getting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	user, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
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
//
// NO lleva el guard de identidad que sí llevan Create/BulkCreate/Update en el repo de
// user_tenant_roles (CreateQuery, "identidad de plataforma"). Es deliberado: ahí el
// caller elige a QUIÉN le escribe la membresía —el userId viaja en el body— y por eso
// podía apuntar al super_admin. Acá el destino no existe todavía: CreateUserCommand no
// tiene campo de id, y CreateWithRole inserta un users nuevo con uuid fresco en la misma
// transacción, así que el sujeto de la UTR nunca puede ser una cuenta ya existente. El
// guard sería código muerto. Tampoco hay oráculo por colisión de email: el único índice
// es UNIQUE (tenant_id, email), o sea que reusar el email de una cuenta de plataforma
// desde otro tenant crea una fila distinta y devuelve 201, no ErrEmailTaken.
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
// crossTenant lo decide el handler vía security.IsCrossTenantRole (extensión de
// Hallazgo C acordada): sin esto, un super_admin/platform_admin parado en un
// tenant distinto al del target no podía editarlo. El UPDATE en sí no necesita
// su propio crossTenant: repo.Update ya escribe contra current.TenantID (el
// tenant real del target, resuelto acá abajo), no contra el tenantID de la
// request.
func (s *Service) UpdateUser(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool, cmd *domainUsers.UpdateUserCommand) (*domainUsers.User, error) {
	if err := cmd.Validate(); err != nil {
		s.logger.Warn("invalid update user command", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domainUsers.ErrValidation, err)
	}

	s.logger.Debug("updating user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	// Get current user
	current, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
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
		//
		// Se valida contra current.TenantID (el tenant REAL del target), no contra
		// tenantID (el de la request): bajo crossTenant=true esos dos pueden ser
		// distintos, y tenant_can_use_role() debe evaluarse para el tenant donde
		// el rol realmente se va a aplicar.
		targetTenantUUID, err := uuid.Parse(current.TenantID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid tenant_id: %v", domainUsers.ErrValidation, err)
		}
		if err := appRoles.EnsureAssignable(ctx, s.roleRepo, *cmd.Role, targetTenantUUID, includeGlobal); err != nil {
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
// crossTenant lo decide el handler vía security.IsCrossTenantRole (Hallazgo C):
// sin esto, el precheck de abajo solo podía resolver un usuario del tenant de
// la request, así que un super_admin/platform_admin parado en un tenant
// distinto al del target recibía 404 en vez de poder borrarlo.
//
// Resuelve el usuario con GetByID (mismo scoping que GetUser/UpdateUser) antes de
// tocar el repo.Delete: repo.Delete no filtra por rol, así que sin este precheck un
// caller no-superadmin podría soft-borrar a un usuario con rol global aunque no
// pudiera verlo — un efecto observable (el usuario oculto deja de poder operar)
// que delata su existencia igual que un 403. Con el precheck, un usuario oculto
// da 404 y el DELETE nunca se ejecuta.
//
// El DELETE en sí se ejecuta contra current.TenantID (el tenant REAL del target,
// resuelto por el precheck), no contra el tenantID de la request: así
// repo.Delete no necesita su propio parámetro crossTenant/escape hatch — ya
// recibe el tenant correcto para el usuario que se está borrando.
func (s *Service) DeleteUser(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) error {
	s.logger.Debug("deleting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	current, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return err
		}
		s.logger.Error("failed to get user for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return err
	}

	if err := s.repo.Delete(ctx, current.TenantID, userID); err != nil {
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
// crossTenant lo decide el handler vía security.IsCrossTenantRole (extensión de
// Hallazgo C acordada): sin esto, el precheck solo resolvía un usuario del
// tenant de la request, y la mutación de abajo (userRoleRepo.UpdateStatus)
// usaba ese mismo tenantID de la request en vez del tenant real del target —
// un super_admin parado en tenantA no podía suspender a un usuario de
// tenantB. Con el precheck resolviendo current.TenantID (el tenant real), la
// mutación usa ese tenant resuelto, no el de la request.
func (s *Service) UpdateUserStatus(ctx context.Context, tenantID, userID, callerID, status string, crossTenant, includeGlobal bool) (*domainUsers.User, error) {
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

	// Resolve the user (existence check + su tenant real, ver comentario arriba)
	current, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for status update", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user for status update", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	targetTenantUUID, err := uuid.Parse(current.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	_, err = s.userRoleRepo.UpdateStatus(ctx, userUUID, targetTenantUUID, utrStatus, includeGlobal)
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

	// Return current (ya resuelto arriba) en vez de re-fetchear: domainUsers.User
	// no tiene un campo Status (el estado vive únicamente en user_tenant_roles),
	// así que un GetByID posterior no aportaría nada nuevo — y de hecho es
	// activamente incorrecto acá. GetByID resuelve tenant_id/role vía un JOIN
	// LATERAL que exige t.status = 'active' (ver postgres.go); tras suspender o
	// revocar la UTR que acabamos de mutar, ese JOIN ya no la encuentra, y para
	// un usuario sin users.tenant_id legado (el caso normal, ver comentario en
	// postgres.go:32) GetByID devolvía ErrNotFound pese a que la mutación fue
	// exitosa — un bug preexistente e independiente de este fix, no capturado
	// hasta ahora porque no había un test de integración que ejercitara el
	// round-trip completo de Service.UpdateUserStatus. Ninguno de los otros
	// campos de current (nombre, email, rol, tenant) cambia con un status
	// update, así que devolverlo es equivalente y evita el re-fetch roto.
	return current, nil
}
