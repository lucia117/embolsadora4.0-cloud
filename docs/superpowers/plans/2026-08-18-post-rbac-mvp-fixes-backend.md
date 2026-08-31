# Post-RBAC MVP fixes (backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cerrar 2 bugs del backend documentados en `embolsadora-frontend/docs/superpowers/specs/2026-08-18-post-rbac-mvp-fixes-design.md` — activación de invitaciones que falla en silencio, y un 404 incorrecto en `GET /users/:id` para roles cross-tenant.

**Architecture:** Dos cambios independientes y desacoplados en capas ya existentes (middleware de auth, repo de usuarios), sin migraciones de DB ni cambios de contrato HTTP público (los shapes de request/response no cambian).

**Tech Stack:** Go 1.24, Gin, pgx/v5, testify. Go no está instalado en el host — todos los `go build`/`go test`/`go vet` corren vía Docker (ver Global Constraints).

## Global Constraints

- Todos los comandos `go` corren vía Docker, nunca directo en el host:
  ```bash
  docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine sh -c "<comando>"
  ```
- Los tests que requieren DB (`poolOrSkip(t)` / `DATABASE_URL`) necesitan `-e DATABASE_URL=postgres://...` en el `docker run`. Si no hay una DB de test disponible en el entorno de ejecución, estos tests se saltean solos (`t.Skip`) — no es un bloqueante para el resto de la tarea, pero hay que dejarlo explícito en el reporte de cada tarea si pasó así.
- Nunca usar `git add -A`/`git add .` — agregar archivos por nombre.
- Cada tarea termina con `go build ./...` y `go vet ./...` limpios antes de considerarse terminada, además de sus tests específicos.

---

### Task 1: `ActivatePendingInvitations` deja de fallar en silencio

**Files:**
- Modify: `internal/api/middleware/middleware.go:108-114`
- Create: `internal/api/middleware/jwt_auth_activation_test.go`

**Interfaces:**
- Consumes: `usecases.InvitationActivator` (`ActivatePendingInvitations(ctx, email, userID string) error`, ya existe, sin cambios), `security.Verifier` (`Verify(tokenString string) (*jwt.Token, error)`, ya existe), `usecases.AuthUsecase` (`NewAuthUsecase(userRepo users.UserRepository) *AuthUsecase`, ya existe), `users.UserRepository` (interfaz de 6 métodos en `internal/repo/pg/users/users_repo.go:15-22`, ya existe).
- Produces: ningún símbolo nuevo — es un cambio de comportamiento interno de `JWTAuth` (mismo nombre, misma firma exportada).

- [ ] **Step 1: Escribir el test que falla contra el comportamiento actual**

Crear `internal/api/middleware/jwt_auth_activation_test.go`:

```go
package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// fakeVerifier evita depender de un JWKS real: cualquier token string
// "válido" (no vacío) devuelve claims fijas.
type fakeVerifier struct{}

func (fakeVerifier) Verify(tokenString string) (*jwt.Token, error) {
	if tokenString == "" {
		return nil, errors.New("empty token")
	}
	return &jwt.Token{Claims: jwt.MapClaims{
		"sub":   "test-supabase-sub",
		"email": "invited@example.com",
	}}, nil
}

// fakeUserRepo implementa users.UserRepository con el mínimo necesario:
// UpsertBySupabaseID (llamado por ProvisionUser) devuelve un usuario
// 'invited' fijo. El resto de los métodos no los llama JWTAuth en este flujo.
type fakeUserRepo struct{}

func (fakeUserRepo) UpsertBySupabaseID(ctx context.Context, supabaseUserID, email string) (*domain.User, error) {
	return &domain.User{ID: "test-user-id", Email: email, Status: domain.UserStatusInvited}, nil
}
func (fakeUserRepo) GetBySupabaseID(ctx context.Context, supabaseUserID string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (fakeUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, errors.New("not implemented")
}
func (fakeUserRepo) SetStatus(ctx context.Context, userID string, status domain.UserStatus) error {
	return nil
}
func (fakeUserRepo) SetPasswordChangeRequired(ctx context.Context, userID string, value bool) error {
	return nil
}
func (fakeUserRepo) IsActiveMemberOfTenant(ctx context.Context, userID, tenantID string) (bool, error) {
	return false, nil
}

// failingActivator siempre falla — simula el caso real que dejó 5
// invitaciones activadas con rol incorrecto en producción.
type failingActivator struct{}

func (failingActivator) ActivatePendingInvitations(ctx context.Context, email, userID string) error {
	return errors.New("simulated activation failure")
}

func runJWTAuth(t *testing.T, activator usecases.InvitationActivator) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	authUC := usecases.NewAuthUsecase(fakeUserRepo{})
	r := gin.New()
	r.Use(apimw.JWTAuth(fakeVerifier{}, authUC, activator))
	r.GET("/probe", func(c *gin.Context) {
		user := platform.DomainUser(c.Request.Context())
		require.NotNil(t, user)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuth_ActivacionFallaAbortaElRequest(t *testing.T) {
	w := runJWTAuth(t, failingActivator{})
	assert.Equal(t, http.StatusInternalServerError, w.Code,
		"si ActivatePendingInvitations falla, el request debe abortar en vez de continuar con el usuario 'invited' en un estado inconsistente")
}
```

- [ ] **Step 2: Correr el test y confirmar que falla**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/middleware/... -run TestJWTAuth_ActivacionFallaAbortaElRequest -v"
```

Esperado: **FAIL** — hoy `w.Code` es `200` (el middleware sigue con `c.Next()` pese al error de activación), no `500`.

- [ ] **Step 3: Aplicar el fix**

En `internal/api/middleware/middleware.go`, reemplazar el bloque de activación (líneas 107-124 del archivo actual):

```go
		if activator != nil && user.Status == domain.UserStatusInvited && email != "" {
			if err := activator.ActivatePendingInvitations(ctx, email, user.ID); err != nil {
				Log.Warn("failed to activate pending invitations",
					zap.String("user_id", user.ID),
					zap.Error(err),
				)
			} else if refreshed, err := authUC.ProvisionUser(ctx, sub, email); err != nil {
```

por:

```go
		if activator != nil && user.Status == domain.UserStatusInvited && email != "" {
			if err := activator.ActivatePendingInvitations(ctx, email, user.ID); err != nil {
				Log.Error("failed to activate pending invitations",
					zap.String("user_id", user.ID),
					zap.Error(err),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "activation failed"})
				return
			} else if refreshed, err := authUC.ProvisionUser(ctx, sub, email); err != nil {
```

Solo cambia esa rama: `Log.Warn` → `Log.Error` + `c.AbortWithStatusJSON(...)` + `return`. El `else if` que sigue (falla el re-fetch post-activación exitosa) no se toca — sigue como `Log.Warn` sin abortar.

- [ ] **Step 4: Correr el test y confirmar que pasa**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/middleware/... -run TestJWTAuth_ActivacionFallaAbortaElRequest -v"
```

Esperado: **PASS**.

- [ ] **Step 5: Test de control — el camino feliz no se rompe**

Agregar al mismo archivo `jwt_auth_activation_test.go`:

```go
// succeedingActivator no falla — control positivo: el camino feliz (sin
// invitaciones pendientes, o activación exitosa) no debe verse afectado.
type succeedingActivator struct{}

func (succeedingActivator) ActivatePendingInvitations(ctx context.Context, email, userID string) error {
	return nil
}

func TestJWTAuth_ActivacionExitosaSigueDeLargo(t *testing.T) {
	w := runJWTAuth(t, succeedingActivator{})
	assert.Equal(t, http.StatusOK, w.Code, "una activación exitosa no debe abortar el request")
}
```

Correr:

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/api/middleware/... -run TestJWTAuth -v"
```

Esperado: **PASS** en ambos tests (`TestJWTAuth_ActivacionFallaAbortaElRequest`, `TestJWTAuth_ActivacionExitosaSigueDeLargo`).

- [ ] **Step 6: Build y vet completos**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./... && go vet ./..."
```

Esperado: sin errores.

- [ ] **Step 7: Commit**

```bash
git add internal/api/middleware/middleware.go internal/api/middleware/jwt_auth_activation_test.go
git commit -m "fix: JWTAuth aborta el request si falla activar invitaciones pendientes

Antes solo logueaba Warn y seguía con el usuario en estado 'invited',
lo que ya causó 5 invitaciones activadas con rol incorrecto en
producción (detectado por auditoría SQL manual, no por logs)."
```

---

### Task 2: `GET /users/:id` deja de dar 404 para roles cross-tenant

**Files:**
- Modify: `internal/repo/pg/users/repository.go:16-17` (interfaz)
- Modify: `internal/repo/pg/users/postgres.go:96-131` (implementación de `GetByID`)
- Modify: `internal/app/users/service.go:54-70` (`GetUser`), `:176-242` (`UpdateUser`), `:243-269` (`DeleteUser`), `:290-352` (`UpdateUserStatus`)
- Modify: `internal/api/usecases/password_usecase.go:76`
- Modify: `internal/api/handler/users/handler.go:97-` (`GetUser` handler)
- Modify: `internal/repo/pg/users/postgres_repo_test.go:75`, `internal/repo/pg/users/cloaking_test.go:135,138` (llamadas existentes a `GetByID`, agregar el nuevo argumento)
- Test: `internal/repo/pg/users/cross_tenant_test.go` (nuevo)

**Interfaces:**
- Consumes: `security.IsCrossTenantRole(ctx context.Context) bool` (ya existe, `internal/security/rbac.go:98`, misma convención que `get_tenant.go`/`get_user_roles.go`).
- Produces: `Repository.GetByID(ctx, tenantID, userID string, crossTenant, includeGlobal bool) (*users.User, error)` — firma nueva, reemplaza la de 4 argumentos. `Service.GetUser(ctx, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.User, error)` — firma nueva.

**Importante — alcance:** este cambio agrega un parámetro nuevo a `Repository.GetByID`, que hoy tiene 5 call sites en código de producción además de `GetUser`. Los otros 4 (`UpdateUser`, `DeleteUser`, `UpdateUserStatus` ×2, y `ForcePasswordChange` en `password_usecase.go`) **deben seguir pasando `false` literal** — no forman parte de este fix, y pasar `true` ahí sería una expansión de alcance no pedida (le daría a roles cross-tenant la posibilidad de mutar/forzar cambio de password de usuarios de otros tenants, que no es lo que se decidió). Solo `Service.GetUser` recibe el valor real calculado por el handler.

- [ ] **Step 1: Escribir el test que falla contra el comportamiento actual**

Crear `internal/repo/pg/users/cross_tenant_test.go`:

```go
package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// seedTenant crea un tenant nuevo con subdominio único y lo limpia al terminar.
func seedTenant(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Cross Tenant Test', 'Cross Tenant Test', $2)
	`, id, "xt-"+id[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	return id
}

// seedUserInTenant crea un usuario con membresía activa 'cliente_operario' en
// el tenant dado (rol tenant-scoped, no is_global, para no mezclar con el eje
// includeGlobal que ya cubre cloaking_test.go).
func seedUserInTenant(t *testing.T, pool *pgxpool.Pool, tenantID string) string {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New().String()
	utrID := uuid.New().String()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Cross Tenant User', 'active')`,
		userID, userID+"@xtenant.local")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, 'cliente_operario', 'active', NOW())`,
		utrID, userID, tenantID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

func TestGetByIDCrossTenantFalseDaNotFoundParaUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	// Regresión: un caller sin capability cross-tenant (crossTenant=false, el
	// comportamiento de siempre) NO debe poder ver un usuario de otro tenant.
	_, err := repo.GetByID(ctx, tenantA, userInB, false, false)
	require.Error(t, err, "sin crossTenant, un usuario de otro tenant debe seguir devolviendo 404")
}

func TestGetByIDCrossTenantTrueEncuentraUsuarioDeOtroTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	tenantA := seedTenant(t, pool)
	tenantB := seedTenant(t, pool)
	userInB := seedUserInTenant(t, pool, tenantB)

	// Hallazgo A: un super_admin parado en el contexto de tenantA pidiendo un
	// usuario de tenantB debe encontrarlo, no 404.
	u, err := repo.GetByID(ctx, tenantA, userInB, true, false)
	require.NoError(t, err)
	require.Equal(t, userInB, u.ID)
}
```

- [ ] **Step 2: Correr el test y confirmar que falla (no compila todavía)**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... -run TestGetByIDCrossTenant -v"
```

Esperado: **FAIL en compilación** — `GetByID` todavía tiene 4 argumentos, no 5. Este fallo de compilación es la confirmación de que el test ejercita la firma nueva.

- [ ] **Step 3: Actualizar la interfaz**

En `internal/repo/pg/users/repository.go:16-17`:

```go
	// GetByID retrieves a single user by ID (returns ErrNotFound if soft-deleted, not found u oculto)
	GetByID(ctx context.Context, tenantID, userID string, includeGlobal bool) (*users.User, error)
```

por:

```go
	// GetByID retrieves a single user by ID (returns ErrNotFound if soft-deleted, not found u oculto).
	// crossTenant=true omite el filtro de tenant por completo (ver Hallazgo A:
	// un caller con security.IsCrossTenantRole debe poder resolver un usuario
	// de cualquier tenant, no solo el de la request). includeGlobal sigue
	// siendo un eje separado — decide si un usuario cuyo rol es is_global es
	// visible, no afecta el scoping por tenant.
	GetByID(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*users.User, error)
```

- [ ] **Step 4: Actualizar la query en `postgres.go`**

En `internal/repo/pg/users/postgres.go:96-131`, la firma y el WHERE:

```go
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, userID string, includeGlobal bool) (*users.User, error) {
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, $1) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		LEFT JOIN user_tenant_roles utr
			ON utr.user_id = u.id AND utr.tenant_id = $1 AND utr.status = 'active'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.id = $1
```

(`$1` = `userID`, `$2` = `tenantID`, `$3` = `includeGlobal` — confirmado contra `r.db.QueryRow(ctx, query, userID, tenantID, includeGlobal)` en el archivo real).

Cambiar a (agregando el nuevo parámetro `crossTenant` como `$4` y el predicado `OR $4`):

```go
func (r *PostgresRepository) GetByID(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*users.User, error) {
	query := `
		SELECT u.id,
		       COALESCE(u.tenant_id, $2) AS tenant_id,
		       COALESCE(u.first_name, u.name, '') AS first_name,
		       COALESCE(u.last_name, '') AS last_name,
		       u.email,
		       COALESCE(utr.role_id, u.role) AS role,
		       u.image, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		LEFT JOIN user_tenant_roles utr
			ON utr.user_id = u.id AND utr.tenant_id = $2 AND utr.status = 'active'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.id = $1
		  AND u.deleted_at IS NULL
		  AND (u.tenant_id = $2 OR utr.id IS NOT NULL OR $4)
		  AND (COALESCE(r.is_global, FALSE) = FALSE OR $3)
	`

	row := r.db.QueryRow(ctx, query, userID, tenantID, includeGlobal, crossTenant)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, users.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}
```

`crossTenant` es un `OR` agregado al predicado de tenant (línea `AND (u.tenant_id = $2 OR utr.id IS NOT NULL OR $4)`) — el predicado de `is_global` (`$3`) no cambia.

- [ ] **Step 5: Propagar el parámetro por `Service.GetUser` y los otros 4 call sites**

En `internal/app/users/service.go`, cambiar la firma de `GetUser` (línea 54):

```go
func (s *Service) GetUser(ctx context.Context, tenantID, userID string, includeGlobal bool) (*domainUsers.User, error) {
	s.logger.Debug("getting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	user, err := s.repo.GetByID(ctx, tenantID, userID, includeGlobal)
```

por:

```go
func (s *Service) GetUser(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.User, error) {
	s.logger.Debug("getting user", zap.String("tenant_id", tenantID), zap.String("user_id", userID))

	user, err := s.repo.GetByID(ctx, tenantID, userID, crossTenant, includeGlobal)
```

En las otras 4 líneas que llaman `s.repo.GetByID(ctx, tenantID, userID, includeGlobal)` dentro del mismo archivo (`UpdateUser:185`, `DeleteUser:246`, `UpdateUserStatus:310` y `:346` — verificar los números de línea exactos con `Read` antes de editar, pueden haber corrido levemente), agregar `false` como `crossTenant` **sin cambiar la firma de esas 4 funciones**:

```go
current, err := s.repo.GetByID(ctx, tenantID, userID, false, includeGlobal)
```

(mismo patrón en las otras 3 líneas — `false` primero, `includeGlobal` después, para que coincida con el orden `(ctx, tenantID, userID, crossTenant, includeGlobal)` de la interfaz).

- [ ] **Step 6: Propagar `false` en `password_usecase.go`**

En `internal/api/usecases/password_usecase.go:76`:

```go
	if _, err := uc.mgmtUserRepo.GetByID(ctx, tenantID, targetUserID, includeGlobal); err != nil {
```

por:

```go
	if _, err := uc.mgmtUserRepo.GetByID(ctx, tenantID, targetUserID, false, includeGlobal); err != nil {
```

- [ ] **Step 7: Calcular `crossTenant` en el handler**

En `internal/api/handler/users/handler.go`, dentro de `GetUser` (línea ~97), después de la línea `includeGlobal := security.CanSeePlatformInternals(c.Request.Context())`:

```go
	includeGlobal := security.CanSeePlatformInternals(c.Request.Context())
```

agregar debajo:

```go
	crossTenant := security.IsCrossTenantRole(c.Request.Context())
```

Y en la rama que llama `h.service.GetUser` (no la rama `include=roles`, que queda sin tocar — fuera de alcance):

```go
	user, err := h.service.GetUser(c.Request.Context(), tenantID, userID, includeGlobal)
```

por:

```go
	user, err := h.service.GetUser(c.Request.Context(), tenantID, userID, crossTenant, includeGlobal)
```

- [ ] **Step 8: Actualizar los call sites existentes en tests**

En `internal/repo/pg/users/postgres_repo_test.go:75`:

```go
	current, err := repo.GetByID(ctx, tenantID, userID, false)
```

por:

```go
	current, err := repo.GetByID(ctx, tenantID, userID, false, false)
```

En `internal/repo/pg/users/cloaking_test.go:135,138`:

```go
	_, err := repo.GetByID(ctx, platformTenant, superID, false)
	...
	u, err := repo.GetByID(ctx, platformTenant, superID, true)
```

por:

```go
	_, err := repo.GetByID(ctx, platformTenant, superID, false, false)
	...
	u, err := repo.GetByID(ctx, platformTenant, superID, false, true)
```

(el nuevo argumento `crossTenant` va **antes** de `includeGlobal`, en `false` en ambos casos — ese test ejercita el eje `includeGlobal`, no `crossTenant`, así que debe quedar fijo en `false` para no cambiar lo que el test verifica).

- [ ] **Step 9: Correr los tests nuevos y confirmar que pasan**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... -run TestGetByIDCrossTenant -v"
```

Esperado: **PASS** en los dos tests nuevos.

- [ ] **Step 10: Correr toda la suite de `users` y confirmar que no hay regresiones**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go test ./internal/repo/pg/users/... ./internal/app/users/... ./internal/api/handler/users/... ./internal/api/usecases/... -v"
```

Esperado: **PASS** en todo (incluye `TestGetByIDDevuelveNotFoundParaUsuarioOculto`, `TestUpdate_UserWithNullTenantIDColumn_Succeeds`, y cualquier test de `password_usecase.go` que exista — confirmar que ninguno rompió por el nuevo argumento).

- [ ] **Step 11: Build y vet completos**

```bash
docker run --rm -v /tmp/go-mod-cache:/go/pkg/mod -v $(pwd):/app -w /app golang:1.24-alpine \
  sh -c "go build ./... && go vet ./..."
```

Esperado: sin errores — esto confirma que no quedó ningún call site de `GetByID` sin actualizar en todo el repo.

- [ ] **Step 12: Commit**

```bash
git add internal/repo/pg/users/repository.go internal/repo/pg/users/postgres.go \
  internal/app/users/service.go internal/api/usecases/password_usecase.go \
  internal/api/handler/users/handler.go \
  internal/repo/pg/users/postgres_repo_test.go internal/repo/pg/users/cloaking_test.go \
  internal/repo/pg/users/cross_tenant_test.go
git commit -m "fix: GET /users/:id encuentra usuarios de otro tenant para roles cross-tenant

Repository.GetByID gana un parámetro crossTenant explícito (mismo
patrón que user_roles.FindByUser), calculado en el handler vía
security.IsCrossTenantRole. Los demás call sites (UpdateUser,
DeleteUser, UpdateUserStatus, ForcePasswordChange) mantienen
crossTenant=false — este fix es solo para GET /users/:id."
```
