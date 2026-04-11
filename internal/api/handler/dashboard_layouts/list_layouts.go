package dashboard_layouts

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListLayouts returns all active dashboard layouts for the (tenant, user).
func ListLayouts(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: "missing or invalid X-Tenant-ID header"})
			return
		}

		userID := platform.UserID(c.Request.Context())
		if userID == nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Success: false, Error: "user not authenticated"})
			return
		}

		layouts, err := service.ListLayouts(c.Request.Context(), tenantID, *userID)
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
