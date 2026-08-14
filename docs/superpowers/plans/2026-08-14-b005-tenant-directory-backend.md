# B-005 Tenant Directory — Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose a session-less endpoint to resolve a tenant by UUID or subdomain, so the frontend can stop depending on a static `tenants.json` file for tenant existence checks (spec: `embolsadora-frontend/docs/superpowers/specs/2026-08-14-b005-tenant-directory-design.md`).

**Architecture:** Add `FindBySubdomain` to the existing `TenantRepository` (repo layer already has `FindByID`; subdomain lookup is new). Add a new usecase + handler + trimmed response DTO, registered directly on the root `gin.Engine` (bypassing the `/api/v1` group's `JWTAuth`/`TenantFromHeader` middleware chain — same pattern already used by `POST /api/v1/auth/login`).

**Tech Stack:** Go 1.24, Gin, pgx/v5, testify (all commands run via Docker per `CLAUDE.md` — Go is not installed on the host).

## Global Constraints

- No Go toolchain on macOS host — every `go` command below runs via the Docker one-liners documented in `CLAUDE.md`.
- New route returns **exactly** these tenant fields, nothing else: `id, subdomain, name, companyName, isActive, theme{primaryColor,secondaryColor,accentColor,backgroundColor,textColor,logoUrl,faviconUrl}, settings{locale,timezone,dateFormat,timeFormat,currency}`. Never `address`, `contactEmail`, `companyWebsite`, `description`.
- 404 uniformly for "does not exist" and "exists but `isActive=false`" — never distinguish the two in the response.
- **Deviation from the approved spec, flagged here rather than silently applied:** the spec calls for reusing "el middleware Redis existente" for rate limiting on this endpoint. That middleware doesn't exist — `internal/consumers/middleware/middleware.go:9` has `func RateLimit() gin.HandlerFunc { return func(c *gin.Context) { /* TODO */ c.Next() } }`, a no-op stub, and no other Redis-backed per-route rate limiter exists anywhere in this codebase (the only real Redis rate limiting is `InviteRateLimitHour`, which is invitation-email-specific, not a generic HTTP middleware). `POST /api/v1/auth/login` — the one other unauthenticated route — also ships with zero rate limiting today. This plan does **not** build new rate-limiting infrastructure to match that line of the spec; it follows the existing precedent (no rate limit on public routes) instead. Flag to the user if this needs to be revisited.

---

## Task 1: `FindBySubdomain` on the tenant repository

**Files:**
- Modify: `internal/repo/pg/tenants/resources.go`
- Modify: `internal/repo/pg/tenants/repository.go`
- Modify: `internal/api/handler/tenants/get_tenant/get_tenant_test.go:21-36` (fake needs the new method)
- Modify: `internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go:20-30` (fake needs the new method)
- Modify: `internal/api/handler/tenants/delete_tenant/delete_tenant_test.go:18-26` (fake needs the new method)
- Modify: `internal/api/handler/tenants/update_tenant/update_tenant_test.go:22-74` (fake needs the new method)
- Modify: `internal/api/usecases/tenants/get_all_tenants/get_all_tenants_test.go:12-28` (fake needs the new method)
- Test: `internal/repo/pg/tenants/repository_test.go`

**Interfaces:**
- Consumes: nothing new — extends the existing `domain.Tenant` (`internal/domain/tenants.go`) and `pgxpool.Pool`.
- Produces: `TenantRepository.FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error)` — same nil-on-not-found / error-on-DB-failure contract as the existing `FindByID`. Task 2 consumes this.

- [ ] **Step 1: Write the failing integration test**

Add to `internal/repo/pg/tenants/repository_test.go` (same file as `TestSettings_RoundTrip`, same `DATABASE_URL` skip guard):

```go
func TestFindBySubdomain_RoundTrip(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	ctx := context.Background()

	tenant := newTestTenant()
	require.NoError(t, repo.Create(ctx, tenant))
	defer func() { _ = repo.Delete(ctx, tenant.ID) }()

	found, err := repo.FindBySubdomain(ctx, tenant.Subdomain)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, tenant.ID, found.ID)
	assert.Equal(t, tenant.Subdomain, found.Subdomain)
}

func TestFindBySubdomain_NotFound(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	found, err := repo.FindBySubdomain(context.Background(), "no-such-subdomain-"+uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, found)
}
```

- [ ] **Step 2: Run the test to verify it fails to compile**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$DATABASE_URL" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/tenants/... -run TestFindBySubdomain -v"
```
Expected: compile error, `repo.FindBySubdomain undefined`.

- [ ] **Step 3: Add the query**

In `internal/repo/pg/tenants/resources.go`, add after `FindByIDQuery` (after line 14):

```go
	// FindBySubdomainQuery retrieves a tenant by subdomain with all related data
	FindBySubdomainQuery = `
		SELECT
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			contact_email, company_website, locale, timezone, date_format, time_format, currency,
			created_at, updated_at
		FROM tenants
		WHERE subdomain = $1
	`
```

- [ ] **Step 4: Add the interface method and implementation**

In `internal/repo/pg/tenants/repository.go`, add `FindBySubdomain` to the `TenantRepository` interface (after `FindByID` on line 17):

```go
	FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error)
```

Add the implementation after the existing `FindByID` method (after line 68) — identical scan logic, different query and param type:

```go
func (r *tenantRepository) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	var theme domain.Theme
	var address domain.Address
	var settings domain.TenantSettings
	var tenantID uuid.UUID

	var description, logoUrl, faviconUrl *string
	var street, city, state, postalCode, country *string

	err := r.db.QueryRow(ctx, FindBySubdomainQuery, subdomain).Scan(
		&tenantID, &tenant.Name, &tenant.CompanyName, &tenant.Subdomain, &description, &tenant.IsActive,
		&theme.PrimaryColor, &theme.SecondaryColor, &theme.AccentColor, &theme.TextColor, &theme.BackgroundColor, &logoUrl, &faviconUrl,
		&street, &city, &state, &postalCode, &country,
		&settings.ContactEmail, &settings.CompanyWebsite, &settings.Locale, &settings.Timezone, &settings.DateFormat, &settings.TimeFormat, &settings.Currency,
		&tenant.CreatedAt, &tenant.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	tenant.ID = tenantID
	tenant.Description = derefString(description)
	theme.LogoUrl = derefString(logoUrl)
	theme.FaviconUrl = derefString(faviconUrl)
	tenant.Theme = theme
	address.Street = derefString(street)
	address.City = derefString(city)
	address.State = derefString(state)
	address.PostalCode = derefString(postalCode)
	address.Country = derefString(country)
	tenant.Address = address
	tenant.Settings = settings
	return &tenant, nil
}
```

- [ ] **Step 5: Update the five test fakes so the build compiles again**

Each of these structs implements the full `TenantRepository` interface; add the same one-liner to each (mirroring how each file already stubs `Update`/`Delete`):

`internal/api/handler/tenants/get_tenant/get_tenant_test.go` — add after line 34 (`func (f *fakeRepo) FindByID...}`):
```go
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	if f.tenant == nil {
		return nil, nil
	}
	t := *f.tenant
	t.Subdomain = subdomain
	return &t, nil
}
```

`internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go` — add near the other `fakeRepo` methods:
```go
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
```

`internal/api/handler/tenants/delete_tenant/delete_tenant_test.go` — add near the other `mockRepo` methods:
```go
func (m *mockRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
```

`internal/api/handler/tenants/update_tenant/update_tenant_test.go` — add near the other `mockRepo` methods:
```go
func (m *mockRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
```

`internal/api/usecases/tenants/get_all_tenants/get_all_tenants_test.go` — add near the other `fakeRepo` methods:
```go
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
```

- [ ] **Step 6: Run the full build and the new tests**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./..."
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app -e DATABASE_URL="$DATABASE_URL" golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/tenants/... ./internal/api/handler/tenants/... ./internal/api/usecases/tenants/... -v"
```
Expected: build succeeds; all tests PASS (the two new ones run for real if `DATABASE_URL` is set, otherwise SKIP — both are acceptable, but prefer running with `DATABASE_URL` set at least once locally to prove the query is correct).

- [ ] **Step 7: Commit**

```bash
git add internal/repo/pg/tenants/resources.go internal/repo/pg/tenants/repository.go \
  internal/api/handler/tenants/get_tenant/get_tenant_test.go \
  internal/api/handler/tenants/get_all_tenants/get_all_tenants_test.go \
  internal/api/handler/tenants/delete_tenant/delete_tenant_test.go \
  internal/api/handler/tenants/update_tenant/update_tenant_test.go \
  internal/api/usecases/tenants/get_all_tenants/get_all_tenants_test.go
git commit -m "feat(tenants): add FindBySubdomain to TenantRepository"
```

---

## Task 2: Public tenant lookup endpoint

**Files:**
- Create: `internal/api/usecases/tenants/get_public_tenant/usecase.go`
- Create: `internal/api/handler/tenants/get_public_tenant/models/response.go`
- Create: `internal/api/handler/tenants/get_public_tenant/get_public_tenant.go`
- Test: `internal/api/handler/tenants/get_public_tenant/get_public_tenant_test.go`
- Modify: `internal/routes/url_mappings.go`

**Interfaces:**
- Consumes: `tenants.TenantRepository` (Task 1's `FindByID` + `FindBySubdomain`).
- Produces: `GET /api/v1/public/tenants/:idOrSubdomain` — no auth, no `X-Tenant-ID` required. 200 with the trimmed DTO, 404 (`{"error":"NOT_FOUND", ...}`) if missing/inactive, 400 on empty param (Gin routing already prevents this — an empty path segment doesn't match `:idOrSubdomain`, so 400 isn't reachable in practice; no explicit check needed).

- [ ] **Step 1: Write the failing usecase test**

Create `internal/api/usecases/tenants/get_public_tenant/usecase_test.go`:

```go
package get_public_tenant

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	byID        map[uuid.UUID]*domain.Tenant
	bySubdomain map[string]*domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.byID[id], nil
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return f.bySubdomain[subdomain], nil
}

func activeTenant(id uuid.UUID) *domain.Tenant {
	return &domain.Tenant{ID: id, Subdomain: "cordoba", Name: "Cordoba SA", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func TestExecute_ByUUID_Found(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepo{byID: map[uuid.UUID]*domain.Tenant{id: activeTenant(id)}}
	uc := NewUseCase(repo)

	tenant, err := uc.Execute(context.Background(), id.String())
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
}

func TestExecute_BySubdomain_Found(t *testing.T) {
	id := uuid.New()
	repo := &fakeRepo{bySubdomain: map[string]*domain.Tenant{"cordoba": activeTenant(id)}}
	uc := NewUseCase(repo)

	tenant, err := uc.Execute(context.Background(), "cordoba")
	require.NoError(t, err)
	assert.Equal(t, id, tenant.ID)
}

func TestExecute_NotFound(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), "no-such-tenant")
	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestExecute_Inactive_TreatedAsNotFound(t *testing.T) {
	id := uuid.New()
	inactive := activeTenant(id)
	inactive.IsActive = false
	repo := &fakeRepo{byID: map[uuid.UUID]*domain.Tenant{id: inactive}}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), id.String())
	assert.ErrorIs(t, err, ErrTenantNotFound)
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/usecases/tenants/get_public_tenant/... -v"
```
Expected: FAIL — package `get_public_tenant` (the non-test file) doesn't exist yet.

- [ ] **Step 3: Implement the usecase**

Create `internal/api/usecases/tenants/get_public_tenant/usecase.go`:

```go
package get_public_tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var ErrTenantNotFound = errors.New("tenant not found")

type UseCase struct {
	repo tenants.TenantRepository
}

func NewUseCase(repo tenants.TenantRepository) *UseCase {
	return &UseCase{repo: repo}
}

// Execute resolves a tenant by its UUID or its subdomain — whichever
// idOrSubdomain parses as. No auth/tenant-membership check: this backs the
// unauthenticated public tenant lookup (invitation callback link, public
// landing page), so it deliberately doesn't distinguish "doesn't exist" from
// "exists but inactive" — both come back as ErrTenantNotFound.
func (uc *UseCase) Execute(ctx context.Context, idOrSubdomain string) (*domain.Tenant, error) {
	var tenant *domain.Tenant
	var err error

	if id, parseErr := uuid.Parse(idOrSubdomain); parseErr == nil {
		tenant, err = uc.repo.FindByID(ctx, id)
	} else {
		tenant, err = uc.repo.FindBySubdomain(ctx, idOrSubdomain)
	}
	if err != nil {
		return nil, err
	}

	if tenant == nil || !tenant.IsActive {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}
```

- [ ] **Step 4: Run the usecase tests to verify they pass**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/usecases/tenants/get_public_tenant/... -v"
```
Expected: PASS (4 tests).

- [ ] **Step 5: Write the failing handler test**

Create `internal/api/handler/tenants/get_public_tenant/get_public_tenant_test.go`:

```go
package get_public_tenant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_public_tenant"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// stubRepo implements the full tenants.TenantRepository interface — only
// FindByID/FindBySubdomain matter for these tests, the rest are unused no-ops.
type stubRepo struct {
	tenant *domain.Tenant
}

func (s *stubRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (s *stubRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)    { return nil, nil }
func (s *stubRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (s *stubRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }
func (s *stubRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return s.tenant, nil
}
func (s *stubRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return s.tenant, nil
}

func newTestRouter(tenant *domain.Tenant) *gin.Engine {
	gin.SetMode(gin.TestMode)
	uc := get_public_tenant.NewUseCase(&stubRepo{tenant: tenant})
	r := gin.Default()
	h := NewGetPublicTenantHandler(uc)
	r.GET("/api/v1/public/tenants/:idOrSubdomain", h.GetPublicTenant)
	return r
}

func TestGetPublicTenant_Found_NoBodyLeaks(t *testing.T) {
	id := uuid.New()
	tenant := &domain.Tenant{
		ID: id, Subdomain: "cordoba", Name: "Cordoba SA", CompanyName: "Cordoba SA", IsActive: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	tenant.Settings.ContactEmail = "secret@cordoba.com"
	tenant.Address.Street = "Av. Secreta 123"
	r := newTestRouter(tenant)

	req, _ := http.NewRequest("GET", "/api/v1/public/tenants/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "secret@cordoba.com")
	assert.NotContains(t, w.Body.String(), "Av. Secreta")
}

func TestGetPublicTenant_NotFound(t *testing.T) {
	r := newTestRouter(nil)

	req, _ := http.NewRequest("GET", "/api/v1/public/tenants/no-such-tenant", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetPublicTenant_BySubdomain(t *testing.T) {
	id := uuid.New()
	tenant := &domain.Tenant{ID: id, Subdomain: "cordoba", Name: "Cordoba SA", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r := newTestRouter(tenant)

	req, _ := http.NewRequest("GET", "/api/v1/public/tenants/cordoba", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 6: Run to verify it fails to compile**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/handler/tenants/get_public_tenant/... -v"
```
Expected: compile error — `NewGetPublicTenantHandler` undefined.

- [ ] **Step 7: Implement the response DTO**

Create `internal/api/handler/tenants/get_public_tenant/models/response.go`:

```go
package models

import "github.com/tu-org/embolsadora-api/internal/domain"

// Theme is the branding-safe subset of domain.Theme.
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Settings is the localization subset of domain.TenantSettings — no
// contactEmail, no companyWebsite (those are not safe to expose without auth).
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

// PublicTenantResponse is deliberately narrower than get_tenant/models.TenantResponse:
// no address, no contactEmail, no companyWebsite, no description. This is served
// without authentication, so only branding/routing fields belong here.
type PublicTenantResponse struct {
	ID          string   `json:"id"`
	Subdomain   string   `json:"subdomain"`
	Name        string   `json:"name"`
	CompanyName string   `json:"companyName"`
	IsActive    bool     `json:"isActive"`
	Theme       Theme    `json:"theme"`
	Settings    Settings `json:"settings"`
}

func FromDomain(tenant *domain.Tenant) *PublicTenantResponse {
	return &PublicTenantResponse{
		ID:          tenant.ID.String(),
		Subdomain:   tenant.Subdomain,
		Name:        tenant.Name,
		CompanyName: tenant.CompanyName,
		IsActive:    tenant.IsActive,
		Theme: Theme{
			PrimaryColor:    tenant.Theme.PrimaryColor,
			SecondaryColor:  tenant.Theme.SecondaryColor,
			AccentColor:     tenant.Theme.AccentColor,
			TextColor:       tenant.Theme.TextColor,
			BackgroundColor: tenant.Theme.BackgroundColor,
			LogoUrl:         tenant.Theme.LogoUrl,
			FaviconUrl:      tenant.Theme.FaviconUrl,
		},
		Settings: Settings{
			Locale:     tenant.Settings.Locale,
			Timezone:   tenant.Settings.Timezone,
			DateFormat: tenant.Settings.DateFormat,
			TimeFormat: tenant.Settings.TimeFormat,
			Currency:   tenant.Settings.Currency,
		},
	}
}
```

- [ ] **Step 8: Implement the handler**

Create `internal/api/handler/tenants/get_public_tenant/get_public_tenant.go`:

```go
package get_public_tenant

import (
	"net/http"

	"github.com/gin-gonic/gin"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_public_tenant/models"
	"github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_public_tenant"
)

type GetPublicTenantHandler struct {
	uc *get_public_tenant.UseCase
}

func NewGetPublicTenantHandler(uc *get_public_tenant.UseCase) *GetPublicTenantHandler {
	return &GetPublicTenantHandler{uc: uc}
}

// GetPublicTenant serves GET /api/v1/public/tenants/:idOrSubdomain — no
// session, no X-Tenant-ID. Backs the invitation/password-reset callback link
// (which runs before any session exists) and the public tenant landing page.
func (h *GetPublicTenantHandler) GetPublicTenant(c *gin.Context) {
	idOrSubdomain := c.Param("idOrSubdomain")

	tenant, err := h.uc.Execute(c.Request.Context(), idOrSubdomain)
	if err != nil {
		if err == get_public_tenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, tenantserrors.ErrorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Error al obtener tenant", Status: http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, models.FromDomain(tenant))
}
```

- [ ] **Step 9: Run the handler tests to verify they pass**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/handler/tenants/get_public_tenant/... -v"
```
Expected: PASS (3 tests), including the body-leak assertion confirming `contactEmail`/`address` never appear in the response.

- [ ] **Step 10: Register the route**

In `internal/routes/url_mappings.go`, add the import (alongside the other `tenants` handler imports, near line 12):

```go
	getPublicTenant "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/get_public_tenant"
	ucGetPublicTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/get_public_tenant"
```

Then, right after `tenantRepo := tenantsRepository.NewTenantRepository(db)` (line 76), add the wiring and route registration **directly on `r`**, not on `v1` — `v1` has `JWTAuth` applied to every route in the group, so a route registered there can never be session-less:

```go
	// Public tenant lookup — no session, no X-Tenant-ID. Backs the invitation/
	// password-reset callback link (runs before any session exists) and the
	// public tenant landing page. Registered on `r` directly (like POST
	// /api/v1/auth/login above), NOT on the `v1` group, which requires JWTAuth
	// on every route.
	getPublicTenantUC := ucGetPublicTenant.NewUseCase(tenantRepo)
	getPublicTenantHandler := getPublicTenant.NewGetPublicTenantHandler(getPublicTenantUC)
	r.GET("/api/v1/public/tenants/:idOrSubdomain", getPublicTenantHandler.GetPublicTenant)
```

- [ ] **Step 11: Full build + full test suite**

Run:
```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./..."
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go vet ./..."
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./... -short"
```
Expected: build succeeds, `go vet` clean, all tests pass (integration tests requiring `DATABASE_URL` will skip under `-short` if not set — that's fine here, Task 1's DB-backed tests were already verified in Task 1).

- [ ] **Step 12: Manual smoke test against a running server**

Run the server locally (`go run ./cmd/api` via the same Docker pattern, or whatever the engineer's existing local-run setup is) and:

```bash
curl -s http://localhost:8080/api/v1/public/tenants/cordoba | jq .
curl -s http://localhost:8080/api/v1/public/tenants/<a-real-tenant-uuid> | jq .
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/public/tenants/no-such-tenant
```
Expected: first two return 200 with the trimmed JSON shape (no `address`/`contactEmail`); third returns 404. No `Authorization` header sent in any of the three.

- [ ] **Step 13: Commit**

```bash
git add internal/api/usecases/tenants/get_public_tenant/ internal/api/handler/tenants/get_public_tenant/ internal/routes/url_mappings.go
git commit -m "feat(tenants): add public GET /api/v1/public/tenants/:idOrSubdomain

Session-less tenant lookup by UUID or subdomain, trimmed to branding/
routing fields only. Unblocks the invitation callback link and public
tenant landing page for tenants not (yet) known to the frontend's
static config — see B-005."
```

---

## Self-review notes

- **Spec coverage:** endpoint shape, field allow-list, 404-uniform behavior, UUID-or-subdomain dual lookup — all covered (Task 2). Repo-layer prerequisite for subdomain lookup — covered (Task 1). Rate limiting — explicitly *not* implemented, deviation flagged in Global Constraints.
- **Type consistency:** `get_public_tenant.UseCase.Execute` returns `(*domain.Tenant, error)` in both the usecase file and every test that calls it; `NewGetPublicTenantHandler(uc *get_public_tenant.UseCase)` matches the constructor signature used in the handler test's `newTestRouter`.
- **No placeholders:** every step has real, complete code — including the full `stubRepo` in Task 2 Step 5 (no reduced/partial interface implementations left for the engineer to fill in).
