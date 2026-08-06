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
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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

// SC-008: 1000 eventos por debajo de 500 ms.
func TestSC008ThousandEventsLatency(t *testing.T) {
	if os.Getenv("MONGO_URI") == "" {
		t.Skip("MONGO_URI no seteada; se omite el benchmark")
	}
	repo, _ := newMongoRepo(t)
	r := newRouter(repo)

	body := makeBatch(t, 1000)
	start := time.Now()
	code, res := post(t, r, body)
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 1000, res.Accepted)
	assert.Less(t, elapsed, 500*time.Millisecond, "SC-008: 1000 eventos en menos de 500 ms")
	t.Logf("1000 eventos en %v", elapsed)
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

// SC-006: superado el limite, 429 con Retry-After.
func TestSC006RateLimitReturns429WithRetryAfter(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	// rps y burst chicos para agotar el bucket sin mandar 200 requests.
	limiter := consumers.NewRateLimiter(rdb, 1, 3)
	key := "test-" + time.Now().Format("150405.000")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		allowed, _, err := limiter.Allow(ctx, key)
		require.NoError(t, err)
		require.True(t, allowed, "los primeros %d deben pasar (burst)", 3)
	}

	allowed, retryAfter, err := limiter.Allow(ctx, key)
	require.NoError(t, err)
	assert.False(t, allowed, "agotado el burst, se rechaza")
	assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After debe ser >= 1 segundo")
}

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

// makeBatch arma un batch de n eventos validos y distintos.
func makeBatch(t *testing.T, n int) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString(`{"events":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, `{"eventId":"bench-%d-%d","machineId":"EMB-DEV-001",`+
			`"ts":"2026-07-31T01:06:37Z","kind":"metric","schemaVersion":1,`+
			`"payload":{"aasPath":"Operativos/Pesada/peso","value":%d}}`, time.Now().UnixNano(), i, i)
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
