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

type Handler func(http.ResponseWriter, *http.Request) error

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

	// Logging cause
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

	// Response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpErr.Status)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":      httpErr.Message,
		"details":    httpErr.Details,
		"request_id": reqID,
	})
}

// JSON — simple success responder
func RespondJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// DecodeJSON decodes request body into target
func DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil || r.ContentLength == 0 {
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

// ParseUUID parses UUID from URL parameter
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

// GetRequestID extracts the request ID from context (safe fallback)
func getReqID(ctx context.Context) string {
	if ctx == nil {
		return "unknown"
	}
	if id := middleware.GetReqID(ctx); id != "" {
		return id
	}
	return "unknown"
}
