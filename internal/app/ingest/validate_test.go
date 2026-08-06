package ingest_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

var dev = ingest.DeviceContext{
	TenantID:  "11111111-1111-1111-1111-111111111111",
	DeviceID:  "22222222-2222-2222-2222-222222222222",
	MachineID: "EMB-DEV-001",
}

const validEvent = `{
	"eventId": "c2a7a6a240e21113474b88c5352cec7ddbb3e942b29651b06173371afc09186f",
	"machineId": "EMB-DEV-001",
	"ts": "2026-07-31T01:06:37.147166Z",
	"seq": 6,
	"kind": "metric",
	"schemaVersion": 1,
	"payload": {"aasPath":"Operativos/Pesada/peso","value":1,"unit":"kg","valueType":"xs:float"}
}`

func TestValidateEventHappyPath(t *testing.T) {
	now := time.Now().UTC()
	m, evErr, skew := ingestapp.ValidateEvent(json.RawMessage(validEvent), dev, now)
	require.Nil(t, evErr)
	assert.Equal(t, ingest.SkewReasonNone, skew, "un evento normal no tiene version skew")

	assert.Equal(t, "c2a7a6a240e21113474b88c5352cec7ddbb3e942b29651b06173371afc09186f", m.EventID)
	assert.Equal(t, dev.TenantID, m.TenantID, "tenantId sale de la API key, no del body")
	assert.Equal(t, dev.DeviceID, m.DeviceID)
	assert.Equal(t, "EMB-DEV-001", m.MachineID)
	assert.Equal(t, 2026, m.Ts.Year())
	require.NotNil(t, m.Seq)
	assert.EqualValues(t, 6, *m.Seq)
	assert.Equal(t, ingest.KindMetric, m.Kind)
	assert.Equal(t, 1, m.SchemaVersion)
	assert.Equal(t, now, m.ReceivedAt)
	assert.Equal(t, "Operativos/Pesada/peso", m.Payload["aasPath"])
}

// seq es opcional: el primer evento de cada instante no lo trae.
func TestValidateEventWithoutSeq(t *testing.T) {
	doc := `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
	         "kind":"metric","schemaVersion":1,"payload":{"value":1}}`
	m, evErr, _ := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
	require.Nil(t, evErr)
	assert.Nil(t, m.Seq)
}

// El contenido de payload NO se valida jamas (D-8). Un payload vacio, con claves
// desconocidas o con tipos raros se persiste igual: validarlo haria que un
// cambio del catalogo AAS mandara datos reales a DEAD.
func TestValidateEventNeverValidatesPayloadContents(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"clave-desconocida":"lo que sea"}`,
		`{"value":null}`,
		`{"anidado":{"profundo":[1,2,{"x":true}]}}`,
	} {
		doc := `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
		         "kind":"metric","schemaVersion":1,"payload":` + payload + `}`
		_, evErr, _ := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
		assert.Nil(t, evErr, "payload %s no deberia rechazarse", payload)
	}
}

func TestValidateEventInvalidSchema(t *testing.T) {
	cases := map[string]string{
		"json roto":            `{"eventId":`,
		"no es objeto":         `"soy un string"`,
		"sin eventId":          `{"machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"eventId vacio":        `{"eventId":"","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"sin machineId":        `{"eventId":"e1","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
		"sin ts":               `{"eventId":"e1","machineId":"EMB-DEV-001","kind":"metric","schemaVersion":1,"payload":{}}`,
		"ts no RFC3339":        `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"31/07/2026","kind":"metric","schemaVersion":1,"payload":{}}`,
		"ts numerico":          `{"eventId":"e1","machineId":"EMB-DEV-001","ts":1753923997,"kind":"metric","schemaVersion":1,"payload":{}}`,
		"sin kind":             `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","schemaVersion":1,"payload":{}}`,
		"sin schemaVersion":    `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","payload":{}}`,
		"schemaVersion 0":      `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":0,"payload":{}}`,
		"schemaVersion string": `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":"1","payload":{}}`,
		"schemaVersion float":  `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1.5,"payload":{}}`,
		"sin payload":          `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1}`,
		"payload es array":     `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":[1,2]}`,
		"payload es string":    `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":"x"}`,
		"seq negativo":         `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"seq":-1,"payload":{}}`,
		"seq string":           `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"seq":"6","payload":{}}`,
		"seq float":            `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"seq":1.5,"payload":{}}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, evErr, _ := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
			require.NotNil(t, evErr, "deberia rechazarse")
			assert.Equal(t, ingest.CodeInvalidSchema, evErr.Code)
		})
	}
}

// El sobre esta bien formado, pero el evento no corresponde a este device: es
// VALIDATION_FAILED, no INVALID_SCHEMA. A diferencia de un kind desconocido o
// un schemaVersion del futuro (ver TestValidateEventAcceptsVersionSkew...),
// esto SI se rechaza: es una falla de autorizacion real, no version skew.
func TestValidateEventValidationFailed(t *testing.T) {
	cases := map[string]string{
		"machineId de otra maquina": `{"eventId":"e1","machineId":"EMB-OTRA-999","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,"payload":{}}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			_, evErr, _ := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
			require.NotNil(t, evErr)
			assert.Equal(t, ingest.CodeValidationFailed, evErr.Code)
		})
	}
}

func TestValidateEventAcceptsAllKinds(t *testing.T) {
	for _, kind := range []string{ingest.KindMetric, ingest.KindAlarm, ingest.KindHeartbeat} {
		doc := `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
		         "kind":"` + kind + `","schemaVersion":1,"payload":{}}`
		_, evErr, skew := ingestapp.ValidateEvent(json.RawMessage(doc), dev, time.Now().UTC())
		assert.Nil(t, evErr, "kind %q es del enum del contrato", kind)
		assert.Equal(t, ingest.SkewReasonNone, skew, "un kind del enum no es skew")
	}
}

// Ruling del review final (item 2): un kind fuera del enum, o un
// schemaVersion mayor a MaxSchemaVersion, ya NO se rechazan. Son sobres bien
// formados de una version del Edge mas nueva que este build del cloud, y
// aceptarlos es reversible (se puede filtrar o borrar despues) mientras que
// mandarlos a DEAD no lo es (el Edge borra el evento de su outbox). El evento
// se persiste igual y el llamador se entera por el SkewReason devuelto.
func TestValidateEventAcceptsVersionSkewInsteadOfRejecting(t *testing.T) {
	cases := map[string]struct {
		doc  string
		skew ingest.SkewReason
	}{
		"kind fuera del enum": {
			doc:  `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"telemetria","schemaVersion":1,"payload":{}}`,
			skew: ingest.SkewReasonUnknownKind,
		},
		"schemaVersion del futuro": {
			doc:  `{"eventId":"e1","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":99,"payload":{}}`,
			skew: ingest.SkewReasonSchemaVersionAhead,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m, evErr, skew := ingestapp.ValidateEvent(json.RawMessage(tc.doc), dev, time.Now().UTC())
			require.Nil(t, evErr, "version skew ya no se rechaza")
			assert.Equal(t, tc.skew, skew)
			assert.Equal(t, "e1", m.EventID, "el evento debe quedar armado para persistirse, no descartado")
		})
	}
}
