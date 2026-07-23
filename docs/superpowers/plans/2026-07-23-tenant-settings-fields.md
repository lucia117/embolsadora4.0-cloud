# Tenant Settings Fields Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add real backend persistence for 7 tenant fields (`contactEmail`, `companyWebsite`, `locale`, `timezone`, `dateFormat`, `timeFormat`, `currency`) that the frontend `/settings` form already sends but the backend silently discards today.

**Architecture:** A new `domain.TenantSettings` sub-struct (mirroring the existing `Theme`/`Address` pattern) is embedded in `domain.Tenant`. 7 new flat, `NOT NULL DEFAULT`, `CHECK`-constrained Postgres columns back it. The repository, response DTOs, update-tenant handler/usecase, and create-tenant defaults are extended to read/write/validate/serialize the new fields, following each layer's existing conventions exactly (no ORM, no JSONB, no new validation library, no DTO consolidation).

**Tech Stack:** Go 1.24, Gin, pgx/v5, golang-migrate, testify.

## Global Constraints

- Flat Postgres columns only — no JSONB, no ORM, no validation library beyond what's already in `go.mod`.
- No `binding:` struct tags in `update_tenant`'s request DTO — validation is manual code in the handler, matching that file's existing style.
- Do not consolidate the 4 duplicated `TenantResponse` DTOs — replicate the change in all 4 independently.
- `contactEmail`/`companyWebsite` are JSON fields at the tenant root (not nested under `settings`) even though they live in `domain.Tenant.Settings` internally — this mismatch is deliberate, matching the already-shipped frontend's two-separate-PATCH-payload shape. Do not "fix" it by moving them into the JSON `settings` object.
- The 5 enum fields' catalog values must be identical between the SQL `CHECK` constraints and the Go validation slices:
  - `locale`: `es-AR`, `es-ES`, `en-US`, `pt-BR`
  - `timezone`: `America/Argentina/Buenos_Aires`, `America/Sao_Paulo`, `America/Santiago`, `America/Lima`, `America/Bogota`, `America/Mexico_City`, `UTC`
  - `dateFormat`: `dd/MM/yyyy`, `MM/dd/yyyy`, `yyyy-MM-dd`
  - `timeFormat`: `HH:mm`, `hh:mm a`
  - `currency`: `ARS`, `USD`, `EUR`, `BRL`, `CLP`, `MXN`
- Migration must be numbered `000005_add_tenant_settings` (confirmed: `000004_enforce_platform_role_tenant` is the latest existing migration; `000005` is free).
- `go build ./...` and `go test ./...` must both be clean (0 failures) at the end of every task — this matches the confirmed-clean baseline this plan started from. `go` runs natively in this environment; no Docker wrapper needed despite `CLAUDE.md`'s stale instructions.
- A local Postgres is available via `docker-compose.yml` (service `db`, `postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable`) for migration and repository-integration testing.

---

## Task 1: Domain struct + migration

**Files:**
- Modify: `internal/domain/tenants.go`
- Create: `migrations/000005_add_tenant_settings.up.sql`
- Create: `migrations/000005_add_tenant_settings.down.sql`

**Interfaces:**
- Produces: `domain.TenantSettings` struct (`ContactEmail`, `CompanyWebsite`, `Locale`, `Timezone`, `DateFormat`, `TimeFormat`, `Currency`, all `string`, `json`+`db` tagged) and `domain.Tenant.Settings TenantSettings` field. Tasks 2-5 all consume this struct and field name.

- [ ] **Step 1: Add `TenantSettings` to the domain**

Edit `internal/domain/tenants.go` — add the new struct after `Address` (before `Tenant`), and add the `Settings` field to `Tenant`:

```go
// TenantSettings represents tenant-level contact and localization preferences
type TenantSettings struct {
	ContactEmail   string `json:"contactEmail" db:"contact_email"`
	CompanyWebsite string `json:"companyWebsite" db:"company_website"`
	Locale         string `json:"locale" db:"locale"`
	Timezone       string `json:"timezone" db:"timezone"`
	DateFormat     string `json:"dateFormat" db:"date_format"`
	TimeFormat     string `json:"timeFormat" db:"time_format"`
	Currency       string `json:"currency" db:"currency"`
}

// Tenant representa una organización/empresa en el sistema
type Tenant struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	Name        string         `json:"name" db:"name"`
	CompanyName string         `json:"companyName" db:"company_name"`
	Subdomain   string         `json:"subdomain" db:"subdomain"`
	Description string         `json:"description" db:"description"`
	IsActive    bool           `json:"isActive" db:"is_active"`
	Theme       Theme          `json:"theme" db:"theme"`
	Address     Address        `json:"address" db:"address"`
	Settings    TenantSettings `json:"settings" db:"settings"`
	CreatedAt   time.Time      `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time      `json:"updatedAt" db:"updated_at"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0. (Adding a struct field with named-literal construction elsewhere in the codebase is backward compatible — nothing else needs to change yet for this to build.)

- [ ] **Step 3: Write the migration**

Create `migrations/000005_add_tenant_settings.up.sql`:

```sql
-- ============================================================================
-- Migration 000005: Add tenant settings fields (contact + localization)
-- ============================================================================
-- The frontend /settings page already sends contactEmail, companyWebsite,
-- locale, timezone, dateFormat, timeFormat, and currency on tenant update,
-- but the backend has never persisted them. This adds 7 flat columns,
-- following the existing Theme/Address precedent (no JSONB), with CHECK
-- constraints on the 5 fields that have a fixed catalog matching the
-- frontend's own <Select> options exactly.
-- ============================================================================

ALTER TABLE tenants
    ADD COLUMN contact_email character varying(255) NOT NULL DEFAULT '',
    ADD COLUMN company_website character varying(255) NOT NULL DEFAULT '',
    ADD COLUMN locale character varying(10) NOT NULL DEFAULT 'es-AR'
        CHECK (locale IN ('es-AR', 'es-ES', 'en-US', 'pt-BR')),
    ADD COLUMN timezone character varying(64) NOT NULL DEFAULT 'America/Argentina/Buenos_Aires'
        CHECK (timezone IN ('America/Argentina/Buenos_Aires', 'America/Sao_Paulo', 'America/Santiago', 'America/Lima', 'America/Bogota', 'America/Mexico_City', 'UTC')),
    ADD COLUMN date_format character varying(20) NOT NULL DEFAULT 'dd/MM/yyyy'
        CHECK (date_format IN ('dd/MM/yyyy', 'MM/dd/yyyy', 'yyyy-MM-dd')),
    ADD COLUMN time_format character varying(10) NOT NULL DEFAULT 'HH:mm'
        CHECK (time_format IN ('HH:mm', 'hh:mm a')),
    ADD COLUMN currency character varying(3) NOT NULL DEFAULT 'ARS'
        CHECK (currency IN ('ARS', 'USD', 'EUR', 'BRL', 'CLP', 'MXN'));
```

Create `migrations/000005_add_tenant_settings.down.sql`:

```sql
ALTER TABLE tenants
    DROP COLUMN contact_email,
    DROP COLUMN company_website,
    DROP COLUMN locale,
    DROP COLUMN timezone,
    DROP COLUMN date_format,
    DROP COLUMN time_format,
    DROP COLUMN currency;
```

- [ ] **Step 4: Apply and verify the migration against a local Postgres**

Start the local DB:

```bash
docker compose up -d db
```

Wait for it to be healthy, then apply migrations:

```bash
export DATABASE_URL="postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable"
migrate -path migrations/ -database "$DATABASE_URL" up
```

Expected: no error; output ends with the new version number printed (`6` if this is applied after `000001`-`000004` — golang-migrate's internal version counter, not the file prefix).

Verify the columns exist with correct defaults and constraints:

```bash
psql "$DATABASE_URL" -c "\d tenants" | grep -E "contact_email|company_website|locale|timezone|date_format|time_format|currency"
```

Expected: 7 rows showing each column with its type and `NOT NULL`.

Verify the `CHECK` constraint rejects an invalid value:

```bash
psql "$DATABASE_URL" -c "INSERT INTO tenants (id, name, company_name, subdomain, description, is_active, primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url, street, city, state, postal_code, country, locale, created_at, updated_at) VALUES (gen_random_uuid(), 'Test', 'Test Co', 'test-locale-check', '', true, '#000000', '#000000', '#000000', '#000000', '#000000', '', '', '', '', '', '', '', 'xx-XX', now(), now());"
```

Expected: the command fails with a `new row for relation "tenants" violates check constraint "tenants_locale_check"` error — confirming the constraint is active. (This test row is never actually inserted since the CHECK rejects it — no cleanup needed.)

- [ ] **Step 5: Verify the down migration**

```bash
migrate -path migrations/ -database "$DATABASE_URL" down 1
psql "$DATABASE_URL" -c "\d tenants" | grep -E "contact_email|company_website|locale|timezone|date_format|time_format|currency"
```

Expected: the `grep` returns nothing (all 7 columns dropped).

Re-apply so the DB is left migrated for Task 2:

```bash
migrate -path migrations/ -database "$DATABASE_URL" up
```

- [ ] **Step 6: Commit**

```bash
git add internal/domain/tenants.go migrations/000005_add_tenant_settings.up.sql migrations/000005_add_tenant_settings.down.sql
git commit -m "feat: add TenantSettings domain struct and migration"
```

---

## Task 2: Repository layer

**Files:**
- Modify: `internal/repo/pg/tenants/resources.go`
- Modify: `internal/repo/pg/tenants/repository.go`
- Create: `internal/repo/pg/tenants/repository_test.go`

**Interfaces:**
- Consumes: `domain.TenantSettings` and `domain.Tenant.Settings` from Task 1.
- Produces: `TenantRepository.Create/FindByID/FindAll/Update` all read/write the 7 new columns via `tenant.Settings.*`. Tasks 3-5 rely on this being wired correctly for the feature to actually persist (though their own unit/mock tests don't require it to pass).

- [ ] **Step 1: Update the SQL query constants**

Edit `internal/repo/pg/tenants/resources.go` — replace the full file:

```go
package tenants

const (
	// FindByIDQuery retrieves a tenant by ID with all related data
	FindByIDQuery = `
		SELECT 
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			contact_email, company_website, locale, timezone, date_format, time_format, currency,
			created_at, updated_at
		FROM tenants 
		WHERE id = $1
	`

	// CreateQuery inserts a new tenant with all fields
	CreateQuery = `
		INSERT INTO tenants (
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			contact_email, company_website, locale, timezone, date_format, time_format, currency,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24, $25,
			$26, $27
		)
	`

	// FindAllQuery retrieves all tenants ordered by creation date
	FindAllQuery = `
		SELECT 
			id, name, company_name, subdomain, description, is_active,
			primary_color, secondary_color, accent_color, text_color, background_color, logo_url, favicon_url,
			street, city, state, postal_code, country,
			contact_email, company_website, locale, timezone, date_format, time_format, currency,
			created_at, updated_at
		FROM tenants 
		ORDER BY created_at DESC
	`

	// UpdateQuery updates an existing tenant by ID
	UpdateQuery = `
		UPDATE tenants SET
			name = $1, company_name = $2, subdomain = $3, description = $4, is_active = $5,
			primary_color = $6, secondary_color = $7, accent_color = $8, text_color = $9, background_color = $10, logo_url = $11, favicon_url = $12,
			street = $13, city = $14, state = $15, postal_code = $16, country = $17,
			contact_email = $18, company_website = $19, locale = $20, timezone = $21, date_format = $22, time_format = $23, currency = $24,
			updated_at = $25
		WHERE id = $26
	`

	// DeleteQuery removes a tenant by ID
	DeleteQuery = `DELETE FROM tenants WHERE id = $1`
)
```

- [ ] **Step 2: Update the repository's Scan/Exec calls**

Edit `internal/repo/pg/tenants/repository.go` — replace the full file:

```go
package tenants

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// TenantRepository define la interfaz para el repositorio de tenants
type TenantRepository interface {
	Create(ctx context.Context, tenant *domain.Tenant) error
	FindAll(ctx context.Context) ([]domain.Tenant, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
	Update(ctx context.Context, tenant *domain.Tenant) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type tenantRepository struct {
	db *pgxpool.Pool
}

// NewTenantRepository crea una nueva instancia del repositorio de tenants
func NewTenantRepository(db *pgxpool.Pool) TenantRepository {
	return &tenantRepository{db: db}
}
func (r *tenantRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	var tenant domain.Tenant
	var theme domain.Theme
	var address domain.Address
	var settings domain.TenantSettings
	var tenantID uuid.UUID

	var description, logoUrl, faviconUrl *string
	var street, city, state, postalCode, country *string

	err := r.db.QueryRow(ctx, FindByIDQuery, id).Scan(
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

func (r *tenantRepository) Create(ctx context.Context, tenant *domain.Tenant) error {
	_, err := r.db.Exec(ctx, CreateQuery,
		tenant.ID, tenant.Name, tenant.CompanyName, tenant.Subdomain, tenant.Description, tenant.IsActive,
		tenant.Theme.PrimaryColor, tenant.Theme.SecondaryColor, tenant.Theme.AccentColor, tenant.Theme.TextColor, tenant.Theme.BackgroundColor, tenant.Theme.LogoUrl, tenant.Theme.FaviconUrl,
		tenant.Address.Street, tenant.Address.City, tenant.Address.State, tenant.Address.PostalCode, tenant.Address.Country,
		tenant.Settings.ContactEmail, tenant.Settings.CompanyWebsite, tenant.Settings.Locale, tenant.Settings.Timezone, tenant.Settings.DateFormat, tenant.Settings.TimeFormat, tenant.Settings.Currency,
		tenant.CreatedAt, tenant.UpdatedAt,
	)
	return err
}

func (r *tenantRepository) FindAll(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := r.db.Query(ctx, FindAllQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenants []domain.Tenant
	for rows.Next() {
		var tenant domain.Tenant
		var theme domain.Theme
		var address domain.Address
		var settings domain.TenantSettings

		var description, logoUrl, faviconUrl *string
		var street, city, state, postalCode, country *string

		err := rows.Scan(
			&tenant.ID, &tenant.Name, &tenant.CompanyName, &tenant.Subdomain, &description, &tenant.IsActive,
			&theme.PrimaryColor, &theme.SecondaryColor, &theme.AccentColor, &theme.TextColor, &theme.BackgroundColor, &logoUrl, &faviconUrl,
			&street, &city, &state, &postalCode, &country,
			&settings.ContactEmail, &settings.CompanyWebsite, &settings.Locale, &settings.Timezone, &settings.DateFormat, &settings.TimeFormat, &settings.Currency,
			&tenant.CreatedAt, &tenant.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

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
		tenants = append(tenants, tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tenants, nil
}

func (r *tenantRepository) Update(ctx context.Context, tenant *domain.Tenant) error {
	_, err := r.db.Exec(ctx, UpdateQuery,
		tenant.Name, tenant.CompanyName, tenant.Subdomain, tenant.Description, tenant.IsActive,
		tenant.Theme.PrimaryColor, tenant.Theme.SecondaryColor, tenant.Theme.AccentColor, tenant.Theme.TextColor, tenant.Theme.BackgroundColor, tenant.Theme.LogoUrl, tenant.Theme.FaviconUrl,
		tenant.Address.Street, tenant.Address.City, tenant.Address.State, tenant.Address.PostalCode, tenant.Address.Country,
		tenant.Settings.ContactEmail, tenant.Settings.CompanyWebsite, tenant.Settings.Locale, tenant.Settings.Timezone, tenant.Settings.DateFormat, tenant.Settings.TimeFormat, tenant.Settings.Currency,
		tenant.UpdatedAt,
		tenant.ID,
	)
	return err
}

func (r *tenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, DeleteQuery, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// derefString convierte un *string nullable a string, retornando "" si es nil
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 4: Write the repository integration test**

This repo has no existing tests for `internal/repo/pg/tenants/`, but `internal/repo/pg/users/users_repo_test.go` establishes the pattern for DB-integration tests in this codebase: skip via `t.Skip()` if `DATABASE_URL` is unset, otherwise run against a real Postgres. Follow that exact pattern.

Create `internal/repo/pg/tenants/repository_test.go`:

```go
package tenants_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

func newTestTenant() *domain.Tenant {
	now := time.Now().UTC().Truncate(time.Second)
	return &domain.Tenant{
		ID:          uuid.New(),
		Name:        "Repo Test Tenant",
		CompanyName: "Repo Test Co",
		Subdomain:   "repo-test-" + uuid.NewString()[:8],
		Description: "created by repository_test.go",
		IsActive:    true,
		Theme: domain.Theme{
			PrimaryColor:    "#111111",
			SecondaryColor:  "#222222",
			AccentColor:     "#333333",
			TextColor:       "#444444",
			BackgroundColor: "#555555",
			LogoUrl:         "",
			FaviconUrl:      "",
		},
		Address: domain.Address{
			Street:     "Test St 123",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1001",
			Country:    "Argentina",
		},
		Settings: domain.TenantSettings{
			ContactEmail:   "contacto@repotest.com",
			CompanyWebsite: "https://repotest.com",
			Locale:         "es-AR",
			Timezone:       "America/Argentina/Buenos_Aires",
			DateFormat:     "dd/MM/yyyy",
			TimeFormat:     "HH:mm",
			Currency:       "ARS",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// TestSettings_RoundTrip verifies TenantSettings fields survive Create -> FindByID.
func TestSettings_RoundTrip(t *testing.T) {
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

	found, err := repo.FindByID(ctx, tenant.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, tenant.Settings, found.Settings)
}

// TestSettings_Update verifies TenantSettings fields survive Update -> FindByID.
func TestSettings_Update(t *testing.T) {
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

	tenant.Settings = domain.TenantSettings{
		ContactEmail:   "nuevo@repotest.com",
		CompanyWebsite: "https://nuevo.repotest.com",
		Locale:         "en-US",
		Timezone:       "UTC",
		DateFormat:     "yyyy-MM-dd",
		TimeFormat:     "hh:mm a",
		Currency:       "USD",
	}
	tenant.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.Update(ctx, tenant))

	found, err := repo.FindByID(ctx, tenant.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, tenant.Settings, found.Settings)
}
```

- [ ] **Step 5: Run the test against the local Postgres**

```bash
docker compose up -d db
export DATABASE_URL="postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable"
go test ./internal/repo/pg/tenants/... -run TestSettings -v
```

Expected: `--- PASS: TestSettings_RoundTrip` and `--- PASS: TestSettings_Update`, both passing (the Postgres started here already has migration `000005` applied from Task 1, Step 4-5).

- [ ] **Step 6: Run the full suite to confirm no regressions**

```bash
unset DATABASE_URL  # confirm the test still skips gracefully without a DB, like users_repo_test.go does
go build ./... && go test ./...
```

Expected: `go build` silent, `go test ./...` all `ok` or `[no test files]`, with `internal/repo/pg/tenants` showing `--- SKIP: TestSettings_RoundTrip` and `--- SKIP: TestSettings_Update` (not a failure) when `DATABASE_URL` is unset.

- [ ] **Step 7: Commit**

```bash
git add internal/repo/pg/tenants/resources.go internal/repo/pg/tenants/repository.go internal/repo/pg/tenants/repository_test.go
git commit -m "feat: persist tenant settings fields in the repository layer"
```

---

## Task 3: Response DTOs (4 duplicated files)

**Files:**
- Modify: `internal/api/handler/tenants/create_tenant/models/response.go`
- Modify: `internal/api/handler/tenants/get_tenant/models/response.go`
- Modify: `internal/api/handler/tenants/update_tenant/models/response.go`
- Modify: `internal/api/handler/tenants/get_all_tenants/models/response.go`
- Create: `internal/api/handler/tenants/create_tenant/models/response_test.go`
- Create: `internal/api/handler/tenants/get_tenant/models/response_test.go`
- Create: `internal/api/handler/tenants/get_all_tenants/models/response_test.go`

**Interfaces:**
- Consumes: `domain.TenantSettings`/`domain.Tenant.Settings` from Task 1.
- Produces: each of the 4 `TenantResponse` types (and `get_all_tenants`'s `GetAllTenantsResponse` slice type) gains `ContactEmail string`, `CompanyWebsite string`, `Settings Settings` (new `Settings` struct: `Locale, Timezone, DateFormat, TimeFormat, Currency string`, all json-tagged). Task 4's test for the update-tenant flow asserts against `update_tenant/models.TenantResponse`'s new fields — this task must land before Task 4 for that test to compile.

- [ ] **Step 1: Update `update_tenant/models/response.go`**

Replace the full file:

```go
package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Theme represents the visual theme configuration for a tenant
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Address represents the address information for a tenant
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// Settings represents the localization/preferences configuration for a tenant
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

// TenantResponse define la estructura de respuesta para los tenants
type TenantResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	CompanyName    string   `json:"companyName"`
	Subdomain      string   `json:"subdomain"`
	Description    string   `json:"description"`
	IsActive       bool     `json:"isActive"`
	ContactEmail   string   `json:"contactEmail"`
	CompanyWebsite string   `json:"companyWebsite"`
	Theme          Theme    `json:"theme"`
	Address        Address  `json:"address"`
	Settings       Settings `json:"settings"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

func FromDomain(tenant *domain.Tenant) *TenantResponse {
	return &TenantResponse{
		ID:             tenant.ID.String(),
		Name:           tenant.Name,
		CompanyName:    tenant.CompanyName,
		Subdomain:      tenant.Subdomain,
		Description:    tenant.Description,
		IsActive:       tenant.IsActive,
		ContactEmail:   tenant.Settings.ContactEmail,
		CompanyWebsite: tenant.Settings.CompanyWebsite,
		Theme: Theme{
			PrimaryColor:    tenant.Theme.PrimaryColor,
			SecondaryColor:  tenant.Theme.SecondaryColor,
			AccentColor:     tenant.Theme.AccentColor,
			TextColor:       tenant.Theme.TextColor,
			BackgroundColor: tenant.Theme.BackgroundColor,
			LogoUrl:         tenant.Theme.LogoUrl,
			FaviconUrl:      tenant.Theme.FaviconUrl,
		},
		Address: Address{
			Street:     tenant.Address.Street,
			City:       tenant.Address.City,
			State:      tenant.Address.State,
			PostalCode: tenant.Address.PostalCode,
			Country:    tenant.Address.Country,
		},
		Settings: Settings{
			Locale:     tenant.Settings.Locale,
			Timezone:   tenant.Settings.Timezone,
			DateFormat: tenant.Settings.DateFormat,
			TimeFormat: tenant.Settings.TimeFormat,
			Currency:   tenant.Settings.Currency,
		},
		CreatedAt: tenant.CreatedAt.Format(time.RFC3339),
		UpdatedAt: tenant.UpdatedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 2: Update `create_tenant/models/response.go`**

Replace the full file:

```go
package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Theme represents the visual theme configuration for a tenant
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Address represents the address information for a tenant
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// Settings represents the localization/preferences configuration for a tenant
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

// TenantResponse define la estructura de respuesta para los tenants
type TenantResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	CompanyName    string   `json:"companyName"`
	Subdomain      string   `json:"subdomain"`
	Description    string   `json:"description"`
	IsActive       bool     `json:"isActive"`
	ContactEmail   string   `json:"contactEmail"`
	CompanyWebsite string   `json:"companyWebsite"`
	Theme          Theme    `json:"theme"`
	Address        Address  `json:"address"`
	Settings       Settings `json:"settings"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// TenantResponseSingle define la estructura de respuesta para un solo tenant
type TenantResponseSingle struct {
	Tenant TenantResponse `json:"tenant"`
}

func FromDomain(tenant *domain.Tenant) *TenantResponse {
	return &TenantResponse{
		ID:             tenant.ID.String(),
		Name:           tenant.Name,
		CompanyName:    tenant.CompanyName,
		Subdomain:      tenant.Subdomain,
		Description:    tenant.Description,
		IsActive:       tenant.IsActive,
		ContactEmail:   tenant.Settings.ContactEmail,
		CompanyWebsite: tenant.Settings.CompanyWebsite,
		Theme: Theme{
			PrimaryColor:    tenant.Theme.PrimaryColor,
			SecondaryColor:  tenant.Theme.SecondaryColor,
			AccentColor:     tenant.Theme.AccentColor,
			TextColor:       tenant.Theme.TextColor,
			BackgroundColor: tenant.Theme.BackgroundColor,
			LogoUrl:         tenant.Theme.LogoUrl,
			FaviconUrl:      tenant.Theme.FaviconUrl,
		},
		Address: Address{
			Street:     tenant.Address.Street,
			City:       tenant.Address.City,
			State:      tenant.Address.State,
			PostalCode: tenant.Address.PostalCode,
			Country:    tenant.Address.Country,
		},
		Settings: Settings{
			Locale:     tenant.Settings.Locale,
			Timezone:   tenant.Settings.Timezone,
			DateFormat: tenant.Settings.DateFormat,
			TimeFormat: tenant.Settings.TimeFormat,
			Currency:   tenant.Settings.Currency,
		},
		CreatedAt: tenant.CreatedAt.Format(time.RFC3339),
		UpdatedAt: tenant.UpdatedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 3: Write and run the test for `create_tenant`'s `FromDomain`**

Create `internal/api/handler/tenants/create_tenant/models/response_test.go`:

```go
package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_IncludesSettings(t *testing.T) {
	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Name: "Test",
		Settings: domain.TenantSettings{
			ContactEmail:   "contacto@test.com",
			CompanyWebsite: "https://test.com",
			Locale:         "pt-BR",
			Timezone:       "America/Sao_Paulo",
			DateFormat:     "MM/dd/yyyy",
			TimeFormat:     "hh:mm a",
			Currency:       "BRL",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := FromDomain(tenant)

	assert.Equal(t, "contacto@test.com", resp.ContactEmail)
	assert.Equal(t, "https://test.com", resp.CompanyWebsite)
	assert.Equal(t, "pt-BR", resp.Settings.Locale)
	assert.Equal(t, "America/Sao_Paulo", resp.Settings.Timezone)
	assert.Equal(t, "MM/dd/yyyy", resp.Settings.DateFormat)
	assert.Equal(t, "hh:mm a", resp.Settings.TimeFormat)
	assert.Equal(t, "BRL", resp.Settings.Currency)
}
```

Run: `go test ./internal/api/handler/tenants/create_tenant/models/... -run TestFromDomain_IncludesSettings -v`
Expected: `--- PASS: TestFromDomain_IncludesSettings`.

- [ ] **Step 4: Update `get_tenant/models/response.go`**

Replace the full file (identical structure to `create_tenant`'s, minus `TenantResponseSingle`):

```go
package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// Theme represents the visual theme configuration for a tenant
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Address represents the address information for a tenant
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// Settings represents the localization/preferences configuration for a tenant
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

// TenantResponse define la estructura de respuesta para los tenants
type TenantResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	CompanyName    string   `json:"companyName"`
	Subdomain      string   `json:"subdomain"`
	Description    string   `json:"description"`
	IsActive       bool     `json:"isActive"`
	ContactEmail   string   `json:"contactEmail"`
	CompanyWebsite string   `json:"companyWebsite"`
	Theme          Theme    `json:"theme"`
	Address        Address  `json:"address"`
	Settings       Settings `json:"settings"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

func FromDomain(tenant *domain.Tenant) *TenantResponse {
	return &TenantResponse{
		ID:             tenant.ID.String(),
		Name:           tenant.Name,
		CompanyName:    tenant.CompanyName,
		Subdomain:      tenant.Subdomain,
		Description:    tenant.Description,
		IsActive:       tenant.IsActive,
		ContactEmail:   tenant.Settings.ContactEmail,
		CompanyWebsite: tenant.Settings.CompanyWebsite,
		Theme: Theme{
			PrimaryColor:    tenant.Theme.PrimaryColor,
			SecondaryColor:  tenant.Theme.SecondaryColor,
			AccentColor:     tenant.Theme.AccentColor,
			TextColor:       tenant.Theme.TextColor,
			BackgroundColor: tenant.Theme.BackgroundColor,
			LogoUrl:         tenant.Theme.LogoUrl,
			FaviconUrl:      tenant.Theme.FaviconUrl,
		},
		Address: Address{
			Street:     tenant.Address.Street,
			City:       tenant.Address.City,
			State:      tenant.Address.State,
			PostalCode: tenant.Address.PostalCode,
			Country:    tenant.Address.Country,
		},
		Settings: Settings{
			Locale:     tenant.Settings.Locale,
			Timezone:   tenant.Settings.Timezone,
			DateFormat: tenant.Settings.DateFormat,
			TimeFormat: tenant.Settings.TimeFormat,
			Currency:   tenant.Settings.Currency,
		},
		CreatedAt: tenant.CreatedAt.Format(time.RFC3339),
		UpdatedAt: tenant.UpdatedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 5: Write and run the test for `get_tenant`'s `FromDomain`**

Create `internal/api/handler/tenants/get_tenant/models/response_test.go`:

```go
package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_IncludesSettings(t *testing.T) {
	tenant := &domain.Tenant{
		ID:   uuid.New(),
		Name: "Test",
		Settings: domain.TenantSettings{
			ContactEmail:   "contacto@test.com",
			CompanyWebsite: "https://test.com",
			Locale:         "en-US",
			Timezone:       "UTC",
			DateFormat:     "yyyy-MM-dd",
			TimeFormat:     "HH:mm",
			Currency:       "USD",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	resp := FromDomain(tenant)

	assert.Equal(t, "contacto@test.com", resp.ContactEmail)
	assert.Equal(t, "https://test.com", resp.CompanyWebsite)
	assert.Equal(t, "en-US", resp.Settings.Locale)
	assert.Equal(t, "UTC", resp.Settings.Timezone)
	assert.Equal(t, "yyyy-MM-dd", resp.Settings.DateFormat)
	assert.Equal(t, "HH:mm", resp.Settings.TimeFormat)
	assert.Equal(t, "USD", resp.Settings.Currency)
}
```

Run: `go test ./internal/api/handler/tenants/get_tenant/models/... -run TestFromDomain_IncludesSettings -v`
Expected: `--- PASS: TestFromDomain_IncludesSettings`.

- [ ] **Step 6: Update `get_all_tenants/models/response.go`**

This file has a different shape (`GetAllTenantsResponse []TenantResponse` and a slice-mapping `FromDomain(tenants []domain.Tenant) GetAllTenantsResponse`) — adapt accordingly. Replace the full file:

```go
package models

import (
	"time"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// GetAllTenantsResponse representa la respuesta del endpoint GET /api/tenants
type GetAllTenantsResponse []TenantResponse

// TenantResponse representa un tenant individual en la respuesta
type TenantResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	CompanyName    string   `json:"companyName"`
	Subdomain      string   `json:"subdomain"`
	Description    string   `json:"description"`
	IsActive       bool     `json:"isActive"`
	ContactEmail   string   `json:"contactEmail"`
	CompanyWebsite string   `json:"companyWebsite"`
	Theme          Theme    `json:"theme"`
	Address        Address  `json:"address"`
	Settings       Settings `json:"settings"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// Theme representa la configuración de tema de un tenant
type Theme struct {
	PrimaryColor    string `json:"primaryColor"`
	SecondaryColor  string `json:"secondaryColor"`
	AccentColor     string `json:"accentColor"`
	TextColor       string `json:"textColor"`
	BackgroundColor string `json:"backgroundColor"`
	LogoUrl         string `json:"logoUrl"`
	FaviconUrl      string `json:"faviconUrl"`
}

// Address representa la dirección de un tenant
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
}

// Settings representa la configuración de localización/preferencias de un tenant
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

func FromDomain(tenants []domain.Tenant) GetAllTenantsResponse {
	response := make(GetAllTenantsResponse, len(tenants))
	for i, tenant := range tenants {
		response[i] = TenantResponse{
			ID:             tenant.ID.String(),
			Name:           tenant.Name,
			CompanyName:    tenant.CompanyName,
			Subdomain:      tenant.Subdomain,
			Description:    tenant.Description,
			IsActive:       tenant.IsActive,
			ContactEmail:   tenant.Settings.ContactEmail,
			CompanyWebsite: tenant.Settings.CompanyWebsite,
			Theme: Theme{
				PrimaryColor:    tenant.Theme.PrimaryColor,
				SecondaryColor:  tenant.Theme.SecondaryColor,
				AccentColor:     tenant.Theme.AccentColor,
				TextColor:       tenant.Theme.TextColor,
				BackgroundColor: tenant.Theme.BackgroundColor,
				LogoUrl:         tenant.Theme.LogoUrl,
				FaviconUrl:      tenant.Theme.FaviconUrl,
			},
			Address: Address{
				Street:     tenant.Address.Street,
				City:       tenant.Address.City,
				State:      tenant.Address.State,
				PostalCode: tenant.Address.PostalCode,
				Country:    tenant.Address.Country,
			},
			Settings: Settings{
				Locale:     tenant.Settings.Locale,
				Timezone:   tenant.Settings.Timezone,
				DateFormat: tenant.Settings.DateFormat,
				TimeFormat: tenant.Settings.TimeFormat,
				Currency:   tenant.Settings.Currency,
			},
			CreatedAt: tenant.CreatedAt.Format(time.RFC3339),
			UpdatedAt: tenant.UpdatedAt.Format(time.RFC3339),
		}
	}
	return response
}
```

- [ ] **Step 7: Write and run the test for `get_all_tenants`'s `FromDomain`**

Create `internal/api/handler/tenants/get_all_tenants/models/response_test.go`:

```go
package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestFromDomain_IncludesSettings(t *testing.T) {
	tenants := []domain.Tenant{
		{
			ID:   uuid.New(),
			Name: "Test",
			Settings: domain.TenantSettings{
				ContactEmail:   "contacto@test.com",
				CompanyWebsite: "https://test.com",
				Locale:         "es-ES",
				Timezone:       "America/Santiago",
				DateFormat:     "dd/MM/yyyy",
				TimeFormat:     "HH:mm",
				Currency:       "CLP",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	resp := FromDomain(tenants)

	assert.Len(t, resp, 1)
	assert.Equal(t, "contacto@test.com", resp[0].ContactEmail)
	assert.Equal(t, "https://test.com", resp[0].CompanyWebsite)
	assert.Equal(t, "es-ES", resp[0].Settings.Locale)
	assert.Equal(t, "America/Santiago", resp[0].Settings.Timezone)
	assert.Equal(t, "CLP", resp[0].Settings.Currency)
}
```

Run: `go test ./internal/api/handler/tenants/get_all_tenants/models/... -run TestFromDomain_IncludesSettings -v`
Expected: `--- PASS: TestFromDomain_IncludesSettings`.

- [ ] **Step 8: Run the full suite to confirm no regressions**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests `ok` or `[no test files]` (or `SKIP` for the Task 2 integration tests if `DATABASE_URL` is unset in this shell).

- [ ] **Step 9: Commit**

```bash
git add internal/api/handler/tenants/create_tenant/models/response.go internal/api/handler/tenants/create_tenant/models/response_test.go internal/api/handler/tenants/get_tenant/models/response.go internal/api/handler/tenants/get_tenant/models/response_test.go internal/api/handler/tenants/update_tenant/models/response.go internal/api/handler/tenants/get_all_tenants/models/response.go internal/api/handler/tenants/get_all_tenants/models/response_test.go
git commit -m "feat: include tenant settings fields in all response DTOs"
```

---

## Task 4: Update-tenant flow (handler DTO, validation, usecase, tests)

**Files:**
- Modify: `internal/api/handler/tenants/update_tenant/models/request.go`
- Modify: `internal/api/handler/tenants/update_tenant/update_tenant.go`
- Modify: `internal/api/usecases/tenants/update_tenant/usecase.go`
- Modify: `internal/api/handler/tenants/update_tenant/update_tenant_test.go`

**Interfaces:**
- Consumes: `domain.TenantSettings` / `domain.Tenant.Settings` from Task 1; `update_tenant/models.TenantResponse`'s `ContactEmail`/`CompanyWebsite`/`Settings` fields from Task 3 (this task's test asserts against them).
- Produces: `models.TenantUpdateRequest{ContactEmail, CompanyWebsite *string, Settings *models.SettingsUpdate}`, `models.SettingsUpdate{Locale, Timezone, DateFormat, TimeFormat, Currency *string}`; `update_tenant.UpdateTenantRequest{ContactEmail, CompanyWebsite *string, Settings *update_tenant.SettingsUpdate}`, `update_tenant.SettingsUpdate{Locale, Timezone, DateFormat, TimeFormat, Currency *string}`.

- [ ] **Step 1: Add the new fields to the handler request DTO**

Edit `internal/api/handler/tenants/update_tenant/models/request.go` — replace the full file:

```go
package models

// ThemeUpdate represents the theme configuration for tenant update
type ThemeUpdate struct {
	PrimaryColor    *string `json:"primaryColor"`
	SecondaryColor  *string `json:"secondaryColor"`
	AccentColor     *string `json:"accentColor"`
	TextColor       *string `json:"textColor"`
	BackgroundColor *string `json:"backgroundColor"`
	LogoUrl         *string `json:"logoUrl"`
	FaviconUrl      *string `json:"faviconUrl"`
}

// AddressUpdate represents the address information for tenant update
type AddressUpdate struct {
	Street     *string `json:"street"`
	City       *string `json:"city"`
	State      *string `json:"state"`
	PostalCode *string `json:"postalCode"`
	Country    *string `json:"country"`
}

// SettingsUpdate represents the localization/preferences sub-object for tenant update
type SettingsUpdate struct {
	Locale     *string `json:"locale"`
	Timezone   *string `json:"timezone"`
	DateFormat *string `json:"dateFormat"`
	TimeFormat *string `json:"timeFormat"`
	Currency   *string `json:"currency"`
}

// TenantUpdateRequest define la estructura para actualizar un tenant (con campos opcionales)
type TenantUpdateRequest struct {
	Name           *string         `json:"name"`
	CompanyName    *string         `json:"companyName"`
	Subdomain      *string         `json:"subdomain"`
	Description    *string         `json:"description"`
	IsActive       *bool           `json:"isActive"`
	ContactEmail   *string         `json:"contactEmail"`
	CompanyWebsite *string         `json:"companyWebsite"`
	Theme          *ThemeUpdate    `json:"theme"`
	Address        *AddressUpdate  `json:"address"`
	Settings       *SettingsUpdate `json:"settings"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 3: Add the matching fields to the usecase request DTO and update logic**

Edit `internal/api/usecases/tenants/update_tenant/usecase.go` — replace the full file:

```go
package update_tenant

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

var ErrTenantNotFound = errors.New("tenant not found")

// UseCase defines the interface for tenant update use case
type UseCase interface {
	Update(ctx context.Context, id uuid.UUID, req *UpdateTenantRequest) (*domain.Tenant, error)
}

// UpdateTenantRequest represents the request to update a tenant
type UpdateTenantRequest struct {
	Name           *string
	CompanyName    *string
	Subdomain      *string
	Description    *string
	IsActive       *bool
	ContactEmail   *string
	CompanyWebsite *string
	Theme          *ThemeUpdate
	Address        *AddressUpdate
	Settings       *SettingsUpdate
}

// ThemeUpdate represents the theme configuration for update
type ThemeUpdate struct {
	PrimaryColor    *string
	SecondaryColor  *string
	AccentColor     *string
	TextColor       *string
	BackgroundColor *string
	LogoUrl         *string
	FaviconUrl      *string
}

// AddressUpdate represents the address information for update
type AddressUpdate struct {
	Street     *string
	City       *string
	State      *string
	PostalCode *string
	Country    *string
}

// SettingsUpdate represents the localization/preferences configuration for update
type SettingsUpdate struct {
	Locale     *string
	Timezone   *string
	DateFormat *string
	TimeFormat *string
	Currency   *string
}

type useCase struct {
	repo tenants.TenantRepository
}

// NewUseCase creates a new instance of the tenant update use case
func NewUseCase(repo tenants.TenantRepository) UseCase {
	return &useCase{
		repo: repo,
	}
}

// Update updates an existing tenant
func (uc *useCase) Update(ctx context.Context, id uuid.UUID, req *UpdateTenantRequest) (*domain.Tenant, error) {
	// First, get the existing tenant
	tenant, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}

	// Update fields if they are provided (not nil)
	if req.Name != nil {
		tenant.Name = *req.Name
	}
	if req.CompanyName != nil {
		tenant.CompanyName = *req.CompanyName
	}
	if req.Subdomain != nil {
		tenant.Subdomain = *req.Subdomain
	}
	if req.Description != nil {
		tenant.Description = *req.Description
	}
	if req.IsActive != nil {
		tenant.IsActive = *req.IsActive
	}

	// Update theme fields if provided
	if req.Theme != nil {
		if req.Theme.PrimaryColor != nil {
			tenant.Theme.PrimaryColor = *req.Theme.PrimaryColor
		}
		if req.Theme.SecondaryColor != nil {
			tenant.Theme.SecondaryColor = *req.Theme.SecondaryColor
		}
		if req.Theme.AccentColor != nil {
			tenant.Theme.AccentColor = *req.Theme.AccentColor
		}
		if req.Theme.TextColor != nil {
			tenant.Theme.TextColor = *req.Theme.TextColor
		}
		if req.Theme.BackgroundColor != nil {
			tenant.Theme.BackgroundColor = *req.Theme.BackgroundColor
		}
		if req.Theme.LogoUrl != nil {
			tenant.Theme.LogoUrl = *req.Theme.LogoUrl
		}
		if req.Theme.FaviconUrl != nil {
			tenant.Theme.FaviconUrl = *req.Theme.FaviconUrl
		}
	}

	// Update address fields if provided
	if req.Address != nil {
		if req.Address.Street != nil {
			tenant.Address.Street = *req.Address.Street
		}
		if req.Address.City != nil {
			tenant.Address.City = *req.Address.City
		}
		if req.Address.State != nil {
			tenant.Address.State = *req.Address.State
		}
		if req.Address.PostalCode != nil {
			tenant.Address.PostalCode = *req.Address.PostalCode
		}
		if req.Address.Country != nil {
			tenant.Address.Country = *req.Address.Country
		}
	}

	// Update contact/website fields if provided
	if req.ContactEmail != nil {
		tenant.Settings.ContactEmail = *req.ContactEmail
	}
	if req.CompanyWebsite != nil {
		tenant.Settings.CompanyWebsite = *req.CompanyWebsite
	}

	// Update settings fields if provided
	if req.Settings != nil {
		if req.Settings.Locale != nil {
			tenant.Settings.Locale = *req.Settings.Locale
		}
		if req.Settings.Timezone != nil {
			tenant.Settings.Timezone = *req.Settings.Timezone
		}
		if req.Settings.DateFormat != nil {
			tenant.Settings.DateFormat = *req.Settings.DateFormat
		}
		if req.Settings.TimeFormat != nil {
			tenant.Settings.TimeFormat = *req.Settings.TimeFormat
		}
		if req.Settings.Currency != nil {
			tenant.Settings.Currency = *req.Settings.Currency
		}
	}

	// Update the timestamp
	tenant.UpdatedAt = time.Now()

	err = uc.repo.Update(ctx, tenant)
	if err != nil {
		return nil, err
	}

	return tenant, nil
}
```

- [ ] **Step 4: Add validation and mapping in the handler**

Edit `internal/api/handler/tenants/update_tenant/update_tenant.go` — replace the full file:

```go
package update_tenant

import (
	"log"
	"net/http"
	"net/mail"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	tenantserrors "github.com/tu-org/embolsadora-api/internal/api/handler/tenants/errors"
	"github.com/tu-org/embolsadora-api/internal/api/handler/tenants/update_tenant/models"
	ucUpdateTenant "github.com/tu-org/embolsadora-api/internal/api/usecases/tenants/update_tenant"
)

var (
	validLocales     = []string{"es-AR", "es-ES", "en-US", "pt-BR"}
	validTimezones   = []string{"America/Argentina/Buenos_Aires", "America/Sao_Paulo", "America/Santiago", "America/Lima", "America/Bogota", "America/Mexico_City", "UTC"}
	validDateFormats = []string{"dd/MM/yyyy", "MM/dd/yyyy", "yyyy-MM-dd"}
	validTimeFormats = []string{"HH:mm", "hh:mm a"}
	validCurrencies  = []string{"ARS", "USD", "EUR", "BRL", "CLP", "MXN"}
)

func isOneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

type UpdateTenantHandler struct {
	useCase ucUpdateTenant.UseCase
}

func NewUpdateTenantHandler(useCase ucUpdateTenant.UseCase) *UpdateTenantHandler {
	return &UpdateTenantHandler{
		useCase: useCase,
	}
}

func (h *UpdateTenantHandler) UpdateTenant(c *gin.Context) {
	id, err := uuid.Parse(c.Param("tenantId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "ID de tenant inválido", Status: http.StatusBadRequest})
		return
	}

	var req models.TenantUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
		return
	}

	if req.ContactEmail != nil {
		if _, err := mail.ParseAddress(*req.ContactEmail); err != nil {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Email de contacto inválido", Status: http.StatusBadRequest})
			return
		}
	}

	if req.CompanyWebsite != nil && *req.CompanyWebsite != "" {
		if _, err := url.ParseRequestURI(*req.CompanyWebsite); err != nil {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Sitio web inválido", Status: http.StatusBadRequest})
			return
		}
	}

	if req.Settings != nil {
		if req.Settings.Locale != nil && !isOneOf(*req.Settings.Locale, validLocales) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Idioma inválido", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.Timezone != nil && !isOneOf(*req.Settings.Timezone, validTimezones) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Zona horaria inválida", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.DateFormat != nil && !isOneOf(*req.Settings.DateFormat, validDateFormats) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Formato de fecha inválido", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.TimeFormat != nil && !isOneOf(*req.Settings.TimeFormat, validTimeFormats) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Formato de hora inválido", Status: http.StatusBadRequest})
			return
		}
		if req.Settings.Currency != nil && !isOneOf(*req.Settings.Currency, validCurrencies) {
			c.JSON(http.StatusBadRequest, tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "Moneda inválida", Status: http.StatusBadRequest})
			return
		}
	}

	// Convert request to usecase request
	useCaseReq := &ucUpdateTenant.UpdateTenantRequest{}

	// Only set fields that are provided in the request
	if req.Name != nil {
		useCaseReq.Name = req.Name
	}
	if req.CompanyName != nil {
		useCaseReq.CompanyName = req.CompanyName
	}
	if req.Subdomain != nil {
		useCaseReq.Subdomain = req.Subdomain
	}
	if req.Description != nil {
		useCaseReq.Description = req.Description
	}
	if req.IsActive != nil {
		useCaseReq.IsActive = req.IsActive
	}
	if req.ContactEmail != nil {
		useCaseReq.ContactEmail = req.ContactEmail
	}
	if req.CompanyWebsite != nil {
		useCaseReq.CompanyWebsite = req.CompanyWebsite
	}

	// Handle theme updates
	if req.Theme != nil {
		themeUpdate := &ucUpdateTenant.ThemeUpdate{}
		if req.Theme.PrimaryColor != nil {
			themeUpdate.PrimaryColor = req.Theme.PrimaryColor
		}
		if req.Theme.SecondaryColor != nil {
			themeUpdate.SecondaryColor = req.Theme.SecondaryColor
		}
		if req.Theme.AccentColor != nil {
			themeUpdate.AccentColor = req.Theme.AccentColor
		}
		if req.Theme.TextColor != nil {
			themeUpdate.TextColor = req.Theme.TextColor
		}
		if req.Theme.BackgroundColor != nil {
			themeUpdate.BackgroundColor = req.Theme.BackgroundColor
		}
		if req.Theme.LogoUrl != nil {
			themeUpdate.LogoUrl = req.Theme.LogoUrl
		}
		if req.Theme.FaviconUrl != nil {
			themeUpdate.FaviconUrl = req.Theme.FaviconUrl
		}
		useCaseReq.Theme = themeUpdate
	}

	// Handle address updates
	if req.Address != nil {
		addressUpdate := &ucUpdateTenant.AddressUpdate{}
		if req.Address.Street != nil {
			addressUpdate.Street = req.Address.Street
		}
		if req.Address.City != nil {
			addressUpdate.City = req.Address.City
		}
		if req.Address.State != nil {
			addressUpdate.State = req.Address.State
		}
		if req.Address.PostalCode != nil {
			addressUpdate.PostalCode = req.Address.PostalCode
		}
		if req.Address.Country != nil {
			addressUpdate.Country = req.Address.Country
		}
		useCaseReq.Address = addressUpdate
	}

	// Handle settings updates
	if req.Settings != nil {
		settingsUpdate := &ucUpdateTenant.SettingsUpdate{}
		if req.Settings.Locale != nil {
			settingsUpdate.Locale = req.Settings.Locale
		}
		if req.Settings.Timezone != nil {
			settingsUpdate.Timezone = req.Settings.Timezone
		}
		if req.Settings.DateFormat != nil {
			settingsUpdate.DateFormat = req.Settings.DateFormat
		}
		if req.Settings.TimeFormat != nil {
			settingsUpdate.TimeFormat = req.Settings.TimeFormat
		}
		if req.Settings.Currency != nil {
			settingsUpdate.Currency = req.Settings.Currency
		}
		useCaseReq.Settings = settingsUpdate
	}

	tenant, err := h.useCase.Update(c.Request.Context(), id, useCaseReq)
	if err != nil {
		if err == ucUpdateTenant.ErrTenantNotFound {
			c.JSON(http.StatusNotFound, tenantserrors.ErrorResponse{Error: "NOT_FOUND", Message: "Tenant no encontrado", Status: http.StatusNotFound})
			return
		}
		log.Printf("error updating tenant: %v", err)
		c.JSON(http.StatusInternalServerError, tenantserrors.ErrorResponse{Error: "INTERNAL_ERROR", Message: "Failed to update tenant", Status: http.StatusInternalServerError})
		return
	}

	response := models.FromDomain(tenant)
	c.JSON(http.StatusOK, response)
}
```

- [ ] **Step 5: Verify it compiles**

Run: `go build ./...`
Expected: no output, exit code 0.

- [ ] **Step 6: Write the tests**

Edit `internal/api/handler/tenants/update_tenant/update_tenant_test.go` — replace the full file:

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
		Settings: domain.TenantSettings{
			ContactEmail:   "",
			CompanyWebsite: "",
			Locale:         "es-AR",
			Timezone:       "America/Argentina/Buenos_Aires",
			DateFormat:     "dd/MM/yyyy",
			TimeFormat:     "HH:mm",
			Currency:       "ARS",
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

func newTestHandler() (*UpdateTenantHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	useCase := ucUpdateTenant.NewUseCase(&mockRepo{})
	h := NewUpdateTenantHandler(useCase)
	r := gin.Default()
	r.PATCH("/api/v1/tenants/:tenantId", h.UpdateTenant)
	return h, r
}

func doPatch(t *testing.T, r *gin.Engine, id string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest("PATCH", "/api/v1/tenants/"+id, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUpdateTenantHandler(t *testing.T) {
	_, r := newTestHandler()

	id := uuid.New().String()
	updateReq := models.TenantUpdateRequest{
		Name:           ptrString("Updated Tenant Name"),
		Description:    ptrString("Updated description"),
		IsActive:       ptrBool(true),
		ContactEmail:   ptrString("contacto@empresa.com"),
		CompanyWebsite: ptrString("https://empresa.com"),
		Theme: &models.ThemeUpdate{
			PrimaryColor: ptrString("#4f46e5"),
		},
		Settings: &models.SettingsUpdate{
			Locale:     ptrString("en-US"),
			Timezone:   ptrString("UTC"),
			DateFormat: ptrString("yyyy-MM-dd"),
			TimeFormat: ptrString("hh:mm a"),
			Currency:   ptrString("USD"),
		},
	}
	body, _ := json.Marshal(updateReq)
	w := doPatch(t, r, id, body)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.TenantResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Tenant Name", resp.Name)
	assert.Equal(t, "Updated description", resp.Description)
	assert.Equal(t, true, resp.IsActive)
	assert.Equal(t, "#4f46e5", resp.Theme.PrimaryColor)
	assert.Equal(t, "contacto@empresa.com", resp.ContactEmail)
	assert.Equal(t, "https://empresa.com", resp.CompanyWebsite)
	assert.Equal(t, "en-US", resp.Settings.Locale)
	assert.Equal(t, "UTC", resp.Settings.Timezone)
	assert.Equal(t, "yyyy-MM-dd", resp.Settings.DateFormat)
	assert.Equal(t, "hh:mm a", resp.Settings.TimeFormat)
	assert.Equal(t, "USD", resp.Settings.Currency)
}

func TestUpdateTenantHandler_InvalidID(t *testing.T) {
	_, r := newTestHandler()

	updateReq := models.TenantUpdateRequest{
		Name: ptrString("Updated Tenant Name"),
	}
	body, _ := json.Marshal(updateReq)
	w := doPatch(t, r, "invalid-id", body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTenantHandler_InvalidSettings(t *testing.T) {
	_, r := newTestHandler()

	id := uuid.New().String()
	updateReq := models.TenantUpdateRequest{
		Settings: &models.SettingsUpdate{
			Locale: ptrString("xx-XX"),
		},
	}
	body, _ := json.Marshal(updateReq)
	w := doPatch(t, r, id, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateTenantHandler_InvalidContactEmail(t *testing.T) {
	_, r := newTestHandler()

	id := uuid.New().String()
	updateReq := models.TenantUpdateRequest{
		ContactEmail: ptrString("no-es-un-email"),
	}
	body, _ := json.Marshal(updateReq)
	w := doPatch(t, r, id, body)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func ptrString(s string) *string { return &s }
func ptrBool(b bool) *bool       { return &b }
```

- [ ] **Step 7: Run the tests**

Run: `go test ./internal/api/handler/tenants/update_tenant/... -v`
Expected: all 4 tests pass — `TestUpdateTenantHandler`, `TestUpdateTenantHandler_InvalidID`, `TestUpdateTenantHandler_InvalidSettings`, `TestUpdateTenantHandler_InvalidContactEmail` (Task 3 already added `ContactEmail`/`CompanyWebsite`/`Settings` to `models.TenantResponse`, so this compiles and passes cleanly).

- [ ] **Step 8: Commit**

```bash
git add internal/api/handler/tenants/update_tenant/models/request.go internal/api/handler/tenants/update_tenant/update_tenant.go internal/api/usecases/tenants/update_tenant/usecase.go internal/api/handler/tenants/update_tenant/update_tenant_test.go
git commit -m "feat: validate and persist tenant settings fields in the update-tenant flow"
```

---

## Task 5: Create-tenant defaults

**Files:**
- Modify: `internal/api/handler/tenants/create_tenant/models/request.go`
- Create: `internal/api/handler/tenants/create_tenant/models/request_test.go`

**Interfaces:**
- Consumes: `domain.TenantSettings` from Task 1.
- Produces: `Parse()` sets `tenant.Settings` to the fixed defaults (`es-AR`, `America/Argentina/Buenos_Aires`, `dd/MM/yyyy`, `HH:mm`, `ARS`, empty contact/website) on every newly created tenant. No other task consumes this directly.

- [ ] **Step 1: Write the failing test**

This package already has `internal/api/handler/tenants/create_tenant/models/response_test.go` from Task 3, in the same package (`models`) — the new test below lives alongside it as a separate file. Create `internal/api/handler/tenants/create_tenant/models/request_test.go`:

```go
package models

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SetsSettingsDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	reqBody := TenantRequest{
		Name:        "New Tenant",
		CompanyName: "New Tenant Co",
		Subdomain:   "new-tenant",
		AdminUser: AdminUser{
			Email:     "admin@newtenant.com",
			FirstName: "Admin",
			LastName:  "User",
			Password:  "password123",
		},
		Theme: ThemeRequest{
			PrimaryColor: "#000000",
		},
		Address: AddressRequest{
			Street:     "Main St 1",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1001",
			Country:    "Argentina",
		},
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	httpReq, _ := http.NewRequest("POST", "/api/v1/tenants", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httpReq

	tenant, err := Parse(c)
	require.NoError(t, err)

	assert.Equal(t, "es-AR", tenant.Settings.Locale)
	assert.Equal(t, "America/Argentina/Buenos_Aires", tenant.Settings.Timezone)
	assert.Equal(t, "dd/MM/yyyy", tenant.Settings.DateFormat)
	assert.Equal(t, "HH:mm", tenant.Settings.TimeFormat)
	assert.Equal(t, "ARS", tenant.Settings.Currency)
	assert.Equal(t, "", tenant.Settings.ContactEmail)
	assert.Equal(t, "", tenant.Settings.CompanyWebsite)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/api/handler/tenants/create_tenant/models/... -run TestParse_SetsSettingsDefaults -v`
Expected: FAIL — `tenant.Settings.Locale` compares `""` to `"es-AR"` (the field exists from Task 1 but `Parse()` never sets it, so it's zero-valued).

- [ ] **Step 3: Implement the minimal change**

Edit `internal/api/handler/tenants/create_tenant/models/request.go` — in `Parse()`, add the `Settings` field to the constructed `tenant` literal, right after `Address`:

```go
	tenant := &domain.Tenant{
		ID:          uuid.New(),
		Name:        req.Name,
		CompanyName: req.CompanyName,
		Subdomain:   req.Subdomain,
		Description: req.Description,
		IsActive:    true,
		Theme: domain.Theme{
			PrimaryColor:    req.Theme.PrimaryColor,
			SecondaryColor:  req.Theme.SecondaryColor,
			AccentColor:     req.Theme.AccentColor,
			TextColor:       req.Theme.TextColor,
			BackgroundColor: req.Theme.BackgroundColor,
			LogoUrl:         "",
			FaviconUrl:      "/favicon.ico",
		},
		Address: domain.Address{
			Street:     req.Address.Street,
			City:       req.Address.City,
			State:      req.Address.State,
			PostalCode: req.Address.PostalCode,
			Country:    req.Address.Country,
		},
		Settings: domain.TenantSettings{
			Locale:     "es-AR",
			Timezone:   "America/Argentina/Buenos_Aires",
			DateFormat: "dd/MM/yyyy",
			TimeFormat: "HH:mm",
			Currency:   "ARS",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
```

(`ContactEmail`/`CompanyWebsite` are omitted from the literal, which leaves them at their zero value `""` — matching the spec's intent that the admin fills them in later from `/settings`.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/api/handler/tenants/create_tenant/models/... -run TestParse_SetsSettingsDefaults -v`
Expected: `--- PASS: TestParse_SetsSettingsDefaults`.

- [ ] **Step 5: Run the full suite to confirm no regressions**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests `ok` or `[no test files]` (or `SKIP` for the Task 2 integration tests if `DATABASE_URL` is unset in this shell).

- [ ] **Step 6: Commit**

```bash
git add internal/api/handler/tenants/create_tenant/models/request.go internal/api/handler/tenants/create_tenant/models/request_test.go
git commit -m "feat: default new tenants' settings fields on creation"
```
