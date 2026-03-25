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

// DeleteLayout soft-deletes a dashboard layout for the tenant.
func DeleteLayout(service *app.Service) gin.HandlerFunc {
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

		if err := service.DeleteLayout(c.Request.Context(), *tenantID, layoutID); err != nil {
			switch {
			case errors.Is(err, domain.ErrLayoutNotFound):
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Success: false, Error: "Layout no encontrado"})
			case errors.Is(err, domain.ErrCannotDeleteLastLayout):
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Success: false, Error: "No se puede eliminar el único layout"})
			default:
				c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Success: false, Error: "failed to delete layout"})
			}
			return
		}

		c.JSON(http.StatusOK, dto.DeleteLayoutResponse{
			Success: true,
			Message: "Layout eliminado correctamente",
		})
	}
}
