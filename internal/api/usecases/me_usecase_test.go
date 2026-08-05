package usecases_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/api/usecases"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// seedMember crea un usuario con membresía activa en el tenant dado y devuelve su ID.
// Limpia todo con t.Cleanup.
func seedMember(t *testing.T, pool *pgxpool.Pool, tenantID, roleID string) string {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New().String()
	utrID := uuid.New().String()

	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Test User', 'active')`,
		userID, userID+"@test.local")
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, $4, 'active', NOW())`,
		utrID, userID, tenantID, roleID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE id = $1`, utrID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

// platformTenantID es el tenant MRG sembrado por la migración 000002.
const platformTenantID = "11b36b85-033d-4bb3-9e31-4c92161887c0"

func TestGetMeAdminDePlataformaEsPlatformAdmin(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	// NOTA: pool.Close() se registra vía t.Cleanup (no `defer`) y ANTES que
	// seedMember para que, por el orden LIFO de t.Cleanup, las limpiezas de
	// filas corran primero y el pool se cierre al final. Con `defer
	// pool.Close()` el pool se cierra al retornar la función del test, que
	// ocurre antes de que corran los t.Cleanup registrados — todo DELETE de
	// limpieza falla con "closed pool" y deja basura en la DB. Confirmado
	// empíricamente contra la DB local: t.Logf en el cleanup mostró
	// "closed pool" en las 3 corridas antes de este fix.
	t.Cleanup(func() { pool.Close() })

	userID := seedMember(t, pool, platformTenantID, "admin")

	uc := usecases.NewMeUsecase(pool)
	ctx = platform.WithDomainUser(ctx, &domain.User{ID: userID, Email: userID + "@test.local"})

	resp, err := uc.GetMe(ctx)
	require.NoError(t, err)

	require.NotNil(t, resp.Role)
	require.Equal(t, "platform_admin", resp.Role.ID, "un admin del tenant plataforma actúa como platform_admin")
	require.NotNil(t, resp.Tenant)
	require.True(t, resp.Tenant.IsPlatform, "el tenant MRG debe venir marcado como plataforma")
	require.True(t, resp.Capabilities.CanCrossTenant, "platform_admin puede operar cross-tenant")
	require.Contains(t, resp.Permissions, "perm_tenants", "los permisos deben venir en vocabulario perm_*")
	require.Contains(t, resp.Permissions, "perm_logs_view")
	require.NotContains(t, resp.Permissions, "users:read", "el vocabulario resource:action no se expone")
	require.NotContains(t, resp.Permissions, "perm_all_tenants", "perm_all_tenants ya no existe")
}

func TestGetMeAdminDeTenantClienteNoAsciende(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	// Ver nota en TestGetMeAdminDePlataformaEsPlatformAdmin sobre por qué
	// pool.Close() va en t.Cleanup y no en `defer`.
	t.Cleanup(func() { pool.Close() })

	// Tenant cliente propio del test, para no depender de datos sembrados.
	clientTenantID := uuid.New().String()
	_, err = pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain, is_active, is_platform_tenant)
		 VALUES ($1, 'Test Client', 'Test Client SA', $2, TRUE, FALSE)`,
		clientTenantID, "test-"+clientTenantID[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, clientTenantID)
	})

	userID := seedMember(t, pool, clientTenantID, "admin")

	uc := usecases.NewMeUsecase(pool)
	ctx = platform.WithDomainUser(ctx, &domain.User{ID: userID, Email: userID + "@test.local"})

	resp, err := uc.GetMe(ctx)
	require.NoError(t, err)

	require.Equal(t, "admin", resp.Role.ID, "un admin de tenant cliente no asciende")
	require.False(t, resp.Tenant.IsPlatform)
	require.False(t, resp.Capabilities.CanCrossTenant, "un admin de tenant cliente no opera cross-tenant")
}

func TestGetMeSuperAdminConservaSuRol(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	// Ver nota en TestGetMeAdminDePlataformaEsPlatformAdmin sobre por qué
	// pool.Close() va en t.Cleanup y no en `defer`.
	t.Cleanup(func() { pool.Close() })

	userID := seedMember(t, pool, platformTenantID, "super_admin")

	uc := usecases.NewMeUsecase(pool)
	ctx = platform.WithDomainUser(ctx, &domain.User{ID: userID, Email: userID + "@test.local"})

	resp, err := uc.GetMe(ctx)
	require.NoError(t, err)

	require.Equal(t, "super_admin", resp.Role.ID)
	require.True(t, resp.Capabilities.CanCrossTenant)
	require.Contains(t, resp.Permissions, "perm_users")
}
