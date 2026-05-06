package dashboard_layouts

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboard_layouts/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

func GetLayout(service *app.Service) gin.HandlerFunc {
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

		layoutID, err := uuid.Parse(c.Param("layoutId"))
		if err != nil {
			invalidIDResponse(c)
			return
		}

		layout, err := service.GetLayout(c.Request.Context(), tenantID, *userID, layoutID)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, dto.ToLayoutDTO(layout))
	}
}
