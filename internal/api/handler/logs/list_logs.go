package logs

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/api/handler/logs/dto"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/domain"
	logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// ListLogs handles GET /logs
func ListLogs(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		var params dto.ListLogsParams
		if err := c.ShouldBindQuery(&params); err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("list", "400").Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
			return
		}

		if err := validateSeverity(params.Severity); err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("list", "400").Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
			return
		}
		if err := validateEventType(params.EventType); err != nil {
			telemetry.LogRequestsTotal.WithLabelValues("list", "400").Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: err.Error(), Status: http.StatusBadRequest})
			return
		}

		if params.From != nil && params.To != nil && params.From.After(*params.To) {
			telemetry.LogRequestsTotal.WithLabelValues("list", "400").Inc()
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: "from must be before to", Status: http.StatusBadRequest})
			return
		}

		repoParams := logsRepo.ListParams{
			TenantID:  tenantID,
			EventType: params.EventType,
			Severity:  params.Severity,
			From:      params.From,
			To:        params.To,
			Q:         params.Q,
			Cursor:    params.Cursor,
			Limit:     params.Limit,
		}
		if params.MachineID != "" {
			mid, err := uuid.Parse(params.MachineID)
			if err != nil {
				telemetry.LogRequestsTotal.WithLabelValues("list", "400").Inc()
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: "invalid machine_id", Status: http.StatusBadRequest})
				return
			}
			repoParams.MachineID = &mid
		}

		result, err := svc.List(c.Request.Context(), repoParams)
		if err != nil {
			if err == domain.ErrInvalidCursor {
				telemetry.LogRequestsTotal.WithLabelValues("list", "400").Inc()
				c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: "invalid cursor", Status: http.StatusBadRequest})
				return
			}
			telemetry.LogRequestsTotal.WithLabelValues("list", "500").Inc()
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "INTERNAL_ERROR", Message: "internal server error", Status: http.StatusInternalServerError})
			return
		}

		telemetry.LogRequestsTotal.WithLabelValues("list", "200").Inc()
		telemetry.LogListLatency.WithLabelValues("list").Observe(time.Since(start).Seconds())

		data := make([]dto.LogResponse, len(result.Entries))
		for i, e := range result.Entries {
			data[i] = dto.ToLogResponse(e)
		}

		c.JSON(http.StatusOK, dto.LogListResponse{
			Data:       data,
			NextCursor: result.NextCursor,
			Total:      result.Total,
		})
	}
}
