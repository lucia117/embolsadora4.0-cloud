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
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: "invalid id", Status: http.StatusBadRequest})
			return
		}

		entry, err := svc.Get(c.Request.Context(), tenantID, id)
		if err != nil {
			if err == domain.ErrLogNotFound {
				telemetry.LogRequestsTotal.WithLabelValues("get", "404").Inc()
				c.JSON(http.StatusNotFound, ErrorResponse{Error: "NOT_FOUND", Message: "log entry not found", Status: http.StatusNotFound})
				return
			}
			telemetry.LogRequestsTotal.WithLabelValues("get", "500").Inc()
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "INTERNAL_ERROR", Message: "internal server error", Status: http.StatusInternalServerError})
			return
		}

		telemetry.LogRequestsTotal.WithLabelValues("get", "200").Inc()
		c.JSON(http.StatusOK, dto.ToLogResponse(*entry))
	}
}
