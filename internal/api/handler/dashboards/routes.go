package dashboards

import (
	"github.com/gin-gonic/gin"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
)

// RegisterRoutes registra las 3 rutas de metricas de dashboard sobre el
// grupo dado. El grupo ya trae RBACCheck(perm_metrics_view) y
// DashboardRateLimit aplicados (routes/url_mappings.go).
func RegisterRoutes(g *gin.RouterGroup, service *app.Service) {
	g.POST("/query", QueryMetrics(service))
	g.GET("/catalog", CatalogMetrics(service))
	g.POST("/query/batch", BatchQueryMetrics(service))
}
