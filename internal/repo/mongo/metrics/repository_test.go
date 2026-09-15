package metrics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
)

func mustConnect(t *testing.T) *mongodriver.Database {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("MONGO_URI no seteado, se salta el integration test")
	}
	cli, err := mongodriver.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Disconnect(context.Background()) })
	return cli.Database("embolsadora_test_metrics")
}

// cleanTenant borra todos los measurements de un tenant en la coleccion de
// test. Se usa antes de sembrar (por si una corrida anterior quedo a mitad
// de camino, sin llegar a su propio t.Cleanup) y se registra via t.Cleanup
// para correr tambien despues — asi las corridas repetidas contra el mismo
// Mongo local de larga vida (sin recrear el container) nunca acumulan
// documentos, sin importar si la corrida anterior se interrumpio.
func cleanTenant(t *testing.T, db *mongodriver.Database, tenantID string) {
	t.Helper()
	del := func() {
		_, err := db.Collection("measurements").DeleteMany(context.Background(), bson.M{"tenantId": tenantID})
		require.NoError(t, err)
	}
	del()
	t.Cleanup(del)
}

func seedMeasurement(t *testing.T, db *mongodriver.Database, tenantID, machineID, aasPath string, ts time.Time, value any) {
	_, err := db.Collection("measurements").InsertOne(context.Background(), bson.M{
		"eventId":   ts.Format(time.RFC3339Nano) + aasPath,
		"tenantId":  tenantID,
		"machineId": machineID,
		"ts":        ts,
		"kind":      "metric",
		"payload":   bson.M{"aasPath": aasPath, "value": value},
	})
	require.NoError(t, err)
}

func TestScalar_Avg(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-scalar-avg"
	now := time.Now().UTC()

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-1*time.Hour), 1.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-30*time.Minute), 3.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-10*time.Minute), "no-numerico")

	result, dataAsOf, err := repo.Scalar(ctx, tenantID, "M1", now.Add(-2*time.Hour), now, domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)

	if result.Value != 2.0 {
		t.Fatalf("avg = %v, esperaba 2.0 (el valor no numerico se descarta)", result.Value)
	}
	if result.SampleCount != 2 {
		t.Fatalf("sampleCount = %v, esperaba 2", result.SampleCount)
	}
}

func TestSeries_AvgBucketized(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-series-avg"
	base := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "peso", base.Add(10*time.Minute), 1.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", base.Add(50*time.Minute), 3.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", base.Add(90*time.Minute), 5.0)

	points, dataAsOf, err := repo.Series(ctx, tenantID, "M1", base, base.Add(2*time.Hour), domain.Bucket1h, domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	require.Len(t, points, 2)

	if points[0].Value != 2.0 || points[0].SampleCount != 2 {
		t.Fatalf("bucket 0: value=%v sampleCount=%v, esperaba value=2.0 sampleCount=2", points[0].Value, points[0].SampleCount)
	}
	if points[1].Value != 5.0 || points[1].SampleCount != 1 {
		t.Fatalf("bucket 1: value=%v sampleCount=%v, esperaba value=5.0 sampleCount=1", points[1].Value, points[1].SampleCount)
	}
}

func TestRaw_ReturnsPointsOrderedByTs(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(2*time.Second), 80.0)
	seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(1*time.Second), 79.0)

	points, dataAsOf, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", nil, 5001, 0)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	require.Len(t, points, 2)
	if !points[0].Ts.Before(points[1].Ts) {
		t.Fatalf("puntos no ordenados por ts ascendente")
	}
}

func TestRaw_DecimatesWhenMaxPointsSet(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw-decimate"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	for i := 0; i < 10; i++ {
		seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(time.Duration(i)*time.Second), float64(i))
	}

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", nil, 5001, 3)
	require.NoError(t, err)
	require.LessOrEqual(t, len(points), 3)
	// El primer y ultimo punto original siempre se preservan.
	if points[0].Value != 0.0 {
		t.Fatalf("primer punto = %v, esperaba 0.0", points[0].Value)
	}
	if points[len(points)-1].Value != 9.0 {
		t.Fatalf("ultimo punto = %v, esperaba 9.0", points[len(points)-1].Value)
	}
}

func TestRaw_MaxPointsOneReturnsMostRecentPoint(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw-maxpoints-one"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	for i := 0; i < 4; i++ {
		seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(time.Duration(i)*time.Second), float64(i))
	}

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", nil, 5001, 1)
	require.NoError(t, err)
	require.Len(t, points, 1)
	if points[0].Value != 3.0 {
		t.Fatalf("punto = %v, esperaba 3.0 (el mas reciente)", points[0].Value)
	}
	if !points[0].Ts.Equal(base.Add(3 * time.Second)) {
		t.Fatalf("ts = %v, esperaba %v", points[0].Ts, base.Add(3*time.Second))
	}
}

func TestGrouped_CountByField(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-grouped"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	seedAlarm := func(tipo string, at time.Time) {
		_, err := db.Collection("measurements").InsertOne(ctx, bson.M{
			"eventId":   tipo + at.Format(time.RFC3339Nano),
			"tenantId":  tenantID,
			"machineId": "M1",
			"ts":        at,
			"kind":      "alarm",
			"payload":   bson.M{"aasPath": "Alarmas/tipo", "value": tipo, "tipo": tipo},
		})
		require.NoError(t, err)
	}
	seedAlarm("sellado_defectuoso", base)
	seedAlarm("sellado_defectuoso", base.Add(time.Minute))
	seedAlarm("temperatura_alta", base.Add(2*time.Minute))

	groups, dataAsOf, err := repo.Grouped(ctx, tenantID, "M1", base.Add(-time.Hour), base.Add(time.Hour), "payload.tipo", domain.MetricSpec{AasPath: "Alarmas/tipo", Agg: domain.AggCount}, nil, 201)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	require.Len(t, groups, 2)
	if groups[0].Key != "sellado_defectuoso" || groups[0].Value != 2 {
		t.Fatalf("primer grupo = %+v, esperaba sellado_defectuoso:2 (orden descendente por value)", groups[0])
	}
}

func TestCatalog_ReturnsObservedAasPaths(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-catalog"
	now := time.Now().UTC()

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "peso", now, 1.0)
	seedMeasurement(t, db, tenantID, "M1", "temperatura", now, 80.0)
	seedMeasurement(t, db, tenantID, "M2", "otra_maquina", now, 1.0) // otro machineId, no debe aparecer

	paths, err := repo.Catalog(ctx, tenantID, "M1")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"peso", "temperatura"}, paths)
}

// TestScalar_QueryTimeout cubre I3: un maxTime artificialmente chico debe
// hacer que Aggregate falle con context.DeadlineExceeded, y ese error debe
// llegar envuelto en un *domain.ValidationError con CodeQueryTimeout (no un
// error generico) -- HandleError ya sabia mapear ese codigo a 504, pero
// nada lo construia antes de esta fix.
func TestScalar_QueryTimeout(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 1*time.Nanosecond)
	ctx := context.Background()
	tenantID := "tenant-query-timeout"
	now := time.Now().UTC()

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-1*time.Hour), 1.0)

	_, _, err := repo.Scalar(ctx, tenantID, "M1", now.Add(-2*time.Hour), now, domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.Error(t, err)
	var ve *domain.ValidationError
	require.ErrorAs(t, err, &ve, "esperaba *domain.ValidationError, obtuvo %T: %v", err, err)
	require.Equal(t, domain.CodeQueryTimeout, ve.Code)
}

// TestScalar_ValueEqualsFilter_AppliedInNumericAgg cubre I4b: filter.ValueEquals
// debe honrarse tambien en el camino numerico de Scalar (avg/sum/min/max),
// no solo en scalarCount -- antes se ignoraba silenciosamente aca.
func TestScalar_ValueEqualsFilter_AppliedInNumericAgg(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-scalar-valueequals-filter"
	now := time.Now().UTC()

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-3*time.Hour), 5.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-2*time.Hour), 5.0)
	seedMeasurement(t, db, tenantID, "M1", "peso", now.Add(-1*time.Hour), 7.0)

	result, _, err := repo.Scalar(ctx, tenantID, "M1", now.Add(-4*time.Hour), now, domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, &domain.ValueFilter{ValueEquals: 5.0})
	require.NoError(t, err)
	if result.Value != 5.0 {
		t.Fatalf("avg = %v, esperaba 5.0 (filter.valueEquals debe excluir el 7.0)", result.Value)
	}
	if result.SampleCount != 2 {
		t.Fatalf("sampleCount = %v, esperaba 2", result.SampleCount)
	}
}

// TestRaw_DoesNotDecimateWhenFetchHitCap cubre I5a: si el fetch llega al
// cap (limit), Raw NO debe decimar aunque maxPoints este seteado -- eso
// dejaria una vista silenciosamente truncada del rango pedido. El llamador
// (app/dashboards.queryRaw) es quien debe rechazar con RANGE_TOO_WIDE en
// ese caso.
func TestRaw_DoesNotDecimateWhenFetchHitCap(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw-no-decimate-at-cap"
	base := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	const seeded = 6
	const limit = 5
	for i := 0; i < seeded; i++ {
		seedMeasurement(t, db, tenantID, "M1", "temp", base.Add(time.Duration(i)*time.Second), float64(i))
	}

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", nil, limit, 3)
	require.NoError(t, err)
	require.Len(t, points, limit, "el fetch llego al cap; no debia decimarse a maxPoints")
}

// TestCrossTenantIsolation es el caso mas importante de la spec (seccion
// Testing): el $match de tenantId nunca debe dejar leer datos de otro
// tenant, ni siquiera con un groupBy o aasPath que coincida por casualidad.
func TestCrossTenantIsolation(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	now := time.Now().UTC()

	// Mismo machineId y aasPath en dos tenants distintos -- exactamente el
	// caso "coincide por casualidad" que la spec pide cubrir.
	cleanTenant(t, db, "tenant-a")
	cleanTenant(t, db, "tenant-b")
	seedMeasurement(t, db, "tenant-a", "SHARED-ID", "peso", now, 100.0)
	seedMeasurement(t, db, "tenant-b", "SHARED-ID", "peso", now, 999.0)

	result, _, err := repo.Scalar(ctx, "tenant-a", "SHARED-ID", now.Add(-time.Hour), now.Add(time.Hour), domain.MetricSpec{AasPath: "peso", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	if result.Value != 100.0 {
		t.Fatalf("Scalar devolvio %v, esperaba 100.0 (leyo de otro tenant)", result.Value)
	}

	paths, err := repo.Catalog(ctx, "tenant-a", "SHARED-ID")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"peso"}, paths)

	groups, _, err := repo.Grouped(ctx, "tenant-a", "SHARED-ID", now.Add(-time.Hour), now.Add(time.Hour), "payload.value", domain.MetricSpec{AasPath: "peso", Agg: domain.AggCount}, nil, 201)
	require.NoError(t, err)
	for _, g := range groups {
		if g.Key == "999" {
			t.Fatalf("Grouped devolvio un valor del tenant-b")
		}
	}
}

// TestGrouped_ExcludesNonNumericValueFromNumericAgg cubre la falta del
// isNumberMatch en Grouped: sin el, un payload.value no numerico entra al
// acumulador de max/avg/sum/min -- BSON ordena string por encima de
// cualquier numero, asi que $max elegia el string y el decode a
// GroupResult.Value float64 fallaba (o, en avg/sum, contaminaba el
// resultado).
func TestGrouped_ExcludesNonNumericValueFromNumericAgg(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-grouped-nonnumeric"
	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	seed := func(tipo string, value any, at time.Time) {
		_, err := db.Collection("measurements").InsertOne(ctx, bson.M{
			"eventId":   tipo + at.Format(time.RFC3339Nano),
			"tenantId":  tenantID,
			"machineId": "M1",
			"ts":        at,
			"kind":      "measurement",
			"payload":   bson.M{"aasPath": "peso", "value": value, "tipo": tipo},
		})
		require.NoError(t, err)
	}
	seed("linea1", 5.0, base)
	seed("linea1", 7.0, base.Add(time.Minute))
	seed("linea1", "error_sensor", base.Add(2*time.Minute))

	groups, _, err := repo.Grouped(ctx, tenantID, "M1", base.Add(-time.Hour), base.Add(time.Hour), "payload.tipo", domain.MetricSpec{AasPath: "peso", Agg: domain.AggMax}, nil, 201)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	if groups[0].Value != 7.0 {
		t.Fatalf("max = %v, esperaba 7.0 (el valor no numerico debia excluirse del acumulador)", groups[0].Value)
	}
}

// TestSeries_DataAsOfReflectsLatestSampleWithinBucket cubre que dataAsOf se
// calcule sobre el "$max":"$ts" real de cada bucket, no sobre el limite
// inferior que produce $dateTrunc -- un sample nuevo dentro del bucket mas
// reciente no debe dejar dataAsOf pegado al inicio del bucket.
func TestSeries_DataAsOfReflectsLatestSampleWithinBucket(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-series-dataasof"
	bucketStart := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "temp", bucketStart.Add(2*time.Second), 10.0)
	latest := bucketStart.Add(58 * time.Minute)
	seedMeasurement(t, db, tenantID, "M1", "temp", latest, 12.0)

	_, dataAsOf, err := repo.Series(ctx, tenantID, "M1", bucketStart, bucketStart.Add(time.Hour), domain.Bucket1h, domain.MetricSpec{AasPath: "temp", Agg: domain.AggAvg}, nil)
	require.NoError(t, err)
	require.NotNil(t, dataAsOf)
	if !dataAsOf.Equal(latest) {
		t.Fatalf("dataAsOf = %v, esperaba %v (el ultimo sample real dentro del bucket, no su limite inferior)", dataAsOf, latest)
	}
}

// TestRaw_AppliesValueEqualsFilter cubre que Raw honre filter.ValueEquals
// igual que Scalar/Series/Grouped -- antes se ignoraba silenciosamente y el
// modo raw devolvia todos los puntos sin filtrar.
func TestRaw_AppliesValueEqualsFilter(t *testing.T) {
	db := mustConnect(t)
	repo := New(db, 5*time.Second)
	ctx := context.Background()
	tenantID := "tenant-raw-filter"
	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)

	cleanTenant(t, db, tenantID)
	seedMeasurement(t, db, tenantID, "M1", "estado", base, 1.0)
	seedMeasurement(t, db, tenantID, "M1", "estado", base.Add(time.Second), 2.0)
	seedMeasurement(t, db, tenantID, "M1", "estado", base.Add(2*time.Second), 1.0)

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "estado", &domain.ValueFilter{ValueEquals: 1.0}, 5001, 0)
	require.NoError(t, err)
	require.Len(t, points, 2)
	for _, p := range points {
		if p.Value != 1.0 {
			t.Fatalf("point value = %v, esperaba solo 1.0 (filter.valueEquals)", p.Value)
		}
	}
}
