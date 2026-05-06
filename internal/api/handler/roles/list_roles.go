package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func ListRoles(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		roles, err := service.ListRoles(c.Request.Context(), tenantID)
		if err != nil {
			HandleError(c, err)
			return
		}

		items := make([]dto.RoleResponse, len(roles))
		for i, r := range roles {
			items[i] = dto.FromDomain(r)
		}

		c.JSON(http.StatusOK, items)
	}
}
