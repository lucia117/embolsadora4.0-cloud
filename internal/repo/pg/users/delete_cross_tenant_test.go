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

// TestDeleteUserCrossTenant cubre Hallazgo C: DeleteUser hardcodeaba
// crossTenant=false en su precheck, así que un super_admin parado en tenantA
// nunca podía borrar a un usuario cuya membresía real vive en tenantB —
// recibía 404 aunque el usuario existiera. Ver
// docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md §A.
func TestDeleteUserCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	err := svc.DeleteUser(ctx, tenantA, userInB, false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")

	var deletedAt *string
	scanErr := pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", userInB).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.Nil(t, deletedAt, "el usuario no debe haberse borrado")
}

func TestDeleteUserCrossTenantTrueBorraUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	err := svc.DeleteUser(ctx, tenantA, userInB, true, false)
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder borrar a un usuario de tenantB")

	var deletedAt *string
	scanErr := pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", userInB).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.NotNil(t, deletedAt, "el usuario debe haber quedado soft-deleted")
}
