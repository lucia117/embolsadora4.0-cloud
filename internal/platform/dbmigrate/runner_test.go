package dbmigrate_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/platform/dbmigrate"
)

// TestRun_AppliesAndIsIdempotent runs the real migrations from migrations/
// against a Postgres pointed to by DBMIGRATE_TEST_DATABASE_URL (falls back to
// DATABASE_URL). Skipped when neither is set so the unit suite stays fast.
func TestRun_AppliesAndIsIdempotent(t *testing.T) {
	dbURL := os.Getenv("DBMIGRATE_TEST_DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("DBMIGRATE_TEST_DATABASE_URL (or DATABASE_URL) not set")
	}
	logger := zap.NewNop()

	// Build an absolute path so the file:// URL is unambiguous regardless of CWD.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	migrationsAbs, err := filepath.Abs(filepath.Join(wd, "..", "..", "..", "migrations"))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	sourceURL := "file://" + filepath.ToSlash(migrationsAbs)

	if err := dbmigrate.Run(sourceURL, dbURL, logger); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(context.Background())

	var version int
	var dirty bool
	if err := conn.QueryRow(context.Background(),
		"SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 13 || dirty {
		t.Fatalf("expected version=13 dirty=false, got version=%d dirty=%v", version, dirty)
	}

	// Second run should be a no-op (ErrNoChange handled internally).
	if err := dbmigrate.Run(sourceURL, dbURL, logger); err != nil {
		t.Fatalf("second Run (idempotency): %v", err)
	}

	// 000005 translated the permission catalog to Spanish and populated
	// roles.permissions for the 6 system roles — assert both landed.
	var permName, permDescription string
	if err := conn.QueryRow(context.Background(),
		"SELECT name, description FROM permissions WHERE id = 'perm_dashboard'").
		Scan(&permName, &permDescription); err != nil {
		t.Fatalf("query perm_dashboard: %v", err)
	}
	if permName != "Ver Panel" || permDescription != "Acceso al panel principal y widgets" {
		t.Fatalf("expected translated perm_dashboard, got name=%q description=%q", permName, permDescription)
	}

	// Arrays leídos de la DB migrada a version 13 (000011 fine-grained users +
	// 000013 reseed edge). El orden importa: 000013 hace `permissions - k || [...]`,
	// que mueve las 4 claves perm_edge_devices_* al tail en el orden del literal.
	expectedRolePermissions := map[string][]string{
		"super_admin": {
			"perm_dashboard", "perm_alerts", "perm_reports", "perm_settings", "perm_maintenance",
			"perm_analytics", "perm_logs_view", "perm_logs_export", "perm_logs_admin",
			"perm_reports_view", "perm_reports_manage", "perm_users_view", "perm_users_manage",
			"perm_tenants_view", "perm_tenants_manage",
			"perm_edge_devices_view", "perm_edge_devices_check", "perm_edge_devices_manage", "perm_edge_devices_create",
		},
		"tenant_manager": {
			"perm_dashboard", "perm_alerts", "perm_reports", "perm_reports_view",
			"perm_users_view", "perm_users_manage", "perm_tenants_view",
			"perm_edge_devices_view", "perm_edge_devices_check", "perm_edge_devices_manage", "perm_edge_devices_create",
		},
		"admin": {
			"perm_dashboard", "perm_alerts", "perm_reports", "perm_reports_view", "perm_reports_manage",
			"perm_settings", "perm_maintenance", "perm_analytics", "perm_logs_view",
			"perm_users_view", "perm_users_manage", "perm_tenants_view",
			"perm_edge_devices_view", "perm_edge_devices_check", "perm_edge_devices_manage",
		},
		"operario": {
			"perm_dashboard", "perm_alerts", "perm_reports_view", "perm_edge_devices_view", "perm_edge_devices_check",
		},
		// cliente_admin: la DB local de desarrollo trae `perm_users` (permiso
		// grueso pre-000011) en vez de `perm_users_view`/`perm_users_manage` —
		// suciedad de la DB local documentada en el ledger del controller, NO el
		// estado que produce 000011 en limpio. El `want` de abajo es el array
		// que una DB limpia (000011 + 000013) dejaría: base sin claves edge
		// (perm_dashboard, perm_alerts, perm_reports_view, perm_users_view,
		// perm_users_manage) + las claves edge que 000013 mueve al tail para
		// cliente_admin (view, check, manage). Si la DB local está sucia, esta
		// entrada falla con `got` conteniendo "perm_users"; eso es esperado y no
		// bloquea (ver task-A4-report.md).
		"cliente_admin": {
			"perm_dashboard", "perm_alerts", "perm_reports_view",
			"perm_users_view", "perm_users_manage",
			"perm_edge_devices_view", "perm_edge_devices_check", "perm_edge_devices_manage",
		},
		"cliente_operario": {
			"perm_dashboard", "perm_edge_devices_view", "perm_edge_devices_check",
		},
	}

	for roleID, want := range expectedRolePermissions {
		var got []string
		if err := conn.QueryRow(context.Background(),
			"SELECT permissions FROM roles WHERE id = $1", roleID).Scan(&got); err != nil {
			t.Fatalf("query role %s permissions: %v", roleID, err)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("role %s: expected permissions %v, got %v", roleID, want, got)
		}
	}
}

func TestAdminRoleHasLogsView(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var perms []string
	err = pool.QueryRow(ctx,
		`SELECT ARRAY(SELECT jsonb_array_elements_text(permissions)) FROM roles WHERE id = 'admin'`,
	).Scan(&perms)
	require.NoError(t, err)
	require.Contains(t, perms, "perm_logs_view")
}

func TestAllTenantsPermissionIsGone(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var count int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM permissions WHERE id = 'perm_all_tenants'`,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "perm_all_tenants debe estar eliminado del catálogo")

	var rolesWithIt int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM roles WHERE permissions @> '["perm_all_tenants"]'::jsonb`,
	).Scan(&rolesWithIt)
	require.NoError(t, err)
	require.Equal(t, 0, rolesWithIt, "ningún rol debe conservar perm_all_tenants")
}
