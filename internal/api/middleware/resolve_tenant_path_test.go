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
	"github.com/tu-org/embolsadora-api/internal/security"
)

// platformTenantUUID es el tenant plataforma de MRG (mismo id que usa el resto
// de la suite de integración, p. ej. usecases/user_roles/assign_user_role).
var platformTenantUUID = uuid.MustParse("11b36b85-033d-4bb3-9e31-4c92161887c0")

// poolOrSkip abre un pool contra DATABASE_URL o salta el test si no está seteada.
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

// seedUser inserta un usuario y programa su limpieza (junto con cualquier
// user_tenant_roles que se le haya asignado).
func seedUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM user_tenant_roles WHERE user_id = $1`, id)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, email, name, status) VALUES ($1, $2, 'Resolve Tenant Test', 'active')`,
		id, id.String()+"@resolve-tenant.local")
	require.NoError(t, err)
	return id
}

// seedTenant inserta un tenant cliente (no plataforma) con un subdomain único y
// programa su limpieza.
func seedTenant(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	subdomain := "cba-" + id.String()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, id)
	})
	_, err := pool.Exec(context.Background(), `
		INSERT INTO tenants (id, name, company_name, subdomain)
		VALUES ($1, 'Cordoba test', 'Cordoba test', $2)
	`, id, subdomain)
	require.NoError(t, err)
	return id, subdomain
}

// seedMembership asigna un rol activo al usuario en el tenant dado. La limpieza
// la cubre seedUser (DELETE ... WHERE user_id).
func seedMembership(t *testing.T, pool *pgxpool.Pool, userID, tenantID uuid.UUID, roleID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO user_tenant_roles (id, user_id, tenant_id, role_id, status, assigned_at)
		 VALUES ($1, $2, $3, $4, 'active', NOW())`,
		uuid.New(), userID, tenantID, roleID)
	require.NoError(t, err)
}

// platformSubdomain consulta el subdomain del tenant plataforma (no lo hardcodea:
// en envs sembrados es "mrgsrl" pero eso es dato, no contrato).
func platformSubdomain(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var sub string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT subdomain FROM tenants WHERE id = $1`, platformTenantUUID).Scan(&sub))
	return sub
}

// newMiddlewareTestRouter monta el middleware real detrás de un handler que
// devuelve el rol efectivo resuelto en contexto, para poder assertear QUÉ rol
// resolvió el middleware (no solo que "algún rol" pasó).
func newMiddlewareTestRouter(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		ctx := platform.WithDomainUser(c.Request.Context(), &domain.User{ID: userID.String()})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	grp := r.Group("/api/v1/tenants/:tenantId", ResolveTenantAndCheckMembership(pool))
	grp.GET("/edge-devices", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": security.RoleFromContext(c.Request.Context())})
	})
	return r
}

func doGet(r *gin.Engine, tenantSlug string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+tenantSlug+"/edge-devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestResolveTenant_SubdomainInexistente_Da404(t *testing.T) {
	pool := poolOrSkip(t)
	u := seedUser(t, pool)
	r := newMiddlewareTestRouter(t, pool, u)
	w := doGet(r, "nonexistent-"+uuid.NewString())
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "tenant not found")
}

func TestResolveTenant_MiembroDirecto_Da200(t *testing.T) {
	pool := poolOrSkip(t)
	u := seedUser(t, pool)
	tid, sub := seedTenant(t, pool)
	seedMembership(t, pool, u, tid, "cliente_admin")

	r := newMiddlewareTestRouter(t, pool, u)
	w := doGet(r, sub)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"role":"cliente_admin"}`, w.Body.String())
}

func TestResolveTenant_OperadorPlataformaSinMembresia_Da200(t *testing.T) {
	pool := poolOrSkip(t)
	u := seedUser(t, pool)
	// Membresía SOLO en el tenant plataforma (tenant_manager es is_global; el
	// trigger trg_enforce_platform_role_tenant lo permite ahí).
	seedMembership(t, pool, u, platformTenantUUID, "tenant_manager")
	// Tenant cliente fresco, SIN membresía directa del usuario.
	_, sub := seedTenant(t, pool)

	r := newMiddlewareTestRouter(t, pool, u)
	w := doGet(r, sub)

	require.Equal(t, http.StatusOK, w.Code)
	// El fallback resolvió un rol global -> prueba que el branch de fallback
	// corrió (un miembro directo habría resuelto su propio role_id).
	require.JSONEq(t, `{"role":"tenant_manager"}`, w.Body.String())
}

func TestResolveTenant_NoMiembroNoOperador_Da403(t *testing.T) {
	pool := poolOrSkip(t)
	u := seedUser(t, pool)
	t1, s1 := seedTenant(t, pool)
	seedMembership(t, pool, u, t1, "cliente_admin")
	_, s2 := seedTenant(t, pool) // tenant cliente DISTINTO, sin membresía

	r := newMiddlewareTestRouter(t, pool, u)

	// Deny sobre el tenant ajeno.
	wDeny := doGet(r, s2)
	require.Equal(t, http.StatusForbidden, wDeny.Code)
	require.Contains(t, wDeny.Body.String(), "tenant access denied")

	// Control positivo: el MISMO usuario sí entra a su propio tenant -> el deny
	// es por scope de membresía, no porque el usuario no exista.
	wOK := doGet(r, s1)
	require.Equal(t, http.StatusOK, wOK.Code)
	require.JSONEq(t, `{"role":"cliente_admin"}`, wOK.Body.String())
}

// TestResolveTenant_AdminDelTenantPlataforma_ResuelvePlatformAdmin: un 'admin'
// con membresía directa en el tenant plataforma actúa como platform_admin
// (EffectiveRole). Esto es lo que el path-based middleware alinea con
// TenantFromHeader; el test bloquea la escalación como intencional.
func TestResolveTenant_AdminDelTenantPlataforma_ResuelvePlatformAdmin(t *testing.T) {
	pool := poolOrSkip(t)
	u := seedUser(t, pool)
	seedMembership(t, pool, u, platformTenantUUID, "admin")
	sub := platformSubdomain(t, pool)

	r := newMiddlewareTestRouter(t, pool, u)
	w := doGet(r, sub)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"role":"platform_admin"}`, w.Body.String())
}
