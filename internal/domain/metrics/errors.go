package metrics

const (
	CodeInvalidParams   = "INVALID_PARAMS"
	CodeTooManyMetrics  = "TOO_MANY_METRICS"
	CodeRawModeConflict = "RAW_MODE_CONFLICT"
	CodeGroupByConflict = "GROUP_BY_CONFLICT"
	CodeRangeTooWide    = "RANGE_TOO_WIDE"
	CodeTooManyGroups   = "TOO_MANY_GROUPS"
	CodeTooManyQueries  = "TOO_MANY_QUERIES"
	CodeQueryTimeout    = "QUERY_TIMEOUT"
)

// ValidationError es el unico tipo de error que devuelven Validate,
// ResolveWindow y ValidateBatchIDs. El handler lo mapea 1:1 a
// {success:false, error, code} con errors.As.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func newValidationError(code, message string) *ValidationError {
	return &ValidationError{Code: code, Message: message}
}
