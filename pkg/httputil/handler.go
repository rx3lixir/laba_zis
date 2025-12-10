package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// HandlerFunc is a custom handler that can return errors
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// Handler wraps error-returning handler into a standard http.HandlerFunc
// This is the key adapter that makes everything work
func Handler(h HandlerFunc, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			RespondError(w, r, err, log)
		}
	}
}

// Centralized error responder
func RespondError(w http.ResponseWriter, r *http.Request, err error, log *slog.Logger) {
	reqID := getReqID(r.Context())

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		httpErr = &HTTPError{
			Status:  http.StatusInternalServerError,
			Message: "Internal Server Error",
			Cause:   err,
		}
	}

	// Logging based on severity
	if httpErr.Status >= 500 {
		log.Error(
			"request failed",
			"error", err,
			"status", httpErr.Status,
			"path", r.URL.Path,
			"request_id", reqID,
		)
	} else {
		log.Warn(
			"client error",
			"error", err,
			"status", httpErr.Status,
			"path", r.URL.Path,
			"request_id", reqID,
		)
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.Status)

	response := map[string]any{
		"error":      httpErr.Message,
		"request_id": reqID,
	}

	if httpErr.Details != nil {
		response["details"] = httpErr.Details
	}

	_ = json.NewEncoder(w).Encode(response)
}

// RespondJSON sends a successful JSON response
func RespondJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// DecodeJSON decodes request body into target with validation
func DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil || r.ContentLength == 0 {
		return BadRequest("Request body is required")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return BadRequest("Invalid JSON format", map[string]string{
			"parse_error": err.Error(),
		})
	}

	return nil
}

// ParseUUID extracts and parses a UUID from URL parameters
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

// getReqID safely extracts request ID from context
func getReqID(ctx context.Context) string {
	if ctx == nil {
		return "unknown"
	}
	if id := middleware.GetReqID(ctx); id != "" {
		return id
	}
	return "unknown"
}
