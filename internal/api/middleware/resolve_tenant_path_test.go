package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// Fixtures en la DB de DATABASE_URL (embolsadora_test), creados fuera de este
// archivo salvo adminPlatformUserID (creado en TestMain más abajo):
const (
	platformUserID      = "a0000000-0000-4000-8000-000000000001" // tenant_manager, solo en mrgsrl
	clientAdminUserID   = "90a21976-ccb2-4b1e-86ce-4d2c1b52bfa5" // cliente_admin, solo en cordoba
	otherClientUserID   = "a0000000-0000-4000-8000-000000000002" // cliente_admin, solo en salta
	adminPlatformUserID = "a0000000-0000-4000-8000-000000000003" // admin, membresía directa en mrgsrl
	platformTenantID    = "11b36b85-033d-4bb3-9e31-4c92161887c0"
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn != "" {
		pool, err := pgxpool.New(context.Background(), dsn)
		if err == nil {
			ctx := context.Background()
			pool.Exec(ctx,
				`INSERT INTO users (id, email, name, first_name, last_name, status, role, created_at, updated_at)
				 VALUES ($1, 'federicoadegiovanni+mrg-admin@gmail.com', 'MRG Admin', 'MRG', 'Admin', 'active', 'user', now(), now())
				 ON CONFLICT (id) DO NOTHING`, adminPlatformUserID)
			pool.Exec(ctx,
				`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, created_at, updated_at)
				 VALUES ('b0000000-0000-4000-8000-000000000003', $1, $2, 'admin', 'active', now(), now())
				 ON CONFLICT (id) DO NOTHING`, adminPlatformUserID, platformTenantID)
			pool.Close()
		}
	}
	os.Exit(m.Run())
}

func newMiddlewareTestRouter(t *testing.T, pool *pgxpool.Pool, userID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		ctx := platform.WithDomainUser(c.Request.Context(), &domain.User{ID: userID})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	grp := r.Group("/api/v1/tenants/:tenantId", ResolveTenantAndCheckMembership(pool))
	grp.GET("/edge-devices", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
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

func doGet(r *gin.Engine, tenantSlug string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+tenantSlug+"/edge-devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestResolveTenant_SubdomainInexistente_Da404(t *testing.T) {
	pool := poolOrSkip(t)
	r := newMiddlewareTestRouter(t, pool, platformUserID)
	w := doGet(r, "no-existe-este-tenant")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "tenant not found")
}

func TestResolveTenant_MiembroDirecto_Da200(t *testing.T) {
	pool := poolOrSkip(t)
	r := newMiddlewareTestRouter(t, pool, clientAdminUserID)
	w := doGet(r, "cordoba")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestResolveTenant_OperadorPlataformaSinMembresia_Da200(t *testing.T) {
	pool := poolOrSkip(t)
	// platformUserID (tenant_manager) NO tiene fila en user_tenant_roles para 'cordoba'
	r := newMiddlewareTestRouter(t, pool, platformUserID)
	w := doGet(r, "cordoba")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestResolveTenant_NoMiembroNoOperador_Da403(t *testing.T) {
	pool := poolOrSkip(t)
	// otherClientUserID es cliente_admin de 'salta', no de 'cordoba' ni plataforma
	r := newMiddlewareTestRouter(t, pool, otherClientUserID)
	w := doGet(r, "cordoba")
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "tenant access denied")
}

// Un 'admin' con membresía DIRECTA en el tenant plataforma: el branch de
// membresía directa aplica EffectiveRole -> platform_admin. Esto es nuevo para
// el middleware path-based (lo alinea con TenantFromHeader). El test bloquea la
// escalación como intencional: platform_admin resuelve y el request pasa (200).
func TestResolveTenant_AdminDelTenantPlataforma_ResuelvePlatformAdmin(t *testing.T) {
	pool := poolOrSkip(t)
	r := newMiddlewareTestRouter(t, pool, adminPlatformUserID)
	w := doGet(r, "mrgsrl")
	require.Equal(t, http.StatusOK, w.Code)
}
