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

// UpdateLayout replaces the name and widgets of an existing dashboard layout.
func UpdateLayout(service *app.Service) gin.HandlerFunc {
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

		layoutID, err := uuid.Parse(c.Param("layoutId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: "invalid layout ID"})
			return
		}

		var req dto.UpdateLayoutRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: err.Error()})
			return
		}

		cmd := domain.UpdateLayoutCommand{
			Name:    req.Name,
			Widgets: dto.ToWidgetsDomain(req.Widgets),
		}

		layout, err := service.UpdateLayout(c.Request.Context(), tenantID, *userID, layoutID, cmd)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrLayoutNotFound):
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Success: false, Error: "NOT_FOUND"})
			case errors.Is(err, domain.ErrDuplicateName):
				c.JSON(http.StatusConflict, dto.ErrorResponse{Success: false, Error: "DUPLICATE_NAME"})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Success: false, Error: "failed to update layout"})
			}
			return
		}

		c.JSON(http.StatusOK, dto.SingleLayoutResponse{
			Success: true,
			Data:    dto.ToLayoutDTO(layout),
		})
	}
}
