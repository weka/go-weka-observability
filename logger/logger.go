// Package logger provides production-ready structured logging with automatic log rotation
// and environment-based configuration.
//
// # Key Features
//
//   - Zero-allocation JSON logging via zerolog
//   - Automatic log rotation via lumberjack
//   - Environment variable overrides (12-factor app compliance)
//   - Context-based logger propagation
//   - Separate info and error log files
//   - Integration with OpenTelemetry tracing
//
// # Quick Start
//
// Create a logger with defaults:
//
//	logr := logger.CreateLogger()
//	logr.Info("Application started")
//	// Override: LOG_LEVEL=0 LOG_FORMAT=raw
//
// Create a file logger with rotation:
//
//	logr := logger.CreateLogger(
//	    logger.WithFileSink("/var/log", "app.log"),
//	    logger.WithRotation(100, 5, 28), // 100MB, 5 files, 28 days
//	)
//
// Create with explicit configuration:
//
//	logr := logger.CreateLogger(
//	    logger.WithConsoleSink(),
//	    logger.WithInfoLevel(),
//	    logger.WithJSONFormat(),
//	)
//	ctx = logger.ContextWithLogr(ctx, logr)
//
// # Environment Variables
//
// All logger creation methods automatically respect environment variables:
//
//   - LOG_MODE: "console" or "file"
//   - LOG_LEVEL: -1=trace, 0=debug, 1=info, 2=warn, 3=error
//   - LOG_FORMAT: "json", "raw", "plain"
//   - LOG_DIR: Log directory path
//   - LOG_FILE_NAME: Log file name
//   - LOG_MAX_SIZE_MB: Max file size before rotation
//   - LOG_MAX_FILES: Max backup files to keep
//   - LOG_MAX_AGE_DAYS: Max retention in days
//
// Environment variables always override code defaults, following the 12-factor app pattern.
//
// # Integration with OpenTelemetry
//
// Use with instrumentation package for traced logging:
//
//	logr := logger.CreateLogger()
//	shutdown, _ := instrumentation.SetupOTelSDKWithOptions(ctx, "service", "v1.0.0", logr)
//	defer shutdown(ctx)
//
//	ctx, spanLogger, end := instrumentation.GetLogSpan(ctx, "operation")
//	defer end()
//	spanLogger.Info("Processing", "user_id", 123)
//
// See package documentation for complete examples and configuration options.
package logger

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
)

// Logger wraps zerolog.Logger and provides additional methods
type Logger struct {
	*zerolog.Logger
}

// NewLogger creates a new Logger with environment configuration
func NewLogger() *Logger {
	log := NewZeroLogger()
	return &Logger{log}
}

// By default, log string in zerolog that uses `caller` will have formart:
// 2024-09-26T00:00:00+00:00 ERR path/to/file.go:217 > Error running some operation error="error text" additional_field=value logger=TopLevelName.NestedLoggerName
// without `caller`:
// 2024-09-26T00:00:00+00:00 ERR Error running some operation error="error text" additional_field=value logger=TopLevelName.NestedLoggerName
// ---
// This function will change the `logger` field to be put instead of `caller`:
// 2024-09-26T00:00:00+00:00 ERR TopLevelName.NestedLoggerName > Error running some operation error="error text" additional_field=value
func NewZerologrWithLoggerNameInsteadCaller() logr.Logger {
	initLogger := NewZeroLoggerWithoutCaller()
	zerologr.NameFieldName = "caller"
	return zerologr.New(initLogger)
}

// NewLoggerWithoutCaller creates a new Logger without caller information
func NewLoggerWithoutCaller() *Logger {
	log := NewZeroLoggerWithoutCaller()
	return &Logger{log}
}

// NewZeroLogger creates zerolog logger with environment configuration
func NewZeroLogger() *zerolog.Logger {
	log := zerolog.New(GetStderrWriter()).
		Level(GetLogLevel()).
		With().
		Timestamp().
		Caller().
		Logger()
	return &log
}

// NewZeroLoggerWithoutCaller creates zerolog logger without caller information
func NewZeroLoggerWithoutCaller() *zerolog.Logger {
	log := zerolog.New(GetStderrWriter()).
		Level(GetLogLevel()).
		With().
		Timestamp().
		Logger()
	return &log
}

// NewZeroLoggerWithConfig creates zerolog logger from complete config
func NewZeroLoggerWithConfig(config Config) *zerolog.Logger {
	// Create base logger with configured writer and level
	logCtx := zerolog.New(GetMultiLevelWriterWithConfig(config)).
		Level(config.Format.Level).
		With().
		Timestamp()

	// Apply caller configuration if enabled
	if config.Format.CallerDirLvl != callerDisabled {
		setCallerMarshalFunc(config.Format.CallerDirLvl)
		logCtx = logCtx.Caller()
	}

	log := logCtx.Logger()
	return &log
}

// NewNamedLogger creates a logger with service name field
func NewNamedLogger(serviceName string) *Logger {
	log := zerolog.New(GetStderrWriter()).
		Level(GetLogLevel()).
		With().
		Str("service", serviceName).
		Timestamp().
		Caller().
		Logger()
	return &Logger{&log}
}

// Printf logs an info message with formatting
func (l *Logger) Printf(msg string, opts ...any) {
	l.Info().CallerSkipFrame(1).Msgf(msg, opts...)
}

// Warnf logs a warning message with formatting
func (l *Logger) Warnf(msg string, opts ...any) {
	l.Warn().CallerSkipFrame(1).Msgf(msg, opts...)
}

// Errorf logs an error message with formatting
func (l *Logger) Errorf(msg string, opts ...any) {
	l.Error().CallerSkipFrame(1).Msgf(msg, opts...)
}

// Debugf logs a debug message with formatting
func (l *Logger) Debugf(msg string, opts ...any) {
	l.Debug().CallerSkipFrame(1).Msgf(msg, opts...)
}

// Tracef logs a trace message with formatting
func (l *Logger) Tracef(msg string, opts ...any) {
	l.Trace().CallerSkipFrame(1).Msgf(msg, opts...)
}

// PrintErr logs an error at info level
func (l *Logger) PrintErr(err error) {
	l.Info().CallerSkipFrame(1).Err(err).Send()
}

// WarnErr logs an error at warn level
func (l *Logger) WarnErr(err error) {
	l.Warn().CallerSkipFrame(1).Err(err).Send()
}

// FatalErr logs an error at fatal level
func (l *Logger) FatalErr(err error) {
	l.Fatal().CallerSkipFrame(1).Err(err).Send()
}
