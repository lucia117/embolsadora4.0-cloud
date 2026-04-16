package logs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tu-org/embolsadora-api/internal/api/handler/logs/dto"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// UpdateRetention handles PATCH /logs/retention
func UpdateRetention(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		var req dto.UpdateRetentionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("update_retention", "400").Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
			return
		}

		policy, err := svc.UpdateRetention(c.Request.Context(), tenantID, req.RetentionDays)
		if err != nil {
			if err == domain.ErrInvalidRetentionDays {
				telemetry.LogRequestsTotal.WithLabelValues("update_retention", "400").Inc()
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
				return
			}
			telemetry.LogRequestsTotal.WithLabelValues("update_retention", "500").Inc()
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "INTERNAL_ERROR", Message: "internal server error", Status: http.StatusInternalServerError})
			return
		}

		telemetry.LogRequestsTotal.WithLabelValues("update_retention", "200").Inc()
		c.JSON(http.StatusOK, dto.ToRetentionResponse(*policy))
	}
}
