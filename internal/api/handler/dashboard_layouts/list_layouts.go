package dashboard_layouts

import (
	"net/http"

	"github.com/gin-gonic/gin"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListLayouts returns all active dashboard layouts for the tenant.
func ListLayouts(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: "tenant ID not found"})
			return
		}

		layouts, err := service.ListLayouts(c.Request.Context(), *tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Success: false, Error: "failed to list layouts"})
			return
		}

		layoutDTOs := make([]dto.LayoutDTO, len(layouts))
		for i, l := range layouts {
			layoutDTOs[i] = dto.ToLayoutDTO(l)
		}

		c.JSON(http.StatusOK, dto.ListLayoutsResponse{
			Success: true,
			Data:    layoutDTOs,
			Meta: dto.MetaDTO{
				Total: len(layoutDTOs),
				Limit: 3,
			},
		})
	}
}
