package roles

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListRoles godoc
// GET /api/v1/roles
func ListRoles(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID inválido o ausente"})
			return
		}

		roles, err := service.ListRoles(c.Request.Context(), tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno del servidor"})
			return
		}

		items := make([]dto.RoleResponse, len(roles))
		for i, r := range roles {
			items[i] = dto.FromDomain(r)
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
	}
}
