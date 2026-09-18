package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// RequireRole y SetUserRoleFromJWT son placeholders sin implementar (ver los
// TODOs en rbac.go: "Temporarily disabled for testing") y no están wireados
// en ningún router real -- estos tests documentan el comportamiento actual
// (siempre dejan pasar) para que una implementación futura rompa el test en
// vez de cambiar de comportamiento en silencio.
func TestRequireRole_UnimplementedPlaceholder_AlwaysCallsNext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin-only", RequireRole("super_admin"), func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "RequireRole no valida ningún rol todavía")
}

func TestSetUserRoleFromJWT_UnimplementedPlaceholder_NeverSetsUserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/probe", SetUserRoleFromJWT(), func(c *gin.Context) {
		_, exists := c.Get(UserRole)
		require.False(t, exists, "SetUserRoleFromJWT no setea nada todavía")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
