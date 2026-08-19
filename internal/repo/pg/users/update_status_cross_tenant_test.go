package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestUpdateUserStatusCrossTenant cubre la extensión de Hallazgo C acordada a
// UpdateUserStatus: mismo root cause, con una capa extra de bug — la
// mutación real (userRoleRepo.UpdateStatus) usaba el tenantID de la request
// en vez del tenant real del target, así que incluso resolviendo el precheck
// no alcanzaba (ver el fix en service.go para el detalle).
const dummyCallerID = "00000000-0000-0000-0000-000000000000"

func TestUpdateUserStatusCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	_, err := svc.UpdateUserStatus(ctx, tenantA, userInB, dummyCallerID, "suspended", false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestUpdateUserStatusCrossTenantTrueActualizaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	updated, err := svc.UpdateUserStatus(ctx, tenantA, userInB, dummyCallerID, "suspended", true, false)
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder suspender a un usuario de tenantB")
	require.Equal(t, tenantB, updated.TenantID)
}
