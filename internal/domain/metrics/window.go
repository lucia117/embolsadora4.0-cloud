package metrics

import (
	"fmt"
	"time"
)

// ResolveWindow resuelve `range` o el par `from`/`to` a un rango absoluto.
// `now` es inyectado por el llamador (nunca time.Now() adentro de esta
// funcion) para que sea testeable de forma determinista.
func ResolveWindow(q MetricQuery, now time.Time) (from, to time.Time, err error) {
	hasRange := q.Range != ""
	hasAbsolute := q.From != nil && q.To != nil
	hasPartialAbsolute := (q.From != nil) != (q.To != nil)

	switch {
	case hasRange && (hasAbsolute || hasPartialAbsolute):
		return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, "no se puede combinar range con from/to")
	case hasRange:
		dur, ok := rangeDurations[q.Range]
		if !ok {
			return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, fmt.Sprintf("range invalido: %q", q.Range))
		}
		return now.Add(-dur), now, nil
	case hasAbsolute:
		if !q.To.After(*q.From) {
			return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, "to debe ser posterior a from")
		}
		return *q.From, *q.To, nil
	default:
		return time.Time{}, time.Time{}, newValidationError(CodeInvalidParams, "falta range o el par from y to")
	}
}
