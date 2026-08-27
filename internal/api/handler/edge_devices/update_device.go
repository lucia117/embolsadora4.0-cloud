package edge_devices

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	edgeerrors "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// UpdateDevice updates a device's mutable fields (name, description,
// raspberryBaseUrl, plcAddress). machineId and edgeType are immutable.
func UpdateDevice(service *edge_devices.Service) gin.HandlerFunc {
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

		// Parse request body
		var req dto.UpdateDeviceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
			return
		}

		// Reject a blank name on update (CreateDevice guards this too).
		if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "VALIDATION_ERROR: name no puede estar vacío"})
			return
		}

		// Validate raspberryBaseUrl as an http(s) URL when present
		if req.RaspberryBaseURL != nil {
			u, perr := url.ParseRequestURI(*req.RaspberryBaseURL)
			if perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "VALIDATION_ERROR: raspberryBaseUrl inválida"})
				return
			}
		}

		// Build command
		cmd := edgeerrors.UpdateDeviceCommand{
			Name:             req.Name,
			Description:      req.Description,
			RaspberryBaseURL: req.RaspberryBaseURL,
			PLCAddress:       req.PLCAddress,
		}

		// Update device
		device, err := service.UpdateDevice(c.Request.Context(), *tenantID, deviceID, cmd)
		if err != nil {
			if errors.Is(err, edgeerrors.ErrDeviceNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to update device"})
			return
		}

		response := dto.EdgeDeviceToResponse(device)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
	}
}
