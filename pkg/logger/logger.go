package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// Config defines the logger configuration
type Config struct {
	Env              string
	Level            string
	AddSource        bool
	SourcePathLength int
	TimeFormat       string
	Output           io.Writer
}

// Logger wraps slog.Logger with additional convenience methods
type Logger struct {
	*slog.Logger
}

// New creates a new configured logger instance
func New(config Config) (*Logger, error) {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if config.TimeFormat == "" {
		config.TimeFormat = time.RFC3339
	}
	if config.Env == "" {
		config.Env = "dev"
	}

	handler, err := createHandler(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)

	return &Logger{Logger: logger}, nil
}

// WithContext adds context-aware logging (placeholder for future context extraction)
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// You could extract trace IDs, request IDs, etc. from context here
	return &Logger{l.Logger.With()}
}

// WithFields creates a logger with pre-populated fields
// Useful for request IDs, user IDs, service names, etc.
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{Logger: l.Logger.With(args...)}
}
