package errors

// TooManyRequests represents a 429 error.
type TooManyRequests struct {
	message string
}

// NewTooManyRequests creates a TooManyRequests error with the provided message.
func NewTooManyRequests(msg string) TooManyRequests {
	return TooManyRequests{message: msg}
}

// Error implements the error interface.
func (e TooManyRequests) Error() string {
	return e.message
}
