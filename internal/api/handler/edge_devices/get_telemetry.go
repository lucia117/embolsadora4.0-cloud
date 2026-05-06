package edge_devices

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	edgeerrors "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// GetTelemetry retrieves the latest telemetry snapshot from a device.
func GetTelemetry(service *edge_devices.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant ID from context
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}

		// Extract device ID from path parameter
		deviceIDStr := c.Param("deviceId")
		deviceID, err := uuid.Parse(deviceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid device ID format"})
			return
		}

		// Get telemetry
		telemetry, err := service.GetTelemetry(c.Request.Context(), *tenantID, deviceID)
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Not found"})
				return
			}
			if errors.Is(err, edgeerrors.ErrDeviceDisabled) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "EDGE_DEVICE_DISABLED"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to retrieve telemetry"})
			return
		}

		response := dto.TelemetryToResponse(telemetry)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
	}
}
