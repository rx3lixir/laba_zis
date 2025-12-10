package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

func Handle(log logger.Logger, h HandlerFunc) http.HandlerFunc {
	return ErrorHandlerMiddleware(log)(h)
}

func ErrorHandlerMiddleware(log logger.Logger) func(next HandlerFunc) http.HandlerFunc {
	return func(next HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Execute handler and capture error
			err := next(w, r)
			if err == nil {
				return
			}

			requestID := GetRequestID(r.Context())

			var httpErr *HTTPError

			if errors.As(err, &httpErr) {
				httpErr.RequestID = requestID
				handleHTTPError(w, r, httpErr, log)
				return
			}

			internalErr := &HTTPError{
				Code:      http.StatusInternalServerError,
				Message:   "An unexptecte error occured",
				Internal:  err,
				RequestID: requestID,
				LogLevel:  "error",
			}
			handleHTTPError(w, r, internalErr, log)
		}
	}
}

// RequestLogger logs every request with status, latency, request_id, etc.
func RequestLogger(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			latency := time.Since(start)

			log.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", latency.Milliseconds(),
				"size", ww.BytesWritten(),
				"request_id", GetRequestID(r.Context()),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
