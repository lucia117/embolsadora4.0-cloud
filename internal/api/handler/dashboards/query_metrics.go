package dashboards

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tu-org/embolsadora-api/internal/api/handler/dashboards/dto"
	app "github.com/tu-org/embolsadora-api/internal/app/dashboards"
)

func QueryMetrics(service *app.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := normalizedTenantID(c.Request.Context())
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "X-Tenant-ID invalido o ausente", "code": "INVALID_PARAMS"})
			return
		}

		var req dto.QueryRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "JSON malformado", "code": "INVALID_PARAMS"})
			return
		}

		result, err := service.Query(c.Request.Context(), tenantID, req.ToDomain(), time.Now().UTC())
		if err != nil {
			HandleError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "data": dto.QueryResultToResponse(result)})
	}
}
