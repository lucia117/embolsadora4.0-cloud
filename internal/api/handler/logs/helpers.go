package logs

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var validSeverities = map[string]bool{
	"":         true,
	"info":     true,
	"warning":  true,
	"critical": true,
	"error":    true,
}

var validEventTypes = map[string]bool{
	"":                      true,
	"alarm_triggered":       true,
	"alarm_resolved":        true,
	"device_connected":      true,
	"device_disconnected":   true,
	"device_state_changed":  true,
	"user_action":           true,
	"system":                true,
}

func parseTenantID(c *gin.Context) (uuid.UUID, bool) {
	raw := c.GetHeader("X-Tenant-ID")
	if raw == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "UNAUTHORIZED", Message: "X-Tenant-ID header required", Status: http.StatusUnauthorized})
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "BAD_REQUEST", Message: "invalid X-Tenant-ID", Status: http.StatusBadRequest})
		return uuid.UUID{}, false
	}
	return id, true
}

func validateSeverity(s string) error {
	if !validSeverities[s] {
		return fmt.Errorf("invalid severity %q", s)
	}
	return nil
}

func validateEventType(et string) error {
	if !validEventTypes[et] {
		return fmt.Errorf("invalid event_type %q", et)
	}
	return nil
}
