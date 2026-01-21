package logger

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
)

// Sentinel errors for context operations
var (
	// ErrLoggerNotFound is returned when no logger exists in context.
	ErrLoggerNotFound = errors.New("logger not found in context")
	// ErrLoggerWrongType is returned when context value is not a logr.Logger.
	ErrLoggerWrongType = errors.New("context value is not a logr.Logger")
)

type (
	ctxLogger      struct{}
	logrContextKey struct{}
)

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
// Use this after creating a logger with CreateLogger or CreateLoggerFrom.
//
// Example:
//
//	logr := logger.CreateLogger(logger.WithConsoleSink(), logger.WithInfoLevel())
//	ctx = logger.ContextWithLogr(ctx, logr)
//	// Later retrieve: logger.MustLogrFromContext(ctx)
func ContextWithLogr(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, logrContextKey{}, logger)
}

// LogrFromContext retrieves the logr.Logger from context.
// Returns an error if no logger is found - caller decides how to handle.
// This follows Go best practices for error handling.
//
// Example with graceful fallback:
//
//	logger, err := logger.LogrFromContext(ctx)
//	if err != nil {
//	    // Create default logger as fallback
//	    logger = logger.CreateLogger()
//	}
//	logger.Info("Operation started")
func LogrFromContext(ctx context.Context) (logr.Logger, error) {
	logger := ctx.Value(logrContextKey{})
	if logger == nil {
		return logr.Logger{}, ErrLoggerNotFound
	}
	logrLogger, ok := logger.(logr.Logger)
	if !ok {
		return logr.Logger{}, ErrLoggerWrongType
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
// Environment variables (LOG_MODE, LOG_LEVEL, LOG_FORMAT, etc.) will override the provided config.
// This is the recommended way to create a logger for use with the instrumentation package.
//
// The function applies environment variable overrides automatically, so you can provide
// application defaults that users can override via environment variables.
//
// Example with environment defaults:
//
//	logr := logger.CreateLoggerFrom(logger.DefaultConfig())
//	ctx = logger.ContextWithLogr(ctx, logr)
//	// Environment variables can override any field
//
// Example with custom defaults:
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
//	// Users can override: LOG_LEVEL=1 to change to info level
//	// Or: LOG_MODE=console to output to stderr instead of file
func CreateLoggerFrom(config Config) logr.Logger {
	overridenConfig := NewConfigFromEnv(config)
	zlog := NewZeroLoggerWithConfig(overridenConfig)

	return zerologr.New(zlog)
}

// CreateLogger creates a logr.Logger with functional options.
// Environment variables (LOG_MODE, LOG_LEVEL, LOG_FORMAT, etc.) will override the options.
//
// This provides a clean API for setting application defaults that users can override via
// environment variables, following the 12-factor app pattern.
//
// Example with defaults:
//
//	// Creates console logger with JSON format, info level
//	logr := logger.CreateLogger()
//	logr.Info("Application started")
//	// Override: LOG_LEVEL=0 LOG_FORMAT=raw
//
// Example with file logging:
//
//	logr := logger.CreateLogger(
//	    logger.WithFileSink("/var/log", "app.log"),
//	    logger.WithInfoLevel(),
//	)
//	// Users can override: LOG_MODE=console to switch to stderr
//	// Or: LOG_LEVEL=0 to enable debug logging
//
// Example with context:
//
//	logr := logger.CreateLogger(
//	    logger.WithConsoleSink(),
//	    logger.WithDebugLevel(),
//	)
//	ctx = logger.ContextWithLogr(ctx, logr)
//	// Retrieve later: logger.MustLogrFromContext(ctx)
func CreateLogger(opts ...LoggerOption) logr.Logger {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}

	return CreateLoggerFrom(config)
}
