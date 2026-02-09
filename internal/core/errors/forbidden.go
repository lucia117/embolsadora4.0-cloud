package errors

// Forbidden represents a 403 error.
type Forbidden struct {
	message string
}

// NewForbidden creates a Forbidden error with the provided message.
func NewForbidden(msg string) Forbidden {
	return Forbidden{message: msg}
}

// Error implements the error interface.
func (e Forbidden) Error() string {
	return e.message
}
