package errors

// NotFound represents a 404 error.
type NotFound struct {
	message string
}

// NewNotFound creates a NotFound error with the provided message.
func NewNotFound(msg string) NotFound {
	return NotFound{message: msg}
}

// Error implements the error interface.
func (e NotFound) Error() string {
	return e.message
}
