package measurements_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
	"github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
)

// newDB abre una base descartable por test. Sin MONGO_URI hace skip, igual que
// los tests de Postgres del repo.
func newDB(t *testing.T) *mongodriver.Database {
	t.Helper()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	// Driver v2: Connect no recibe context.
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)

	db := cli.Database("embolsadora_test_" + uuid.NewString()[:8])
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = cli.Disconnect(context.Background())
	})
	return db
}

func newRepo(t *testing.T) (*measurements.Repository, *mongodriver.Database) {
	t.Helper()
	db := newDB(t)
	repo := measurements.New(db)
	require.NoError(t, repo.EnsureIndexes(context.Background()))
	return repo, db
}

func doc(eventID string) ingest.Measurement {
	seq := int64(1)
	return ingest.Measurement{
		EventID:       eventID,
		TenantID:      uuid.NewString(),
		DeviceID:      uuid.NewString(),
		MachineID:     "EMB-DEV-001",
		Ts:            time.Now().UTC().Truncate(time.Millisecond),
		Seq:           &seq,
		Kind:          ingest.KindMetric,
		SchemaVersion: 1,
		Payload: map[string]any{
			"aasPath": "Operativos/Pesada/peso", "value": 1.0,
			"unit": "kg", "valueType": "xs:float",
		},
		ReceivedAt: time.Now().UTC(),
	}
}

func count(t *testing.T, db *mongodriver.Database) int64 {
	t.Helper()
	n, err := db.Collection(measurements.CollectionName).CountDocuments(context.Background(), bson.D{})
	require.NoError(t, err)
	return n
}

func TestInsertManyCleanBatch(t *testing.T) {
	repo, db := newRepo(t)

	docs := []ingest.Measurement{doc("a1"), doc("a2"), doc("a3")}
	report, err := repo.InsertMany(context.Background(), docs)
	require.NoError(t, err)

	assert.Empty(t, report.Duplicated)
	assert.Empty(t, report.Failed)
	assert.Equal(t, 3, report.Inserted(len(docs)))
	assert.EqualValues(t, 3, count(t, db))
}

// El nucleo de la idempotencia: reinsertar el mismo lote no crea documentos
// nuevos y reporta cada eventId repetido como DUPLICATE.
func TestInsertManyReportsDuplicates(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	docs := []ingest.Measurement{doc("b1"), doc("b2")}
	_, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err)

	report, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err, "un lote 100% duplicado NO es un error de operacion")

	assert.Len(t, report.Duplicated, 2)
	assert.Contains(t, report.Duplicated, 0)
	assert.Contains(t, report.Duplicated, 1)
	assert.Empty(t, report.Failed)
	assert.EqualValues(t, 2, count(t, db), "no se duplicaron documentos")
}

// ordered:false es lo que hace que un duplicado en el medio no aborte el resto.
// Con ordered:true, "c2" frenaria el lote y "c3" se perderia.
func TestInsertManyIsUnorderedAndReportsCorrectIndices(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	_, err := repo.InsertMany(ctx, []ingest.Measurement{doc("c2")})
	require.NoError(t, err)

	docs := []ingest.Measurement{doc("c1"), doc("c2"), doc("c3")}
	report, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err)

	require.Len(t, report.Duplicated, 1)
	assert.Contains(t, report.Duplicated, 1, "el indice reportado es la posicion dentro de docs")
	assert.Equal(t, 2, report.Inserted(len(docs)))
	assert.EqualValues(t, 3, count(t, db))
}

func TestInsertManyEmptyIsNoop(t *testing.T) {
	repo, _ := newRepo(t)
	report, err := repo.InsertMany(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Inserted(0))
}

// Un evento sin aasPath se persiste igual: el indice compuesto es sparse. Esto
// cubre los eventos que quedaron en el outbox del Pi antes del commit 76c3805.
func TestInsertManyAcceptsPayloadWithoutAASPath(t *testing.T) {
	repo, db := newRepo(t)

	d := doc("d1")
	d.Payload = map[string]any{"value": 1.0, "unit": "kg"}
	_, err := repo.InsertMany(context.Background(), []ingest.Measurement{d})
	require.NoError(t, err)
	assert.EqualValues(t, 1, count(t, db))
}

func TestEnsureIndexesIsIdempotent(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.EnsureIndexes(ctx), "correrlo dos veces no debe fallar")

	cur, err := db.Collection(measurements.CollectionName).Indexes().List(ctx)
	require.NoError(t, err)
	var idx []bson.M
	require.NoError(t, cur.All(ctx, &idx))

	// _id + los 3 del diseno (§6.3).
	assert.Len(t, idx, 4)

	var foundUnique bool
	for _, i := range idx {
		if i["name"] == "uq_eventId" {
			foundUnique = true
			assert.Equal(t, true, i["unique"], "el indice de eventId DEBE ser unico")
		}
	}
	assert.True(t, foundUnique, "falta el indice unico sobre eventId")
}

func TestPing(t *testing.T) {
	repo, _ := newRepo(t)
	assert.NoError(t, repo.Ping(context.Background()))
}
