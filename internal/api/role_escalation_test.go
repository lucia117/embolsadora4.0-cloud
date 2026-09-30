package api_test

// Regresión del issue #107. Antes del fix, un cliente_admin (perm_users_manage,
// sin perm_tenants_manage) podía: crear un rol custom con perm_tenants_manage
// (201), pasarse a ese rol con PUT /user-roles/{su asignación} (200) y borrar
// su propio tenant (200). Cada paso tiene que cortarse ahora.
// Integración: necesita DATABASE_URL.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/api"
	rolesHandler "github.com/tu-org/embolsadora-api/internal/api/handler/roles"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	rolesApp "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	permissionsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/permissions"
	rolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/roles"
	tenantsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/tenants"
	userRolesRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/user_roles"
	usersRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/users"
)

func TestRolCustom_NoPermiteEscalarPrivilegios(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Tenant cliente y un cliente_admin en él.
	tenantID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO tenants (id, name, company_name, subdomain) VALUES ($1,'esc','esc',$2)`,
		tenantID, "esc-"+tenantID.String()[:8])
	require.NoError(t, err)
	userID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, name, status) VALUES ($1,$2,'esc','active')`,
		userID, userID.String()+"@esc.local")
	require.NoError(t, err)
	utrID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO user_tenant_roles (id,user_id,tenant_id,role_id,status,assigned_at) VALUES ($1,$2,$3,'cliente_admin','active',NOW())`,
		utrID, userID, tenantID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM user_tenant_roles WHERE user_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM roles WHERE tenant_id=$1`, tenantID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenantID)
	})

	// Router con la cadena real de /api/v1 salvo JWTAuth, reemplazado por un
	// middleware que inyecta al usuario autenticado.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1", func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithDomainUser(c.Request.Context(), &domain.User{ID: userID.String()}))
		c.Request = c.Request.WithContext(platform.WithUserID(c.Request.Context(), userID))
		c.Next()
	}, apimw.TenantFromHeader(pool), apimw.PasswordChangeGuard())
	rRepo := rolesRepo.NewPostgresRepository(pool)
	api.RegisterAdminRoutes(v1, api.Deps{
		TenantRepo:   tenantsRepo.NewTenantRepository(pool),
		UserRoleRepo: userRolesRepo.NewUserRoleRepository(pool),
		Logger:       zap.NewNop(),
		UserRepo:     usersRepo.NewPostgresRepository(pool),
		RoleRepo:     rRepo,
	}, api.Config{})
	rolesHandler.RegisterRoutes(v1, v1.Group("", apimw.RBACCheck("perm_users_manage")), rolesApp.NewService(rRepo, permissionsRepo.NewPostgresRepository(pool), zap.NewNop()))

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Tenant-ID", tenantID.String())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		t.Logf("%s %s -> %d %s", method, path, w.Code, w.Body.String())
		return w
	}

	// Control: sin permiso de gestión de tenants, borrar el tenant se rechaza.
	require.Equal(t, http.StatusForbidden, do(http.MethodDelete, "/api/v1/tenants/"+tenantID.String(), nil).Code)

	// Paso 2: crear un rol custom con un permiso que el creador no tiene.
	w := do(http.MethodPost, "/api/v1/roles", map[string]any{
		"name": "escalada", "permissions": []string{"perm_tenants_manage", "perm_users_manage"},
	})
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "PERMISSION_NOT_HELD")
	require.Contains(t, w.Body.String(), "perm_tenants_manage")

	// Con permisos que sí tiene, el rol se crea (la regla no bloquea el uso normal).
	w = do(http.MethodPost, "/api/v1/roles", map[string]any{
		"name": "lector", "permissions": []string{"perm_users_view"},
	})
	require.Equal(t, http.StatusCreated, w.Code)
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// Paso 3: cambiarse a sí mismo de rol se rechaza, aunque el rol sea legítimo.
	w = do(http.MethodPut, "/api/v1/user-roles/"+utrID.String(), map[string]any{"roleId": created.ID})
	require.Equal(t, http.StatusForbidden, w.Code)

	// Paso 4: sigue sin poder borrar su tenant.
	require.Equal(t, http.StatusForbidden, do(http.MethodDelete, "/api/v1/tenants/"+tenantID.String(), nil).Code)
}
