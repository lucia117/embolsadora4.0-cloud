package roles

import (
	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
)

// RegisterRoutes registra los endpoints de roles.
// readGroup: sin RBAC adicional (GET /roles, GET /roles/:id).
// writeGroup: con RBACCheck("users:write") aplicado externamente (POST, PUT, DELETE).
func RegisterRoutes(readGroup, writeGroup *gin.RouterGroup, service *appRoles.Service) {
	readGroup.GET("/roles", ListRoles(service))
	readGroup.GET("/roles/:id", GetRole(service))

	writeGroup.POST("/roles", CreateRole(service))
	writeGroup.PUT("/roles/:id", UpdateRole(service))
	writeGroup.DELETE("/roles/:id", DeleteRole(service))
}
