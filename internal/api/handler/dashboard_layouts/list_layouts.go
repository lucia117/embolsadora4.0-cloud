package dashboard_layouts

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func ListLayouts(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := uuid.Parse(platform.TenantID(c.Request.Context()))
		if err != nil {
			invalidTenantResponse(c)
			return
		}

		userID := platform.UserID(c.Request.Context())
		if userID == nil {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Error:   "UNAUTHORIZED",
				Message: "user not authenticated",
				Status:  http.StatusUnauthorized,
			})
			return
		}

		layouts, err := service.ListLayouts(c.Request.Context(), tenantID, *userID)
		if err != nil {
			HandleError(c, err)
			return
		}

		layoutDTOs := make([]dto.LayoutDTO, len(layouts))
		for i, l := range layouts {
			layoutDTOs[i] = dto.ToLayoutDTO(l)
		}

		c.JSON(http.StatusOK, dto.ListLayoutsResponse{
			Data: layoutDTOs,
			Meta: dto.MetaDTO{Total: len(layoutDTOs), Limit: 3},
		})
	}
}
