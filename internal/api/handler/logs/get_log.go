package logs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/logs/dto"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// GetLog handles GET /logs/:id
func GetLog(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("get", "400").Inc()
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": "invalid id"})
			return
		}

		entry, err := svc.Get(c.Request.Context(), tenantID, id)
		if err != nil {
			if err == domain.ErrLogNotFound {
				telemetry.LogRequestsTotal.WithLabelValues("get", "404").Inc()
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "NOT_FOUND", "status": 404})
				return
			}
			telemetry.LogRequestsTotal.WithLabelValues("get", "500").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_ERROR"})
			return
		}

		telemetry.LogRequestsTotal.WithLabelValues("get", "200").Inc()
		c.JSON(http.StatusOK, dto.ToLogResponse(*entry))
	}
}
