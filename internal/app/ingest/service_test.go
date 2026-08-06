package ingest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	ingestapp "github.com/tu-org/embolsadora-api/internal/app/ingest"
	"github.com/tu-org/embolsadora-api/internal/domain/ingest"
)

// fakeRepo permite guionar el desenlace de InsertMany por posicion.
type fakeRepo struct {
	report      ingest.InsertReport
	err         error
	received    []ingest.Measurement
	receivedCtx context.Context
}

func (f *fakeRepo) InsertMany(ctx context.Context, docs []ingest.Measurement) (ingest.InsertReport, error) {
	f.received = docs
	f.receivedCtx = ctx
	if f.err != nil {
		return ingest.InsertReport{}, f.err
	}
	r := f.report
	if r.Duplicated == nil {
		r.Duplicated = map[int]struct{}{}
	}
	if r.Failed == nil {
		r.Failed = map[int]string{}
	}
	return r, nil
}
func (f *fakeRepo) Ping(context.Context) error { return nil }

func rawEvents(docs ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(docs))
	for i, d := range docs {
		out[i] = json.RawMessage(d)
	}
	return out
}

func evt(id string) string {
	return `{"eventId":"` + id + `","machineId":"EMB-DEV-001","ts":"2026-07-31T01:06:37Z",
	         "kind":"metric","schemaVersion":1,"payload":{"value":1}}`
}

func TestIngestBatchAllValid(t *testing.T) {
	repo := &fakeRepo{}
	svc := ingestapp.NewService(repo, zap.NewNop(), 0)

	res, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a"), evt("b"), evt("c")))
	require.NoError(t, err)

	assert.Equal(t, 3, res.Accepted)
	assert.Equal(t, 0, res.Rejected)
	assert.Empty(t, res.Errors)
	assert.Len(t, repo.received, 3)
}

// SC-003 exacto: [valido, valido, sin ts, valido, ya existente] debe reportar
// los indices 2 y 4 — posiciones del array ORIGINAL, no del filtrado.
//
// Este es el test del invariante I-3. Si el service reportara indices del slice
// que le pasa al repo, el duplicado (posicion 3 de ese slice, que no existe)
// caeria en el lugar equivocado y el Edge mataria el evento sano.
func TestIngestBatchReportsOriginalIndices(t *testing.T) {
	// El evento valido en la posicion 3 del array original es el indice 2 del
	// slice filtrado, y ese es el que el repo marca como duplicado.
	repo := &fakeRepo{report: ingest.InsertReport{Duplicated: map[int]struct{}{3: {}}}}
	svc := ingestapp.NewService(repo, zap.NewNop(), 0)

	sinTs := `{"eventId":"c","machineId":"EMB-DEV-001","kind":"metric","schemaVersion":1,"payload":{}}`
	batch := rawEvents(evt("a"), evt("b"), sinTs, evt("d"), evt("e"))

	res, err := svc.IngestBatch(context.Background(), dev, batch)
	require.NoError(t, err)

	assert.Equal(t, 3, res.Accepted)
	assert.Equal(t, 2, res.Rejected)
	require.Len(t, res.Errors, 2)

	assert.Equal(t, 2, res.Errors[0].Index)
	assert.Equal(t, ingest.CodeInvalidSchema, res.Errors[0].Code)

	assert.Equal(t, 4, res.Errors[1].Index)
	assert.Equal(t, ingest.CodeDuplicate, res.Errors[1].Code)
}

// I-4 sobre una combinacion amplia de casos.
func TestIngestBatchAccountingInvariant(t *testing.T) {
	cases := []struct {
		name   string
		batch  []json.RawMessage
		report ingest.InsertReport
	}{
		{"todos validos", rawEvents(evt("a"), evt("b")), ingest.InsertReport{}},
		{"todos duplicados", rawEvents(evt("a"), evt("b")),
			ingest.InsertReport{Duplicated: map[int]struct{}{0: {}, 1: {}}}},
		{"todos invalidos", rawEvents(`{}`, `{"x":1}`), ingest.InsertReport{}},
		{"mixto con storage", rawEvents(evt("a"), `{}`, evt("c")),
			ingest.InsertReport{Failed: map[int]string{1: "disco lleno"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := ingestapp.NewService(&fakeRepo{report: tc.report}, zap.NewNop(), 0)
			res, err := svc.IngestBatch(context.Background(), dev, tc.batch)
			require.NoError(t, err)
			assert.Equal(t, len(tc.batch), res.Accepted+res.Rejected,
				"I-4: accepted + rejected debe ser igual a len(events)")
			assert.Equal(t, res.Rejected, len(res.Errors))
		})
	}
}

// I-1: un fallo de storage se reporta como STORAGE_UNAVAILABLE, que el Edge
// reintenta. Nunca como INVALID_SCHEMA ni VALIDATION_FAILED.
func TestIngestBatchPartialStorageFailure(t *testing.T) {
	repo := &fakeRepo{report: ingest.InsertReport{Failed: map[int]string{1: "write concern"}}}
	svc := ingestapp.NewService(repo, zap.NewNop(), 0)

	res, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a"), evt("b")))
	require.NoError(t, err)

	assert.Equal(t, 1, res.Accepted)
	require.Len(t, res.Errors, 1)
	assert.Equal(t, 1, res.Errors[0].Index)
	assert.Equal(t, ingest.CodeStorageUnavailable, res.Errors[0].Code)
}

// I-1 en su forma fuerte: si Mongo esta caido, el service propaga el error para
// que el handler devuelva 500. No inventa un resultado parcial ni marca los
// eventos como invalidos.
func TestIngestBatchTotalStorageFailurePropagates(t *testing.T) {
	boom := errors.New("mongo inalcanzable")
	svc := ingestapp.NewService(&fakeRepo{err: boom}, zap.NewNop(), 0)

	_, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a")))
	assert.ErrorIs(t, err, boom)
}

// Si TODOS los eventos son invalidos no hay nada que insertar: no se debe
// llamar al repo, y menos aun devolver 500 por un batch de basura.
func TestIngestBatchAllInvalidSkipsRepo(t *testing.T) {
	repo := &fakeRepo{err: errors.New("no deberia llamarse")}
	svc := ingestapp.NewService(repo, zap.NewNop(), 0)

	res, err := svc.IngestBatch(context.Background(), dev, rawEvents(`{}`, `{}`))
	require.NoError(t, err)
	assert.Equal(t, 0, res.Accepted)
	assert.Equal(t, 2, res.Rejected)
	assert.Nil(t, repo.received)
}

// Item 6 de la review final: sin un limite propio, un primario de Mongo
// COLGADO (no caido: caido falla rapido) deja el goroutine del request
// esperando para siempre en vez de devolver el 500 que el Edge sabe
// reintentar (I-1). NewService recibe mongoTimeout y IngestBatch debe atar
// CADA llamada a InsertMany a un context.WithTimeout derivado de ese valor.
func TestIngestBatchBoundsInsertManyWithMongoTimeout(t *testing.T) {
	repo := &fakeRepo{}
	svc := ingestapp.NewService(repo, zap.NewNop(), 50*time.Millisecond)

	_, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a")))
	require.NoError(t, err)

	require.NotNil(t, repo.receivedCtx, "InsertMany debe haberse llamado")
	deadline, ok := repo.receivedCtx.Deadline()
	require.True(t, ok, "el contexto pasado a InsertMany debe tener un deadline cuando mongoTimeout > 0")
	assert.WithinDuration(t, time.Now().Add(50*time.Millisecond), deadline, 25*time.Millisecond)
}

// mongoTimeout == 0 (el default de una Config sin setear, o de un test que no
// lo necesita) no debe forzar un deadline artificial: se propaga el ctx del
// caller tal cual.
func TestIngestBatchWithZeroMongoTimeoutDoesNotForceDeadline(t *testing.T) {
	repo := &fakeRepo{}
	svc := ingestapp.NewService(repo, zap.NewNop(), 0)

	_, err := svc.IngestBatch(context.Background(), dev, rawEvents(evt("a")))
	require.NoError(t, err)

	require.NotNil(t, repo.receivedCtx)
	_, ok := repo.receivedCtx.Deadline()
	assert.False(t, ok, "mongoTimeout=0 no debe agregar un deadline que no estaba en el ctx original")
}
