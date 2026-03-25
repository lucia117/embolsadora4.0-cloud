package dashboard_layouts

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts/dto"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// GetLayout returns a single dashboard layout by ID for the tenant.
func GetLayout(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: "tenant ID not found"})
			return
		}

		layoutID, err := uuid.Parse(c.Param("layoutId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: "invalid layout ID"})
			return
		}

		layout, err := service.GetLayout(c.Request.Context(), *tenantID, layoutID)
		if err != nil {
			if errors.Is(err, domain.ErrLayoutNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Success: false, Error: "Layout no encontrado"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Success: false, Error: "failed to get layout"})
			return
		}

		c.JSON(http.StatusOK, dto.SingleLayoutResponse{
			Success: true,
			Data:    dto.ToLayoutDTO(layout),
		})
	}
}
