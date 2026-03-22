package logger

import (
	"log/slog"
	"os"
)

var log *slog.Logger

// Init initializes the structured logger
func Init(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	log = slog.New(handler)
	slog.SetDefault(log)
}

// Info logs an info message with key-value pairs
func Info(msg string, args ...any) {
	log.Info(msg, args...)
}

// Debug logs a debug message with key-value pairs
func Debug(msg string, args ...any) {
	log.Debug(msg, args...)
}

// Warn logs a warning message with key-value pairs
func Warn(msg string, args ...any) {
	log.Warn(msg, args...)
}

// Error logs an error message with key-value pairs
func Error(msg string, args ...any) {
	log.Error(msg, args...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, args ...any) {
	log.Error(msg, args...)
	os.Exit(1)
}
