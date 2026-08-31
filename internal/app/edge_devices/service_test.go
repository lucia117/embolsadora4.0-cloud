package edge_devices_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	domain "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
)

// fakeRepo implementa domain.Repository; solo GetByID y Update tienen lógica real.
type fakeRepo struct {
	device      *domain.EdgeDevice
	updateCalls []*domain.EdgeDevice
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.EdgeDevice, error) {
	return nil, nil
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.EdgeDevice, error) {
	if f.device == nil {
		return nil, domain.ErrDeviceNotFound
	}
	// devolver copia para que el service mute la copia, no el original
	cp := *f.device
	return &cp, nil
}
func (f *fakeRepo) Create(context.Context, *domain.EdgeDevice) error { return nil }
func (f *fakeRepo) Update(_ context.Context, d *domain.EdgeDevice) error {
	f.updateCalls = append(f.updateCalls, d)
	f.device = d
	return nil
}
func (f *fakeRepo) SetStatus(context.Context, uuid.UUID, uuid.UUID, string) (*domain.EdgeDevice, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateHealthState(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return nil
}
func (f *fakeRepo) SaveEvent(context.Context, *domain.DeviceEvent) error { return nil }
func (f *fakeRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]*domain.DeviceEvent, error) {
	return nil, nil
}

func strptr(s string) *string { return &s }

func TestUpdateDeviceAplicaRaspberryBaseURLYPLCAddress(t *testing.T) {
	tenantID := uuid.New()
	deviceID := uuid.New()
	repo := &fakeRepo{device: &domain.EdgeDevice{
		ID:               deviceID,
		TenantID:         tenantID,
		Name:             "Edge viejo",
		MachineID:        "machine-1",
		EdgeType:         "RASPBERRY_PLC",
		RaspberryBaseURL: "http://old.local",
		Status:           "ACTIVE",
		LastHealthStatus: "UNKNOWN",
	}}
	svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)

	updated, err := svc.UpdateDevice(context.Background(), tenantID, deviceID, domain.UpdateDeviceCommand{
		RaspberryBaseURL: strptr("http://new.local:8080"),
		PLCAddress:       strptr("192.168.0.50"),
	})

	require.NoError(t, err)
	require.Equal(t, "http://new.local:8080", updated.RaspberryBaseURL)
	require.NotNil(t, updated.PLCAddress)
	require.Equal(t, "192.168.0.50", *updated.PLCAddress)
	require.Equal(t, "machine-1", updated.MachineID, "machineId no debe cambiar")
	require.Len(t, repo.updateCalls, 1)
}

func TestUpdateDeviceSinCamposNoRompe(t *testing.T) {
	tenantID := uuid.New()
	deviceID := uuid.New()
	repo := &fakeRepo{device: &domain.EdgeDevice{
		ID: deviceID, TenantID: tenantID, Name: "Edge", MachineID: "m1",
		EdgeType: "RASPBERRY_PLC", RaspberryBaseURL: "http://x.local", Status: "ACTIVE",
		LastHealthStatus: "UNKNOWN",
	}}
	svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)

	updated, err := svc.UpdateDevice(context.Background(), tenantID, deviceID, domain.UpdateDeviceCommand{
		Name: strptr("Edge nuevo"),
	})

	require.NoError(t, err)
	require.Equal(t, "Edge nuevo", updated.Name)
	require.Equal(t, "http://x.local", updated.RaspberryBaseURL)
}
