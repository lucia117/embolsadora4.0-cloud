package edge_devices

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform/edgeclient"
)

// Service implements application business logic for edge devices.
type Service struct {
	repo   edge_devices.Repository
	client edgeclient.EdgeDeviceClient
	logger *zap.Logger
}

// NewService creates a new edge devices service.
func NewService(repo edge_devices.Repository, client edgeclient.EdgeDeviceClient, logger *zap.Logger) *Service {
	return &Service{
		repo:   repo,
		client: client,
		logger: logger,
	}
}

// ListDevices returns all devices for a tenant.
func (s *Service) ListDevices(ctx context.Context, tenantID uuid.UUID) ([]*edge_devices.EdgeDevice, error) {
	devices, err := s.repo.List(ctx, tenantID)
	if err != nil {
		s.logger.Error("failed to list devices", zap.Error(err), zap.String("tenant_id", tenantID.String()))
		return nil, err
	}

	s.logger.Info("devices listed", zap.String("tenant_id", tenantID.String()), zap.Int("count", len(devices)))
	return devices, nil
}

// GetDevice returns a single device by ID.
func (s *Service) GetDevice(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.EdgeDevice, error) {
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		s.logger.Error("failed to get device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceNotFound
	}

	s.logger.Info("device retrieved", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
	return device, nil
}

// CreateDevice creates a new device.
func (s *Service) CreateDevice(ctx context.Context, tenantID uuid.UUID, cmd edge_devices.CreateDeviceCommand) (*edge_devices.EdgeDevice, error) {
	device := &edge_devices.EdgeDevice{
		ID:               uuid.New(),
		TenantID:         tenantID,
		Name:             cmd.Name,
		MachineID:        cmd.MachineID,
		EdgeType:         cmd.EdgeType,
		RaspberryBaseURL: cmd.RaspberryBaseURL,
		Description:      cmd.Description,
		PLCAddress:       cmd.PLCAddress,
		Status:           "ACTIVE",
		LastHealthStatus: "UNKNOWN",
	}

	err := s.repo.Create(ctx, device)
	if err != nil {
		s.logger.Error("failed to create device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("machine_id", cmd.MachineID))
		return nil, err
	}

	s.logger.Info("device created", zap.String("tenant_id", tenantID.String()), zap.String("machine_id", cmd.MachineID), zap.String("device_id", device.ID.String()))
	return device, nil
}

// UpdateDevice updates an existing device (name, description).
func (s *Service) UpdateDevice(ctx context.Context, tenantID, deviceID uuid.UUID, cmd edge_devices.UpdateDeviceCommand) (*edge_devices.EdgeDevice, error) {
	// Get current device
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		s.logger.Error("failed to get device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceNotFound
	}

	// Update mutable fields
	if cmd.Name != nil {
		device.Name = *cmd.Name
	}
	if cmd.Description != nil {
		device.Description = cmd.Description
	}

	// Persist update
	if err := s.repo.Update(ctx, device); err != nil {
		s.logger.Error("failed to update device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	s.logger.Info("device updated", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()), zap.String("name", device.Name))
	return device, nil
}

// EnableDevice enables a device (sets status to ACTIVE).
func (s *Service) EnableDevice(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.EdgeDevice, error) {
	device, err := s.repo.SetStatus(ctx, tenantID, deviceID, "ACTIVE")
	if err != nil {
		s.logger.Error("failed to enable device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceNotFound
	}

	s.logger.Info("device enabled", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()), zap.String("status", device.Status))
	return device, nil
}

// DisableDevice disables a device (sets status to DISABLED).
func (s *Service) DisableDevice(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.EdgeDevice, error) {
	device, err := s.repo.SetStatus(ctx, tenantID, deviceID, "DISABLED")
	if err != nil {
		s.logger.Error("failed to disable device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceNotFound
	}

	s.logger.Info("device disabled", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()), zap.String("status", device.Status))
	return device, nil
}

// StatusCheck performs a connectivity + version check.
func (s *Service) StatusCheck(ctx context.Context, tenantID, deviceID, userID uuid.UUID, userEmail string) (*edge_devices.CheckResult, error) {
	// Get device
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		s.logger.Error("device not found", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceNotFound
	}

	// Check if device is disabled
	if device.Status == "DISABLED" {
		s.logger.Warn("attempted status check on disabled device", zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceDisabled
	}

	// Call client to perform status check
	result, err := s.client.StatusCheck(ctx, device.RaspberryBaseURL)
	if err != nil || result == nil {
		result = &edge_devices.CheckResult{
			CheckType:     "STATUS",
			OverallStatus: "ERROR",
		}
	}
	result.CheckType = "STATUS"

	// Persist event
	event := &edge_devices.DeviceEvent{
		ID:            uuid.New(),
		DeviceID:      deviceID,
		TenantID:      tenantID,
		CheckType:     "STATUS",
		CheckedAt:     result.CheckedAt,
		OverallStatus: result.OverallStatus,
		Summary:       result.Summary,
		Details:       result.Details,
		UserID:        userID,
		UserEmail:     userEmail,
	}

	if err := s.repo.SaveEvent(ctx, event); err != nil {
		s.logger.Error("failed to save event", zap.Error(err), zap.String("device_id", deviceID.String()))
	}

	// Update device health state
	summary := ""
	if result.Summary != nil {
		summary = *result.Summary
	}
	if err := s.repo.UpdateHealthState(ctx, tenantID, deviceID, result.OverallStatus, summary); err != nil {
		s.logger.Error("failed to update health state", zap.Error(err), zap.String("device_id", deviceID.String()))
	}

	s.logger.Info("status check completed", zap.String("device_id", deviceID.String()), zap.String("check_type", "STATUS"), zap.String("overall_status", result.OverallStatus))
	return result, nil
}

// HealthCheck performs a full hardware diagnostic.
// Stub — implementation added per user story phase.
func (s *Service) HealthCheck(ctx context.Context, tenantID, deviceID, userID uuid.UUID, userEmail string) (*edge_devices.CheckResult, error) {
	return nil, nil
}

// GetTelemetry retrieves a live telemetry snapshot.
// Stub — implementation added per user story phase.
func (s *Service) GetTelemetry(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.TelemetrySnapshot, error) {
	return nil, nil
}

// ListEvents returns all events for a device.
// Stub — implementation added per user story phase.
func (s *Service) ListEvents(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*edge_devices.DeviceEvent, error) {
	return nil, nil
}
