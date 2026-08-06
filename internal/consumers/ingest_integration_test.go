package consumers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/consumers"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
)

// deadRepo simula Mongo caido: TODA operacion falla.
type deadRepo struct{}

func (deadRepo) InsertMany(context.Context, []ingest.Measurement) (ingest.InsertReport, error) {
	return ingest.InsertReport{}, errors.New("mongo inalcanzable")
}
func (deadRepo) Ping(context.Context) error { return errors.New("mongo inalcanzable") }

// SC-004 — el criterio mas importante del plan.
//
// Con el storage caido, la respuesta NUNCA puede contener INVALID_SCHEMA,
// VALIDATION_FAILED ni un 400: cualquiera de los tres haria que el Edge marque
// los eventos como DEAD y no los reintente jamas. Una caida de diez minutos se
// convertiria en perdida permanente.
func TestSC004StorageDownNeverReportsPayloadErrors(t *testing.T) {
	r := newRouter(deadRepo{})
	body := fixture(t)

	code, _ := post(t, r, body)
	assert.Equal(t, http.StatusInternalServerError, code,
		"storage caido debe ser 500 —que el Edge reintenta—, nunca 400")

	// Y la respuesta no debe mencionar ningun codigo terminal.
	raw := postRaw(t, r, body)
	for _, terminal := range []string{ingest.CodeInvalidSchema, ingest.CodeValidationFailed} {
		assert.NotContains(t, string(raw), terminal,
			"un fallo de infraestructura no puede reportarse como error de payload (I-1)")
	}
}

// SC-004, segunda mitad: al volver el storage, el reintento persiste todo sin
// duplicar. Se simula reusando el mismo repo en memoria.
func TestSC004RetryAfterRecoveryPersistsWithoutDuplicates(t *testing.T) {
	repo := newMemRepo()
	body := fixture(t)

	// Primer intento con storage caido.
	code, _ := post(t, newRouter(deadRepo{}), body)
	require.Equal(t, http.StatusInternalServerError, code)

	// El Edge reintenta contra el storage ya recuperado.
	code, res := post(t, newRouter(repo), body)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 108, res.Accepted)
	assert.Len(t, repo.stored, 108)

	// Y un tercer reintento tampoco duplica.
	_, third := post(t, newRouter(repo), body)
	assert.Equal(t, 108, third.Rejected)
	assert.Len(t, repo.stored, 108)
}

// SC-007: un batch cuyo machineId no es el del device escribe CERO documentos.
func TestSC007ForeignMachineIDWritesNothing(t *testing.T) {
	repo := newMemRepo()

	body := `{"events":[{"eventId":"x1","machineId":"EMB-OTRA-999","ts":"2026-07-31T01:06:37Z",` +
		`"kind":"metric","schemaVersion":1,"payload":{"value":1}}]}`

	code, res := post(t, newRouter(repo), []byte(body))
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, 0, res.Accepted)
	assert.Equal(t, 1, res.Rejected)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, ingest.CodeValidationFailed, res.Errors[0].Code)
	assert.Empty(t, repo.stored, "no se escribio ningun documento")
}

// SC-008: p95 de la latencia de un batch de 1000 eventos por debajo de 500 ms.
//
// Una sola muestra no alcanza para hablar de un percentil (la primera version
// de este test tomaba una unica medicion): se corren 20 iteraciones y se
// calcula el p95 sobre las 20 duraciones ordenadas.
//
// Cada iteracion usa un batch FRESCO —eventId distintos, via el parametro iter
// de makeBatch— y no el mismo batch reenviado. Reenviar el mismo batch haria
// que las iteraciones 2..20 midieran el camino de duplicado (un rechazo por
// E11000 contra el indice unico, mucho mas barato que escribir 1000 documentos
// nuevos) en lugar del camino de insercion real, lo que dejaria el numero sin
// sentido: se estaria cronometrando el caso feliz equivocado.
func TestSC008ThousandEventsLatency(t *testing.T) {
	if os.Getenv("MONGO_URI") == "" {
		t.Skip("MONGO_URI no seteada; se omite el benchmark")
	}
	repo, _ := newMongoRepo(t)
	r := newRouter(repo)

	const iterations = 20
	samples := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		body := makeBatch(t, 1000, i)

		start := time.Now()
		code, res := post(t, r, body)
		elapsed := time.Since(start)

		require.Equal(t, http.StatusOK, code)
		require.Equal(t, 1000, res.Accepted,
			"iteracion %d: los 1000 deben ser inserciones nuevas, no duplicados", i)
		samples = append(samples, elapsed)
	}

	sort.Slice(samples, func(a, b int) bool { return samples[a] < samples[b] })
	p95Index := int(0.95 * float64(len(samples)))
	if p95Index >= len(samples) {
		p95Index = len(samples) - 1
	}
	min, p95, max := samples[0], samples[p95Index], samples[len(samples)-1]

	t.Logf("SC-008: 1000 eventos x %d iteraciones — min=%v p95=%v max=%v", iterations, min, p95, max)
	assert.Less(t, p95, 500*time.Millisecond, "SC-008: p95 de 1000 eventos debe ser menor a 500 ms")
}

// SC-009: la consulta "ultimo valor de una propiedad" resuelve por IXSCAN.
// Sin este test es facil terminar con cientos de GB que hay que escanear
// enteros para dibujar un grafico.
func TestSC009LatestValueQueryUsesIndex(t *testing.T) {
	repo, db := newMongoRepo(t)
	ctx := context.Background()

	require.NoError(t, seedFixture(t, repo))

	coll := db.Collection(measurements.CollectionName)
	cmd := bson.D{
		{Key: "explain", Value: bson.D{
			{Key: "find", Value: measurements.CollectionName},
			{Key: "filter", Value: bson.D{
				{Key: "tenantId", Value: "t1"},
				{Key: "machineId", Value: "EMB-DEV-001"},
				{Key: "payload.aasPath", Value: "Operativos/Pesada/peso"},
			}},
			{Key: "sort", Value: bson.D{{Key: "ts", Value: -1}}},
			{Key: "limit", Value: 1},
		}},
		{Key: "verbosity", Value: "queryPlanner"},
	}
	var out bson.M
	require.NoError(t, coll.Database().RunCommand(ctx, cmd).Decode(&out))

	plan, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Contains(t, string(plan), "IXSCAN", "la consulta debe resolver por indice")
	assert.NotContains(t, string(plan), "COLLSCAN", "no puede escanear la coleccion entera")
}

// SC-006 (429 con Retry-After al superar el limite) se prueba de punta a
// punta, contra un status code y un header HTTP reales, en
// internal/consumers/middleware/middleware_test.go
// (TestSC006RateLimitReturns429WithRetryAfter). La primera version de este
// test vivia aca y probaba unicamente RateLimiter.Allow() — una tupla
// (bool, int) — sin emitir jamas un request HTTP ni leer un header; quedaba
// en la capa equivocada para el criterio que dice testear.

// El limitador sin Redis deja pasar todo: fail-open deliberado. Un Redis caido
// no puede cortar la ingesta de datos reales.
func TestRateLimiterFailsOpenWithoutRedis(t *testing.T) {
	limiter := consumers.NewRateLimiter(nil, 1, 1)
	for i := 0; i < 10; i++ {
		allowed, _, err := limiter.Allow(context.Background(), "k")
		require.NoError(t, err)
		assert.True(t, allowed)
	}
}

func newMongoRepo(t *testing.T) (*measurements.Repository, *mongodriver.Database) {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	db := cli.Database("embolsadora_ingest_test")
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = cli.Disconnect(context.Background())
	})
	repo := measurements.New(db)
	require.NoError(t, repo.EnsureIndexes(context.Background()))
	return repo, db
}

// postRaw devuelve el body crudo de la respuesta.
func postRaw(t *testing.T, r *gin.Engine, body []byte) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Body.Bytes()
}

// makeBatch arma un batch de n eventos validos y distintos. `iter` distingue
// llamadas sucesivas (p.ej. las 20 iteraciones de SC-008): el eventId incluye
// iter ademas de la posicion dentro del batch, asi que dos llamadas con iter
// distinto nunca colisionan entre si, sin depender de la resolucion del reloj.
func makeBatch(t *testing.T, n, iter int) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString(`{"events":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"eventId":"bench-%d-%d","machineId":"EMB-DEV-001",`+
			`"ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,`+
			`"payload":{"aasPath":"Operativos/Pesada/peso","value":%d}}`, iter, i, i)
	}
	buf.WriteString(`]}`)
	return buf.Bytes()
}

// seedFixture carga el fixture en el repo dado, con tenantId "t1".
func seedFixture(t *testing.T, repo ingest.Repository) error {
	t.Helper()
	var batch struct {
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(fixture(t), &batch))

	docs := make([]ingest.Measurement, 0, len(batch.Events))
	for _, raw := range batch.Events {
		m, evErr := ingestapp.ValidateEvent(raw,
			ingest.DeviceContext{TenantID: "t1", DeviceID: "d1", MachineID: "EMB-DEV-001"},
			time.Now().UTC())
		require.Nil(t, evErr)
		docs = append(docs, m)
	}
	_, err := repo.InsertMany(context.Background(), docs)
	return err
}
