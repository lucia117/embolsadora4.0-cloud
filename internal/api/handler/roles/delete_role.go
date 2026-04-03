package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// DeleteRole godoc
// DELETE /api/v1/roles/:id
func DeleteRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		err := service.DeleteRole(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, domain.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "NOT_FOUND", "message": "rol no encontrado"})
				return
			}
			if errors.Is(err, domain.ErrRoleIsSystemRole) {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "SYSTEM_ROLE", "message": err.Error()})
				return
			}
			if errors.Is(err, domain.ErrRoleHasAssignments) {
				count, countErr := service.CountActiveAssignments(c.Request.Context(), id)
				if countErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_SERVER_ERROR", "message": "error interno del servidor"})
					return
				}
				c.JSON(http.StatusConflict, gin.H{
					"success":       false,
					"error":         "ROLE_HAS_ASSIGNMENTS",
					"message":       err.Error(),
					"usersAffected": count,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_SERVER_ERROR", "message": "error interno del servidor"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}
