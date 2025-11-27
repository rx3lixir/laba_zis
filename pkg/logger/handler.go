package logger

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

func createHandler(config Config) (slog.Handler, error) {
	// Determine log level
	level := parseLogLevel(config.Env, config.Level)

	// Create base handler options
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	// Add source path shortening if needed
	if config.AddSource && config.SourcePathLength > 0 {
		handlerOpts.ReplaceAttr = createSourceReplacer(config.SourcePathLength)
	}

	// Create appropriate handler based on env
	env := strings.ToLower(config.Env)

	switch env {
	case "prod":
		return slog.NewJSONHandler(config.Output, handlerOpts), nil

	case "dev":
		textOpts := *handlerOpts
		textOpts.ReplaceAttr = createTextReplacer(config.TimeFormat, config.SourcePathLength)
		return slog.NewTextHandler(config.Output, &textOpts), nil

	case "test":
		return slog.NewTextHandler(config.Output, &slog.HandlerOptions{
			Level: slog.LevelError,
		}), nil

	default:
		return nil, fmt.Errorf("unknown environment: %s (use 'dev', 'prod', or 'test')", config.Env)
	}
}

func parseLogLevel(env, explicitLevel string) slog.Level {
	if explicitLevel != "" {
		switch strings.ToLower(explicitLevel) {
		case "debug":
			return slog.LevelDebug
		case "info":
			return slog.LevelInfo
		case "warn":
			return slog.LevelWarn
		case "error":
			return slog.LevelError
		}
	}

	switch strings.ToLower(env) {
	case "dev":
		return slog.LevelDebug
	case "prod":
		return slog.LevelInfo
	case "test":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// createSourceReplacer creates a replacer that shortens source file paths
func createSourceReplacer(pathLength int) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.SourceKey {
			source := a.Value.Any().(*slog.Source)
			if source != nil {
				source.File = shortenPath(source.File, pathLength)
			}
		}
		return a
	}
}

// createTextReplacer handles both time format and source path for text logs
func createTextReplacer(timeFormat string, pathLength int) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		// Custom time format for dev logs
		if a.Key == slog.TimeKey {
			if t, ok := a.Value.Any().(time.Time); ok {
				a.Value = slog.StringValue(t.Format(timeFormat))
			}
		}

		// Shorten source paths if needed
		if a.Key == slog.SourceKey && pathLength > 0 {
			source := a.Value.Any().(*slog.Source)
			if source != nil {
				source.File = shortenPath(source.File, pathLength)
			}
		}

		return a
	}
}

// shortenPath reduces path length to the specified number of segments
func shortenPath(path string, segments int) string {
	if segments == 0 {
		return path
	}

	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) <= segments {
		return path
	}

	return strings.Join(parts[len(parts)-segments:], "/")
}
