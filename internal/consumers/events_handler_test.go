package consumers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/consumers"
	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// memRepo es un repositorio en memoria con la MISMA semantica de idempotencia
// que Mongo: un eventId repetido se reporta como duplicado y no se guarda dos
// veces. Permite testear el contrato sin infraestructura.
type memRepo struct {
	stored map[string]ingest.Measurement
}

func newMemRepo() *memRepo { return &memRepo{stored: map[string]ingest.Measurement{}} }

func (m *memRepo) InsertMany(_ context.Context, docs []ingest.Measurement) (ingest.InsertReport, error) {
	rep := ingest.InsertReport{Duplicated: map[int]struct{}{}, Failed: map[int]string{}}
	for i, d := range docs {
		if _, dup := m.stored[d.EventID]; dup {
			rep.Duplicated[i] = struct{}{}
			continue
		}
		m.stored[d.EventID] = d
	}
	return rep, nil
}
func (m *memRepo) Ping(context.Context) error { return nil }

// newRouter monta el handler con una identidad ya inyectada, salteando
// APIKeyAuth: lo que se testea aca es el contrato del body, no la auth.
func newRouter(repo ingest.Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	identity := &domainapikeys.DeviceIdentity{
		TenantID:  uuid.New(),
		DeviceID:  uuid.New(),
		MachineID: "EMB-DEV-001",
		KeyID:     "0123456789ab",
	}
	r.POST("/events", func(c *gin.Context) {
		c.Request = c.Request.WithContext(security.WithDeviceIdentity(c.Request.Context(), identity))
		c.Next()
	}, consumers.IngestEvents(
		ingestapp.NewService(repo, zap.NewNop(), 0),
		consumers.HandlerConfig{MaxBodyBytes: 4194304, MaxEvents: 1000},
		zap.NewNop(),
	))
	return r
}

func post(t *testing.T, r *gin.Engine, body []byte) (int, ingest.Result) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		return w.Code, ingest.Result{}
	}
	var resp struct {
		Data ingest.Result `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return w.Code, resp.Data
}

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/last-batch.json")
	require.NoError(t, err, "correr: go run ./scripts/genfixture > internal/consumers/testdata/last-batch.json")
	return b
}

// SC-001: el batch de 108 eventos entra completo.
func TestSC001CleanBatch(t *testing.T) {
	repo := newMemRepo()
	code, res := post(t, newRouter(repo), fixture(t))

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 108, res.Accepted)
	assert.Equal(t, 0, res.Rejected)
	assert.Empty(t, res.Errors)
	assert.Len(t, repo.stored, 108, "deben quedar exactamente 108 documentos")
}

// SC-002: reenviar el MISMO batch no duplica nada. Es la prueba de que la
// idempotencia funciona — el escenario real de un reintento del Pi.
func TestSC002ReplayIsIdempotent(t *testing.T) {
	repo := newMemRepo()
	r := newRouter(repo)
	body := fixture(t)

	_, first := post(t, r, body)
	require.Equal(t, 108, first.Accepted)

	code, second := post(t, r, body)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 0, second.Accepted)
	assert.Equal(t, 108, second.Rejected)
	require.Len(t, second.Errors, 108)
	for _, e := range second.Errors {
		assert.Equal(t, ingest.CodeDuplicate, e.Code)
	}
	assert.Len(t, repo.stored, 108, "sigue habiendo 108 documentos, no 216")
}

// SC-003: [valido, valido, sin ts, valido, ya existente] -> errores en 2 y 4.
// Es el test del invariante I-3, con los indices del array ORIGINAL.
func TestSC003MixedBatchReportsOriginalIndices(t *testing.T) {
	repo := newMemRepo()
	r := newRouter(repo)

	ev := func(id string) string {
		return `{"eventId":"` + id + `","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",` +
			`"kind":"metric","schemaVersion":1,"payload":{"value":1}}`
	}
	// Preexistente, para que el quinto elemento sea un duplicado real.
	_, pre := post(t, r, []byte(`{"events":[`+ev("ya-existente")+`]}`))
	require.Equal(t, 1, pre.Accepted)

	sinTs := `{"eventId":"c","machineId":"EMB-DEV-001","kind":"metric","schemaVersion":1,"payload":{}}`
	body := `{"events":[` + ev("a") + `,` + ev("b") + `,` + sinTs + `,` + ev("d") + `,` + ev("ya-existente") + `]}`

	code, res := post(t, r, []byte(body))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 3, res.Accepted)
	assert.Equal(t, 2, res.Rejected)

	require.Len(t, res.Errors, 2)
	assert.Equal(t, 2, res.Errors[0].Index)
	assert.Equal(t, ingest.CodeInvalidSchema, res.Errors[0].Code)
	assert.Equal(t, 4, res.Errors[1].Index)
	assert.Equal(t, ingest.CodeDuplicate, res.Errors[1].Code)
}

// I-2: los requests rotos a nivel de SOBRE son 400. Estos son los unicos casos
// en que el Edge manda el batch entero a DEAD, asi que la lista es cerrada.
func TestEnvelopeErrorsReturn400(t *testing.T) {
	r := newRouter(newMemRepo())
	cases := map[string]string{
		"json malformado": `{"events":`,
		"sin events":      `{}`,
		"events vacio":    `{"events":[]}`,
		"events null":     `{"events":null}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			code, _ := post(t, r, []byte(body))
			assert.Equal(t, http.StatusBadRequest, code)
		})
	}
}

// Un batch de mas de 1000 eventos es 400: es el limite de negocio del contrato.
func TestTooManyEventsReturns400(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString(`{"events":[`)
	for i := 0; i < 1001; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"eventId":"e%d","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",`+
			`"kind":"metric","schemaVersion":1,"payload":{}}`, i)
	}
	buf.WriteString(`]}`)

	code, _ := post(t, newRouter(newMemRepo()), buf.Bytes())
	assert.Equal(t, http.StatusBadRequest, code)
}

// I-4 sobre el fixture real.
func TestAccountingInvariantOnFixture(t *testing.T) {
	_, res := post(t, newRouter(newMemRepo()), fixture(t))
	assert.Equal(t, 108, res.Accepted+res.Rejected)
}

// La respuesta NO lleva el envelope {"success":true,...} del resto del repo.
// El parser del Edge esta congelado contra {"data":{...}}.
func TestResponseShapeMatchesFrozenContract(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(fixture(t)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newRouter(newMemRepo()).ServeHTTP(w, req)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	assert.Contains(t, raw, "data")
	assert.NotContains(t, raw, "success", "este endpoint no usa el envelope success del repo")

	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["data"], &data))
	assert.Contains(t, data, "accepted")
	assert.Contains(t, data, "rejected")
}
