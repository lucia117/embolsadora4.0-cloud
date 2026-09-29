package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/security"
)

func newRBACCheckRouter(rc *security.RoleContext) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if rc != nil {
			ctx := security.WithRoleContext(c.Request.Context(), *rc)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	})
	r.GET("/protected", RBACCheck("perm_users_manage"), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func doRBACCheckRequest(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRBACCheck_MissingPermission_Returns403(t *testing.T) {
	r := newRBACCheckRouter(&security.RoleContext{Name: "cliente_admin", Permissions: []string{"perm_users_view"}})
	w := doRBACCheckRequest(r)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRBACCheck_HasPermission_CallsNext(t *testing.T) {
	r := newRBACCheckRouter(&security.RoleContext{Name: "cliente_admin", Permissions: []string{"perm_users_manage"}})
	w := doRBACCheckRequest(r)
	require.Equal(t, http.StatusOK, w.Code)
}

// TestRBACCheck_NoRoleContext_FailsClosed cubre el caso en que RBACCheck
// corre sin haber pasado antes por TenantFromHeader (ningún RoleContext en
// contexto): security.Can devuelve ErrForbidden por rc.Name == "", nunca un
// permiso implícito.
func TestRBACCheck_NoRoleContext_FailsClosed(t *testing.T) {
	r := newRBACCheckRouter(nil)
	w := doRBACCheckRequest(r)
	require.Equal(t, http.StatusForbidden, w.Code)
}
