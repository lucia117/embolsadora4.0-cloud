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

	service := appEdgeDevices.NewService(nil, nil, zap.NewNop(), nil, nil)
	group := r.Group("")
	edge_devices.RegisterRoutes(group, group, service)
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

func TestEdgeDevicesPostCreateRequiereCreateNoManage(t *testing.T) {
	// Solo view: 403
	rView := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w := httptest.NewRecorder()
	rView.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "perm_edge_devices_view no alcanza para POST /edge-devices")

	// Solo manage: 403 (manage ya no cubre el alta — ver migración 000013)
	rManage := newTestRouterWithRole(t, []string{"perm_edge_devices_manage"})
	req2 := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w2 := httptest.NewRecorder()
	rManage.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusForbidden, w2.Code, "perm_edge_devices_manage no alcanza para POST /edge-devices, requiere _create")

	// Con create: pasa el gate
	rCreate := newTestRouterWithRole(t, []string{"perm_edge_devices_create"})
	req3 := httptest.NewRequest(http.MethodPost, "/edge-devices", nil)
	w3 := httptest.NewRecorder()
	rCreate.ServeHTTP(w3, req3)
	require.NotEqual(t, http.StatusForbidden, w3.Code, "perm_edge_devices_create debe pasar el gate de POST /edge-devices")
}

// TestEdgeDevicesCreateNoAbreManage es la assertion conversa de
// TestEdgeDevicesPostCreateRequiereCreateNoManage: así como _manage no alcanza
// para el alta, _create SOLO no alcanza para las rutas que piden _manage
// (update / enable / disable).
func TestEdgeDevicesCreateNoAbreManage(t *testing.T) {
	const deviceID = "11111111-1111-1111-1111-111111111111"
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/edge-devices/" + deviceID},
		{http.MethodPost, "/edge-devices/" + deviceID + "/enable"},
		{http.MethodPost, "/edge-devices/" + deviceID + "/disable"},
	}
	for _, tc := range cases {
		r := newTestRouterWithRole(t, []string{"perm_edge_devices_create"})
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code,
			"%s %s requiere perm_edge_devices_manage; _create solo no debe pasar el gate", tc.method, tc.path)
	}
}

func TestEdgeDevicesStatusCheckRequiereCheckNoViewNiManage(t *testing.T) {
	// Sin permisos: debe dar 403
	rNoPerms := newTestRouterWithRole(t, nil)
	req := httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/status", nil)
	w := httptest.NewRecorder()
	rNoPerms.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "sin perm_edge_devices_check debe dar 403 en status check")

	// Con solo view: no alcanza, debe dar 403
	rView := newTestRouterWithRole(t, []string{"perm_edge_devices_view"})
	req = httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/status", nil)
	w = httptest.NewRecorder()
	rView.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "perm_edge_devices_view no alcanza para status check, requiere _check")

	// Con solo manage: no alcanza, debe dar 403.
	// (manage nunca implicó check; son permisos independientes en el seed)
	rManage := newTestRouterWithRole(t, []string{"perm_edge_devices_manage"})
	req = httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/status", nil)
	w = httptest.NewRecorder()
	rManage.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "perm_edge_devices_manage no alcanza para status check, requiere _check")

	// Con check: debe pasar el gate
	rCheck := newTestRouterWithRole(t, []string{"perm_edge_devices_check"})
	req = httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/status", nil)
	w = httptest.NewRecorder()
	rCheck.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusForbidden, w.Code, "perm_edge_devices_check debe pasar el gate de status check")

	// Mismo para health-check
	rCheckHealth := newTestRouterWithRole(t, []string{"perm_edge_devices_check"})
	req = httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/health-check", nil)
	w = httptest.NewRecorder()
	rCheckHealth.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusForbidden, w.Code, "perm_edge_devices_check debe pasar el gate de health-check")

	// health-check con solo manage debe dar 403
	rManageHealth := newTestRouterWithRole(t, []string{"perm_edge_devices_manage"})
	req = httptest.NewRequest(http.MethodPost, "/edge-devices/11111111-1111-1111-1111-111111111111/health-check", nil)
	w = httptest.NewRecorder()
	rManageHealth.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code, "health-check con solo _manage debe dar 403, requiere _check")
}
