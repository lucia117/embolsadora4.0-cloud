// Package measurements es la unica implementacion de ingest.Repository, y el
// unico lugar del codigo —junto con internal/platform/mongo— que sabe que
// MongoDB existe.
package measurements

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// CollectionName es la coleccion de mediciones.
const CollectionName = "measurements"

// duplicateKeyCode es el codigo de error de MongoDB para violacion de indice
// unico. Es la senal de idempotencia: no es un fallo, es "ya lo tenia".
const duplicateKeyCode = 11000

// Repository implementa ingest.Repository sobre MongoDB.
type Repository struct {
	coll *mongodriver.Collection
	db   *mongodriver.Database
}

// New construye el repositorio de mediciones.
func New(db *mongodriver.Database) *Repository {
	return &Repository{coll: db.Collection(CollectionName), db: db}
}

// legacyUniqueEventIDIndexName es el nombre del indice unico global sobre
// eventId que este paquete creaba antes de este fix. eventId es
// sha256(machineId | aasPath | ts_nano) y NO lleva tenant, pero machineId solo
// es unico POR TENANT (uq_edge_devices_tenant_machine): dos tenants con un
// device "EMB-DEV-001" leyendo el mismo aasPath producen el mismo eventId, y
// el indice global rechazaba el segundo como si fuera un reintento. Mongo
// devolvia E11000, el service lo reportaba DUPLICATE, y el Edge del segundo
// tenant ACKeaba y borraba de su outbox una medicion que jamas se guardo.
//
// EnsureIndexes no puede reemplazar este indice in place —Mongo no permite
// redefinir las claves de un indice existente manteniendo el nombre—, asi que
// hay que borrarlo explicitamente antes de crear uq_tenant_eventId. En una
// base nueva este indice nunca existio y el borrado es un no-op tolerado. Este
// borrado se puede eliminar una vez que todos los ambientes esten en una
// version posterior a este fix (no antes: un despliegue viejo todavia podria
// tener el indice global creado).
const legacyUniqueEventIDIndexName = "uq_eventId"

// indexNotFoundCode y namespaceNotFoundCode son codigos de error estandar del
// servidor de Mongo (no son especificos de este driver). 27 aparece cuando la
// coleccion existe pero el indice no; 26, cuando la coleccion (o la base)
// todavia no existe — el caso de una base recien creada.
const (
	indexNotFoundCode     = 27
	namespaceNotFoundCode = 26
)

// EnsureIndexes crea los indices de §6.3 si no existen. Es idempotente, asi que
// se puede llamar en cada arranque.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	if err := r.coll.Indexes().DropOne(ctx, legacyUniqueEventIDIndexName); err != nil {
		// "index not found" (o "namespace not found", si la coleccion ni
		// siquiera existe todavia) es el caso esperado en cualquier base que
		// jamas corrio el codigo viejo. Cualquier otro error (por ejemplo, un
		// problema real de conectividad) se propaga: no queremos seguir y
		// terminar con los dos indices coexistiendo sin saberlo.
		var cmdErr mongodriver.CommandError
		tolerated := errors.As(err, &cmdErr) &&
			(cmdErr.Code == indexNotFoundCode || cmdErr.Code == namespaceNotFoundCode)
		if !tolerated {
			return err
		}
	}

	_, err := r.coll.Indexes().CreateMany(ctx, []mongodriver.IndexModel{
		{
			// El indice critico: es toda la idempotencia del sistema. Sin el,
			// cada reintento del Pi duplicaria mediciones en silencio. La clave
			// compuesta (tenantId, eventId) —y no solo eventId— es lo que
			// mantiene esa idempotencia acotada al tenant: eventId no lleva
			// tenant (es sha256(machineId | aasPath | ts_nano)), y machineId
			// solo es unico POR TENANT, asi que dos tenants pueden compartir un
			// eventId de forma legitima.
			Keys: bson.D{
				{Key: "tenantId", Value: 1},
				{Key: "eventId", Value: 1},
			},
			Options: options.Index().SetName("uq_tenant_eventId").SetUnique(true),
		},
		{
			// Sirve las dos consultas previstas del frontend: "ultimo valor de
			// esta propiedad" (igualdad en los tres primeros campos + limit 1) y
			// "esta propiedad en las ultimas 24 hs" (igualdad + rango sobre ts).
			//
			// Sparse porque los eventos anteriores al commit 76c3805 del Edge no
			// traen payload.aasPath: se persisten igual, solo no entran al
			// indice. Es la razon por la que el deploy de los dos repos no
			// necesita coordinarse.
			Keys: bson.D{
				{Key: "tenantId", Value: 1},
				{Key: "machineId", Value: 1},
				{Key: "payload.aasPath", Value: 1},
				{Key: "ts", Value: -1},
			},
			Options: options.Index().SetName("ix_tenant_machine_path_ts").SetSparse(true),
		},
		{
			// Barridos por tenant y lectura de lo reciente por el futuro motor
			// de alarmas.
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "ts", Value: -1}},
			Options: options.Index().SetName("ix_tenant_ts"),
		},
	})
	return err
}

// InsertMany inserta el lote con ordered:false y traduce el resultado a un
// InsertReport.
//
// ordered:false es esencial y no una optimizacion: con ordered:true el primer
// duplicado abortaria el resto del lote, y en un reintento del Pi —donde el
// primer evento casi siempre ya existe— se perderian los 999 siguientes.
func (r *Repository) InsertMany(ctx context.Context, docs []ingest.Measurement) (ingest.InsertReport, error) {
	report := ingest.InsertReport{
		Duplicated: make(map[int]struct{}),
		Failed:     make(map[int]string),
	}
	if len(docs) == 0 {
		return report, nil
	}

	raw := make([]any, len(docs))
	for i := range docs {
		raw[i] = docs[i]
	}

	_, err := r.coll.InsertMany(ctx, raw, options.InsertMany().SetOrdered(false))
	if err == nil {
		return report, nil
	}

	// BulkWriteException significa que la operacion llego al servidor y este
	// respondio por documento. Todo lo demas —conexion caida, timeout, seleccion
	// de servidor— es un fallo total que se propaga para terminar en un 500.
	var bwe mongodriver.BulkWriteException
	if !errors.As(err, &bwe) {
		return ingest.InsertReport{}, err
	}

	for _, we := range bwe.WriteErrors {
		if we.Code == duplicateKeyCode {
			report.Duplicated[we.Index] = struct{}{}
			continue
		}
		report.Failed[we.Index] = we.Message
	}

	// Un write concern error afecta al lote entero, no a documentos puntuales:
	// no sabemos que se persistio. Se propaga como fallo total (I-1).
	if bwe.WriteConcernError != nil {
		return ingest.InsertReport{}, err
	}
	return report, nil
}

// Ping verifica la conectividad con el primario.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Client().Ping(ctx, readpref.Primary())
}
