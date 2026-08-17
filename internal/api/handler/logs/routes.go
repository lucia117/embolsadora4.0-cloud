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
	// NOTE: "logs:admin" appeared in NO role's list under the old hardcoded Go
	// map, so this route was effectively dead (403 for everyone). perm_logs_admin
	// is a new grant, not a like-for-like string swap — it's live for super_admin,
	// which holds perm_logs_admin in the DB catalog since migration 000005.
	rg.PATCH("/logs/retention", apimw.RBACCheck("perm_logs_admin"), UpdateRetention(svc))
	rg.GET("/logs/stream", StreamLogs(svc))
	rg.GET("/logs/export", ExportLogs(svc))

	// List
	rg.GET("/logs", ListLogs(svc))

	// Parameterized routes last
	rg.GET("/logs/:id", GetLog(svc))
	rg.GET("/logs/:id/context", GetLogContext(svc))
}
