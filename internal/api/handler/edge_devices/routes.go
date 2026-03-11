package edge_devices

import (
	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
)

// RegisterRoutes registers all edge device endpoints on the given Gin group.
func RegisterRoutes(g *gin.RouterGroup, service *edge_devices.Service) {
	// US1 – List
	g.GET("/edge-devices", ListDevices(service))

	// US2 – Create
	g.POST("/edge-devices", CreateDevice(service))

	// US6 – Status Check
	g.POST("/edge-devices/:deviceId/status", StatusCheck(service))

	// Routes for individual device operations will be added per user story phase:
	// US3 – Get: GET /edge-devices/:deviceId
	// US4 – Update: PUT /edge-devices/:deviceId
	// US5 – Enable/Disable: POST /edge-devices/:deviceId/enable, /disable
	// US7 – Health Check: POST /edge-devices/:deviceId/health-check
	// US8 – Telemetry: GET /edge-devices/:deviceId/telemetry
	// US9 – Events: GET /edge-devices/:deviceId/events
}
