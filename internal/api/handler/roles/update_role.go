package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// UpdateRole godoc
// PUT /api/v1/roles/:id
func UpdateRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req dto.UpdateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "INVALID_REQUEST", "message": err.Error()})
			return
		}

		role, err := service.UpdateRole(c.Request.Context(), id, req.Name, req.Description, req.Permissions)
		if err != nil {
			if errors.Is(err, domain.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "NOT_FOUND", "message": "rol no encontrado"})
				return
			}
			if errors.Is(err, domain.ErrRoleIsSystemRole) {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "SYSTEM_ROLE", "message": err.Error()})
				return
			}
			if errors.Is(err, domain.ErrRoleDuplicateName) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "error": "DUPLICATE_NAME", "message": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_SERVER_ERROR", "message": "error interno del servidor"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": dto.FromDomain(role)})
	}
}
