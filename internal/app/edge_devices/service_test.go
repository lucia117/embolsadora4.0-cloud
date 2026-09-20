package edge_devices_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	apikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	domain "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
)

type fakeRepo struct {
	device      *domain.EdgeDevice
	updateCalls []*domain.EdgeDevice

	getErr error // distinto de ErrDeviceNotFound, para probar el branch de error de infra

	listResult []*domain.EdgeDevice
	listErr    error

	createErr error

	setStatusResult *domain.EdgeDevice
	setStatusErr    error

	updateErr error

	updateHealthErr   error
	updateHealthCalls []healthCall

	saveEventErr   error
	saveEventCalls []*domain.DeviceEvent

	listEventsResult []*domain.DeviceEvent
	listEventsErr    error
}

type healthCall struct {
	status  string
	summary string
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.EdgeDevice, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.EdgeDevice, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.device == nil {
		return nil, domain.ErrDeviceNotFound
	}
	cp := *f.device
	return &cp, nil
}
func (f *fakeRepo) Create(context.Context, *domain.EdgeDevice) error { return f.createErr }
func (f *fakeRepo) Update(_ context.Context, d *domain.EdgeDevice) error {
	f.updateCalls = append(f.updateCalls, d)
	if f.updateErr != nil {
		return f.updateErr
	}
	f.device = d
	return nil
}
func (f *fakeRepo) SetStatus(context.Context, uuid.UUID, uuid.UUID, string) (*domain.EdgeDevice, error) {
	return f.setStatusResult, f.setStatusErr
}
func (f *fakeRepo) UpdateHealthState(_ context.Context, _ uuid.UUID, _ uuid.UUID, status, summary string) error {
	f.updateHealthCalls = append(f.updateHealthCalls, healthCall{status, summary})
	return f.updateHealthErr
}
func (f *fakeRepo) SaveEvent(_ context.Context, e *domain.DeviceEvent) error {
	f.saveEventCalls = append(f.saveEventCalls, e)
	return f.saveEventErr
}
func (f *fakeRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]*domain.DeviceEvent, error) {
	return f.listEventsResult, f.listEventsErr
}

// fakeClient implementa edgeclient.EdgeDeviceClient (satisfacción estructural,
// no hace falta importar el paquete edgeclient).
type fakeClient struct {
	statusResult *domain.CheckResult
	statusErr    error
	healthResult *domain.CheckResult
	healthErr    error
	telemetryResult *domain.TelemetrySnapshot
	telemetryErr    error
}

func (f *fakeClient) StatusCheck(ctx context.Context, baseURL string) (*domain.CheckResult, error) {
	return f.statusResult, f.statusErr
}
func (f *fakeClient) HealthCheck(ctx context.Context, baseURL string) (*domain.CheckResult, error) {
	return f.healthResult, f.healthErr
}
func (f *fakeClient) GetTelemetry(ctx context.Context, baseURL string) (*domain.TelemetrySnapshot, error) {
	return f.telemetryResult, f.telemetryErr
}

// fakeAPIKeysRepo implementa domainapikeys.Repository.
type fakeAPIKeysRepo struct {
	createErr   error
	createCalls []*apikeys.APIKey

	listResult []*apikeys.APIKey
	listErr    error

	revokeErr   error
	revokeCalls []uuid.UUID
}

func (f *fakeAPIKeysRepo) GetByKeyID(context.Context, string) (*apikeys.Credential, error) {
	return nil, nil
}
func (f *fakeAPIKeysRepo) Create(_ context.Context, k *apikeys.APIKey) error {
	f.createCalls = append(f.createCalls, k)
	return f.createErr
}
func (f *fakeAPIKeysRepo) ListByDevice(context.Context, uuid.UUID, uuid.UUID) ([]*apikeys.APIKey, error) {
	return f.listResult, f.listErr
}
func (f *fakeAPIKeysRepo) Revoke(_ context.Context, _ uuid.UUID, keyPK uuid.UUID) error {
	f.revokeCalls = append(f.revokeCalls, keyPK)
	return f.revokeErr
}
func (f *fakeAPIKeysRepo) TouchLastUsed(context.Context, uuid.UUID) error { return nil }

func TestListDevices(t *testing.T) {
	tests := []struct {
		name    string
		result  []*domain.EdgeDevice
		err     error
	}{
		{"feliz", []*domain.EdgeDevice{{ID: uuid.New()}}, nil},
		{"error de repo", nil, errors.New("db down")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{listResult: tt.result, listErr: tt.err}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			got, err := svc.ListDevices(context.Background(), uuid.New())
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.result, got)
		})
	}
}

func TestGetDevice(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	infraErr := errors.New("conexión perdida")

	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Name: "Edge"}}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		got, err := svc.GetDevice(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.Equal(t, "Edge", got.Name)
	})
	t.Run("no encontrado", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.GetDevice(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})
	t.Run("error de infra propaga sin mapear", func(t *testing.T) {
		repo := &fakeRepo{getErr: infraErr}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.GetDevice(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, infraErr)
		require.NotErrorIs(t, err, domain.ErrDeviceNotFound)
	})
}

func TestCreateDevice(t *testing.T) {
	tenantID := uuid.New()
	t.Run("feliz setea status y health por defecto", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		got, err := svc.CreateDevice(context.Background(), tenantID, domain.CreateDeviceCommand{
			Name: "Nuevo", MachineID: "m1", EdgeType: "RASPBERRY_PLC", RaspberryBaseURL: "http://x.local",
		})
		require.NoError(t, err)
		require.Equal(t, "ACTIVE", got.Status)
		require.Equal(t, "UNKNOWN", got.LastHealthStatus)
		require.Equal(t, tenantID, got.TenantID)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRepo{createErr: errors.New("dup")}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.CreateDevice(context.Background(), tenantID, domain.CreateDeviceCommand{})
		require.Error(t, err)
	})
}

func TestEnableDisableDevice(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	infraErr := errors.New("timeout")

	for _, tc := range []struct {
		name     string
		call     func(*app.Service) (*domain.EdgeDevice, error)
		wantStat string
	}{
		{"enable", func(s *app.Service) (*domain.EdgeDevice, error) {
			return s.EnableDevice(context.Background(), tenantID, deviceID)
		}, "ACTIVE"},
		{"disable", func(s *app.Service) (*domain.EdgeDevice, error) {
			return s.DisableDevice(context.Background(), tenantID, deviceID)
		}, "DISABLED"},
	} {
		t.Run(tc.name+"/feliz", func(t *testing.T) {
			repo := &fakeRepo{setStatusResult: &domain.EdgeDevice{ID: deviceID, Status: tc.wantStat}}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			got, err := tc.call(svc)
			require.NoError(t, err)
			require.Equal(t, tc.wantStat, got.Status)
		})
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			repo := &fakeRepo{setStatusErr: domain.ErrDeviceNotFound}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			_, err := tc.call(svc)
			require.ErrorIs(t, err, domain.ErrDeviceNotFound)
		})
		t.Run(tc.name+"/error_infra", func(t *testing.T) {
			repo := &fakeRepo{setStatusErr: infraErr}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			_, err := tc.call(svc)
			require.ErrorIs(t, err, infraErr)
		})
	}
}

func TestStatusAndHealthCheck(t *testing.T) {
	tenantID, deviceID, userID := uuid.New(), uuid.New(), uuid.New()
	activeDevice := &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "ACTIVE", RaspberryBaseURL: "http://x.local"}
	disabledDevice := &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "DISABLED"}

	calls := []struct {
		name string
		call func(*app.Service, context.Context) (*domain.CheckResult, error)
		setClientErr func(*fakeClient, error)
		setClientResult func(*fakeClient, *domain.CheckResult)
	}{
		{"StatusCheck", func(s *app.Service, ctx context.Context) (*domain.CheckResult, error) {
			return s.StatusCheck(ctx, tenantID, deviceID, userID, "user@x.com")
		}, func(c *fakeClient, e error) { c.statusErr = e }, func(c *fakeClient, r *domain.CheckResult) { c.statusResult = r }},
		{"HealthCheck", func(s *app.Service, ctx context.Context) (*domain.CheckResult, error) {
			return s.HealthCheck(ctx, tenantID, deviceID, userID, "user@x.com")
		}, func(c *fakeClient, e error) { c.healthErr = e }, func(c *fakeClient, r *domain.CheckResult) { c.healthResult = r }},
	}

	for _, tc := range calls {
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			repo := &fakeRepo{}
			svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
			_, err := tc.call(svc, context.Background())
			require.ErrorIs(t, err, domain.ErrDeviceNotFound)
		})
		t.Run(tc.name+"/disabled", func(t *testing.T) {
			repo := &fakeRepo{device: disabledDevice}
			svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
			_, err := tc.call(svc, context.Background())
			require.ErrorIs(t, err, domain.ErrDeviceDisabled)
		})
		t.Run(tc.name+"/client_error_sintetiza_resultado_ERROR", func(t *testing.T) {
			repo := &fakeRepo{device: activeDevice}
			client := &fakeClient{}
			tc.setClientErr(client, errors.New("unreachable"))
			svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
			got, err := tc.call(svc, context.Background())
			require.NoError(t, err, "un client error no falla el request, sintetiza un resultado ERROR")
			require.Equal(t, "ERROR", got.OverallStatus)
			require.Len(t, repo.saveEventCalls, 1, "el resultado sintético también se audita")
		})
		t.Run(tc.name+"/save_event_error_propaga", func(t *testing.T) {
			repo := &fakeRepo{device: activeDevice, saveEventErr: errors.New("insert failed")}
			client := &fakeClient{}
			tc.setClientResult(client, &domain.CheckResult{OverallStatus: "OK"})
			svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
			_, err := tc.call(svc, context.Background())
			require.Error(t, err)
		})
		t.Run(tc.name+"/update_health_state_error_es_solo_warning", func(t *testing.T) {
			repo := &fakeRepo{device: activeDevice, updateHealthErr: errors.New("cache write failed")}
			client := &fakeClient{}
			tc.setClientResult(client, &domain.CheckResult{OverallStatus: "OK"})
			svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
			got, err := tc.call(svc, context.Background())
			require.NoError(t, err, "UpdateHealthState fallando no debe fallar el request")
			require.Equal(t, "OK", got.OverallStatus)
		})
	}
}

func TestGetTelemetry(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	activeDevice := &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "ACTIVE", RaspberryBaseURL: "http://x.local"}

	t.Run("not_found", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
		_, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})
	t.Run("disabled", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "DISABLED"}}
		svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
		_, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceDisabled)
	})
	t.Run("error de client propaga sin sintetizar", func(t *testing.T) {
		repo := &fakeRepo{device: activeDevice}
		client := &fakeClient{telemetryErr: errors.New("unreachable")}
		svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
		got, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.Error(t, err)
		require.Nil(t, got)
	})
	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRepo{device: activeDevice}
		client := &fakeClient{telemetryResult: &domain.TelemetrySnapshot{}}
		svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
		got, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.NotNil(t, got)
	})
}

func TestListEvents(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	t.Run("not_found", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.ListEvents(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID}, listEventsErr: errors.New("db down")}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.ListEvents(context.Background(), tenantID, deviceID)
		require.Error(t, err)
	})
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.DeviceEvent{{ID: uuid.New()}}
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID}, listEventsResult: want}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		got, err := svc.ListEvents(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

func TestCreateAPIKey(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	t.Run("device no existe propaga sin mapear", func(t *testing.T) {
		repo := &fakeRepo{getErr: errors.New("no existe")}
		svc := app.NewService(repo, nil, zap.NewNop(), &fakeAPIKeysRepo{}, nil)
		_, _, _, err := svc.CreateAPIKey(context.Background(), tenantID, deviceID, "k1", nil, nil)
		require.Error(t, err)
	})
	t.Run("error de apiKeys.Create", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "ACTIVE"}}
		apiRepo := &fakeAPIKeysRepo{createErr: errors.New("insert failed")}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		_, _, _, err := svc.CreateAPIKey(context.Background(), tenantID, deviceID, "k1", nil, nil)
		require.Error(t, err)
	})
	t.Run("feliz devuelve plaintext y el status ACTUAL del device", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "ACTIVE"}}
		apiRepo := &fakeAPIKeysRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		key, plaintext, status, err := svc.CreateAPIKey(context.Background(), tenantID, deviceID, "k1", nil, nil)
		require.NoError(t, err)
		require.NotEmpty(t, plaintext)
		require.NotNil(t, key)
		require.Equal(t, "ACTIVE", status)
		require.Len(t, apiRepo.createCalls, 1)
	})
}

func TestListAPIKeys(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	t.Run("error de GetByID", func(t *testing.T) {
		repo := &fakeRepo{getErr: errors.New("boom")}
		svc := app.NewService(repo, nil, zap.NewNop(), &fakeAPIKeysRepo{}, nil)
		_, _, err := svc.ListAPIKeys(context.Background(), tenantID, deviceID)
		require.Error(t, err)
	})
	t.Run("error de ListByDevice", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "ACTIVE"}}
		apiRepo := &fakeAPIKeysRepo{listErr: errors.New("db down")}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		_, _, err := svc.ListAPIKeys(context.Background(), tenantID, deviceID)
		require.Error(t, err)
	})
	t.Run("feliz devuelve status del device", func(t *testing.T) {
		want := []*apikeys.APIKey{{ID: uuid.New()}}
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "DISABLED"}}
		apiRepo := &fakeAPIKeysRepo{listResult: want}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		keys, status, err := svc.ListAPIKeys(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.Equal(t, want, keys)
		require.Equal(t, "DISABLED", status)
	})
}

func TestRevokeAPIKey(t *testing.T) {
	tenantID, deviceID, keyPK := uuid.New(), uuid.New(), uuid.New()
	t.Run("key no encontrada en el device", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listResult: []*apikeys.APIKey{{ID: uuid.New()}}} // otro id
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.ErrorIs(t, err, apikeys.ErrKeyNotFound)
	})
	t.Run("error de ListByDevice", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listErr: errors.New("db down")}
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.Error(t, err)
	})
	t.Run("error de Revoke", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listResult: []*apikeys.APIKey{{ID: keyPK, KeyID: "kid1"}}, revokeErr: errors.New("db down")}
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.Error(t, err)
	})
	t.Run("feliz con redis nil no falla (nil-safety)", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listResult: []*apikeys.APIKey{{ID: keyPK, KeyID: "kid1"}}}
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.NoError(t, err)
		require.Len(t, apiRepo.revokeCalls, 1)
		require.Equal(t, keyPK, apiRepo.revokeCalls[0])
	})
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
