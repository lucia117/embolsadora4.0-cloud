package metrics

import (
	"context"
	"time"
)

type MetricResult struct {
	AasPath     string
	Agg         Agg
	Value       float64
	SampleCount int64
}

type BucketPoint struct {
	Ts          time.Time
	Value       float64
	SampleCount int64
}

type RawPoint struct {
	Ts    time.Time
	Value any
}

type GroupResult struct {
	Key   string
	Value float64
}

type SeriesPoint struct {
	Ts      time.Time
	Results []MetricResult
}

// QueryResult es la union discriminada por Mode que arma app/dashboards.
// Solo los campos relevantes al Mode detectado vienen poblados.
type QueryResult struct {
	Mode      Mode
	MachineID string
	From      time.Time
	To        time.Time
	DataAsOf  *time.Time

	Bucket  Bucket
	Results []MetricResult

	Series []SeriesPoint

	AasPath string
	Points  []RawPoint

	Agg     Agg
	GroupBy string
	Groups  []GroupResult
}

// Repository resuelve, para UNA combinacion (tenant, machine, MetricSpec,
// ventana), el pipeline de Mongo que corresponde al modo pedido. No conoce
// el concepto de "query completa" — ese fan-out multi-metrica lo hace
// app/dashboards.Service con errgroup.
type Repository interface {
	// Scalar resuelve avg/sum/min/max/last/delta/count sin bucketizar.
	Scalar(ctx context.Context, tenantID, machineID string, from, to time.Time, spec MetricSpec, filter *ValueFilter) (MetricResult, *time.Time, error)

	// Series resuelve la misma familia de aggs que Scalar, bucketizada.
	Series(ctx context.Context, tenantID, machineID string, from, to time.Time, bucket Bucket, spec MetricSpec, filter *ValueFilter) ([]BucketPoint, *time.Time, error)

	// Raw devuelve puntos crudos ordenados por ts, honrando filter.ValueEquals
	// igual que Scalar/Series/Grouped. limit es maxRawPoints+1 (para poder
	// distinguir "hay exactamente el maximo" de "hay mas"); si maxPoints > 0,
	// decima a maxPoints en vez de fallar.
	Raw(ctx context.Context, tenantID, machineID string, from, to time.Time, aasPath string, filter *ValueFilter, limit, maxPoints int) ([]RawPoint, *time.Time, error)

	// Grouped agrupa por groupBy con el acumulador de spec.Agg. limit es
	// maxGroups+1, misma logica de deteccion de exceso que Raw.
	Grouped(ctx context.Context, tenantID, machineID string, from, to time.Time, groupBy string, spec MetricSpec, filter *ValueFilter, limit int) ([]GroupResult, *time.Time, error)

	// Catalog devuelve los aasPath observados para (tenant, machine).
	Catalog(ctx context.Context, tenantID, machineID string) ([]string, error)
}
