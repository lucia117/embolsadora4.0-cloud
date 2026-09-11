package metrics

import (
	"fmt"
	"regexp"
	"time"
)

// groupByFieldPattern es la unica defensa contra inyeccion NoSQL vía
// groupBy: el string del cliente se interpola como "$"+groupBy en un $group
// (repo/mongo/metrics). Sin este patron, un valor como "$where" o un
// operador de agregacion podria leerse como algo distinto de un nombre de
// campo.
var groupByFieldPattern = regexp.MustCompile(`^payload\.[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]+)*$`)

// Validate corre las reglas de exclusion mutua y los guardrails
// estructurales (los que no requieren tocar Mongo) y devuelve el Mode
// detectado. RANGE_TOO_WIDE por exceso de puntos crudos y TOO_MANY_GROUPS se
// validan en el repositorio (Task 8/9): dependen del resultado real de la
// consulta, no de la forma del request.
func Validate(q MetricQuery, from, to time.Time, limits Limits) (Mode, error) {
	if q.MachineID == "" {
		return "", newValidationError(CodeInvalidParams, "machineId es requerido")
	}
	if len(q.Metrics) == 0 {
		return "", newValidationError(CodeInvalidParams, "metrics es requerido y no puede estar vacio")
	}
	if len(q.Metrics) > limits.MaxSpecs {
		return "", newValidationError(CodeTooManyMetrics, fmt.Sprintf("metrics excede el maximo de %d", limits.MaxSpecs))
	}
	for _, m := range q.Metrics {
		if m.AasPath == "" {
			return "", newValidationError(CodeInvalidParams, "aasPath es requerido en cada metric")
		}
		if !m.Agg.Valid() {
			return "", newValidationError(CodeInvalidParams, fmt.Sprintf("agg invalido: %q", m.Agg))
		}
	}
	if q.Bucket != "" && !q.Bucket.valid() {
		return "", newValidationError(CodeInvalidParams, fmt.Sprintf("bucket invalido: %q", q.Bucket))
	}
	if q.GroupBy != "" && !groupByFieldPattern.MatchString(q.GroupBy) {
		return "", newValidationError(CodeInvalidParams, "groupBy no cumple el patron payload.<campo>")
	}
	if q.Filter != nil && q.Filter.ValueEquals != nil {
		switch q.Filter.ValueEquals.(type) {
		case bool, string, float64:
			// ok — unicos tipos que encoding/json produce para un primitivo JSON.
		default:
			return "", newValidationError(CodeInvalidParams, "filter.valueEquals debe ser bool, string o number")
		}
	}
	if q.MaxPoints < 0 || q.MaxPoints > limits.MaxRawPoints {
		return "", newValidationError(CodeInvalidParams, "maxPoints debe estar entre 1 y el maximo de puntos crudos permitido")
	}

	isRaw := len(q.Metrics) == 1 && q.Metrics[0].Agg == AggRaw
	isGrouped := q.GroupBy != ""

	switch {
	case hasRawAmongMultiple(q.Metrics):
		return "", newValidationError(CodeRawModeConflict, "agg:raw no admite bucket, groupBy ni mas de una metrica")
	case isRaw:
		if q.Bucket != "" || isGrouped {
			return "", newValidationError(CodeRawModeConflict, "agg:raw no admite bucket, groupBy ni mas de una metrica")
		}
		return ModeRaw, nil
	case isGrouped:
		if q.Bucket != "" || len(q.Metrics) != 1 {
			return "", newValidationError(CodeGroupByConflict, "groupBy requiere exactamente 1 metrica y no admite bucket")
		}
		if agg := q.Metrics[0].Agg; agg == AggLast || agg == AggDelta {
			return "", newValidationError(CodeInvalidParams, "agg no soportado en modo agrupado: last/delta")
		}
		return ModeGrouped, nil
	case q.Bucket != "":
		if err := checkBucketCount(from, to, q.Bucket, limits.MaxBuckets); err != nil {
			return "", err
		}
		for _, m := range q.Metrics {
			if m.Agg == AggLast || m.Agg == AggDelta {
				return "", newValidationError(CodeInvalidParams, "agg no soportado en modo serie: last/delta")
			}
		}
		return ModeSeries, nil
	default:
		return ModeScalar, nil
	}
}

// hasRawAmongMultiple detecta agg:"raw" mezclado con otras metricas (caso
// "raw con mas de una metrica" del test) — isRaw exige exactamente 1 metrica,
// asi que ese caso no cae en la rama isRaw y necesita chequearse aparte.
func hasRawAmongMultiple(specs []MetricSpec) bool {
	if len(specs) < 2 {
		return false
	}
	for _, s := range specs {
		if s.Agg == AggRaw {
			return true
		}
	}
	return false
}

func checkBucketCount(from, to time.Time, bucket Bucket, maxBuckets int) error {
	dur, _ := bucket.duration()
	expected := int(to.Sub(from)/dur) + 1
	if expected > maxBuckets {
		return newValidationError(CodeRangeTooWide, fmt.Sprintf("el rango produce %d buckets, maximo %d", expected, maxBuckets))
	}
	return nil
}
