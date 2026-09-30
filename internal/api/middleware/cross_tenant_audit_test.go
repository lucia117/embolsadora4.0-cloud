package middleware

// Auditoría de accesos cross-tenant (issue #102): cuando un operador de
// plataforma entra a un tenant del que no es miembro, el acceso concedido
// tiene que dejar rastro (log + métrica), no solo las denegaciones. Aplica a
// los dos middlewares que resuelven el tenant: por path
// (ResolveTenantAndCheckMembership) y por header (TenantFromHeader).
//
// Tests de integración: necesitan DATABASE_URL (se saltean si no está).

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

const crossTenantGrantMsg = "cross-tenant access granted"

// observeLogs reemplaza el logger del paquete por uno que captura entradas y
// lo restaura al terminar el test.
func observeLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	prev := Log
	Log = zap.New(core)
	t.Cleanup(func() { Log = prev })
	return logs
}

func grantsFor(role string) float64 {
	return testutil.ToFloat64(telemetry.AuthCrossTenantGrantsTotal.WithLabelValues(role))
}

func newHeaderTestRouter(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := platform.WithDomainUser(c.Request.Context(), &domain.User{ID: userID.String()})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/api/v1/users", TenantFromHeader(pool), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": security.RoleFromContext(c.Request.Context())})
	})
	return r
}

func doHeaderGet(r *gin.Engine, tenantID uuid.UUID) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func requireOneGrantLog(t *testing.T, logs *observer.ObservedLogs, userID, tenantID uuid.UUID, role, path string) {
	t.Helper()
	entries := logs.FilterMessage(crossTenantGrantMsg).All()
	require.Len(t, entries, 1, "la concesión cross-tenant tiene que loguearse exactamente una vez")
	fields := entries[0].ContextMap()
	require.Equal(t, userID.String(), fields["user_id"])
	require.Equal(t, role, fields["role"])
	require.Equal(t, tenantID.String(), fields["target_tenant_id"])
	require.Equal(t, http.MethodGet, fields["method"])
	require.Equal(t, path, fields["path"])
}

func TestResolveTenant_ConcesionCrossTenant_SeAudita(t *testing.T) {
	pool := poolOrSkip(t)
	logs := observeLogs(t)
	u := seedUser(t, pool)
	seedMembership(t, pool, u, platformTenantUUID, "tenant_manager")
	tenantID, sub := seedTenant(t, pool)
	before := grantsFor("tenant_manager")

	w := doGet(newMiddlewareTestRouter(t, pool, u), sub)

	require.Equal(t, http.StatusOK, w.Code)
	requireOneGrantLog(t, logs, u, tenantID, "tenant_manager", "/api/v1/tenants/"+sub+"/edge-devices")
	require.Equal(t, before+1, grantsFor("tenant_manager"))
}

func TestResolveTenant_MiembroDirecto_NoSeAuditaComoCrossTenant(t *testing.T) {
	pool := poolOrSkip(t)
	logs := observeLogs(t)
	u := seedUser(t, pool)
	tenantID, sub := seedTenant(t, pool)
	seedMembership(t, pool, u, tenantID, "cliente_admin")

	w := doGet(newMiddlewareTestRouter(t, pool, u), sub)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, logs.FilterMessage(crossTenantGrantMsg).All())
}

func TestTenantFromHeader_ConcesionCrossTenant_SeAudita(t *testing.T) {
	pool := poolOrSkip(t)
	logs := observeLogs(t)
	u := seedUser(t, pool)
	seedMembership(t, pool, u, platformTenantUUID, "tenant_manager")
	tenantID, _ := seedTenant(t, pool)
	before := grantsFor("tenant_manager")

	w := doHeaderGet(newHeaderTestRouter(t, pool, u), tenantID)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"role":"tenant_manager"}`, w.Body.String())
	requireOneGrantLog(t, logs, u, tenantID, "tenant_manager", "/api/v1/users")
	require.Equal(t, before+1, grantsFor("tenant_manager"))
}

func TestTenantFromHeader_MiembroDirecto_NoSeAuditaComoCrossTenant(t *testing.T) {
	pool := poolOrSkip(t)
	logs := observeLogs(t)
	u := seedUser(t, pool)
	tenantID, _ := seedTenant(t, pool)
	seedMembership(t, pool, u, tenantID, "cliente_admin")

	w := doHeaderGet(newHeaderTestRouter(t, pool, u), tenantID)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, logs.FilterMessage(crossTenantGrantMsg).All())
}

// El admin del tenant plataforma que entra a un tenant cliente se audita con
// su rol efectivo (platform_admin), no con el rol crudo de la membresía.
func TestTenantFromHeader_AdminDePlataforma_SeAuditaComoPlatformAdmin(t *testing.T) {
	pool := poolOrSkip(t)
	logs := observeLogs(t)
	u := seedUser(t, pool)
	seedMembership(t, pool, u, platformTenantUUID, "admin")
	tenantID, _ := seedTenant(t, pool)

	w := doHeaderGet(newHeaderTestRouter(t, pool, u), tenantID)

	require.Equal(t, http.StatusOK, w.Code)
	requireOneGrantLog(t, logs, u, tenantID, "platform_admin", "/api/v1/users")
}
