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
