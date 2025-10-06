// Package logger provides structured logging and observability utilities for Weka applications.
//
// This package supports multiple output modes (console and file), configurable via environment
// variables using kelseyhightower/envconfig. Environment variables are prefixed with "LOG_"
// (e.g., LOG_MODE, LOG_DIR, LOG_FILE_NAME). The package integrates with OpenTelemetry tracing
// and provides both zerolog-based Logger and logr.Logger interfaces for different use cases.
package logger

import (
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
