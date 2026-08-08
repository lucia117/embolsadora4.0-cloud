package revoke_user_role

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	userrolesrepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
)

// UseCase defines the interface for revoking a user-role assignment.
type UseCase interface {
	Execute(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, includeGlobal bool) (*domain.UserTenantRole, error)
}

type useCase struct {
	repo userrolesrepo.UserRoleRepository
}

// NewUseCase creates a new revoke_user_role use case.
func NewUseCase(repo userrolesrepo.UserRoleRepository) UseCase {
	return &useCase{repo: repo}
}

// Execute soft-deletes a UTR assignment by setting its status to 'revoked'.
// Returns ErrAssignmentNotFound if the assignment does not exist, belongs to a different
// tenant, o apunta a un rol global y el caller no es super_admin (misma respuesta en los
// tres casos, así que nadie puede usar esta ruta para sondear ids ajenos ni para
// confirmar la membresía del superadmin).
//
// includeGlobal viaja al precheck FindByID y también al UPDATE de Revoke: el precheck
// solo lee, y la lección de DeleteUser (Task 5) es que la mutación tiene que negarse
// sola, sin depender de que alguien la llame en el orden correcto.
func (uc *useCase) Execute(ctx context.Context, id uuid.UUID, tenantID uuid.UUID, includeGlobal bool) (*domain.UserTenantRole, error) {
	existing, err := uc.repo.FindByID(ctx, id, includeGlobal)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.TenantID != tenantID {
		return nil, domain.ErrAssignmentNotFound
	}

	result, err := uc.repo.Revoke(ctx, id, tenantID, includeGlobal)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, domain.ErrAssignmentNotFound
	}
	return result, nil
}
