// Package metrics contiene el modelo de dominio del motor de consultas de
// dashboards. No conoce Mongo ni Gin.
package metrics

import "time"

type Agg string

const (
	AggAvg   Agg = "avg"
	AggSum   Agg = "sum"
	AggMin   Agg = "min"
	AggMax   Agg = "max"
	AggLast  Agg = "last"
	AggCount Agg = "count"
	AggDelta Agg = "delta"
	AggRaw   Agg = "raw"
)

// Valid reporta si a es uno de los 8 aggs soportados.
func (a Agg) Valid() bool {
	switch a {
	case AggAvg, AggSum, AggMin, AggMax, AggLast, AggCount, AggDelta, AggRaw:
		return true
	}
	return false
}

// Bucket es siempre UTC en v1 (fork/C1 del plan): no hay parametro de
// timezone. $dateTrunc en el pipeline de Mongo trunca con el reloj UTC.
type Bucket string

const (
	Bucket1m  Bucket = "1m"
	Bucket5m  Bucket = "5m"
	Bucket15m Bucket = "15m"
	Bucket1h  Bucket = "1h"
	Bucket6h  Bucket = "6h"
	Bucket1d  Bucket = "1d"
)

func (b Bucket) duration() (time.Duration, bool) {
	switch b {
	case Bucket1m:
		return time.Minute, true
	case Bucket5m:
		return 5 * time.Minute, true
	case Bucket15m:
		return 15 * time.Minute, true
	case Bucket1h:
		return time.Hour, true
	case Bucket6h:
		return 6 * time.Hour, true
	case Bucket1d:
		return 24 * time.Hour, true
	}
	return 0, false
}

func (b Bucket) valid() bool { _, ok := b.duration(); return ok }

type Range string

const (
	Range15m Range = "15m"
	Range30m Range = "30m"
	Range1h  Range = "1h"
	Range8h  Range = "8h"
	Range24h Range = "24h"
	Range7d  Range = "7d"
	Range30d Range = "30d"
)

var rangeDurations = map[Range]time.Duration{
	Range15m: 15 * time.Minute,
	Range30m: 30 * time.Minute,
	Range1h:  time.Hour,
	Range8h:  8 * time.Hour,
	Range24h: 24 * time.Hour,
	Range7d:  7 * 24 * time.Hour,
	Range30d: 30 * 24 * time.Hour,
}

type Mode string

const (
	ModeScalar  Mode = "scalar"
	ModeSeries  Mode = "series"
	ModeRaw     Mode = "raw"
	ModeGrouped Mode = "grouped"
)

type MetricSpec struct {
	AasPath string
	Agg     Agg
}

type ValueFilter struct {
	ValueEquals any
}

type MetricQuery struct {
	MachineID string
	Range     Range
	From      *time.Time
	To        *time.Time
	Bucket    Bucket
	Metrics   []MetricSpec
	GroupBy   string
	Filter    *ValueFilter
	// MaxPoints activa decimacion server-side en modo raw (fork/C3) en vez
	// de RANGE_TOO_WIDE. 0 = no seteado, se mantiene el hard-reject.
	MaxPoints int
}

// Limits son los guardrails de la tabla de la spec, parametrizados
// (fork/C8) en vez de constantes — el llamador (routes/url_mappings.go) los
// arma desde config.DashboardsConfig y los pasa a Validate/ValidateBatchIDs
// y al repositorio.
type Limits struct {
	MaxSpecs        int
	MaxBuckets      int
	MaxRawPoints    int
	MaxGroups       int
	MaxBatchQueries int
}

// DefaultLimits reproduce los numeros de la tabla de guardrails de la spec.
// Solo se usa en tests; en produccion los valores vienen siempre de config.
func DefaultLimits() Limits {
	return Limits{MaxSpecs: 10, MaxBuckets: 1000, MaxRawPoints: 5000, MaxGroups: 200, MaxBatchQueries: 50}
}
