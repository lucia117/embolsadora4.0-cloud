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
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "UNAUTHORIZED", "message": "X-Tenant-ID header required"})
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "BAD_REQUEST", "message": "invalid X-Tenant-ID"})
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
