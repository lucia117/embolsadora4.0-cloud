//go:build integration

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

	points, dataAsOf, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", 5001, 0)
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

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", 5001, 3)
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

	points, _, err := repo.Raw(ctx, tenantID, "M1", base, base.Add(time.Hour), "temp", 5001, 1)
	require.NoError(t, err)
	require.Len(t, points, 1)
	if points[0].Value != 3.0 {
		t.Fatalf("punto = %v, esperaba 3.0 (el mas reciente)", points[0].Value)
	}
	if !points[0].Ts.Equal(base.Add(3 * time.Second)) {
		t.Fatalf("ts = %v, esperaba %v", points[0].Ts, base.Add(3*time.Second))
	}
}
