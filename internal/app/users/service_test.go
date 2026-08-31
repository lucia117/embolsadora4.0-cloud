package users_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appUsers "github.com/tu-org/embolsadora-api/internal/app/users"
	"github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

// Regresión del Crítico 1 por la vía POST /api/v1/users, que era la más directa:
// CreateWithRole no validaba nada (la FK acepta 'super_admin' porque el rol existe
// de verdad), así que un admin de MRG creaba un usuario suyo con role='super_admin'
// y después con POST /users/{id}/force-password-change se mandaba a su propia casilla
// el mail de reset de esa cuenta.

const platformTenant = "11b36b85-033d-4bb3-9e31-4c92161887c0"

func poolOrSkip(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func newService(pool *pgxpool.Pool) *appUsers.Service {
	return appUsers.NewService(
		usersRepo.NewPostgresRepository(pool),
		userRolesRepo.NewUserRoleRepository(pool),
		rolesRepo.NewPostgresRepository(pool),
		zap.NewNop(),
	)
}

// seedCaller crea el usuario que figura como assigned_by (tiene FK).
func seedCaller(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	id := uuid.New().String()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE assigned_by = $1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Caller', 'active')`,
		id, id+"@caller.local")
	require.NoError(t, err)
	return id
}

func cleanupByEmail(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM user_tenant_roles WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})
}

// TestCreateUserConRolGlobalFallaParaPlatformAdmin es el escenario del informe.
func TestCreateUserConRolGlobalFallaParaPlatformAdmin(t *testing.T) {
	pool := poolOrSkip(t)
	svc := newService(pool)
	ctx := context.Background()

	caller := seedCaller(t, pool)
	email := uuid.New().String() + "@escalada.local"
	cleanupByEmail(t, pool, email)

	user, err := svc.CreateUser(ctx, platformTenant, &domainUsers.CreateUserCommand{
		TenantID:   platformTenant,
		FirstName:  "Atacante",
		LastName:   "MRG",
		Email:      email,
		Role:       "super_admin",
		AssignedBy: caller,
	}, false) // includeGlobal=false: lo que ve un platform_admin

	require.Nil(t, user)
	require.ErrorIs(t, err, domain.ErrInvalidRoleID)

	// Ni el usuario ni la UTR llegaron a existir: la validación corre antes de la
	// transacción de CreateWithRole.
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email = $1`, email).Scan(&count))
	require.Zero(t, count, "no debe haberse creado el usuario")
}

// TestCreateUserConRolGlobalFuncionaParaSuperAdmin: control positivo.
func TestCreateUserConRolGlobalFuncionaParaSuperAdmin(t *testing.T) {
	pool := poolOrSkip(t)
	svc := newService(pool)
	ctx := context.Background()

	caller := seedCaller(t, pool)
	email := uuid.New().String() + "@legitimo.local"
	cleanupByEmail(t, pool, email)

	user, err := svc.CreateUser(ctx, platformTenant, &domainUsers.CreateUserCommand{
		TenantID:   platformTenant,
		FirstName:  "Nuevo",
		LastName:   "Superadmin",
		Email:      email,
		Role:       "super_admin",
		AssignedBy: caller,
	}, true)

	require.NoError(t, err)
	require.NotNil(t, user)
}

// TestCreateUserConRolNormalSigueFuncionando: el flujo cotidiano no se rompió.
func TestCreateUserConRolNormalSigueFuncionando(t *testing.T) {
	pool := poolOrSkip(t)
	svc := newService(pool)
	ctx := context.Background()

	caller := seedCaller(t, pool)
	email := uuid.New().String() + "@normal.local"
	cleanupByEmail(t, pool, email)

	user, err := svc.CreateUser(ctx, platformTenant, &domainUsers.CreateUserCommand{
		TenantID:   platformTenant,
		FirstName:  "Usuario",
		LastName:   "Normal",
		Email:      email,
		Role:       "operario",
		AssignedBy: caller,
	}, false)

	require.NoError(t, err)
	require.NotNil(t, user)
}

// TestUpdateUserNoPuedePintarRolGlobal: users.role es la columna legada y no otorga
// permisos (la membresía real vive en user_tenant_roles, que es lo que lee /me), pero
// los listados la muestran vía COALESCE(utr.role_id, u.role). Dejarla libre permitía
// escribir 'super_admin' ahí, con el efecto secundario de sacar al usuario del filtro
// de cloaking de los listados (el minor diferido en Task 5).
func TestUpdateUserNoPuedePintarRolGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	svc := newService(pool)
	ctx := context.Background()

	caller := seedCaller(t, pool)
	email := uuid.New().String() + "@pintar.local"
	cleanupByEmail(t, pool, email)

	user, err := svc.CreateUser(ctx, platformTenant, &domainUsers.CreateUserCommand{
		TenantID: platformTenant, FirstName: "Usuario", LastName: "Normal",
		Email: email, Role: "operario", AssignedBy: caller,
	}, false)
	require.NoError(t, err)

	rolGlobal := "super_admin"
	updated, err := svc.UpdateUser(ctx, platformTenant, user.ID, false, false, &domainUsers.UpdateUserCommand{
		TenantID: platformTenant, UserID: user.ID, Role: &rolGlobal,
	})
	require.Nil(t, updated)
	require.ErrorIs(t, err, domain.ErrInvalidRoleID)

	var role string
	require.NoError(t, pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, user.ID).Scan(&role))
	require.Equal(t, "operario", role, "users.role no debe haber cambiado")
}
