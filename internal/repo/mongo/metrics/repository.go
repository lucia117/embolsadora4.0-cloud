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

// applyValueFilter agrega filter.ValueEquals (si esta seteado) como una
// condicion literal sobre "payload.value" al $match ya armado por baseMatch.
// Compartido por Scalar/Series/Grouped (I4b) para que filter.valueEquals se
// aplique consistentemente en todos los modos que reciben un
// *domain.ValueFilter -- antes solo lo honraban scalarCount, el branch count
// de Series y (para cualquier agg) Grouped.
func applyValueFilter(matchStage bson.D, filter *domain.ValueFilter) bson.D {
	if filter == nil || filter.ValueEquals == nil {
		return matchStage
	}
	matchStage[0].Value = append(matchStage[0].Value.(bson.D), bson.E{Key: "payload.value", Value: filter.ValueEquals})
	return matchStage
}

// wrapTimeoutErr traduce un context.DeadlineExceeded (disparado por el
// context.WithTimeout(ctx, r.maxTime) de cada metodo) a un
// *domain.ValidationError con CodeQueryTimeout, que HandleError ya sabia
// mapear a HTTP 504 pero que nada construia (I3/C4). Cualquier otro error
// pasa sin tocar.
func wrapTimeoutErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &domain.ValidationError{Code: domain.CodeQueryTimeout, Message: "la consulta excedio el tiempo maximo permitido"}
	}
	return err
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
		return r.scalarNumeric(ctx, tenantID, machineID, from, to, spec, filter)
	case domain.AggLast:
		return r.scalarLast(ctx, tenantID, machineID, from, to, spec, filter)
	case domain.AggDelta:
		return r.scalarDelta(ctx, tenantID, machineID, from, to, spec, filter)
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

func (r *Repository) scalarNumeric(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	accumulator, ok := numericAccumulator(spec.Agg)
	if !ok {
		return domain.MetricResult{}, nil, fmt.Errorf("agg no numerico: %s", spec.Agg)
	}

	pipeline := mongodriver.Pipeline{
		applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter),
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
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)

	var results []numericGroupResult
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}

	// Discard count: consulta liviana separada (mismo $match, $count) para no
	// complicar el pipeline principal con un $facet. Best-effort: si esta
	// falla, no se pierde el resultado principal, solo la instrumentacion.
	// Se dispara DESPUES del aggregate principal (no antes) para que, si come
	// el resto del presupuesto de ctx/maxTime, no pueda convertir un query
	// valido en QUERY_TIMEOUT -- ver comentario de Copilot en Series().
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec, filter)

	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Value, SampleCount: g.SampleCount}, &dataAsOf, nil
}

// reportNonNumericDiscards issues a second, synchronous Mongo aggregate per
// numeric Scalar/Series call -- a deliberate trade-off (best-effort
// telemetry, simplest pipeline shape), not an oversight, but it does mean
// every numeric query pays a second round-trip and Batch's real Mongo call
// volume is ~2x its documented worst case (see the comment on Batch in
// app/dashboards/service.go). Making this async would cut response latency
// but let these best-effort aggregates run outside Batch's g.SetLimit(10),
// which could raise peak concurrent Mongo connections instead of lowering
// load -- not a clear win, so left synchronous pending real usage data.
// Must always be called after the primary aggregate on a shared ctx (never
// before), and must apply the same filter, so it can't (a) steal maxTime
// budget from the query that actually answers the request, or (b) count
// non-numeric documents that filter.ValueEquals would have excluded anyway.
func (r *Repository) reportNonNumericDiscards(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) {
	pipeline := mongodriver.Pipeline{
		applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter),
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

func (r *Repository) scalarLast(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	pipeline := mongodriver.Pipeline{
		applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter),
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
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)
	var results []numericGroupResult
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec, filter)
	if len(results) == 0 {
		return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg}, nil, nil
	}
	g := results[0]
	dataAsOf := g.DataAsOf
	return domain.MetricResult{AasPath: spec.AasPath, Agg: spec.Agg, Value: g.Value, SampleCount: g.SampleCount}, &dataAsOf, nil
}

func (r *Repository) scalarDelta(ctx context.Context, tenantID, machineID string, from, to time.Time, spec domain.MetricSpec, filter *domain.ValueFilter) (domain.MetricResult, *time.Time, error) {
	pipeline := mongodriver.Pipeline{
		applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter),
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
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)
	var results []struct {
		First       float64   `bson:"first"`
		Last        float64   `bson:"last"`
		SampleCount int64     `bson:"sampleCount"`
		DataAsOf    time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}
	r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec, filter)
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
	pipeline := mongodriver.Pipeline{
		applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter),
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)
	var results []struct {
		Value    int64     `bson:"value"`
		DataAsOf time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &results); err != nil {
		return domain.MetricResult{}, nil, wrapTimeoutErr(err)
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
		matchStage := applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter)
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: truncExpr},
			{Key: "value", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
		return r.runSeriesPipeline(ctx, mongodriver.Pipeline{matchStage, groupStage})
	default:
		accumulator, ok := numericAccumulator(spec.Agg)
		if !ok {
			return nil, nil, fmt.Errorf("agg no soportado en modo serie: %s", spec.Agg)
		}
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: truncExpr},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "sampleCount", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
		pipeline := mongodriver.Pipeline{
			applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter),
			isNumberMatch(false),
			groupStage,
		}
		// La telemetria corre DESPUES del pipeline principal, no antes: ambos
		// comparten ctx/maxTime, y si corriera primero podria agotar el
		// presupuesto de tiempo del query real y devolver QUERY_TIMEOUT para
		// un request que hubiera andado bien (hallazgo de Copilot en PR #78).
		points, dataAsOf, err := r.runSeriesPipeline(ctx, pipeline)
		r.reportNonNumericDiscards(ctx, tenantID, machineID, from, to, spec, filter)
		return points, dataAsOf, err
	}
}

func (r *Repository) runSeriesPipeline(ctx context.Context, pipeline mongodriver.Pipeline) ([]domain.BucketPoint, *time.Time, error) {
	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}})
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)

	var rows []struct {
		Ts          time.Time `bson:"_id"`
		Value       float64   `bson:"value"`
		SampleCount int64     `bson:"sampleCount"`
		DataAsOf    time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, wrapTimeoutErr(err)
	}

	// dataAsOf se calcula sobre row.DataAsOf (el "$max":"$ts" real de cada
	// bucket), no sobre row.Ts (el limite inferior del bucket que da
	// $dateTrunc) -- un sample nuevo dentro del bucket mas reciente no mueve
	// row.Ts, asi que usarlo dejaba dataAsOf pegado al inicio del bucket en
	// vez de reflejar la medicion mas reciente.
	points := make([]domain.BucketPoint, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		points = append(points, domain.BucketPoint{Ts: row.Ts, Value: row.Value, SampleCount: row.SampleCount})
		if dataAsOf == nil || row.DataAsOf.After(*dataAsOf) {
			t := row.DataAsOf
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
func (r *Repository) Raw(ctx context.Context, tenantID, machineID string, from, to time.Time, aasPath string, filter *domain.ValueFilter, limit, maxPoints int) ([]domain.RawPoint, *time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, r.maxTime)
	defer cancel()

	pipeline := mongodriver.Pipeline{
		applyValueFilter(baseMatch(tenantID, machineID, from, to, aasPath), filter),
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ts", Value: 1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	}
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)

	var rows []struct {
		Ts      time.Time `bson:"ts"`
		Payload struct {
			Value any `bson:"value"`
		} `bson:"payload"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, wrapTimeoutErr(err)
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
	// no cambia. I5a: decimar SOLO si el fetch no llego al cap (len(points) <
	// limit) -- si llego al cap, no sabemos si la ventana real tiene mas
	// puntos que los primeros `limit` (los mas viejos), asi que decimar aca
	// devolveria una vista silenciosamente truncada. En ese caso el llamador
	// (app/dashboards.queryRaw) debe rechazar con RANGE_TOO_WIDE en vez de
	// aceptar una respuesta parcial.
	if maxPoints > 0 && len(points) > maxPoints && len(points) < limit {
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
		// El ultimo indice se fuerza a len(points)-1 en vez de confiar en
		// int(float64(i)*step): para algunos n/maxPoints el redondeo de
		// punto flotante deja i==maxPoints-1 en len(points)-2 (p.ej.
		// n=16,maxPoints=12 -> 11*15/11 evalua a 14.999... en vez de 15),
		// violando la garantia documentada de preservar el ultimo punto.
		if i == maxPoints-1 {
			out = append(out, points[len(points)-1])
			continue
		}
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

	matchStage := applyValueFilter(baseMatch(tenantID, machineID, from, to, spec.AasPath), filter)

	pipeline := mongodriver.Pipeline{matchStage}

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
		// A diferencia de Scalar/Series, este isNumberMatch faltaba: sin el,
		// un payload.value no numerico entra al acumulador de avg/sum/min/max
		// -- BSON ordena string > cualquier numero, asi que $max/$min puede
		// devolver un string y romper el decode a float64, o inflar
		// sampleCount en avg/sum.
		pipeline = append(pipeline, isNumberMatch(false))
		groupStage = bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: fieldRef},
			{Key: "value", Value: bson.D{{Key: accumulator, Value: "$payload.value"}}},
			{Key: "dataAsOf", Value: bson.D{{Key: "$max", Value: "$ts"}}},
		}}}
	}

	pipeline = append(pipeline,
		groupStage,
		bson.D{{Key: "$sort", Value: bson.D{{Key: "value", Value: -1}}}},
		bson.D{{Key: "$limit", Value: limit}},
	)
	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, nil, wrapTimeoutErr(err)
	}
	defer cur.Close(ctx)

	var rows []struct {
		Key      any       `bson:"_id"`
		Value    float64   `bson:"value"`
		DataAsOf time.Time `bson:"dataAsOf"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, nil, wrapTimeoutErr(err)
	}

	groups := make([]domain.GroupResult, 0, len(rows))
	var dataAsOf *time.Time
	for _, row := range rows {
		// Convertir Key a string — puede ser string directamente o numeric si se agrupó por campo numérico.
		keyStr := fmt.Sprintf("%v", row.Key)
		groups = append(groups, domain.GroupResult{Key: keyStr, Value: row.Value})
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
		return nil, wrapTimeoutErr(err)
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
func (u *unavailable) Raw(context.Context, string, string, time.Time, time.Time, string, *domain.ValueFilter, int, int) ([]domain.RawPoint, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Grouped(context.Context, string, string, time.Time, time.Time, string, domain.MetricSpec, *domain.ValueFilter, int) ([]domain.GroupResult, *time.Time, error) {
	return nil, nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
func (u *unavailable) Catalog(context.Context, string, string) ([]string, error) {
	return nil, fmt.Errorf("mongo no disponible desde el arranque: %w", u.err)
}
