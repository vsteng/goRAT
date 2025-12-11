package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

// Logger wraps slog.Logger for structured logging
type Logger struct {
	*slog.Logger
}

// Global logger instance
var globalLogger *Logger

// Init initializes the global logger
func Init(level LogLevel, format string) {
	logLevel := slog.LevelInfo
	switch strings.ToLower(string(level)) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	globalLogger = &Logger{
		Logger: slog.New(handler),
	}
	slog.SetDefault(globalLogger.Logger)
}

// Get returns the global logger instance
func Get() *Logger {
	if globalLogger == nil {
		// Fallback to default text handler if not initialized
		handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
		globalLogger = &Logger{
			Logger: slog.New(handler),
		}
	}
	return globalLogger
}

// With returns a new logger with additional attributes
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
	}
}

// WithContext returns a new logger with context attributes
func (l *Logger) WithContext(ctx context.Context) *Logger {
	// Extract request ID if present in context
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return l.With("request_id", requestID)
	}
	return l
}

// DebugWith logs a debug message with attributes
func (l *Logger) DebugWith(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// InfoWith logs an info message with attributes
func (l *Logger) InfoWith(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// WarnWith logs a warning message with attributes
func (l *Logger) WarnWith(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// ErrorWith logs an error message with attributes
func (l *Logger) ErrorWith(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// ErrorWithErr logs an error message with an error object
func (l *Logger) ErrorWithErr(msg string, err error, args ...any) {
	args = append(args, slog.Any("error", err))
	l.Logger.Error(msg, args...)
}
