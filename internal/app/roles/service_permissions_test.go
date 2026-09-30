package roles_test

// Validación de los permisos de roles custom contra el catálogo (issue #93).
// Desde el PR #62 la autorización lee roles.permissions de la base, así que un
// id que no existe en el catálogo (typo, permiso de otro tenant, permiso
// borrado) no puede guardarse en silencio.

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

// allPermissionsKnown es el catálogo de los tests que no ejercitan la
// validación: acepta cualquier id.
type allPermissionsKnown struct{}

func (allPermissionsKnown) UnknownPermissionIDs(context.Context, uuid.UUID, []string) ([]string, error) {
	return nil, nil
}

// fakeCatalog marca como desconocidos los ids de unknown y registra con qué
// ids lo consultaron.
type fakeCatalog struct {
	unknown map[string]bool
	err     error
	calls   [][]string
}

func (f *fakeCatalog) UnknownPermissionIDs(_ context.Context, _ uuid.UUID, ids []string) ([]string, error) {
	f.calls = append(f.calls, ids)
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, id := range ids {
		if f.unknown[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

func TestCreateRole_PermisoDesconocidoRechazaSinCrear(t *testing.T) {
	repo := &fakeRolesRepo{}
	catalog := &fakeCatalog{unknown: map[string]bool{"perm_dashbaord": true}}
	svc := appRoles.NewService(repo, catalog, zap.NewNop())

	_, err := svc.CreateRole(context.Background(), uuid.New(), "Supervisor", "",
		[]string{"perm_alerts", "perm_dashbaord"})

	require.ErrorIs(t, err, domain.ErrRoleUnknownPermissions)
	require.Contains(t, err.Error(), "perm_dashbaord")
	require.NotContains(t, err.Error(), "perm_alerts", "solo se informan los ids desconocidos")
	require.Empty(t, repo.createCalls, "no se crea el rol si algún permiso no existe")
}

func TestCreateRole_PermisosValidosConsultaElCatalogoDeduplicado(t *testing.T) {
	repo := &fakeRolesRepo{}
	catalog := &fakeCatalog{}
	svc := appRoles.NewService(repo, catalog, zap.NewNop())

	role, err := svc.CreateRole(context.Background(), uuid.New(), "Supervisor", "",
		[]string{"perm_alerts", "perm_dashboard", "perm_alerts"})

	require.NoError(t, err)
	require.Equal(t, []string{"perm_alerts", "perm_dashboard"}, role.Permissions)
	require.Equal(t, [][]string{{"perm_alerts", "perm_dashboard"}}, catalog.calls)
	require.Len(t, repo.createCalls, 1)
}

func TestCreateRole_SinPermisosNoConsultaElCatalogo(t *testing.T) {
	repo := &fakeRolesRepo{}
	catalog := &fakeCatalog{}
	svc := appRoles.NewService(repo, catalog, zap.NewNop())

	_, err := svc.CreateRole(context.Background(), uuid.New(), "Vacío", "", nil)

	require.NoError(t, err)
	require.Empty(t, catalog.calls)
	require.Len(t, repo.createCalls, 1)
}

func TestCreateRole_ErrorDelCatalogoSePropaga(t *testing.T) {
	repo := &fakeRolesRepo{}
	catalog := &fakeCatalog{err: errors.New("db down")}
	svc := appRoles.NewService(repo, catalog, zap.NewNop())

	_, err := svc.CreateRole(context.Background(), uuid.New(), "Supervisor", "", []string{"perm_alerts"})

	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrRoleUnknownPermissions, "un fallo de infraestructura no es un permiso inválido")
	require.Empty(t, repo.createCalls)
}

func TestUpdateRole_PermisoDesconocidoRechaza(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc123", Name: "Supervisor"}}
	catalog := &fakeCatalog{unknown: map[string]bool{"perm_inexistente": true}}
	svc := appRoles.NewService(repo, catalog, zap.NewNop())

	_, err := svc.UpdateRole(context.Background(), "custom_abc123", uuid.New(), false,
		"Supervisor", "", []string{"perm_inexistente"})

	require.ErrorIs(t, err, domain.ErrRoleUnknownPermissions)
	require.Contains(t, err.Error(), "perm_inexistente")
}
