package edge_devices

import (
	"time"

	"github.com/google/uuid"
)

// EdgeDevice represents a physical edge computing unit (Raspberry Pi + PLC).
type EdgeDevice struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	Name               string
	Description        *string
	MachineID          string
	EdgeType           string // "RASPBERRY_PLC"
	RaspberryBaseURL   string
	PLCAddress         *string
	Status             string // "ACTIVE", "DISABLED"
	LastSeenAt         *time.Time
	LastHealthCheckAt  *time.Time
	LastHealthStatus   string // "OK", "DEGRADED", "ERROR", "UNKNOWN"
	LastHealthSummary  *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// DeviceEvent represents an immutable record of a triggered check.
type DeviceEvent struct {
	ID            uuid.UUID
	DeviceID      uuid.UUID
	TenantID      uuid.UUID
	CheckType     string // "STATUS", "HEALTH_CHECK"
	CheckedAt     time.Time
	OverallStatus string // "OK", "DEGRADED", "ERROR", "UNKNOWN"
	Summary       *string
	Details       map[string]interface{}
	UserID        uuid.UUID
	UserEmail     string
}

// CheckResult represents the result of a device check.
type CheckResult struct {
	CheckType     string
	CheckedAt     time.Time
	OverallStatus string // "OK", "DEGRADED", "ERROR", "UNKNOWN"
	Summary       *string
	Details       map[string]interface{}
}

// TelemetrySnapshot represents a live hardware + PLC snapshot.
type TelemetrySnapshot struct {
	CapturedAt        time.Time
	CPU               *float64
	RAM               *float64 // percentage
	Disk              *float64 // percentage
	TemperatureCelsius *float64
	UptimeSeconds     *int64
	PLC               *PLCSnapshot
}

// PLCSnapshot represents PLC connectivity state.
type PLCSnapshot struct {
	Reachable bool
	Address   *string
}
