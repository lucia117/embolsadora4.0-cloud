package errors

// BadRequest represents a 400 error.
type BadRequest struct {
	message string
}

// NewBadRequest creates a BadRequest error with the provided message.
func NewBadRequest(msg string) BadRequest {
	return BadRequest{message: msg}
}

// Error implements the error interface.
func (e BadRequest) Error() string {
	return e.message
}
