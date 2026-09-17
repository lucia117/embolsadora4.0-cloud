package metrics

import "fmt"

// ValidateBatchIDs valida el batch a nivel de ids: no vacio, no mas del
// limite, cada id no vacio y unico. La validacion de cada MetricQuery
// individual la hace Validate, item por item, en app/dashboards (Task 13).
func ValidateBatchIDs(ids []string, limits Limits) error {
	if len(ids) == 0 {
		return newValidationError(CodeInvalidParams, "queries es requerido y no puede estar vacio")
	}
	if len(ids) > limits.MaxBatchQueries {
		return newValidationError(CodeTooManyQueries, fmt.Sprintf("queries excede el maximo de %d", limits.MaxBatchQueries))
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			return newValidationError(CodeInvalidParams, "cada query del batch requiere un id")
		}
		if _, dup := seen[id]; dup {
			return newValidationError(CodeInvalidParams, fmt.Sprintf("id duplicado en el batch: %q", id))
		}
		seen[id] = struct{}{}
	}
	return nil
}
