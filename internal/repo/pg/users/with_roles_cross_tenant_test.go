package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestGetByIDWithRolesCrossTenant cubre el item #3 del handoff 2026-08-19: el
// fix de Hallazgo A solo llegó a GetByID (la variante sin ?include=roles),
// GetByIDWithRoles quedó con el mismo bug de resolución cross-tenant.
func TestGetByIDWithRolesCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	_, err := repo.GetByIDWithRoles(ctx, tenantA, userInB, false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestGetByIDWithRolesCrossTenantTrueEncuentraUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	uwr, err := repo.GetByIDWithRoles(ctx, tenantA, userInB, true, false)
	require.NoError(t, err)
	require.Equal(t, userInB, uwr.User.ID)
	require.Equal(t, tenantB, uwr.User.TenantID, "el tenant_id debe ser el real del target")
	require.Len(t, uwr.Roles, 1)
	require.Equal(t, "cliente_operario", uwr.Roles[0].ID)
}
