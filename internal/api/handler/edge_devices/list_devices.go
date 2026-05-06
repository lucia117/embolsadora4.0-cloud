package edge_devices

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// ListDevices returns all edge devices for the tenant.
func ListDevices(service *edge_devices.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant ID from context (set by ResolveTenantFromPath middleware)
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}

		devices, err := service.ListDevices(c.Request.Context(), *tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to list devices"})
			return
		}

		// Convert domain devices to response DTOs
		responses := make([]dto.EdgeDeviceResponse, len(devices))
		for i, device := range devices {
			responses[i] = dto.EdgeDeviceToResponse(device)
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": responses})
	}
}
