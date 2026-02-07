package errors

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strings"
)

// WebError represents a standardized error response for web handlers.
type WebError struct {
	Status  int
	Code    string
	Message string
}

// NewWebError builds a WebError with the code derived from the HTTP status.
func NewWebError(status int, message string) *WebError {
	return &WebError{
		Status:  status,
		Code:    statusToCode(status),
		Message: message,
	}
}

// Error implements the error interface.
func (e *WebError) Error() string {
	return e.Message
}

// StatusCode returns the HTTP status code.
func (e *WebError) StatusCode() int {
	return e.Status
}

// MarshalJSON ensures the "error" field is derived from the HTTP status text.
func (e *WebError) MarshalJSON() ([]byte, error) {
	type payload struct {
		Status  int    `json:"status"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	return json.Marshal(payload{
		Status:  e.Status,
		Error:   statusToCode(e.Status),
		Message: e.Message,
	})
}

// ToWeb converts domain errors to WebError.
func ToWeb(err error) error {
	if err == nil {
		return nil
	}

	var webErr *WebError
	if stderrors.As(err, &webErr) {
		return webErr
	}

	switch {
	case isBadRequest(err):
		return NewWebError(http.StatusBadRequest, err.Error())
	case isUnauthorized(err):
		return NewWebError(http.StatusUnauthorized, err.Error())
	case isForbidden(err):
		return NewWebError(http.StatusForbidden, err.Error())
	case isNotFound(err):
		return NewWebError(http.StatusNotFound, err.Error())
	case isTooManyRequests(err):
		return NewWebError(http.StatusTooManyRequests, err.Error())
	case isInternalServer(err):
		return NewWebError(http.StatusInternalServerError, err.Error())
	default:
		return NewWebError(http.StatusInternalServerError, "internal error")
	}
}

func statusToCode(status int) string {
	text := http.StatusText(status)
	if text == "" {
		return "unknown"
	}

	return strings.ToLower(strings.ReplaceAll(text, " ", "_"))
}

func isBadRequest(err error) bool {
	var target BadRequest
	return stderrors.As(err, &target)
}

func isUnauthorized(err error) bool {
	var target Unauthorized
	return stderrors.As(err, &target)
}

func isForbidden(err error) bool {
	var target Forbidden
	return stderrors.As(err, &target)
}

func isNotFound(err error) bool {
	var target NotFound
	return stderrors.As(err, &target)
}

func isTooManyRequests(err error) bool {
	var target TooManyRequests
	return stderrors.As(err, &target)
}

func isInternalServer(err error) bool {
	var target InternalServerError
	return stderrors.As(err, &target)
}
