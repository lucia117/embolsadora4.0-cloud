package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

func UpdateRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id := c.Param("id")

		var req dto.UpdateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: err.Error(),
				Status:  http.StatusBadRequest,
			})
			return
		}

		includeGlobal := security.CanSeePlatformInternals(c.Request.Context())

		role, err := service.UpdateRole(c.Request.Context(), id, tenantID, includeGlobal, req.Name, req.Description, req.Permissions)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(role))
	}
}
