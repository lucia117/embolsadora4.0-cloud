package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// GetRole godoc
// GET /api/v1/roles/:id
func GetRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		role, err := service.GetRole(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, domain.ErrRoleNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "rol no encontrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno del servidor"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": dto.FromDomain(role)})
	}
}
