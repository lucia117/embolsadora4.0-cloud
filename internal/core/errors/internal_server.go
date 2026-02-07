package errors

// InternalServerError represents a 500 error.
type InternalServerError struct {
	message string
}

// NewInternalServerError creates an InternalServerError with the provided message.
func NewInternalServerError(msg string) InternalServerError {
	return InternalServerError{message: msg}
}

// Error implements the error interface.
func (e InternalServerError) Error() string {
	return e.message
}
