# Roles & Permissions Seed Data Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new database migration that translates the 17 system permission catalog rows to Spanish and populates the `permissions` jsonb column for all 6 system roles, fixing two data-only bugs surfaced in frontend integration testing.

**Architecture:** A single new migration file pair (`000005_translate_permissions_and_seed_role_permissions.{up,down}.sql`) using idempotent `UPDATE` statements against the existing `permissions` and `roles` tables — no schema changes, no application code changes. A focused Go test in `internal/platform/dbmigrate/runner_test.go` verifies the migration applies and produces the expected data.

**Tech Stack:** PostgreSQL (`golang-migrate` SQL migrations), Go 1.24 (`pgx/v5` for the verification test).

## Global Constraints

- New migration only — do not edit `migrations/000002_seed_essentials.up.sql` or `.down.sql`. That migration has already been applied against the live Supabase-hosted database; editing its literal `INSERT` values has no effect there because its inserts use `ON CONFLICT DO NOTHING`.
- `internal/security/rbac.go`'s `rolePermissions` map is a separate vocabulary (`"resource:action"` strings) used for actual backend authorization — do not touch it, and do not use its permission strings in this migration.
- The `permissions` jsonb values on `roles` use the `perm_*` catalog IDs (from the `permissions` table), matching what `embolsadora-frontend` expects for display and UI gating.
- All `go` commands run via Docker per this repo's `CLAUDE.md` (Go is not installed on the host):
  ```bash
  docker run --rm \
    -v /tmp/go-mod-cache:/go/pkg/mod \
    -v /tmp/go-build-cache:/root/.cache/go-build \
    -v "$(pwd)":/app -w /app \
    golang:1.24-alpine sh -c "<cmd>"
  ```
- Local Postgres for manual/dbmigrate-test verification comes from this repo's `docker-compose.yml` (`db` service, `postgres:16-alpine`, published on `localhost:5432`, credentials `embolsadora_user`/`embolsadora_password`/db `embolsadora_dev`). The host already has `migrate`, `docker`, and `psql` CLIs installed — no need to route those through Docker.

---

### Task 1: Create migration 000005 (translations + role permissions)

**Files:**
- Create: `migrations/000005_translate_permissions_and_seed_role_permissions.up.sql`
- Create: `migrations/000005_translate_permissions_and_seed_role_permissions.down.sql`

**Interfaces:**
- Consumes: existing `permissions` table (`id`, `name`, `description` columns) and `roles` table (`id`, `permissions` jsonb column) from `migrations/000001_initial_schema.up.sql` / `000002_seed_essentials.up.sql`. No Go code involved in this task.
- Produces: a migration that, once applied, leaves `schema_migrations.version = 5`. Task 2 depends on this exact version number and on the exact `name`/`description`/`permissions` values below.

- [ ] **Step 1: Write the up migration**

Create `migrations/000005_translate_permissions_and_seed_role_permissions.up.sql`:

```sql
-- ============================================================================
-- Migration 000005: Translate permission catalog + seed role permissions
-- ============================================================================
-- 000002_seed_essentials seeded the 17 system permissions with English
-- name/description, and all 6 system roles with permissions: '[]'::jsonb.
-- This migration is a data-only fix, applied as UPDATEs (not edits to 000002)
-- because 000002 has already run against the live database and its INSERTs
-- use ON CONFLICT DO NOTHING, so re-running it with edited values would not
-- change already-seeded rows.
--
-- All statements are UPDATEs against existing rows by primary key, so this
-- migration is idempotent and safe to re-run.
-- ============================================================================

-- 1. Translate the 17 system permissions to Spanish (name + description).
UPDATE permissions SET name = 'Ver Panel', description = 'Acceso al panel principal y widgets' WHERE id = 'perm_dashboard';
UPDATE permissions SET name = 'Ver Alertas', description = 'Acceso al centro de alertas y notificaciones' WHERE id = 'perm_alerts';
UPDATE permissions SET name = 'Ver Reportes', description = 'Acceso a reportes y analítica' WHERE id = 'perm_reports';
UPDATE permissions SET name = 'Gestionar Usuarios', description = 'Crear, editar y eliminar usuarios' WHERE id = 'perm_users';
UPDATE permissions SET name = 'Gestionar Tenants', description = 'Acceso a la gestión de tenants' WHERE id = 'perm_tenants';
UPDATE permissions SET name = 'Gestionar Configuración', description = 'Acceso a la configuración del sistema' WHERE id = 'perm_settings';
UPDATE permissions SET name = 'Ver Mantenimiento', description = 'Acceso al módulo de mantenimiento' WHERE id = 'perm_maintenance';
UPDATE permissions SET name = 'Ver Analítica', description = 'Acceso a paneles de analítica avanzada' WHERE id = 'perm_analytics';
UPDATE permissions SET name = 'Acceso a Todos los Tenants', description = 'Acceso cross-tenant (solo Super Admin)' WHERE id = 'perm_all_tenants';
UPDATE permissions SET name = 'Ver Logs', description = 'Acceso al visor de logs' WHERE id = 'perm_logs_view';
UPDATE permissions SET name = 'Exportar Logs', description = 'Exportar datos de logs a archivo' WHERE id = 'perm_logs_export';
UPDATE permissions SET name = 'Gestionar Configuración de Logs', description = 'Gestionar retención y configuración de logs' WHERE id = 'perm_logs_admin';
UPDATE permissions SET name = 'Ver Dispositivos Edge', description = 'Ver el listado y estado de dispositivos edge' WHERE id = 'perm_edge_devices_view';
UPDATE permissions SET name = 'Gestionar Dispositivos Edge', description = 'Crear, editar, habilitar y deshabilitar dispositivos edge' WHERE id = 'perm_edge_devices_manage';
UPDATE permissions SET name = 'Ejecutar Chequeos Edge', description = 'Ejecutar chequeos de estado y salud en dispositivos edge' WHERE id = 'perm_edge_devices_check';
UPDATE permissions SET name = 'Ver Reportes', description = 'Acceso al historial de reportes y descargas' WHERE id = 'perm_reports_view';
UPDATE permissions SET name = 'Gestionar Reportes', description = 'Generar reportes, gestionar programaciones y retención' WHERE id = 'perm_reports_manage';

-- 2. Populate roles.permissions for the 6 system roles (perm_* catalog IDs).
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports","perm_users","perm_tenants","perm_settings","perm_maintenance","perm_analytics","perm_all_tenants","perm_logs_view","perm_logs_export","perm_logs_admin","perm_edge_devices_view","perm_edge_devices_manage","perm_edge_devices_check","perm_reports_view","perm_reports_manage"]'::jsonb WHERE id = 'super_admin';
UPDATE roles SET permissions = '["perm_all_tenants","perm_dashboard","perm_alerts","perm_reports","perm_reports_view","perm_users","perm_edge_devices_view","perm_edge_devices_check"]'::jsonb WHERE id = 'tenant_manager';
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports","perm_reports_view","perm_reports_manage","perm_users","perm_tenants","perm_settings","perm_maintenance","perm_analytics","perm_edge_devices_view","perm_edge_devices_manage"]'::jsonb WHERE id = 'admin';
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports_view","perm_edge_devices_view","perm_edge_devices_check"]'::jsonb WHERE id = 'operario';
UPDATE roles SET permissions = '["perm_dashboard","perm_alerts","perm_reports_view","perm_users","perm_edge_devices_view"]'::jsonb WHERE id = 'cliente_admin';
UPDATE roles SET permissions = '["perm_dashboard","perm_edge_devices_view"]'::jsonb WHERE id = 'cliente_operario';
```

- [ ] **Step 2: Write the down migration**

Create `migrations/000005_translate_permissions_and_seed_role_permissions.down.sql`:

```sql
-- Reverts 000005: restore English permission catalog text and clear
-- roles.permissions back to the empty state left by 000002.

-- 1. Revert roles.permissions to empty.
UPDATE roles SET permissions = '[]'::jsonb WHERE id IN ('super_admin', 'tenant_manager', 'admin', 'operario', 'cliente_admin', 'cliente_operario');

-- 2. Revert permission catalog to English (values as seeded by 000002).
UPDATE permissions SET name = 'View Dashboard', description = 'Access to main dashboard and widgets' WHERE id = 'perm_dashboard';
UPDATE permissions SET name = 'View Alerts', description = 'Access to alerts and notification center' WHERE id = 'perm_alerts';
UPDATE permissions SET name = 'View Reports', description = 'Access to reports and analytics' WHERE id = 'perm_reports';
UPDATE permissions SET name = 'Manage Users', description = 'Create, edit and delete users' WHERE id = 'perm_users';
UPDATE permissions SET name = 'Manage Tenants', description = 'Access to tenant management' WHERE id = 'perm_tenants';
UPDATE permissions SET name = 'Manage Settings', description = 'Access to system settings' WHERE id = 'perm_settings';
UPDATE permissions SET name = 'View Maintenance', description = 'Access to maintenance module' WHERE id = 'perm_maintenance';
UPDATE permissions SET name = 'View Analytics', description = 'Access to analytics dashboards' WHERE id = 'perm_analytics';
UPDATE permissions SET name = 'Access All Tenants', description = 'Cross-tenant access (Super Admin only)' WHERE id = 'perm_all_tenants';
UPDATE permissions SET name = 'View Logs', description = 'Access to log viewer' WHERE id = 'perm_logs_view';
UPDATE permissions SET name = 'Export Logs', description = 'Export log data to file' WHERE id = 'perm_logs_export';
UPDATE permissions SET name = 'Manage Log Settings', description = 'Manage log retention and configuration' WHERE id = 'perm_logs_admin';
UPDATE permissions SET name = 'View Edge Devices', description = 'View edge device list and status' WHERE id = 'perm_edge_devices_view';
UPDATE permissions SET name = 'Manage Edge Devices', description = 'Create, edit, enable and disable edge devices' WHERE id = 'perm_edge_devices_manage';
UPDATE permissions SET name = 'Run Edge Checks', description = 'Execute status and health checks on edge devices' WHERE id = 'perm_edge_devices_check';
UPDATE permissions SET name = 'View Reports', description = 'Access to report history and download' WHERE id = 'perm_reports_view';
UPDATE permissions SET name = 'Manage Reports', description = 'Generate reports, manage schedules and retention settings' WHERE id = 'perm_reports_manage';
```

- [ ] **Step 3: Start a clean local Postgres**

```bash
docker compose down db -v 2>/dev/null    # in case a stale volume exists from earlier work
docker compose up -d db
```

Wait for healthy (poll every 2s, up to ~30s):

```bash
until [ "$(docker inspect --format='{{.State.Health.Status}}' embolsadora_db)" = "healthy" ]; do sleep 2; done
echo ready
```

Expected: prints `ready`.

- [ ] **Step 4: Apply all migrations including the new one**

```bash
export DBURL="postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable"
migrate -path migrations/ -database "$DBURL" up
psql "$DBURL" -c "SELECT version, dirty FROM schema_migrations;"
```

Expected: four `N/u <name> (...)` lines ending with `5/u translate_permissions_and_seed_role_permissions (...)`, then:
```
 version | dirty
---------+-------
       5 | f
```

- [ ] **Step 5: Verify the data with psql**

```bash
psql "$DBURL" -c "SELECT id, name, description FROM permissions ORDER BY id;"
psql "$DBURL" -c "SELECT id, permissions FROM roles ORDER BY id;"
```

Expected: `name`/`description` in Spanish for all 17 rows (e.g. `perm_dashboard` → `Ver Panel` / `Acceso al panel principal y widgets`), and each of the 6 roles has a non-empty `permissions` array matching Step 1's values (e.g. `admin` has 12 entries including `perm_tenants` and `perm_edge_devices_manage`; `cliente_operario` has exactly `["perm_dashboard", "perm_edge_devices_view"]`).

- [ ] **Step 6: Verify the down migration round-trips cleanly**

```bash
migrate -path migrations/ -database "$DBURL" down 1
psql "$DBURL" -c "SELECT id, name FROM permissions WHERE id = 'perm_dashboard';"
psql "$DBURL" -c "SELECT id, permissions FROM roles WHERE id = 'admin';"
migrate -path migrations/ -database "$DBURL" up 1
psql "$DBURL" -c "SELECT id, name FROM permissions WHERE id = 'perm_dashboard';"
```

Expected: after `down 1`, `perm_dashboard` name is back to `View Dashboard` and `admin`'s `permissions` is `[]`; after `up 1` again, `perm_dashboard` name is back to `Ver Panel`.

- [ ] **Step 7: Tear down the local Postgres**

```bash
docker compose down db -v
```

- [ ] **Step 8: Commit**

```bash
git add migrations/000005_translate_permissions_and_seed_role_permissions.up.sql migrations/000005_translate_permissions_and_seed_role_permissions.down.sql
git commit -m "feat: translate permission catalog to Spanish and seed role permissions"
```

---

### Task 2: Extend migration test coverage

**Files:**
- Modify: `internal/platform/dbmigrate/runner_test.go`

**Interfaces:**
- Consumes: `dbmigrate.Run(sourceURL, dbURL string, logger *zap.Logger) error` (existing function, unchanged signature) and the exact `permissions`/`roles` values Task 1 wrote to the database.
- Produces: nothing consumed by later tasks — this is the last task in the plan.

**Context:** `TestRun_AppliesAndIsIdempotent` currently asserts `version != 2` after running all migrations, which is already stale — this repo has 4 migration files (`000001`-`000004`), so a correct run leaves `version == 4` today, and `5` once Task 1's migration is added. This stale assertion has gone uncaught because the test is skipped whenever `DBMIGRATE_TEST_DATABASE_URL`/`DATABASE_URL` isn't set (true in CI today). Fix the version number in the same edit that adds the new assertions, since both stem from the same migration count change.

- [ ] **Step 1: Read the current test to confirm the exact insertion points**

Run: `sed -n '1,64p' internal/platform/dbmigrate/runner_test.go`

Expected: the file shown in this plan's context above — one test function, `version != 2` check around line 55.

- [ ] **Step 2: Fix the stale version assertion and add data assertions**

Replace the whole body of `TestRun_AppliesAndIsIdempotent` in `internal/platform/dbmigrate/runner_test.go` with:

```go
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
	if version != 5 || dirty {
		t.Fatalf("expected version=5 dirty=false, got version=%d dirty=%v", version, dirty)
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
```

Add `"slices"` to the import block (alongside the existing `"context"`, `"os"`, `"path/filepath"`, `"testing"`):

```go
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
```

- [ ] **Step 3: Run the test against a local Postgres**

```bash
docker compose down db -v 2>/dev/null
docker compose up -d db
until [ "$(docker inspect --format='{{.State.Health.Status}}' embolsadora_db)" = "healthy" ]; do sleep 2; done

docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v "$(pwd)":/app -w /app \
  --add-host=host.docker.internal:host-gateway \
  -e DBMIGRATE_TEST_DATABASE_URL="postgres://embolsadora_user:embolsadora_password@host.docker.internal:5432/embolsadora_dev?sslmode=disable" \
  golang:1.24-alpine sh -c "go test ./internal/platform/dbmigrate/... -run TestRun_AppliesAndIsIdempotent -v"

docker compose down db -v
```

Expected: `--- PASS: TestRun_AppliesAndIsIdempotent` and final `PASS` / `ok`.

- [ ] **Step 4: Run the full test suite to confirm no regressions**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v "$(pwd)":/app -w /app \
  golang:1.24-alpine sh -c "go build ./... && go vet ./... && go test ./..."
```

Expected: build succeeds, vet is clean, all tests `ok` (the `dbmigrate` package's other tests will `SKIP` here since `DATABASE_URL`/`DBMIGRATE_TEST_DATABASE_URL` aren't set in this run — that's expected and matches today's baseline).

- [ ] **Step 5: Commit**

```bash
git add internal/platform/dbmigrate/runner_test.go
git commit -m "test: verify translated permissions and role permission seeding in migration 000005"
```

---

## Self-Review Notes

- **Spec coverage:** Design §1 (new migration, not editing 000002) → Task 1. Design §2 (permission-to-role mapping table) → Task 1 Step 1's `UPDATE roles` statements, byte-for-byte from the spec's table. Design §3 (translation table) → Task 1 Step 1's `UPDATE permissions` statements, byte-for-byte from the spec's table. Down migration → Task 1 Step 2. Testing section → Task 2.
- **Placeholder scan:** none found — every step has literal, runnable SQL/Go/shell content.
- **Type consistency:** Task 2's `expectedRolePermissions` role IDs and permission ID strings match Task 1's `UPDATE roles` statements exactly (verified by diffing while writing this plan). `dbmigrate.Run`'s signature is unchanged from the existing `runner_test.go` — Task 2 doesn't touch `runner.go`.
- **Manual verification performed while writing this plan:** the exact SQL in Task 1 Steps 1–2 was run against a real local Postgres (this repo's `docker-compose.yml` `db` service) end-to-end — up, data verification, down, and re-up — before being written into this plan. All of it worked as described above.
