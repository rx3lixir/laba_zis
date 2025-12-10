package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

// HTTPError represents an error that can be sent to clients
type HTTPError struct {
	Code      int    `json:"-"`                 // HTTP status code
	Message   string `json:"message"`           // User-facing message
	Details   any    `json:"details,omitempty"` // Additional context
	Internal  error  `json:"-"`                 // Internal error (not sent to client)
	RequestID string `json:"request_id,omitempty"`
	LogLevel  string `json:"-"` // How severe (for logging)
}

// In order to HTTPError implement Error
func (e *HTTPError) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Internal)
	}
	return e.Message
}

func (e *HTTPError) Unwrap() error {
	return e.Internal
}

// ErrorResponse is what gets sent to the client
type ErrorResponse struct {
	Error     string `json:"error"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// HandlerFunc is like http.HandlerFunc but returns an error
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// GetRequestID extracts the request ID from context (safe fallback)
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return "unknown"
	}
	if id := middleware.GetReqID(ctx); id != "" {
		return id
	}
	return "unknown"
}

// ServeHTTP implements http.Handler
func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handleHTTPError(w http.ResponseWriter, r *http.Request, err *HTTPError, log logger.Logger) {
	logFields := []any{
		"status", err.Code,
		"message", err.Message,
		"path", r.URL.Path,
		"method", r.Method,
		"request_id", err.RequestID,
	}

	if err.Internal != nil {
		logFields = append(logFields, "internal_error", err.Internal.Error())
	}

	switch err.LogLevel {
	case "error":
		log.Error("HTTP error", logFields...)
	case "warn":
		log.Warn("HTTP warning", logFields...)
	default:
		log.Warn("HTTP response", logFields...)
	}

	response := ErrorResponse{
		Error:     err.Message,
		Details:   err.Details,
		RequestID: err.RequestID,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)

	if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
		log.Error("Failed to encode error response", "error", encErr)
	}
}

func BadRequest(message string, details ...any) *HTTPError {
	return &HTTPError{
		Code:     http.StatusBadRequest,
		Message:  message,
		Details:  getDetails(details...),
		LogLevel: "warn",
	}
}

func Unauthorized(message string) *HTTPError {
	return &HTTPError{
		Code:     http.StatusUnauthorized,
		Message:  message,
		LogLevel: "warn",
	}
}

func Forbidden(message string) *HTTPError {
	return &HTTPError{
		Code:     http.StatusForbidden,
		Message:  message,
		LogLevel: "warn",
	}
}

func NotFound(resource string) *HTTPError {
	return &HTTPError{
		Code:     http.StatusNotFound,
		Message:  fmt.Sprintf("%s not found", resource),
		LogLevel: "info",
	}
}

func Conflict(message string, details ...any) *HTTPError {
	return &HTTPError{
		Code:     http.StatusConflict,
		Message:  message,
		Details:  getDetails(details...),
		LogLevel: "warn",
	}
}

func InternalError(message string, err error) *HTTPError {
	return &HTTPError{
		Code:     http.StatusInternalServerError,
		Message:  message,
		Internal: err,
		LogLevel: "error",
	}
}

func ServiceUnavailable(message string) *HTTPError {
	return &HTTPError{
		Code:     http.StatusServiceUnavailable,
		Message:  message,
		LogLevel: "error",
	}
}

func getDetails(details ...any) any {
	if len(details) == 0 {
		return nil
	}
	if len(details) == 1 {
		return details[0]
	}
	return details
}

// JSON sends a JSON response with status code
func JSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return InternalError("Failed to encode response", err)
	}

	return nil
}

// DecodeJSON decodes request body into target
func DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return BadRequest("Request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return BadRequest("Invalid JSON format", map[string]string{
			"error": err.Error(),
		})
	}

	return nil
}

// Parse UUID parses UUID from URL parameter
func ParseUUID(r *http.Request, paramName string) (uuid.UUID, error) {
	idStr := chi.URLParam(r, paramName)
	if idStr == "" {
		return uuid.Nil, BadRequest(fmt.Sprintf("%s is required", paramName))
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, BadRequest(fmt.Sprintf("Invalid %s", paramName))
	}

	return id, nil
}
