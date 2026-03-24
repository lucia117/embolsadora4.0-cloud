package edge_devices

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	edgeerrors "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// StatusCheck performs a connectivity + version check on a device.
func StatusCheck(service *edge_devices.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant ID from context
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}

		// Extract device ID from path
		deviceIDStr := c.Param("deviceId")
		deviceID, err := uuid.Parse(deviceIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid device ID"})
			return
		}

		// Extract user ID and email from JWT context
		userID := platform.UserID(c.Request.Context())
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "user ID not found in context"})
			return
		}
		userEmail := platform.UserEmail(c.Request.Context())
		if userEmail == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "user email not found in context"})
			return
		}

		// Perform status check
		result, err := service.StatusCheck(c.Request.Context(), *tenantID, deviceID, *userID, userEmail)
		if err != nil {
			// Handle domain errors
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Not found"})
				return
			}
			if errors.Is(err, edgeerrors.ErrDeviceDisabled) {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "EDGE_DEVICE_DISABLED"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to perform status check"})
			return
		}

		response := dto.CheckResultToResponse(result)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
	}
}
