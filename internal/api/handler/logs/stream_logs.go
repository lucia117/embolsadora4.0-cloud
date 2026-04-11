package logs

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	appLogs "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

// StreamLogs handles GET /logs/stream (SSE)
func StreamLogs(svc *appLogs.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, ok := parseTenantID(c)
		if !ok {
			return
		}

		// Optional filters (for future client-side filtering if needed)
		_ = c.Query("event_type")
		_ = c.Query("severity")

		ch := svc.Subscribe(tenantID)
		defer svc.Unsubscribe(tenantID, ch)

		telemetry.LogRequestsTotal.WithLabelValues("stream", "200").Inc()

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Status(http.StatusOK)

		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		c.Stream(func(w io.Writer) bool {
			select {
			case entry, open := <-ch:
				if !open {
					return false
				}
				data, err := json.Marshal(entry)
				if err != nil {
					return true
				}
				c.SSEvent("", string(data))
				return true
			case <-heartbeat.C:
				c.SSEvent("heartbeat", "")
				return true
			case <-c.Request.Context().Done():
				return false
			}
		})
	}
}
