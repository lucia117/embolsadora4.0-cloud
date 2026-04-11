package consumers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

// GetShell handles GET /api/v1/consumers/shells/:id
// Allows an IoT device to fetch its own AAS shell (digital twin) using an API key.
// Tenant is resolved from the X-Tenant-ID header.
func GetShell(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.ShellRepo == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mongo unavailable"})
			return
		}

		raw := c.GetHeader("X-Tenant-ID")
		tenantID, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid X-Tenant-ID header"})
			return
		}

		shell, err := deps.ShellRepo.GetByID(c.Request.Context(), tenantID, c.Param("id"))
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "shell not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.JSON(http.StatusOK, shell)
	}
}
