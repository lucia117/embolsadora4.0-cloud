package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
)

// EdgeDeviceResponse represents an edge device in JSON (camelCase).
type EdgeDeviceResponse struct {
	ID                 uuid.UUID              `json:"id"`
	TenantID           uuid.UUID              `json:"tenantId"`
	Name               string                 `json:"name"`
	Description        *string                `json:"description"`
	MachineID          string                 `json:"machineId"`
	EdgeType           string                 `json:"edgeType"`
	RaspberryBaseURL   string                 `json:"raspberryBaseUrl"`
	PLCAddress         *string                `json:"plcAddress"`
	Status             string                 `json:"status"`
	LastSeenAt         *time.Time             `json:"lastSeenAt"`
	LastHealthCheckAt  *time.Time             `json:"lastHealthCheckAt"`
	LastHealthStatus   string                 `json:"lastHealthStatus"`
	LastHealthSummary  *string                `json:"lastHealthSummary"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// CreateDeviceRequest is the request body for creating a device.
type CreateDeviceRequest struct {
	Name             string  `json:"name" binding:"required"`
	MachineID        string  `json:"machineId" binding:"required"`
	EdgeType         string  `json:"edgeType" binding:"required"`
	RaspberryBaseURL string  `json:"raspberryBaseUrl" binding:"required"`
	Description      *string `json:"description"`
	PLCAddress       *string `json:"plcAddress"`
}

// UpdateDeviceRequest is the request body for updating a device.
type UpdateDeviceRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// CheckResultResponse represents a check result in JSON.
type CheckResultResponse struct {
	CheckType     string                 `json:"checkType"`
	CheckedAt     time.Time              `json:"checkedAt"`
	OverallStatus string                 `json:"overallStatus"`
	Summary       *string                `json:"summary"`
	Details       map[string]interface{} `json:"details"`
}

// TelemetryResponse represents telemetry in JSON.
type TelemetryResponse struct {
	CapturedAt        time.Time          `json:"capturedAt"`
	CPU               *float64           `json:"cpu"`
	RAM               *float64           `json:"ram"`
	Disk              *float64           `json:"disk"`
	TemperatureCelsius *float64           `json:"temperatureCelsius"`
	UptimeSeconds     *int64             `json:"uptimeSeconds"`
	PLC               *PLCSnapshotResponse `json:"plc"`
}

// PLCSnapshotResponse represents PLC state in JSON.
type PLCSnapshotResponse struct {
	Reachable bool    `json:"reachable"`
	Address   *string `json:"address"`
}

// DeviceEventResponse represents a device event in JSON.
type DeviceEventResponse struct {
	ID            uuid.UUID              `json:"id"`
	DeviceID      uuid.UUID              `json:"deviceId"`
	TenantID      uuid.UUID              `json:"tenantId"`
	CheckType     string                 `json:"checkType"`
	CheckedAt     time.Time              `json:"checkedAt"`
	OverallStatus string                 `json:"overallStatus"`
	Summary       *string                `json:"summary"`
	Details       map[string]interface{} `json:"details"`
	UserID        uuid.UUID              `json:"userId"`
	UserEmail     string                 `json:"userEmail"`
}

// SuccessResponse is a generic success response envelope.
type SuccessResponse[T any] struct {
	Success bool   `json:"success"`
	Data    T      `json:"data"`
	Error   string `json:"error,omitempty"`
}

// ErrorResponse is a generic error response envelope.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// Mapping functions

// EdgeDeviceToResponse converts a domain EdgeDevice to a response DTO.
func EdgeDeviceToResponse(device *edge_devices.EdgeDevice) EdgeDeviceResponse {
	return EdgeDeviceResponse{
		ID:                 device.ID,
		TenantID:           device.TenantID,
		Name:               device.Name,
		Description:        device.Description,
		MachineID:          device.MachineID,
		EdgeType:           device.EdgeType,
		RaspberryBaseURL:   device.RaspberryBaseURL,
		PLCAddress:         device.PLCAddress,
		Status:             device.Status,
		LastSeenAt:         device.LastSeenAt,
		LastHealthCheckAt:  device.LastHealthCheckAt,
		LastHealthStatus:   device.LastHealthStatus,
		LastHealthSummary:  device.LastHealthSummary,
		CreatedAt:          device.CreatedAt,
		UpdatedAt:          device.UpdatedAt,
	}
}

// CheckResultToResponse converts a domain CheckResult to a response DTO.
func CheckResultToResponse(result *edge_devices.CheckResult) CheckResultResponse {
	return CheckResultResponse{
		CheckType:     result.CheckType,
		CheckedAt:     result.CheckedAt,
		OverallStatus: result.OverallStatus,
		Summary:       result.Summary,
		Details:       result.Details,
	}
}

// TelemetryToResponse converts a domain TelemetrySnapshot to a response DTO.
func TelemetryToResponse(snapshot *edge_devices.TelemetrySnapshot) TelemetryResponse {
	var plcResp *PLCSnapshotResponse
	if snapshot.PLC != nil {
		plcResp = &PLCSnapshotResponse{
			Reachable: snapshot.PLC.Reachable,
			Address:   snapshot.PLC.Address,
		}
	}
	return TelemetryResponse{
		CapturedAt:         snapshot.CapturedAt,
		CPU:                snapshot.CPU,
		RAM:                snapshot.RAM,
		Disk:               snapshot.Disk,
		TemperatureCelsius: snapshot.TemperatureCelsius,
		UptimeSeconds:      snapshot.UptimeSeconds,
		PLC:                plcResp,
	}
}

// DeviceEventToResponse converts a domain DeviceEvent to a response DTO.
func DeviceEventToResponse(event *edge_devices.DeviceEvent) DeviceEventResponse {
	return DeviceEventResponse{
		ID:            event.ID,
		DeviceID:      event.DeviceID,
		TenantID:      event.TenantID,
		CheckType:     event.CheckType,
		CheckedAt:     event.CheckedAt,
		OverallStatus: event.OverallStatus,
		Summary:       event.Summary,
		Details:       event.Details,
		UserID:        event.UserID,
		UserEmail:     event.UserEmail,
	}
}
