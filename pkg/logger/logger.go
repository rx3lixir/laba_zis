package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Env        string
	JSONOutput bool
	AddSource  bool
}

// Logger is a wrapper around slog.Logger with additional methods
type Logger struct {
	*slog.Logger
}

func New(config Config) (*Logger, error) {
	level := parseLogLevel(config.Env)

	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: config.AddSource,
	}

	handler, err := determineHandler(config.Env, handlerOpts)
	if err != nil {
		return nil, fmt.Errorf("Failed to determine handler %d", err)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return &Logger{
		Logger: logger,
	}, nil
}

func determineHandler(env string, opts *slog.HandlerOptions) (slog.Handler, error) {
	switch strings.ToLower(env) {
	case "prod":
		return slog.NewJSONHandler(os.Stdout, opts), nil
	case "dev":
		return slog.NewTextHandler(os.Stdout, opts), nil
	default:
		return nil, fmt.Errorf("lox")
	}
}

func parseLogLevel(env string) slog.Level {
	switch strings.ToLower(env) {
	case "dev":
		return slog.LevelDebug
	case "prod":
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
