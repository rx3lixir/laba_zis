package httputil

import (
	"net/http"
)

// APIError represents an error that can be sent to clients
type HTTPError struct {
	Status  int    // HTTP status code
	Message string // User-facing message
	Cause   error  // Optional wrapped internal error (for logging)
	Details any    // Optional extra context (e.g. validation errors)
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	return e.Message
}

// Unwrap allows errors.Is and errors.As to work
func (e *HTTPError) Unwrap() error {
	return e.Cause
}

// Error with 400 status code
func BadRequest(msg string, details ...any) error {
	return &HTTPError{
		Status:  http.StatusBadRequest,
		Message: msg,
		Details: singleOrSlice(details),
	}
}

// Error with 404 status code
func NotFound(msg string) error {
	return &HTTPError{Status: http.StatusNotFound, Message: msg}
}

// Error with 500 status code
func Internal(err error) error {
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Message: "Something went wrong",
		Cause:   err,
	}
}

// Error with 401 status code
func Unauthorized(msg string) error {
	return &HTTPError{Status: http.StatusUnauthorized, Message: msg}
}

// Error with 403 status code
func Forbidden(msg string) error {
	return &HTTPError{Status: http.StatusForbidden, Message: msg}
}

// tiny helper so you can pass one detail or many
func singleOrSlice(v []any) any {
	switch len(v) {
	case 0:
		return nil
	case 1:
		return v[0]
	default:
		return v
	}
}
