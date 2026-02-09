package errors

// Unauthorized represents a 401 error.
type Unauthorized struct {
	message string
}

// NewUnauthorized creates an Unauthorized error with the provided message.
func NewUnauthorized(msg string) Unauthorized {
	return Unauthorized{message: msg}
}

// Error implements the error interface.
func (e Unauthorized) Error() string {
	return e.message
}
