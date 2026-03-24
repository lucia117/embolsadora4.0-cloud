package edge_devices

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices/dto"
	edgeerrors "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// CreateDevice creates a new edge device.
func CreateDevice(service *edge_devices.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract tenant ID from context
		tenantID := platform.TenantUUID(c.Request.Context())
		if tenantID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "tenant ID not found"})
			return
		}

		// Parse request body
		var req dto.CreateDeviceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
			return
		}

		// Validate required fields
		if req.Name == "" || req.MachineID == "" || req.EdgeType == "" || req.RaspberryBaseURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "name, machineId, edgeType, and raspberryBaseUrl are required"})
			return
		}

		if req.EdgeType != "RASPBERRY_PLC" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "edgeType must be RASPBERRY_PLC"})
			return
		}

		// Build command
		cmd := edgeerrors.CreateDeviceCommand{
			Name:             req.Name,
			MachineID:        req.MachineID,
			EdgeType:         req.EdgeType,
			RaspberryBaseURL: req.RaspberryBaseURL,
			Description:      req.Description,
			PLCAddress:       req.PLCAddress,
		}

		// Create device
		device, err := service.CreateDevice(c.Request.Context(), *tenantID, cmd)
		if err != nil {
			// Handle domain errors
			if errors.Is(err, edgeerrors.ErrMachineIDConflict) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "error": "CONFLICT: machineId ya existe en el tenant"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create device"})
			return
		}

		response := dto.EdgeDeviceToResponse(device)
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": response})
	}
}
