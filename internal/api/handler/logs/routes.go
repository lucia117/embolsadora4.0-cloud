package logs

import (
	"github.com/gin-gonic/gin"
	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
)

// RegisterRoutes registers all log service routes on the given router group.
// Static routes (retention, stream, export) MUST be registered before the :id
// wildcard to avoid Gin routing conflicts.
func RegisterRoutes(rg *gin.RouterGroup, svc *appLogs.Service) {
	// Static routes first
	rg.GET("/logs/retention", GetRetention(svc))
	rg.PATCH("/logs/retention", apimw.RBACCheck("logs:admin"), UpdateRetention(svc))
	rg.GET("/logs/stream", StreamLogs(svc))
	rg.GET("/logs/export", ExportLogs(svc))

	// List
	rg.GET("/logs", ListLogs(svc))

	// Parameterized routes last
	rg.GET("/logs/:id", GetLog(svc))
	rg.GET("/logs/:id/context", GetLogContext(svc))
}
