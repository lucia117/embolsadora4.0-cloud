package logs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/logs/dto"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// GetRetention handles GET /logs/retention
func GetRetention(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		policy, err := svc.GetRetention(c.Request.Context(), tenantID)
		if err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("get_retention", "500").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_ERROR"})
			return
		}

		telemetry.LogRequestsTotal.WithLabelValues("get_retention", "200").Inc()
		c.JSON(http.StatusOK, dto.ToRetentionResponse(*policy))
	}
}
