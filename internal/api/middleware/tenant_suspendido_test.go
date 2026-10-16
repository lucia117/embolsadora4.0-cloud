package middleware_test

// Integración de la suspensión de cliente (CLOUD-ADR-020): `tenants.is_active`
// dejó de ser un campo que se guardaba y nunca se leía. Necesita DATABASE_URL.

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

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// suspensionFixture crea un tenant con el is_active pedido y un usuario con
// membresía activa en él, con el rol indicado. Devuelve ambos ids.
func suspensionFixture(t *testing.T, pool *pgxpool.Pool, isActive, isPlatform bool, roleID string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	tenantID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain, is_active, is_platform_tenant)
		 VALUES ($1,'susp','susp',$2,$3,$4)`,
		tenantID, "susp-"+tenantID.String()[:8], isActive, isPlatform)
	require.NoError(t, err)

	userID := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO users (id, email, name, status) VALUES ($1,$2,'susp','active')`,
		userID, userID.String()+"@susp.local")
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO user_tenant_roles (id,user_id,tenant_id,role_id,status,assigned_at)
		 VALUES ($1,$2,$3,$4,'active',NOW())`,
		uuid.New(), userID, tenantID, roleID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE user_id=$1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenantID)
	})
	return tenantID, userID
}

// suspensionRouter monta la cadena real con TenantFromHeader sobre el pool,
// inyectando al usuario ya autenticado (JWTAuth queda fuera a propósito).
func suspensionRouter(pool *pgxpool.Pool, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/dashboards",
		func(c *gin.Context) {
			ctx := platform.WithDomainUser(c.Request.Context(), &domain.User{ID: userID.String()})
			ctx = platform.WithUserID(ctx, userID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		},
		apimw.TenantFromHeader(pool),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return r
}

func suspensionPool(t *testing.T) *pgxpool.Pool {
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

func doGet(r *gin.Engine, tenantID uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboards", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Es la regresión que da sentido al cambio: antes esto devolvía 200 y el
// usuario de un cliente suspendido seguía operando como si nada.
func TestTenantSuspendido_MiembroComun_Returns403(t *testing.T) {
	pool := suspensionPool(t)
	tenantID, userID := suspensionFixture(t, pool, false, false, "cliente_admin")

	w := doGet(suspensionRouter(pool, userID), tenantID)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "tenant suspended")
}

func TestTenantActivo_MiembroComun_Pasa(t *testing.T) {
	pool := suspensionPool(t)
	tenantID, userID := suspensionFixture(t, pool, true, false, "cliente_admin")

	w := doGet(suspensionRouter(pool, userID), tenantID)

	require.Equal(t, http.StatusOK, w.Code)
}

// Sin esto la suspensión sería irreversible: si el operador de plataforma
// también quedara afuera, nadie podría volver a poner is_active en true.
func TestTenantSuspendido_OperadorDePlataforma_Pasa(t *testing.T) {
	pool := suspensionPool(t)
	ctx := context.Background()

	// Tenant plataforma con un admin: es el operador.
	platformTenantID, operatorID := suspensionFixture(t, pool, true, true, "admin")
	require.NotEqual(t, uuid.Nil, platformTenantID)

	// Cliente suspendido, sin membresía del operador.
	suspendedID := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, company_name, subdomain, is_active)
		 VALUES ($1,'susp-target','susp-target',$2,FALSE)`,
		suspendedID, "tgt-"+suspendedID.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, suspendedID)
	})

	w := doGet(suspensionRouter(pool, operatorID), suspendedID)

	require.Equal(t, http.StatusOK, w.Code,
		"un operador de plataforma tiene que poder actuar sobre un cliente suspendido para reactivarlo; body=%s", w.Body.String())
}

// El tenant plataforma no se bloquea a sí mismo aunque alguien ponga su
// is_active en false: sería un lock-out total del sistema.
func TestTenantPlataformaSuspendido_NoSeBloquea(t *testing.T) {
	pool := suspensionPool(t)
	tenantID, userID := suspensionFixture(t, pool, false, true, "admin")

	w := doGet(suspensionRouter(pool, userID), tenantID)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
}
