package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Severity represents the severity level of a log entry.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
	SeverityError    Severity = "error"
)

// EventType represents the type of event recorded in a log entry.
type EventType string

const (
	EventTypeAlarmTriggered      EventType = "alarm_triggered"
	EventTypeAlarmResolved       EventType = "alarm_resolved"
	EventTypeDeviceConnected     EventType = "device_connected"
	EventTypeDeviceDisconnected  EventType = "device_disconnected"
	EventTypeDeviceStateChanged  EventType = "device_state_changed"
	EventTypeUserAction          EventType = "user_action"
	EventTypeSystem              EventType = "system"
)

// LogEntry is an immutable record of an operational event.
type LogEntry struct {
	ID        uuid.UUID      `json:"id"`
	TenantID  uuid.UUID      `json:"tenant_id"`
	CreatedAt time.Time      `json:"created_at"`
	Severity  Severity       `json:"severity"`
	EventType EventType      `json:"event_type"`
	SourceID  *uuid.UUID     `json:"source_id"`
	MachineID *uuid.UUID     `json:"machine_id"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
}

// RetentionPolicy defines how long logs are kept for a tenant.
type RetentionPolicy struct {
	TenantID      uuid.UUID `json:"tenant_id"`
	RetentionDays int       `json:"retention_days"`
	UpdatedAt     time.Time `json:"updated_at"`
	NextPurgeAt   time.Time `json:"next_purge_at"`
}

// Domain errors for the log service.
var (
	ErrLogNotFound          = errors.New("log entry not found")
	ErrRetentionNotFound    = errors.New("retention policy not found")
	ErrInvalidCursor        = errors.New("invalid pagination cursor")
	ErrInvalidRetentionDays = errors.New("retention_days must be between 1 and 3650")
)
