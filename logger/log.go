package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitializeLogger initializes the global logger.
func InitializeLogger() {

	env := os.Getenv("ENV")
	var err error
	var logger *zap.Logger
	if env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}

	// Use the Sugared logger for a user-friendly API
	Logger = logger
}

// Close flushes the logger buffers (important for production to avoid losing log entries)
func Close() {
	if err := Logger.Sync(); err != nil {
		log.Fatalf("failed to flush log entries: %v", err)
	}
}

func GetLogger() *zap.Logger {
	return Logger
}

// Global logging methods to avoid `logger.Logger` repetition

func Info(msg string, args ...zapcore.Field) {
	Logger.Info(msg, args...)
}

func Warn(msg string, args ...zapcore.Field) {
	Logger.Warn(msg, args...)
}

func Error(msg string, args ...zapcore.Field) {
	Logger.Error(msg, args...)
}

func Fatal(msg string, args ...zapcore.Field) {
	Logger.Fatal(msg, args...)
}

func Debug(msg string, args ...zapcore.Field) {
	Logger.Debug(msg, args...)
}
