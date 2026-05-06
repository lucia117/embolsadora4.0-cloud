package dashboard_layouts

import (
	"github.com/gin-gonic/gin"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
)

// RegisterRoutes registers all dashboard layout endpoints on the given Gin group.
func RegisterRoutes(g *gin.RouterGroup, service *app.Service) {
	// US1 – List
	g.GET("/dashboard-layouts", ListLayouts(service))

	// US2 – Create
	g.POST("/dashboard-layouts", CreateLayout(service))

	// US3 – Get by ID
	g.GET("/dashboard-layouts/:layoutId", GetLayout(service))

	// US4 – Update
	g.PUT("/dashboard-layouts/:layoutId", UpdateLayout(service))

	// US5 – Delete
	g.DELETE("/dashboard-layouts/:layoutId", DeleteLayout(service))
}
