# Gaps de cobertura en repos (roles, tenants, users) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cerrar los gaps de cobertura detectados en `internal/repo/pg/{roles,tenants,users}` — tres paquetes que ya tenían tests (de cloaking/cross-tenant/settings) pero les faltaban métodos CRUD básicos sin ningún test, incluyendo el repo de auto-provisioning (`users.NewUserRepository`) que corre en cada request autenticado.

**Architecture:** Cada tarea agrega un archivo de test nuevo a un paquete que YA tiene tests de integración — reutiliza los helpers (`openPool`/`createTestTenant`, `poolOrSkip`/`seedTenant`, etc.) ya definidos en archivos hermanos del mismo paquete de test, en vez de redeclararlos. Mismo patrón que `docs/superpowers/plans/2026-09-21-provider-test-coverage.md`: tests de integración reales contra Postgres, gateados por `DATABASE_URL`, `t.Skip` si falta.

**Tech Stack:** Go 1.24, testify (`require`/`assert`), `pgx/v5/pgxpool`, `google/uuid`.

**Spec:** No hay spec previa — nace del relevamiento de cobertura hecho en conversación (verificado leyendo cada `repository.go` y grepeando qué métodos aparecen invocados en los `_test.go` existentes de cada paquete).

## Global Constraints

- **Nunca usar `uber/mock` / `mockgen`.** Tests de integración contra Postgres real, igual que el resto del repo.
- **`testify`** (`require` para asserts que abortan el test, `assert` para el resto).
- **Gateo por `DATABASE_URL`, siempre `t.Skip`, nunca `t.Fatal`.**
- **No redeclarar helpers que ya existen en el paquete de test.** Cada tarea vive en el mismo `package X_test` que archivos hermanos ya committeados — reusar sus helpers, o el `go vet`/build falla por redeclaración:
  - `internal/repo/pg/roles`: `openPool(t)`, `createTestTenant(t, pool)`, `platformTenantUUID`, `roleIDs(...)` (definidos en `repository_test.go`).
  - `internal/repo/pg/tenants`: `newTestTenant()` (definido en `repository_test.go`).
  - `internal/repo/pg/users`: `poolOrSkip(t)`, `platformTenant` (en `cloaking_test.go`); `seedTenant(t, pool)` (en `cross_tenant_test.go`).
- **Constraints reales de la DB relevantes para estas tareas** (confirmado leyendo `migrations/000001_initial_schema.up.sql` y `000012_cascade_user_tenant_roles_tenant_fkey.up.sql`):
  - `idx_roles_tenant_name_active`: `UNIQUE (tenant_id, name) WHERE deleted_at IS NULL AND is_system_role = false` → dispara `domain.ErrRoleDuplicateName` en `Create`/`Update` (mapeado por código `23505`).
  - `users_tenant_id_email_key`: `UNIQUE (tenant_id, email)` → dispara `users.ErrEmailTaken` en `Create`/`CreateWithRole`.
  - `user_tenant_roles_role_id_fkey`: FK a `roles(id)` sin cascade → dispara `domain.ErrInvalidRoleID` en `CreateWithRole` cuando el `role_id` no existe (el código solo mapea este error si `pgErr.ConstraintName == "user_tenant_roles_role_id_fkey"`).
  - `idx_utr_active_unique`: `UNIQUE (user_id, tenant_id) WHERE status = 'active'` → mapeado en código a `domain.ErrUserAlreadyHasActiveRole`, pero **inalcanzable vía la API pública actual**: `CreateWithRole` sobreescribe `user.ID = uuid.New().String()` incondicionalmente antes de insertar, así que dos llamadas nunca comparten `user_id`. No forzar un test para esa rama — quedaría marcado como código muerto/defensivo, no un caso real.
  - `roles_tenant_id_fkey` / `users_tenant_id_fkey`: `ON DELETE CASCADE` hacia `tenants(id)` — un tenant de test se puede limpiar sin borrar manualmente sus roles/users custom, pero cada tarea igual limpia explícito con `t.Cleanup` por claridad y porque el orden LIFO entre archivos hermanos no está garantizado.
  - `users.status` (columna) NO tiene CHECK constraint en la DB — la validación de `UserStatus` es solo en Go (`domain.UserStatus`), cualquier string cabe en `varchar(20)`.
- **`SetStatus`/`SetPasswordChangeRequired` (en `pgUserRepo`) no chequean `RowsAffected`.** Un `id` inexistente no da error, es un no-op silencioso — es el comportamiento real del código, documentarlo con un test, no "corregirlo" (este plan es backfill de cobertura, no TDD clásico: si algo da RED es señal de bug real a reportar aparte, no de que falte implementar algo).
- **IDs de test únicos vía `uuid.NewString()[:8]`** en emails/nombres para evitar choques con los `UNIQUE` de arriba si los tests corren en paralelo contra la misma DB compartida.
- **Comando de verificación por tarea:** el `go test` exacto de cada tarea. Verificación global al final: `go build ./...` y `go test ./...` (sin `DATABASE_URL` para confirmar que sigue skippeando limpio).

## File Structure

| # | Paquete | Acción | Archivo de test |
|---|---|---|---|
| 1 | `internal/repo/pg/roles` | Crear | `crud_test.go` (complementa `repository_test.go`, no lo toca) |
| 2 | `internal/repo/pg/tenants` | Crear | `find_all_test.go` (complementa `repository_test.go`, no lo toca) |
| 3 | `internal/repo/pg/users` (tipo `PostgresRepository`) | Crear | `create_test.go` |
| 4 | `internal/repo/pg/users` (tipo `pgUserRepo` / `UserRepository`) | Crear | `user_repository_test.go` |

---

## Task 1: `internal/repo/pg/roles` — `Create` (duplicado), `Update`, `SoftDelete`, `CountCustomByTenant`, `CountActiveAssignments` sin test

**Files:**
- Create: `internal/repo/pg/roles/crud_test.go`

**Interfaces:**
- Consumes: `rolesRepo.NewPostgresRepository(pool)` (`Create`, `Update`, `SoftDelete`, `CountCustomByTenant`, `CountActiveAssignments`, `GetByIDForTenant`), `domain.Role`, `domain.ErrRoleNotFound`, `domain.ErrRoleDuplicateName`. Reusa `openPool(t)`, `createTestTenant(t, pool)`, `platformTenantUUID` de `repository_test.go` (mismo paquete, no se importan — ya están en el package).
- Produces: n/a (archivo de test hoja).

`repository_test.go` ya cubre `List`/`GetByIDForTenant` a fondo (cloaking/scoping) y usa `Create` como setup, pero nunca verifica el propio comportamiento de `Create` (duplicado), ni toca `Update`, `SoftDelete`, `CountCustomByTenant` o `CountActiveAssignments`.

- [x] **Step 1: Crear el archivo con los helpers propios de esta tarea y el test de `Create` duplicado**

```go
package roles_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
)

// seedUser crea un usuario minimo descartable, sin tenant ni rol — alcanza
// como FK target de user_tenant_roles.user_id para los tests de
// CountActiveAssignments.
func seedUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, status) VALUES ($1, $2, 'active')`,
		id, id.String()+"@roles-crud.local")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func newCustomRole(tenantID uuid.UUID, name string) *domain.Role {
	return &domain.Role{
		ID:          "custom_" + uuid.NewString()[:8],
		Name:        name,
		Description: "rol de test",
		Permissions: []string{},
		TenantID:    &tenantID,
	}
}

func TestCreateDuplicateNameEnElMismoTenant(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	first := newCustomRole(tenantID, "Rol duplicado")
	require.NoError(t, repo.Create(ctx, first))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, first.ID) })

	dup := newCustomRole(tenantID, "Rol duplicado")
	err := repo.Create(ctx, dup)
	assert.ErrorIs(t, err, domain.ErrRoleDuplicateName)
}
```

- [x] **Step 2: `Update` — feliz, not-found, nombre duplicado**

```go
func TestUpdateHappyPathAndNotFound(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	role := newCustomRole(tenantID, "Original")
	require.NoError(t, repo.Create(ctx, role))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, role.ID) })

	role.Name = "Renombrado"
	role.Description = "nueva descripcion"
	role.Permissions = []string{"perm_users_view"}
	require.NoError(t, repo.Update(ctx, role))

	got, err := repo.GetByIDForTenant(ctx, role.ID, tenantID, false)
	require.NoError(t, err)
	assert.Equal(t, "Renombrado", got.Name)
	assert.Equal(t, "nueva descripcion", got.Description)
	assert.Equal(t, []string{"perm_users_view"}, got.Permissions)

	ghost := newCustomRole(tenantID, "fantasma")
	ghost.ID = "no-existe-" + uuid.NewString()[:8]
	err = repo.Update(ctx, ghost)
	assert.ErrorIs(t, err, domain.ErrRoleNotFound)
}

func TestUpdateNombreDuplicado(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	roleA := newCustomRole(tenantID, "Rol A")
	require.NoError(t, repo.Create(ctx, roleA))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA.ID) })

	roleB := newCustomRole(tenantID, "Rol B")
	require.NoError(t, repo.Create(ctx, roleB))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleB.ID) })

	roleB.Name = "Rol A"
	err := repo.Update(ctx, roleB)
	assert.ErrorIs(t, err, domain.ErrRoleDuplicateName)
}
```

- [x] **Step 3: `SoftDelete` — feliz, not idempotente, y no visible tras borrar**

```go
func TestSoftDeleteHappyPathAndNotFound(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	role := newCustomRole(tenantID, "Para borrar")
	require.NoError(t, repo.Create(ctx, role))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, role.ID) })

	require.NoError(t, repo.SoftDelete(ctx, role.ID))

	_, err := repo.GetByIDForTenant(ctx, role.ID, tenantID, false)
	assert.ErrorIs(t, err, domain.ErrRoleNotFound, "un rol soft-deleted debe dejar de ser visible")

	err = repo.SoftDelete(ctx, role.ID)
	assert.ErrorIs(t, err, domain.ErrRoleNotFound, "borrar de nuevo un rol ya borrado debe fallar, no ser idempotente")
}
```

- [x] **Step 4: `CountCustomByTenant` y `CountActiveAssignments`**

```go
func TestCountCustomByTenant(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantA := createTestTenant(t, pool)
	tenantB := createTestTenant(t, pool)

	roleA1 := newCustomRole(tenantA, "A1")
	require.NoError(t, repo.Create(ctx, roleA1))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA1.ID) })
	roleA2 := newCustomRole(tenantA, "A2")
	require.NoError(t, repo.Create(ctx, roleA2))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleA2.ID) })
	roleB := newCustomRole(tenantB, "B1")
	require.NoError(t, repo.Create(ctx, roleB))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, roleB.ID) })

	count, err := repo.CountCustomByTenant(ctx, tenantA)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, repo.SoftDelete(ctx, roleA2.ID))
	count, err = repo.CountCustomByTenant(ctx, tenantA)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "un rol soft-deleted no debe contarse")
}

func TestCountActiveAssignments(t *testing.T) {
	pool := openPool(t)
	repo := rolesRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := createTestTenant(t, pool)

	role := newCustomRole(tenantID, "Con asignaciones")
	require.NoError(t, repo.Create(ctx, role))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM roles WHERE id = $1`, role.ID) })

	count, err := repo.CountActiveAssignments(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	userA := seedUser(t, pool)
	utrAID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at) VALUES ($1, $2, $3, $4, 'active', NOW())`,
		utrAID, userA, tenantID, role.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrAID) })

	userB := seedUser(t, pool)
	utrBID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at) VALUES ($1, $2, $3, $4, 'revoked', NOW())`,
		utrBID, userB, tenantID, role.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrBID) })

	count, err = repo.CountActiveAssignments(ctx, role.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "solo debe contar la asignacion 'active', no la 'revoked'")
}
```

- [x] **Step 5: Correr los tests del paquete completo (nuevo + `repository_test.go` existente)**

Run: `go test ./internal/repo/pg/roles/... -v`
Expected: sin `DATABASE_URL`, todos SKIP. Con `DATABASE_URL` (`docker compose up -d db`), todos PASS.

- [x] **Step 6: Commit**

```bash
git add internal/repo/pg/roles/crud_test.go
git commit -m "test(roles): cubrir Create(duplicado)/Update/SoftDelete/CountCustomByTenant/CountActiveAssignments"
```

---

## Task 2: `internal/repo/pg/tenants` — `FindAll` sin test

**Files:**
- Create: `internal/repo/pg/tenants/find_all_test.go`

**Interfaces:**
- Consumes: `tenants.NewTenantRepository(db)` (`FindAll`, `Create`, `Delete`), `domain.Tenant`. Reusa `newTestTenant()` de `repository_test.go` (mismo paquete).
- Produces: n/a.

`FindAll` no está scopeado por tenant (devuelve todos los tenants del sistema), así que el test verifica membership (que los tenants creados aparezcan en el resultado) en vez de igualdad exacta de la lista — la DB puede tener otros tenants sembrados por migraciones o por otros tests corriendo en la misma base compartida.

- [x] **Step 1: Crear el archivo**

```go
package tenants_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
)

func TestFindAll_IncluyeLosTenantsCreados(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	require.NoError(t, err)
	defer db.Close()

	repo := tenants.NewTenantRepository(db)
	ctx := context.Background()

	first := newTestTenant()
	require.NoError(t, repo.Create(ctx, first))
	defer func() { _ = repo.Delete(ctx, first.ID) }()

	second := newTestTenant()
	require.NoError(t, repo.Create(ctx, second))
	defer func() { _ = repo.Delete(ctx, second.ID) }()

	all, err := repo.FindAll(ctx)
	require.NoError(t, err)

	ids := make(map[uuid.UUID]bool, len(all))
	for _, tn := range all {
		ids[tn.ID] = true
	}
	assert.True(t, ids[first.ID], "FindAll debe incluir el primer tenant creado")
	assert.True(t, ids[second.ID], "FindAll debe incluir el segundo tenant creado")
}
```

- [x] **Step 2: Correr los tests del paquete completo (nuevo + `repository_test.go` existente)**

Run: `go test ./internal/repo/pg/tenants/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [x] **Step 3: Commit**

```bash
git add internal/repo/pg/tenants/find_all_test.go
git commit -m "test(tenants): cubrir FindAll (sin ningun test hasta ahora)"
```

---

## Task 3: `internal/repo/pg/users` (`PostgresRepository`) — `Create` y `CreateWithRole` sin test

**Files:**
- Create: `internal/repo/pg/users/create_test.go`

**Interfaces:**
- Consumes: `usersRepo.NewPostgresRepository(pool)` (`Create`, `CreateWithRole`), `domusers.User`/`ErrValidation`/`ErrEmailTaken`, `domain.UserTenantRole`/`ErrInvalidRoleID`. Reusa `poolOrSkip(t)` (`cloaking_test.go`) y `seedTenant(t, pool)` (`cross_tenant_test.go`), mismo paquete.
- Produces: n/a.

`CreateWithRole` usa el rol seed `cliente_operario` (archetype global, `tenant_id IS NULL`, reusable por cualquier tenant — mismo rol que usa `seedUserInTenant` en `cross_tenant_test.go`) para no depender de crear un rol custom aparte. La rama `ErrUserAlreadyHasActiveRole` de `CreateWithRole` no se testea: es inalcanzable vía la API pública actual (ver Global Constraints).

- [x] **Step 1: Crear el archivo con `Create` — feliz, email duplicado, validación**

```go
package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	domusers "github.com/tu-org/embolsadora-api/internal/domain/users"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

func newCrudUser(tenantID, email string) *domusers.User {
	return &domusers.User{
		TenantID:  tenantID,
		FirstName: "Test",
		LastName:  "User",
		Email:     email,
		Role:      domusers.RoleUser,
	}
}

func TestCreate_HappyPathAndDuplicateEmail(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)

	email := "crud-" + uuid.NewString()[:8] + "@test.local"
	user := newCrudUser(tenantID, email)
	created, err := repo.Create(ctx, user)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	assert.Equal(t, email, created.Email)
	assert.False(t, created.CreatedAt.IsZero())

	dup := newCrudUser(tenantID, email)
	_, err = repo.Create(ctx, dup)
	assert.ErrorIs(t, err, domusers.ErrEmailTaken)
}

func TestCreate_ValidacionFalla(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	invalid := &domusers.User{TenantID: "algun-tenant", Email: "sin-nombre@test.local", Role: domusers.RoleUser}
	_, err := repo.Create(ctx, invalid)
	assert.ErrorIs(t, err, domusers.ErrValidation, "sin FirstName/LastName debe fallar la validacion antes de tocar la DB")
}
```

- [x] **Step 2: `CreateWithRole` — feliz, `role_id` inexistente (con rollback), email duplicado**

```go
func ptrTo(s string) *string { return &s }

func TestCreateWithRole_HappyPath(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)
	tenantUUID, err := uuid.Parse(tenantID)
	require.NoError(t, err)

	email := "crud-role-" + uuid.NewString()[:8] + "@test.local"
	user := newCrudUser(tenantID, email)
	utr := &domain.UserTenantRole{
		ID:       uuid.New(),
		TenantID: tenantUUID,
		RoleID:   ptrTo("cliente_operario"),
		Status:   domain.UserRoleStatusActive,
	}

	created, err := repo.CreateWithRole(ctx, user, utr)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utr.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID)
	})

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM user_tenant_roles WHERE id = $1`, utr.ID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "active", status)
}

func TestCreateWithRole_RoleIDInexistente(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)
	tenantUUID, err := uuid.Parse(tenantID)
	require.NoError(t, err)

	email := "crud-badrole-" + uuid.NewString()[:8] + "@test.local"
	user := newCrudUser(tenantID, email)
	utr := &domain.UserTenantRole{
		ID:       uuid.New(),
		TenantID: tenantUUID,
		RoleID:   ptrTo("no-existe-" + uuid.NewString()[:8]),
		Status:   domain.UserRoleStatusActive,
	}

	_, err = repo.CreateWithRole(ctx, user, utr)
	assert.ErrorIs(t, err, domain.ErrInvalidRoleID)

	// CreateWithRole hace rollback de toda la transaccion: el usuario no debe quedar huerfano.
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count))
	assert.Equal(t, 0, count, "el rollback debe deshacer tambien el INSERT de users")
}

func TestCreateWithRole_EmailDuplicadoEnElMismoTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)
	tenantUUID, err := uuid.Parse(tenantID)
	require.NoError(t, err)

	email := "crud-dup-" + uuid.NewString()[:8] + "@test.local"
	first := newCrudUser(tenantID, email)
	firstUTR := &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantUUID, RoleID: ptrTo("cliente_operario"), Status: domain.UserRoleStatusActive}
	createdFirst, err := repo.CreateWithRole(ctx, first, firstUTR)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, firstUTR.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, createdFirst.ID)
	})

	second := newCrudUser(tenantID, email)
	secondUTR := &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantUUID, RoleID: ptrTo("cliente_operario"), Status: domain.UserRoleStatusActive}
	_, err = repo.CreateWithRole(ctx, second, secondUTR)
	assert.ErrorIs(t, err, domusers.ErrEmailTaken)
}
```

- [x] **Step 3: Correr los tests del paquete completo (nuevo + los `_test.go` existentes)**

Run: `go test ./internal/repo/pg/users/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [x] **Step 4: Commit**

```bash
git add internal/repo/pg/users/create_test.go
git commit -m "test(users): cubrir Create y CreateWithRole de PostgresRepository (sin ningun test hasta ahora)"
```

---

## Task 4: `internal/repo/pg/users` (`pgUserRepo` / `UserRepository`) — todo excepto `UpsertBySupabaseID` sin test

**Files:**
- Create: `internal/repo/pg/users/user_repository_test.go`

**Interfaces:**
- Consumes: `usersRepo.NewUserRepository(pool)` (`UpsertBySupabaseID`, `GetBySupabaseID`, `GetByID`, `SetStatus`, `SetPasswordChangeRequired`, `IsActiveMemberOfTenant`), `domain.User`/`UserStatus*`/`ErrNotFound`. Reusa `poolOrSkip(t)` y `seedTenant(t, pool)`, mismo paquete.
- Produces: n/a.

Este es el repo que usa `AuthUsecase.ProvisionUser()` en cada request autenticado (`JWTAuth` middleware, ver CLAUDE.md "Auto-provisioning"). El único test existente (`users_repo_test.go`) cubre `UpsertBySupabaseID` (idempotencia); el resto de la interfaz no tiene ningún test.

- [x] **Step 1: `GetBySupabaseID` y `GetByID` — feliz y not-found**

```go
package users_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

func TestGetBySupabaseIDAndGetByID(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()

	supabaseID := "supabase-" + uuid.NewString()[:8]
	email := supabaseID + "@auth.local"
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, email)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	bySupabase, err := repo.GetBySupabaseID(ctx, supabaseID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, bySupabase.ID)
	assert.Equal(t, email, bySupabase.Email)

	byID, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, email, byID.Email)
}

func TestGetBySupabaseID_NotFound(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)

	_, err := repo.GetBySupabaseID(context.Background(), "no-existe-"+uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
```

- [x] **Step 2: `SetStatus` — feliz y comportamiento real con id inexistente**

```go
func TestSetStatus(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()

	supabaseID := "supabase-status-" + uuid.NewString()[:8]
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, supabaseID+"@auth.local")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	require.NoError(t, repo.SetStatus(ctx, created.ID, domain.UserStatusRevoked))

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.UserStatusRevoked, got.Status)
}

func TestSetStatus_IDInexistenteNoDaError(t *testing.T) {
	// SetStatus no chequea RowsAffected: un UPDATE que no matchea ninguna fila
	// no es un error, solo un no-op silencioso. Este test documenta ese
	// comportamiento real, no lo que "deberia" hacer.
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)

	err := repo.SetStatus(context.Background(), uuid.NewString(), domain.UserStatusActive)
	assert.NoError(t, err)
}
```

- [x] **Step 3: `SetPasswordChangeRequired`**

```go
func TestSetPasswordChangeRequired(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()

	supabaseID := "supabase-pwd-" + uuid.NewString()[:8]
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, supabaseID+"@auth.local")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })
	require.False(t, created.PasswordChangeRequired, "el default de UpsertBySupabaseID es false")

	require.NoError(t, repo.SetPasswordChangeRequired(ctx, created.ID, true))
	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.True(t, got.PasswordChangeRequired)

	require.NoError(t, repo.SetPasswordChangeRequired(ctx, created.ID, false))
	got, err = repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.False(t, got.PasswordChangeRequired)
}
```

- [x] **Step 4: `IsActiveMemberOfTenant` — sin membresia, activa, revoked**

```go
func TestIsActiveMemberOfTenant(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewUserRepository(pool)
	ctx := context.Background()
	tenantID := seedTenant(t, pool)

	supabaseID := "supabase-member-" + uuid.NewString()[:8]
	created, err := repo.UpsertBySupabaseID(ctx, supabaseID, supabaseID+"@auth.local")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, created.ID) })

	isMember, err := repo.IsActiveMemberOfTenant(ctx, created.ID, tenantID)
	require.NoError(t, err)
	assert.False(t, isMember, "todavia no tiene ninguna membresia")

	utrID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at) VALUES ($1, $2, $3, 'cliente_operario', 'active', NOW())`,
		utrID, created.ID, tenantID)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID) })

	isMember, err = repo.IsActiveMemberOfTenant(ctx, created.ID, tenantID)
	require.NoError(t, err)
	assert.True(t, isMember)

	_, err = pool.Exec(ctx, `UPDATE user_tenant_roles SET status = 'revoked' WHERE id = $1`, utrID)
	require.NoError(t, err)

	isMember, err = repo.IsActiveMemberOfTenant(ctx, created.ID, tenantID)
	require.NoError(t, err)
	assert.False(t, isMember, "una membresia revoked no cuenta como activa")
}
```

- [x] **Step 5: Correr los tests del paquete completo (nuevo + todos los `_test.go` existentes de `users`)**

Run: `go test ./internal/repo/pg/users/... -v`
Expected: sin `DATABASE_URL`, SKIP. Con ella, PASS.

- [x] **Step 6: Commit**

```bash
git add internal/repo/pg/users/user_repository_test.go
git commit -m "test(users): cubrir pgUserRepo completo — GetBySupabaseID/GetByID/SetStatus/SetPasswordChangeRequired/IsActiveMemberOfTenant"
```

---

## Verificación global (después de las 4 tareas)

Run: `go build ./...`
Expected: sin errores.

Run: `go test ./...`
Expected: todo PASS/SKIP limpio sin `DATABASE_URL`/`MONGO_URI`. Con `docker compose up -d db mongo` y las env vars seteadas, todo PASS.
