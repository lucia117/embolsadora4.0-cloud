# RBAC: permisos dinámicos (backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `security.Can()` deja de leer el mapa Go hardcodeado `rolePermissions` y pasa a leer siempre `roles.permissions` (DB) — cierra B-004 de verdad: un rol custom asignado hoy no otorga permisos funcionales reales, y va a empezar a hacerlo.

**Architecture:** `TenantFromHeader` (middleware) hace una query extra por request para cargar `permissions` + `is_global` del rol efectivo ya resuelto, y las guarda en una única estructura de contexto (`security.RoleContext`). `Can()` e `IsCrossTenantRole()` leen de ahí en vez de mapas hardcodeados. `platform_admin` — hoy 100% virtual — pasa a ser una fila real en `roles`. Una migración SQL nueva (000011) siembra el catálogo `perm_users_view/_manage` y `perm_tenants_view/_manage` y resiembra los 7 roles de sistema replicando exactamente el comportamiento que hoy define el mapa Go.

**Tech Stack:** Go 1.24, Gin, pgx/v5, golang-migrate, testify. Todos los comandos `go` corren por Docker (Go no está instalado en el host).

## Global Constraints

- No hay datos reales que migrar (MVP, confirmado con el usuario) — la migración puede hacer `DELETE`/reseed directos sin capa de compatibilidad.
- `machines:*` y las rutas `/edge-devices/*` sin RBAC quedan **fuera de alcance** — no tocar.
- No agregar gating de `_manage` a nivel de botón en ningún handler — ya está fuera de alcance (es un tema de frontend, spec §2.6).
- Seguir el estilo de migraciones ya establecido: UPDATEs idempotentes con operadores jsonb `-`/`||`/`@>` (ver 000008, 000009), nunca overwrite completo del array.
- Spec de referencia: `embolsadora-frontend/docs/superpowers/specs/2026-08-17-rbac-dynamic-permissions-design.md` (commit `9b050fa` en `develop` de `embolsadora-frontend`).

---

## File Structure

- **Create:** `migrations/000011_dynamic_role_permissions.up.sql` / `.down.sql` — catálogo `_view`/`_manage` + fila `platform_admin` + reseed de los 7 roles.
- **Create:** `internal/repo/pg/roles/seed_test.go` — test de integración que verifica que el reseed de la migración 000011 coincide exactamente con lo que hoy define el mapa Go.
- **Modify:** `internal/security/rbac.go` — nuevo `RoleContext`, `Can()`/`IsCrossTenantRole()` leen de contexto, se borran `rolePermissions`, `crossTenantRoles`, `PermissionsForRole`.
- **Modify:** `internal/security/rbac_test.go` — tests reescritos contra la nueva API.
- **Modify:** `internal/api/router.go`, `internal/routes/url_mappings.go`, `internal/api/handler/logs/routes.go` — 24 call sites de `RBACCheck(...)` migrados del vocabulario `resource:action` (`"users:write"`, `"tenants:read"`, `"invitations:write"`, `"logs:admin"`) al catálogo `perm_*` que `Can()` pasa a entender exclusivamente.
- **Modify:** `internal/api/middleware/middleware.go` — `TenantFromHeader` carga permisos+is_global del rol efectivo desde la DB.
- **Modify:** `internal/api/usecases/me_usecase.go` y `internal/api/usecases/me_usecase_test.go` — `GetMe` resuelve permisos del rol efectivo, no del rol crudo; dos aserciones del test existente que apuntaban a `perm_tenants`/`perm_users` se actualizan a las variantes `_view`/`_manage`.
- **Modify:** `internal/api/handler/tenants/get_all_tenants/get_all_tenants.go`, `get_tenant/get_tenant.go`, `delete_tenant/delete_tenant.go`, `update_tenant/update_tenant.go`, `internal/api/handler/user_roles/get_user_roles/get_user_roles.go` — 5 call sites de `IsCrossTenantRole`, nueva firma.
- **Modify:** `internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go`, `get_tenant/get_tenant_test.go`, `delete_tenant/delete_tenant_test.go`, `update_tenant/update_tenant_test.go` — el helper `withActorContext` de cada uno pasa a setear `IsGlobal` explícitamente, no solo el nombre del rol.

**⚠️ Nota crítica de orden:** Task 2 (rewrite de `Can()`) y Task 3 (call sites de `RBACCheck`) tienen que aterrizar juntos antes de mergear la rama. `Can()` pasa a comparar el `perm` recibido contra `RoleContext.Permissions`, que desde la migración 000011 (Task 1) solo contiene ids `perm_*` — ningún rol va a tener nunca un string literal `"users:write"` en su lista. Si Task 3 no se hace, **todas las rutas admin devuelven 403 a todo el mundo** apenas se mergee Task 2, porque ninguna de las 24 llamadas a `RBACCheck(...)` del router pasa un id `perm_*` todavía.

---

### Task 1: Migración 000011 — catálogo `_view`/`_manage`, fila `platform_admin`, reseed

**Files:**
- Create: `migrations/000011_dynamic_role_permissions.up.sql`
- Create: `migrations/000011_dynamic_role_permissions.down.sql`
- Test: `internal/repo/pg/roles/seed_test.go`

**Interfaces:**
- Consumes: nada (solo SQL + `internal/repo/pg/roles` ya existente, `pgxpool.Pool`).
- Produces: catálogo `permissions` con `perm_users_view`, `perm_users_manage`, `perm_tenants_view`, `perm_tenants_manage` (y sin `perm_users`/`perm_tenants`); fila `roles.id = 'platform_admin'`; `roles.permissions` resembrado para los 7 roles de sistema. Tasks 2-4 asumen que esta data ya existe en cualquier DB de test/dev/prod donde corran.

- [ ] **Step 1: Escribir la migración up**

`migrations/000011_dynamic_role_permissions.up.sql`:

```sql
-- ============================================================================
-- Migration 000011: permisos dinámicos — catálogo perm_users/tenants view+manage,
-- fila platform_admin, reseed de los 7 roles de sistema
-- ============================================================================
-- Cierra B-004: security.Can() va a leer roles.permissions en vez del mapa Go
-- hardcodeado (ver internal/security/rbac.go, cambia en un commit de Go aparte
-- de esta migración). Ver docs/superpowers/specs/2026-08-17-rbac-dynamic-permissions-design.md
-- en embolsadora-frontend (spec cross-repo) para el diseño completo y la tabla
-- de mapeo de la que sale este reseed.
--
-- 1. Nuevo catálogo perm_users_view/_manage y perm_tenants_view/_manage,
--    reemplazando los permisos gruesos perm_users/perm_tenants (sin
--    distinción de acción — la causa de que un admin de tenant cliente viera
--    la sección Tenants en el menú y recibiera 403 al intentar escribir).
-- 2. Fila nueva platform_admin en roles: hoy es 100% virtual (derivado en
--    runtime por security.EffectiveRole), sin ninguna fila propia.
-- 3. Reseed de roles.permissions para los 7 roles de sistema, replicando
--    exactamente lo que hoy define rolePermissions en rbac.go.
-- ============================================================================

-- 1. Catálogo: agregar los 4 permisos nuevos.
INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_users_view',     'Ver Usuarios',      'users',   'Ver el listado de usuarios y sus roles',          TRUE, NULL),
    ('perm_users_manage',   'Gestionar Usuarios','users',   'Crear, editar, eliminar usuarios e invitaciones', TRUE, NULL),
    ('perm_tenants_view',   'Ver Tenants',       'tenants', 'Ver el listado de tenants',                       TRUE, NULL),
    ('perm_tenants_manage', 'Gestionar Tenants', 'tenants', 'Crear, editar y eliminar tenants',                TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

-- 2. Fila platform_admin: mismo shape que los otros roles is_global=TRUE
--    (super_admin, tenant_manager) — solo visible dentro del tenant
--    plataforma vía tenant_can_use_role (migraciones 000004/000010).
INSERT INTO roles (id, name, description, is_system_role, is_global, tenant_id, permissions) VALUES
    ('platform_admin', 'Platform Admin', 'Admin de tenant cuya membresía pertenece al tenant plataforma MRG. Mismos permisos que Admin más gestión de tenants.', TRUE, TRUE, NULL, '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- 3. Reseed: sacar los permisos gruesos y agregar los finos, por rol.

--    super_admin: users read+write, tenants read+write -> view + manage de ambos.
UPDATE roles
SET permissions = (permissions - 'perm_users' - 'perm_tenants')
                   || '["perm_users_view","perm_users_manage","perm_tenants_view","perm_tenants_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'super_admin';

--    tenant_manager: users read-only, tenants read-only -> solo view de ambos.
UPDATE roles
SET permissions = (permissions - 'perm_users')
                   || '["perm_users_view","perm_tenants_view"]'::jsonb,
    updated_at = NOW()
WHERE id = 'tenant_manager';

--    admin: users read+write -> view+manage; tenants read-only -> view SIN manage.
--    Este es el fix real de B-004: hoy perm_tenants sin distinción le daba
--    acceso visual a Tenants aunque el backend rechace la escritura con 403.
UPDATE roles
SET permissions = (permissions - 'perm_users')
                   || '["perm_users_view","perm_users_manage","perm_tenants_view"]'::jsonb,
    updated_at = NOW()
WHERE id = 'admin';

--    platform_admin (fila nueva, arranca en '[]'): mismo set que admin, pero
--    con tenants manage completo — la única diferencia real vs. admin.
UPDATE roles
SET permissions = '["perm_dashboard","perm_alerts","perm_reports","perm_reports_view","perm_reports_manage","perm_users_view","perm_users_manage","perm_tenants_view","perm_tenants_manage","perm_settings","perm_maintenance","perm_analytics","perm_edge_devices_view","perm_edge_devices_manage","perm_logs_view"]'::jsonb,
    updated_at = NOW()
WHERE id = 'platform_admin';

--    cliente_admin: users read+write -> view+manage. Nunca tuvo tenants.
UPDATE roles
SET permissions = (permissions - 'perm_users')
                   || '["perm_users_view","perm_users_manage"]'::jsonb,
    updated_at = NOW()
WHERE id = 'cliente_admin';

-- operario y cliente_operario no tenían perm_users ni perm_tenants -> sin cambios.

-- 4. Catálogo: borrar los permisos gruesos ya reemplazados. Sin FKs entrantes
--    (roles.permissions es JSONB, no relacional) — mismo patrón que 000008.
DELETE FROM permissions WHERE id IN ('perm_users', 'perm_tenants');
```

- [ ] **Step 2: Escribir la migración down**

`migrations/000011_dynamic_role_permissions.down.sql`:

```sql
-- Revierte 000011: vuelve a perm_users/perm_tenants sin distinción de acción,
-- borra platform_admin y el catálogo _view/_manage nuevo.

INSERT INTO permissions (id, name, section, description, is_system_permission, tenant_id) VALUES
    ('perm_users',   'Gestionar Usuarios', 'users',   'Crear, editar y eliminar usuarios', TRUE, NULL),
    ('perm_tenants', 'Gestionar Tenants',  'tenants', 'Acceso a la gestión de tenants',    TRUE, NULL)
ON CONFLICT (id) DO NOTHING;

-- Si el rol tenía _view (siempre presente cuando había _manage, ver up.sql),
-- se le restaura el permiso grueso equivalente.
UPDATE roles
SET permissions = (permissions - 'perm_users_view' - 'perm_users_manage' - 'perm_tenants_view' - 'perm_tenants_manage')
                   || (CASE WHEN permissions @> '["perm_users_view"]'::jsonb THEN '["perm_users"]'::jsonb ELSE '[]'::jsonb END)
                   || (CASE WHEN permissions @> '["perm_tenants_view"]'::jsonb THEN '["perm_tenants"]'::jsonb ELSE '[]'::jsonb END),
    updated_at = NOW()
WHERE id != 'platform_admin';

DELETE FROM roles WHERE id = 'platform_admin';

DELETE FROM permissions WHERE id IN ('perm_users_view', 'perm_users_manage', 'perm_tenants_view', 'perm_tenants_manage');
```

- [ ] **Step 3: Aplicar la migración contra la DB de test**

```bash
migrate -path migrations/ -database "$DATABASE_URL" up
```

Expected: sale `000011` sin error, `migrate -path migrations/ -database "$DATABASE_URL" version` reporta `11, dirty=false`.

- [ ] **Step 4: Escribir el test de integración que verifica el reseed**

`internal/repo/pg/roles/seed_test.go` (nuevo archivo, mismo patrón `openPool`/skip-si-no-hay-DATABASE_URL que `repository_test.go`):

```go
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
// es la traducción 1:1 del mapa Go rolePermissions (rbac.go) que existía antes
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
		{"tenant_manager", true, []string{"perm_users_view"}, []string{"perm_tenants_view"}},
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
```

- [ ] **Step 5: Correr el test para verificar que pasa**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL="$DATABASE_URL" \
  golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/roles/... -run TestSeedPermissions -v"
```

Expected: `TestSeedPermissionsMatchDesign` (7 subtests) y `TestSeedPermissionsCatalogCleanedUp` en PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/000011_dynamic_role_permissions.up.sql migrations/000011_dynamic_role_permissions.down.sql internal/repo/pg/roles/seed_test.go
git commit -m "feat: seed perm_users/tenants view+manage catalog and platform_admin role row"
```

---

### Task 2: `security.RoleContext` — reemplazo del mapa hardcodeado

**Files:**
- Modify: `internal/security/rbac.go`
- Modify: `internal/security/rbac_test.go`
- Modify: `internal/api/handler/tenants/get_all_tenants/get_all_tenants.go:29-30,33`
- Modify: `internal/api/handler/tenants/get_tenant/get_tenant.go:32-33`
- Modify: `internal/api/handler/tenants/delete_tenant/delete_tenant.go:32-33`
- Modify: `internal/api/handler/tenants/update_tenant/update_tenant.go:52-53`
- Modify: `internal/api/handler/user_roles/get_user_roles/get_user_roles.go:50`
- Modify: `internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go`, `get_tenant/get_tenant_test.go`, `delete_tenant/delete_tenant_test.go`, `update_tenant/update_tenant_test.go` — las 4 comparten un helper `withActorContext` que arma el contexto de test con `security.WithRole`; con `IsGlobal` fuera de ese wrapper, sus casos con rol `"super_admin"` dejan de comportarse cross-tenant si no se actualiza.

**Interfaces:**
- Consumes: nada nuevo de Task 1 todavía (este task es un refactor puro de `security`, testeable con contextos construidos a mano — Task 3 es quien conecta la DB real).
- Produces:
  - `type RoleContext struct { Name string; Permissions []string; IsGlobal bool }`
  - `func WithRoleContext(ctx context.Context, rc RoleContext) context.Context`
  - `func RoleContextFromContext(ctx context.Context) RoleContext`
  - `func WithRole(ctx context.Context, roleName string) context.Context` (wrapper: `WithRoleContext(ctx, RoleContext{Name: roleName})`)
  - `func RoleFromContext(ctx context.Context) string` (wrapper: `RoleContextFromContext(ctx).Name`)
  - `func IsCrossTenantRole(ctx context.Context) bool` — **firma cambiada**, antes tomaba `roleName string`
  - `func Can(ctx context.Context, perm string) error` — misma firma, nueva implementación
  - Se borran: `rolePermissions`, `crossTenantRoles`, `PermissionsForRole`
  - Task 4 (middleware) y Task 5 (me_usecase) consumen `WithRoleContext`/`RoleContext` directamente.

- [ ] **Step 1: Reescribir `internal/security/rbac.go`**

Reemplazar el archivo completo por:

```go
package security

import (
	"context"
	"fmt"
	"slices"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Role represents a named role.
type Role string

// Permission uses the perm_* catalog ids from the `permissions` table
// (e.g., "perm_users_view"). Antes de esta versión el enforcement usaba un
// vocabulario "resource:action" propio (users:read, tenants:write) definido
// en un mapa Go hardcodeado; ahora es el mismo catálogo perm_* que ya
// consumía el frontend (GET /me), cargado desde roles.permissions.
type Permission string

// RoleContext agrupa todo lo que TenantFromHeader resuelve una vez por
// request sobre el rol efectivo del caller: su nombre, el catálogo perm_*
// que tiene asignado (roles.permissions) y si puede actuar cross-tenant
// (roles.is_global). Can() e IsCrossTenantRole() leen de acá — ningún mapa
// hardcodeado queda en este archivo.
type RoleContext struct {
	Name        string
	Permissions []string
	IsGlobal    bool
}

// platformTenantAdminRole es el rol efectivo que toma un `admin` cuya membresía
// pertenece al tenant plataforma de MRG: mismos permisos que admin más
// tenants:write. Existe como fila real en `roles` desde la migración 000011
// (antes era 100% virtual, derivado solo en runtime).
const platformTenantAdminRole = "platform_admin"

// EffectiveRole traduce el role_id almacenado en user_tenant_roles al rol con el
// que el usuario actúa realmente. Única definición de la regla: la consumen
// TenantFromHeader (para el enforcement) y GetMe (para lo que ve el frontend).
// Si divergieran, el frontend mostraría capacidades que el backend niega.
func EffectiveRole(roleID string, isPlatformTenant bool) string {
	if roleID == "admin" && isPlatformTenant {
		return platformTenantAdminRole
	}
	return roleID
}

// CanSeePlatformInternals reporta si el caller puede ver las internas de
// plataforma: roles globales (super_admin, tenant_manager), sus miembros y las
// invitaciones a esos roles. Solo super_admin — ni tenant_manager ni
// platform_admin, que pertenecen a la misma capa pero no la administran.
//
// Fail-closed: sin rol en contexto devuelve false.
func CanSeePlatformInternals(ctx context.Context) bool {
	return RoleFromContext(ctx) == "super_admin"
}

// roleContextKeyType is an unexported type to store RoleContext in context.
type roleContextKeyType struct{}

var roleContextKey = roleContextKeyType{}

// WithRoleContext stores the caller's resolved role (name, permissions,
// is_global) in context. Called once per request by
// middleware.TenantFromHeader after resolving the effective role.
func WithRoleContext(ctx context.Context, rc RoleContext) context.Context {
	return context.WithValue(ctx, roleContextKey, rc)
}

// RoleContextFromContext extracts the RoleContext. Returns the zero value
// (empty name, nil permissions, IsGlobal=false) if none was set — fail-closed.
func RoleContextFromContext(ctx context.Context) RoleContext {
	if rc, ok := ctx.Value(roleContextKey).(RoleContext); ok {
		return rc
	}
	return RoleContext{}
}

// WithRole is a convenience wrapper for callers that only need the role name
// (tests, CanSeePlatformInternals-style checks) without a real permission
// set. Request code should use WithRoleContext instead.
func WithRole(ctx context.Context, roleName string) context.Context {
	return WithRoleContext(ctx, RoleContext{Name: roleName})
}

// RoleFromContext extracts just the role name, for consumers that don't need
// the permission list (logging, telemetry, CanSeePlatformInternals).
func RoleFromContext(ctx context.Context) string {
	return RoleContextFromContext(ctx).Name
}

// IsCrossTenantRole reports whether the caller in context may act on tenants
// other than its own. Backed by roles.is_global, loaded into context by
// middleware.TenantFromHeader — never call this outside a request that went
// through that middleware (GET /me does not; it computes its own
// cross-tenant flag locally, see usecases/me_usecase.go).
func IsCrossTenantRole(ctx context.Context) bool {
	return RoleContextFromContext(ctx).IsGlobal
}

// Can checks whether the caller in context has the given permission.
// Returns domain.ErrForbidden if the user lacks the permission, or if no
// role is set in context at all (fail-closed).
func Can(ctx context.Context, perm string) error {
	rc := RoleContextFromContext(ctx)
	if rc.Name == "" {
		return domain.ErrForbidden
	}
	if slices.Contains(rc.Permissions, perm) {
		return nil
	}
	return fmt.Errorf("%w: role %q lacks permission %q", domain.ErrForbidden, rc.Name, perm)
}
```

- [ ] **Step 2: Reescribir `internal/security/rbac_test.go`**

```go
package security

import (
	"context"
	"errors"
	"testing"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestIsCrossTenantRole(t *testing.T) {
	cases := []struct {
		name     string
		isGlobal bool
		want     bool
	}{
		{"is_global=true es cross-tenant", true, true},
		{"is_global=false no es cross-tenant", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := WithRoleContext(context.Background(), RoleContext{IsGlobal: c.isGlobal})
			if got := IsCrossTenantRole(ctx); got != c.want {
				t.Errorf("IsCrossTenantRole() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestIsCrossTenantRoleSinContexto(t *testing.T) {
	if IsCrossTenantRole(context.Background()) {
		t.Error("sin RoleContext debe devolver false (fail-closed)")
	}
}

func TestEffectiveRole(t *testing.T) {
	cases := []struct {
		name             string
		roleID           string
		isPlatformTenant bool
		want             string
	}{
		{"admin en tenant plataforma asciende", "admin", true, "platform_admin"},
		{"admin en tenant cliente no asciende", "admin", false, "admin"},
		{"super_admin no cambia en plataforma", "super_admin", true, "super_admin"},
		{"super_admin no cambia fuera de plataforma", "super_admin", false, "super_admin"},
		{"tenant_manager no cambia", "tenant_manager", true, "tenant_manager"},
		{"operario no cambia en plataforma", "operario", true, "operario"},
		{"cliente_admin no cambia en plataforma", "cliente_admin", true, "cliente_admin"},
		{"rol vacío no cambia", "", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EffectiveRole(c.roleID, c.isPlatformTenant); got != c.want {
				t.Errorf("EffectiveRole(%q, %v) = %q, want %q", c.roleID, c.isPlatformTenant, got, c.want)
			}
		})
	}
}

func TestCanSeePlatformInternals(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{"super_admin", true},
		{"tenant_manager", false},
		{"platform_admin", false},
		{"admin", false},
		{"operario", false},
		{"", false},
	}
	for _, c := range cases {
		ctx := WithRole(context.Background(), c.role)
		if got := CanSeePlatformInternals(ctx); got != c.want {
			t.Errorf("CanSeePlatformInternals(role=%q) = %v, want %v", c.role, got, c.want)
		}
	}
}

func TestCanSeePlatformInternalsSinRolEnContexto(t *testing.T) {
	if CanSeePlatformInternals(context.Background()) {
		t.Error("sin rol en contexto debe devolver false (fail-closed)")
	}
}

func TestCan(t *testing.T) {
	ctx := WithRoleContext(context.Background(), RoleContext{
		Name:        "cliente_admin",
		Permissions: []string{"perm_dashboard", "perm_users_view"},
	})
	if err := Can(ctx, "perm_dashboard"); err != nil {
		t.Errorf("Can(perm_dashboard) = %v, want nil", err)
	}
	if err := Can(ctx, "perm_users_manage"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("Can(perm_users_manage) = %v, want ErrForbidden", err)
	}
}

func TestCanSinRolEnContexto(t *testing.T) {
	if err := Can(context.Background(), "perm_dashboard"); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("Can() sin rol = %v, want ErrForbidden (fail-closed)", err)
	}
}
```

- [ ] **Step 3: Correr los tests de `security` para verificar que pasan**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go test ./internal/security/... -v"
```

Expected: todos los tests en PASS, `TestClienteAdminTieneUsersWrite` ya no existe (se borró junto con `PermissionsForRole`).

- [ ] **Step 4: Actualizar los 5 call sites de `IsCrossTenantRole`**

`internal/api/handler/tenants/get_all_tenants/get_all_tenants.go` — reemplazar:

```go
	role := security.RoleFromContext(c.Request.Context())

	var scopeToTenantID *uuid.UUID
	if !security.IsCrossTenantRole(role) {
```

por:

```go
	var scopeToTenantID *uuid.UUID
	if !security.IsCrossTenantRole(c.Request.Context()) {
```

`internal/api/handler/tenants/get_tenant/get_tenant.go`, `internal/api/handler/tenants/delete_tenant/delete_tenant.go`, `internal/api/handler/tenants/update_tenant/update_tenant.go` — las tres tienen el mismo bloque idéntico, reemplazar en cada una:

```go
	role := security.RoleFromContext(c.Request.Context())
	if !security.IsCrossTenantRole(role) && !platform.TenantMatches(c.Request.Context(), id) {
```

por:

```go
	if !security.IsCrossTenantRole(c.Request.Context()) && !platform.TenantMatches(c.Request.Context(), id) {
```

`internal/api/handler/user_roles/get_user_roles/get_user_roles.go` — reemplazar:

```go
		CrossTenant:   security.IsCrossTenantRole(security.RoleFromContext(ctx)),
```

por:

```go
		CrossTenant:   security.IsCrossTenantRole(ctx),
```

- [ ] **Step 5: Arreglar los 4 tests de handlers de tenants que dependían del nombre de rol para simular cross-tenant**

`get_all_tenants_test.go`, `get_tenant_test.go`, `delete_tenant_test.go` y `update_tenant_test.go` definen, cada uno por su cuenta, un helper idéntico:

```go
func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}
```

Antes de este task, `IsCrossTenantRole("super_admin")` funcionaba porque miraba el mapa `crossTenantRoles` por nombre. Ahora mira `RoleContext.IsGlobal`, y `WithRole` solo setea `Name` — sin este fix, los tests que pasan `role="super_admin"` (`TestGetAllTenants_CrossTenantRole_ReturnsFullList`, `TestDeleteTenantHandler_CrossTenantRole_Allowed`, `TestDeleteTenantHandler_InvalidID`, `TestGetTenantHandler_ForeignTenant_CrossTenantRole_Allowed`, y el equivalente en `update_tenant_test.go`) van a fallar porque el handler va a tratarlos como no-cross-tenant.

En cada uno de los 4 archivos, reemplazar:

```go
func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}
```

por:

```go
func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	// Los 4 archivos de test de este paquete solo ejercitan "super_admin" (global)
	// y "admin" (no-global) — misma señal que crossTenantRoles tenía hardcodeada
	// antes de que IsCrossTenantRole pasara a leer RoleContext.IsGlobal.
	ctx := security.WithRoleContext(req.Context(), security.RoleContext{Name: role, IsGlobal: role == "super_admin"})
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}
```

- [ ] **Step 6: Correr los 4 paquetes de test de handlers de tenants para verificar que pasan**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go test ./internal/api/handler/tenants/... -v"
```

Expected: todos los tests de `get_all_tenants`, `get_tenant`, `delete_tenant`, `update_tenant` en PASS — en particular los que tienen `CrossTenantRole` en el nombre, que son los que este step arregla.

- [ ] **Step 7: Compilar todo el módulo para verificar que no queda ningún call site roto**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./... && go vet ./..."
```

Expected: sin errores. Si `go vet` marca `security` como import no usado en algún archivo de handler tras sacar `security.RoleFromContext(...)`, confirmar que `security.IsCrossTenantRole` sigue usándose en ese mismo archivo (debería, no hace falta sacar el import).

- [ ] **Step 8: Commit**

```bash
git add internal/security/rbac.go internal/security/rbac_test.go \
  internal/api/handler/tenants/get_all_tenants/get_all_tenants.go \
  internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go \
  internal/api/handler/tenants/get_tenant/get_tenant.go \
  internal/api/handler/tenants/get_tenant/get_tenant_test.go \
  internal/api/handler/tenants/delete_tenant/delete_tenant.go \
  internal/api/handler/tenants/delete_tenant/delete_tenant_test.go \
  internal/api/handler/tenants/update_tenant/update_tenant.go \
  internal/api/handler/tenants/update_tenant/update_tenant_test.go \
  internal/api/handler/user_roles/get_user_roles/get_user_roles.go
git commit -m "refactor: security.Can/IsCrossTenantRole read from RoleContext, drop hardcoded maps"
```

---

### Task 3: `RBACCheck` call sites — migrar del vocabulario `resource:action` a `perm_*`

**Files:**
- Modify: `internal/api/router.go` (16 call sites)
- Modify: `internal/routes/url_mappings.go` (7 call sites)
- Modify: `internal/api/handler/logs/routes.go` (1 call site)

**Interfaces:**
- Consumes: `security.Can(ctx, perm string) error` (Task 2, firma sin cambios — solo cambia qué vocabulario entiende internamente).
- Produces: cada ruta protegida sigue exigiendo exactamente el mismo nivel de acceso que exige hoy, solo que expresado en ids `perm_*` en vez de `resource:action`. Sin esto, Task 2 deja **toda la API sin acceso para nadie** (ver nota crítica de orden arriba) — este task tiene que mergearse en la misma rama que Task 2, antes de cualquier deploy.

**Tabla de mapeo completa** (string viejo → id `perm_*` nuevo):

| Vocabulario viejo | Id `perm_*` nuevo |
|---|---|
| `"users:read"` | `"perm_users_view"` |
| `"users:write"` | `"perm_users_manage"` |
| `"tenants:read"` | `"perm_tenants_view"` |
| `"tenants:write"` | `"perm_tenants_manage"` |
| `"invitations:write"` | `"perm_users_manage"` (spec §2.3: invitar se pliega en gestión de usuarios, no tiene catálogo propio) |
| `"logs:admin"` | `"perm_logs_admin"` (ya existía 1:1 en el catálogo DB, solo cambia el string del call site) |

- [ ] **Step 1: Editar `internal/api/router.go`**

Reemplazar cada ocurrencia (16 en total) siguiendo la tabla de arriba. El bloque completo de `RegisterAdminRoutes` queda así (reemplazar líneas 66-140, la función completa a partir de `func RegisterAdminRoutes`):

```go
	userRoutes := g.Group("")
	userRoutes.Use(middleware.ExtractTenantID())

	// Read operations (no RBAC required — ver DEUDA-TECNICA.md: "RBAC en GET /users")
	// NOTE: /users/pending MUST be registered before /users/:id to avoid Gin treating "pending" as :id
	userRoutes.GET("/users/pending", middleware.RBACCheck("perm_users_view"), uh.ListPendingUsers)
	userRoutes.GET("/users", uh.ListUsers)
	userRoutes.GET("/users/:id", uh.GetUser)
	// RBACCheck("perm_users_view"): esta ruta se había registrado sin ningún chequeo, así
	// que cualquier usuario autenticado (un operario) podía pedir los roles y los
	// tenants de cualquier user_id. Es el mismo permiso que GET /user-roles porque es
	// la misma información indexada de otra forma. El scoping por tenant y el cloaking
	// los aplica el handler (ver get_user_roles).
	userRoutes.GET("/users/:id/roles", middleware.RBACCheck("perm_users_view"), getUserRolesHandler.Handle)

	// Write operations (admin only)
	userRoutes.POST("/users", middleware.RBACCheck("perm_users_manage"), uh.CreateUser)
	// Self-service: sin RBAC, el userID sale del JWT (uh.UpdateMe), nunca de la URL.
	// Registrada antes de "/users/:id" — mismo motivo que /users/pending arriba.
	userRoutes.PATCH("/users/me", uh.UpdateMe)
	userRoutes.PATCH("/users/:id", middleware.RBACCheck("perm_users_manage"), uh.UpdateUser)
	userRoutes.PATCH("/users/:id/status", middleware.RBACCheck("perm_users_manage"), uh.UpdateUserStatus)
	userRoutes.DELETE("/users/:id", middleware.RBACCheck("perm_users_manage"), uh.DeleteUser)

	// Machines
	g.GET("/machines", ListMachines)
	g.POST("/machines", CreateMachine)

	// Tenants
	getAllTenantsUseCase := ucGetAllTenants.NewUseCase(deps.TenantRepo)
	getTenantUseCase := ucGetTenant.NewUseCase(deps.TenantRepo)
	createTenantUseCase := ucCreateTenant.NewUseCase(deps.TenantRepo)
	updateTenantUseCase := ucUpdateTenant.NewUseCase(deps.TenantRepo)
	deleteTenantUseCase := ucDeleteTenant.NewUseCase(deps.TenantRepo)

	getAllTenantsHandler := getAllTenants.NewGetAllTenantsHandler(getAllTenantsUseCase)
	getTenantHandler := getTenant.NewGetTenantHandler(getTenantUseCase)
	createTenantHandler := createTenant.NewCreateTenantHandler(createTenantUseCase)
	updateTenantHandler := updateTenant.NewUpdateTenantHandler(updateTenantUseCase)
	deleteTenantHandler := deleteTenant.NewDeleteTenantHandler(deleteTenantUseCase)

	g.GET("/tenants", middleware.RBACCheck("perm_tenants_view"), getAllTenantsHandler.GetAllTenants)
	g.POST("/tenants", middleware.RBACCheck("perm_tenants_manage"), createTenantHandler.CreateTenant)
	g.GET("/tenants/:tenantId", middleware.RBACCheck("perm_tenants_view"), getTenantHandler.GetTenant)
	g.PATCH("/tenants/:tenantId", middleware.RBACCheck("perm_tenants_manage"), updateTenantHandler.UpdateTenant)
	g.DELETE("/tenants/:tenantId", middleware.RBACCheck("perm_tenants_manage"), deleteTenantHandler.DeleteTenant)

	// User Roles
	assignUserRoleUseCase := ucAssignUserRole.NewUseCase(deps.UserRoleRepo, deps.RoleRepo)
	assignUserRoleHandler := assignUserRole.NewAssignUserRoleHandler(assignUserRoleUseCase)
	g.POST("/user-roles", middleware.RBACCheck("perm_users_manage"), assignUserRoleHandler.Handle)

	listUserRolesUseCase := ucListUserRoles.NewUseCase(deps.UserRoleRepo)
	listUserRolesHandler := listUserRoles.NewListUserRolesHandler(listUserRolesUseCase)
	g.GET("/user-roles", middleware.RBACCheck("perm_users_view"), listUserRolesHandler.Handle)

	bulkAssignUserRoleUseCase := ucBulkAssignUserRole.NewUseCase(deps.UserRoleRepo, deps.RoleRepo)
	bulkAssignUserRoleHandler := bulkAssignUserRole.NewBulkAssignUserRolesHandler(bulkAssignUserRoleUseCase)
	g.POST("/user-roles/bulk", middleware.RBACCheck("perm_users_manage"), bulkAssignUserRoleHandler.Handle)

	updateUserRoleUseCase := ucUpdateUserRole.NewUseCase(deps.UserRoleRepo, deps.RoleRepo)
	updateUserRoleHandler := updateUserRole.NewUpdateUserRoleHandler(updateUserRoleUseCase)
	g.PUT("/user-roles/:id", middleware.RBACCheck("perm_users_manage"), updateUserRoleHandler.Handle)

	revokeUserRoleUseCase := ucRevokeUserRole.NewUseCase(deps.UserRoleRepo)
	revokeUserRoleHandler := revokeUserRole.NewRevokeUserRoleHandler(revokeUserRoleUseCase)
	g.DELETE("/user-roles/:id", middleware.RBACCheck("perm_users_manage"), revokeUserRoleHandler.Handle)
}
```

(Solo cambiaron los strings pasados a `RBACCheck(...)` y el comentario de la línea 76; el resto de la función — imports, wiring de usecases/handlers — queda idéntico.)

- [ ] **Step 2: Editar `internal/routes/url_mappings.go`**

Reemplazar (líneas 145-152):

```go
	// Invitations
	v1.GET("/invitations", listInvHandler.Handle)
	v1.POST("/invitations", apimw.RBACCheck("invitations:write"), createInvHandler.Handle)
	v1.POST("/invitations/:id/resend", apimw.RBACCheck("invitations:write"), resendInvHandler.Handle)
	v1.DELETE("/invitations/:id", apimw.RBACCheck("invitations:write"), revokeInvHandler.Handle)

	// Force password change
	v1.POST("/users/:id/force-password-change", apimw.RBACCheck("users:write"), forcePasswordHandler.Handle)
```

por:

```go
	// Invitations
	v1.GET("/invitations", listInvHandler.Handle)
	v1.POST("/invitations", apimw.RBACCheck("perm_users_manage"), createInvHandler.Handle)
	v1.POST("/invitations/:id/resend", apimw.RBACCheck("perm_users_manage"), resendInvHandler.Handle)
	v1.DELETE("/invitations/:id", apimw.RBACCheck("perm_users_manage"), revokeInvHandler.Handle)

	// Force password change
	v1.POST("/users/:id/force-password-change", apimw.RBACCheck("perm_users_manage"), forcePasswordHandler.Handle)
```

Reemplazar (líneas 196-200):

```go
	// Roles surface (/api/v1/roles)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado puede listar/ver roles)
	// POST/PUT/DELETE: requieren permiso users:write (solo administradores)
	rService := rolesApp.NewService(rRepo, logger)
	rolesWriteGroup := v1.Group("", apimw.RBACCheck("users:write"))
```

por:

```go
	// Roles surface (/api/v1/roles)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado puede listar/ver roles)
	// POST/PUT/DELETE: requieren perm_users_manage (solo administradores)
	rService := rolesApp.NewService(rRepo, logger)
	rolesWriteGroup := v1.Group("", apimw.RBACCheck("perm_users_manage"))
```

Reemplazar (líneas 203-208):

```go
	// Alarm Rules surface (/api/v1/alarm-rules)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado del tenant puede listar/ver reglas)
	// POST/PATCH/DELETE: requieren permiso users:write (solo administradores)
	arRepo := alarmRulesRepo.NewPostgresRepository(db)
	arService := alarmRulesApp.NewService(arRepo, logger)
	alarmRulesWriteGroup := v1.Group("", apimw.RBACCheck("users:write"))
```

por:

```go
	// Alarm Rules surface (/api/v1/alarm-rules)
	// GET endpoints: sin RBAC adicional (cualquier usuario autenticado del tenant puede listar/ver reglas)
	// POST/PATCH/DELETE: requieren perm_users_manage (solo administradores)
	arRepo := alarmRulesRepo.NewPostgresRepository(db)
	arService := alarmRulesApp.NewService(arRepo, logger)
	alarmRulesWriteGroup := v1.Group("", apimw.RBACCheck("perm_users_manage"))
```

Reemplazar (líneas 222-228):

```go
	// Permissions Service (/api/v1/permissions)
	// GET /permissions y GET /permissions/:id — sin RBAC adicional (cualquier usuario autenticado puede consultar)
	// POST/PUT/DELETE — requieren permiso users:write (solo administradores)
	pRepo := permissionsRepo.NewPostgresRepository(db)
	pService := permissionsApp.NewService(pRepo, logger)
	pHandler := permissionsHandler.NewHandler(pService, logger)
	permissionsWriteGroup := v1.Group("", apimw.RBACCheck("users:write"))
```

por:

```go
	// Permissions Service (/api/v1/permissions)
	// GET /permissions y GET /permissions/:id — sin RBAC adicional (cualquier usuario autenticado puede consultar)
	// POST/PUT/DELETE — requieren perm_users_manage (solo administradores)
	pRepo := permissionsRepo.NewPostgresRepository(db)
	pService := permissionsApp.NewService(pRepo, logger)
	pHandler := permissionsHandler.NewHandler(pService, logger)
	permissionsWriteGroup := v1.Group("", apimw.RBACCheck("perm_users_manage"))
```

- [ ] **Step 3: Editar `internal/api/handler/logs/routes.go`**

Reemplazar:

```go
	rg.PATCH("/logs/retention", apimw.RBACCheck("logs:admin"), UpdateRetention(svc))
```

por:

```go
	rg.PATCH("/logs/retention", apimw.RBACCheck("perm_logs_admin"), UpdateRetention(svc))
```

- [ ] **Step 4: Grep de verificación — no debe quedar ningún string viejo**

```bash
grep -rn 'RBACCheck("users:\|RBACCheck("tenants:\|RBACCheck("invitations:\|RBACCheck("logs:admin"' internal --include="*.go"
```

Expected: sin resultados. Cualquier línea que aparezca acá quedó sin migrar.

- [ ] **Step 5: Compilar para verificar**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./... && go vet ./..."
```

Expected: build limpio.

- [ ] **Step 6: Commit**

```bash
git add internal/api/router.go internal/routes/url_mappings.go internal/api/handler/logs/routes.go
git commit -m "refactor: migrate RBACCheck call sites from resource:action strings to perm_* catalog ids"
```

---

### Task 4: `TenantFromHeader` — cargar permisos reales por request

**Files:**
- Modify: `internal/api/middleware/middleware.go`

**Interfaces:**
- Consumes: `security.RoleContext`, `security.WithRoleContext`, `security.EffectiveRole` (Task 2). Requiere la migración 000011 aplicada (Task 1) para que el `SELECT` no falle con "role not found".
- Produces: todo request autenticado con `X-Tenant-ID` válido llega a los handlers con `security.RoleContextFromContext(ctx)` poblado — de esto dependen `Can()` (usado por todo `RBACCheck`) y los 5 call sites de `IsCrossTenantRole` actualizados en Task 2.

- [ ] **Step 1: Agregar el helper `loadRolePermissions`**

En `internal/api/middleware/middleware.go`, agregar cerca de `resolvePlatformOperator` (después de su cierre, línea ~253 aprox.):

```go
// loadRolePermissions fetches the DB-backed permission catalog (perm_*) and
// cross-tenant flag for a resolved role id (already passed through
// security.EffectiveRole). Called once per request by TenantFromHeader.
func loadRolePermissions(ctx context.Context, db *pgxpool.Pool, roleID string) ([]string, bool, error) {
	var permissions []string
	var isGlobal bool
	err := db.QueryRow(ctx,
		`SELECT ARRAY(SELECT jsonb_array_elements_text(permissions)), is_global
		 FROM roles
		 WHERE id = $1 AND deleted_at IS NULL`,
		roleID,
	).Scan(&permissions, &isGlobal)
	if err != nil {
		return nil, false, fmt.Errorf("role %q not found in roles table: %w", roleID, err)
	}
	return permissions, isGlobal, nil
}
```

- [ ] **Step 2: Conectar el helper en `TenantFromHeader`**

Reemplazar (líneas ~193-200 de `middleware.go`):

```go
		} else {
			roleID = security.EffectiveRole(roleID, isPlatformTenant)
		}

		ctx := platform.WithTenantID(c.Request.Context(), tenantID)
		ctx = security.WithRole(ctx, roleID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
```

por:

```go
		} else {
			roleID = security.EffectiveRole(roleID, isPlatformTenant)
		}

		permissions, isGlobal, err := loadRolePermissions(c.Request.Context(), db, roleID)
		if err != nil {
			Log.Error("failed to load role permissions",
				zap.String("role_id", roleID),
				zap.String("user_id", user.ID),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "internal error"})
			return
		}

		ctx := platform.WithTenantID(c.Request.Context(), tenantID)
		ctx = security.WithRoleContext(ctx, security.RoleContext{
			Name:        roleID,
			Permissions: permissions,
			IsGlobal:    isGlobal,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
```

- [ ] **Step 3: Compilar para verificar que `fmt` ya está importado**

`middleware.go` ya importa `"fmt"` (lo usa `resolvePlatformOperator`), así que no hace falta tocar el bloque de imports.

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."
```

Expected: build limpio.

- [ ] **Step 4: Test de integración — un rol real resuelve los permisos correctos**

Si existe ya un archivo de tests de integración para middleware, agregar ahí; si no, crear `internal/api/middleware/tenant_from_header_test.go` con el patrón `openPool`/skip-si-no-hay-DATABASE_URL (mismo que `roles/repository_test.go`). Como `TenantFromHeader` depende de `gin.Context` y de filas reales en `user_tenant_roles`, el camino más simple y realista es testear `loadRolePermissions` directamente (es la pieza nueva; el resto de `TenantFromHeader` no cambió):

```go
package middleware

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestLoadRolePermissions(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	perms, isGlobal, err := loadRolePermissions(context.Background(), pool, "cliente_admin")
	require.NoError(t, err)
	require.False(t, isGlobal)
	require.Contains(t, perms, "perm_users_manage")
	require.NotContains(t, perms, "perm_users")
}

func TestLoadRolePermissionsRolInexistente(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	_, _, err = loadRolePermissions(context.Background(), pool, "rol_que_no_existe")
	require.Error(t, err)
}
```

- [ ] **Step 5: Correr el test para verificar que pasa**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL="$DATABASE_URL" \
  golang:1.24-alpine \
  sh -c "go test ./internal/api/middleware/... -run TestLoadRolePermissions -v"
```

Expected: ambos tests en PASS (requiere que la migración 000011 de Task 1 ya esté aplicada contra `$DATABASE_URL`).

- [ ] **Step 6: Commit**

```bash
git add internal/api/middleware/middleware.go internal/api/middleware/tenant_from_header_test.go
git commit -m "feat: TenantFromHeader loads real permissions+is_global from roles table"
```

---

### Task 5: `GetMe` — no divergir del rol efectivo

**Files:**
- Modify: `internal/api/usecases/me_usecase.go`
- Modify: `internal/api/usecases/me_usecase_test.go` (ya existe, con fixtures reales — `seedMember`, `platformTenantID` — dos de sus tests actuales quedan rotos por el reseed de la migración 000011 y hay que actualizarlos, no solo agregar uno nuevo)

**Interfaces:**
- Consumes: `security.EffectiveRole` (sin cambios). No usa `security.IsCrossTenantRole` (esa función ahora es context-based y `/me` no pasa por `TenantFromHeader` — este usecase calcula `CanCrossTenant` con su propio dato local, ver Step 1).
- Produces: `MeResponse.Permissions` y `MeResponse.Capabilities.CanCrossTenant` reflejan el rol efectivo, no el rol crudo de `user_tenant_roles`.

- [ ] **Step 1: Reescribir `GetMe`**

Reemplazar el bloque completo de la query y el `if err == nil` (líneas ~82-125 de `me_usecase.go`):

```go
	// Query the user's active tenant+role. `roles.permissions` es la fuente de
	// verdad del catálogo perm_* que consume el frontend — mismo catálogo que
	// usa el enforcement (security.Can) desde la migración 000011.
	var tenantID, tenantName, tenantSubdomain, roleID, roleName string
	var isPlatformTenant, roleIsGlobal bool
	var permissions []string
	err := uc.db.QueryRow(ctx, `
		SELECT t.id, t.name, t.subdomain, t.is_platform_tenant,
		       r.id, r.name, r.is_global,
		       ARRAY(SELECT jsonb_array_elements_text(r.permissions))
		FROM user_tenant_roles utr
		JOIN tenants t ON t.id = utr.tenant_id
		JOIN roles r ON r.id = utr.role_id
		WHERE utr.user_id = $1 AND utr.status = 'active'
		LIMIT 1
	`, user.ID).Scan(&tenantID, &tenantName, &tenantSubdomain, &isPlatformTenant,
		&roleID, &roleName, &roleIsGlobal, &permissions)

	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if err == nil {
		effectiveRole := security.EffectiveRole(roleID, isPlatformTenant)

		if permissions == nil {
			permissions = []string{}
		}

		// El JOIN de arriba trae permisos/is_global del role_id crudo asignado
		// en user_tenant_roles. Si EffectiveRole promovió el rol (admin ->
		// platform_admin dentro del tenant plataforma), esos valores no son
		// los correctos: hay que resolver los del rol efectivo. Mismo criterio
		// que middleware.TenantFromHeader (internal/api/middleware/middleware.go).
		if effectiveRole != roleID {
			permErr := uc.db.QueryRow(ctx,
				`SELECT ARRAY(SELECT jsonb_array_elements_text(permissions)), is_global
				 FROM roles WHERE id = $1 AND deleted_at IS NULL`,
				effectiveRole,
			).Scan(&permissions, &roleIsGlobal)
			if permErr != nil {
				return nil, permErr
			}
		}

		resp.Tenant = &TenantInfoResponse{
			ID:         tenantID,
			Name:       tenantName,
			Subdomain:  tenantSubdomain,
			IsPlatform: isPlatformTenant,
		}
		resp.Role = &RoleInfoResponse{
			ID:   effectiveRole,
			Name: roleName,
		}
		resp.Permissions = permissions
		resp.Capabilities = CapabilitiesResponse{
			CanCrossTenant: roleIsGlobal,
		}
	}

	return resp, nil
}
```

- [ ] **Step 2: Actualizar el comentario de `CapabilitiesResponse.CanCrossTenant`**

Reemplazar (líneas ~49-50):

```go
	// CanCrossTenant: el usuario puede ver y operar datos de tenants distintos
	// al suyo. Deriva de security.IsCrossTenantRole sobre el rol efectivo.
```

por:

```go
	// CanCrossTenant: el usuario puede ver y operar datos de tenants distintos
	// al suyo. Viene de roles.is_global del rol efectivo — GetMe lo resuelve
	// localmente (no vía security.IsCrossTenantRole, que es context-based y
	// GET /me no pasa por TenantFromHeader).
```

- [ ] **Step 3: Compilar para verificar**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./... && go vet ./..."
```

Expected: build limpio.

- [ ] **Step 4: Actualizar las aserciones de `me_usecase_test.go` que quedan rotas por el reseed de la migración 000011**

`internal/api/usecases/me_usecase_test.go` ya existe con fixtures reales (`seedMember`, `platformTenantID`, tenant plataforma MRG sembrado por la 000002) — no hace falta crear nada nuevo, dos de sus tres tests actuales asertan ids que la migración 000011 (Task 1) borró del catálogo.

En `TestGetMeAdminDePlataformaEsPlatformAdmin`, reemplazar:

```go
	require.Contains(t, resp.Permissions, "perm_tenants", "los permisos deben venir en vocabulario perm_*")
	require.Contains(t, resp.Permissions, "perm_logs_view")
```

por:

```go
	// perm_tenants_manage, NO perm_tenants_view solo: es la aserción que hace
	// de este test la regresión real del fix de este task. admin (el rol
	// crudo asignado en el seed) solo tiene perm_tenants_view — si GetMe
	// devolviera los permisos del JOIN sin resolver el rol efectivo, este
	// require fallaría, porque platform_admin es quien tiene manage completo.
	require.Contains(t, resp.Permissions, "perm_tenants_manage", "platform_admin debe tener gestión completa de tenants, no solo view")
	require.Contains(t, resp.Permissions, "perm_logs_view")
```

En `TestGetMeSuperAdminConservaSuRol`, reemplazar:

```go
	require.Contains(t, resp.Permissions, "perm_users")
```

por:

```go
	require.Contains(t, resp.Permissions, "perm_users_view")
	require.Contains(t, resp.Permissions, "perm_users_manage")
```

`TestGetMeClienteAdminDeTenantClienteNoAsciende` no asserta ningún id de permiso específico — no necesita cambios.

- [ ] **Step 5: Correr los tests para verificar que pasan**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL="$DATABASE_URL" \
  golang:1.24-alpine \
  sh -c "go test ./internal/api/usecases/... -run TestGetMe -v"
```

Expected: los 3 tests en PASS (requiere la migración 000011 de Task 1 ya aplicada contra `$DATABASE_URL`). Si `TestGetMeAdminDePlataformaEsPlatformAdmin` falla en `perm_tenants_manage`, es señal de que el Step 1 de este task (el re-fetch cuando `effectiveRole != roleID`) tiene un bug — no avanzar sin que este test pase de verdad.

- [ ] **Step 6: Commit**

```bash
git add internal/api/usecases/me_usecase.go internal/api/usecases/me_usecase_test.go
git commit -m "fix: GetMe resolves permissions/is_global for the effective role, not the raw one"
```

---

### Task 6: Verificación end-to-end — B-004 cerrado de verdad

**Files:**
- Test: `internal/security/e2e_permissions_test.go` (nuevo)

**Interfaces:**
- Consumes: todo lo anterior (Tasks 1-4) ya aplicado contra una DB de test real.
- Produces: el criterio de cierre de B-004 automatizado — un rol custom con un subconjunto de permisos efectivamente limita lo que `Can()` autoriza, sin pasar por ningún mapa Go.

- [ ] **Step 1: Escribir el test end-to-end**

`internal/security/e2e_permissions_test.go`:

```go
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
	defer pool.Close()

	tenantID := uuid.New()
	_, err = pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name, company_name, subdomain, is_platform_tenant)
		 VALUES ($1, 'B-004 E2E Test', 'B-004 E2E Test', $2, FALSE)`,
		tenantID, "b004-e2e-"+tenantID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	roleID := "custom_e2e_test"
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
```

- [ ] **Step 2: Correr el test para verificar que pasa**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL="$DATABASE_URL" \
  golang:1.24-alpine \
  sh -c "go test ./internal/security/... -run TestCustomRolePermissionsAreEnforced -v"
```

Expected: PASS. Este es el test que **no se podía escribir antes de este plan** — es la prueba automatizada de que B-004 está cerrado de verdad.

- [ ] **Step 3: Correr la suite completa del módulo antes de dar por terminada la rama**

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v /tmp/go-build-cache:/root/.cache/go-build \
  -v $(pwd):/app -w /app \
  -e DATABASE_URL="$DATABASE_URL" \
  golang:1.24-alpine \
  sh -c "go build ./... && go vet ./... && go test ./..."
```

Expected: todo en PASS, sin warnings de `go vet`.

- [ ] **Step 4: Commit**

```bash
git add internal/security/e2e_permissions_test.go
git commit -m "test: add end-to-end coverage that custom role permissions are actually enforced (closes B-004)"
```

---

## Después de este plan

- Aplicar la migración 000011 contra la DB real de producción (`migrate ... up`, paso manual — `RunMigrationsOnBoot` está apagado en Cloud Run, confirmado en `handoff-2026-08-13-uat-rbac-followups.md`).
- Verificar el deploy con `curl` real, mismo criterio que B-005: pedir `GET /api/v1/me` con un usuario `admin` de un tenant cliente y confirmar que `permissions` ya no incluye `perm_tenants` sino `perm_tenants_view`.
- El plan de frontend (`docs/superpowers/plans/2026-08-17-rbac-dynamic-permissions-frontend.md` en `embolsadora-frontend`) depende de que este backend ya esté deployado en producción — mismo orden que B-005.
