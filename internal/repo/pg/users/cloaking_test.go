package users_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

const platformTenant = "11b36b85-033d-4bb3-9e31-4c92161887c0"

// seedMemberWithRole crea un usuario con membresía en el tenant plataforma y el
// rol/estado dados. Devuelve el user id y limpia todo al terminar.
func seedMemberWithRole(t *testing.T, pool *pgxpool.Pool, roleID, status string) string {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New().String()
	utrID := uuid.New().String()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Cloak Test', 'active')`,
		userID, userID+"@cloak.local")
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		utrID, userID, platformTenant, roleID, status)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

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

func TestListByTenantOcultaMiembrosConRolGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	superID := seedMemberWithRole(t, pool, "super_admin", "active")
	adminID := seedMemberWithRole(t, pool, "admin", "active")

	list, total, err := repo.ListByTenant(ctx, platformTenant, 100, 0, false)
	require.NoError(t, err)

	var ids []string
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	require.NotContains(t, ids, superID, "el superadmin debe ser invisible")
	require.Contains(t, ids, adminID, "los miembros con rol tenant-scoped siguen visibles")

	// El COUNT debe aplicar el mismo filtro que el SELECT: si no, la UI mostraría
	// un total mayor que las filas listadas y una última página vacía.
	// Se compara contra un COUNT filtrado hecho a mano, no contra len(list):
	// len(list) está topeado por el limit y coincidiría por casualidad solo
	// mientras el tenant tenga menos usuarios que el límite de la página.
	var expected int64
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM users u
		LEFT JOIN user_tenant_roles utr
			ON utr.user_id = u.id AND utr.tenant_id = $1 AND utr.status = 'active'
		LEFT JOIN roles r ON r.id = utr.role_id
		WHERE u.deleted_at IS NULL
		  AND (u.tenant_id = $1 OR utr.id IS NOT NULL)
		  AND COALESCE(r.is_global, FALSE) = FALSE`, platformTenant).Scan(&expected)
	require.NoError(t, err)
	require.Equal(t, expected, total, "el COUNT debe aplicar el mismo filtro que el SELECT")
}

func TestListByTenantMuestraTodoAlSuperadmin(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	superID := seedMemberWithRole(t, pool, "super_admin", "active")

	list, _, err := repo.ListByTenant(ctx, platformTenant, 100, 0, true)
	require.NoError(t, err)

	var ids []string
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	require.Contains(t, ids, superID)
}

func TestListPendingByTenantOcultaInvitadosConRolGlobal(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	pendingSuper := seedMemberWithRole(t, pool, "super_admin", "pending")
	pendingAdmin := seedMemberWithRole(t, pool, "admin", "pending")

	list, err := repo.ListPendingByTenant(ctx, platformTenant, false)
	require.NoError(t, err)

	var ids []string
	for _, u := range list {
		ids = append(ids, u.ID)
	}
	require.NotContains(t, ids, pendingSuper, "una membresía pendiente a super_admin delata igual")
	require.Contains(t, ids, pendingAdmin)
}

func TestGetByIDDevuelveNotFoundParaUsuarioOculto(t *testing.T) {
	pool := poolOrSkip(t)
	repo := usersRepo.NewPostgresRepository(pool)
	ctx := context.Background()

	superID := seedMemberWithRole(t, pool, "super_admin", "active")

	_, err := repo.GetByID(ctx, platformTenant, superID, false)
	require.Error(t, err, "404, no 403: un 403 confirmaría que el usuario existe")

	u, err := repo.GetByID(ctx, platformTenant, superID, true)
	require.NoError(t, err)
	require.Equal(t, superID, u.ID)
}
