package roles_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// fakeRolesRepo es un doble de prueba mínimo de rolesRepo.Repository. Su único
// propósito es fijar, a nivel de Service, el orden de checks que la revisión
// de la Tarea 4 encontró roto: UpdateRole/DeleteRole tienen que resolver
// visibilidad (GetByIDForTenant) antes de evaluar IsSystemRole. El campo
// hidden simula lo que la consulta SQL real de GetByIDForTenant hace cuando
// el rol no es visible para el caller (tenant equivocado o includeGlobal
// insuficiente): devuelve ErrRoleNotFound sin que el service llegue a ver
// is_system_role en absoluto.
type fakeRolesRepo struct {
	role   *domain.Role
	hidden bool
}

func (f *fakeRolesRepo) List(ctx context.Context, tenantID uuid.UUID, includeGlobal bool) ([]*domain.Role, error) {
	return nil, nil
}

func (f *fakeRolesRepo) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error) {
	if f.hidden {
		return nil, domain.ErrRoleNotFound
	}
	return f.role, nil
}

func (f *fakeRolesRepo) CountCustomByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	return 0, nil
}

func (f *fakeRolesRepo) Create(ctx context.Context, role *domain.Role) error { return nil }

func (f *fakeRolesRepo) Update(ctx context.Context, role *domain.Role) error { return nil }

func (f *fakeRolesRepo) SoftDelete(ctx context.Context, id string) error { return nil }

func (f *fakeRolesRepo) CountActiveAssignments(ctx context.Context, roleID string) (int, error) {
	return 0, nil
}

// TestUpdateRoleRolGlobalOcultoDevuelveNotFoundNoSystemRole es el regression
// test que pidió la revisión: un caller no-superadmin que intenta modificar
// un rol global (invisible para él) tiene que recibir ErrRoleNotFound, nunca
// ErrRoleIsSystemRole — ese último confirmaría que el rol existe.
func TestUpdateRoleRolGlobalOcultoDevuelveNotFoundNoSystemRole(t *testing.T) {
	repo := &fakeRolesRepo{hidden: true}
	svc := appRoles.NewService(repo, zap.NewNop())

	_, err := svc.UpdateRole(context.Background(), "super_admin", uuid.New(), false, "x", "y", nil)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)
	require.NotErrorIs(t, err, domain.ErrRoleIsSystemRole)
}

// TestDeleteRoleRolGlobalOcultoDevuelveNotFoundNoSystemRole es el mismo
// regression test para DeleteRole.
func TestDeleteRoleRolGlobalOcultoDevuelveNotFoundNoSystemRole(t *testing.T) {
	repo := &fakeRolesRepo{hidden: true}
	svc := appRoles.NewService(repo, zap.NewNop())

	err := svc.DeleteRole(context.Background(), "super_admin", uuid.New(), false)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)
	require.NotErrorIs(t, err, domain.ErrRoleIsSystemRole)
}

// TestUpdateRoleRolDeSistemaVisibleDevuelveSystemRole es el complemento: para
// un rol que SÍ es visible para el caller (por ejemplo "admin", un archetype
// tenant-scoped que cualquier tenant puede ver) pero es de sistema, la
// protección de IsSystemRole tiene que seguir funcionando — el fix de
// visibilidad no puede convertir esos 403 legítimos en 404.
func TestUpdateRoleRolDeSistemaVisibleDevuelveSystemRole(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "admin", IsSystemRole: true}}
	svc := appRoles.NewService(repo, zap.NewNop())

	_, err := svc.UpdateRole(context.Background(), "admin", uuid.New(), false, "x", "y", nil)
	require.ErrorIs(t, err, domain.ErrRoleIsSystemRole)
}

// TestDeleteRoleRolDeSistemaVisibleDevuelveSystemRole es el mismo complemento
// para DeleteRole.
func TestDeleteRoleRolDeSistemaVisibleDevuelveSystemRole(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "admin", IsSystemRole: true}}
	svc := appRoles.NewService(repo, zap.NewNop())

	err := svc.DeleteRole(context.Background(), "admin", uuid.New(), false)
	require.ErrorIs(t, err, domain.ErrRoleIsSystemRole)
}
