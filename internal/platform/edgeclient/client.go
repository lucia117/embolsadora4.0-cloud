package edgeclient

import (
	"context"

	"github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
)

// EdgeDeviceClient defines the interface for communicating with edge devices.
type EdgeDeviceClient interface {
	// StatusCheck performs a connectivity + version check on the device.
	StatusCheck(ctx context.Context, baseURL string) (*edge_devices.CheckResult, error)

	// HealthCheck performs a full hardware diagnostic.
	HealthCheck(ctx context.Context, baseURL string) (*edge_devices.CheckResult, error)

	// GetTelemetry retrieves a live hardware + PLC snapshot.
	GetTelemetry(ctx context.Context, baseURL string) (*edge_devices.TelemetrySnapshot, error)
}
