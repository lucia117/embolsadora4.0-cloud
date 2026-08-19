package edge_devices_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices"
	appEdgeDevices "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// newTestRouterWithRole monta las rutas reales de edge-devices detrás de un
// middleware que inyecta un RoleContext fijo, para poder probar el gate de
// RBAC sin depender de JWT/DB reales. El service se construye con
// repo/client nil a propósito: si una request pasa el gate de permisos,
// el handler puede fallar después (nil pointer) — gin.Recovery() lo
// convierte en 500 en vez de crashear el test, y un 500 sigue probando que
// NO fue un 403 (que es lo único que este test necesita confirmar).
func newTestRouterWithRole(t *testing.T, permissions []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		ctx := security.WithRoleContext(c.Request.Context(), security.RoleContext{
			Name:        "test_role",
			Permissions: permissions,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	service := appEdgeDevices.NewService(nil, nil, zap.NewNop())
	group := r.Group("")
	edge_devices.RegisterRoutes(group, service)
	return r
}

func TestEdgeDevicesGetSinPermisoDa403(t *testing.T) {
	r := newTestRouterWithRole(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/edge-devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "sin perm_edge_devices_view debe dar 403")
}

func TestEdgeDevicesGetConPermisoViewPasaElGate(t *testing.T) {
	r := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodGet, "/edge-devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusForbidden, w.Code, "con perm_edge_devices_view el request debe pasar el gate de RBAC")
}

func TestEdgeDevicesPostCreateSoloConManagePasaElGate(t *testing.T) {
	rView := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w := httptest.NewRecorder()
	rView.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "perm_edge_devices_view no debe alcanzar para POST /edge-devices")

	rManage := newTestRouterWithRole(t, []string{"perm_edge_devices_manage"})
	req2 := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w2 := httptest.NewRecorder()
	rManage.ServeHTTP(w2, req2)
	require.NotEqual(t, http.StatusForbidden, w2.Code, "perm_edge_devices_manage debe pasar el gate de POST /edge-devices")
}

func TestEdgeDevicesStatusCheckRequiereManageNoSoloView(t *testing.T) {
	r := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "status check dispara una acción activa contra el device (pasa userID/userEmail para audit trail), requiere _manage no _view")
}
