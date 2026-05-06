package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func CreateRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		var req dto.CreateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "VALIDATION_ERROR",
				Message: err.Error(),
				Status:  http.StatusBadRequest,
			})
			return
		}

		role, err := service.CreateRole(c.Request.Context(), tenantID, req.Name, req.Description, req.Permissions)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusCreated, dto.FromDomain(role))
	}
}
