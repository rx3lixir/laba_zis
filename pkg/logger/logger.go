package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Env    string
	Output io.Writer
}

func New(c Config) *slog.Logger {
	if c.Output == nil {
		c.Output = os.Stdout
	}
	if c.Env == "" {
		c.Env = "dev"
	}

	level := resolveLevel(c.Env)

	opts := &slog.HandlerOptions{
		AddSource: false,
		Level:     level,
	}

	handler := resolveHandlerType(c, opts)

	l := slog.New(handler)
	slog.SetDefault(l)

	return l
}

func resolveHandlerType(c Config, opts *slog.HandlerOptions) slog.Handler {
	var handler slog.Handler

	switch strings.ToLower(c.Env) {
	case "prod":
		handler = slog.NewJSONHandler(c.Output, opts)
	case "dev":
		handler = slog.NewTextHandler(c.Output, opts)
	case "test":
		handler = slog.NewTextHandler(c.Output, &slog.HandlerOptions{Level: slog.LevelError})
	default:
		handler = slog.NewJSONHandler(c.Output, opts)
	}

	return handler
}

func resolveLevel(env string) slog.Leveler {
	switch strings.ToLower(env) {
	case "dev", "development":
		return slog.LevelDebug
	case "test":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
