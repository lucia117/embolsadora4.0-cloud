package permissions_test

// Catálogo de permisos visto por la validación de roles (issue #93), y limpieza
// de roles.permissions al borrar un permiso custom. Integración: necesita
// DATABASE_URL.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/repo/pg/permissions"
)

func seedCustomRole(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, perms string) string {
	t.Helper()
	id := "custom_" + uuid.NewString()[:6]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions)
		 VALUES ($1, $1, 'rol de prueba', FALSE, FALSE, $2, $3::jsonb)`,
		id, tenantID, perms)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, id)
	})
	return id
}

func rolePermissions(t *testing.T, pool *pgxpool.Pool, roleID string) []string {
	t.Helper()
	var perms []string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT ARRAY(SELECT jsonb_array_elements_text(permissions)) FROM roles WHERE id = $1`,
		roleID).Scan(&perms))
	return perms
}

func TestUnknownPermissionIDs(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	propio := newCustomPermission(tenantA)
	require.NoError(t, repo.Create(ctx, propio))
	ajeno := newCustomPermission(tenantB)
	require.NoError(t, repo.Create(ctx, ajeno))

	unknown, err := repo.UnknownPermissionIDs(ctx, tenantA,
		[]string{"perm_dashboard", propio.ID, ajeno.ID, "perm_no_existe"})
	require.NoError(t, err)
	// Sistema y custom propio se conocen; el custom de otro tenant y el
	// inexistente no. El orden de entrada se respeta.
	require.Equal(t, []string{ajeno.ID, "perm_no_existe"}, unknown)

	unknown, err = repo.UnknownPermissionIDs(ctx, tenantA, []string{"perm_dashboard", propio.ID})
	require.NoError(t, err)
	require.Empty(t, unknown)
}

func TestDeleteQuitaElPermisoDeLosRolesDelTenant(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	p := newCustomPermission(tenantA)
	require.NoError(t, repo.Create(ctx, p))

	rolA := seedCustomRole(t, pool, tenantA, `["perm_dashboard","`+p.ID+`"]`)
	// Un rol de otro tenant con el mismo id (sembrado por SQL: la API no lo
	// permitiría) no se toca: la limpieza está acotada al tenant.
	rolB := seedCustomRole(t, pool, tenantB, `["`+p.ID+`"]`)

	require.NoError(t, repo.Delete(ctx, p.ID, tenantA))

	require.Equal(t, []string{"perm_dashboard"}, rolePermissions(t, pool, rolA))
	require.Equal(t, []string{p.ID}, rolePermissions(t, pool, rolB))
}

func TestSystemPermissionIDs(t *testing.T) {
	pool := newPool(t)
	repo := permissions.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantID := seedTenant(t, pool)
	custom := newCustomPermission(tenantID)
	require.NoError(t, repo.Create(ctx, custom))

	system, err := repo.SystemPermissionIDs(ctx, []string{"perm_tenants_manage", custom.ID, "perm_no_existe"})
	require.NoError(t, err)
	require.Equal(t, []string{"perm_tenants_manage"}, system)
}
