package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

// LogResponse is the JSON representation of a single log entry.
type LogResponse struct {
	ID        uuid.UUID      `json:"id"`
	TenantID  uuid.UUID      `json:"tenant_id"`
	CreatedAt time.Time      `json:"created_at"`
	Severity  string         `json:"severity"`
	EventType string         `json:"event_type"`
	SourceID  *uuid.UUID     `json:"source_id"`
	MachineID *uuid.UUID     `json:"machine_id"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata"`
}

// LogListResponse is the paginated response for GET /logs.
type LogListResponse struct {
	Data       []LogResponse `json:"data"`
	NextCursor *string       `json:"next_cursor"`
	Total      int           `json:"total"`
}

// LogExportResponse is the response for GET /logs/export.
type LogExportResponse struct {
	Data           []LogResponse `json:"data"`
	Truncated      bool          `json:"truncated"`
	ExportedCount  int           `json:"exported_count"`
	TotalAvailable int           `json:"total_available"`
}

// LogContextResponse is the response for GET /logs/:id/context.
type LogContextResponse struct {
	Before []LogResponse `json:"before"`
	Anchor LogResponse   `json:"anchor"`
	After  []LogResponse `json:"after"`
}

// RetentionResponse is the response for GET/PATCH /logs/retention.
type RetentionResponse struct {
	TenantID      uuid.UUID `json:"tenant_id"`
	RetentionDays int       `json:"retention_days"`
	NextPurgeAt   time.Time `json:"next_purge_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToLogResponse converts a domain.LogEntry to its DTO.
func ToLogResponse(e domain.LogEntry) LogResponse {
	return LogResponse{
		ID:        e.ID,
		TenantID:  e.TenantID,
		CreatedAt: e.CreatedAt,
		Severity:  string(e.Severity),
		EventType: string(e.EventType),
		SourceID:  e.SourceID,
		MachineID: e.MachineID,
		Message:   e.Message,
		Metadata:  e.Metadata,
	}
}

// ToRetentionResponse converts a domain.RetentionPolicy to its DTO.
func ToRetentionResponse(p domain.RetentionPolicy) RetentionResponse {
	return RetentionResponse{
		TenantID:      p.TenantID,
		RetentionDays: p.RetentionDays,
		NextPurgeAt:   p.NextPurgeAt,
		UpdatedAt:     p.UpdatedAt,
	}
}
