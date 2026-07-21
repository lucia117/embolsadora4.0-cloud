# Tenant Ownership Scoping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the authorization gap where a tenant-scoped `admin` can read or (if the role map ever changes) mutate any other tenant's record by UUID, since `GetTenant`/`UpdateTenant`/`DeleteTenant`/`GetAllTenants` currently authorize by permission name only, with no check against the actor's own tenant.

**Architecture:** Add a `security.IsCrossTenantRole(role string) bool` allowlist (`super_admin`, `tenant_manager`, `platform_admin`) and use it, together with the already-existing `platform.TenantMatches(ctx, id)` helper (already used by the user-roles handlers for the same class of check), to gate the four tenant-CRUD handlers. Non-cross-tenant roles may only act on their own tenant; `GetAllTenants` returns a single-element list (their own tenant) for them instead of the full table.

**Tech Stack:** Go 1.24, Gin, testify, `internal/security`, `internal/platform`.

## Global Constraints

- Every handler change must preserve the existing `BAD_REQUEST` (400, invalid UUID) and `NOT_FOUND` (404, tenant genuinely absent) behaviors already in place — the new check is an additional early-return, not a replacement.
- Denial responses use `tenantserrors.ErrorResponse{Error: "FORBIDDEN", Message: "No tenés acceso a este tenant", Status: http.StatusForbidden}` — same generic 403 regardless of whether the target tenant exists, per the enumeration-avoidance convention already established in `internal/api/middleware/resolve_tenant_path.go`.
- Cross-tenant roles (`super_admin`, `tenant_manager`, `platform_admin`) keep today's unrestricted behavior on all four endpoints — do not add any check that narrows their access.
- All Go commands run via Docker (Go is not installed on the host):
  `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "<cmd>"`
- Spec: `docs/superpowers/specs/2026-07-20-tenant-ownership-scoping-design.md`

---

### Task 1: `security.IsCrossTenantRole` helper

**Files:**
- Modify: `internal/security/rbac.go`
- Test: `internal/security/rbac_test.go` (new)

**Interfaces:**
- Produces: `func IsCrossTenantRole(roleName string) bool` — later tasks call this exactly.

- [ ] **Step 1: Write the failing test**

Create `internal/security/rbac_test.go`:

```go
package security

import "testing"

func TestIsCrossTenantRole(t *testing.T) {
	cases := []struct {
		role string
		want bool
	}{
		{"super_admin", true},
		{"tenant_manager", true},
		{"platform_admin", true},
		{"admin", false},
		{"operario", false},
		{"cliente_admin", false},
		{"cliente_operario", false},
		{"", false},
		{"unknown_role", false},
	}
	for _, c := range cases {
		if got := IsCrossTenantRole(c.role); got != c.want {
			t.Errorf("IsCrossTenantRole(%q) = %v, want %v", c.role, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/security/... -run TestIsCrossTenantRole -v"`
Expected: FAIL with `undefined: IsCrossTenantRole`

- [ ] **Step 3: Write minimal implementation**

In `internal/security/rbac.go`, add after the `rolePermissions` map (after the closing `}` that ends the map, currently around line 44):

```go
// crossTenantRoles lists roles allowed to act on any tenant, not just their own.
// super_admin and tenant_manager are the DB-seeded global roles; platform_admin
// is the effective role TenantFromHeader assigns to admins of the MRG platform
// tenant acting cross-tenant (see ADR-015).
var crossTenantRoles = map[string]bool{
	"super_admin":    true,
	"tenant_manager": true,
	"platform_admin": true,
}

// IsCrossTenantRole reports whether roleName may act on tenants other than its own.
func IsCrossTenantRole(roleName string) bool {
	return crossTenantRoles[roleName]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/security/... -run TestIsCrossTenantRole -v"`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/security/rbac.go internal/security/rbac_test.go
git commit -m "feat: add security.IsCrossTenantRole helper"
```

---

### Task 2: Scope `GetTenant` to the actor's own tenant

**Files:**
- Modify: `internal/api/handler/tenants/get_tenant/get_tenant.go`
- Test: `internal/api/handler/tenants/get_tenant/get_tenant_test.go` (new)

**Interfaces:**
- Consumes: `security.IsCrossTenantRole(role string) bool` (Task 1), `security.RoleFromContext(ctx) string` (existing), `platform.TenantMatches(ctx, id uuid.UUID) bool` (existing, `internal/platform/tenantctx.go:43`).
- Produces: no new exported symbols; `GetTenantHandler.GetTenant` now returns 403 for cross-tenant reads by non-cross-tenant roles.

- [ ] **Step 1: Write the failing test**

Create `internal/api/handler/tenants/get_tenant/get_tenant_test.go`:

```go
package get_tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// fakeRepo returns tenant (with its ID overwritten to whatever ID was requested)
// for every FindByID call, so tests can focus purely on the scoping check.
type fakeRepo struct {
	tenant *domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if f.tenant == nil {
		return nil, nil
	}
	t := *f.tenant
	t.ID = id
	return &t, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := get_tenant.NewUseCase(&fakeRepo{tenant: &domain.Tenant{Name: "Demo", CreatedAt: time.Now(), UpdatedAt: time.Now()}})
	r := gin.Default()
	h := NewGetTenantHandler(uc)
	r.GET("/api/v1/tenants/:tenantId", h.GetTenant)
	return r
}

func TestGetTenantHandler_OwnTenant_NonGlobalRole_Allowed(t *testing.T) {
	id := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	req = withActorContext(req, "admin", id.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTenantHandler_ForeignTenant_NonGlobalRole_Forbidden(t *testing.T) {
	id := uuid.New()
	otherTenantID := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	req = withActorContext(req, "admin", otherTenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetTenantHandler_ForeignTenant_CrossTenantRole_Allowed(t *testing.T) {
	id := uuid.New()
	actorTenantID := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	req = withActorContext(req, "super_admin", actorTenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetTenantHandler_NoActorContext_Forbidden(t *testing.T) {
	id := uuid.New()
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetTenantHandler_InvalidID(t *testing.T) {
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants/invalid-id", nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/get_tenant/... -v"`
Expected: FAIL — `TestGetTenantHandler_ForeignTenant_NonGlobalRole_Forbidden` and `TestGetTenantHandler_NoActorContext_Forbidden` get 200 instead of 403 (no scoping check exists yet).

- [ ] **Step 3: Write minimal implementation**

Replace `internal/api/handler/tenants/get_tenant/get_tenant.go` in full:

```go
package get_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_tenant"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type GetTenantHandler struct {
	uc *get_tenant.UseCase
}

func NewGetTenantHandler(uc *get_tenant.UseCase) *GetTenantHandler {
	return &GetTenantHandler{
		uc: uc,
	}
}

func (h *GetTenantHandler) GetTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("tenantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	role := security.RoleFromContext(c.Request.Context())
	if !security.IsCrossTenantRole(role) && !platform.TenantMatches(c.Request.Context(), id) {
		c.JSON(http.StatusForbidden, tenantserrors.ErrorResponse{Error: "FORBIDDEN", Message: "No tenés acceso a este tenant", Status: http.StatusForbidden})
		return
	}

	tenant, err := h.uc.Execute(c.Request.Context(), id)
	if err != nil {
		if err == get_tenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, tenantserrors.ErrorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Error al obtener tenant", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenant))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/get_tenant/... -v"`
Expected: PASS (all 5 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/tenants/get_tenant/get_tenant.go internal/api/handler/tenants/get_tenant/get_tenant_test.go
git commit -m "fix: scope GetTenant to actor's own tenant for non-cross-tenant roles"
```

---

### Task 3: Scope `UpdateTenant` to the actor's own tenant

**Files:**
- Modify: `internal/api/handler/tenants/update_tenant/update_tenant.go`
- Modify: `internal/api/handler/tenants/update_tenant/update_tenant_test.go`

**Interfaces:**
- Consumes: same as Task 2 (`security.IsCrossTenantRole`, `security.RoleFromContext`, `platform.TenantMatches`).
- Produces: `UpdateTenantHandler.UpdateTenant` now returns 403 for cross-tenant writes by non-cross-tenant roles.

- [ ] **Step 1: Write the failing test**

The existing `TestUpdateTenantHandler` and `TestUpdateTenantHandler_InvalidID` build requests with no role/tenant context at all. Once Task 3's Step 3 lands, `TestUpdateTenantHandler` would start failing (403 instead of 200) because an empty-context request is treated as a non-matching non-cross-tenant actor — that's the fail-closed behavior we want, but the pre-existing test needs to keep asserting the *update* behavior, not accidentally start asserting the *scoping* behavior. Update it to inject a cross-tenant actor, and add new scoping-focused tests.

Replace `internal/api/handler/tenants/update_tenant/update_tenant_test.go` in full:

```go
package update_tenant

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant/models"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type mockRepo struct{}

func (m *mockRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	return nil
}

func (m *mockRepo) FindAll(ctx context.Context) ([]domain.Tenant, error) {
	return []domain.Tenant{}, nil
}

func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{
		ID:          id,
		Name:        "Demo Tenant",
		CompanyName: "Demo Company",
		Subdomain:   "demo",
		Description: "Demo tenant for testing purposes",
		IsActive:    true,
		Theme: domain.Theme{
			PrimaryColor:    "#3b82f6",
			SecondaryColor:  "#6366f1",
			AccentColor:     "#8b5cf6",
			TextColor:       "#1f2937",
			BackgroundColor: "#ffffff",
			LogoUrl:         "/logos/demo-logo.png",
			FaviconUrl:      "/favicon.ico",
		},
		Address: domain.Address{
			Street:     "123 Main St",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1001",
			Country:    "Argentina",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockRepo) Update(ctx context.Context, tenant *domain.Tenant) error {
	return nil
}

func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	mockRepo := &mockRepo{}
	useCase := ucUpdateTenant.NewUseCase(mockRepo)
	h := NewUpdateTenantHandler(useCase)
	r := gin.Default()
	r.PATCH("/api/v1/tenants/:tenantId", h.UpdateTenant)
	return r
}

func TestUpdateTenantHandler(t *testing.T) {
	r := newTestRouter()

	id := uuid.New().String()
	updateReq := models.TenantUpdateRequest{
		Name:        ptrString("Updated Tenant Name"),
		Description: ptrString("Updated description"),
		IsActive:    ptrBool(true),
		Theme: &models.ThemeUpdate{
			PrimaryColor: ptrString("#4f46e5"),
		},
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PATCH", "/api/v1/tenants/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.TenantResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Tenant Name", resp.Name)
	assert.Equal(t, "Updated description", resp.Description)
	assert.Equal(t, true, resp.IsActive)
	assert.Equal(t, "#4f46e5", resp.Theme.PrimaryColor)
}

func TestUpdateTenantHandler_InvalidID(t *testing.T) {
	r := newTestRouter()

	updateReq := models.TenantUpdateRequest{
		Name: ptrString("Updated Tenant Name"),
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PATCH", "/api/v1/tenants/invalid-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTenantHandler_OwnTenant_NonGlobalRole_Allowed(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()

	updateReq := models.TenantUpdateRequest{Name: ptrString("Updated Tenant Name")}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PATCH", "/api/v1/tenants/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActorContext(req, "admin", id)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateTenantHandler_ForeignTenant_NonGlobalRole_Forbidden(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()
	otherTenantID := uuid.New().String()

	updateReq := models.TenantUpdateRequest{Name: ptrString("Updated Tenant Name")}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PATCH", "/api/v1/tenants/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActorContext(req, "admin", otherTenantID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func ptrString(s string) *string { return &s }
func ptrBool(b bool) *bool       { return &b }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/update_tenant/... -v"`
Expected: FAIL — `TestUpdateTenantHandler_ForeignTenant_NonGlobalRole_Forbidden` gets 200 instead of 403.

- [ ] **Step 3: Write minimal implementation**

In `internal/api/handler/tenants/update_tenant/update_tenant.go`:

Add to the import block (after the `ucUpdateTenant` import):

```go
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
```

Insert immediately after the existing `uuid.Parse` block (right before `var req models.TenantUpdateRequest`):

```go
	role := security.RoleFromContext(c.Request.Context())
	if !security.IsCrossTenantRole(role) && !platform.TenantMatches(c.Request.Context(), id) {
		c.JSON(http.StatusForbidden, tenantserrors.ErrorResponse{Error: "FORBIDDEN", Message: "No tenés acceso a este tenant", Status: http.StatusForbidden})
		return
	}

```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/update_tenant/... -v"`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/tenants/update_tenant/update_tenant.go internal/api/handler/tenants/update_tenant/update_tenant_test.go
git commit -m "fix: scope UpdateTenant to actor's own tenant for non-cross-tenant roles"
```

---

### Task 4: Scope `DeleteTenant` to the actor's own tenant

**Files:**
- Modify: `internal/api/handler/tenants/delete_tenant/delete_tenant.go`
- Test: `internal/api/handler/tenants/delete_tenant/delete_tenant_test.go` (new)

**Interfaces:**
- Consumes: same as Task 2.
- Produces: `DeleteTenantHandler.DeleteTenant` now returns 403 for cross-tenant deletes by non-cross-tenant roles.

- [ ] **Step 1: Write the failing test**

Create `internal/api/handler/tenants/delete_tenant/delete_tenant_test.go`:

```go
package delete_tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type mockRepo struct{}

func (m *mockRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (m *mockRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (m *mockRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{ID: id}, nil
}
func (m *mockRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (m *mockRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	useCase := ucDeleteTenant.NewUseCase(&mockRepo{})
	h := NewDeleteTenantHandler(useCase)
	r := gin.Default()
	r.DELETE("/api/v1/tenants/:tenantId", h.DeleteTenant)
	return r
}

func TestDeleteTenantHandler_CrossTenantRole_Allowed(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/"+id, nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteTenantHandler_OwnTenant_NonGlobalRole_Allowed(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/"+id, nil)
	req = withActorContext(req, "admin", id)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDeleteTenantHandler_ForeignTenant_NonGlobalRole_Forbidden(t *testing.T) {
	r := newTestRouter()
	id := uuid.New().String()
	otherTenantID := uuid.New().String()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/"+id, nil)
	req = withActorContext(req, "admin", otherTenantID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestDeleteTenantHandler_InvalidID(t *testing.T) {
	r := newTestRouter()

	req, _ := http.NewRequest("DELETE", "/api/v1/tenants/invalid-id", nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/delete_tenant/... -v"`
Expected: FAIL — `TestDeleteTenantHandler_ForeignTenant_NonGlobalRole_Forbidden` gets 200 instead of 403.

- [ ] **Step 3: Write minimal implementation**

Replace `internal/api/handler/tenants/delete_tenant/delete_tenant.go` in full:

```go
package delete_tenant

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	ucDeleteTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/delete_tenant"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type DeleteTenantHandler struct {
	useCase ucDeleteTenant.UseCase
}

func NewDeleteTenantHandler(useCase ucDeleteTenant.UseCase) *DeleteTenantHandler {
	return &DeleteTenantHandler{
		useCase: useCase,
	}
}

func (h *DeleteTenantHandler) DeleteTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("tenantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	role := security.RoleFromContext(c.Request.Context())
	if !security.IsCrossTenantRole(role) && !platform.TenantMatches(c.Request.Context(), id) {
		c.JSON(http.StatusForbidden, tenantserrors.ErrorResponse{Error: "FORBIDDEN", Message: "No tenés acceso a este tenant", Status: http.StatusForbidden})
		return
	}

	err = h.useCase.Delete(c.Request.Context(), id)
	if err != nil {
		if err == ucDeleteTenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, tenantserrors.ErrorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		log.Printf("error deleting tenant: %v", err)
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Failed to delete tenant", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tenant deleted successfully",
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/delete_tenant/... -v"`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/tenants/delete_tenant/delete_tenant.go internal/api/handler/tenants/delete_tenant/delete_tenant_test.go
git commit -m "fix: scope DeleteTenant to actor's own tenant for non-cross-tenant roles"
```

---

### Task 5: Scope `GetAllTenants` to the actor's own tenant

**Files:**
- Modify: `internal/api/usecases/tenants/get_all_tenants/get_all_tenants.go`
- Modify: `internal/api/handler/tenants/get_all_tenants/get_all_tenants.go`
- Test: `internal/api/usecases/tenants/get_all_tenants/get_all_tenants_test.go` (new)
- Test: `internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go` (new)

**Interfaces:**
- Consumes: `security.IsCrossTenantRole`, `security.RoleFromContext`, `platform.TenantID(ctx) string` (existing, `internal/platform/tenantctx.go:29`).
- Produces: `UseCase.Execute` signature changes from `Execute(ctx context.Context) ([]domain.Tenant, error)` to `Execute(ctx context.Context, scopeToTenantID *uuid.UUID) ([]domain.Tenant, error)`. No other file in the repo calls this `Execute` (verified via repo-wide grep), so this is the only call site to update besides the handler.

- [ ] **Step 1: Write the failing test — use case layer**

Create `internal/api/usecases/tenants/get_all_tenants/get_all_tenants_test.go`:

```go
package get_all_tenants

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	all []domain.Tenant
	one *domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return f.all, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	if f.one == nil {
		return nil, nil
	}
	t := *f.one
	t.ID = id
	return &t, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func TestExecute_NoScopeReturnsAll(t *testing.T) {
	repo := &fakeRepo{all: []domain.Tenant{{Name: "A"}, {Name: "B"}, {Name: "C"}}}
	uc := NewUseCase(repo)

	result, err := uc.Execute(context.Background(), nil)

	assert.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestExecute_WithScopeReturnsOnlyThatTenant(t *testing.T) {
	scopeID := uuid.New()
	repo := &fakeRepo{
		all: []domain.Tenant{{Name: "A"}, {Name: "B"}},
		one: &domain.Tenant{Name: "Own Tenant"},
	}
	uc := NewUseCase(repo)

	result, err := uc.Execute(context.Background(), &scopeID)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Own Tenant", result[0].Name)
	assert.Equal(t, scopeID, result[0].ID)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/usecases/tenants/get_all_tenants/... -v"`
Expected: FAIL — `uc.Execute(context.Background(), nil)` compile error, `Execute` takes 1 argument in current code.

- [ ] **Step 3: Write minimal implementation — use case layer**

Replace `internal/api/usecases/tenants/get_all_tenants/get_all_tenants.go` in full:

```go
package get_all_tenants

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

type UseCase struct {
	repo tenants.TenantRepository
}

func NewUseCase(repo tenants.TenantRepository) *UseCase {
	return &UseCase{repo: repo}
}

// Execute returns every tenant when scopeToTenantID is nil (cross-tenant roles),
// or a single-element list containing only that tenant otherwise (non-cross-tenant
// roles must not see other tenants' records via the list endpoint).
func (uc *UseCase) Execute(ctx context.Context, scopeToTenantID *uuid.UUID) ([]domain.Tenant, error) {
	if scopeToTenantID == nil {
		return uc.repo.FindAll(ctx)
	}

	tenant, err := uc.repo.FindByID(ctx, *scopeToTenantID)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return []domain.Tenant{}, nil
	}
	return []domain.Tenant{*tenant}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/usecases/tenants/get_all_tenants/... -v"`
Expected: PASS (both tests)

- [ ] **Step 5: Write the failing test — handler layer**

Create `internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go`:

```go
package get_all_tenants

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

type fakeRepo struct{}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error) {
	return []domain.Tenant{{Name: "A"}, {Name: "B"}, {Name: "C"}}, nil
}
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return &domain.Tenant{ID: id, Name: "Own Tenant"}, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func withActorContext(req *http.Request, role, tenantID string) *http.Request {
	ctx := security.WithRole(req.Context(), role)
	ctx = platform.WithTenantID(ctx, tenantID)
	return req.WithContext(ctx)
}

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := get_all_tenants.NewUseCase(&fakeRepo{})
	h := NewGetAllTenantsHandler(uc)
	r := gin.Default()
	r.GET("/api/v1/tenants", h.GetAllTenants)
	return r
}

func TestGetAllTenants_CrossTenantRole_ReturnsFullList(t *testing.T) {
	r := newTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/tenants", nil)
	req = withActorContext(req, "super_admin", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.GetAllTenantsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 3)
}

func TestGetAllTenants_NonGlobalRole_ReturnsOnlyOwnTenant(t *testing.T) {
	r := newTestRouter()
	actorTenantID := uuid.New()

	req, _ := http.NewRequest("GET", "/api/v1/tenants", nil)
	req = withActorContext(req, "admin", actorTenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.GetAllTenantsResponse
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.Equal(t, actorTenantID.String(), resp[0].ID)
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/get_all_tenants/... -v"`
Expected: FAIL — compile error (`h.uc.Execute(ctx)` call inside the handler still uses the old 1-argument signature) and `TestGetAllTenants_NonGlobalRole_ReturnsOnlyOwnTenant` returns all 3 tenants instead of 1.

- [ ] **Step 7: Write minimal implementation — handler layer**

Replace `internal/api/handler/tenants/get_all_tenants/get_all_tenants.go` in full:

```go
package get_all_tenants

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_all_tenants/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_all_tenants"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// GetAllTenantsHandler maneja las solicitudes para obtener todos los tenants
type GetAllTenantsHandler struct {
	uc *get_all_tenants.UseCase
}

// NewGetAllTenantsHandler crea una nueva instancia del handler
func NewGetAllTenantsHandler(uc *get_all_tenants.UseCase) *GetAllTenantsHandler {
	return &GetAllTenantsHandler{
		uc: uc,
	}
}

// GetAllTenants obtiene todos los tenants para roles cross-tenant, o únicamente
// el tenant propio para roles no-globales.
func (h *GetAllTenantsHandler) GetAllTenants(c *gin.Context) {
	role := security.RoleFromContext(c.Request.Context())

	var scopeToTenantID *uuid.UUID
	if !security.IsCrossTenantRole(role) {
		actorTenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Tenant context inválido", Status: http.StatusInternalServerError})
			return
		}
		scopeToTenantID = &actorTenantID
	}

	tenants, err := h.uc.Execute(c.Request.Context(), scopeToTenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Error al obtener tenants", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenants))
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go test ./internal/api/handler/tenants/get_all_tenants/... ./internal/api/usecases/tenants/get_all_tenants/... -v"`
Expected: PASS (all tests, both packages)

- [ ] **Step 9: Commit**

```bash
git add internal/api/usecases/tenants/get_all_tenants/get_all_tenants.go internal/api/usecases/tenants/get_all_tenants/get_all_tenants_test.go internal/api/handler/tenants/get_all_tenants/get_all_tenants.go internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go
git commit -m "fix: scope GetAllTenants to actor's own tenant for non-cross-tenant roles"
```

---

### Task 6: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run every touched package's test suite together**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/security/... ./internal/api/handler/tenants/... ./internal/api/usecases/tenants/get_all_tenants/... -v"
```
Expected: PASS, no failures, no skipped tests.

- [ ] **Step 2: Confirm the whole module still builds**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go build ./..."`
Expected: exits 0, no output.

- [ ] **Step 3: Vet the changed packages**

Run: `docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "go vet ./internal/security/... ./internal/api/handler/tenants/... ./internal/api/usecases/tenants/get_all_tenants/..."`
Expected: exits 0, no output.

- [ ] **Step 4: Manually confirm against the spec**

Re-read `docs/superpowers/specs/2026-07-20-tenant-ownership-scoping-design.md` and confirm every bullet in its "Design" section (4 numbered changes) has a corresponding completed task above. No commit needed for this step — it's a checklist, not a code change.

---

## Self-Review Notes

- **Spec coverage**: Design items 1–4 map to Tasks 1, 2/3/4, 5, and the "cross-tenant roles unchanged" note is covered by the `_CrossTenantRole_Allowed` test in each of Tasks 2, 3 (implicit via the pre-existing test now using `super_admin`), 4, and 5.
- **Placeholder scan**: none — every step has runnable code and exact commands.
- **Type consistency**: `security.IsCrossTenantRole(roleName string) bool` (Task 1) is called identically in Tasks 2, 3, 4, 5. `platform.TenantMatches(ctx context.Context, tenantID uuid.UUID) bool` and `platform.TenantID(ctx context.Context) string` are pre-existing, unchanged. `get_all_tenants.UseCase.Execute` signature is defined once (Task 5) and its only two callers (its own test and the handler, both in Task 5) use the new signature.
