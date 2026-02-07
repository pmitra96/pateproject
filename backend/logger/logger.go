package logger

import (
	"log/slog"
	"os"
	"sync"
)

var (
	logger *slog.Logger
	once   sync.Once
)

// Init initializes the global structured logger
func Init() {
	once.Do(func() {
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug, // Log everything for now
		}
		// Use TextHandler for human readability in terminal/logs
		handler := slog.NewTextHandler(os.Stdout, opts)
		logger = slog.New(handler)
		slog.SetDefault(logger)
	})
}

// L returns the global logger instance
func L() *slog.Logger {
	if logger == nil {
		Init()
	}
	return logger
}

// Info is a shorthand for L().Info
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Error is a shorthand for L().Error
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// Debug is a shorthand for L().Debug
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Warn is a shorthand for L().Warn
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}
