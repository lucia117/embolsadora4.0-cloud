# Production Readiness Cleanup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the 15 pending items consolidated in the design spec (`docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md`) so the MVP is production-ready: 2 backend cross-tenant bugs (extended to 2 more call sites per user's approval), 1 missing RBAC gate, 1 data edge case fix, 1 verification test, a frontend RBAC/UI cleanup batch, and 2 investigation tasks with open-ended outcomes.

**Architecture:** No new subsystems. Backend fixes mirror the existing "Hallazgo A" cross-tenant pattern (thread `crossTenant bool` from `security.IsCrossTenantRole(ctx)` through handler → service → repo). Frontend fixes wire an already-existing but under-used hook (`useCanPerformAction`) into 3 more surfaces and delete confirmed-dead code.

**Tech Stack:** Go 1.24 (Gin, pgx/v5) for backend tasks; TypeScript/Next.js 16 (React Hook Form, Zod) for frontend tasks.

## Global Constraints

- Backend tests are DB-integration tests gated by `DATABASE_URL` (`poolOrSkip(t)` calls `t.Skip` if unset) — run them via Docker per the backend `CLAUDE.md`: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL=<dsn> golang:1.24-alpine sh -c "go test ./... -run <TestName> -v"`. The DSN lives in `~/.supabase-db-url` on this machine.
- **The frontend repo has no test runner** (`package.json`: `"test": "echo \"Tests not implemented yet\" && exit 0"`) — there is no Jest/RTL config to write real unit tests against. Frontend tasks substitute `pnpm tsc --noEmit && pnpm lint && pnpm build` (the repo's own documented "pre-push validation") as the automated gate, plus a manual browser check for the 2 tasks where behavior can't be confirmed by types alone (button gating, the dead-header fix). Do not introduce a new test framework as part of this plan — that would be its own separate initiative.
- Backend tasks 1–7 happen on branch `fix/production-readiness-backend-batch`, created fresh off `origin/develop` (run `git fetch origin` first — local `develop` drifts, see repo's own memory note). One PR at the end targeting `develop`.
- Frontend tasks 8–11 happen on branch `fix/production-readiness-frontend-batch` in `embolsadora-frontend`, created fresh off `origin/develop`. One PR at the end targeting `develop`.
- Tasks 2 through 6 all touch `internal/app/users/service.go` and/or `internal/repo/pg/users/postgres.go` — **execute them in order, not in parallel**, to avoid merge conflicts within the same task branch.
- Every backend task that changes a `Repository`/`Service` method signature must update **all** existing call sites in the same commit (Go won't compile otherwise) — each task below lists them explicitly.
- Tasks 12 (Hallazgo D investigation) and 13 (Nuevo Tenant form investigation) are open-ended: the "fix" step cannot be fully specified in advance because the root cause isn't known yet. Their steps give the exact commands/actions to run and a decision tree for what to do with the result — this is intentional, not a placeholder (see the design spec's own §B/§F, already approved).

---

### Task 1: RBAC gate for edge-devices routes

**Files:**
- Modify: `internal/api/handler/edge_devices/routes.go`
- Test: `internal/api/handler/edge_devices/routes_rbac_test.go` (create)

**Interfaces:**
- Produces: no new exported symbols — `RegisterRoutes(g *gin.RouterGroup, service *edge_devices.Service)` keeps its signature, only gains middleware internally.

- [ ] **Step 1: Write the failing test**

Create `internal/api/handler/edge_devices/routes_rbac_test.go`:

```go
package edge_devices_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices"
	appEdgeDevices "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// newTestRouterWithRole monta las rutas reales de edge-devices detrás de un
// middleware que inyecta un RoleContext fijo, para poder probar el gate de
// RBAC sin depender de JWT/DB reales. El service se construye con
// repo/client nil a propósito: si una request pasa el gate de permisos,
// el handler puede fallar después (nil pointer) — gin.Recovery() lo
// convierte en 500 en vez de crashear el test, y un 500 sigue probando que
// NO fue un 403 (que es lo único que este test necesita confirmar).
func newTestRouterWithRole(t *testing.T, permissions []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		ctx := security.WithRoleContext(c.Request.Context(), security.RoleContext{
			Name:        "test_role",
			Permissions: permissions,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	service := appEdgeDevices.NewService(nil, nil, zap.NewNop())
	group := r.Group("")
	edge_devices.RegisterRoutes(group, service)
	return r
}

func TestEdgeDevicesGetSinPermisoDa403(t *testing.T) {
	r := newTestRouterWithRole(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/edge-devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "sin perm_edge_devices_view debe dar 403")
}

func TestEdgeDevicesGetConPermisoViewPasaElGate(t *testing.T) {
	r := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodGet, "/edge-devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusForbidden, w.Code, "con perm_edge_devices_view el request debe pasar el gate de RBAC")
}

func TestEdgeDevicesPostCreateSoloConManagePasaElGate(t *testing.T) {
	rView := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w := httptest.NewRecorder()
	rView.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "perm_edge_devices_view no debe alcanzar para POST /edge-devices")

	rManage := newTestRouterWithRole(t, []string{"perm_edge_devices_manage"})
	req2 := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w2 := httptest.NewRecorder()
	rManage.ServeHTTP(w2, req2)
	require.NotEqual(t, http.StatusForbidden, w2.Code, "perm_edge_devices_manage debe pasar el gate de POST /edge-devices")
}

func TestEdgeDevicesStatusCheckRequiereManageNoSoloView(t *testing.T) {
	r := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "status check dispara una acción activa contra el device (pasa userID/userEmail para audit trail), requiere _manage no _view")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run (Docker not needed here — no DB access in this test):
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/handler/edge_devices/... -v"
```
Expected: `TestEdgeDevicesGetSinPermisoDa403` and the other 403-expecting assertions FAIL (current routes have no middleware, so every request reaches the nil-backed handler and panics into a 500 recovered by `gin.Recovery()` — not a 403).

- [ ] **Step 3: Implement the RBAC gate**

Replace `internal/api/handler/edge_devices/routes.go` entirely:

```go
package edge_devices

import (
	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
)

// RegisterRoutes registers all edge device endpoints on the given Gin group.
//
// Hasta esta fix, ningún endpoint acá tenía RBACCheck — cualquier miembro
// autenticado del tenant (incluido operario) podía crear/actualizar/habilitar
// dispositivos. Los permission ids ya existían en el seed (migración 000011),
// solo faltaba cablearlos. Ver docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md §C.
func RegisterRoutes(g *gin.RouterGroup, service *edge_devices.Service) {
	view := middleware.RBACCheck("perm_edge_devices_view")
	manage := middleware.RBACCheck("perm_edge_devices_manage")

	// US1 – List
	g.GET("/edge-devices", view, ListDevices(service))

	// US2 – Create
	g.POST("/edge-devices", manage, CreateDevice(service))

	// US3 – Get
	g.GET("/edge-devices/:deviceId", view, GetDevice(service))

	// US4 – Update
	g.PUT("/edge-devices/:deviceId", manage, UpdateDevice(service))

	// US5 – Enable/Disable
	g.POST("/edge-devices/:deviceId/enable", manage, EnableDevice(service))
	g.POST("/edge-devices/:deviceId/disable", manage, DisableDevice(service))

	// US6 – Status Check: dispara una acción activa contra el device (el
	// handler pasa userID/userEmail al service para audit trail), no es una
	// lectura pasiva — requiere _manage, igual que enable/disable.
	g.POST("/edge-devices/:deviceId/status", manage, StatusCheck(service))

	// US7 – Health Check: mismo criterio que status check.
	g.POST("/edge-devices/:deviceId/health-check", manage, HealthCheck(service))

	// US8 – Telemetry
	g.GET("/edge-devices/:deviceId/telemetry", view, GetTelemetry(service))

	// US9 – Events
	g.GET("/edge-devices/:deviceId/events", view, ListEvents(service))
}
```

- [ ] **Step 4: Run test to verify it passes**

Same command as Step 2. Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/edge_devices/routes.go internal/api/handler/edge_devices/routes_rbac_test.go
git commit -m "$(cat <<'EOF'
fix: agregar RBACCheck a las rutas de edge-devices

Ningún endpoint de edge-devices tenía autorización — cualquier miembro
autenticado del tenant podía crear/habilitar/deshabilitar dispositivos.
Los permission ids ya existían en el seed (migración 000011), solo
faltaba cablearlos vía RBACCheck, mismo patrón que users/tenants.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Cross-tenant fix for `DeleteUser` (Hallazgo C)

**Files:**
- Modify: `internal/api/handler/users/handler.go:358-382` (`DeleteUser`)
- Modify: `internal/app/users/service.go:237-270` (`DeleteUser`)
- Modify: `internal/repo/pg/users/delete_sidedoor_test.go` (existing call sites — signature gains a param)
- Test: `internal/repo/pg/users/delete_cross_tenant_test.go` (create)

**Interfaces:**
- Produces: `Service.DeleteUser(ctx, tenantID, userID string, crossTenant, includeGlobal bool) error` (was `(ctx, tenantID, userID string, includeGlobal bool) error`). `Repository.Delete` signature is unchanged — the fix calls it with the *resolved* target tenant instead of the request tenant, so no repo-layer change is needed.

- [ ] **Step 1: Update existing call sites + write the new test**

In `internal/repo/pg/users/delete_sidedoor_test.go`, update both calls (they gain a `crossTenant` bool before `includeGlobal`):

```go
	// includeGlobal=false (caller no-superadmin): debe fallar con ErrNotFound, NO
	// debe borrar la fila.
	err := svc.DeleteUser(ctx, platformTenant, superID, false, false)
	require.Error(t, err)

	var deletedAt *string
	scanErr := pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", superID).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.Nil(t, deletedAt, "el usuario oculto NO debe quedar soft-deleted por un caller no-superadmin")

	// includeGlobal=true (caller superadmin): debe poder borrarlo normalmente.
	err = svc.DeleteUser(ctx, platformTenant, superID, false, true)
	require.NoError(t, err)
```

Create `internal/repo/pg/users/delete_cross_tenant_test.go`:

```go
package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestDeleteUserCrossTenant cubre Hallazgo C: DeleteUser hardcodeaba
// crossTenant=false en su precheck, así que un super_admin parado en tenantA
// nunca podía borrar a un usuario cuya membresía real vive en tenantB —
// recibía 404 aunque el usuario existiera. Ver
// docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md §A.
func TestDeleteUserCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	err := svc.DeleteUser(ctx, tenantA, userInB, false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")

	var deletedAt *string
	scanErr := pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", userInB).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.Nil(t, deletedAt, "el usuario no debe haberse borrado")
}

func TestDeleteUserCrossTenantTrueBorraUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	err := svc.DeleteUser(ctx, tenantA, userInB, true, false)
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder borrar a un usuario de tenantB")

	var deletedAt *string
	scanErr := pool.QueryRow(ctx, "SELECT deleted_at::text FROM users WHERE id = $1", userInB).Scan(&deletedAt)
	require.NoError(t, scanErr)
	require.NotNil(t, deletedAt, "el usuario debe haber quedado soft-deleted")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... -run CrossTenant -v"
```
Expected: **compile error** — `svc.DeleteUser` still only takes 3 args (`tenantID, userID string, includeGlobal bool`), the new/updated call sites pass 4. This is the expected "red" state for a Go signature change.

- [ ] **Step 3: Implement the fix**

In `internal/api/handler/users/handler.go`, replace the `DeleteUser` handler (lines 357-382):

```go
// DeleteUser handles DELETE /api/v1/users/:id - soft delete a user
func (h *Handler) DeleteUser(c *gin.Context) {
	tenantID := platform.TenantID(c.Request.Context())

	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "MISSING_PARAM",
			Message: "User ID is required",
			Status:  http.StatusBadRequest,
		})
		return
	}

	h.logger.Debug("delete user request", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	includeGlobal := security.CanSeePlatformInternals(c.Request.Context())
	crossTenant := security.IsCrossTenantRole(c.Request.Context())
	err := h.service.DeleteUser(c.Request.Context(), tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		h.logger.Error("delete user failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
```

In `internal/app/users/service.go`, replace the `DeleteUser` method (lines 237-270):

```go
// DeleteUser soft-deletes a user.
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
// crossTenant lo decide el handler vía security.IsCrossTenantRole (Hallazgo C):
// sin esto, el precheck de abajo solo podía resolver un usuario del tenant de
// la request, así que un super_admin/platform_admin parado en un tenant
// distinto al del target recibía 404 en vez de poder borrarlo.
//
// Resuelve el usuario con GetByID (mismo scoping que GetUser/UpdateUser) antes de
// tocar el repo.Delete: repo.Delete no filtra por rol, así que sin este precheck un
// caller no-superadmin podría soft-borrar a un usuario con rol global aunque no
// pudiera verlo — un efecto observable (el usuario oculto deja de poder operar)
// que delata su existencia igual que un 403. Con el precheck, un usuario oculto
// da 404 y el DELETE nunca se ejecuta.
//
// El DELETE en sí se ejecuta contra current.TenantID (el tenant REAL del target,
// resuelto por el precheck), no contra el tenantID de la request: así
// repo.Delete no necesita su propio parámetro crossTenant/escape hatch — ya
// recibe el tenant correcto para el usuario que se está borrando.
func (s *Service) DeleteUser(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) error {
	s.logger.Debug("deleting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	current, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return err
		}
		s.logger.Error("failed to get user for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return err
	}

	if err := s.repo.Delete(ctx, current.TenantID, userID); err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for deletion", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return err
		}
		s.logger.Error("failed to delete user", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return err
	}

	s.logger.Info("user soft-deleted", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Same command as Step 2. Expected: all 4 tests (2 in `delete_sidedoor_test.go`, 2 new) PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/users/handler.go internal/app/users/service.go internal/repo/pg/users/delete_sidedoor_test.go internal/repo/pg/users/delete_cross_tenant_test.go
git commit -m "$(cat <<'EOF'
fix: DeleteUser resuelve el usuario cross-tenant (Hallazgo C)

Mismo root cause que Hallazgo A: el precheck de DeleteUser hardcodeaba
crossTenant=false, así que un super_admin/platform_admin parado en un
tenant distinto al del target recibía 404 al intentar borrarlo. El
DELETE en sí ahora se ejecuta contra el tenant real del target
(resuelto por el precheck), no contra el de la request.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Cross-tenant fix for `UpdateUser` (extensión de Hallazgo C acordada)

**Files:**
- Modify: `internal/api/handler/users/handler.go:262-355` (`UpdateMe`, `UpdateUser`)
- Modify: `internal/app/users/service.go:175-235` (`UpdateUser`)
- Modify: `internal/app/users/service_test.go:170` (existing call site — signature gains a param)
- Test: `internal/repo/pg/users/update_cross_tenant_test.go` (create)

**Interfaces:**
- Produces: `Service.UpdateUser(ctx, tenantID, userID string, crossTenant, includeGlobal bool, cmd *domainUsers.UpdateUserCommand) (*domainUsers.User, error)` (was `(ctx, tenantID, userID string, includeGlobal bool, cmd) (...)`).

- [ ] **Step 1: Update existing call site + write the new test**

In `internal/app/users/service_test.go`, update the call at line 170:

```go
	rolGlobal := "super_admin"
	updated, err := svc.UpdateUser(ctx, platformTenant, user.ID, false, false, &domainUsers.UpdateUserCommand{
		TenantID: platformTenant, UserID: user.ID, Role: &rolGlobal,
	})
```

Create `internal/repo/pg/users/update_cross_tenant_test.go`:

```go
package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestUpdateUserCrossTenant cubre la extensión de Hallazgo C acordada con el
// usuario: mismo root cause que DeleteUser, mismo fix.
func TestUpdateUserCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	newName := "No Debe Aplicar"
	_, err := svc.UpdateUser(ctx, tenantA, userInB, false, false, &domainUsers.UpdateUserCommand{
		TenantID: tenantA, UserID: userInB, FirstName: &newName,
	})
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestUpdateUserCrossTenantTrueActualizaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	newName := "Actualizado Cross Tenant"
	updated, err := svc.UpdateUser(ctx, tenantA, userInB, true, false, &domainUsers.UpdateUserCommand{
		TenantID: tenantA, UserID: userInB, FirstName: &newName,
	})
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder editar a un usuario de tenantB")
	require.Equal(t, newName, updated.FirstName)
	require.Equal(t, tenantB, updated.TenantID, "el update debe haberse aplicado contra el tenant real del target")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... ./internal/app/users/... -run CrossTenant -v"
```
Expected: compile error (signature mismatch), same reasoning as Task 2 Step 2.

- [ ] **Step 3: Implement the fix**

In `internal/api/handler/users/handler.go`, update `UpdateMe` (around line 298, self-service — no cross-tenant reach needed, pass `false` explicitly):

```go
	includeGlobal := security.CanSeePlatformInternals(c.Request.Context())
	updated, err := h.service.UpdateUser(c.Request.Context(), tenantID, user.ID, false, includeGlobal, cmd)
```

And `UpdateUser` (around line 346-347):

```go
	includeGlobal := security.CanSeePlatformInternals(c.Request.Context())
	crossTenant := security.IsCrossTenantRole(c.Request.Context())
	user, err := h.service.UpdateUser(c.Request.Context(), tenantID, userID, crossTenant, includeGlobal, cmd)
```

In `internal/app/users/service.go`, replace `UpdateUser` (lines 175-235):

```go
// UpdateUser updates user fields (name, role, image).
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals: resuelve
// el usuario actual con el mismo scoping que GetUser, para que un usuario oculto dé
// 404 antes de llegar al UPDATE — no 200/403, que confirmarían su existencia.
// crossTenant lo decide el handler vía security.IsCrossTenantRole (extensión de
// Hallazgo C acordada): sin esto, un super_admin/platform_admin parado en un
// tenant distinto al del target no podía editarlo. El UPDATE en sí no necesita
// su propio crossTenant: repo.Update ya escribe contra current.TenantID (el
// tenant real del target, resuelto acá abajo), no contra el tenantID de la
// request.
func (s *Service) UpdateUser(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool, cmd *domainUsers.UpdateUserCommand) (*domainUsers.User, error) {
	if err := cmd.Validate(); err != nil {
		s.logger.Warn("invalid update user command", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", domainUsers.ErrValidation, err)
	}

	s.logger.Debug("updating user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	// Get current user
	current, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for update", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user for update", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	// Apply updates (only updatable fields)
	if cmd.FirstName != nil {
		current.FirstName = *cmd.FirstName
	}
	if cmd.LastName != nil {
		current.LastName = *cmd.LastName
	}
	if cmd.Role != nil {
		// users.role es la columna legada (la membresía real vive en
		// user_tenant_roles, que es lo que lee /me), pero los listados la muestran
		// vía COALESCE(utr.role_id, u.role) cuando no hay UTR activa. Escribir
		// 'super_admin' acá no da permisos, y aun así se valida con el mismo lookup
		// cloakeado: dejarla libre permitiría pintar a un usuario como superadmin y,
		// peor, sacarlo del filtro de cloaking de los listados de users.
		//
		// Se valida contra current.TenantID (el tenant REAL del target), no contra
		// tenantID (el de la request): bajo crossTenant=true esos dos pueden ser
		// distintos, y tenant_can_use_role() debe evaluarse para el tenant donde
		// el rol realmente se va a aplicar.
		targetTenantUUID, err := uuid.Parse(current.TenantID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid tenant_id: %v", domainUsers.ErrValidation, err)
		}
		if err := appRoles.EnsureAssignable(ctx, s.roleRepo, *cmd.Role, targetTenantUUID, includeGlobal); err != nil {
			s.logger.Warn("rol no asignable en update user",
				zap.String("tenant_id", tenantID), zap.String("role", *cmd.Role), zap.Error(err))
			return nil, err
		}
		current.Role = *cmd.Role
	}
	if cmd.Image != nil {
		current.Image = cmd.Image
	}

	updated, err := s.repo.Update(ctx, current)
	if err != nil {
		s.logger.Error("failed to update user", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Info("user updated", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
	return updated, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Same command as Step 2. Expected: all tests PASS, including the pre-existing `TestUpdateUserNoPuedePintarRolGlobal`.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/users/handler.go internal/app/users/service.go internal/app/users/service_test.go internal/repo/pg/users/update_cross_tenant_test.go
git commit -m "$(cat <<'EOF'
fix: UpdateUser resuelve el usuario cross-tenant (extensión de Hallazgo C)

Mismo root cause y mismo fix que DeleteUser. También corrige un bug
adyacente: la validación de EnsureAssignable para cmd.Role usaba el
tenant de la request en vez del tenant real del target, lo que evaluaba
tenant_can_use_role() contra el tenant equivocado bajo crossTenant=true.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Cross-tenant fix for `UpdateUserStatus` (extensión de Hallazgo C acordada)

**Files:**
- Modify: `internal/api/handler/users/handler.go:162-212` (`UpdateUserStatus`)
- Modify: `internal/app/users/service.go:287-356` (`UpdateUserStatus`)
- Test: `internal/repo/pg/users/update_status_cross_tenant_test.go` (create)

**Interfaces:**
- Produces: `Service.UpdateUserStatus(ctx, tenantID, userID, callerID, status string, crossTenant, includeGlobal bool) (*domainUsers.User, error)` (was `(..., status string, includeGlobal bool) (...)`). No existing call sites outside the handler — no other file to update.

- [ ] **Step 1: Write the new test**

Create `internal/repo/pg/users/update_status_cross_tenant_test.go`:

```go
package users_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// TestUpdateUserStatusCrossTenant cubre la extensión de Hallazgo C acordada a
// UpdateUserStatus: mismo root cause, con una capa extra de bug — la
// mutación real (userRoleRepo.UpdateStatus) usaba el tenantID de la request
// en vez del tenant real del target, así que incluso resolviendo el precheck
// no alcanzaba (ver el fix en service.go para el detalle).
const dummyCallerID = "00000000-0000-0000-0000-000000000000"

func TestUpdateUserStatusCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	_, err := svc.UpdateUserStatus(ctx, tenantA, userInB, dummyCallerID, "suspended", false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestUpdateUserStatusCrossTenantTrueActualizaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	urRepo := userRolesRepo.NewUserRoleRepository(pool)
	svc := appUsers.NewService(repo, urRepo, rolesRepo.NewPostgresRepository(pool), zap.NewNop())
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	updated, err := svc.UpdateUserStatus(ctx, tenantA, userInB, dummyCallerID, "suspended", true, false)
	require.NoError(t, err, "con crossTenant=true, un super_admin en tenantA debe poder suspender a un usuario de tenantB")
	require.Equal(t, tenantB, updated.TenantID)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... -run UpdateStatusCrossTenant -v"
```
Expected: compile error (signature mismatch).

- [ ] **Step 3: Implement the fix**

In `internal/api/handler/users/handler.go`, update `UpdateUserStatus` (around lines 203-204):

```go
	includeGlobal := security.CanSeePlatformInternals(c.Request.Context())
	crossTenant := security.IsCrossTenantRole(c.Request.Context())
	user, err := h.service.UpdateUserStatus(c.Request.Context(), tenantID, userID, callerID, req.Status, crossTenant, includeGlobal)
```

In `internal/app/users/service.go`, replace `UpdateUserStatus` (lines 287-356):

```go
// UpdateUserStatus changes the UTR status for a user in the tenant.
// callerID is the ID of the authenticated admin making the request.
// Allowed status values: "active", "inactive", "suspended".
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals: así un
// platform_admin tampoco puede cambiarle el estado a un superadmin invisible, y
// recibe el mismo 404 coherente con GetUser/ListUsers.
// crossTenant lo decide el handler vía security.IsCrossTenantRole (extensión de
// Hallazgo C acordada): sin esto, el precheck solo resolvía un usuario del
// tenant de la request, y la mutación de abajo (userRoleRepo.UpdateStatus)
// usaba ese mismo tenantID de la request en vez del tenant real del target —
// un super_admin parado en tenantA no podía suspender a un usuario de
// tenantB. Con el precheck resolviendo current.TenantID (el tenant real),
// tanto la mutación como el re-fetch final usan ese tenant resuelto, no el
// de la request.
func (s *Service) UpdateUserStatus(ctx context.Context, tenantID, userID, callerID, status string, crossTenant, includeGlobal bool) (*domainUsers.User, error) {
	// Guard: admin cannot deactivate themselves
	if userID == callerID {
		return nil, domainUsers.ErrCannotDeactivateSelf
	}

	// Validate allowed status values
	var utrStatus domain.UserRoleStatus
	switch status {
	case "active":
		utrStatus = domain.UserRoleStatusActive
	case "inactive":
		utrStatus = domain.UserRoleStatusRevoked
	case "suspended":
		utrStatus = domain.UserRoleStatusSuspended
	default:
		return nil, domainUsers.ErrInvalidStatus
	}

	// Resolve the user (existence check + su tenant real, ver comentario arriba)
	current, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found for status update", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user for status update", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	targetTenantUUID, err := uuid.Parse(current.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant_id: %w", err)
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	_, err = s.userRoleRepo.UpdateStatus(ctx, userUUID, targetTenantUUID, utrStatus, includeGlobal)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveAssignment) {
			s.logger.Warn("no active assignment found for status update",
				zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to update user status",
			zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Info("user status updated",
		zap.String("tenant_id", tenantID),
		zap.String("user_id", userID),
		zap.String("status", status))

	// Re-fetch to return the latest state (updatedAt reflects the mutation).
	// crossTenant=false porque current.TenantID ya es el tenant real del target
	// (resuelto arriba) — no hace falta relajar el filtro de nuevo.
	updated, err := s.repo.GetByID(ctx, current.TenantID, userID, false, includeGlobal)
	if err != nil {
		s.logger.Error("failed to re-fetch user after status update",
			zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}
	return updated, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Same command as Step 2. Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/users/handler.go internal/app/users/service.go internal/repo/pg/users/update_status_cross_tenant_test.go
git commit -m "$(cat <<'EOF'
fix: UpdateUserStatus resuelve el usuario cross-tenant (extensión de Hallazgo C)

Mismo root cause que DeleteUser/UpdateUser, con un bug adicional: la
mutación real (userRoleRepo.UpdateStatus) usaba el tenantID de la
request en vez del tenant resuelto del target, así que ni siquiera
arreglando el precheck alcanzaba. Ahora usa current.TenantID en la
mutación y en el re-fetch final.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Cross-tenant fix for `GetUserWithRoles`/`GetByIDWithRoles` + NULLS LAST (item #3)

**Files:**
- Modify: `internal/api/handler/users/handler.go:96-135` (`GetUser`, `include=roles` branch)
- Modify: `internal/app/users/service.go:74-91` (`GetUserWithRoles`)
- Modify: `internal/repo/pg/users/repository.go:24-28` (`Repository.GetByIDWithRoles` interface)
- Modify: `internal/repo/pg/users/postgres.go:311-389` (`GetByIDWithRoles` implementation)
- Modify: `internal/api/handler/users/update_me_test.go:39` (`fakeUserRepo.GetByIDWithRoles` — signature gains a param)
- Test: `internal/repo/pg/users/with_roles_cross_tenant_test.go` (create)

**Interfaces:**
- Produces: `Repository.GetByIDWithRoles(ctx, tenantID, userID string, crossTenant, includeGlobal bool) (*users.UserWithRoles, error)`; `Service.GetUserWithRoles(ctx, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.UserWithRoles, error)`.

- [ ] **Step 1: Update the fake repo + write the new test**

In `internal/api/handler/users/update_me_test.go`, update the fake's method signature (line 39):

```go
func (f *fakeUserRepo) GetByIDWithRoles(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	return nil, domainUsers.ErrNotFound
}
```

Create `internal/repo/pg/users/with_roles_cross_tenant_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... ./internal/api/handler/users/... -run WithRoles -v"
```
Expected: compile error (signature mismatch).

- [ ] **Step 3: Implement the fix**

In `internal/api/handler/users/handler.go`, update the `include=roles` branch inside `GetUser` (lines 116-122 — `crossTenant` is already computed above this branch at line 113, reuse it):

```go
	// If include=roles is requested, fetch user with role data
	if c.Query("include") == "roles" {
		uwr, err := h.service.GetUserWithRoles(c.Request.Context(), tenantID, userID, crossTenant, includeGlobal)
		if err != nil {
			h.logger.Error("get user with roles failed", zap.Error(err))
			HandleError(c, err)
			return
		}
		c.JSON(http.StatusOK, userWithRolesToResponse(uwr))
		return
	}
```

In `internal/app/users/service.go`, replace `GetUserWithRoles` (lines 74-91):

```go
// GetUserWithRoles retrieves a user and their active role assignment in the tenant.
// includeGlobal lo decide el handler vía security.CanSeePlatformInternals.
// crossTenant lo decide el handler vía security.IsCrossTenantRole (item #3 del
// handoff 2026-08-19: el fix de Hallazgo A solo cubrió GetUser sin
// include=roles, esta variante quedó con el mismo bug).
func (s *Service) GetUserWithRoles(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	s.logger.Debug("getting user with roles", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	uwr, err := s.repo.GetByIDWithRoles(ctx, tenantID, userID, crossTenant, includeGlobal)
	if err != nil {
		if errors.Is(err, domainUsers.ErrNotFound) {
			s.logger.Debug("user not found", zap.String("tenant_id", tenantID), zap.String("user_id", userID))
			return nil, err
		}
		s.logger.Error("failed to get user with roles", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Error(err))
		return nil, err
	}

	s.logger.Debug("user with roles retrieved", zap.String("tenant_id", tenantID), zap.String("user_id", userID), zap.Int("role_count", len(uwr.Roles)))
	return uwr, nil
}
```

In `internal/repo/pg/users/repository.go`, replace the `GetByIDWithRoles` interface doc/signature (lines 24-28):

```go
	// GetByIDWithRoles retrieves a user with their active role assignment.
	// crossTenant=true resuelve la membresía activa REAL del target en
	// cualquier tenant, no solo en tenantID — mismo patrón LATERAL que GetByID
	// (ver su comentario para el detalle de por qué no alcanza un simple OR $N
	// sobre un join fijo). Returns ErrNotFound if user doesn't exist, is
	// soft-deleted, or (includeGlobal=false) su rol activo es is_global.
	// The Roles field is an empty slice if no active UTR is found.
	GetByIDWithRoles(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*users.UserWithRoles, error)
```

In `internal/repo/pg/users/postgres.go`, replace `GetByIDWithRoles` entirely (lines 311-389):

```go
// GetByIDWithRoles retrieves a user with their active role assignment in the tenant.
// Uses a LEFT JOIN LATERAL (mismo patrón que GetByID) en vez de un join fijo a
// $2, para poder resolver la membresía REAL del target bajo crossTenant=true —
// ver el comentario de GetByID para el detalle completo de por qué un join fijo
// no alcanza (bypasea includeGlobal, y role/tenant caen al fallback legado).
//
// includeGlobal=false aplica la misma regla de cloaking que GetByID: un miembro
// con rol is_global es indistinguible de uno inexistente (ErrNotFound → 404).
// Esta consulta expone además el nombre y permisos del rol, así que sin este
// filtro el ?include=roles delataría no solo la existencia del super_admin sino
// su rol completo — una fuga peor que la de GetByID.
//
// ORDER BY ... assigned_at DESC NULLS LAST: sin esto, una membresía activa con
// assigned_at NULL ganaba el desempate por ser NULLS FIRST el default de
// Postgres en DESC — lo mismo que se corrigió en GetByID (item #7 del handoff).
func (r *PostgresRepository) GetByIDWithRoles(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*users.UserWithRoles, error) {
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, utr.tenant_id, $2) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image,
		       u.created_at, u.updated_at, u.deleted_at,
		       r.id        AS role_id,
		       r.name      AS role_name,
		       r.permissions AS role_permissions
		FROM users u
		LEFT JOIN LATERAL (
			SELECT t.tenant_id, t.role_id
			FROM user_tenant_roles t
			WHERE t.user_id = u.id
			  AND t.status = 'active'
			  AND (t.tenant_id = $2 OR $4)
			ORDER BY (t.tenant_id = $2) DESC, t.assigned_at DESC NULLS LAST
			LIMIT 1
		) utr ON TRUE
		LEFT JOIN roles r ON r.id = utr.role_id AND r.deleted_at IS NULL
		WHERE u.id = $1
		  AND u.deleted_at IS NULL
		  AND (u.tenant_id = $2 OR utr.tenant_id IS NOT NULL OR $4)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
	`

	row := r.db.QueryRow(ctx, query, userID, tenantID, includeGlobal, crossTenant)

	var u users.User
	var roleID *string
	var roleName *string
	var rolePermsJSON []byte // JSONB scanned as raw bytes, then unmarshalled

	err := row.Scan(
		&u.ID, &u.TenantID, &u.FirstName, &u.LastName, &u.Email, &u.Role, &u.Image,
		&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
		&roleID, &roleName, &rolePermsJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user with roles: %w", err)
	}

	uwr := &users.UserWithRoles{
		User:  u,
		Roles: []users.AssignedRole{},
	}

	if roleID != nil && roleName != nil {
		var perms []string
		if len(rolePermsJSON) > 0 {
			if jsonErr := json.Unmarshal(rolePermsJSON, &perms); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse role permissions: %w", jsonErr)
			}
		}
		if perms == nil {
			perms = []string{}
		}
		uwr.Roles = append(uwr.Roles, users.AssignedRole{
			ID:          *roleID,
			Name:        *roleName,
			Permissions: perms,
		})
	}

	return uwr, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Same command as Step 2. Expected: both new tests PASS, `update_me_test.go` still PASSES.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/users/handler.go internal/app/users/service.go internal/repo/pg/users/repository.go internal/repo/pg/users/postgres.go internal/api/handler/users/update_me_test.go internal/repo/pg/users/with_roles_cross_tenant_test.go
git commit -m "$(cat <<'EOF'
fix: GetByIDWithRoles (?include=roles) resuelve cross-tenant (item #3)

El fix de Hallazgo A solo cubrió GetByID; la variante con
?include=roles usaba un join fijo al tenant de la request y nunca
recibió el parámetro crossTenant. Reescrita con el mismo patrón LEFT
JOIN LATERAL que GetByID, incluyendo NULLS LAST en el desempate por
assigned_at (item #7) ya que es una query nueva.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: NULLS LAST fix for `GetByID`'s existing tiebreak (item #7)

**Files:**
- Modify: `internal/repo/pg/users/postgres.go:96-152` (`GetByID`)
- Modify: `internal/repo/pg/users/cross_tenant_test.go` (append new test)

- [ ] **Step 1: Write the failing test**

Append to `internal/repo/pg/users/cross_tenant_test.go` (after the last existing test function, same file — imports already include `uuid`, `require`, `usersRepo`):

```go
// TestGetByIDCrossTenantTrueConMembresiaSinAssignedAtNoGanaElDesempate cubre
// item #7 del handoff 2026-08-19: sin NULLS LAST explícito, el default de
// Postgres para DESC es NULLS FIRST, así que una membresía activa con
// assigned_at NULL ganaba el desempate aunque no fuera "la más reciente".
func TestGetByIDCrossTenantTrueConMembresiaSinAssignedAtNoGanaElDesempate(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)

	// Membresía en tenantB con assigned_at seteado (la que debe ganar el desempate).
	userID := seedUserWithRole(t, pool, tenantB, "cliente_operario")

	// Segunda membresía activa, en tenantC, con assigned_at NULL — antes del
	// fix, NULLS FIRST la hacía ganar el ORDER BY ... assigned_at DESC pese a
	// no tener fecha real.
	tenantC := seedTenant(t, pool)
	utrCID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'cliente_admin', 'active', NULL)`,
		utrCID, userID, tenantC)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrCID)
	})

	// La request pide parada en tenantA (no matchea ninguna de las dos
	// membresías), así que el desempate cae en assigned_at DESC: debe ganar
	// la de tenantB (assigned_at real), no la de tenantC (NULL).
	u, err := repo.GetByID(ctx, tenantA, userID, true, false)
	require.NoError(t, err)
	require.Equal(t, tenantB, u.TenantID, "la membresía con assigned_at real debe ganar sobre la de assigned_at NULL")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... -run NoGanaElDesempate -v"
```
Expected: FAIL — `u.TenantID` resolves to `tenantC` (the NULL-`assigned_at` row) instead of `tenantB`.

- [ ] **Step 3: Implement the fix**

In `internal/repo/pg/users/postgres.go`, in `GetByID` (around line 132), change:

```go
			ORDER BY (t.tenant_id = $2) DESC, t.assigned_at DESC
```
to:
```go
			ORDER BY (t.tenant_id = $2) DESC, t.assigned_at DESC NULLS LAST
```

Also update the doc comment above `GetByID` (around lines 113-115) to add one line after the existing tiebreak explanation:

```go
// membresía del tenant de la request ($2) gana si existe, y si no, la más
// reciente por assigned_at (NULLS LAST explícito: el default de Postgres para
// DESC es NULLS FIRST, lo que haría ganar a una membresía sin assigned_at
// aunque no sea la más reciente — item #7 del handoff 2026-08-19).
```

- [ ] **Step 4: Run test to verify it passes**

Same command as Step 2. Expected: PASS. Also re-run the full cross-tenant suite to confirm no regression:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... -v"
```

- [ ] **Step 5: Commit**

```bash
git add internal/repo/pg/users/postgres.go internal/repo/pg/users/cross_tenant_test.go
git commit -m "$(cat <<'EOF'
fix: NULLS LAST en el desempate de assigned_at de GetByID (item #7)

Sin NULLS LAST explícito, el default de Postgres para ORDER BY ... DESC
es NULLS FIRST: una membresía activa con assigned_at NULL ganaba el
desempate del LEFT JOIN LATERAL aunque no fuera la más reciente.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Verificar que `platform_admin`/`super_admin` sean rechazados fuera del tenant plataforma (item #8)

**Files:**
- Modify: `internal/repo/pg/user_roles/repository_test.go` (append 2 tests after the existing `TestTriggerRechazaInsertRawDeAdminEnTenantNoPlataforma`)

Esto es una tarea de **verificación**, no de fix: si el trigger ya rechaza correctamente estos roles (lo esperable, dado que comparten el mismo eje `is_global`/no-`admin`/no-`operario` que ya cubre `tenant_can_use_role`), los tests pasan en el primer run y el pendiente se cierra sin tocar código de producción. Si fallan, ahí aparece un bug real que requeriría una spec/plan separado.

- [ ] **Step 1: Write the tests**

Append to `internal/repo/pg/user_roles/repository_test.go` (imports already include `uuid`, `pgconn`, `errors`, `require` — no import changes needed):

```go
// TestTriggerRechazaInsertRawDePlatformAdminEnTenantNoPlataforma y su par
// ...SuperAdminEnTenantNoPlataforma cierran el item #8 del handoff
// 2026-08-19: no había test directo confirmando que platform_admin/
// super_admin (ambos is_global=TRUE) sean rechazados fuera del tenant
// plataforma por el mismo trigger que ya se probaba solo para 'admin'.
func TestTriggerRechazaInsertRawDePlatformAdminEnTenantNoPlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	ctx := context.Background()

	tenantID := seedTenant(t, pool)
	s := seedMembership(t, pool, tenantID, "", "revoked")

	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'platform_admin', 'active', NOW())`,
		uuid.New(), s.UserID, tenantID,
	)
	require.Error(t, err)

	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "el rechazo del trigger debe llegar como *pgconn.PgError")
	require.Equal(t, "23514", pgErr.Code,
		"trg_enforce_platform_role_tenant debe rechazar platform_admin con check_violation (23514)")
}

func TestTriggerRechazaInsertRawDeSuperAdminEnTenantNoPlataforma(t *testing.T) {
	pool := poolOrSkip(t)
	ctx := context.Background()

	tenantID := seedTenant(t, pool)
	s := seedMembership(t, pool, tenantID, "", "revoked")

	_, err := pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'super_admin', 'active', NOW())`,
		uuid.New(), s.UserID, tenantID,
	)
	require.Error(t, err)

	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "el rechazo del trigger debe llegar como *pgconn.PgError")
	require.Equal(t, "23514", pgErr.Code,
		"trg_enforce_platform_role_tenant debe rechazar super_admin con check_violation (23514)")
}
```

- [ ] **Step 2: Run the tests**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$(cat ~/.supabase-db-url)" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/user_roles/... -run TenantNoPlataforma -v"
```

- [ ] **Step 3: Branch on the result**

- If both PASS: no production code change needed. Proceed straight to Step 4 (commit the tests as regression coverage).
- If either FAILS: **stop** — do not attempt a fix inline. This means `tenant_can_use_role`/`trg_enforce_platform_role_tenant` has a real gap for that role. Report the failure with the exact error message; it needs its own spec (`superpowers:brainstorming`) before a fix, since it would touch a security-enforcing DB trigger.

- [ ] **Step 4: Commit**

```bash
git add internal/repo/pg/user_roles/repository_test.go
git commit -m "$(cat <<'EOF'
test: verificar que platform_admin/super_admin sean platform-only (item #8)

Cierra el gap de cobertura: el único test existente del trigger
trg_enforce_platform_role_tenant cubría 'admin', no los otros 2 roles
is_global (platform_admin, super_admin) que dependen de la misma regla.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Frontend — gating de botón `_manage` en users/tenants/roles (item #5)

**Files (repo `embolsadora-frontend`):**
- Modify: `src/components/users/columns.tsx`, `src/components/users/user-list.tsx`
- Modify: `src/components/tenants/columns.tsx`, `src/components/tenants/tenant-list.tsx`, `src/app/s/[tenantId]/(dashboard)/tenants/page.tsx`
- Modify: `src/components/roles/columns.tsx`, `src/components/roles/role-list.tsx`

- [ ] **Step 1: Users — gate edit/delete columns**

In `src/components/users/columns.tsx`, add `canManage` and make the actions column conditional:

```tsx
import type { ColumnDef } from '@/components/ui/data-table';
import type { User } from '@/types/user';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Pencil, Trash2 } from 'lucide-react';
import Image from 'next/image';
import { PREDEFINED_ROLES } from '@/lib/role-registry';

interface UserColumnsParams {
  tenantId: string;
  showTenantColumn?: boolean;
  canManage: boolean;
  onEdit: (userId: string) => void;
  onDelete: (user: User) => void;
}

export function getUserColumns({
  tenantId: _tenantId,
  showTenantColumn = false,
  canManage,
  onEdit,
  onDelete,
}: UserColumnsParams): ColumnDef<User>[] {
  const columns: ColumnDef<User>[] = [
    {
      id: 'name',
      header: 'Nombre',
      cell: (row) => (
        <div className="flex items-center space-x-3">
          {row.image ? (
            <Image
              src={row.image}
              alt={`${row.firstName} ${row.lastName}`}
              className="h-8 w-8 rounded-full"
              width={32}
              height={32}
            />
          ) : (
            <div className="h-8 w-8 rounded-full bg-muted flex items-center justify-center text-xs font-medium">
              {row.firstName[0]}
              {row.lastName[0]}
            </div>
          )}
          <span className="font-medium">
            {row.firstName} {row.lastName}
          </span>
        </div>
      ),
    },
    {
      id: 'email',
      header: 'Email',
      accessorKey: 'email',
    },
  ];

  if (showTenantColumn) {
    columns.push({
      id: 'tenant',
      header: 'Tenant',
      cell: (row) => <span>{row.tenantName ?? '—'}</span>,
    });
  }

  columns.push(
    {
      id: 'role',
      header: 'Rol',
      cell: (row) => {
        if (!row.role) return <Badge variant="outline">Sin rol</Badge>;
        const role = Object.values(PREDEFINED_ROLES).find((r) => r.id === row.role);
        return (
          <div className="flex items-center gap-2">
            <Badge
              variant={role?.isGlobal ? 'default' : role?.isSystemRole ? 'secondary' : 'outline'}
            >
              {role?.name || row.role}
            </Badge>
            {role?.isGlobal && <span className="text-xs text-muted-foreground">Global</span>}
          </div>
        );
      },
    },
    {
      id: 'createdAt',
      header: 'Creado',
      cell: (row) => new Date(row.createdAt).toLocaleDateString('es-AR'),
    }
  );

  if (canManage) {
    columns.push({
      id: 'actions',
      header: 'Acciones',
      align: 'right',
      cell: (row) => (
        <div className="flex justify-end space-x-2">
          <Button variant="ghost" size="icon" onClick={() => onEdit(row.id)}>
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={() => onDelete(row)}
            className="text-destructive hover:text-destructive"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ),
    });
  }

  return columns;
}
```

In `src/components/users/user-list.tsx`, add the import and wire `canManage` (add near the other hooks around line 14, and update the `useMemo` around line 149):

```tsx
import { useCanPerformAction } from '@/hooks/use-can-access';
import { PERMISSIONS } from '@/lib/permissions';
```

```tsx
  const canManage = useCanPerformAction(PERMISSIONS.USERS_MANAGE.id);
```

```tsx
  const columns = useMemo(
    () =>
      getUserColumns({
        tenantId: tenantId as string,
        showTenantColumn,
        canManage,
        onEdit: (id) => router.push(`/s/${tenantId}/users/${id}`),
        onDelete: handleDelete,
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [tenantId, showTenantColumn, canManage]
  );
```

- [ ] **Step 2: Tenants — gate edit/delete columns and the "Nuevo Tenant" button**

In `src/components/tenants/columns.tsx`, add `canManage` and make the actions column conditional:

```tsx
import type { ColumnDef } from '@/components/ui/data-table';
import { StatusBadge } from '@/components/ui/status-badge';
import { Button } from '@/components/ui/button';
import { Pencil, Trash2 } from 'lucide-react';

export interface TenantRow {
  id: string;
  name: string;
  companyName: string;
  subdomain: string;
  isActive: boolean;
  createdAt: string;
}

interface TenantColumnsParams {
  basePath: string;
  canManage: boolean;
  onEdit: (tenantId: string) => void;
  onDelete: (tenantId: string) => void;
}

const appOrigin = (() => {
  try {
    return new URL(process.env.NEXT_PUBLIC_APP_URL ?? 'https://embolsadora.site');
  } catch {
    return new URL('https://embolsadora.site');
  }
})();

export function getTenantColumns({
  basePath: _basePath,
  canManage,
  onEdit,
  onDelete,
}: TenantColumnsParams): ColumnDef<TenantRow>[] {
  const columns: ColumnDef<TenantRow>[] = [
    {
      id: 'name',
      header: 'Nombre',
      cell: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      id: 'companyName',
      header: 'Empresa',
      accessorKey: 'companyName',
    },
    {
      id: 'subdomain',
      header: 'Subdominio',
      cell: (row) => (
        <a
          href={`${appOrigin.protocol}//${row.subdomain}.${appOrigin.host}`}
          target="_blank"
          rel="noopener noreferrer"
          className="text-primary hover:underline"
        >
          {row.subdomain}.{appOrigin.host}
        </a>
      ),
    },
    {
      id: 'status',
      header: 'Estado',
      cell: (row) => <StatusBadge status={row.isActive ? 'active' : 'inactive'} />,
    },
    {
      id: 'createdAt',
      header: 'Creado',
      cell: (row) => new Date(row.createdAt).toLocaleDateString('es-AR'),
    },
  ];

  if (canManage) {
    columns.push({
      id: 'actions',
      header: 'Acciones',
      align: 'right',
      cell: (row) => (
        <div className="flex justify-end space-x-2">
          <Button variant="ghost" size="icon" onClick={() => onEdit(row.id)}>
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="text-destructive hover:text-destructive"
            onClick={() => onDelete(row.id)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ),
    });
  }

  return columns;
}
```

In `src/components/tenants/tenant-list.tsx`, add the import (near line 10) and wire `canManage` into the `useMemo` (around line 105):

```tsx
import { useCanPerformAction } from '@/hooks/use-can-access';
import { PERMISSIONS } from '@/lib/permissions';
```

```tsx
  const canManage = useCanPerformAction(PERMISSIONS.TENANTS_MANAGE.id);
```

```tsx
  const columns = useMemo(
    () =>
      getTenantColumns({
        basePath,
        canManage,
        onEdit: (id) => router.push(`${basePath}/tenants/${id}`),
        onDelete: handleDelete,
      }),
    [basePath, canManage, handleDelete, router]
  );
```

Replace `src/app/s/[tenantId]/(dashboard)/tenants/page.tsx` entirely:

```tsx
'use client';

import { useParams, useRouter } from 'next/navigation';
import { TenantList } from '@/components/tenants/tenant-list';
import { PageHeader } from '@/components/ui/page-header';
import { Button } from '@/components/ui/button';
import { useCanPerformAction } from '@/hooks/use-can-access';
import { PERMISSIONS } from '@/lib/permissions';

export default function TenantsPage() {
  const { tenantId } = useParams();
  const router = useRouter();
  const canManage = useCanPerformAction(PERMISSIONS.TENANTS_MANAGE.id);

  return (
    <div className="space-y-6">
      <PageHeader
        title="Tenants"
        description="Administra los tenants de la plataforma"
        actions={
          canManage ? (
            <Button onClick={() => router.push(`/s/${tenantId}/tenants/new`)}>Nuevo Tenant</Button>
          ) : undefined
        }
      />
      <TenantList />
    </div>
  );
}

export const dynamic = 'force-dynamic';
```

- [ ] **Step 3: Roles — gate edit/delete actions (uses `USERS_MANAGE`, since roles piggyback on the users permission namespace — see `route-permissions.ts:44`, "Need users permission to see roles")**

In `src/components/roles/columns.tsx`, add `canManage` and combine it with the existing `!rep.isSystemRole` check:

```tsx
import type { ColumnDef } from '@/components/ui/data-table';
import type { Role } from '@/types/role';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { PermissionsPopover } from './permissions-popover';
import { Eye, Pencil, Trash2 } from 'lucide-react';
import Link from 'next/link';

export interface GroupedRole {
  key: string;
  name: string;
  description: string;
  isSystemRole: boolean;
  isGlobal: boolean;
  permissions: string[];
  tenants: { slug: string; name: string }[];
  representative: Role;
}

interface RoleColumnsParams {
  tenantId: string;
  showTenantColumn?: boolean;
  canManage: boolean;
  onDelete?: (role: Role) => void;
}

export function getRoleColumns({
  tenantId,
  showTenantColumn = false,
  canManage,
  onDelete,
}: RoleColumnsParams): ColumnDef<GroupedRole>[] {
  const columns: ColumnDef<GroupedRole>[] = [
    {
      id: 'name',
      header: 'Nombre',
      cell: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      id: 'description',
      header: 'Descripción',
      cell: (row) => (
        <span className="text-muted-foreground max-w-xs truncate block">{row.description}</span>
      ),
    },
  ];

  if (showTenantColumn) {
    columns.push({
      id: 'tenant',
      header: 'Tenant',
      cell: (row) => {
        if (row.isGlobal) {
          return <span className="text-muted-foreground text-sm">Global</span>;
        }
        if (row.tenants.length === 0) {
          return <span className="text-muted-foreground">—</span>;
        }
        return (
          <div className="flex flex-wrap gap-1">
            {row.tenants.map((t) => (
              <Badge key={t.slug} variant="outline" className="text-xs">
                {t.name}
              </Badge>
            ))}
          </div>
        );
      },
    });
  }

  columns.push(
    {
      id: 'type',
      header: 'Tipo',
      cell: (row) => (
        <div className="flex gap-2">
          {row.isSystemRole ? (
            <Badge variant="secondary">Predefinido</Badge>
          ) : (
            <Badge variant="outline">Personalizado</Badge>
          )}
          {row.isGlobal && <Badge variant="default">Global</Badge>}
        </div>
      ),
    },
    {
      id: 'permissions',
      header: 'Permisos',
      align: 'right',
      cell: (row) => <PermissionsPopover permissions={row.permissions} />,
    },
    {
      id: 'actions',
      header: 'Acciones',
      align: 'right',
      cell: (row) => {
        const rep = row.representative;
        const targetSlug = rep.tenantSlug ?? tenantId;
        const canEditThis = canManage && !rep.isSystemRole;
        return (
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="icon" asChild title="Ver detalles">
              <Link href={`/s/${targetSlug}/roles/${rep.id}`}>
                <Eye className="h-4 w-4" />
              </Link>
            </Button>
            {canEditThis && (
              <>
                <Button variant="ghost" size="icon" asChild title="Editar rol">
                  <Link href={`/s/${targetSlug}/roles/${rep.id}/edit`}>
                    <Pencil className="h-4 w-4" />
                  </Link>
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => onDelete?.(rep)}
                  title="Eliminar rol"
                  className="text-destructive hover:text-destructive"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </>
            )}
            {!canEditThis && <div className="w-[72px]" />}
          </div>
        );
      },
    }
  );

  return columns;
}
```

In `src/components/roles/role-list.tsx`, add the import (near line 18) and wire `canManage` into the `useMemo` (around line 128):

```tsx
import { useCanPerformAction } from '@/hooks/use-can-access';
import { PERMISSIONS } from '@/lib/permissions';
```

```tsx
  const canManage = useCanPerformAction(PERMISSIONS.USERS_MANAGE.id);
```

```tsx
  const columns = useMemo(
    () =>
      getRoleColumns({
        tenantId,
        showTenantColumn,
        canManage,
        onDelete: (role) => setTarget(role),
      }),
    [tenantId, showTenantColumn, canManage]
  );
```

- [ ] **Step 4: Verify — type-check, lint, build**

```bash
pnpm tsc --noEmit && pnpm lint && pnpm build
```
Expected: no errors. This catches any missed prop (e.g., a call site still passing the old `getUserColumns`/`getTenantColumns`/`getRoleColumns` params shape without `canManage`).

- [ ] **Step 5: Manual browser verification**

Start the dev server (`pnpm dev`), log in as `admin@test.com` / `Test1234!` against tenant `demo`, and:
1. Visit `/s/demo/users` — confirm the Edit/Delete icons still show (admin has `perm_users_manage`).
2. Visit `/s/demo/tenants` — confirm "Nuevo Tenant" button and edit/delete icons still show.
3. Visit `/s/demo/roles` — confirm edit/delete icons still show for custom roles.
This confirms the gating didn't accidentally hide actions for a role that should have them. (A negative check — logging in as a `_view`-only role — is out of scope for this manual pass unless such a test account already exists; if none does, the positive check plus the type-check pass is sufficient evidence for this task.)

- [ ] **Step 6: Commit**

```bash
git add src/components/users/columns.tsx src/components/users/user-list.tsx src/components/tenants/columns.tsx src/components/tenants/tenant-list.tsx "src/app/s/[tenantId]/(dashboard)/tenants/page.tsx" src/components/roles/columns.tsx src/components/roles/role-list.tsx
git commit -m "$(cat <<'EOF'
fix: gating de botón _manage en users/tenants/roles (item #5)

useCanPerformAction existía pero tenía un solo call site en todo el
repo (reports). Ahora también gatea editar/borrar usuario, crear/
editar/borrar tenant, y editar/borrar rol — hasta ahora solo el
sidebar/ruta estaban gateados, cualquiera con acceso al módulo veía
los botones de acción sin importar su permiso _manage.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Frontend — sacar el header `x-tenant-id` muerto de `UserTenantRoles` (item #6)

**Files (repo `embolsadora-frontend`):**
- Modify: `src/components/users/user-tenant-roles.tsx`

**Nota importante — el fix real difiere del descrito en la spec original:** la spec decía "usar el prop `currentTenantId` en vez de `profile?.tenant?.id`". Al armar este plan encontré que **eso no arregla nada**: `src/proxy.ts:99-101` descarta *cualquier* `x-tenant-id` provisto por el cliente en rutas `/api/*` y lo re-deriva del Referer — el propio `page.tsx` padre (línea 29-37) ya documenta este mismo comportamiento para la misma ruta (`/api/user-roles`) y por eso deliberadamente NO manda el header. El fix correcto es sacar el header por completo, no cambiar qué valor lleva.

- [ ] **Step 1: Implement the fix**

Replace `src/components/users/user-tenant-roles.tsx` lines 1-65 (imports through the `useEffect`):

```tsx
'use client';
/* eslint-disable no-console */

import { useEffect, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

interface UserTenantRolesProps {
  userId: string;
  currentTenantId: string;
}

interface RoleAssignment {
  id: string;
  userId: string;
  tenantId: string;
  roleId: string;
  roleName?: string;
  status: string;
  assignedAt: string;
}

export function UserTenantRoles({ userId, currentTenantId }: UserTenantRolesProps) {
  const [assignments, setAssignments] = useState<RoleAssignment[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!currentTenantId) return;
    const load = async () => {
      try {
        setLoading(true);
        // Sin header x-tenant-id a propósito: src/proxy.ts lo descarta para
        // cualquier valor client-supplied en rutas /api/* y lo re-deriva del
        // Referer (/s/{tenantId}/...) — mismo comportamiento que ya
        // documenta el sibling loadCurrentAssignment en el page.tsx padre
        // para esta misma ruta. Antes acá se mandaba profile?.tenant?.id (el
        // tenant del viewer, no el que se está mirando) — un valor que
        // además el proxy siempre pisaba, doblemente inútil (item #6).
        const res = await fetch(`/api/user-roles?userId=${userId}`);
        if (!res.ok) {
          setAssignments([]);
          return;
        }
        const data = await res.json();
        const items: RoleAssignment[] = data.assignments ?? data.items ?? data ?? [];
        setAssignments(items);
      } catch (err) {
        console.error('Failed to load tenant roles:', err);
        setAssignments([]);
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [userId, currentTenantId]);
```

The rest of the file (the `if (loading)` block and everything below it) is unchanged.

- [ ] **Step 2: Verify — type-check, lint, build**

```bash
pnpm tsc --noEmit && pnpm lint && pnpm build
```
Expected: no errors (confirms `useBackendProfile` import removal doesn't leave a dangling reference — it was only used for the removed `tenantUuid`).

- [ ] **Step 3: Manual browser verification**

With the dev server running, visit `/s/demo/users/{any-user-id}` as `admin@test.com`, scroll to "Roles asignados", and confirm the table still loads role assignments (proves the fetch works without the header, same as it did before — the proxy was always the real source of truth).

- [ ] **Step 4: Commit**

```bash
git add src/components/users/user-tenant-roles.tsx
git commit -m "$(cat <<'EOF'
fix: sacar header x-tenant-id muerto de UserTenantRoles (item #6)

El prop currentTenantId se recibía pero se descartaba; el header
mandaba profile?.tenant?.id (tenant del viewer, no del que se mira).
Pero src/proxy.ts descarta cualquier x-tenant-id client-supplied en
/api/* y lo re-deriva del Referer — el header era doblemente inútil,
no solo tenía el valor equivocado. Se saca por completo, mismo patrón
que ya usa el sibling loadCurrentAssignment en el page.tsx padre.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 10: Frontend — tipado estricto de `PERMISSIONS` y `sidebar.tsx` (items #10, #13)

**Files (repo `embolsadora-frontend`):**
- Modify: `src/lib/permissions.ts`
- Modify: `src/components/sidebar.tsx`

- [ ] **Step 1: Implement the fix**

In `src/lib/permissions.ts`, replace lines 74-76:

```ts
/**
 * Complete catalog of system permissions, keyed by symbolic name. Display fields
 * come from the DB (baked JSON); ids come from the PERMISSION_IDS contract.
 */
export const PERMISSIONS: Record<keyof typeof PERMISSION_IDS, Permission> = Object.fromEntries(
  Object.entries(PERMISSION_IDS).map(([key, id]) => [key, buildPermission(id)])
) as Record<keyof typeof PERMISSION_IDS, Permission>;
```

In `src/components/sidebar.tsx`, add the import (near line 8) and change the `permission` field type (line 42):

```tsx
import { PERMISSION_IDS } from '@/lib/permissions';
```

```tsx
const navItems: Array<{
  title: string;
  href: string;
  icon: React.ReactNode;
  group: NavGroup;
  permission?: (typeof PERMISSION_IDS)[keyof typeof PERMISSION_IDS];
  showBadge?: boolean;
}> = [
```

No other lines in `sidebar.tsx` change — every existing `permission: 'perm_xxx'` literal already matches a value in `PERMISSION_IDS`, so they type-check against the new union without edits.

- [ ] **Step 2: Verify — type-check, lint, build**

```bash
pnpm tsc --noEmit && pnpm lint && pnpm build
```
Expected: no errors. If any `navItems` entry had a typo'd permission string, this step would now catch it at compile time (that's the point of the fix) — if it does, fix the typo as part of this same commit.

- [ ] **Step 3: Commit**

```bash
git add src/lib/permissions.ts src/components/sidebar.tsx
git commit -m "$(cat <<'EOF'
refactor: tipar PERMISSIONS y sidebar.tsx contra PERMISSION_IDS (items #10, #13)

PERMISSIONS era Record<string, Permission> y sidebar.tsx's navItems
tipaba permission como string suelto — un id de permiso mal escrito
type-checkeaba igual y fallaba en runtime silenciosamente. Ahora ambos
están atados al contrato PERMISSION_IDS.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 11: Frontend — borrar `SECTION_PERMISSIONS`/`useCanAccess`/`useAccessibleSections` muertos + comentarios desactualizados (items #11, #12)

**Files (repo `embolsadora-frontend`):**
- Modify: `src/hooks/use-can-access.tsx`
- Modify: `src/lib/route-permissions.ts` (comment only)
- Modify: `scripts/generate-system-permissions.mjs` (comment only)

- [ ] **Step 1: Implement the fix**

Replace `src/hooks/use-can-access.tsx` entirely:

```tsx
/**
 * useCanAccess - Hooks for permission-based access control
 *
 * Usage:
 * ```tsx
 * const canManageUsers = useCanPerformAction(PERMISSIONS.USERS_MANAGE.id);
 * ```
 */

'use client';

import { useMemo } from 'react';
import { useRole } from '@/contexts/RoleContext';

/**
 * Check if user can perform a specific action
 *
 * @param permissionId - The exact permission ID to check
 * @returns boolean indicating if user has the permission
 *
 * @example
 * ```tsx
 * const canManageUsers = useCanPerformAction('perm_users_manage');
 * ```
 */
export function useCanPerformAction(permissionId: string): boolean {
  const { hasPermission } = useRole();
  return useMemo(() => hasPermission(permissionId), [permissionId, hasPermission]);
}

/**
 * Check if user is a global role (Super Admin or Tenant Manager)
 *
 * Global roles have access to multiple tenants.
 *
 * @returns boolean indicating if user has a global role
 */
export function useIsGlobalRole(): boolean {
  const { role } = useRole();
  return useMemo(() => role?.isGlobal ?? false, [role]);
}
```

This removes `SECTION_PERMISSIONS`, `useCanAccess`, `useAccessibleSections`, and their now-unused imports (`PERMISSIONS`, `PermissionSection`) — confirmed zero call sites for the 3 removed exports anywhere else in the repo.

In `src/lib/route-permissions.ts`, update the comment at lines 84-85 (it references the hook this task deletes):

```ts
  //  3. perm_reports es el permiso grueso legado. Su único otro consumidor era
  //     SECTION_PERMISSIONS en use-can-access.tsx (hook sin call-sites,
  //     eliminado en la limpieza de producción de 2026-08-19).
```

In `scripts/generate-system-permissions.mjs`, update line 4:

```js
// of truth for the 18 system permissions' display metadata (name/section/
```

- [ ] **Step 2: Verify — type-check, lint, build**

```bash
pnpm tsc --noEmit && pnpm lint && pnpm build
```
Expected: no errors — this is the real regression check here (an unnoticed call site for the removed exports would show up as a type error).

- [ ] **Step 3: Commit**

```bash
git add src/hooks/use-can-access.tsx src/lib/route-permissions.ts scripts/generate-system-permissions.mjs
git commit -m "$(cat <<'EOF'
chore: borrar SECTION_PERMISSIONS/useCanAccess/useAccessibleSections sin uso (item #12)

Cero call sites reales en todo el repo — el propio código ya lo
documentaba como muerto. De paso, item #11: corrige "17" a "18" en el
comentario de generate-system-permissions.mjs (18 permisos reales).

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 12: Investigar Hallazgo D — 5xx pese a escritura exitosa

**Repo:** `embolsadora4.0-cloud` (Cloud Run) — read-only investigation, no branch needed for the investigation itself.

Esta tarea es de **investigación abierta**: la causa raíz no se conoce todavía (ver spec §B). No hay un "Step 3: implementar el fix" predefinido porque el fix depende de lo que aparezca en los logs.

- [ ] **Step 1: Ubicar el timestamp exacto del borrado del usuario de prueba**

```bash
psql "$(cat ~/.supabase-db-url)" -c "SELECT id, email, deleted_at FROM users WHERE email = 'smoketest-b005-user@gmail.com';"
```
Esto da el `deleted_at` real (el `UPDATE` que sí committeó, pese al 502 devuelto). Anotar el valor exacto (formato `2026-08-19T HH:MM:SS.ssssssZ`).

- [ ] **Step 2: Traer los logs de Cloud Run alrededor de ese timestamp**

Reemplazar `<DELETED_AT>` por el valor del paso 1, ventana de ±10 minutos:

```bash
gcloud logging read \
  'resource.type="cloud_run_revision"
   AND resource.labels.service_name="embolsadora-api"
   AND timestamp>="<DELETED_AT_MENOS_10MIN>"
   AND timestamp<="<DELETED_AT_MAS_10MIN>"' \
  --project=embolsadora \
  --format=json \
  --limit=500 > /tmp/hallazgo-d-logs.json
```

- [ ] **Step 3: Revisar los logs con esta guía de lectura**

Buscar en `/tmp/hallazgo-d-logs.json`, en este orden de sospecha (según lo que ya documenta el handoff):
1. Cualquier entrada de severidad ERROR/WARNING en la ventana, especialmente después del log `"user soft-deleted"` (el `Info` que emite `DeleteUser` tras el `repo.Delete` exitoso — ver `internal/app/users/service.go`).
2. Cualquier mención a Supabase, GoTrue, o una llamada HTTP saliente fallando/timeouteando — candidato principal, dado que no hay ninguna llamada así en el código actual de `DeleteUser` (revisado en este plan, Task 2) lo cual es en sí mismo un dato: **si los logs muestran una llamada a Supabase que este handler no hace explícitamente, buscar dónde se dispara** (¿un trigger de DB? ¿un middleware de request?).
3. Un patrón de "revision starting"/cold start exactamente en la ventana — indicaría que el 5xx es infraestructura (Cloud Run reiniciando/escalando en el momento del request), no un bug de aplicación. Si es esto, el "fix" es documentar el hallazgo, no cambiar código.
4. Cualquier panic/recover de Go (`"panic:"` en el log) — indicaría un bug real post-commit en el código del handler/response.

- [ ] **Step 4: Decidir según lo encontrado**

- Si es un bug de aplicación con un fix acotado (p.ej. un paso post-commit que debería ser fire-and-forget o loguear en vez de fallar la response): implementarlo en un commit separado, con su propio test, siguiendo el mismo patrón TDD de las tareas anteriores. Abrir un nuevo task/plan corto si el fix resulta no trivial.
- Si es infraestructura (cold start, deploy en curso) o no hay evidencia concluyente: documentar el hallazgo (o la falta de uno) como una nueva subsección en `docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md` §B, y cerrar el pendiente como "monitoreado, sin causa de aplicación identificada" — no inventar un fix especulativo para algo no confirmado.

- [ ] **Step 5: Registrar el resultado**

Cualquiera sea el resultado, actualizar §B de la spec con lo encontrado (2-3 frases) y commitear ese cambio solo (sin mezclar con código, salvo que el fix haya sido trivial y se commiteé junto).

---

### Task 13: Investigar el bug de UX del formulario "Nuevo Tenant" (item #14)

**Repo:** `embolsadora-frontend` — investigación interactiva vía browser, no hay fix predefinido (ver spec §F: el análisis estático no encontró la causa).

- [ ] **Step 1: Levantar el dev server**

```bash
pnpm dev
```
(puerto 3000, credenciales de prueba `admin@test.com` / `Test1234!`, tenant `demo` — ver `CLAUDE.md`).

- [ ] **Step 2: Reproducir el flujo con browser automation**

Usando las herramientas `mcp__claude-in-chrome__*`:
1. Navegar a `http://localhost:3000/s/demo/tenants/new`.
2. Completar **todos** los campos de "Información Básica" y "Dirección" (Calle, Ciudad, Provincia, Código Postal, País) con valores de prueba.
3. Dejar vacío el campo "Email" de "Administrador del Tenant" (el único que dispara `form.setError('adminEmail', ...)` sin resetear nada más, según el código revisado en la spec).
4. Hacer submit.
5. Tras el error de validación, leer los valores actuales de los 5 campos de dirección (vía `read_page` o inspeccionando los inputs).

- [ ] **Step 3: Decidir según lo encontrado**

- Si los valores de dirección **persisten**: el bug no reproduce con este código actual. Documentar en la spec §F que se intentó reproducir sin éxito, con los pasos exactos usados, y cerrar el pendiente — no hay nada que arreglar sin una reproducción real. Vale la pena probar 1-2 variantes (dejar un campo de dirección vacío en vez de adminEmail; probar con autofill del browser desactivado vs activado) antes de concluir esto, ya que el handoff original no especifica el trigger exacto.
- Si los valores **se pierden**: usar `read_console_messages` y, de ser necesario, agregar un `console.log` temporal en `onSubmit`/el `useEffect` de `tenant-form.tsx` para capturar el re-render exacto que los limpia, luego escribir un fix acotado (probablemente una dependencia de más en algún `useEffect`, o un remount vía `key` en el padre) con su propio test manual de regresión (repetir estos mismos pasos post-fix).

- [ ] **Step 4: Registrar el resultado**

Actualizar §F de la spec con lo encontrado. Si hubo fix, commitear el código + la actualización de la spec juntos, con mensaje describiendo la causa raíz real (no "fix bug de formulario" genérico).

---

### Task 14: Cerrar items #9 y #15 con la decisión ya documentada

**Files:** `~/Develop/UTN/handoff-2026-08-19-cleanup-completa-y-pendientes-consolidados.md` (no es un repo git — edición directa, sin commit)

La decisión de diferir estos 2 items ya está completamente documentada en `docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md` §G (aprobada por el usuario). Esta tarea solo cierra el loop en el handoff original para que no quede leyéndose como un pendiente abierto indefinidamente.

- [ ] **Step 1: Editar el handoff**

En la sección "9. Baja prioridad / cosmético" (o equivalente), agregar debajo de los items #9 y #15 una línea cada uno:

```
   → Diferido con decisión documentada, no implementado: ver
     docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md §G
     (embolsadora4.0-cloud). Señal para retomar: [la del §G correspondiente].
```

- [ ] **Step 2: No hay paso de verificación ni commit** — este archivo vive fuera de ambos repos git.

---

## Post-plan: PRs

- Backend (`embolsadora4.0-cloud`): al terminar Tasks 1-7, abrir un PR `fix/production-readiness-backend-batch` → `develop`.
- Frontend (`embolsadora-frontend`): al terminar Tasks 8-11, abrir un PR `fix/production-readiness-frontend-batch` → `develop`.
- Tasks 12-14 producen sus propios commits/PRs según lo que encuentren (ver sus Steps finales).
- Ninguna migración de DB nueva en este plan — todos los fixes backend son cambios de código Go (queries embebidas, no schema).
