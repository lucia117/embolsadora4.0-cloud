package roles_test

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestSeedPermissionsMatchDesign verifica que la migración 000011 dejó
// roles.permissions exactamente como especifica la tabla de mapeo de
// docs/superpowers/specs/2026-08-17-rbac-dynamic-permissions-design.md §3.4 —
// es la traducción 1:1 del mapa Go que existía en rbac.go antes
// de esta migración, para los recursos users/tenants.
func TestSeedPermissionsMatchDesign(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	cases := []struct {
		roleID       string
		wantIsGlobal bool
		wantUsers    []string // subset de perm_users_view/perm_users_manage esperado
		wantTenants  []string // subset de perm_tenants_view/perm_tenants_manage esperado
	}{
		{"super_admin", true, []string{"perm_users_view", "perm_users_manage"}, []string{"perm_tenants_view", "perm_tenants_manage"}},
		{"tenant_manager", true, []string{"perm_users_view", "perm_users_manage"}, []string{"perm_tenants_view"}},
		{"admin", false, []string{"perm_users_view", "perm_users_manage"}, []string{"perm_tenants_view"}},
		{"platform_admin", true, []string{"perm_users_view", "perm_users_manage"}, []string{"perm_tenants_view", "perm_tenants_manage"}},
		{"operario", false, nil, nil},
		{"cliente_admin", false, []string{"perm_users_view", "perm_users_manage"}, nil},
		{"cliente_operario", false, nil, nil},
	}

	for _, c := range cases {
		t.Run(c.roleID, func(t *testing.T) {
			var permissions []string
			var isGlobal bool
			err := pool.QueryRow(context.Background(),
				`SELECT ARRAY(SELECT jsonb_array_elements_text(permissions)), is_global
				 FROM roles WHERE id = $1 AND deleted_at IS NULL`,
				c.roleID,
			).Scan(&permissions, &isGlobal)
			require.NoError(t, err, "rol %q debe existir tras la migración 000011", c.roleID)

			require.Equal(t, c.wantIsGlobal, isGlobal, "is_global de %q", c.roleID)

			sort.Strings(permissions)
			for _, want := range c.wantUsers {
				require.Contains(t, permissions, want, "%q debería tener %q", c.roleID, want)
			}
			for _, want := range c.wantTenants {
				require.Contains(t, permissions, want, "%q debería tener %q", c.roleID, want)
			}
			require.NotContains(t, permissions, "perm_users", "%q no debería tener el permiso grueso perm_users", c.roleID)
			require.NotContains(t, permissions, "perm_tenants", "%q no debería tener el permiso grueso perm_tenants", c.roleID)
		})
	}
}

// TestSeedPermissionsCatalogCleanedUp verifica que perm_users/perm_tenants ya
// no existen en el catálogo (fueron reemplazados por las variantes _view/_manage).
func TestSeedPermissionsCatalogCleanedUp(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM permissions WHERE id IN ('perm_users', 'perm_tenants')`,
	).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count, "perm_users/perm_tenants deberían estar borrados del catálogo")
}
