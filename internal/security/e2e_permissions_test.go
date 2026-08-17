package security_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// TestCustomRolePermissionsAreEnforced es el criterio de cierre real de B-004:
// un rol custom creado con un subconjunto de permisos (acá, solo
// perm_dashboard + perm_users_view, sin perm_users_manage) efectivamente
// limita lo que security.Can() autoriza — sin pasar por ningún mapa Go
// hardcodeado, el gap que hacía que B-004 nunca se cerrara de verdad.
func TestCustomRolePermissionsAreEnforced(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	tenantID := uuid.New()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain, is_platform_tenant)
		 VALUES ($1, 'B-004 E2E Test', 'B-004 E2E Test', $2, FALSE)`,
		tenantID, "b004-e2e-"+tenantID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	roleID := "custom_" + uuid.New().String()[:6]
	_, err = pool.Exec(context.Background(),
		`INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions)
		 VALUES ($1, 'E2E Read Only', 'test', FALSE, FALSE, $2, '["perm_dashboard","perm_users_view"]'::jsonb)`,
		roleID, tenantID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleID)
	})

	var permissions []string
	var isGlobal bool
	err = pool.QueryRow(context.Background(),
		`SELECT ARRAY(SELECT jsonb_array_elements_text(permissions)), is_global
		 FROM roles WHERE id = $1`, roleID,
	).Scan(&permissions, &isGlobal)
	require.NoError(t, err)

	ctx := security.WithRoleContext(context.Background(), security.RoleContext{
		Name:        roleID,
		Permissions: permissions,
		IsGlobal:    isGlobal,
	})

	require.NoError(t, security.Can(ctx, "perm_dashboard"), "el rol custom tiene perm_dashboard")
	require.NoError(t, security.Can(ctx, "perm_users_view"), "el rol custom tiene perm_users_view")

	err = security.Can(ctx, "perm_users_manage")
	require.True(t, errors.Is(err, domain.ErrForbidden), "el rol custom NO tiene perm_users_manage, Can() debe negarlo: %v", err)
}
