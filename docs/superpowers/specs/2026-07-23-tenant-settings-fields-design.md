# Soporte de campos adicionales de tenant — Design Spec

**Fecha:** 2026-07-23
**Feature:** Agregar persistencia real en el backend para 7 campos de tenant (`contactEmail`, `companyWebsite`, `locale`, `timezone`, `dateFormat`, `timeFormat`, `currency`) que el frontend de `/settings` ya envía pero el backend descarta en silencio hoy.
**Repo:** embolsadora4.0-cloud
**Estado:** Aprobado, pendiente de plan de implementación

---

## Contexto

Durante el ciclo de fix de `/settings` + `/account` en `embolsadora-frontend`, se descubrió que el formulario de `/settings` ya envía `contactEmail`, `companyWebsite` (a nivel raíz del PATCH) y `dateFormat`, `timeFormat`, `currency` (anidados en `settings`, junto a `locale`/`timezone` que también se descartan hoy) — pero el backend Go no tiene **ningún** soporte para estos 7 campos en ninguna capa: ni en `domain.Tenant`, ni en la tabla `tenants`, ni en el repositorio, ni en los DTOs de request/response.

Confirmado por grep en todo el repo: cero referencias a `TenantSettings`, `contactEmail`, `companyWebsite` en `internal/`. Esto no es un bug — es una funcionalidad nunca implementada. El usuario pidió explícitamente un "fix integral": agregar soporte real de persistencia, no solo evitar que el frontend los descarte.

Este sub-proyecto es un prerequisito para completar el fix de `/settings` en el frontend (branch `fix/settings-account-fix`), que queda pausado hasta que este trabajo backend esté mergeado.

**Decisiones de diseño ya resueltas con el usuario:**
1. Columnas planas en Postgres (no JSONB) — sigue el patrón 100% existente (`Theme`/`Address` ya son columnas planas).
2. Los 7 campos se agrupan en un único sub-objeto de dominio nuevo, `domain.TenantSettings`, análogo a `Theme`/`Address`.
3. Se mantiene el patrón de duplicación de los 4 `TenantResponse` (no se consolidan en este sub-proyecto).
4. `locale`, `timezone`, `dateFormat`, `timeFormat`, `currency` se validan contra un catálogo fijo (enum) tanto en el backend como en el frontend, con los mismos valores exactos en ambos lados. `contactEmail`/`companyWebsite` no llevan catálogo (son datos libres del tenant, solo se validan como email/URL).

**Nuance descubierta durante el diseño:** aunque los 7 campos viven juntos en `domain.TenantSettings` (dominio interno), el contrato JSON de la request/response mantiene `contactEmail`/`companyWebsite` a nivel raíz del tenant y el resto anidado bajo `settings` — porque el frontend ya existente (`settings-client.tsx`) envía dos PATCH separados con esa forma exacta (`{contactEmail, companyWebsite, ...}` desde la card "Información", `{settings: {locale, timezone, dateFormat, timeFormat, currency}}` desde la card "Preferencias"). El DTO del handler mapea ambas formas hacia el mismo struct de dominio.

---

## Sección 1: Dominio y migración

### `internal/domain/tenants.go`

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

type Tenant struct {
	// ... campos existentes (ID, Name, CompanyName, Subdomain, Description, IsActive, Theme, Address, CreatedAt, UpdatedAt)
	Settings TenantSettings `json:"settings" db:"settings"`
}
```

### Migración nueva: `migrations/000005_add_tenant_settings.up.sql`

Columnas `NOT NULL DEFAULT` con los mismos defaults que ya usa el frontend (`settings-client.tsx`), más `CHECK` de catálogo para los 5 campos con enum (no para `contact_email`/`company_website`, que son datos libres):

```sql
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

`migrations/000005_add_tenant_settings.down.sql`: `ALTER TABLE tenants DROP COLUMN contact_email, DROP COLUMN company_website, DROP COLUMN locale, DROP COLUMN timezone, DROP COLUMN date_format, DROP COLUMN time_format, DROP COLUMN currency;`

---

## Sección 2: Repositorio y queries SQL

`internal/repo/pg/tenants/resources.go` — `FindByIDQuery`, `CreateQuery`, `FindAllQuery`, `UpdateQuery` ganan las 7 columnas:

```go
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

UpdateQuery = `
	UPDATE tenants SET
		name = $1, company_name = $2, subdomain = $3, description = $4, is_active = $5,
		primary_color = $6, secondary_color = $7, accent_color = $8, text_color = $9, background_color = $10, logo_url = $11, favicon_url = $12,
		street = $13, city = $14, state = $15, postal_code = $16, country = $17,
		contact_email = $18, company_website = $19, locale = $20, timezone = $21, date_format = $22, time_format = $23, currency = $24,
		updated_at = $25
	WHERE id = $26
`
```

`internal/repo/pg/tenants/repository.go` — en `FindByID`, `FindAll`, `Create`, `Update`: agregar `&tenant.Settings.ContactEmail, &tenant.Settings.CompanyWebsite, &tenant.Settings.Locale, &tenant.Settings.Timezone, &tenant.Settings.DateFormat, &tenant.Settings.TimeFormat, &tenant.Settings.Currency` a los `Scan()`/parámetros de `Exec()`, en el mismo orden que las queries. Son columnas `NOT NULL`, así que se escanean directo a `string` (sin puntero/`derefString`, a diferencia de `description`/`street` que sí son nullable).

---

## Sección 3: DTOs (handler + usecase) y validación

### `internal/api/handler/tenants/update_tenant/models/request.go`

```go
package models

type ThemeUpdate struct {
	PrimaryColor    *string `json:"primaryColor"`
	SecondaryColor  *string `json:"secondaryColor"`
	AccentColor     *string `json:"accentColor"`
	TextColor       *string `json:"textColor"`
	BackgroundColor *string `json:"backgroundColor"`
	LogoUrl         *string `json:"logoUrl"`
	FaviconUrl      *string `json:"faviconUrl"`
}

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

### Validación de catálogo (`internal/api/handler/tenants/update_tenant/update_tenant.go`)

Este handler nunca usó tags `binding:` (a diferencia de `create_tenant`) — se mantiene el estilo manual field-by-field ya existente en el archivo:

```go
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
```

En `UpdateTenant`, después de `ShouldBindJSON` y antes de armar `useCaseReq`:
- Si `req.ContactEmail != nil`: validar con `net/mail.ParseAddress(*req.ContactEmail)`. Inválido → 400 `"Email de contacto inválido"`.
- Si `req.CompanyWebsite != nil && *req.CompanyWebsite != ""`: validar con `net/url.ParseRequestURI(*req.CompanyWebsite)` (acepta `""` igual que el frontend, que permite el campo vacío). Inválido → 400 `"Sitio web inválido"`.
- Si `req.Settings != nil`: para cada campo no-nil, chequear `isOneOf` contra su catálogo. Inválido → 400 `"<campo> inválido"`.

Todas las respuestas de error usan el mismo formato ya existente: `tenantserrors.ErrorResponse{Error: "BAD_REQUEST", Message: "...", Status: http.StatusBadRequest}`.

### `internal/api/usecases/tenants/update_tenant/usecase.go`

```go
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

type SettingsUpdate struct {
	Locale     *string
	Timezone   *string
	DateFormat *string
	TimeFormat *string
	Currency   *string
}
```

En `Update()`, agregar tras el bloque de `Address`:

```go
if req.ContactEmail != nil {
	tenant.Settings.ContactEmail = *req.ContactEmail
}
if req.CompanyWebsite != nil {
	tenant.Settings.CompanyWebsite = *req.CompanyWebsite
}
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
```

En el handler (`update_tenant.go`), mapear `req.ContactEmail`/`req.CompanyWebsite`/`req.Settings.*` (DTO handler) hacia `useCaseReq.ContactEmail`/`useCaseReq.CompanyWebsite`/`useCaseReq.Settings` (DTO usecase), mismo patrón ya usado para `Theme`/`Address`.

### Creación de tenants (`internal/api/handler/tenants/create_tenant/models/request.go`, `Parse()`)

`CreateQuery` inserta las 27 columnas explícitas — los `DEFAULT` de la migración nunca se aplican en un INSERT explícito. Hay que fijar los defaults en código, igual que ya se hace con `FaviconUrl: "/favicon.ico"`:

```go
Settings: domain.TenantSettings{
	Locale:     "es-AR",
	Timezone:   "America/Argentina/Buenos_Aires",
	DateFormat: "dd/MM/yyyy",
	TimeFormat: "HH:mm",
	Currency:   "ARS",
	// ContactEmail, CompanyWebsite quedan "" — el admin las completa después desde /settings
},
```

---

## Sección 4: DTOs de respuesta (los 4 duplicados)

Mismo shape JSON que la request: `contactEmail`/`companyWebsite` a nivel raíz, resto anidado en `settings`. Se replica idéntico en `create_tenant/models/response.go`, `get_tenant/models/response.go`, `update_tenant/models/response.go`, `get_all_tenants/models/response.go`:

```go
// Settings represents the localization/preferences configuration for a tenant
type Settings struct {
	Locale     string `json:"locale"`
	Timezone   string `json:"timezone"`
	DateFormat string `json:"dateFormat"`
	TimeFormat string `json:"timeFormat"`
	Currency   string `json:"currency"`
}

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

`get_all_tenants/models/response.go` aplica el mismo `FromDomain` por cada tenant del slice — confirmar su wrapper exacto al escribir el plan (puede envolver la lista en un tipo distinto).

---

## Sección 5: Testing

Patrón ya existente: tests con `mockRepo` en `*_test.go`, corridos con `go test ./...` (sin Docker — confirmado que `go` corre nativo en este entorno pese a lo que indica `CLAUDE.md`).

- `internal/api/handler/tenants/update_tenant/update_tenant_test.go`:
  - `mockRepo.FindByID` gana `Settings: domain.TenantSettings{Locale: "es-AR", Timezone: "America/Argentina/Buenos_Aires", DateFormat: "dd/MM/yyyy", TimeFormat: "HH:mm", Currency: "ARS"}` en el tenant de prueba.
  - Ampliar `TestUpdateTenantHandler`: PATCH con `contactEmail`, `companyWebsite`, `settings.{locale,timezone,dateFormat,timeFormat,currency}` válidos → 200, response refleja los valores enviados.
  - Nuevo `TestUpdateTenantHandler_InvalidSettings`: PATCH con `settings.locale = "xx-XX"` (fuera de catálogo) → 400.
  - Nuevo `TestUpdateTenantHandler_InvalidContactEmail`: PATCH con `contactEmail = "no-es-un-email"` → 400.
- Verificación manual de la migración contra una DB local: `migrate -path migrations/ -database $DATABASE_URL up`, confirmar que las 7 columnas existen con los defaults correctos y que los `CHECK` rechazan valores fuera de catálogo.
- Verificación completa antes de cerrar cada tarea: `go build ./...` && `go test ./...` (baseline ya confirmado limpio: todos los tests pasan, sin fallos, antes de empezar este sub-proyecto).

---

## Non-goals

- No se toca el frontend en este sub-proyecto — ese trabajo ya vive en `embolsadora-frontend` branch `fix/settings-account-fix`, pendiente de retomar una vez este backend esté mergeado.
- No se consolidan los 4 `TenantResponse` duplicados — se replica el cambio en los 4; la consolidación queda como refactor futuro, no mezclado con esta feature.
- No se auto-genera `tenants.json` — sub-proyecto de arquitectura futuro, ya diferido en el spec del frontend.
- No se resuelve Problema 5 del ciclo frontend (name/lastname "user not found") — ya documentado como fuera de alcance ahí.
- No se agrega soporte para editar `subdomain` a través de estos campos nuevos — no forma parte del pedido original.
