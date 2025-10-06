package logger

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
)

type ctxLogger struct{}
type logrContextKey struct{}

// GetLoggerFromContext retrieves the legacy Logger from context
func GetLoggerFromContext(ctx context.Context) *Logger {
	if logger := ctx.Value(ctxLogger{}); logger != nil {
		ctxLogger, ok := logger.(*Logger)
		if ok {
			return ctxLogger
		}
	}
	return NewLogger()
}

// ContextWithLogger stores a legacy Logger in the context
func ContextWithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxLogger{}, l)
}

// ContextWithLogr stores a logr.Logger in the context.
// Returns a new context containing the logger.
// Follows Go stdlib pattern (context.WithValue, context.WithCancel, etc.).
//
// Use this after creating a logger with CreateLoggerFrom.
//
// Example:
//
//	logr := logger.CreateLoggerFrom(logger.DefaultLogConfig())
//	ctx = logger.ContextWithLogr(ctx, logr)
func ContextWithLogr(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, logrContextKey{}, logger)
}

// LogrFromContext retrieves the logr.Logger from context.
// Returns an error if no logger is found - caller decides how to handle.
// This follows Go best practices for error handling.
//
// Example:
//
//	logger, err := logger.LogrFromContext(ctx)
//	if err != nil {
//	    // Handle missing logger
//	}
func LogrFromContext(ctx context.Context) (logr.Logger, error) {
	logger := ctx.Value(logrContextKey{})
	if logger == nil {
		return logr.Logger{}, fmt.Errorf("logger not found in context")
	}
	logrLogger, ok := logger.(logr.Logger)
	if !ok {
		return logr.Logger{}, fmt.Errorf("context value is not a logr.Logger")
	}
	return logrLogger, nil
}

// MustLogrFromContext retrieves the logr.Logger from context.
// Panics if no logger is found - use when logger is required for operation.
// Follows Go stdlib pattern (e.g., regexp.MustCompile, template.Must).
//
// Example:
//
//	logger := logger.MustLogrFromContext(ctx)
//	logger.Info("Operation started")
func MustLogrFromContext(ctx context.Context) logr.Logger {
	logger, err := LogrFromContext(ctx)
	if err != nil {
		panic(err)
	}
	return logger
}

// LogrFromContextOrDefault retrieves logr.Logger from context, or creates a default one.
// Never fails - gracefully falls back to environment-configured logger.
// Use this when you want to ensure a logger always exists.
//
// Example:
//
//	logger := logger.LogrFromContextOrDefault(ctx)
//	logger.Info("Always has a logger")
func LogrFromContextOrDefault(ctx context.Context) logr.Logger {
	logger, err := LogrFromContext(ctx)
	if err == nil {
		return logger
	}

	// Create default logger with environment configuration
	zlog := NewZeroLogger()
	return zerologr.New(zlog)
}

// GetLoggerFromExistingWithStrValues creates a new Logger with additional string fields
func GetLoggerFromExistingWithStrValues(logger *Logger, vals map[string]string) *Logger {
	newLoggerCtx := logger.With()
	for k, v := range vals {
		newLoggerCtx = newLoggerCtx.Str(k, v)
	}
	newLogger := newLoggerCtx.Logger()
	return &Logger{&newLogger}
}

// CreateLoggerFrom creates a logr.Logger with the specified configuration.
// This is the recommended way to create a logger for use with the instrumentation package.
//
// Example with environment defaults:
//
//	logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())
//	ctx = logger.ContextWithLogr(ctx, logr)
//
// Example with explicit config:
//
//	config := logger.Config{
//	    Sink: logger.SinkConfig{
//	        Mode: logger.FileMode,
//	        Dir: "/var/log",
//	        FileName: "app.log",
//	    },
//	    Format: logger.FormatConfig{
//	        Level: zerolog.DebugLevel,
//	        Format: logger.LogFormatJSON,
//	    },
//	}
//	logr := logger.CreateLoggerFrom(config)
func CreateLoggerFrom(config Config) logr.Logger {
	zlog := NewZeroLoggerWithConfig(config)
	return zerologr.New(zlog)
}

// CreateLogger creates a logr.Logger with functional options
func CreateLogger(opts ...LoggerOption) logr.Logger {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return CreateLoggerFrom(config)
}
