package roles

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	appRoles "github.com/tu-org/embolsadora-api/internal/app/roles"
	"github.com/tu-org/embolsadora-api/internal/api/handler/roles/dto"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateRole godoc
// POST /api/v1/roles
func CreateRole(service *appRoles.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID inválido o ausente"})
			return
		}

		var req dto.CreateRoleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "cuerpo de la petición inválido: " + err.Error()})
			return
		}

		role, err := service.CreateRole(c.Request.Context(), tenantID, req.Name, req.Description, req.Permissions)
		if err != nil {
			if errors.Is(err, domain.ErrRoleLimitReached) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "error": err.Error()})
				return
			}
			if errors.Is(err, domain.ErrRoleDuplicateName) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "error interno del servidor"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": dto.FromDomain(role)})
	}
}
