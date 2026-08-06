package edge_devices

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	"github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform/edgeclient"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// Service implements application business logic for edge devices.
type Service struct {
	repo    edge_devices.Repository
	client  edgeclient.EdgeDeviceClient
	logger  *zap.Logger
	apiKeys domainapikeys.Repository
	redis   *redis.Client
}

// NewService creates a new edge devices service.
func NewService(
	repo edge_devices.Repository,
	client edgeclient.EdgeDeviceClient,
	logger *zap.Logger,
	apiKeys domainapikeys.Repository,
	redisClient *redis.Client,
) *Service {
	return &Service{repo: repo, client: client, logger: logger, apiKeys: apiKeys, redis: redisClient}
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
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to get device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, err
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
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to get device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, err
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
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to enable device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	s.logger.Info("device enabled", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()), zap.String("status", device.Status))
	return device, nil
}

// DisableDevice disables a device (sets status to DISABLED).
func (s *Service) DisableDevice(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.EdgeDevice, error) {
	device, err := s.repo.SetStatus(ctx, tenantID, deviceID, "DISABLED")
	if err != nil {
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to disable device", zap.Error(err), zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	s.logger.Info("device disabled", zap.String("tenant_id", tenantID.String()), zap.String("device_id", deviceID.String()), zap.String("status", device.Status))
	return device, nil
}

// StatusCheck performs a connectivity + version check.
func (s *Service) StatusCheck(ctx context.Context, tenantID, deviceID, userID uuid.UUID, userEmail string) (*edge_devices.CheckResult, error) {
	// Get device
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to get device", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	// Check if device is disabled
	if device.Status == "DISABLED" {
		s.logger.Warn("attempted status check on disabled device", zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceDisabled
	}

	// Call client to perform status check
	result, err := s.client.StatusCheck(ctx, device.RaspberryBaseURL)
	if err != nil || result == nil {
		s.logger.Warn("status check unreachable", zap.Error(err), zap.String("device_id", deviceID.String()))
		summary := "unreachable"
		if err != nil {
			summary = err.Error()
		}
		result = &edge_devices.CheckResult{
			CheckType:     "STATUS",
			CheckedAt:     time.Now(),
			OverallStatus: "ERROR",
			Summary:       &summary,
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
		s.logger.Error("failed to save audit event", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	// Update device health state (derived/cached data — log on failure but don't fail the request)
	summary := ""
	if result.Summary != nil {
		summary = *result.Summary
	}
	if err := s.repo.UpdateHealthState(ctx, tenantID, deviceID, result.OverallStatus, summary); err != nil {
		s.logger.Warn("failed to update health state", zap.Error(err), zap.String("device_id", deviceID.String()))
	}

	s.logger.Info("status check completed", zap.String("device_id", deviceID.String()), zap.String("check_type", "STATUS"), zap.String("overall_status", result.OverallStatus))
	return result, nil
}

// HealthCheck performs a full hardware diagnostic.
func (s *Service) HealthCheck(ctx context.Context, tenantID, deviceID, userID uuid.UUID, userEmail string) (*edge_devices.CheckResult, error) {
	// Get device
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to get device", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	// Check if device is disabled
	if device.Status == "DISABLED" {
		s.logger.Warn("attempted health check on disabled device", zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceDisabled
	}

	// Call client to perform health check
	result, err := s.client.HealthCheck(ctx, device.RaspberryBaseURL)
	if err != nil || result == nil {
		s.logger.Warn("health check unreachable", zap.Error(err), zap.String("device_id", deviceID.String()))
		summary := "unreachable"
		if err != nil {
			summary = err.Error()
		}
		result = &edge_devices.CheckResult{
			CheckType:     "HEALTH_CHECK",
			CheckedAt:     time.Now(),
			OverallStatus: "ERROR",
			Summary:       &summary,
		}
	}
	result.CheckType = "HEALTH_CHECK"

	// Persist event
	event := &edge_devices.DeviceEvent{
		ID:            uuid.New(),
		DeviceID:      deviceID,
		TenantID:      tenantID,
		CheckType:     "HEALTH_CHECK",
		CheckedAt:     result.CheckedAt,
		OverallStatus: result.OverallStatus,
		Summary:       result.Summary,
		Details:       result.Details,
		UserID:        userID,
		UserEmail:     userEmail,
	}

	if err := s.repo.SaveEvent(ctx, event); err != nil {
		s.logger.Error("failed to save audit event", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	// Update device health state (derived/cached data — log on failure but don't fail the request)
	summary := ""
	if result.Summary != nil {
		summary = *result.Summary
	}
	if err := s.repo.UpdateHealthState(ctx, tenantID, deviceID, result.OverallStatus, summary); err != nil {
		s.logger.Warn("failed to update health state", zap.Error(err), zap.String("device_id", deviceID.String()))
	}

	s.logger.Info("health check completed", zap.String("device_id", deviceID.String()), zap.String("check_type", "HEALTH_CHECK"), zap.String("overall_status", result.OverallStatus))
	return result, nil
}

// GetTelemetry retrieves a live telemetry snapshot from a device.
func (s *Service) GetTelemetry(ctx context.Context, tenantID, deviceID uuid.UUID) (*edge_devices.TelemetrySnapshot, error) {
	// Get device
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to get device", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	// Check if device is disabled
	if device.Status == "DISABLED" {
		s.logger.Warn("attempted telemetry retrieval on disabled device", zap.String("device_id", deviceID.String()))
		return nil, edge_devices.ErrDeviceDisabled
	}

	// Call client to retrieve telemetry
	telemetry, err := s.client.GetTelemetry(ctx, device.RaspberryBaseURL)
	if err != nil || telemetry == nil {
		s.logger.Error("failed to retrieve telemetry", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	s.logger.Info("telemetry retrieved", zap.String("device_id", deviceID.String()))
	return telemetry, nil
}

// ListEvents returns all events for a device.
func (s *Service) ListEvents(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*edge_devices.DeviceEvent, error) {
	// Verify device exists in tenant
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		// Only map to 404 if device truly not found
		if errors.Is(err, edge_devices.ErrDeviceNotFound) {
			s.logger.Warn("device not found", zap.String("device_id", deviceID.String()))
			return nil, edge_devices.ErrDeviceNotFound
		}
		// Infrastructure errors should propagate as 500
		s.logger.Error("failed to get device", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	// Get events for device
	events, err := s.repo.ListEvents(ctx, tenantID, deviceID)
	if err != nil {
		s.logger.Error("failed to list events", zap.Error(err), zap.String("device_id", deviceID.String()))
		return nil, err
	}

	s.logger.Info("events listed", zap.String("device_id", deviceID.String()), zap.String("device_name", device.Name), zap.Int("count", len(events)))
	return events, nil
}

// CreateAPIKey genera una credencial nueva para el device y devuelve el valor
// en claro, que el llamador debe mostrar UNA sola vez, junto con el status
// ACTUAL del device ("ACTIVE" | "DISABLED"): el llamador lo necesita para
// completar dto.APIKeyResponse.DeviceStatus sin un segundo roundtrip -- ya se
// pidio el device aca abajo para verificar que existiera.
func (s *Service) CreateAPIKey(ctx context.Context, tenantID, deviceID uuid.UUID, name string, expiresAt *time.Time, createdBy *uuid.UUID) (*domainapikeys.APIKey, string, string, error) {
	// Verificar que el device exista y sea del tenant antes de emitir nada.
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		return nil, "", "", err
	}

	plaintext, keyID, hash, genErr := domainapikeys.Generate()
	if genErr != nil {
		return nil, "", "", genErr
	}

	key := &domainapikeys.APIKey{
		ID:        uuid.New(),
		TenantID:  tenantID,
		DeviceID:  deviceID,
		KeyID:     keyID,
		KeyHash:   hash,
		CreatedAt: time.Now().UTC(),
		CreatedBy: createdBy,
		ExpiresAt: expiresAt,
	}
	if name != "" {
		key.Name = &name
	}

	if err := s.apiKeys.Create(ctx, key); err != nil {
		return nil, "", "", err
	}
	s.logger.Info("api key creada",
		zap.String("device_id", deviceID.String()), zap.String("key_id", keyID))
	return key, plaintext, device.Status, nil
}

// ListAPIKeys devuelve las keys del device, incluidas las revocadas, junto
// con el status ACTUAL del device -- ver el comentario de CreateAPIKey.
func (s *Service) ListAPIKeys(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*domainapikeys.APIKey, string, error) {
	device, err := s.repo.GetByID(ctx, tenantID, deviceID)
	if err != nil {
		return nil, "", err
	}
	keys, err := s.apiKeys.ListByDevice(ctx, tenantID, deviceID)
	if err != nil {
		return nil, "", err
	}
	return keys, device.Status, nil
}

// RevokeAPIKey revoca una key e invalida su entrada de cache.
//
// El orden importa: primero Postgres —la fuente de verdad— y despues la cache.
// Al reves, una escritura concurrente podria recachear la version vieja entre
// medio y la key revocada seguiria autenticando hasta que venza el TTL.
func (s *Service) RevokeAPIKey(ctx context.Context, tenantID, deviceID, keyPK uuid.UUID) error {
	keys, err := s.apiKeys.ListByDevice(ctx, tenantID, deviceID)
	if err != nil {
		return err
	}
	var keyID string
	for _, k := range keys {
		if k.ID == keyPK {
			keyID = k.KeyID
			break
		}
	}
	if keyID == "" {
		return domainapikeys.ErrKeyNotFound
	}

	if err := s.apiKeys.Revoke(ctx, tenantID, keyPK); err != nil {
		return err
	}
	if err := security.InvalidateAPIKeyCache(ctx, s.redis, keyID); err != nil {
		// La revocacion ya es efectiva en Postgres; la cache expira sola por TTL.
		s.logger.Warn("no se pudo invalidar la cache de la api key",
			zap.String("key_id", keyID), zap.Error(err))
	}
	s.logger.Info("api key revocada", zap.String("key_id", keyID))
	return nil
}
