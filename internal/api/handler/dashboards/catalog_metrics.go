package dashboards

import (
	"net/http"

	"github.com/gin-gonic/gin"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
)

func CatalogMetrics(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := normalizedTenantID(c.Request.Context())
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID invalido o ausente", "code": "INVALID_PARAMS"})
			return
		}

		machineID := c.Query("machineId")
		paths, err := service.Catalog(c.Request.Context(), tenantID, machineID)
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"machineId": machineID, "aasPaths": paths}})
	}
}
