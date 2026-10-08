package roles_test

// Issue #107: quien crea o edita un rol custom no puede darle permisos de
// sistema que él mismo no tiene (salvo super_admin). Los permisos custom del
// tenant quedan fuera de la regla: ningún código los usa para autorizar.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// systemCatalog trata como permisos de sistema a los ids de system; el resto
// son custom del tenant. No reporta desconocidos.
type systemCatalog struct{ system map[string]bool }

func (systemCatalog) UnknownPermissionIDs(context.Context, uuid.UUID, []string) ([]string, error) {
	return nil, nil
}

func (c systemCatalog) SystemPermissionIDs(_ context.Context, ids []string) ([]string, error) {
	var out []string
	for _, id := range ids {
		if c.system[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

var catalogo = systemCatalog{system: map[string]bool{
	"perm_users_view": true, "perm_users_manage": true, "perm_tenants_manage": true, "perm_dashboard": true,
}}

func callerCtx(role string, perms ...string) context.Context {
	return security.WithRoleContext(context.Background(), security.RoleContext{Name: role, Permissions: perms})
}

func TestCreateRole_NoOtorgaPermisosDeSistemaQueElCreadorNoTiene(t *testing.T) {
	repo := &fakeRolesRepo{}
	svc := appRoles.NewService(repo, catalogo, zap.NewNop())
	ctx := callerCtx("cliente_admin", "perm_users_view", "perm_users_manage")

	_, err := svc.CreateRole(ctx, uuid.New(), "escalada", "", []string{"perm_users_manage", "perm_tenants_manage"})

	require.ErrorIs(t, err, domain.ErrRolePermissionNotHeld)
	require.Contains(t, err.Error(), "perm_tenants_manage")
	require.NotContains(t, err.Error(), "perm_users_manage")
	require.Empty(t, repo.createCalls)
}

func TestCreateRole_PermisosQueElCreadorTieneSePermiten(t *testing.T) {
	repo := &fakeRolesRepo{}
	svc := appRoles.NewService(repo, catalogo, zap.NewNop())
	ctx := callerCtx("cliente_admin", "perm_users_view", "perm_users_manage")

	_, err := svc.CreateRole(ctx, uuid.New(), "lector", "", []string{"perm_users_view"})

	require.NoError(t, err)
	require.Len(t, repo.createCalls, 1)
}

func TestCreateRole_PermisosCustomDelTenantNoRequierenTenerlos(t *testing.T) {
	repo := &fakeRolesRepo{}
	svc := appRoles.NewService(repo, catalogo, zap.NewNop())
	ctx := callerCtx("cliente_admin", "perm_users_manage")

	_, err := svc.CreateRole(ctx, uuid.New(), "con custom", "", []string{"3f1c2b9e-custom-del-tenant"})

	require.NoError(t, err)
}

func TestCreateRole_SuperAdminPuedeOtorgarCualquierPermiso(t *testing.T) {
	repo := &fakeRolesRepo{}
	svc := appRoles.NewService(repo, catalogo, zap.NewNop())
	ctx := callerCtx("super_admin") // aunque su lista no traiga el permiso

	_, err := svc.CreateRole(ctx, uuid.New(), "plataforma", "", []string{"perm_tenants_manage"})

	require.NoError(t, err)
}

func TestCreateRole_SinRolEnContextoNoOtorgaPermisosDeSistema(t *testing.T) {
	repo := &fakeRolesRepo{}
	svc := appRoles.NewService(repo, catalogo, zap.NewNop())

	_, err := svc.CreateRole(context.Background(), uuid.New(), "x", "", []string{"perm_dashboard"})

	require.ErrorIs(t, err, domain.ErrRolePermissionNotHeld, "fail-closed: sin rol no se tiene ningún permiso")
}

// Editar un rol que ya tenía un permiso que el editor no tiene (por ejemplo,
// creado por super_admin) no falla por ese permiso: solo se controlan los
// que se agregan.
func TestUpdateRole_SoloControlaLosPermisosQueSeAgregan(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc123", Name: "Soporte",
		Permissions: []string{"perm_tenants_manage"}}}
	svc := appRoles.NewService(repo, catalogo, zap.NewNop())
	ctx := callerCtx("cliente_admin", "perm_users_view", "perm_users_manage")

	_, err := svc.UpdateRole(ctx, "custom_abc123", uuid.New(), false, "Soporte N1", "",
		[]string{"perm_tenants_manage", "perm_users_view"})
	require.NoError(t, err, "conservar perm_tenants_manage y agregar uno propio se permite")

	repo.role.Permissions = []string{"perm_users_view"}
	_, err = svc.UpdateRole(ctx, "custom_abc123", uuid.New(), false, "Soporte N1", "",
		[]string{"perm_users_view", "perm_tenants_manage"})
	require.ErrorIs(t, err, domain.ErrRolePermissionNotHeld, "agregar uno que no tiene se rechaza")
}
