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

// HealthCheck performs a full hardware diagnostic on a device.
func HealthCheck(service *edge_devices.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant ID from context
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}

		// Extract user ID (placeholder for now, should come from JWT claims)
		userID := uuid.New()
		userEmail := "operator@example.com"

		// Extract device ID from path parameter
		deviceIDStr := c.Param("deviceId")
		deviceID, err := uuid.Parse(deviceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid device ID format"})
			return
		}

		// Perform health check
		result, err := service.HealthCheck(c.Request.Context(), *tenantID, deviceID, userID, userEmail)
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Not found"})
				return
			}
			if errors.Is(err, edgeerrors.ErrDeviceDisabled) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "EDGE_DEVICE_DISABLED"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to perform health check"})
			return
		}

		response := dto.CheckResultToResponse(result)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
	}
}
