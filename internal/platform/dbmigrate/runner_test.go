package dbmigrate_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
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
	if version != 6 || dirty {
		t.Fatalf("expected version=6 dirty=false, got version=%d dirty=%v", version, dirty)
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

	expectedRolePermissions := map[string][]string{
		"super_admin": {
			"perm_dashboard", "perm_alerts", "perm_reports", "perm_users", "perm_tenants",
			"perm_settings", "perm_maintenance", "perm_analytics", "perm_all_tenants",
			"perm_logs_view", "perm_logs_export", "perm_logs_admin", "perm_edge_devices_view",
			"perm_edge_devices_manage", "perm_edge_devices_check", "perm_reports_view", "perm_reports_manage",
		},
		"tenant_manager": {
			"perm_all_tenants", "perm_dashboard", "perm_alerts", "perm_reports",
			"perm_reports_view", "perm_users", "perm_edge_devices_view", "perm_edge_devices_check",
		},
		"admin": {
			"perm_dashboard", "perm_alerts", "perm_reports", "perm_reports_view", "perm_reports_manage",
			"perm_users", "perm_tenants", "perm_settings", "perm_maintenance", "perm_analytics",
			"perm_edge_devices_view", "perm_edge_devices_manage",
		},
		"operario": {
			"perm_dashboard", "perm_alerts", "perm_reports_view", "perm_edge_devices_view", "perm_edge_devices_check",
		},
		"cliente_admin": {
			"perm_dashboard", "perm_alerts", "perm_reports_view", "perm_users", "perm_edge_devices_view",
		},
		"cliente_operario": {
			"perm_dashboard", "perm_edge_devices_view",
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
