package roles_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// platformTenantUUID es el tenant MRG sembrado por la migración 000002.
var platformTenantUUID = uuid.MustParse("11b36b85-033d-4bb3-9e31-4c92161887c0")

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func roleIDs(roles []*domain.Role) []string {
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestListOcultaRolesGlobalesAlNoSuperadmin(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	roles, err := repo.List(context.Background(), platformTenantUUID, false)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.NotContains(t, ids, "super_admin", "super_admin debe ser invisible")
	require.NotContains(t, ids, "tenant_manager", "tenant_manager también es rol de plataforma")
	require.Contains(t, ids, "admin", "los roles tenant-scoped siguen visibles")
	require.Contains(t, ids, "operario")
}

func TestListMuestraRolesGlobalesAlSuperadmin(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	roles, err := repo.List(context.Background(), platformTenantUUID, true)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.Contains(t, ids, "super_admin")
	require.Contains(t, ids, "tenant_manager")
	require.Contains(t, ids, "admin")
}

func TestGetByIDForTenantDevuelveNotFoundParaRolOculto(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	// 404, no 403: un 403 confirmaría que el rol existe.
	_, err := repo.GetByIDForTenant(context.Background(), "super_admin", platformTenantUUID, false)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)

	role, err := repo.GetByIDForTenant(context.Background(), "super_admin", platformTenantUUID, true)
	require.NoError(t, err)
	require.Equal(t, "super_admin", role.ID)
}
