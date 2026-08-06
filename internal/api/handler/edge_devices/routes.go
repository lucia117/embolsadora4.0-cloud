package edge_devices

import (
	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
)

// RegisterRoutes registra los endpoints de edge devices.
// writeGroup lleva RBACCheck("machines:write"): emitir credenciales que dan
// acceso de escritura a la ingesta no puede ser una operacion de solo-lectura.
func RegisterRoutes(g *gin.RouterGroup, writeGroup *gin.RouterGroup, service *edge_devices.Service) {
	// US1 – List
	g.GET("/edge-devices", ListDevices(service))

	// US2 – Create
	g.POST("/edge-devices", CreateDevice(service))

	// US3 – Get
	g.GET("/edge-devices/:deviceId", GetDevice(service))

	// US4 – Update
	g.PUT("/edge-devices/:deviceId", UpdateDevice(service))

	// US5 – Enable/Disable
	g.POST("/edge-devices/:deviceId/enable", EnableDevice(service))
	g.POST("/edge-devices/:deviceId/disable", DisableDevice(service))

	// US6 – Status Check
	g.POST("/edge-devices/:deviceId/status", StatusCheck(service))

	// US7 – Health Check
	g.POST("/edge-devices/:deviceId/health-check", HealthCheck(service))

	// US8 – Telemetry
	g.GET("/edge-devices/:deviceId/telemetry", GetTelemetry(service))

	// US9 – Events
	g.GET("/edge-devices/:deviceId/events", ListEvents(service))

	// API keys del device
	writeGroup.POST("/edge-devices/:deviceId/api-keys", CreateAPIKey(service))
	g.GET("/edge-devices/:deviceId/api-keys", ListAPIKeys(service))
	writeGroup.DELETE("/edge-devices/:deviceId/api-keys/:keyId", RevokeAPIKey(service))
}
