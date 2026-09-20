package roles_test

import (
	"context"
	"errors"
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

	listResult []*domain.Role
	listErr    error

	countCustomResult int
	countCustomErr     error

	createErr   error
	createCalls []*domain.Role

	updateErr error

	softDeleteErr   error
	softDeleteCalls []string

	countActiveResult int
	countActiveErr    error
}

func (f *fakeRolesRepo) List(ctx context.Context, tenantID uuid.UUID, includeGlobal bool) ([]*domain.Role, error) {
	return f.listResult, f.listErr
}

func (f *fakeRolesRepo) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error) {
	if f.hidden {
		return nil, domain.ErrRoleNotFound
	}
	return f.role, nil
}

func (f *fakeRolesRepo) CountCustomByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	return f.countCustomResult, f.countCustomErr
}

func (f *fakeRolesRepo) Create(ctx context.Context, role *domain.Role) error {
	f.createCalls = append(f.createCalls, role)
	return f.createErr
}

func (f *fakeRolesRepo) Update(ctx context.Context, role *domain.Role) error { return f.updateErr }

func (f *fakeRolesRepo) SoftDelete(ctx context.Context, id string) error {
	f.softDeleteCalls = append(f.softDeleteCalls, id)
	return f.softDeleteErr
}

func (f *fakeRolesRepo) CountActiveAssignments(ctx context.Context, roleID string) (int, error) {
	return f.countActiveResult, f.countActiveErr
}

func TestListRoles(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.Role{{ID: "admin"}}
		repo := &fakeRolesRepo{listResult: want}
		svc := appRoles.NewService(repo, zap.NewNop())
		got, err := svc.ListRoles(context.Background(), uuid.New(), false)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRolesRepo{listErr: errors.New("db down")}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.ListRoles(context.Background(), uuid.New(), false)
		require.Error(t, err)
	})
}

func TestGetRole(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRolesRepo{role: &domain.Role{ID: "admin"}}
		svc := appRoles.NewService(repo, zap.NewNop())
		got, err := svc.GetRole(context.Background(), "admin", uuid.New(), false)
		require.NoError(t, err)
		require.Equal(t, "admin", got.ID)
	})
	t.Run("no encontrado (oculto)", func(t *testing.T) {
		repo := &fakeRolesRepo{hidden: true}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.GetRole(context.Background(), "super_admin", uuid.New(), false)
		require.ErrorIs(t, err, domain.ErrRoleNotFound)
	})
}

func TestCreateRole(t *testing.T) {
	tenantID := uuid.New()

	t.Run("limite de roles custom alcanzado", func(t *testing.T) {
		repo := &fakeRolesRepo{countCustomResult: domain.MaxCustomRolesPerTenant}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.CreateRole(context.Background(), tenantID, "Nuevo", "desc", nil)
		require.ErrorIs(t, err, domain.ErrRoleLimitReached)
		require.Empty(t, repo.createCalls)
	})
	t.Run("error contando roles custom", func(t *testing.T) {
		repo := &fakeRolesRepo{countCustomErr: errors.New("db down")}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.CreateRole(context.Background(), tenantID, "Nuevo", "desc", nil)
		require.Error(t, err)
	})
	t.Run("dedup y sort de permisos", func(t *testing.T) {
		repo := &fakeRolesRepo{}
		svc := appRoles.NewService(repo, zap.NewNop())
		got, err := svc.CreateRole(context.Background(), tenantID, "Nuevo", "desc",
			[]string{"perm_b", " perm_a ", "perm_a", "", "perm_b"})
		require.NoError(t, err)
		require.Equal(t, []string{"perm_a", "perm_b"}, got.Permissions)
		require.Equal(t, tenantID, *got.TenantID)
	})
	t.Run("nombre duplicado propaga sin loguear como error", func(t *testing.T) {
		repo := &fakeRolesRepo{createErr: domain.ErrRoleDuplicateName}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.CreateRole(context.Background(), tenantID, "Dup", "desc", nil)
		require.ErrorIs(t, err, domain.ErrRoleDuplicateName)
	})
}

func TestCountActiveAssignments(t *testing.T) {
	repo := &fakeRolesRepo{countActiveResult: 3}
	svc := appRoles.NewService(repo, zap.NewNop())
	got, err := svc.CountActiveAssignments(context.Background(), "admin")
	require.NoError(t, err)
	require.Equal(t, 3, got)
}

func TestUpdateRoleHappyPathAplicaDedupDePermisos(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc", IsSystemRole: false}}
	svc := appRoles.NewService(repo, zap.NewNop())
	got, err := svc.UpdateRole(context.Background(), "custom_abc", uuid.New(), false, "Nuevo nombre", "desc", []string{"p2", "p1", "p1"})
	require.NoError(t, err)
	require.Equal(t, "Nuevo nombre", got.Name)
	require.Equal(t, []string{"p1", "p2"}, got.Permissions)
}

func TestUpdateRoleNombreDuplicado(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc"}, updateErr: domain.ErrRoleDuplicateName}
	svc := appRoles.NewService(repo, zap.NewNop())
	_, err := svc.UpdateRole(context.Background(), "custom_abc", uuid.New(), false, "x", "y", nil)
	require.ErrorIs(t, err, domain.ErrRoleDuplicateName)
}

func TestDeleteRoleHappyPath(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc", IsSystemRole: false}}
	svc := appRoles.NewService(repo, zap.NewNop())
	err := svc.DeleteRole(context.Background(), "custom_abc", uuid.New(), false)
	require.NoError(t, err)
	require.Equal(t, []string{"custom_abc"}, repo.softDeleteCalls)
}

func TestDeleteRoleConAsignacionesActivasFalla(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc", IsSystemRole: false}, countActiveResult: 2}
	svc := appRoles.NewService(repo, zap.NewNop())
	err := svc.DeleteRole(context.Background(), "custom_abc", uuid.New(), false)
	require.ErrorIs(t, err, domain.ErrRoleHasAssignments)
	require.Empty(t, repo.softDeleteCalls, "no debe borrar si hay asignaciones activas")
}

func TestDeleteRoleErrorContandoAsignaciones(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc"}, countActiveErr: errors.New("db down")}
	svc := appRoles.NewService(repo, zap.NewNop())
	err := svc.DeleteRole(context.Background(), "custom_abc", uuid.New(), false)
	require.Error(t, err)
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
