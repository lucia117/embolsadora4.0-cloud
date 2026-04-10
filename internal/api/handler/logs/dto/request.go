package dto

import "time"

// ListLogsParams holds query parameters for GET /logs.
type ListLogsParams struct {
	EventType string     `form:"event_type"`
	Severity  string     `form:"severity"`
	MachineID string     `form:"machine_id"`
	From      *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	To        *time.Time `form:"to"   time_format:"2006-01-02T15:04:05Z07:00"`
	Q         string     `form:"q"`
	Cursor    string     `form:"cursor"`
	Limit     int        `form:"limit"`
}

// ExportLogsParams holds query parameters for GET /logs/export.
type ExportLogsParams struct {
	EventType string     `form:"event_type"`
	Severity  string     `form:"severity"`
	MachineID string     `form:"machine_id"`
	From      *time.Time `form:"from" time_format:"2006-01-02T15:04:05Z07:00"`
	To        *time.Time `form:"to"   time_format:"2006-01-02T15:04:05Z07:00"`
	Q         string     `form:"q"`
	Format    string     `form:"format"`
}

// UpdateRetentionRequest is the body for PATCH /logs/retention.
type UpdateRetentionRequest struct {
	RetentionDays int `json:"retention_days" binding:"required,min=1,max=3650"`
}

// GetContextParams holds query parameters for GET /logs/:id/context.
type GetContextParams struct {
	WindowSize int `form:"window_size"`
}
