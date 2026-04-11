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

// GetLogContext handles GET /logs/:id/context
func GetLogContext(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": "invalid id"})
			return
		}

		var params dto.GetContextParams
		if err := c.ShouldBindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": err.Error()})
			return
		}

		before, anchor, after, err := svc.GetContext(c.Request.Context(), tenantID, id, params.WindowSize)
		if err != nil {
			if err == domain.ErrLogNotFound {
				telemetry.LogRequestsTotal.WithLabelValues("context", "404").Inc()
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "NOT_FOUND", "status": 404})
				return
			}
			telemetry.LogRequestsTotal.WithLabelValues("context", "500").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_ERROR"})
			return
		}

		telemetry.LogRequestsTotal.WithLabelValues("context", "200").Inc()

		beforeDTO := make([]dto.LogResponse, len(before))
		for i, e := range before {
			beforeDTO[i] = dto.ToLogResponse(e)
		}
		afterDTO := make([]dto.LogResponse, len(after))
		for i, e := range after {
			afterDTO[i] = dto.ToLogResponse(e)
		}

		c.JSON(http.StatusOK, dto.LogContextResponse{
			Before: beforeDTO,
			Anchor: dto.ToLogResponse(*anchor),
			After:  afterDTO,
		})
	}
}
