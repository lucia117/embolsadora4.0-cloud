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

func GetRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		id := c.Param("id")

		includeGlobal := security.CanSeePlatformInternals(c.Request.Context())

		role, err := service.GetRole(c.Request.Context(), id, tenantID, includeGlobal)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.FromDomain(role))
	}
}
