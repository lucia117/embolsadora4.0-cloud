// Package metrics implementa domain/metrics.Repository sobre MongoDB.
package metrics

import (
	"context"
	"errors"
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

// dateTruncUnit mapea Bucket a la unidad que espera $dateTrunc. Bucket es un
// tipo de domain/metrics; su duration() no esta exportada a proposito
// (domain no conoce Mongo), asi que este mapeo vive aca, no ahi.
func dateTruncUnit(b domain.Bucket) (unit string, binSize int, ok bool) {
	switch b {
	case domain.Bucket1m:
		return "minute", 1, true
	case domain.Bucket5m:
		return "minute", 5, true
	case domain.Bucket15m:
		return "minute", 15, true
	case domain.Bucket1h:
		return "hour", 1, true
	case domain.Bucket6h:
		return "hour", 6, true
	case domain.Bucket1d:
		return "day", 1, true
	}
	return "", 0, false
}

// Series resuelve avg/sum/min/max/count bucketizados por Bucket via $dateTrunc.
func (r *Repository) Series(ctx context.Context, tenantID, machineID string, from, to time.Time, bucket domain.Bucket, spec domain.MetricSpec, filter *domain.ValueFilter) ([]domain.BucketPoint, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	unit, binSize, ok := dateTruncUnit(bucket)
	if !ok {
		return nil, nil, fmt.Errorf("bucket no soportado: %s", bucket)
	}

	truncExpr := bson.D{{Key: "$dateTrunc", Value: bson.D{
		{Key: "date", Value: "$ts"},
		{Key: "unit", Value: unit},
		{Key: "binSize", Value: binSize},
	}}}

	var groupStage bson.D
	switch spec.Agg {
	case domain.AggCount:
		matchStage := baseMatch(tenantID, machineID, from, to, spec.AasPath)
		if filter != nil && filter.ValueEquals != nil {
			matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: truncExpr},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}}
		return r.runSeriesPipeline(ctx, mongodriver.Pipeline{matchStage, groupStage}, tenantID, machineID, from, to, spec)
	default:
		accumulator, ok := numericAccumulator(spec.Agg)
		if !ok {
			return nil, nil, fmt.Errorf("agg no soportado en modo serie: %s", spec.Agg)
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: truncExpr},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}}
		pipeline := mongodriver.Pipeline{
			baseMatch(tenantID, machineID, from, to, spec.AasPath),
			isNumberMatch(false),
			groupStage,
		}
		r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec)
		return r.runSeriesPipeline(ctx, pipeline, tenantID, machineID, from, to, spec)
	}
}

func (r *Repository) runSeriesPipeline(ctx context.Context, pipeline mongodriver.Pipeline, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec) ([]domain.BucketPoint, *time.Time, error) {
	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}})
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		Ts          time.Time `bson:"_id"`
		Value       float64   `bson:"value"`
		SampleCount int64     `bson:"sampleCount"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	points := make([]domain.BucketPoint, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		points = append(points, domain.BucketPoint{Ts: row.Ts, Value: row.Value, SampleCount: row.SampleCount})
		if dataAsOf == nil || row.Ts.After(*dataAsOf) {
			t := row.Ts
			dataAsOf = &t
		}
	}
	return points, dataAsOf, nil
}

// Raw devuelve puntos crudos (sin agregar) ordenados por ts ascendente, hasta
// limit documentos. El llamador (Task 12) pasa limit = maxRawPoints+1 para
// poder distinguir "hay exactamente el maximo" (posible mas datos de los
// permitidos, rechazar con RANGE_TOO_WIDE) de "menos que el limite"
// (definitivamente se trajo todo) comparando len(points) == limit — esta
// funcion no decide esa politica, solo devuelve lo que encuentra hasta
// limit y, si maxPoints > 0, decima a maxPoints antes de devolver.
func (r *Repository) Raw(ctx context.Context, tenantID, machineID string, from, to time.Time, aasPath string, limit, maxPoints int) ([]domain.RawPoint, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	pipeline := mongodriver.Pipeline{
		baseMatch(tenantID, machineID, from, to, aasPath),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		Ts      time.Time `bson:"ts"`
		Payload struct {
			Value any `bson:"value"`
		} `bson:"payload"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	points := make([]domain.RawPoint, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		points = append(points, domain.RawPoint{Ts: row.Ts, Value: row.Payload.Value})
		if dataAsOf == nil || row.Ts.After(*dataAsOf) {
			t := row.Ts
			dataAsOf = &t
		}
	}

	// fork/C3: si maxPoints viene seteado, decimar en vez de dejar que el
	// llamador (app/dashboards) rechace con RANGE_TOO_WIDE. len(points) ==
	// limit sigue siendo la senal de "hay mas de los que se pidieron" — eso
	// no cambia; lo que cambia es que aca mismo se recorta a maxPoints en
	// vez de devolver el excedente sin recortar.
	if maxPoints > 0 && len(points) > maxPoints {
		points = decimateUniform(points, maxPoints)
	}

	return points, dataAsOf, nil
}

// decimateUniform reduce points a como maximo maxPoints elementos con
// muestreo por stride uniforme, preservando siempre el primer y el ultimo
// punto original. Decimacion simple (no LTTB): correcta y suficiente para
// v1, mas facil de razonar; upgradeable despues sin cambiar el contrato
// (fork/C3 de la spec).
func decimateUniform(points []domain.RawPoint, maxPoints int) []domain.RawPoint {
	if len(points) <= maxPoints {
		return points
	}
	if maxPoints < 1 {
		return points
	}
	if maxPoints == 1 {
		// Un solo punto permitido: el mas reciente, no el mas viejo ni el
		// slice entero (bug de la v1 del brief: `maxPoints < 2` devolvia
		// todo sin recortar, violando el contrato documentado).
		return points[len(points)-1:]
	}
	step := float64(len(points)-1) / float64(maxPoints-1)
	out := make([]domain.RawPoint, 0, maxPoints)
	for i := 0; i < maxPoints; i++ {
		idx := int(float64(i) * step)
		if idx >= len(points) {
			idx = len(points) - 1
		}
		out = append(out, points[idx])
	}
	return out
}

// Grouped agrupa por groupBy (una referencia de campo tipo "payload.xxx")
// con el acumulador de spec.Agg, ordena descendente por value y trunca a
// limit grupos (misma convencion que Raw: el llamador pasa maxGroups+1 para
// poder distinguir "hay exactamente el maximo" de "hay mas" y rechazar con
// TOO_MANY_GROUPS).
//
// groupBy ya fue validado contra groupByFieldPattern en domain/metrics
// (Task 3) antes de llegar aca — este repositorio confia en esa validacion
// previa para construir la referencia de campo "$"+groupBy sin volver a
// validarla.
func (r *Repository) Grouped(ctx context.Context, tenantID, machineID string, from, to time.Time, groupBy string, spec domain.MetricSpec, filter *domain.ValueFilter, limit int) ([]domain.GroupResult, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	fieldRef := "$" + groupBy

	matchStage := baseMatch(tenantID, machineID, from, to, spec.AasPath)
	if filter != nil && filter.ValueEquals != nil {
		matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
	}

	var groupStage bson.D
	if spec.Agg == domain.AggCount {
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: fieldRef},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
	} else {
		accumulator, ok := numericAccumulator(spec.Agg)
		if !ok {
			return nil, nil, fmt.Errorf("agg no soportado en modo agrupado: %s", spec.Agg)
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: fieldRef},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
	}

	pipeline := mongodriver.Pipeline{
		matchStage,
		groupStage,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "value", Value: -1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, err
	}
	defer cur.Close(ctx)

	var rows []struct {
		Key      string    `bson:"_id"`
		Value    float64   `bson:"value"`
		DataAsOf time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, err
	}

	groups := make([]domain.GroupResult, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		groups = append(groups, domain.GroupResult{Key: row.Key, Value: row.Value})
		if dataAsOf == nil || row.DataAsOf.After(*dataAsOf) {
			t := row.DataAsOf
			dataAsOf = &t
		}
	}
	return groups, dataAsOf, nil
}

// Catalog devuelve los aasPath observados para (tenant, machine), via
// Distinct nativo (no aggregation pipeline).
func (r *Repository) Catalog(ctx context.Context, tenantID, machineID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	filter := bson.D{{Key: "tenantId", Value: tenantID}, {Key: "machineId", Value: machineID}}
	// mongo-driver v2: Distinct devuelve *DistinctResult (no ([]any, error)
	// como en v1) — se decodifica al tipo esperado con Decode.
	var raw []any
	if err := r.coll.Distinct(ctx, "payload.aasPath", filter).Decode(&raw); err != nil {
		if errors.Is(err, mongodriver.ErrNoDocuments) {
			return []string{}, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			paths = append(paths, s)
		}
	}
	return paths, nil
}

// Compile-time check: *Repository debe satisfacer domain.Repository ahora
// que las 5 operaciones estan implementadas.
var _ domain.Repository = (*Repository)(nil)

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
