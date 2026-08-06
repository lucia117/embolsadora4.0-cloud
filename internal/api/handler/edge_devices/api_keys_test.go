package edge_devices_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	edge_devices "github.com/tu-org/embolsadora-api/internal/api/handler/edge_devices"
	appedge "github.com/tu-org/embolsadora-api/internal/app/edge_devices"
	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	domainedge "github.com/tu-org/embolsadora-api/internal/domain/edge_devices"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

// deviceNotFoundRepo implementa domainedge.Repository devolviendo siempre
// ErrDeviceNotFound desde GetByID. Alcanza para estos tests: lo unico que
// importa es que el handler NUNCA llegue a llamarlo cuando el body es
// malformado (400 antes de tocar el service), y que cuando SI lo llama (body
// vacio, valido) el fallo sea un error de dominio normal (404), no un panic
// por interfaz nil.
type deviceNotFoundRepo struct{ domainedge.Repository }

func (deviceNotFoundRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domainedge.EdgeDevice, error) {
	return nil, domainedge.ErrDeviceNotFound
}

// noopAPIKeysRepo implementa domainapikeys.Repository sin persistir nada: no
// deberia llegar a usarse en ningun caso de este archivo, porque
// deviceNotFoundRepo ya corta el flujo antes.
type noopAPIKeysRepo struct{ domainapikeys.Repository }

func testService() *appedge.Service {
	return appedge.NewService(deviceNotFoundRepo{}, nil, zap.NewNop(), noopAPIKeysRepo{}, nil)
}

func newCreateAPIKeyRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	tenantID := uuid.New()
	deviceID := uuid.New()
	r.POST("/devices/:deviceId/api-keys", func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantUUID(c.Request.Context(), tenantID))
		c.Params = gin.Params{{Key: "deviceId", Value: deviceID.String()}}
		edge_devices.CreateAPIKey(testService())(c)
	})
	return r
}

func doCreateAPIKeyRequest(r *gin.Engine, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(http.MethodPost, "/devices/x/api-keys", nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, "/devices/x/api-keys", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Un body vacio (io.EOF) es el caso documentado como valido: una key sin
// nombre ni vencimiento. No debe ser 400. deviceNotFoundRepo hace que el
// siguiente paso (buscar el device) falle con 404 — lo que importa aca es
// que NO haya sido 400 por decode.
func TestCreateAPIKeyEmptyBodyIsNotBadRequest(t *testing.T) {
	r := newCreateAPIKeyRouter()
	w := doCreateAPIKeyRequest(r, "")
	assert.NotEqual(t, http.StatusBadRequest, w.Code, "un body vacio es un caso valido, no un decode error")
	assert.Equal(t, http.StatusNotFound, w.Code, "el body vacio paso el decode; lo que sigue fallando es la busqueda del device (fake)")
}

// El caso critico del fix: un body presente pero MAL FORMADO -JSON invalido,
// o un tipo que no matchea el DTO- ya NO debe colapsarse silenciosamente al
// default (lo que antes producia una key sin nombre y SIN vencimiento,
// devuelta 201 con el secreto en claro). Debe ser 400.
func TestCreateAPIKeyMalformedBodyReturns400(t *testing.T) {
	cases := map[string]string{
		"json roto":                `{"name":`,
		"expiresAt no parseable":   `{"name":"pi-planta-2","expiresAt":"2027-13-45"}`,
		"name con tipo equivocado": `{"name":123}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			r := newCreateAPIKeyRouter()
			w := doCreateAPIKeyRequest(r, body)
			require.Equal(t, http.StatusBadRequest, w.Code,
				"un body malformado no puede convertirse en una key sin nombre y sin vencimiento")
		})
	}
}

// disabledDeviceRepo devuelve un device DISABLED valido, para el camino feliz
// de CreateAPIKey.
type disabledDeviceRepo struct{ domainedge.Repository }

func (disabledDeviceRepo) GetByID(_ context.Context, tenantID, deviceID uuid.UUID) (*domainedge.EdgeDevice, error) {
	return &domainedge.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "DISABLED"}, nil
}

// recordingAPIKeysRepo implementa Create sin persistir de verdad, solo para
// que CreateAPIKey pueda completar el camino feliz.
type recordingAPIKeysRepo struct{ domainapikeys.Repository }

func (recordingAPIKeysRepo) Create(context.Context, *domainapikeys.APIKey) error { return nil }

// Item 7 de la review final: Active (domainapikeys.APIKey.IsActive) solo
// responde si la key en si no esta revocada ni vencida, pero el
// autenticador ADEMAS exige que el device este ACTIVE. Antes de este fix,
// una key nominalmente activa en un device DISABLED se veia identica en la
// respuesta a una key que autentica de verdad. Este test crea la key sobre
// un device DISABLED y verifica que DeviceStatus lo refleje, sin tocar
// Active (que sigue respondiendo solo por la key).
func TestCreateAPIKeyResponseSurfacesDeviceStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	tenantID := uuid.New()
	deviceID := uuid.New()
	svc := appedge.NewService(disabledDeviceRepo{}, nil, zap.NewNop(), recordingAPIKeysRepo{}, nil)
	r.POST("/devices/:deviceId/api-keys", func(c *gin.Context) {
		c.Request = c.Request.WithContext(platform.WithTenantUUID(c.Request.Context(), tenantID))
		c.Params = gin.Params{{Key: "deviceId", Value: deviceID.String()}}
		edge_devices.CreateAPIKey(svc)(c)
	})

	w := doCreateAPIKeyRequest(r, "")
	require.Equal(t, http.StatusCreated, w.Code)

	var body struct {
		Data struct {
			Active       bool   `json:"active"`
			DeviceStatus string `json:"deviceStatus"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Data.Active, "la key recien creada no esta revocada ni vencida: Active sigue respondiendo eso")
	assert.Equal(t, "DISABLED", body.Data.DeviceStatus,
		"DeviceStatus debe reflejar el device real, distinto de si la key en si esta activa")
}
