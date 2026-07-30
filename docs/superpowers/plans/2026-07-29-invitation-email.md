# Mails de autenticación: plantillas propias y URL por instancia — Plan de Implementación

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Que el mail de invitación llegue con la URL de la instancia desde la que se disparó y con una plantilla propia en español, y que el reset de contraseña efectivamente se envíe.

**Architecture:** El BFF de Next.js manda el origin real del navegador en un header `X-App-Base-URL`; un middleware del backend Go lo valida contra un allow-list de origins y lo deja en el contexto del request; los usecases construyen el `redirect_to` con ese valor en lugar de una env var fija. En paralelo, el backend enriquece la invitación con nombre de tenant, de rol y de quien invita vía `user_metadata`, y cuatro plantillas HTML versionadas en el repo reemplazan las default de Supabase.

**Tech Stack:** Go 1.24 (Gin, pgx/v5, Zap, testify), Next.js 16 (App Router, BFF), Supabase Auth / GoTrue, Resend (SMTP), Supabase Management API.

**Spec:** `docs/superpowers/specs/2026-07-29-invitation-email-design.md`

## Global Constraints

- **Go no está instalado en el host.** Todo comando `go` corre en Docker. Comando base, desde la raíz del repo backend:
  ```bash
  docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "<comando go>"
  ```
- **Dos repos.** Las tareas 1–7 y 9–11 son en `~/Develop/UTN/embolsadora4.0-cloud`. La tarea 8 es en `~/Develop/UTN/embolsadora-frontend`. Cada repo tiene su propio git; los commits no se mezclan.
- **El frontend no tiene runner de tests.** El script `test` de `package.json` es un stub (`echo "Tests not implemented yet"`). La verificación del frontend es `pnpm tsc --noEmit && pnpm lint && pnpm build`. No escribir tests unitarios de frontend: no hay dónde correrlos.
- **El allow-list de origins se compara por igualdad exacta, nunca por prefijo.** El resultado se usa para armar un link que se manda por mail; un match por prefijo es un open-redirect con entrega incluida.
- **Ninguna falla de datos decorativos puede abortar un envío.** Si no se resuelve el nombre del tenant o del rol, el mail sale igual con la copy genérica.
- **Copy en español rioplatense (voseo).** "Aceptá", "copiá", "ignorá". Nunca "acepta"/"copia"/"ignora".
- **Módulo Go:** `github.com/tu-org/embolsadora-api`.
- **Proyecto Supabase:** ref `cdjehkbidqqsldaajbui`. Tenant de plataforma MRG: `11b36b85-033d-4bb3-9e31-4c92161887c0`.

---

### Task 1: Paquete `apporigin` — validación del allow-list

Es la única pieza del plan con superficie de seguridad. Vive aislada, sin dependencias del dominio, y se testea sola.

**Files:**
- Create: `internal/platform/apporigin/allowlist.go`
- Test: `internal/platform/apporigin/allowlist_test.go`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `apporigin.AllowList` (struct opaco)
  - `apporigin.Parse(raw string) AllowList`
  - `apporigin.Normalize(raw string) (string, bool)`
  - `(AllowList) Allows(origin string) bool`
  - `(AllowList) Resolve(candidate, fallback string) (string, bool)`

- [ ] **Step 1: Escribir el test que falla**

Crear `internal/platform/apporigin/allowlist_test.go`:

```go
package apporigin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
)

const fallback = "https://embolsadora.site"

func TestResolve(t *testing.T) {
	list := apporigin.Parse("https://embolsadora.site,http://localhost:3000,https://*.vercel.app")

	tests := []struct {
		name      string
		candidate string
		want      string
		wantOK    bool
	}{
		{"origin exacto de produccion", "https://embolsadora.site", "https://embolsadora.site", true},
		{"localhost con puerto", "http://localhost:3000", "http://localhost:3000", true},
		{"barra final se normaliza", "https://embolsadora.site/", "https://embolsadora.site", true},
		{"mayusculas se normalizan", "HTTPS://EMBOLSADORA.SITE", "https://embolsadora.site", true},
		{"el path se descarta", "https://embolsadora.site/s/demo/auth/callback", "https://embolsadora.site", true},
		{"espacios alrededor se recortan", "  https://embolsadora.site  ", "https://embolsadora.site", true},
		{"ataque por sufijo se rechaza", "https://embolsadora.site.atacante.com", fallback, false},
		{"ataque por path se rechaza", "https://atacante.com/embolsadora.site", fallback, false},
		{"esquema incorrecto se rechaza", "http://embolsadora.site", fallback, false},
		{"puerto incorrecto se rechaza", "http://localhost:4000", fallback, false},
		{"preview de vercel se acepta", "https://embolsadora-abc123.vercel.app", "https://embolsadora-abc123.vercel.app", true},
		{"dominio pelado del wildcard se rechaza", "https://vercel.app", fallback, false},
		{"wildcard sobre http se rechaza", "http://x.vercel.app", fallback, false},
		{"lookalike del wildcard se rechaza", "https://evilvercel.app", fallback, false},
		{"vacio cae al fallback", "", fallback, false},
		{"url relativa cae al fallback", "/s/demo/auth/callback", fallback, false},
		{"esquema javascript se rechaza", "javascript:alert(1)", fallback, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := list.Resolve(tc.candidate, fallback)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolve_AllowListVaciaSiempreCaeAlFallback(t *testing.T) {
	list := apporigin.Parse("")
	got, ok := list.Resolve("https://embolsadora.site", fallback)
	assert.False(t, ok)
	assert.Equal(t, fallback, got, "sin allow-list configurada no se confia en ningun header")
}

func TestParse_EntradasInvalidasSeIgnoran(t *testing.T) {
	list := apporigin.Parse("no-es-una-url, ,https://embolsadora.site,ftp://embolsadora.site")
	assert.True(t, list.Allows("https://embolsadora.site"))
	assert.False(t, list.Allows("ftp://embolsadora.site"))
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/platform/apporigin/... -v"
```

Esperado: FAIL — el paquete `apporigin` no existe todavía.

- [ ] **Step 3: Escribir la implementación**

Crear `internal/platform/apporigin/allowlist.go`:

```go
// Package apporigin valida el origin del frontend que un request dice tener,
// para construir links que se envian por mail. Es la unica barrera entre un
// header controlable por el llamador y una URL que termina en la casilla de
// un usuario, asi que el matching es exacto por diseño.
package apporigin

import (
	"net/url"
	"strings"
)

// AllowList es el conjunto de origins en los que el backend confia.
type AllowList struct {
	exact    []string
	wildcard []wildcardEntry
}

type wildcardEntry struct {
	scheme string
	suffix string // incluye el punto inicial, p.ej. ".vercel.app"
}

// Parse construye una AllowList a partir de una lista separada por comas.
// Una entrada con la forma "https://*.example.com" acepta cualquier subdominio
// de example.com bajo ese esquema, pero nunca example.com a secas. Las entradas
// que no parsean se descartan en silencio: una config con una entrada rota no
// debe voltear el arranque del servicio.
func Parse(raw string) AllowList {
	var a AllowList
	for _, item := range strings.Split(raw, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if scheme, rest, found := strings.Cut(item, "://"); found && strings.HasPrefix(rest, "*.") {
			if scheme != "http" && scheme != "https" {
				continue
			}
			a.wildcard = append(a.wildcard, wildcardEntry{scheme: scheme, suffix: rest[1:]})
			continue
		}
		if origin, ok := Normalize(item); ok {
			a.exact = append(a.exact, origin)
		}
	}
	return a
}

// Normalize reduce una URL cruda a su origin: esquema://host[:puerto], en
// minusculas y sin path, query, fragment, userinfo ni barra final. Devuelve
// false si el valor no es una URL absoluta http(s).
func Normalize(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

// Allows indica si el origin ya normalizado esta permitido.
func (a AllowList) Allows(origin string) bool {
	for _, e := range a.exact {
		if e == origin {
			return true
		}
	}
	for _, w := range a.wildcard {
		prefix := w.scheme + "://"
		if !strings.HasPrefix(origin, prefix) {
			continue
		}
		host := origin[len(prefix):]
		// Exigir al menos una etiqueta propia antes del sufijo: con
		// "https://*.vercel.app", tanto "vercel.app" como ".vercel.app"
		// tienen que quedar afuera.
		if len(host) > len(w.suffix) && strings.HasSuffix(host, w.suffix) {
			return true
		}
	}
	return false
}

// Resolve devuelve el base URL a usar para links salientes. Cuando candidate
// esta vacio o no esta permitido devuelve fallback y false, para que el
// llamador pueda loguear el rechazo sin hacer fallar el request.
func (a AllowList) Resolve(candidate, fallback string) (string, bool) {
	origin, ok := Normalize(candidate)
	if !ok || !a.Allows(origin) {
		return fallback, false
	}
	return origin, true
}
```

- [ ] **Step 4: Correr el test y verificar que pasa**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/platform/apporigin/... -v"
```

Esperado: PASS, con los 17 subtests de `TestResolve` en verde.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/apporigin/
git commit -m "feat(apporigin): allow-list de origins con match exacto para links salientes"
```

---

### Task 2: Config — `APP_ALLOWED_ORIGINS`

**Files:**
- Modify: `internal/config/config.go:63-71` (struct `SupabaseConfig`) y `internal/config/config.go:117-126` (bloque `Supabase:` dentro de `Load`)
- Modify: `.env.example:24`

**Interfaces:**
- Consumes: nada.
- Produces: `config.SupabaseConfig.AppAllowedOrigins string` — el valor crudo de la env var, sin parsear. El parseo vive en el wiring (Task 4).

- [ ] **Step 1: Agregar el campo al struct**

En `internal/config/config.go`, dentro de `type SupabaseConfig struct`, agregar debajo de `AppBaseURL`:

```go
	AppBaseURL          string
	AppAllowedOrigins   string
	InviteRateLimitHour int
```

- [ ] **Step 2: Leer la env var en `Load`**

En el bloque `Supabase: SupabaseConfig{...}` dentro de `Load`, agregar debajo de la línea de `AppBaseURL`:

```go
			AppBaseURL:          require("APP_BASE_URL"),
			AppAllowedOrigins:   getEnv("APP_ALLOWED_ORIGINS", ""),
			InviteRateLimitHour: getIntEnv("INVITATION_RATE_LIMIT_PER_HOUR", 20),
```

Usa `getEnv` y no `require`: sin allow-list el sistema sigue funcionando con el fallback de `APP_BASE_URL`, que es exactamente el comportamiento de hoy. No debe volverse un requisito de arranque.

- [ ] **Step 3: Documentar la variable en `.env.example`**

En `.env.example`, debajo de la línea `APP_BASE_URL=http://localhost:3000`, agregar:

```
# Origins del frontend en los que se confia para armar links de mail.
# El BFF manda el suyo en el header X-App-Base-URL y se valida contra esta lista.
# Una entrada "https://*.vercel.app" habilita los previews (subdominios unicamente).
# Si el header falta o no matchea, se usa APP_BASE_URL.
APP_ALLOWED_ORIGINS=http://localhost:3000
```

- [ ] **Step 4: Verificar que compila**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./..."
```

Esperado: sin salida (éxito).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go .env.example
git commit -m "feat(config): agrega APP_ALLOWED_ORIGINS con fallback a APP_BASE_URL"
```

---

### Task 3: Contexto y middleware del base URL

**Files:**
- Modify: `internal/platform/tenantctx.go` (agregar al final del archivo, y una key nueva en el bloque de keys del tope)
- Modify: `internal/api/middleware/middleware.go` (agregar la función al final del archivo)
- Test: `internal/api/middleware/app_base_url_test.go`

**Interfaces:**
- Consumes: `apporigin.AllowList`, `apporigin.Resolve` (Task 1).
- Produces:
  - `platform.WithAppBaseURL(ctx context.Context, baseURL string) context.Context`
  - `platform.AppBaseURL(ctx context.Context) string`
  - `middleware.AppBaseURLFromHeader(allow apporigin.AllowList, fallback string) gin.HandlerFunc`

- [ ] **Step 1: Escribir el test que falla**

Crear `internal/api/middleware/app_base_url_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
)

const fallbackBase = "https://embolsadora.site"

// runWithHeader ejecuta un request a traves del middleware y devuelve el base
// URL que quedo en el contexto del handler.
func runWithHeader(t *testing.T, header string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)

	allow := apporigin.Parse("https://embolsadora.site,http://localhost:3000")
	var seen string

	r := gin.New()
	r.Use(apimw.AppBaseURLFromHeader(allow, fallbackBase))
	r.GET("/probe", func(c *gin.Context) {
		seen = platform.AppBaseURL(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if header != "" {
		req.Header.Set("X-App-Base-URL", header)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "el middleware nunca debe abortar el request")
	return seen
}

func TestAppBaseURLFromHeader_OriginPermitido(t *testing.T) {
	assert.Equal(t, "http://localhost:3000", runWithHeader(t, "http://localhost:3000"))
}

func TestAppBaseURLFromHeader_OriginRechazadoCaeAlFallback(t *testing.T) {
	assert.Equal(t, fallbackBase, runWithHeader(t, "https://embolsadora.site.atacante.com"))
}

func TestAppBaseURLFromHeader_SinHeaderCaeAlFallback(t *testing.T) {
	assert.Equal(t, fallbackBase, runWithHeader(t, ""))
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/middleware/... -run TestAppBaseURLFromHeader -v"
```

Esperado: FAIL de compilación — `AppBaseURLFromHeader` y `platform.AppBaseURL` no existen.

- [ ] **Step 3: Agregar los helpers de contexto**

En `internal/platform/tenantctx.go`, agregar la key al bloque de tipos del tope:

```go
type tenantUUIDKeyType struct{}
type appBaseURLKeyType struct{}
```

y a continuación, junto a las otras variables de key:

```go
var tenantUUIDKey = tenantUUIDKeyType{}
var appBaseURLKey = appBaseURLKeyType{}
```

Al final del archivo, agregar:

```go
// WithAppBaseURL guarda el base URL del frontend ya validado para este request.
// Es el origin desde el que se disparo la accion, y determina a donde apuntan
// los links que se envian por mail.
func WithAppBaseURL(ctx context.Context, baseURL string) context.Context {
	return context.WithValue(ctx, appBaseURLKey, baseURL)
}

// AppBaseURL extrae el base URL del frontend establecido para este request.
// Devuelve string vacio si ningun middleware lo seteo, en cuyo caso el llamador
// debe usar su propio valor por defecto.
func AppBaseURL(ctx context.Context) string {
	if s, ok := ctx.Value(appBaseURLKey).(string); ok {
		return s
	}
	return ""
}
```

- [ ] **Step 4: Agregar el middleware**

En `internal/api/middleware/middleware.go`, agregar el import:

```go
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
```

y al final del archivo:

```go
// AppBaseURLFromHeader resuelve el origin del frontend que origino este request
// a partir del header X-App-Base-URL, validado contra el allow-list, y lo deja
// en el contexto. Un valor ausente o rechazado cae al default configurado: un
// mail con la URL de fallback vale mas que una invitacion que nunca se manda.
func AppBaseURLFromHeader(allow apporigin.AllowList, fallback string) gin.HandlerFunc {
	return func(c *gin.Context) {
		candidate := c.GetHeader("X-App-Base-URL")
		resolved, ok := allow.Resolve(candidate, fallback)
		if !ok && candidate != "" {
			Log.Warn("X-App-Base-URL rechazado, usando fallback",
				zap.String("candidate", candidate),
				zap.String("fallback", fallback),
				zap.String("endpoint", c.Request.URL.Path),
			)
		}
		c.Request = c.Request.WithContext(platform.WithAppBaseURL(c.Request.Context(), resolved))
		c.Next()
	}
}
```

- [ ] **Step 5: Correr el test y verificar que pasa**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/middleware/... -run TestAppBaseURLFromHeader -v"
```

Esperado: PASS, tres tests.

- [ ] **Step 6: Commit**

```bash
git add internal/platform/tenantctx.go internal/api/middleware/middleware.go internal/api/middleware/app_base_url_test.go
git commit -m "feat(middleware): resuelve el base URL del frontend desde X-App-Base-URL"
```

---

### Task 4: `AdminClient.InviteUserByEmail` con metadata

Cambia la firma del método para que acepte los datos que la plantilla del mail necesita.

**Files:**
- Modify: `internal/platform/supabase/admin_client.go:15-22` (interface) y `:39-49` (implementación)
- Modify: `internal/platform/supabase/admin_client_test.go:15-63`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `supabase.InviteParams{Email, RedirectTo, TenantName, InviterName, RoleName string}`
  - `AdminClient.InviteUserByEmail(ctx context.Context, p InviteParams) error` — reemplaza la firma vieja `(ctx, email, redirectTo string)`.

- [ ] **Step 1: Reescribir los tests del invite**

En `internal/platform/supabase/admin_client_test.go`, reemplazar completo `TestAdminClient_InviteUserByEmail_Success` (líneas 15–35) por:

```go
func TestAdminClient_InviteUserByEmail_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth/v1/invite", r.URL.Path)
		assert.Equal(t, "https://app.example.com/s/demo/auth/callback", r.URL.Query().Get("redirect_to"))
		assert.Equal(t, "Bearer test-service-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "user@example.com", body["email"])

		data, ok := body["data"].(map[string]interface{})
		require.True(t, ok, "los datos de la plantilla viajan en data -> user_metadata")
		assert.Equal(t, "MRG SRL", data["tenant_name"])
		assert.Equal(t, "Federico De Giovanni", data["inviter_name"])
		assert.Equal(t, "Operador", data["role_name"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "user-123"})
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.InviteUserByEmail(context.Background(), supabase.InviteParams{
		Email:       "user@example.com",
		RedirectTo:  "https://app.example.com/s/demo/auth/callback",
		TenantName:  "MRG SRL",
		InviterName: "Federico De Giovanni",
		RoleName:    "Operador",
	})
	require.NoError(t, err)
}

func TestAdminClient_InviteUserByEmail_SinMetadataOmiteData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "user@example.com", body["email"])
		assert.NotContains(t, body, "data", "sin datos que mostrar, no se manda data vacio")

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "user-123"})
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.InviteUserByEmail(context.Background(), supabase.InviteParams{
		Email:      "user@example.com",
		RedirectTo: "https://app.example.com/s/demo/auth/callback",
	})
	require.NoError(t, err)
}
```

En los dos tests que quedan (`_4xxNoRetry` líneas 37–49 y `_5xxRetry` líneas 51–63), reemplazar la llamada:

```go
	err := client.InviteUserByEmail(context.Background(), supabase.InviteParams{
		Email:      "user@example.com",
		RedirectTo: "https://app.example.com/callback",
	})
```

- [ ] **Step 2: Correr los tests y verificar que fallan**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/platform/supabase/... -v"
```

Esperado: FAIL de compilación — `supabase.InviteParams` no existe.

- [ ] **Step 3: Cambiar la interface y la implementación**

En `internal/platform/supabase/admin_client.go`, reemplazar la declaración del método en la interface:

```go
// InviteParams lleva todo lo que el mail de invitacion necesita. TenantName,
// InviterName y RoleName son best-effort: aterrizan en el user_metadata del
// invitado y la plantilla cae a una frase generica cuando estan vacios.
type InviteParams struct {
	Email       string
	RedirectTo  string
	TenantName  string
	InviterName string
	RoleName    string
}

// AdminClient interacts with the Supabase Admin REST API.
type AdminClient interface {
	// InviteUserByEmail sends an invitation email via Supabase.
	// p.RedirectTo should be the full frontend callback URL including tenantId.
	InviteUserByEmail(ctx context.Context, p InviteParams) error

	// SendPasswordResetEmail sends a password reset email via Supabase.
	SendPasswordResetEmail(ctx context.Context, userEmail string) error
}
```

y reemplazar el cuerpo del método (líneas 39–49):

```go
func (c *adminClient) InviteUserByEmail(ctx context.Context, p InviteParams) error {
	data := map[string]string{}
	if p.TenantName != "" {
		data["tenant_name"] = p.TenantName
	}
	if p.InviterName != "" {
		data["inviter_name"] = p.InviterName
	}
	if p.RoleName != "" {
		data["role_name"] = p.RoleName
	}

	payload := map[string]any{"email": p.Email}
	if len(data) > 0 {
		payload["data"] = data
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal invite request: %w", err)
	}

	// GoTrue expone el invite en /auth/v1/invite (no bajo /admin) y toma
	// redirect_to como query param; el campo data va a user_metadata y de ahi
	// lo lee la plantilla con {{ .Data.tenant_name }}.
	path := "/auth/v1/invite?redirect_to=" + url.QueryEscape(p.RedirectTo)
	return c.doWithRetry(ctx, http.MethodPost, path, body)
}
```

- [ ] **Step 4: Correr los tests y verificar que pasan**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/platform/supabase/... -v"
```

Esperado: PASS los cinco tests del paquete. `go build ./...` va a seguir fallando en `invitation_usecase.go` — eso se arregla en la Task 7 y es esperado acá.

- [ ] **Step 5: Commit**

```bash
git add internal/platform/supabase/admin_client.go internal/platform/supabase/admin_client_test.go
git commit -m "feat(supabase): InviteUserByEmail acepta metadata para la plantilla del mail"
```

---

### Task 5: Arreglar el envío del reset de contraseña

**Files:**
- Modify: `internal/platform/supabase/admin_client.go:20-21` (interface) y `:51-61` (implementación)
- Modify: `internal/platform/supabase/admin_client_test.go:65-83`

**Interfaces:**
- Consumes: nada.
- Produces: `AdminClient.SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error` — reemplaza la firma vieja de dos parámetros.

- [ ] **Step 1: Verificar el bug contra el proyecto real**

Antes de cambiar nada, confirmar empíricamente que `generate_link` no envía mail. Con la service role key de `.env`:

```bash
curl -sS -X POST "https://cdjehkbidqqsldaajbui.supabase.co/auth/v1/admin/generate_link" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_ROLE_KEY" \
  -H "apikey: $SUPABASE_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"type":"recovery","email":"federicoadegiovanni+recovery-test@gmail.com"}' | head -c 400
```

Esperado si el diagnóstico es correcto: un 200 con un `action_link` en el body, y **ningún mail** en la casilla. Anotar el resultado. Si contra todo pronóstico llega el mail, detener esta tarea y reportarlo — el resto del plan no depende de este cambio.

- [ ] **Step 2: Escribir el test que falla**

En `internal/platform/supabase/admin_client_test.go`, reemplazar completo `TestAdminClient_SendPasswordResetEmail_Success` (líneas 65–83) por:

```go
func TestAdminClient_SendPasswordResetEmail_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /auth/v1/recover es el endpoint que efectivamente envia. El anterior,
		// /auth/v1/admin/generate_link, solo devuelve el link en la respuesta.
		assert.Equal(t, "/auth/v1/recover", r.URL.Path)
		assert.Equal(t, "https://app.example.com/s/demo/auth/callback", r.URL.Query().Get("redirect_to"))
		assert.Equal(t, "Bearer test-service-key", r.Header.Get("Authorization"))

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "user@example.com", body["email"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	client := supabase.NewAdminClient(srv.URL, "test-service-key")
	err := client.SendPasswordResetEmail(context.Background(), "user@example.com", "https://app.example.com/s/demo/auth/callback")
	require.NoError(t, err)
}
```

- [ ] **Step 3: Correr el test y verificar que falla**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/platform/supabase/... -run SendPasswordReset -v"
```

Esperado: FAIL de compilación — la firma actual recibe dos argumentos, no tres.

- [ ] **Step 4: Cambiar la implementación**

En `internal/platform/supabase/admin_client.go`, en la interface:

```go
	// SendPasswordResetEmail sends a password reset email via Supabase.
	// redirectTo should be the full frontend callback URL including tenantId;
	// GoTrue le agrega type=recovery y la pagina de callback redirige de ahi a
	// la pantalla de cambio de contraseña.
	SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error
```

y reemplazar el cuerpo (líneas 51–61):

```go
func (c *adminClient) SendPasswordResetEmail(ctx context.Context, userEmail, redirectTo string) error {
	body, err := json.Marshal(map[string]string{"email": userEmail})
	if err != nil {
		return fmt.Errorf("marshal recovery request: %w", err)
	}

	// /auth/v1/recover es el endpoint que envia el mail. /auth/v1/admin/generate_link
	// solo acuña el link y lo devuelve en la respuesta — que es lo que hacia esta
	// funcion antes, por lo que el reset de contraseña no llegaba a destino.
	path := "/auth/v1/recover?redirect_to=" + url.QueryEscape(redirectTo)
	return c.doWithRetry(ctx, http.MethodPost, path, body)
}
```

- [ ] **Step 5: Correr los tests y verificar que pasan**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/platform/supabase/... -v"
```

Esperado: PASS. `go build ./...` sigue roto en los usecases — se arregla en la Task 7.

- [ ] **Step 6: Commit**

```bash
git add internal/platform/supabase/admin_client.go internal/platform/supabase/admin_client_test.go
git commit -m "fix(supabase): usa /auth/v1/recover para que el reset de contraseña se envie"
```

---

### Task 6: Resolución best-effort de nombres para el mail

Unidad chica y aislada: convierte `tenant_id` y `role_id` en nombres legibles, y nunca falla.

**Files:**
- Create: `internal/api/usecases/invite_metadata.go`
- Test: `internal/api/usecases/invite_metadata_test.go`

**Interfaces:**
- Consumes: `domain.Tenant.Name`, `domain.Role.Name`.
- Produces:
  - `usecases.TenantNameLookup` — interface con `FindByID(ctx, uuid.UUID) (*domain.Tenant, error)`, satisfecha por `tenants.TenantRepository`
  - `usecases.RoleNameLookup` — interface con `GetByIDForTenant(ctx, string, uuid.UUID) (*domain.Role, error)`, satisfecha por `roles.Repository`
  - `usecases.InviteDisplayNames{TenantName, RoleName string}`
  - `resolveInviteDisplayNames(ctx, TenantNameLookup, RoleNameLookup, tenantID, roleID string) InviteDisplayNames` (no exportada)

- [ ] **Step 1: Escribir el test que falla**

Crear `internal/api/usecases/invite_metadata_test.go`:

```go
package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

const testTenantID = "11b36b85-033d-4bb3-9e31-4c92161887c0"

type fakeTenantLookup struct {
	tenant *domain.Tenant
	err    error
}

func (f fakeTenantLookup) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.tenant, f.err
}

type fakeRoleLookup struct {
	role *domain.Role
	err  error
}

func (f fakeRoleLookup) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID) (*domain.Role, error) {
	return f.role, f.err
}

func TestResolveInviteDisplayNames_ResuelveAmbosNombres(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator",
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Equal(t, "Operador", names.RoleName)
}

func TestResolveInviteDisplayNames_FallaDeTenantNoPierdeElRol(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{err: errors.New("db caida")},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		testTenantID, "operator",
	)
	assert.Empty(t, names.TenantName)
	assert.Equal(t, "Operador", names.RoleName, "una falla de tenant no puede arrastrar al rol")
}

func TestResolveInviteDisplayNames_FallaDeRolNoPierdeElTenant(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{err: errors.New("db caida")},
		testTenantID, "operator",
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Empty(t, names.RoleName)
}

func TestResolveInviteDisplayNames_TenantIDInvalidoDevuelveVacio(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{role: &domain.Role{Name: "Operador"}},
		"no-es-un-uuid", "operator",
	)
	assert.Empty(t, names.TenantName)
	assert.Empty(t, names.RoleName)
}

func TestResolveInviteDisplayNames_RoleIDVacioNoConsultaElRepo(t *testing.T) {
	names := resolveInviteDisplayNames(
		context.Background(),
		fakeTenantLookup{tenant: &domain.Tenant{Name: "MRG SRL"}},
		fakeRoleLookup{err: errors.New("no deberia llamarse")},
		testTenantID, "",
	)
	assert.Equal(t, "MRG SRL", names.TenantName)
	assert.Empty(t, names.RoleName)
}
```

- [ ] **Step 2: Correr el test y verificar que falla**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/usecases/... -run TestResolveInviteDisplayNames -v"
```

Esperado: FAIL de compilación — `resolveInviteDisplayNames` no existe.

- [ ] **Step 3: Escribir la implementación**

Crear `internal/api/usecases/invite_metadata.go`:

```go
package usecases

import (
	"context"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"go.uber.org/zap"
)

// TenantNameLookup es la porcion del repositorio de tenants que el mail de
// invitacion necesita. Se declara acá para poder testear la resolucion de
// nombres sin levantar el repositorio completo.
type TenantNameLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error)
}

// RoleNameLookup es la porcion del repositorio de roles que el mail necesita.
type RoleNameLookup interface {
	GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID) (*domain.Role, error)
}

// InviteDisplayNames son los valores legibles que se muestran en el mail.
// Todos los campos son best-effort: si la consulta falla el campo queda vacio
// y la plantilla cae a su copy generica. Resolver un nombre decorativo nunca
// puede bloquear una invitacion.
type InviteDisplayNames struct {
	TenantName string
	RoleName   string
}

func resolveInviteDisplayNames(
	ctx context.Context,
	tenants TenantNameLookup,
	roles RoleNameLookup,
	tenantID string,
	roleID string,
) InviteDisplayNames {
	var out InviteDisplayNames

	tenantUUID, err := uuid.Parse(tenantID)
	if err != nil {
		Log.Warn("metadata de invitacion: tenant id no parseable",
			zap.String("tenant_id", tenantID), zap.Error(err))
		return out
	}

	if tenants != nil {
		t, err := tenants.FindByID(ctx, tenantUUID)
		switch {
		case err != nil:
			Log.Warn("metadata de invitacion: fallo la consulta de tenant",
				zap.String("tenant_id", tenantID), zap.Error(err))
		case t != nil:
			out.TenantName = t.Name
		}
	}

	if roles != nil && roleID != "" {
		r, err := roles.GetByIDForTenant(ctx, roleID, tenantUUID)
		switch {
		case err != nil:
			Log.Warn("metadata de invitacion: fallo la consulta de rol",
				zap.String("role_id", roleID), zap.Error(err))
		case r != nil:
			out.RoleName = r.Name
		}
	}

	return out
}
```

- [ ] **Step 4: Correr el test y verificar que pasa**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/usecases/... -run TestResolveInviteDisplayNames -v"
```

Esperado: PASS, cinco tests.

- [ ] **Step 5: Commit**

```bash
git add internal/api/usecases/invite_metadata.go internal/api/usecases/invite_metadata_test.go
git commit -m "feat(usecases): resolucion best-effort de nombres de tenant y rol para el mail"
```

---

### Task 7: Cablear usecases, handler y rutas

La tarea que vuelve a poner el build en verde. Todo lo anterior deja el repo sin compilar a propósito.

**Files:**
- Modify: `internal/api/usecases/invitation_usecase.go:24-52` (struct y constructor), `:92-103` (envío en Create), `:124-127` (envío en Resend)
- Modify: `internal/api/usecases/password_usecase.go` (struct, constructor, y el envío en `ForcePasswordChange:62`)
- Modify: `internal/api/handler/invitations/create_invitation/create_invitation.go:36-38` (log del 500)
- Modify: `internal/routes/url_mappings.go:91-92` (constructores), `:109-118` (cadena de middleware)

**Interfaces:**
- Consumes: `apporigin.Parse` (T1), `config.SupabaseConfig.AppAllowedOrigins` (T2), `middleware.AppBaseURLFromHeader` y `platform.AppBaseURL` (T3), `supabase.InviteParams` (T4), la firma nueva de `SendPasswordResetEmail` (T5), `resolveInviteDisplayNames` (T6).
- Produces:
  - `usecases.NewInvitationUsecase(invRepo, userRepo, userRoleRepo, tenantRepo TenantNameLookup, roleRepo RoleNameLookup, supabaseClient, redisClient, appBaseURL string, rateLimitHour int)`
  - `usecases.NewPasswordUsecase(userRepo, supabaseClient, appBaseURL string, logger)`

- [ ] **Step 1: Agregar el helper de base URL en los usecases**

Al final de `internal/api/usecases/invite_metadata.go`, agregar:

```go
// callbackURL arma la URL de callback del frontend para este tenant, usando el
// origin que el BFF reporto para este request. Cuando no hay ninguno en el
// contexto —requests que no pasan por el middleware, o un origin rechazado—
// cae al default configurado. Invite y recovery comparten esta misma URL: la
// pagina de callback discrimina por el query param `type` que agrega GoTrue.
func callbackURL(ctx context.Context, fallbackBase, tenantID string) string {
	base := platform.AppBaseURL(ctx)
	if base == "" {
		base = fallbackBase
	}
	return fmt.Sprintf("%s/s/%s/auth/callback", base, tenantID)
}
```

y sumar `"fmt"` y `"github.com/tu-org/embolsadora-api/internal/platform"` a los imports del archivo.

- [ ] **Step 2: Actualizar `InvitationUsecase`**

En `internal/api/usecases/invitation_usecase.go`, agregar los dos campos al struct y al constructor:

```go
type InvitationUsecase struct {
	invRepo        invitations.InvitationRepository
	userRepo       users.UserRepository
	userRoleRepo   userRoles.UserRoleRepository
	tenantRepo     TenantNameLookup
	roleRepo       RoleNameLookup
	supabaseClient supabase.AdminClient
	redis          *redis.Client
	appBaseURL     string
	rateLimitHour  int
}

func NewInvitationUsecase(
	invRepo invitations.InvitationRepository,
	userRepo users.UserRepository,
	userRoleRepo userRoles.UserRoleRepository,
	tenantRepo TenantNameLookup,
	roleRepo RoleNameLookup,
	supabaseClient supabase.AdminClient,
	redisClient *redis.Client,
	appBaseURL string,
	rateLimitHour int,
) *InvitationUsecase {
	return &InvitationUsecase{
		invRepo:        invRepo,
		userRepo:       userRepo,
		userRoleRepo:   userRoleRepo,
		tenantRepo:     tenantRepo,
		roleRepo:       roleRepo,
		supabaseClient: supabaseClient,
		redis:          redisClient,
		appBaseURL:     appBaseURL,
		rateLimitHour:  rateLimitHour,
	}
}
```

Reemplazar el bloque de envío en `CreateInvitation` (las líneas que hoy arman `redirectTo` y llaman a `InviteUserByEmail`) por:

```go
	// Send invite email via Supabase Admin API
	names := resolveInviteDisplayNames(ctx, uc.tenantRepo, uc.roleRepo, tenantID, roleID)
	inviteErr := uc.supabaseClient.InviteUserByEmail(ctx, supabase.InviteParams{
		Email:       email,
		RedirectTo:  callbackURL(ctx, uc.appBaseURL, tenantID),
		TenantName:  names.TenantName,
		InviterName: callerUser.Name,
		RoleName:    names.RoleName,
	})
	if inviteErr != nil {
		// Rollback: mark invitation as revoked since Supabase failed
		if rbErr := uc.invRepo.UpdateStatus(ctx, created.ID, domain.InvitationStatusRevoked); rbErr != nil {
			Log.Error("failed to rollback invitation after supabase error",
				zap.String("invitation_id", created.ID),
				zap.Error(rbErr),
			)
		}
		return nil, fmt.Errorf("supabase invite failed: %w", inviteErr)
	}
```

Reemplazar el bloque equivalente en `ResendInvitation` por:

```go
	names := resolveInviteDisplayNames(ctx, uc.tenantRepo, uc.roleRepo, tenantID, inv.RoleID)

	// InviterName sale de quien esta reenviando, no de quien invito originalmente:
	// el nombre del invitador original exigiria una consulta mas por un dato
	// decorativo, y quien reenvia es igual de valido como referencia para el invitado.
	var inviterName string
	if u, ok := platform.DomainUser(ctx).(*domain.User); ok && u != nil {
		inviterName = u.Name
	}

	if err := uc.supabaseClient.InviteUserByEmail(ctx, supabase.InviteParams{
		Email:       inv.Email,
		RedirectTo:  callbackURL(ctx, uc.appBaseURL, tenantID),
		TenantName:  names.TenantName,
		InviterName: inviterName,
		RoleName:    names.RoleName,
	}); err != nil {
		return err
	}
```

- [ ] **Step 3: Actualizar `PasswordUsecase`**

En `internal/api/usecases/password_usecase.go`, agregar el campo `appBaseURL string` al struct y como parámetro del constructor `NewPasswordUsecase` (entre `supabaseClient` y `logger`), asignándolo en el literal de retorno. Después reemplazar la llamada de envío:

```go
	// Send reset email via Supabase
	if err := uc.supabaseClient.SendPasswordResetEmail(ctx, target.Email, callbackURL(ctx, uc.appBaseURL, tenantID)); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
```

- [ ] **Step 4: Loguear el error real en el 500 del handler**

En `internal/api/handler/invitations/create_invitation/create_invitation.go`, agregar el import `"github.com/tu-org/embolsadora-api/internal/api/usecases"` (ya está) y `"go.uber.org/zap"`, y reemplazar la rama `default` del switch de errores:

```go
		default:
			// Sin esto, una falla de SMTP o de allow-list devuelve un 500 mudo:
			// era el punto ciego que hacia invisible el bug de la URL.
			usecases.Log.Error("create invitation failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
```

- [ ] **Step 5: Cablear en `url_mappings.go`**

Agregar un solo import (los repos de tenants y roles ya están importados, como `tenantsRepository` y `rolesRepo`):

```go
	"github.com/tu-org/embolsadora-api/internal/platform/apporigin"
```

`tenantRepo` ya existe en la línea 75 y sirve tal cual. `rRepo` (roles) también existe, pero recién en la línea 179 — **después** del bloque de use cases. Mover esa línea al bloque de repositorios, junto a las otras, dejando la de roles justo debajo de `invRepo`:

```go
	invRepo := invitationsRepo.NewInvitationRepository(db)
	rRepo := rolesRepo.NewPostgresRepository(db)
```

y borrar la declaración original de `rRepo` en la línea ~179, dejando ahí solo `rService := rolesApp.NewService(rRepo, logger)`.

Cambiar las dos construcciones de usecase:

```go
	invUC := usecases.NewInvitationUsecase(invRepo, userRepo, userRoleRepo, tenantRepo, rRepo, supabaseClient, redisClient, cfg.Supabase.AppBaseURL, cfg.Supabase.InviteRateLimitHour)
	passwordUC := usecases.NewPasswordUsecase(userRepo, supabaseClient, cfg.Supabase.AppBaseURL, logger)
```

> `rRepo` es `*rolesRepo.PostgresRepository` (puntero), y `tenantRepo` es la interface `tenants.TenantRepository`. Ambos satisfacen `RoleNameLookup` y `TenantNameLookup` respectivamente porque tienen los métodos con la firma exacta — verificado contra `internal/repo/pg/roles/repository.go:33` y `internal/repo/pg/tenants/repository.go:27`.

Y agregar el middleware a la cadena del grupo `/api/v1`, después de `PasswordChangeGuard()`:

```go
		apimw.PasswordChangeGuard(),
		apimw.AppBaseURLFromHeader(apporigin.Parse(cfg.Supabase.AppAllowedOrigins), cfg.Supabase.AppBaseURL),
```

- [ ] **Step 6: Compilar y correr toda la suite**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./... && go vet ./... && go test ./internal/..."
```

Esperado: build sin errores y todos los tests en PASS. Los tests de `repo/pg` que necesitan `DATABASE_URL` se saltean o fallan por conexión — es el comportamiento preexistente, no una regresión de esta tarea.

- [ ] **Step 7: Commit**

```bash
git add internal/api/usecases/ internal/api/handler/invitations/create_invitation/create_invitation.go internal/routes/url_mappings.go
git commit -m "feat(invitations): usa el origin del request para el redirect_to y enriquece el mail"
```

---

### Task 8: Frontend — mandar el origin real

Repo `~/Develop/UTN/embolsadora-frontend`. No hay runner de tests: la verificación es type-check, lint y build.

**Files:**
- Create: `src/lib/app-base-url.ts`
- Modify: `src/lib/backend-fetch.ts:8-10` (interface) y `:59,71-75` (destructuring y headers)
- Modify: `src/app/api/invitations/route.ts` (función `POST`)
- Modify: `src/app/api/invitations/[id]/resend/route.ts`
- Modify: `src/app/api/users/[id]/force-password-change/route.ts`

**Interfaces:**
- Consumes: nada del backend Go — el header es aditivo, y un backend que todavía no lo lee lo ignora.
- Produces: `resolveAppBaseUrl(request: NextRequest): string | undefined`, y el campo `appBaseUrl?: string` en `BackendFetchOptions`.

- [ ] **Step 1: Crear el resolver de origin**

Crear `src/lib/app-base-url.ts`:

```ts
// src/lib/app-base-url.ts
// Resuelve el origin que el navegador realmente uso para este request.
// El backend Go corre una sola instancia en Cloud Run que atiende a localhost,
// a produccion y a los previews, asi que no puede deducir por si solo a donde
// deben apuntar los links que manda por mail: se lo tenemos que decir.

import type { NextRequest } from 'next/server';

export function resolveAppBaseUrl(request: NextRequest): string | undefined {
  // El fetch del navegador a una ruta del BFF manda Origin en los metodos que
  // no son GET, que son justo los tres que disparan mails.
  const origin = request.headers.get('origin');
  if (origin) return origin;

  const host = request.headers.get('host');
  if (!host) return undefined;

  // En Vercel el host es el dominio real y x-forwarded-proto viene seteado.
  // En dev local no hay proxy, de ahi el default a http para localhost.
  const proto = request.headers.get('x-forwarded-proto') ?? (host.startsWith('localhost') ? 'http' : 'https');
  return `${proto}://${host}`;
}
```

- [ ] **Step 2: Aceptar el campo en `backendFetch`**

En `src/lib/backend-fetch.ts`, extender la interface:

```ts
interface BackendFetchOptions extends RequestInit {
  tenantId?: string;
  /** Origin del frontend, reenviado al backend para armar links de mail. */
  appBaseUrl?: string;
}
```

cambiar el destructuring:

```ts
  const { tenantId, appBaseUrl, ...fetchOptions } = options;
```

y agregar el header junto a `X-Tenant-ID`:

```ts
  const headers: Record<string, string> = {
    ...((fetchOptions.headers as Record<string, string> | undefined) ?? {}),
    ...authHeaders,
    ...(resolvedTenantId ? { 'X-Tenant-ID': resolvedTenantId } : {}),
    ...(appBaseUrl ? { 'X-App-Base-URL': appBaseUrl } : {}),
  };
```

- [ ] **Step 3: Pasarlo desde las tres rutas que disparan mails**

En `src/app/api/invitations/route.ts`, agregar el import `import { resolveAppBaseUrl } from '@/lib/app-base-url';` y en la función `POST`:

```ts
  const result = await backendFetch('/api/v1/invitations', {
    method: 'POST',
    body: JSON.stringify(invitation),
    tenantId,
    appBaseUrl: resolveAppBaseUrl(request),
  });
```

En `src/app/api/invitations/[id]/resend/route.ts`, mismo import y:

```ts
  const result = await backendFetch(`/api/v1/invitations/${id}/resend`, {
    method: 'POST',
    tenantId,
    appBaseUrl: resolveAppBaseUrl(request),
  });
```

En `src/app/api/users/[id]/force-password-change/route.ts`, mismo import y:

```ts
  const result = await backendFetch(`/api/v1/users/${id}/force-password-change`, {
    method: 'POST',
    tenantId,
    appBaseUrl: resolveAppBaseUrl(request),
  });
```

- [ ] **Step 4: Verificar**

```bash
pnpm tsc --noEmit && pnpm lint && pnpm build
```

Esperado: los tres en verde. El build puede emitir warnings de ESLint; el tope es 100 y la línea base son ~89, así que confirmar que el número no subió.

- [ ] **Step 5: Commit**

```bash
git add src/lib/app-base-url.ts src/lib/backend-fetch.ts src/app/api/invitations/route.ts "src/app/api/invitations/[id]/resend/route.ts" "src/app/api/users/[id]/force-password-change/route.ts"
git commit -m "feat(bff): reenvia el origin del navegador al backend para los mails de auth"
```

---

### Task 9: Las cuatro plantillas HTML y su renderer

Vuelta al repo backend. El renderer no es un extra: es lo único que hace visible la rama `{{ else }}` antes de que un usuario real reciba un mail roto.

**Files:**
- Create: `emails/invite.html`, `emails/recovery.html`, `emails/confirmation.html`, `emails/magic-link.html`
- Create: `cmd/renderemails/main.go`
- Modify: `.gitignore` (agregar `tmp/emails/`)

**Interfaces:**
- Consumes: las variables que expone GoTrue — `{{ .ConfirmationURL }}`, `{{ .Email }}`, `{{ .Data }}` (el `user_metadata` que puebla la Task 4).
- Produces: los cuatro archivos HTML que la Task 10 publica.

- [ ] **Step 1: Escribir `emails/invite.html`**

```html
<!doctype html>
<html lang="es">
  <body style="margin:0;padding:0;background-color:#f4f4f5;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
      <tr>
        <td align="center" style="padding:32px 12px;">
          <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%;background-color:#ffffff;border-radius:8px;border:1px solid #e4e4e7;">
            <tr>
              <td style="padding:32px 32px 0 32px;font-family:Arial,Helvetica,sans-serif;font-size:16px;font-weight:bold;color:#111827;">
                Embolsadora
              </td>
            </tr>
            <tr>
              <td style="padding:24px 32px 0 32px;font-family:Arial,Helvetica,sans-serif;font-size:22px;font-weight:bold;color:#111827;line-height:1.3;">
                {{ if .Data.tenant_name }}Te invitaron a {{ .Data.tenant_name }}{{ else }}Te invitaron a Embolsadora{{ end }}
              </td>
            </tr>
            <tr>
              <td style="padding:12px 32px 0 32px;font-family:Arial,Helvetica,sans-serif;font-size:15px;color:#3f3f46;line-height:1.6;">
                {{ if .Data.inviter_name }}{{ .Data.inviter_name }} te invitó{{ else }}Te invitaron{{ end }}{{ if .Data.tenant_name }} a sumarte a {{ .Data.tenant_name }} en Embolsadora{{ else }} a sumarte a Embolsadora{{ end }}{{ if .Data.role_name }} con el rol de {{ .Data.role_name }}{{ end }}.
              </td>
            </tr>
            <tr>
              <td style="padding:12px 32px 0 32px;font-family:Arial,Helvetica,sans-serif;font-size:13px;color:#71717a;line-height:1.6;">
                El link vence en 24 horas.
              </td>
            </tr>
            <tr>
              <td style="padding:24px 32px 0 32px;">
                <table role="presentation" cellpadding="0" cellspacing="0">
                  <tr>
                    <td style="background-color:#111827;border-radius:6px;">
                      <a href="{{ .ConfirmationURL }}" style="display:inline-block;padding:13px 26px;font-family:Arial,Helvetica,sans-serif;font-size:15px;font-weight:bold;color:#ffffff;text-decoration:none;">Aceptar invitación</a>
                    </td>
                  </tr>
                </table>
              </td>
            </tr>
            <tr>
              <td style="padding:24px 32px 0 32px;font-family:Arial,Helvetica,sans-serif;font-size:12px;color:#71717a;line-height:1.6;word-break:break-all;">
                Si el botón no funciona, copiá y pegá este link en tu navegador:<br />
                <a href="{{ .ConfirmationURL }}" style="color:#71717a;">{{ .ConfirmationURL }}</a>
              </td>
            </tr>
            <tr>
              <td style="padding:24px 32px 32px 32px;">
                <div style="border-top:1px solid #e4e4e7;padding-top:16px;font-family:Arial,Helvetica,sans-serif;font-size:12px;color:#a1a1aa;line-height:1.6;">
                  Si no esperabas esta invitación, ignorá este mail. Nadie va a acceder a tu cuenta sin que aceptes.
                </div>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>
```

- [ ] **Step 2: Escribir las otras tres**

Las tres son el mismo esqueleto sin datos de tenant. Copiar `invite.html` y cambiar únicamente el título, el párrafo, el texto del botón y el cierre:

`emails/recovery.html`:
- Título: `Restablecé tu contraseña`
- Párrafo: `Pediste restablecer la contraseña de tu cuenta de Embolsadora. Hacé clic en el botón para elegir una nueva.`
- Línea de vencimiento: `El link vence en 24 horas.`
- Botón: `Cambiar mi contraseña`
- Cierre: `Si no pediste este cambio, ignorá este mail. Tu contraseña actual sigue funcionando.`

`emails/confirmation.html`:
- Título: `Confirmá tu cuenta`
- Párrafo: `Para terminar de crear tu cuenta de Embolsadora, confirmá tu dirección de mail.`
- Línea de vencimiento: `El link vence en 24 horas.`
- Botón: `Confirmar mi cuenta`
- Cierre: `Si no creaste esta cuenta, ignorá este mail.`

`emails/magic-link.html`:
- Título: `Tu link de acceso`
- Párrafo: `Usá este link para entrar a Embolsadora sin contraseña.`
- Línea de vencimiento: `El link vence en 24 horas.`
- Botón: `Entrar a Embolsadora`
- Cierre: `Si no pediste este link, ignorá este mail.`

Ninguna de las tres usa `{{ .Data.* }}`: no tienen contexto de tenant.

- [ ] **Step 3: Escribir el renderer**

Crear `cmd/renderemails/main.go`:

```go
// Command renderemails renderiza las plantillas de mail localmente, con datos
// completos y con datos vacios, para poder revisar las dos ramas de cada
// {{ if }} antes de que un usuario reciba un mail roto. GoTrue las renderiza
// con html/template de Go, asi que esto reproduce la salida real.
package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

type templateData struct {
	ConfirmationURL string
	Email           string
	SiteURL         string
	Token           string
	Data            map[string]string
}

func main() {
	const outDir = "tmp/emails"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	const confirmURL = "https://cdjehkbidqqsldaajbui.supabase.co/auth/v1/verify?token=abc123&type=invite&redirect_to=https://embolsadora.site/s/11b36b85-033d-4bb3-9e31-4c92161887c0/auth/callback"

	cases := map[string]templateData{
		"completo": {
			ConfirmationURL: confirmURL,
			Email:           "usuario@ejemplo.com",
			SiteURL:         "https://embolsadora.site",
			Token:           "123456",
			Data: map[string]string{
				"tenant_name":  "MRG SRL",
				"inviter_name": "Federico De Giovanni",
				"role_name":    "Operador",
			},
		},
		"vacio": {
			ConfirmationURL: confirmURL,
			Email:           "usuario@ejemplo.com",
			SiteURL:         "https://embolsadora.site",
			Token:           "123456",
			Data:            map[string]string{},
		},
	}

	files, err := filepath.Glob("emails/*.html")
	if err != nil {
		panic(err)
	}
	if len(files) == 0 {
		panic("no se encontraron plantillas en emails/")
	}

	for _, f := range files {
		name := filepath.Base(f)
		tpl, err := template.ParseFiles(f)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", name, err))
		}
		for caseName, data := range cases {
			out := filepath.Join(outDir, caseName+"-"+name)
			fh, err := os.Create(out)
			if err != nil {
				panic(err)
			}
			if err := tpl.Execute(fh, data); err != nil {
				fh.Close()
				panic(fmt.Sprintf("%s (caso %s): %v", name, caseName, err))
			}
			fh.Close()
			fmt.Println("escrito", out)
		}
	}
}
```

- [ ] **Step 4: Renderizar y revisar las ocho salidas**

```bash
echo "tmp/emails/" >> .gitignore
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go run ./cmd/renderemails"
open tmp/emails/completo-invite.html tmp/emails/vacio-invite.html
```

Esperado: ocho archivos escritos sin panic. Abrir los dos de invite y confirmar que la versión "completo" dice *"Federico De Giovanni te invitó a sumarte a MRG SRL en Embolsadora con el rol de Operador."* y la "vacio" dice *"Te invitaron a sumarte a Embolsadora."* — sin comas huérfanas ni dobles espacios.

> **Corrección aplicada durante la ejecución.** Esta sección decía originalmente que si `go run` explotaba con `nil pointer evaluating interface {}.tenant_name`, había que cambiar las plantillas a `{{ if index .Data "tenant_name" }}`. **Eso es falso** y se verificó con un repro aislado en Go: `index` sobre un `interface{}` nulo tira `error calling index: index of untyped nil`. El remedio prescrito no arreglaba nada.
>
> Lo que sí funciona —y quedó implementado— es un guard externo sobre `.Data` antes de cualquier acceso:
>
> ```
> {{ if .Data }}…{{ .Data.tenant_name }}…{{ else }}…copy genérica…{{ end }}
> ```
>
> Un `interface{}` nulo evalúa como falso en ese `if` sin desreferenciar nada. Y el renderer necesita `Data interface{}` (no `map[string]string`) más un caso `nil`: con un map, los casos nulo, vacío y poblado se comportan igual y el panic **nunca** se reproduce, así que el harness daba una falsa sensación de seguridad. El panic era real: `invite.html` explotaba con `.Data` nulo, que es exactamente lo que GoTrue manda para usuarios sin `user_metadata` — la población que esta feature existe para atender.

- [ ] **Step 5: Commit**

```bash
git add emails/ cmd/renderemails/ .gitignore
git commit -m "feat(emails): cuatro plantillas propias en español con renderer local"
```

---

### Task 10: Script de publicación a Supabase

**Files:**
- Create: `scripts/publish-email-templates.sh`
- Modify: `emails/README.md` (crear)

**Interfaces:**
- Consumes: los cuatro HTML de la Task 9.
- Produces: nada en código. Es la herramienta que sincroniza repo → Supabase.

- [ ] **Step 1: Escribir el script**

Crear `scripts/publish-email-templates.sh`:

```bash
#!/usr/bin/env bash
# Publica emails/*.html en Supabase Auth.
#
# El dashboard NO es la fuente de verdad: cualquier cosa editada ahi la pisa la
# proxima corrida de este script. Editar siempre los archivos del repo.
#
# Requiere:
#   SUPABASE_ACCESS_TOKEN  personal access token (Account -> Access Tokens)
#   SUPABASE_PROJECT_REF   ref del proyecto, p.ej. cdjehkbidqqsldaajbui
#   jq
set -euo pipefail

: "${SUPABASE_ACCESS_TOKEN:?falta SUPABASE_ACCESS_TOKEN}"
: "${SUPABASE_PROJECT_REF:?falta SUPABASE_PROJECT_REF}"

cd "$(dirname "$0")/.."

payload=$(jq -n \
  --rawfile invite       emails/invite.html \
  --rawfile recovery     emails/recovery.html \
  --rawfile confirmation emails/confirmation.html \
  --rawfile magiclink    emails/magic-link.html \
  '{
     mailer_templates_invite_content:       $invite,
     mailer_templates_recovery_content:     $recovery,
     mailer_templates_confirmation_content: $confirmation,
     mailer_templates_magic_link_content:   $magiclink,
     mailer_subjects_invite:       "Te invitaron a Embolsadora",
     mailer_subjects_recovery:     "Restablecé tu contraseña de Embolsadora",
     mailer_subjects_confirmation: "Confirmá tu cuenta de Embolsadora",
     mailer_subjects_magic_link:   "Tu link de acceso a Embolsadora"
   }')

curl -sS -X PATCH \
  "https://api.supabase.com/v1/projects/${SUPABASE_PROJECT_REF}/config/auth" \
  -H "Authorization: Bearer ${SUPABASE_ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$payload" \
  | jq '{invite_subject: .mailer_subjects_invite, invite_bytes: (.mailer_templates_invite_content | length)}'

echo "plantillas publicadas en el proyecto ${SUPABASE_PROJECT_REF}"
```

> **Corrección aplicada durante la ejecución.** El `curl -sS` de arriba **no detecta fallas HTTP**: ante un 401 por token vencido imprime el JSON de error y el script igual dice "plantillas publicadas", con exit 0. Un script de publicación que reporta éxito cuando falló es peor que no tenerlo, porque se corre una sola vez y se le cree. La versión implementada captura el status code explícitamente y sale distinto de cero ante cualquier respuesta que no sea 2xx, imprimiendo status y body. Ver `scripts/publish-email-templates.sh` para la forma final.

Hacerlo ejecutable: `chmod +x scripts/publish-email-templates.sh`

- [ ] **Step 2: Documentar el flujo**

Crear `emails/README.md`:

```markdown
# Plantillas de mail de Supabase Auth

Estos cuatro archivos son la **fuente de verdad** de los mails que manda
Supabase Auth. Lo que esté cargado en el dashboard es una copia derivada.

## Editar

1. Modificar el `.html` acá.
2. Previsualizar las dos ramas de cada condicional:
   `go run ./cmd/renderemails` y abrir `tmp/emails/`.
3. Publicar: `SUPABASE_ACCESS_TOKEN=… SUPABASE_PROJECT_REF=cdjehkbidqqsldaajbui ./scripts/publish-email-templates.sh`

## Variables disponibles

- `{{ .ConfirmationURL }}` — el link de acción, ya con el `redirect_to` que armó el backend.
- `{{ .Email }}` — mail del destinatario.
- `{{ .Data.tenant_name }}`, `{{ .Data.inviter_name }}`, `{{ .Data.role_name }}` — solo en `invite.html`.
  Los puebla el backend en `internal/platform/supabase/admin_client.go`. Siempre
  usarlas dentro de un `{{ if }}`: un usuario invitado antes de este cambio no las tiene.

**No usar `{{ .SiteURL }}`.** Es un valor global del proyecto y era la causa de
que los mails mostraran `http://localhost:3000` desde producción.
```

- [ ] **Step 3: Publicar y verificar**

```bash
export SUPABASE_PROJECT_REF=cdjehkbidqqsldaajbui
export SUPABASE_ACCESS_TOKEN=<personal access token>
./scripts/publish-email-templates.sh
```

Esperado: un JSON con `invite_subject` y un `invite_bytes` de varios miles. Confirmar en el dashboard (Authentication → Emails) que el HTML de Invite user es el del repo.

- [ ] **Step 4: Commit**

```bash
git add scripts/publish-email-templates.sh emails/README.md
git commit -m "feat(emails): script de publicacion a Supabase y documentacion del flujo"
```

---

### Task 11: Configuración de entorno y verificación end-to-end

Sin esta tarea nada de lo anterior es visible para un usuario. No hay código: son pasos de configuración y la prueba real.

**Files:** ninguno. Cambios en Resend, DonWeb, Supabase y Cloud Run.

**Interfaces:**
- Consumes: todo lo anterior.
- Produces: el sistema funcionando.

- [ ] **Step 1: Alta del dominio en Resend**

Crear cuenta en resend.com, agregar el dominio `embolsadora.site`, y copiar los registros DNS que muestra (típicamente un TXT de SPF, un TXT/CNAME de DKIM, y opcionalmente uno de DMARC).

- [ ] **Step 2: Cargar los registros en DonWeb**

En el panel de DNS de DonWeb para `embolsadora.site`, cargar los registros del paso anterior. Agregar además el DMARC en modo observación:

```
_dmarc.embolsadora.site   TXT   "v=DMARC1; p=none; rua=mailto:federicoadegiovanni@gmail.com"
```

Volver a Resend y esperar a que el dominio figure como verificado. La propagación puede tardar; no seguir hasta que verifique.

- [ ] **Step 3: Configurar el SMTP en Supabase**

Dashboard → Project Settings → Authentication → SMTP Settings. Habilitar "Enable Custom SMTP" y cargar host, puerto, usuario y contraseña que da Resend. Sender email: `no-responder@embolsadora.site`. Sender name: `Embolsadora`.

- [ ] **Step 4: Cargar las Redirect URLs**

Dashboard → Authentication → URL Configuration:

- Site URL: `https://embolsadora.site`
- Redirect URLs: agregar `https://embolsadora.site/**` y `http://localhost:3000/**`

Sin esto GoTrue descarta el `redirect_to` y manda al Site URL — el fix del backend queda anulado.

- [ ] **Step 5: Setear las env vars en Cloud Run**

```bash
gcloud run services update embolsadora-api \
  --project embolsadora --region us-east1 \
  --update-env-vars 'APP_BASE_URL=https://embolsadora.site,APP_ALLOWED_ORIGINS=https://embolsadora.site^|^http://localhost:3000'
```

> `--update-env-vars` usa la coma como separador entre variables, así que una lista con comas adentro necesita delimitador alternativo. La sintaxis `^|^` le dice a gcloud que use `|` como separador. Verificar el resultado con
> `gcloud run services describe embolsadora-api --project embolsadora --region us-east1 --format='value(spec.template.spec.containers[0].env)'`
> y confirmar que `APP_ALLOWED_ORIGINS` quedó con las dos URLs separadas por coma.

Para desarrollo local, agregar en `.env.local` del backend: `APP_ALLOWED_ORIGINS=http://localhost:3000`.

- [ ] **Step 6: Desplegar backend y frontend**

Backend: mergear a `main` — el workflow `.github/workflows/deploy-cloud-run.yml` despliega solo. Esta feature no agrega migraciones, así que no hay paso manual de DB.

Frontend: mergear a `main` — Vercel despliega solo el proyecto `v0-embolsadora`.

- [ ] **Step 7: Verificación end-to-end — el caso que originó todo esto**

Con el frontend local corriendo (`pnpm dev`) apuntando al backend de Cloud Run:

1. Invitar a `federicoadegiovanni+e2e-local@gmail.com` desde `http://localhost:3000`.
2. Invitar a `federicoadegiovanni+e2e-prod@gmail.com` desde `https://embolsadora.site`.

En cada mail recibido, confirmar:

- [ ] El remitente es `Embolsadora <no-responder@embolsadora.site>` — no `noreply@mail.app.supabase.io`.
- [ ] El asunto y el cuerpo están en español, con el diseño sobrio.
- [ ] Aparecen el nombre de la empresa, quién invitó y el rol.
- [ ] **El del local lleva `redirect_to=http://localhost:3000/...` y el de prod lleva `redirect_to=https://embolsadora.site/...`.** Es el criterio de aceptación central de todo el plan.
- [ ] No aparece el footer "Opt out of these emails".
- [ ] El link efectivamente abre el callback y deja crear la cuenta.

- [ ] **Step 8: Verificación end-to-end del reset de contraseña**

Desde `https://embolsadora.site`, forzar el cambio de contraseña de un usuario de prueba desde la pantalla de usuarios. Confirmar:

- [ ] Llega el mail (que es lo que hoy **no** pasa).
- [ ] Tiene el diseño nuevo y sale del remitente propio.
- [ ] El link lleva a `/s/{tenantId}/auth/change-password`.

- [ ] **Step 9: Anotar los dos resultados abiertos**

Registrar en el spec, al final, dos cosas que solo se saben ejecutando:

1. Si el campo *subject* admitió variables de template (si no, el asunto quedó fijo).
2. El resultado de la verificación del Step 1 de la Task 5 sobre `generate_link`.

```bash
git add docs/superpowers/specs/2026-07-29-invitation-email-design.md
git commit -m "docs: resultados de las dos incognitas abiertas del spec de mails"
```

---

## Autorrevisión del plan

**Cobertura del spec:**

| Requisito del spec | Tarea |
|---|---|
| Allow-list con match exacto, wildcard de previews, normalización | 1 |
| `APP_ALLOWED_ORIGINS` con fallback a `APP_BASE_URL` | 2, 11 |
| Header `X-App-Base-URL` → contexto, degradación con warn | 3 |
| Payload `data` con tenant_name / inviter_name / role_name | 4 |
| Fix de `generate_link` → `/auth/v1/recover` | 5 |
| Degradación: fallo de lookup no aborta el envío | 6 |
| `redirect_to` desde el contexto en invite y recovery | 7 |
| Log del 500 mudo en el handler de invitación | 7 |
| Las tres rutas BFF mandan el header | 8 |
| Cuatro plantillas HTML, table-based, inline CSS, es-AR, con fallbacks | 9 |
| Vencimiento "24 horas", no `expires_at` | 9 |
| Sin `{{ .SiteURL }}` en ninguna plantilla | 9, 10 |
| Renderer local de ambas ramas del `{{ if }}` | 9 |
| Publicación vía Management API, repo como fuente de verdad | 10 |
| SMTP Resend, DNS en DonWeb, Redirect URLs | 11 |
| E2E desde local y desde prod | 11 |

Sin huecos.

**Consistencia de tipos:** `apporigin.AllowList` se produce en T1 y se consume en T3 y T7. `supabase.InviteParams` se produce en T4 y se consume en T7. `TenantNameLookup` / `RoleNameLookup` se declaran en T6 y las satisfacen `tenants.TenantRepository.FindByID` y `roles.Repository.GetByIDForTenant`, verificadas contra el código actual. `callbackURL` se define en T7 Step 1 y se usa en T7 Steps 2 y 3. `platform.AppBaseURL` se define en T3 y se usa en T7.

**Nota sobre el orden:** las tareas 4, 5 y 6 dejan el repo sin compilar a propósito — cambian firmas que la Task 7 recién termina de cablear. Cada una verifica con `go test ./<su-paquete>/...`, no con `go build ./...`. La 7 es la que devuelve el build a verde. Ejecutarlas fuera de orden va a confundir.
