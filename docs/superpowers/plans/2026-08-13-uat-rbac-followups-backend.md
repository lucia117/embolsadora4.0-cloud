# UAT RBAC Follow-ups (Backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Arreglar 3 bugs de la ronda de UAT de `embolsadora-frontend` que tienen causa raíz en este repo: B-004 (403 al crear rol/permiso custom para admin de tenant cliente), B-002 (403 al editar cuenta propia), B-006 (roles "Admin"/"Operario" visibles y asignables fuera del tenant plataforma MRG).

**Architecture:** Un cambio de una línea en el mapa de permisos (B-004), un endpoint HTTP nuevo sin RBAC que resuelve identidad desde el JWT en vez de la URL (B-002), y una migración chica que extiende una función SQL ya existente (`tenant_can_use_role`, migración 000004) para que además de `is_global` conozca los roles platform-only `admin`/`operario` (B-006). Spec completo, incluyendo el diseño del frontend relacionado (mismo backlog), en `embolsadora-frontend/docs/superpowers/specs/2026-08-13-uat-rbac-followups-design.md`.

**Tech Stack:** Go 1.24, Gin, pgx/v5, PostgreSQL, testify, golang-migrate.

## Global Constraints

- Rama: `fix/uat-role-scoping` contra `develop`.
- Go no está instalado en el host — todo comando `go`/`migrate` corre vía Docker, ver `CLAUDE.md` de este repo para los comandos exactos (`docker run ... golang:1.24-alpine sh -c "go test ./..."`).
- Antes de abrir el PR: `go build ./...` y `go test ./...` (vía Docker) deben pasar limpios. Los tests que requieren `DATABASE_URL` se skipean automáticamente si no está seteada (`t.Skip("DATABASE_URL not set")`) — correrlos de verdad contra una base de test antes de mergear.
- No se toca ningún otro rol (`operario`, `cliente_operario`, `super_admin`, `tenant_manager`, `platform_admin`) fuera de lo que cada tarea indica explícitamente.

---

### Task 1: B-004 — dar `users:write` a `cliente_admin`

**Files:**
- Modify: `internal/security/rbac.go:42`
- Test: `internal/security/rbac_test.go`

**Interfaces:**
- Consumes: nada nuevo.
- Produces: `PermissionsForRole("cliente_admin")` (ya exportada, sin cambio de firma) pasa a incluir `"users:write"`.

- [ ] **Step 1: Escribir el test que falla**

Agregar al final de `internal/security/rbac_test.go` (mismo estilo que `TestIsCrossTenantRole`, sin testify — este archivo usa `testing` puro):

```go
func TestClienteAdminTieneUsersWrite(t *testing.T) {
	perms := PermissionsForRole("cliente_admin")
	found := false
	for _, p := range perms {
		if p == "users:write" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("cliente_admin debería tener users:write (para crear roles/permisos custom de su tenant), perms=%v", perms)
	}
}
```

- [ ] **Step 2: Correr el test y confirmar que falla**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/security/... -run TestClienteAdminTieneUsersWrite -v"`
Expected: FAIL — `cliente_admin debería tener users:write ... perms=[users:read invitations:write machines:read]`

- [ ] **Step 3: Aplicar el fix**

En `internal/security/rbac.go`, línea 42:

```diff
- "cliente_admin":    {"users:read", "invitations:write", "machines:read"},
+ "cliente_admin":    {"users:read", "users:write", "invitations:write", "machines:read"},
```

- [ ] **Step 4: Correr el test y confirmar que pasa**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/security/... -v"`
Expected: PASS, todos los tests de `internal/security` en verde (confirmar que no rompiste ningún test existente del mismo archivo).

- [ ] **Step 5: Commit**

```bash
git add internal/security/rbac.go internal/security/rbac_test.go
git commit -m "fix: dar users:write a cliente_admin (B-004)

Sin este permiso, POST /roles y POST /permissions devolvían 403 para un
admin de tenant cliente aunque el producto espera que sí pueda gestionar
sus propios roles/permisos custom."
```

---

### Task 2: B-002 — endpoint dedicado `PATCH /api/v1/users/me`

**Files:**
- Modify: `internal/api/handler/users/dto/update.go`
- Modify: `internal/api/handler/users/handler.go`
- Modify: `internal/routes/url_mappings.go`
- Test: `internal/api/handler/users/update_me_test.go` (nuevo)

**Interfaces:**
- Consumes: `platform.DomainUser(ctx) DomainUserValue` (cast a `*domain.User`, campo `.ID`), `platform.TenantID(ctx) string`, `security.CanSeePlatformInternals(ctx) bool` — los tres ya existentes, sin cambios. `users.Service.UpdateUser(ctx, tenantID, userID string, includeGlobal bool, cmd *domainUsers.UpdateUserCommand) (*domainUsers.User, error)` — ya existente, sin cambios de firma.
- Produces: handler `(h *Handler) UpdateMe(c *gin.Context)`, registrado en `PATCH /api/v1/users/me` sin `RBACCheck`.

- [ ] **Step 1: Agregar el DTO nuevo**

En `internal/api/handler/users/dto/update.go`, agregar al final del archivo (después de `UpdateUserResponse`):

```go
// UpdateMeRequest represents a self-service profile update request.
// Deliberadamente no tiene campo Role: a diferencia de UpdateUserRequest (que sí lo
// tiene, protegido por RBAC + EnsureAssignable), este DTO alimenta un endpoint sin
// RBAC — la ausencia del campo, no una validación, es lo que impide la escalada.
type UpdateMeRequest struct {
	FirstName *string `json:"firstName" binding:"omitempty,max=100"`
	LastName  *string `json:"lastName" binding:"omitempty,max=100"`
}
```

- [ ] **Step 2: Escribir el test que falla**

Crear `internal/api/handler/users/update_me_test.go`:

```go
package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	"github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// fakeUserRepo es un doble mínimo de users.Repository. UpdateMe solo ejercita
// GetByID y Update — el resto de los métodos existen únicamente para satisfacer
// la interfaz.
type fakeUserRepo struct {
	stored *domainUsers.User
}

func (f *fakeUserRepo) ListByTenant(ctx context.Context, tenantID string, limit, offset int, includeGlobal bool) ([]*domainUsers.User, int64, error) {
	return nil, 0, nil
}
func (f *fakeUserRepo) GetByID(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.User, error) {
	if f.stored == nil || f.stored.ID != userID {
		return nil, domainUsers.ErrNotFound
	}
	cp := *f.stored
	return &cp, nil
}
func (f *fakeUserRepo) GetByIDWithRoles(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	return nil, domainUsers.ErrNotFound
}
func (f *fakeUserRepo) ListPendingByTenant(ctx context.Context, tenantID string, includeGlobal bool) ([]*domainUsers.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Create(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUserRepo) CreateWithRole(ctx context.Context, user *domainUsers.User, utr *domain.UserTenantRole) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUserRepo) Update(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	f.stored = user
	return user, nil
}
func (f *fakeUserRepo) Delete(ctx context.Context, tenantID, userID string) error { return nil }

func newTestRouterForUpdateMe(repo *fakeUserRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := appUsers.NewService(repo, nil, nil, zap.NewNop())
	h := NewHandler(svc, zap.NewNop())
	r := gin.New()
	r.PATCH("/api/v1/users/me", h.UpdateMe)
	return r
}

func withDomainUserAndTenant(req *http.Request, userID, tenantID string) *http.Request {
	ctx := platform.WithDomainUser(req.Context(), &domain.User{ID: userID})
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

// TestUpdateMe_SinPermisoRBAC_Funciona es el regression test de B-002: antes,
// este flujo pegaba a PATCH /users/:id (gateado por RBACCheck("users:write")) y
// devolvía 403 para cualquier rol sin ese permiso. Esta ruta no tiene RBACCheck
// en absoluto — el test lo prueba registrando SOLO el handler, sin ningún
// middleware de por medio, y confirma que igual funciona.
func TestUpdateMe_SinPermisoRBAC_Funciona(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111111"
	repo := &fakeUserRepo{stored: &domainUsers.User{ID: userID, FirstName: "Viejo", LastName: "Nombre"}}
	r := newTestRouterForUpdateMe(repo)

	body, _ := json.Marshal(map[string]string{"firstName": "Nuevo", "lastName": "Apellido"})
	req, _ := http.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDomainUserAndTenant(req, userID, "22222222-2222-2222-2222-222222222222")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Nuevo", repo.stored.FirstName)
	assert.Equal(t, "Apellido", repo.stored.LastName)
}

// TestUpdateMe_IgnoraCampoRoleEnElBody confirma que aunque un cliente mande
// "role" en el JSON (UpdateUserRequest sí lo acepta, UpdateMeRequest no), el
// campo se ignora silenciosamente — no hay forma de que llegue a
// UpdateUserCommand.Role porque el DTO no lo tiene.
func TestUpdateMe_IgnoraCampoRoleEnElBody(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111111"
	repo := &fakeUserRepo{stored: &domainUsers.User{ID: userID, Role: "operario"}}
	r := newTestRouterForUpdateMe(repo)

	body, _ := json.Marshal(map[string]string{"firstName": "X", "role": "super_admin"})
	req, _ := http.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withDomainUserAndTenant(req, userID, "22222222-2222-2222-2222-222222222222")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "operario", repo.stored.Role, "role no debe cambiar vía este endpoint")
}

func TestUpdateMe_SinDomainUserEnContexto_Unauthorized(t *testing.T) {
	r := newTestRouterForUpdateMe(&fakeUserRepo{})

	body, _ := json.Marshal(map[string]string{"firstName": "X"})
	req, _ := http.NewRequest("PATCH", "/api/v1/users/me", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 3: Correr el test y confirmar que falla**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go build ./... && go test ./internal/api/handler/users/... -run TestUpdateMe -v"`
Expected: falla en `go build` — `h.UpdateMe` no existe todavía (`undefined: (*Handler).UpdateMe`).

- [ ] **Step 4: Implementar el handler**

En `internal/api/handler/users/handler.go`, agregar el import de `domain` (el paquete raíz, para el type assertion — no confundir con `domainUsers`, ya importado):

```diff
 import (
 	"net/http"
 	"strconv"

 	"github.com/gin-gonic/gin"
 	"go.uber.org/zap"

 	"github.com/tu-org/embolsadora-api/internal/api/handler/users/dto"
 	"github.com/tu-org/embolsadora-api/internal/app/users"
+	"github.com/tu-org/embolsadora-api/internal/domain"
 	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
 	"github.com/tu-org/embolsadora-api/internal/platform"
 	"github.com/tu-org/embolsadora-api/internal/security"
 )
```

Y agregar el handler nuevo, justo antes de `// UpdateUser handles PATCH /api/v1/users/:id`:

```go
// UpdateMe handles PATCH /api/v1/users/me — self-service profile update
// (firstName/lastName propios). A diferencia de UpdateUser, no requiere RBAC:
// el userID sale del JWT vía platform.DomainUser(ctx), nunca del cliente, así
// que no hay forma de apuntar a otro usuario. dto.UpdateMeRequest tampoco tiene
// campo Role, así que tampoco hay forma de auto-asignarse un rol distinto.
func (h *Handler) UpdateMe(c *gin.Context) {
	tenantID := platform.TenantID(c.Request.Context())

	user, ok := platform.DomainUser(c.Request.Context()).(*domain.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "UNAUTHORIZED",
			Message: "No se pudo resolver el usuario autenticado",
			Status:  http.StatusUnauthorized,
		})
		return
	}

	var req dto.UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid update me request", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "VALIDATION_ERROR",
			Message: err.Error(),
			Status:  http.StatusBadRequest,
		})
		return
	}

	cmd := &domainUsers.UpdateUserCommand{
		TenantID:  tenantID,
		UserID:    user.ID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	includeGlobal := security.CanSeePlatformInternals(c.Request.Context())
	updated, err := h.service.UpdateUser(c.Request.Context(), tenantID, user.ID, includeGlobal, cmd)
	if err != nil {
		h.logger.Error("update me failed", zap.Error(err))
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, userToResponse(updated))
}
```

- [ ] **Step 5: Correr el test y confirmar que pasa**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/users/... -run TestUpdateMe -v"`
Expected: los 3 tests nuevos en PASS.

- [ ] **Step 6: Registrar la ruta**

En `internal/api/router.go`, insertar la línea nueva justo antes de `PATCH /users/:id` (mismo motivo que el comentario existente sobre `/users/pending` — una ruta estática debe registrarse antes que su hermana con param):

```diff
 	// Write operations (admin only)
 	userRoutes.POST("/users", middleware.RBACCheck("users:write"), uh.CreateUser)
+	// Self-service: sin RBAC, el userID sale del JWT (uh.UpdateMe), nunca de la URL.
+	// Registrada antes de "/users/:id" — mismo motivo que /users/pending arriba.
+	userRoutes.PATCH("/users/me", uh.UpdateMe)
 	userRoutes.PATCH("/users/:id", middleware.RBACCheck("users:write"), uh.UpdateUser)
```

- [ ] **Step 7: Build completo y suite completa**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go build ./... && go vet ./... && go test ./internal/api/... -v"`
Expected: build y vet limpios, todos los tests de `internal/api/...` en PASS (confirmar que no rompiste ningún test existente del handler de users ni del router).

- [ ] **Step 8: Commit**

```bash
git add internal/api/handler/users/dto/update.go internal/api/handler/users/handler.go internal/api/handler/users/update_me_test.go internal/api/router.go
git commit -m "fix: endpoint dedicado PATCH /api/v1/users/me para autoedición (B-002)

PATCH /users/:id exige users:write sin excepción, así que operario,
cliente_admin y cliente_operario no podían editar ni su propio nombre.
El endpoint nuevo resuelve el userID desde el JWT (nunca de la URL) y
su DTO no tiene campo role, así que no hay bypass de RBAC posible ni
superficie para autoasignarse un rol distinto."
```

---

### Task 3: B-006 — `admin`/`operario` dejan de ser asignables/visibles fuera del tenant plataforma

**Files:**
- Create: `migrations/000010_platform_only_roles.up.sql`
- Create: `migrations/000010_platform_only_roles.down.sql`
- Modify: `internal/repo/pg/roles/repository.go`
- Modify: `internal/repo/pg/user_roles/repository.go`
- Test: `internal/repo/pg/roles/repository_test.go`

**Interfaces:**
- Consumes: función SQL `tenant_can_use_role` (cambia de firma, ver abajo) — solo la usan los dos archivos de este Task, no hay más call sites en el código Go (confirmado por `grep -rn "tenant_can_use_role" --include="*.go"`).
- Produces: `tenant_can_use_role(p_tenant_id uuid, p_role_id text) RETURNS boolean` (antes `p_is_global boolean`). Sin cambios en las firmas Go (`roles.Repository.List`, `GetByIDForTenant`, `user_roles` `checkRoleAllowedForTenant` mantienen exactamente los mismos parámetros — solo cambia qué columna le pasan a la función SQL en el query).

- [ ] **Step 1: Escribir los tests que fallan**

En `internal/repo/pg/roles/repository_test.go`, agregar al final del archivo:

```go
// TestListOcultaRolesPlatformOnlyEnTenantCliente es el regression test de
// B-006: admin/operario son is_global=false (a diferencia de super_admin/
// tenant_manager), así que antes del fix tenant_can_use_role los dejaba pasar
// en cualquier tenant. Ahora deben comportarse igual que los roles is_global:
// invisibles fuera del tenant plataforma.
func TestListOcultaRolesPlatformOnlyEnTenantCliente(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	clientTenant := createTestTenant(t, pool)

	roles, err := repo.List(context.Background(), clientTenant, false)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.NotContains(t, ids, "admin", "admin es platform-only, no debe verse fuera de MRG")
	require.NotContains(t, ids, "operario", "operario es platform-only, no debe verse fuera de MRG")
	require.Contains(t, ids, "cliente_admin", "los roles de tenant cliente siguen visibles")
	require.Contains(t, ids, "cliente_operario")
}

// TestListMuestraRolesPlatformOnlyEnTenantPlataforma confirma que el fix no
// rompe el caso positivo: dentro de MRG, admin/operario siguen visibles.
func TestListMuestraRolesPlatformOnlyEnTenantPlataforma(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)

	roles, err := repo.List(context.Background(), platformTenantUUID, false)
	require.NoError(t, err)

	ids := roleIDs(roles)
	require.Contains(t, ids, "admin")
	require.Contains(t, ids, "operario")
}

// TestGetByIDForTenantOcultaAdminEnTenantCliente es el equivalente para
// GetByIDForTenant (la validación que usa EnsureAssignable en el camino de
// asignación de roles, no solo el listado).
func TestGetByIDForTenantOcultaAdminEnTenantCliente(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	clientTenant := createTestTenant(t, pool)

	_, err := repo.GetByIDForTenant(context.Background(), "admin", clientTenant, false)
	require.ErrorIs(t, err, domain.ErrRoleNotFound)

	role, err := repo.GetByIDForTenant(context.Background(), "cliente_admin", clientTenant, false)
	require.NoError(t, err)
	require.Equal(t, "cliente_admin", role.ID)
}
```

- [ ] **Step 2: Correr los tests y confirmar que fallan**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL=$DATABASE_URL golang:1.24-alpine sh -c "go test ./internal/repo/pg/roles/... -run 'TestListOcultaRolesPlatformOnlyEnTenantCliente|TestGetByIDForTenantOcultaAdminEnTenantCliente' -v"`

(Si no tenés `DATABASE_URL` a mano localmente, correr esto contra la base de Supabase de UAT — ver memoria del proyecto para la connection string en `~/.supabase-db-url`, **de solo lectura para este paso**, y aplicar la migración real más adelante solo cuando el usuario lo confirme explícitamente, igual que cualquier otro cambio de schema.)

Expected: `TestListOcultaRolesPlatformOnlyEnTenantCliente` FAIL (`admin`/`operario` aparecen en `ids`), `TestGetByIDForTenantOcultaAdminEnTenantCliente` FAIL (no da `ErrRoleNotFound`, encuentra el rol).

- [ ] **Step 3: Crear la migración**

`migrations/000010_platform_only_roles.up.sql`:

```sql
-- ============================================================================
-- Migration 000010: extiende tenant_can_use_role (migración 000004) para que
-- también trate admin/operario como platform-only, no solo is_global=true.
-- ============================================================================
-- admin/operario son is_global=FALSE (a diferencia de super_admin/tenant_manager),
-- así que tenant_can_use_role(tenant_id, is_global) los dejaba pasar en cualquier
-- tenant. Un admin de un tenant cliente podía ver y asignar el rol "admin" a
-- usuarios de su propio tenant — ese rol no da acceso cross-tenant (EffectiveRole
-- solo lo promueve a platform_admin dentro del tenant plataforma), pero sí
-- permisos de más (tenants:read, machines:write) que cliente_admin no tiene.
--
-- Se mantiene una sola función como fuente de verdad (mismo objetivo que la
-- migración 000004): en vez de agregar un segundo check en Go, esta función pasa
-- a recibir el role_id en vez de is_global, y resuelve is_global internamente.
-- ============================================================================

CREATE OR REPLACE FUNCTION tenant_can_use_role(p_tenant_id uuid, p_role_id text) RETURNS boolean AS $$
    SELECT
        (
            NOT COALESCE((SELECT is_global FROM roles WHERE id = p_role_id), FALSE)
            AND p_role_id NOT IN ('admin', 'operario')
        )
        OR EXISTS (
            SELECT 1 FROM tenants t WHERE t.id = p_tenant_id AND t.is_platform_tenant = TRUE
        );
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION enforce_platform_role_tenant() RETURNS trigger AS $$
BEGIN
    IF NEW.role_id IS NULL THEN
        RETURN NEW;
    END IF;

    IF NOT tenant_can_use_role(NEW.tenant_id, NEW.role_id) THEN
        RAISE EXCEPTION 'role "%" is reserved for the platform tenant and cannot be assigned in tenant %', NEW.role_id, NEW.tenant_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

`migrations/000010_platform_only_roles.down.sql`:

```sql
-- Revierte a la definición original de la migración 000004.

CREATE OR REPLACE FUNCTION tenant_can_use_role(p_tenant_id uuid, p_is_global boolean) RETURNS boolean AS $$
    SELECT (NOT p_is_global) OR EXISTS (
        SELECT 1 FROM tenants t WHERE t.id = p_tenant_id AND t.is_platform_tenant = TRUE
    );
$$ LANGUAGE sql STABLE;

CREATE OR REPLACE FUNCTION enforce_platform_role_tenant() RETURNS trigger AS $$
DECLARE
    v_is_global boolean;
BEGIN
    IF NEW.role_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT is_global INTO v_is_global FROM roles WHERE id = NEW.role_id;
    IF v_is_global IS NOT TRUE THEN
        RETURN NEW;
    END IF;

    IF NOT tenant_can_use_role(NEW.tenant_id, TRUE) THEN
        RAISE EXCEPTION 'role "%" is reserved for the platform tenant and cannot be assigned in tenant %', NEW.role_id, NEW.tenant_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

- [ ] **Step 4: Actualizar los dos call sites Go**

En `internal/repo/pg/roles/repository.go`, método `List` (query completa):

```diff
 	query := `
 		SELECT id, name, description, is_system_role, is_global, tenant_id, permissions, created_at, updated_at
 		FROM roles
 		WHERE (tenant_id = $1 OR tenant_id IS NULL)
 		  AND deleted_at IS NULL
-		  AND tenant_can_use_role($1, is_global)
+		  AND tenant_can_use_role($1, id)
 		  AND (NOT is_global OR $2)
 		ORDER BY is_system_role DESC, name ASC
 	`
```

Y en `GetByIDForTenant`:

```diff
 	query := `
 		SELECT id, name, description, is_system_role, is_global, tenant_id, permissions, created_at, updated_at
 		FROM roles
 		WHERE id = $1
 		  AND deleted_at IS NULL
 		  AND (tenant_id = $2 OR tenant_id IS NULL)
-		  AND tenant_can_use_role($2, is_global)
+		  AND tenant_can_use_role($2, id)
 		  AND (NOT is_global OR $3)
 	`
```

En `internal/repo/pg/user_roles/repository.go`, método `checkRoleAllowedForTenant`:

```diff
 	err := r.db.QueryRow(ctx, `
-		SELECT tenant_can_use_role($2, r.is_global)
+		SELECT tenant_can_use_role($2, r.id)
 		FROM roles r
 		WHERE r.id = $1
 		  AND r.deleted_at IS NULL
 		  AND (r.tenant_id = $2 OR r.tenant_id IS NULL)
 		  AND (NOT r.is_global OR $3)
 	`, roleID, tenantID, includeGlobal).Scan(&allowed)
```

- [ ] **Step 5: Aplicar la migración localmente y correr los tests**

Run: `migrate -path migrations/ -database "$DATABASE_URL" up`
Expected: aplica `000010` sin error (confirmar con `SELECT version, dirty FROM schema_migrations;` que quedó en `10, false`).

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL=$DATABASE_URL golang:1.24-alpine sh -c "go test ./internal/repo/pg/... -v"`
Expected: los 3 tests nuevos en PASS, y el resto de `internal/repo/pg/roles` y `internal/repo/pg/user_roles` siguen en PASS (en particular `TestListOcultaRolesGlobalesAlNoSuperadmin`, `TestListMuestraRolesGlobalesAlSuperadmin` y `TestGetByIDForTenantDevuelveNotFoundParaRolOculto`, que cubren el comportamiento que ya existía y no debe romperse).

- [ ] **Step 6: Probar el rollback**

Run: `migrate -path migrations/ -database "$DATABASE_URL" down 1`
Expected: revierte sin error. Volver a aplicar con `up` antes de seguir (dejar la base en el estado esperado para el resto del trabajo).

- [ ] **Step 7: Build completo**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go build ./... && go vet ./..."`
Expected: limpio.

- [ ] **Step 8: Commit**

```bash
git add migrations/000010_platform_only_roles.up.sql migrations/000010_platform_only_roles.down.sql \
        internal/repo/pg/roles/repository.go internal/repo/pg/roles/repository_test.go \
        internal/repo/pg/user_roles/repository.go
git commit -m "fix: admin/operario dejan de ser asignables/visibles fuera de MRG (B-006)

tenant_can_use_role (migración 000004) solo restringía roles is_global=true
(super_admin, tenant_manager). admin/operario son is_global=false pero
igual de platform-only en la práctica, así que quedaban visibles en
GET /roles y asignables vía API para cualquier tenant cliente. Se extiende
la misma función (única fuente de verdad, usada por List/GetByIDForTenant
y por el trigger de la DB) en vez de duplicar el check en Go."
```

---

### Task 4: Verificación final y preparación del PR

**Files:** ninguno nuevo — solo verificación.

- [ ] **Step 1: Suite completa**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL=$DATABASE_URL golang:1.24-alpine sh -c "go build ./... && go vet ./... && go test ./... -v"`
Expected: todo en PASS. Confirmar que ningún test de `internal/app/roles`, `internal/app/users`, `internal/api/handler/roles`, `internal/api/handler/users` ni `internal/security` quedó roto por los tres cambios.

- [ ] **Step 2: Checklist manual de regresión**

Con el servidor corriendo localmente (`APP_BASE_URL`/`.env` apuntando a la base con la migración `000010` aplicada) y los usuarios de prueba de `embolsadora-frontend/docs/uat/review-e2e-integracion-admin-rbac.md`:

- MO-07 / CA-11 / CO-07: `PATCH /api/v1/users/me` con `{"firstName": "..."}` para operario MRG, admin Cordoba, operario Cordoba → 200 (antes 403).
- CA-05 / CA-07: admin Cordoba, `POST /api/v1/roles` y `POST /api/v1/permissions` → 200/201 (antes 403).
- CA-02 / CA-04: admin Cordoba, `GET /api/v1/roles` → ya no incluye `admin` ni `operario` en la respuesta.
- Armar a mano un `POST /api/v1/user-roles` (o `/invitations`) con `role_id: "admin"` desde el tenant Cordoba → 403 `ErrRoleNotAllowedForTenant` (antes lo permitía).
- Confirmar que un `admin` de MRG (tenant plataforma) sigue viendo y pudiendo asignar `admin`/`operario` sin cambios.

- [ ] **Step 3: Actualizar el backlog del otro repo**

En `embolsadora-frontend/docs/uat/backlog-e2e-integracion-admin-rbac.md`, marcar B-002, B-004, B-006 como resueltos (agregar la línea de commit/rama debajo de cada uno). Este archivo vive en el otro repo — hacer el commit ahí, no en `embolsadora4.0-cloud`.

- [ ] **Step 4: Dejar la rama lista para PR (no abrirlo todavía)**

Run: `git log --oneline develop..fix/uat-role-scoping`
Expected: 3 commits (uno por tarea), historia limpia. Confirmar con el usuario antes de `git push` / `gh pr create`, y coordinar el orden de deploy con el frontend: Task 4 del plan de frontend (BFF de `/users/me`) depende de que este PR esté mergeado y desplegado primero.
