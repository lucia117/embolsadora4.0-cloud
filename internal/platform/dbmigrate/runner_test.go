package dbmigrate_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/platform/dbmigrate"
)

// upMigrationVersionRe extrae el prefijo numerico de un archivo de migracion
// "up" (p.ej. "000008_edge_device_api_keys.up.sql" -> "000008").
var upMigrationVersionRe = regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)

// highestMigrationVersion inspecciona migrationsDir y devuelve el mayor
// prefijo numerico entre los archivos *.up.sql, para no hardcodear la
// version de schema esperada por el test (se desincroniza cada vez que se
// agrega una migracion nueva).
func highestMigrationVersion(t *testing.T, migrationsDir string) int {
	t.Helper()
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir %s: %v", migrationsDir, err)
	}
	highest := -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := upMigrationVersionRe.FindStringSubmatch(entry.Name())
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("parse migration version from %s: %v", entry.Name(), err)
		}
		if v > highest {
			highest = v
		}
	}
	if highest < 0 {
		t.Fatalf("no *.up.sql migration files found in %s", migrationsDir)
	}
	return highest
}

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

	expectedVersion := highestMigrationVersion(t, migrationsAbs)

	var version int
	var dirty bool
	if err := conn.QueryRow(context.Background(),
		"SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != expectedVersion || dirty {
		t.Fatalf("expected version=%d (highest *.up.sql found in %s) dirty=false, got version=%d dirty=%v",
			expectedVersion, migrationsAbs, version, dirty)
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

	// Conjuntos leídos de la DB migrada a version 13 (000011 fine-grained users +
	// 000013 reseed edge). La comparación de abajo es order-independent (ordena
	// copias de ambos lados), así que el orden en que 000013 hace
	// `permissions - k || [...]` no importa — solo el conjunto de claves.
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
		// cliente_admin: conjunto limpio que dejan 000011 (users fine-grained) +
		// 000013 (reseed edge): base sin claves edge + view/check/manage.
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
		// Order-independent: la migración usa `permissions - k || [...]`, que
		// reordena el JSONB. Lo que importa es el conjunto de claves, no su orden.
		gotSorted := slices.Clone(got)
		wantSorted := slices.Clone(want)
		slices.Sort(gotSorted)
		slices.Sort(wantSorted)
		if !slices.Equal(gotSorted, wantSorted) {
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
