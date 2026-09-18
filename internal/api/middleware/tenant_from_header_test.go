package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func TestLoadRolePermissions(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	perms, isGlobal, err := loadRolePermissions(context.Background(), pool, "cliente_admin")
	require.NoError(t, err)
	require.False(t, isGlobal)
	require.Contains(t, perms, "perm_users_manage")
	require.NotContains(t, perms, "perm_users")
}

func TestLoadRolePermissionsRolInexistente(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	defer pool.Close()

	_, _, err = loadRolePermissions(context.Background(), pool, "rol_que_no_existe")
	require.Error(t, err)
}

// newTenantFromHeaderRouter monta TenantFromHeader con db=nil: solo es seguro
// para las ramas que abortan ANTES de tocar la DB (exención, header
// ausente/inválido, falta de domain.User) -- cualquier caso que llegue al
// db.QueryRow real necesita poolOrSkip, como los tests de arriba.
func newTenantFromHeaderRouter(user *domain.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if user != nil {
			ctx := platform.WithDomainUser(c.Request.Context(), user)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})
	r.GET("/api/v1/dashboards", TenantFromHeader(nil), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/me", TenantFromHeader(nil), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestTenantFromHeader_ExemptRoute_SkipsCheckEvenWithoutHeader(t *testing.T) {
	r := newTenantFromHeaderRouter(&domain.User{ID: "u1"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestTenantFromHeader_MissingHeader_Returns400(t *testing.T) {
	r := newTenantFromHeaderRouter(&domain.User{ID: "u1"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "missing X-Tenant-ID header")
}

func TestTenantFromHeader_InvalidUUIDHeader_Returns400(t *testing.T) {
	r := newTenantFromHeaderRouter(&domain.User{ID: "u1"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "must be a valid UUID")
}

// TestTenantFromHeader_NoDomainUserInContext_Returns401 cubre el orden real
// del middleware: el chequeo de domain.User corre ANTES del db.QueryRow, así
// que este caso es alcanzable con db=nil sin necesidad de Postgres real.
func TestTenantFromHeader_NoDomainUserInContext_Returns401(t *testing.T) {
	r := newTenantFromHeaderRouter(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards", nil)
	req.Header.Set("X-Tenant-ID", uuid.New().String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
