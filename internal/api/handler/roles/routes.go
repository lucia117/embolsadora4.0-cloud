package roles

import (
	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
)

// RegisterRoutes registra todos los endpoints de roles en el grupo Gin dado.
func RegisterRoutes(g *gin.RouterGroup, service *appRoles.Service) {
	g.GET("/roles", ListRoles(service))
	g.POST("/roles", CreateRole(service))
	g.GET("/roles/:id", GetRole(service))
	g.PUT("/roles/:id", UpdateRole(service))
	g.DELETE("/roles/:id", DeleteRole(service))
}
