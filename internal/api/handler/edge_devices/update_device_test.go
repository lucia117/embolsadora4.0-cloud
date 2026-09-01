package edge_devices_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices"
	appEdgeDevices "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	domain "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// minimal fakeRepo stub — see also internal/app/edge_devices/service_test.go
// (mismo package de test pero distinto directorio → no se comparte; la
// duplicación entre paquetes de test es aceptable).
type handlerFakeRepo struct {
	device      *domain.EdgeDevice
	updateCalls []*domain.EdgeDevice
}

func (f *handlerFakeRepo) List(context.Context, uuid.UUID) ([]*domain.EdgeDevice, error) {
	return nil, nil
}
func (f *handlerFakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.EdgeDevice, error) {
	if f.device == nil {
		return nil, domain.ErrDeviceNotFound
	}
	cp := *f.device
	return &cp, nil
}
func (f *handlerFakeRepo) Create(context.Context, *domain.EdgeDevice) error { return nil }
func (f *handlerFakeRepo) Update(_ context.Context, d *domain.EdgeDevice) error {
	f.updateCalls = append(f.updateCalls, d)
	f.device = d
	return nil
}
func (f *handlerFakeRepo) SetStatus(context.Context, uuid.UUID, uuid.UUID, string) (*domain.EdgeDevice, error) {
	return nil, nil
}
func (f *handlerFakeRepo) UpdateHealthState(context.Context, uuid.UUID, uuid.UUID, string, string) error {
	return nil
}
func (f *handlerFakeRepo) SaveEvent(context.Context, *domain.DeviceEvent) error { return nil }
func (f *handlerFakeRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]*domain.DeviceEvent, error) {
	return nil, nil
}

// newUpdateDeviceTestRouter monta las rutas reales detrás de un middleware que
// inyecta un RoleContext con perm_edge_devices_manage y un tenant UUID, para
// probar el handler UpdateDevice de punta a punta con un repo fake.
func newUpdateDeviceTestRouter(repo domain.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		ctx := security.WithRoleContext(c.Request.Context(), security.RoleContext{
			Name:        "t",
			Permissions: []string{"perm_edge_devices_manage"},
		})
		ctx = platform.WithTenantUUID(ctx, uuid.New())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	svc := appEdgeDevices.NewService(repo, nil, zap.NewNop(), nil, nil)
	group := r.Group("")
	edge_devices.RegisterRoutes(group, group, svc)
	return r
}

func TestUpdateDeviceHandlerRechazaURLInvalida(t *testing.T) {
	repo := &handlerFakeRepo{device: &domain.EdgeDevice{
		ID: uuid.New(), Name: "E", MachineID: "m", EdgeType: "RASPBERRY_PLC",
		RaspberryBaseURL: "http://x", Status: "ACTIVE", LastHealthStatus: "UNKNOWN",
	}}
	r := newUpdateDeviceTestRouter(repo)

	body := `{"raspberryBaseUrl":"not-a-url"}`
	req := httptest.NewRequest(http.MethodPut, "/edge-devices/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "VALIDATION_ERROR")
	require.Empty(t, repo.updateCalls, "una URL inválida no debe llegar a persistirse")
}

func TestUpdateDeviceHandlerRechazaURLSoloEsquema(t *testing.T) {
	repo := &handlerFakeRepo{device: &domain.EdgeDevice{
		ID: uuid.New(), Name: "E", MachineID: "m", EdgeType: "RASPBERRY_PLC",
		RaspberryBaseURL: "http://x", Status: "ACTIVE", LastHealthStatus: "UNKNOWN",
	}}
	r := newUpdateDeviceTestRouter(repo)

	body := `{"raspberryBaseUrl":"http://"}`
	req := httptest.NewRequest(http.MethodPut, "/edge-devices/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "VALIDATION_ERROR")
	require.Empty(t, repo.updateCalls, "una URL sin host no debe llegar a persistirse")
}

func TestUpdateDeviceHandlerRechazaNombreVacio(t *testing.T) {
	repo := &handlerFakeRepo{device: &domain.EdgeDevice{
		ID: uuid.New(), Name: "E", MachineID: "m", EdgeType: "RASPBERRY_PLC",
		RaspberryBaseURL: "http://x", Status: "ACTIVE", LastHealthStatus: "UNKNOWN",
	}}
	r := newUpdateDeviceTestRouter(repo)

	body := `{"name":"   "}`
	req := httptest.NewRequest(http.MethodPut, "/edge-devices/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "VALIDATION_ERROR")
	require.Empty(t, repo.updateCalls, "un nombre en blanco no debe llegar a persistirse")
}

func TestUpdateDeviceHandlerIgnoraCamposInmutables(t *testing.T) {
	repo := &handlerFakeRepo{device: &domain.EdgeDevice{
		ID: uuid.New(), Name: "E", MachineID: "m1", EdgeType: "RASPBERRY_PLC",
		RaspberryBaseURL: "http://x", Status: "ACTIVE", LastHealthStatus: "UNKNOWN",
	}}
	r := newUpdateDeviceTestRouter(repo)

	body := `{"name":"Nuevo","machineId":"hacked","edgeType":"OTRO"}`
	req := httptest.NewRequest(http.MethodPut, "/edge-devices/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"machineId":"m1"`)
	require.Contains(t, w.Body.String(), `"edgeType":"RASPBERRY_PLC"`)
	require.NotContains(t, w.Body.String(), "hacked")
}

func TestUpdateDeviceHandlerPersisteCamposNuevos(t *testing.T) {
	repo := &handlerFakeRepo{device: &domain.EdgeDevice{
		ID: uuid.New(), Name: "E", MachineID: "m", EdgeType: "RASPBERRY_PLC",
		RaspberryBaseURL: "http://old.local", Status: "ACTIVE", LastHealthStatus: "UNKNOWN",
	}}
	r := newUpdateDeviceTestRouter(repo)

	body := `{"raspberryBaseUrl":"http://new.local:8080","plcAddress":"192.168.0.50"}`
	req := httptest.NewRequest(http.MethodPut, "/edge-devices/"+uuid.NewString(), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"raspberryBaseUrl":"http://new.local:8080"`)
	require.Contains(t, w.Body.String(), `"plcAddress":"192.168.0.50"`)
	require.Len(t, repo.updateCalls, 1)
}
