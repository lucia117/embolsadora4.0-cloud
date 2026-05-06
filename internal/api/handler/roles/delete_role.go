package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

func DeleteRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		err := service.DeleteRole(c.Request.Context(), id)
		if err != nil {
			if errors.Is(err, domain.ErrRoleHasAssignments) {
				count, countErr := service.CountActiveAssignments(c.Request.Context(), id)
				if countErr != nil {
					HandleError(c, countErr)
					return
				}
				c.JSON(http.StatusConflict, roleHasAssignmentsResponse{
					Error:         "ROLE_HAS_ASSIGNMENTS",
					Message:       err.Error(),
					Status:        http.StatusConflict,
					UsersAffected: count,
				})
				return
			}
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{})
	}
}
