package update_user_role

import (
	"context"

	"github.com/google/uuid"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for updating the role on an existing assignment.
type UseCase interface {
	Execute(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, roleID string, includeGlobal bool) (*domain.UserTenantRole, error)
}

type useCase struct {
	repo     userrolesrepo.UserRoleRepository
	roleRepo rolesRepo.Repository
}

// NewUseCase creates a new update_user_role use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository, roleRepo rolesRepo.Repository) UseCase {
	return &useCase{repo: repo, roleRepo: roleRepo}
}

// Execute updates the roleId on an existing UTR assignment.
// Returns ErrAssignmentNotFound if the assignment does not exist, belongs to a different
// tenant, o es invisible para este caller por apuntar a un rol global (misma respuesta
// en los tres casos, para que ninguno se distinga de los otros).
//
// Dos ejes distintos, los dos necesarios:
//   - includeGlobal en FindByID: la asignación que se está por modificar tiene que ser
//     visible. Sin esto, PUT /user-roles/{id de la UTR del superadmin} cambiaba el rol
//     de una membresía que la lista ya no muestra.
//   - includeGlobal en EnsureAssignable: el rol NUEVO tiene que ser asignable. Sin
//     esto, PUT sobre una asignación cualquiera con roleId="super_admin" era la misma
//     escalada de privilegios que POST /user-roles.
func (uc *useCase) Execute(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, roleID string, includeGlobal bool) (*domain.UserTenantRole, error) {
	utr, err := uc.repo.FindByID(ctx, id, includeGlobal)
	if err != nil {
		return nil, err
	}
	if utr == nil || utr.TenantID != tenantID {
		return nil, domain.ErrAssignmentNotFound
	}

	if err := appRoles.EnsureAssignable(ctx, uc.roleRepo, roleID, tenantID, includeGlobal); err != nil {
		return nil, err
	}

	utr.RoleID = &roleID
	return uc.repo.Update(ctx, utr, includeGlobal)
}
