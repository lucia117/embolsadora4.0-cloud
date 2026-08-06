package measurements_test

import (
	"context"
	"fmt"
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

// docForTenant es como doc(), pero con TenantID explicito: doc() lo randomiza
// (uuid.NewString()) precisamente porque cada test necesita un tenant propio
// para no interferir con otros — pero esa randomizacion es la razon por la que
// quince revisiones por tarea nunca vieron el bug de esta prueba: con
// TenantID al azar, dos documentos jamas terminan compartiendo tenant a menos
// que el test lo fuerce a mano.
func docForTenant(eventID, tenantID string) ingest.Measurement {
	d := doc(eventID)
	d.TenantID = tenantID
	return d
}

// El bug critico de la review final: eventId es sha256(machineId | aasPath |
// ts_nano) y no lleva tenant, pero machineId solo es unico POR TENANT
// (uq_edge_devices_tenant_machine). Dos tenants con un device "EMB-DEV-001"
// leyendo el mismo aasPath en el mismo instante producen el MISMO eventId.
//
// Contra el indice viejo (unico solo por eventId), este test falla: el
// segundo insert choca con E11000, se reporta como DUPLICATE, y solo queda un
// documento en la coleccion — perteneciente a un solo tenant, mientras el
// otro perdio la medicion sin que nadie se entere. Contra el indice nuevo
// (unico por (tenantId, eventId)) los dos entran y la coleccion queda con 2.
func TestInsertManySameEventIDDifferentTenantsBothPersist(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	sharedEventID := "shared-event-cross-tenant-collision"

	docs := []ingest.Measurement{
		docForTenant(sharedEventID, tenantA),
		docForTenant(sharedEventID, tenantB),
	}
	report, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err)

	assert.Empty(t, report.Duplicated, "el mismo eventId en dos tenants distintos NO es un duplicado")
	assert.Empty(t, report.Failed)
	assert.Equal(t, 2, report.Inserted(len(docs)))
	assert.EqualValues(t, 2, count(t, db), "ambos tenants deben conservar su medicion; ninguna se pierde silenciosamente")
}

// ordered:false es lo que hace que un duplicado en el medio no aborte el resto.
// Con ordered:true, "c2" frenaria el lote y "c3" se perderia.
//
// Las tres mediciones comparten tenant a proposito: el indice unico ahora es
// (tenantId, eventId), asi que dos doc("c2") con TenantID al azar (como hace
// doc() sin mas) ya NO colisionarian entre si y el test dejaria de probar lo
// que dice probar. Un batch real siempre viene de un unico tenant de todas
// formas, asi que fijarlo aca es mas fiel al caso real, no menos.
func TestInsertManyIsUnorderedAndReportsCorrectIndices(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()
	tenant := uuid.NewString()

	_, err := repo.InsertMany(ctx, []ingest.Measurement{docForTenant("c2", tenant)})
	require.NoError(t, err)

	docs := []ingest.Measurement{docForTenant("c1", tenant), docForTenant("c2", tenant), docForTenant("c3", tenant)}
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

// indexSpec decodifica una entrada de Indexes().List(). A diferencia de
// bson.M (un mapa, sin orden garantizado), Key es bson.D: preserva el orden
// de las claves tal como Mongo las devuelve, que es lo que hace falta para
// verificar que un indice compuesto no fue transpuesto.
type indexSpec struct {
	Name   string `bson:"name"`
	Key    bson.D `bson:"key"`
	Unique bool   `bson:"unique"`
	Sparse bool   `bson:"sparse"`
}

// keyOrder normaliza un bson.D de claves de indice a strings "campo:valor" en
// orden, para compararlo con assert.Equal sin pelearse con si Mongo devuelve
// los valores 1/-1 como int32, int o float64.
func keyOrder(d bson.D) []string {
	out := make([]string, len(d))
	for i, e := range d {
		out[i] = fmt.Sprintf("%s:%v", e.Key, e.Value)
	}
	return out
}

func TestEnsureIndexesIsIdempotent(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.EnsureIndexes(ctx), "correrlo dos veces no debe fallar")

	cur, err := db.Collection(measurements.CollectionName).Indexes().List(ctx)
	require.NoError(t, err)
	var idx []indexSpec
	require.NoError(t, cur.All(ctx, &idx))

	// _id + los 3 del diseno (§6.3).
	assert.Len(t, idx, 4)

	var foundUnique, foundComposite, foundTenantTs bool
	for _, i := range idx {
		switch i.Name {
		case "uq_tenant_eventId":
			foundUnique = true
			assert.True(t, i.Unique, "el indice de (tenantId, eventId) DEBE ser unico")
			assert.Equal(t,
				[]string{"tenantId:1", "eventId:1"},
				keyOrder(i.Key),
				"la clave debe ser compuesta (tenantId, eventId): un eventId solo sin tenant permite que dos tenants con el mismo machineId choquen entre si",
			)
		case "ix_tenant_machine_path_ts":
			foundComposite = true
			// Sparse es lo que permite que un documento sin payload.aasPath se
			// persista sin entrar a este indice. TestInsertManyAcceptsPayload-
			// WithoutAASPath NO cubre esto: el indice es no-unico, asi que una
			// version NO sparse tambien aceptaria el insert sin error. Sparse
			// cambia si se crea la entrada de indice, no si el insert falla.
			assert.True(t, i.Sparse, "el indice compuesto DEBE ser sparse")
			assert.Equal(t,
				[]string{"tenantId:1", "machineId:1", "payload.aasPath:1", "ts:-1"},
				keyOrder(i.Key),
				"el orden de las claves determina que consultas puede servir el indice; una transposicion compila y no se nota sin este chequeo",
			)
		case "ix_tenant_ts":
			foundTenantTs = true
			assert.Equal(t, []string{"tenantId:1", "ts:-1"}, keyOrder(i.Key))
		}
	}
	assert.True(t, foundUnique, "falta el indice unico sobre (tenantId, eventId)")
	assert.True(t, foundComposite, "falta el indice compuesto tenant/machine/path/ts")
	assert.True(t, foundTenantTs, "falta el indice tenant/ts")
}

func TestPing(t *testing.T) {
	repo, _ := newRepo(t)
	assert.NoError(t, repo.Ping(context.Background()))
}

// TestInsertManyReportsFailedForNonDuplicateWriteError cubre la rama Failed:
// un error de escritura que NO es 11000 debe caer en report.Failed y no en
// report.Duplicated, y no debe convertirse en un error total de la operacion
// — es un fallo por documento. Es la distincion de la que depende el
// invariante I-1 (Duplicated -> ACK silencioso: el Edge no reintenta;
// Failed -> STORAGE_UNAVAILABLE: el Edge reintenta). Antes de este test,
// reemplazar "if we.Code == duplicateKeyCode" por "if true" —clasificando
// TODO error de escritura como duplicado— pasaba los siete tests originales
// sin que ninguno lo notara.
//
// No se puede usar newDB/newRepo aca porque necesitamos crear la coleccion
// con un validador ANTES de que exista, y newRepo ya la deja creada (via
// EnsureIndexes) sin uno. Se replica a mano el mismo patron de aislamiento
// por base descartable que usa newDB.
func TestInsertManyReportsFailedForNonDuplicateWriteError(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteada; se omite el test de integracion")
	}
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	db := cli.Database("embolsadora_test_" + uuid.NewString()[:8])
	t.Cleanup(func() {
		_ = db.Drop(context.Background())
		_ = cli.Disconnect(context.Background())
	})
	ctx := context.Background()

	// payload.value debe ser numerico. Un documento con "value" string viola
	// el schema y el servidor lo rechaza con DocumentValidationFailure
	// (codigo 121) — no con 11000, que es exclusivo de choques de indice unico.
	validator := bson.M{
		"$jsonSchema": bson.M{
			"bsonType": "object",
			"properties": bson.M{
				"payload": bson.M{
					"bsonType": "object",
					"required": []string{"value"},
					"properties": bson.M{
						"value": bson.M{"bsonType": "number"},
					},
				},
			},
		},
	}
	require.NoError(t, db.CreateCollection(ctx, measurements.CollectionName,
		options.CreateCollection().SetValidator(validator)))

	repo := measurements.New(db)
	require.NoError(t, repo.EnsureIndexes(ctx), "el validador no debe impedir crear los indices")

	valid := doc("f1")
	invalid := doc("f2")
	invalid.Payload = map[string]any{"value": "not-a-number", "unit": "kg"}

	docs := []ingest.Measurement{valid, invalid}
	report, err := repo.InsertMany(ctx, docs)
	require.NoError(t, err, "un fallo de validacion por documento no es un fallo total de la operacion")

	assert.Empty(t, report.Duplicated, "un error de schema no es un duplicado (E11000)")
	require.Len(t, report.Failed, 1)
	assert.Contains(t, report.Failed, 1, "el indice reportado es la posicion del documento invalido dentro de docs")
	assert.EqualValues(t, 1, count(t, db), "solo el documento valido debe haberse insertado")
}
