package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	from := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	to := from.Add(8 * time.Hour)
	oneMetric := []MetricSpec{{AasPath: "Operativos/Pesada/peso", Agg: AggAvg}}
	twoMetrics := []MetricSpec{
		{AasPath: "Operativos/Pesada/peso", Agg: AggAvg},
		{AasPath: "Operativos/Pesada/peso", Agg: AggCount},
	}

	tests := []struct {
		name string
		q    MetricQuery
		// strictSpecs corre el caso con MaxSpecs:1 en vez del limite
		// compartido de abajo (MaxSpecs:2) — sirve para disparar
		// TOO_MANY_METRICS con datos minimos (2 metrics identicas) sin que
		// twoMetrics (tambien 2 elementos, pero con aggs distintos) dispare
		// el mismo guardrail en el resto de los casos.
		strictSpecs bool
		wantMode    Mode
		wantCode    string
	}{
		{
			name:     "escalar multi-metrica sin bucket",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: twoMetrics},
			wantMode: ModeScalar,
		},
		{
			name:     "serie con bucket",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: twoMetrics, Bucket: Bucket1h},
			wantMode: ModeSeries,
		},
		{
			name:     "raw solo",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}}},
			wantMode: ModeRaw,
		},
		{
			name:     "raw con bucket es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}}, Bucket: Bucket1h},
			wantCode: CodeRawModeConflict,
		},
		{
			name:     "raw con mas de una metrica es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}, {AasPath: "y", Agg: AggAvg}}},
			wantCode: CodeRawModeConflict,
		},
		{
			name:     "groupBy solo",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, GroupBy: "payload.tipo"},
			wantMode: ModeGrouped,
		},
		{
			name:     "groupBy con bucket es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, GroupBy: "payload.tipo", Bucket: Bucket1h},
			wantCode: CodeGroupByConflict,
		},
		{
			name:     "groupBy con mas de una metrica es conflicto",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: twoMetrics, GroupBy: "payload.tipo"},
			wantCode: CodeGroupByConflict,
		},
		{
			name:     "groupBy con campo fuera del patron payload.<campo>",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, GroupBy: "$where"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "machineId faltante",
			q:        MetricQuery{Metrics: oneMetric},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "metrics vacio",
			q:        MetricQuery{MachineID: "EMB-DEV-001"},
			wantCode: CodeInvalidParams,
		},
		{
			name:        "mas metricas que el limite",
			q:           MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{oneMetric[0], oneMetric[0]}},
			strictSpecs: true,
			wantCode:    CodeTooManyMetrics,
		},
		{
			name:     "agg invalido",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: "bogus"}}},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "demasiados buckets esperados",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, Bucket: Bucket1m},
			wantCode: CodeRangeTooWide,
		},
		{
			name:     "bucket con agg last no soportado",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggLast}}, Bucket: Bucket1h},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "bucket con agg delta no soportado",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggDelta}}, Bucket: Bucket1h},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "groupBy con agg last no soportado",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggLast}}, GroupBy: "payload.tipo"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "groupBy con agg delta no soportado",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggDelta}}, GroupBy: "payload.tipo"},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "filter.valueEquals objeto es invalido (inyeccion de operador)",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: oneMetric, Filter: &ValueFilter{ValueEquals: map[string]any{"$regex": "(a+)+$"}}},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "maxPoints negativo es invalido",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}}, MaxPoints: -1},
			wantCode: CodeInvalidParams,
		},
		{
			name:     "maxPoints excede el maximo de puntos crudos",
			q:        MetricQuery{MachineID: "EMB-DEV-001", Metrics: []MetricSpec{{AasPath: "x", Agg: AggRaw}}, MaxPoints: 5001},
			wantCode: CodeInvalidParams,
		},
	}

	// MaxSpecs:2 alcanza para twoMetrics (2 elementos con aggs distintos,
	// usado en varios casos de exito/conflicto); el caso "mas metricas que
	// el limite" pisa esto a 1 via strictSpecs para poder disparar
	// TOO_MANY_METRICS con solo 2 metrics identicas.
	limits := Limits{MaxSpecs: 2, MaxBuckets: 100, MaxRawPoints: 5000, MaxGroups: 200, MaxBatchQueries: 50}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caseLimits := limits
			if tt.strictSpecs {
				caseLimits.MaxSpecs = 1
			}
			mode, err := Validate(tt.q, from, to, caseLimits)
			if tt.wantCode != "" {
				require.Error(t, err)
				var ve *ValidationError
				require.ErrorAs(t, err, &ve)
				assert.Equal(t, tt.wantCode, ve.Code)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMode, mode)
		})
	}
}
