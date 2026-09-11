// Package metrics implementa domain/metrics.Repository sobre MongoDB.
package metrics

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"

	domain "github.com/tu-org/embolsadora-api/internal/domain/metrics"
	measurements "github.com/tu-org/embolsadora-api/internal/repo/mongo/measurements"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

type Repository struct {
	coll    *mongodriver.Collection
	maxTime time.Duration
}

// New construye el repositorio. maxTime acota cada aggregate (fork/C4) —
// 0 o negativo usa el default de 5s.
func New(db *mongodriver.Database, maxTime time.Duration) *Repository {
	if maxTime <= 0 {
		maxTime = 5 * time.Second
	}
	return &Repository{coll: db.Collection(measurements.CollectionName), maxTime: maxTime}
}

func baseMatch(tenantID, machineID string, from, to time.Time, aasPath string) bson.D {
	return bson.D{{Key: "$match", Value: bson.D{
		{Key: "tenantId", Value: tenantID},
		{Key: "machineId", Value: machineID},
		{Key: "payload.aasPath", Value: aasPath},
		{Key: "ts", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lte", Value: to}}},
	}}}
}

func isNumberMatch(negate bool) bson.D {
	expr := bson.D{{Key: "$isNumber", Value: "$payload.value"}}
	if negate {
		return bson.D{{Key: "$match", Value: bson.D{{Key: "$expr", Value: bson.D{{Key: "$not", Value: bson.A{expr}}}}}}}
	}
	return bson.D{{Key: "$match", Value: bson.D{{Key: "$expr", Value: expr}}}}
}

func numericAccumulator(agg domain.Agg) (string, bool) {
	switch agg {
	case domain.AggAvg:
		return "$avg", true
	case domain.AggSum:
		return "$sum", true
	case domain.AggMin:
		return "$min", true
	case domain.AggMax:
		return "$max", true
	}
	return "", false
}

// Scalar resuelve avg/sum/min/max/last/delta/count sin bucketizar.
func (r *Repository) Scalar(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	switch spec.Agg {
	case domain.AggAvg, domain.AggSum, domain.AggMin, domain.AggMax:
		return r.scalarNumeric(ctx, tenantID, machineID, from, to, spec)
	case domain.AggLast:
		return r.scalarLast(ctx, tenantID, machineID, from, to, spec)
	case domain.AggDelta:
		return r.scalarDelta(ctx, tenantID, machineID, from, to, spec)
	case domain.AggCount:
		return r.scalarCount(ctx, tenantID, machineID, from, to, spec, filter)
	}
	return domain.MetricResult{}, nil, fmt.Errorf("agg no soportado en modo escalar: %s", spec.Agg)
}

type numericGroupResult struct {
	Value       float64   `bson:"value"`
	SampleCount int64     `bson:"sampleCount"`
	DataAsOf    time.Time `bson:"dataAsOf"`
}

func (r *Repository) scalarNumeric(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) (domain.MetricResult, *time.Time, error) {
	accumulator, ok := numericAccumulator(spec.Agg)
	if !ok {
		return domain.MetricResult{}, nil, fmt.Errorf("agg no numerico: %s", spec.Agg)
	}

	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(false),
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}

	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)

	var results []numericGroupResult
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}

	// Discard count: consulta liviana separada (mismo $match, $count) para no
	// complicar el pipeline principal con un $facet. Best-effort: si esta
	// falla, no se pierde el resultado principal, solo la instrumentacion.
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)

	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Value, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) reportNonNumericDiscards(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) {
	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(true),
		bson.D{{Key: "$count", Value: "count"}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return
	}
	defer cur.Close(ctx)
	var results []struct {
		Count int64 `bson:"count"`
	}
	if err := cur.All(ctx, &results); err != nil || len(results) == 0 {
		return
	}
	telemetry.DashboardMetricsNonNumericDiscardedTotal.
		WithLabelValues(tenantID, spec.AasPath, string(spec.Agg)).
		Add(float64(results[0].Count))
}

type lastOrFirstLastResult struct {
	Value    float64   `bson:"value"`
	First    float64   `bson:"first"`
	Last     float64   `bson:"last"`
	DataAsOf time.Time `bson:"dataAsOf"`
}

func (r *Repository) scalarLast(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) (domain.MetricResult, *time.Time, error) {
	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(false),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: "$last", Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)
	var results []numericGroupResult
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Value, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) scalarDelta(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) (domain.MetricResult, *time.Time, error) {
	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, spec.AasPath),
		isNumberMatch(false),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "first", Value: bson.D{{Key: "$first", Value: "$payload.value"}}},
			{Key: "last", Value: bson.D{{Key: "$last", Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)
	var results []struct {
		First       float64   `bson:"first"`
		Last        float64   `bson:"last"`
		SampleCount int64     `bson:"sampleCount"`
		DataAsOf    time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	// La resta se hace aca, no en el pipeline (spec, seccion "Traduccion a
	// pipeline de Mongo"): delta = last - first.
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Last - g.First, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) scalarCount(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	matchStage := baseMatch(tenantID, machineID, from, to, spec.AasPath)
	if filter != nil && filter.ValueEquals != nil {
		matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
	}
	pipeline := mongodriver.Pipeline{
		matchStage,
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, err
	}
	defer cur.Close(ctx)
	var results []struct {
		Value    int64     `bson:"value"`
		DataAsOf time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, err
	}
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: float64(g.Value), SampleCount: g.Value}, &dataAsOf, nil
}

// unavailable implementa domain/metrics.Repository sin Mongo real detras —
// mismo patron que measurements.Unavailable, para que un Mongo caido al
// arrancar deje /dashboards/metrics degradado (500) en vez de tumbar el
// resto de la API.
type unavailable struct{ err error }

func Unavailable(err error) domain.Repository { return &unavailable{err: err} }

func (u *unavailable) Scalar(context.Context, string, string, time.Time, time.Time, domain.MetricSpec, *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	return domain.MetricResult{}, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Series(context.Context, string, string, time.Time, time.Time, domain.Bucket, domain.MetricSpec, *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Raw(context.Context, string, string, time.Time, time.Time, string, int, int) ([]domain.RawPoint, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Grouped(context.Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Catalog(context.Context, string, string) ([]string, error) {
	return nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
