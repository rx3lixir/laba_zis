package logger

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
)

// Must panics if logger creation fails
// Useful for package-level initialization where errors are unrecoverable
func Must(logger *Logger, err error) *Logger {
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	return logger
}

// Caller returns information about the calling function as a slog attribute
// skip=0 returns info about the immediate caller, skip=1 goes one level up, etc.
func Caller(skip int) slog.Attr {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return slog.String("caller", "unknown")
	}
	return slog.String("caller", fmt.Sprintf("%s:%d", filepath.Base(file), line))
}
