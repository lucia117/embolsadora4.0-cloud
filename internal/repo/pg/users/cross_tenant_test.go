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
