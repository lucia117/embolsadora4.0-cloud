package edge_devices

import (
	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
)

// RegisterRoutes registra los endpoints de edge devices.
//
// Hasta la fix de producction-readiness, ningún endpoint acá tenía RBACCheck —
// cualquier miembro autenticado del tenant (incluido operario) podía
// crear/actualizar/habilitar dispositivos. Los permission ids ya existían en
// el seed (migración 000011), solo faltaba cablearlos. Ver
// docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md §C.
//
// Todos los gates de RBAC se aplican acá, por ruta, con ids del catálogo
// `perm_edge_devices_*`. Antes emitir/revocar API keys se delegaba a un
// `writeGroup` pre-gateado por el caller con `RBACCheck("machines:write")` —
// pero `machines:write` es del vocabulario viejo `resource:action`, no está en
// las `permissions` de ningún rol (lo reemplazó el catálogo `perm_*` en la
// migración 000011) y `security.Can()` hace `slices.Contains` exacto: el gate
// negaba a todos, dejando esos dos endpoints inalcanzables. Ahora pasan por
// `manage`, como el resto de las escrituras sobre un device existente.
func RegisterRoutes(g *gin.RouterGroup, service *edge_devices.Service) {
	view := middleware.RBACCheck("perm_edge_devices_view")
	create := middleware.RBACCheck("perm_edge_devices_create")
	manage := middleware.RBACCheck("perm_edge_devices_manage")
	check := middleware.RBACCheck("perm_edge_devices_check")

	// US1 – List
	g.GET("/edge-devices", view, ListDevices(service))

	// US2 – Create
	g.POST("/edge-devices", create, CreateDevice(service))

	// US3 – Get
	g.GET("/edge-devices/:deviceId", view, GetDevice(service))

	// US4 – Update
	g.PUT("/edge-devices/:deviceId", manage, UpdateDevice(service))

	// US5 – Enable/Disable
	g.POST("/edge-devices/:deviceId/enable", manage, EnableDevice(service))
	g.POST("/edge-devices/:deviceId/disable", manage, DisableDevice(service))

	// US6 – Status Check: requiere perm_edge_devices_check (no _manage).
	// Aunque dispara una acción activa contra el device (el handler pasa
	// userID/userEmail al service para audit trail), el seed define check como
	// un permiso distinto e independiente de manage: se otorga a operario y
	// tenant_manager explícitamente (migrations/000002_seed_essentials.up.sql:37,
	// migrations/000005_translate_permissions_and_seed_role_permissions.up.sql:35-38).
	g.POST("/edge-devices/:deviceId/status", check, StatusCheck(service))

	// US7 – Health Check: requiere perm_edge_devices_check, igual que status check.
	g.POST("/edge-devices/:deviceId/health-check", check, HealthCheck(service))

	// US8 – Telemetry
	g.GET("/edge-devices/:deviceId/telemetry", view, GetTelemetry(service))

	// US9 – Events
	g.GET("/edge-devices/:deviceId/events", view, ListEvents(service))

	// API keys del device: emitir/revocar credenciales que dan acceso de
	// escritura a la ingesta no puede ser una operación de solo-lectura →
	// requieren _manage. Listar solo expone metadata (no el secreto), alcanza
	// con _view.
	g.POST("/edge-devices/:deviceId/api-keys", manage, CreateAPIKey(service))
	g.GET("/edge-devices/:deviceId/api-keys", view, ListAPIKeys(service))
	g.DELETE("/edge-devices/:deviceId/api-keys/:keyId", manage, RevokeAPIKey(service))
}
