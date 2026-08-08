package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

func DeleteRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id := c.Param("id")

		includeGlobal := security.CanSeePlatformInternals(c.Request.Context())

		err = service.DeleteRole(c.Request.Context(), id, tenantID, includeGlobal)
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
