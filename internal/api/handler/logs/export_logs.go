package logs

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/logs/dto"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// ExportLogs handles GET /logs/export
func ExportLogs(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		var params dto.ExportLogsParams
		if err := c.ShouldBindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": err.Error()})
			return
		}

		if err := validateSeverity(params.Severity); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": err.Error()})
			return
		}
		if err := validateEventType(params.EventType); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": err.Error()})
			return
		}

		if params.From != nil && params.To != nil && params.From.After(*params.To) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": "from must be before to"})
			return
		}

		repoParams := logsRepo.ExportParams{
			TenantID:  tenantID,
			EventType: params.EventType,
			Severity:  params.Severity,
			From:      params.From,
			To:        params.To,
			Q:         params.Q,
		}
		if params.MachineID != "" {
			mid, err := uuid.Parse(params.MachineID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": "invalid machine_id"})
				return
			}
			repoParams.MachineID = &mid
		}

		result, err := svc.Export(c.Request.Context(), repoParams)
		if err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("export", "500").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "INTERNAL_ERROR"})
			return
		}

		truncatedLabel := "false"
		if result.Truncated {
			truncatedLabel = "true"
		}
		telemetry.LogExportTotal.WithLabelValues(truncatedLabel).Inc()
		telemetry.LogListLatency.WithLabelValues("export").Observe(time.Since(start).Seconds())

		data := make([]dto.LogResponse, len(result.Entries))
		for i, e := range result.Entries {
			data[i] = dto.ToLogResponse(e)
		}

		c.JSON(http.StatusOK, dto.LogExportResponse{
			Data:           data,
			Truncated:      result.Truncated,
			ExportedCount:  len(result.Entries),
			TotalAvailable: result.TotalAvailable,
		})
	}
}
