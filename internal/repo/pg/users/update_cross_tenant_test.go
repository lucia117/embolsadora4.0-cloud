package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestUpdateUserCrossTenant cubre la extensión de Hallazgo C acordada con el
// usuario: mismo root cause que DeleteUser, mismo fix.
func TestUpdateUserCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	newName := "No Debe Aplicar"
	_, err := svc.UpdateUser(ctx, tenantA, userInB, false, false, &domainUsers.UpdateUserCommand{
		TenantID: tenantA, UserID: userInB, FirstName: &newName,
	})
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestUpdateUserCrossTenantTrueActualizaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	newName := "Actualizado Cross Tenant"
	updated, err := svc.UpdateUser(ctx, tenantA, userInB, true, false, &domainUsers.UpdateUserCommand{
		TenantID: tenantA, UserID: userInB, FirstName: &newName,
	})
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder editar a un usuario de tenantB")
	require.Equal(t, newName, updated.FirstName)
	require.Equal(t, tenantB, updated.TenantID, "el update debe haberse aplicado contra el tenant real del target")
}
