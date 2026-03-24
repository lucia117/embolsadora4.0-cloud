package edge_devices

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence interface for edge devices and events.
type Repository interface {
	// List returns all edge devices for a tenant.
	List(ctx context.Context, tenantID uuid.UUID) ([]*EdgeDevice, error)

	// GetByID returns a single device by ID for a tenant.
	GetByID(ctx context.Context, tenantID, deviceID uuid.UUID) (*EdgeDevice, error)

	// Create persists a new edge device.
	Create(ctx context.Context, device *EdgeDevice) error

	// Update persists changes to an existing device.
	Update(ctx context.Context, device *EdgeDevice) error

	// SetStatus updates the device status and returns the updated device.
	SetStatus(ctx context.Context, tenantID, deviceID uuid.UUID, status string) (*EdgeDevice, error)

	// UpdateHealthState updates the device's health check metadata.
	UpdateHealthState(ctx context.Context, tenantID, deviceID uuid.UUID, status, summary string) error

	// SaveEvent persists an immutable device event.
	SaveEvent(ctx context.Context, event *DeviceEvent) error

	// ListEvents returns all events for a device, ordered newest-first.
	ListEvents(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*DeviceEvent, error)
}
