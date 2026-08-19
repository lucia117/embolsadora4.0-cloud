package edge_devices

import (
	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
)

// RegisterRoutes registers all edge device endpoints on the given Gin group.
//
// Hasta esta fix, ningún endpoint acá tenía RBACCheck — cualquier miembro
// autenticado del tenant (incluido operario) podía crear/actualizar/habilitar
// dispositivos. Los permission ids ya existían en el seed (migración 000011),
// solo faltaba cablearlos. Ver docs/superpowers/specs/2026-08-19-production-readiness-cleanup-design.md §C.
func RegisterRoutes(g *gin.RouterGroup, service *edge_devices.Service) {
	view := middleware.RBACCheck("perm_edge_devices_view")
	manage := middleware.RBACCheck("perm_edge_devices_manage")

	// US1 – List
	g.GET("/edge-devices", view, ListDevices(service))

	// US2 – Create
	g.POST("/edge-devices", manage, CreateDevice(service))

	// US3 – Get
	g.GET("/edge-devices/:deviceId", view, GetDevice(service))

	// US4 – Update
	g.PUT("/edge-devices/:deviceId", manage, UpdateDevice(service))

	// US5 – Enable/Disable
	g.POST("/edge-devices/:deviceId/enable", manage, EnableDevice(service))
	g.POST("/edge-devices/:deviceId/disable", manage, DisableDevice(service))

	// US6 – Status Check: dispara una acción activa contra el device (el
	// handler pasa userID/userEmail al service para audit trail), no es una
	// lectura pasiva — requiere _manage, igual que enable/disable.
	g.POST("/edge-devices/:deviceId/status", manage, StatusCheck(service))

	// US7 – Health Check: mismo criterio que status check.
	g.POST("/edge-devices/:deviceId/health-check", manage, HealthCheck(service))

	// US8 – Telemetry
	g.GET("/edge-devices/:deviceId/telemetry", view, GetTelemetry(service))

	// US9 – Events
	g.GET("/edge-devices/:deviceId/events", view, ListEvents(service))
}
