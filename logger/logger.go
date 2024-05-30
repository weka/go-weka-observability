// Package logger provides structured logging and observability utilities for Weka applications.
//
// This package supports multiple output modes (console and file), configurable via environment
// variables using kelseyhightower/envconfig. Environment variables are prefixed with "LOG_"
// (e.g., LOG_MODE, LOG_DIR, LOG_FILE_NAME). The package integrates with OpenTelemetry tracing
// and provides both zerolog-based Logger and logr.Logger interfaces for different use cases.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ctxLogger struct{}
type logrContextKey struct{}

type Logger struct {
	*zerolog.Logger
}

// OutputMode defines where logs should be written
type OutputMode string

const (
	// ConsoleMode outputs logs to stderr
	ConsoleMode OutputMode = "console"
	// FileMode outputs logs to files
	FileMode OutputMode = "file"
)

// LogFormat represents the format of log output
type LogFormat string

const (
	// LogFormatRaw outputs logs in human-readable format with colors
	LogFormatRaw LogFormat = "raw"
	// LogFormatJSON outputs logs in JSON format
	LogFormatJSON LogFormat = "json"
	// LogFormatPlain outputs logs in plain text format without colors
	LogFormatPlain LogFormat = "plain"
)

// SinkConfig configures where logs are written and how they're rotated.
// This is the "destination" or "sink" for log messages.
// Follows industry conventions from Serilog (.NET) and Zap (Go).
type SinkConfig struct {
	// Mode determines where logs are written (console or file)
	Mode OutputMode `envconfig:"MODE"`
	// Dir is the directory for log files (only used in FileMode)
	Dir string `envconfig:"DIR"`
	// FileName is the base name for log files (only used in FileMode)
	FileName string `envconfig:"FILE_NAME"`
	// MaxSizeMB is the maximum size of a log file before rotation (in megabytes)
	MaxSizeMB int `envconfig:"MAX_SIZE_MB"`
	// MaxFiles is the maximum number of log files to keep
	MaxFiles int `envconfig:"MAX_FILES"`
	// MaxAgeDays is the maximum number of days to retain old log files
	MaxAgeDays int `envconfig:"MAX_AGE_DAYS"`
}

// FormatConfig configures how logs are presented.
// This controls the visual appearance and filtering of log messages.
type FormatConfig struct {
	// Level is the minimum log level to output
	Level zerolog.Level `envconfig:"LEVEL"`
	// Format is the output format (json/raw/plain)
	Format LogFormat `envconfig:"FORMAT"`
	// TimeOnly uses time-only format instead of full timestamp
	TimeOnly bool `envconfig:"TIME_ONLY"`
	// CallerDirLvl is the number of nested directories to display in caller field
	CallerDirLvl int `envconfig:"CALLER_DIR_LVL"`
}

// Config represents complete logger configuration.
// Separates concerns: Sink (destination) vs Format (presentation).
type Config struct {
	Sink   SinkConfig
	Format FormatConfig
}

// DefaultSinkConfig returns default sink configuration (console mode)
func DefaultSinkConfig() SinkConfig {
	return SinkConfig{
		Mode:      ConsoleMode,
		Dir:       "/var/log",
		FileName:  "",
		MaxSizeMB: 100,
		MaxFiles:  5,
		MaxAgeDays: 28,
	}
}

// DefaultFormatConfig returns default format configuration
func DefaultFormatConfig() FormatConfig {
	return FormatConfig{
		Level:        zerolog.InfoLevel,
		Format:       LogFormatJSON,
		TimeOnly:     false,
		CallerDirLvl: callerDisabled,
	}
}

// DefaultConfig returns complete default configuration
func DefaultConfig() Config {
	return Config{
		Sink:   DefaultSinkConfig(),
		Format: DefaultFormatConfig(),
	}
}

// NewConfigFromEnv creates Config with environment overrides.
// All fields with envconfig tags are overridable via LOG_* environment variables.
// Since envconfig doesn't automatically walk nested structs, we process Sink and Format separately.
func NewConfigFromEnv(defaultConfig Config) Config {
	// Process sink configuration
	if err := envconfig.Process("LOG", &defaultConfig.Sink); err != nil {
		slog.Warn("failed to process LOG_* environment variables for sink", "error", err)
	}

	// Process format configuration
	if err := envconfig.Process("LOG", &defaultConfig.Format); err != nil {
		slog.Warn("failed to process LOG_* environment variables for format", "error", err)
	}

	return defaultConfig
}

// NewDefaultConfigWithEnvOverrides convenience wrapper for NewConfigFromEnv with defaults
func NewDefaultConfigWithEnvOverrides() Config {
	return NewConfigFromEnv(DefaultConfig())
}

const (
	// logLevelInfo represents the info log level name for file separation
	logLevelInfo = "info"
	// logLevelError represents the error log level name for file separation
	logLevelError = "error"

	// defaultLogFileName is the fallback filename when LogFileName is not provided in FileMode
	defaultLogFileName = "app.log"

	// callerDisabled indicates that caller information should not be included in logs
	callerDisabled = -1
)

var (
	// infoLevels defines the log levels that should be written to the info writer
	infoLevels = []zerolog.Level{
		zerolog.TraceLevel,
		zerolog.DebugLevel,
		zerolog.InfoLevel,
	}
	// errorLevels defines the log levels that should be written to the error writer
	errorLevels = []zerolog.Level{
		zerolog.WarnLevel,
		zerolog.ErrorLevel,
		zerolog.FatalLevel,
		zerolog.PanicLevel,
	}
)


// setCallerMarshalFunc configures caller directory display level
func setCallerMarshalFunc(callerDirLvl int) {
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		dirsNum := callerDirLvl
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				if dirsNum < 1 {
					break
				}
				dirsNum--
			}
		}
		return short + ":" + strconv.Itoa(line)
	}
}

// Set the amount of nested dirs displayed before `<file_name>:<line_number>` for `caller` field in logger.
// `LOG_CALLER_DIR_LVL` is used for this.
// If unset - does nothing (default `caller` formatting is used)
// If `LOG_CALLER_DIR_LVL=0`, only the filename and line number are displayed (e.g. `message_processor.go:89`)
// see https://github.com/rs/zerolog/blob/master/README.md#add-file-and-line-number-to-log
//
// DEPRECATED: Use Config.Format.CallerDirLvl instead.
// This function reads LOG_CALLER_DIR_LVL directly from environment.
// Migrate to FormatConfig for better testability.
func SetCallerDirDisplayLevel() {
	callerDirLvl, ok := os.LookupEnv("LOG_CALLER_DIR_LVL")
	if !ok {
		return
	}
	var lvl int
	if val, err := strconv.Atoi(callerDirLvl); err == nil {
		lvl = val
	}
	setCallerMarshalFunc(lvl)
}

// GetWriterFromFormat creates io.Writer based on format config
func GetWriterFromFormat(format FormatConfig) io.Writer {
	timeFormat := time.RFC3339
	if format.TimeOnly {
		timeFormat = time.TimeOnly
	}

	switch format.Format {
	case LogFormatJSON:
		return os.Stderr
	case LogFormatPlain:
		return zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: timeFormat,
			NoColor:    true,
		}
	case LogFormatRaw:
		fallthrough
	default:
		return zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: timeFormat,
		}
	}
}

type SpecificLevelWriter struct {
	io.Writer
	Levels []zerolog.Level
}

func (w SpecificLevelWriter) WriteLevel(level zerolog.Level, p []byte) (int, error) {
	if slices.Contains(w.Levels, level) {
		return w.Write(p)
	}
	return len(p), nil
}

// GetStderrWriter is DEPRECATED: Use GetWriterFromFormat instead.
// This function reads LOG_FORMAT and LOG_TIME_ONLY directly from environment.
// Migrate to FormatConfig for better testability.
func GetStderrWriter() io.Writer {
	config := DefaultFormatConfig()
	// Override with env vars for backward compatibility
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Format = LogFormat(format)
	}
	if os.Getenv("LOG_TIME_ONLY") == "true" {
		config.TimeOnly = true
	}
	return GetWriterFromFormat(config)
}

// GetMultiLevelWriter is DEPRECATED: Use GetMultiLevelWriterWithConfig instead.
// This function uses environment variables for configuration.
// Migrate to explicit Config for better testability.
func GetMultiLevelWriter() io.Writer {
	config := NewDefaultConfigWithEnvOverrides()
	return GetMultiLevelWriterWithConfig(config)
}

// GetMultiLevelWriterWithConfig creates multi-level writer from complete config
func GetMultiLevelWriterWithConfig(config Config) io.Writer {
	// Validate FileMode sink configuration
	if config.Sink.Mode == FileMode {
		if config.Sink.FileName == "" {
			slog.Warn("FileMode requires FileName, using fallback",
				"fallback", defaultLogFileName)
			config.Sink.FileName = defaultLogFileName
		}
		if config.Sink.Dir == "" {
			fallbackDir := os.TempDir()
			slog.Warn("FileMode requires Dir, using fallback",
				"fallback", fallbackDir)
			config.Sink.Dir = fallbackDir
		}
	}

	infoWriter, errorWriter := createWritersForSink(config.Sink, config.Format)
	return createMultiLevelWriter(infoWriter, errorWriter)
}


// createWritersForSink determines writers based on sink mode
func createWritersForSink(sink SinkConfig, format FormatConfig) (info, error io.Writer) {
	if sink.Mode == FileMode {
		// File mode: separate files for info and error
		return createLumberjackWriter(sink, logLevelInfo),
			createLumberjackWriter(sink, logLevelError)
	}
	// Console mode: same writer for both (format-aware)
	writer := GetWriterFromFormat(format)
	return writer, writer
}


// createMultiLevelWriter assembles the multi-level writer with predefined level groups
func createMultiLevelWriter(infoWriter, errorWriter io.Writer) io.Writer {
	return zerolog.MultiLevelWriter(
		SpecificLevelWriter{Writer: infoWriter, Levels: infoLevels},
		SpecificLevelWriter{Writer: errorWriter, Levels: errorLevels},
	)
}

// createLumberjackWriter creates a lumberjack writer for the specified level with automatic rotation
func createLumberjackWriter(sink SinkConfig, level string) io.Writer {
	// Create log file path
	logFileName := sink.FileName
	if level != logLevelInfo {
		// Add level suffix for error logs
		ext := filepath.Ext(logFileName)
		if ext != "" {
			base := logFileName[:len(logFileName)-len(ext)]
			logFileName = base + "-" + level + ext
		} else {
			logFileName = logFileName + "-" + level
		}
	}

	// Create lumberjack logger with rotation configuration
	return &lumberjack.Logger{
		Filename:   filepath.Join(sink.Dir, logFileName),
		MaxSize:    sink.MaxSizeMB, // megabytes
		MaxBackups: sink.MaxFiles,
		MaxAge:     sink.MaxAgeDays, // days
		Compress:   true,            // compress old files
		LocalTime:  true,            // use local time for timestamps
	}
}

// GetLogLevel is DEPRECATED: Use Config.Format.Level instead.
// This function reads LOG_LEVEL environment variable directly.
// Migrate to structured configuration for better testability.
func GetLogLevel() zerolog.Level {
	lvlStr := os.Getenv("LOG_LEVEL")
	lvl := 1 // info level
	if val, err := strconv.Atoi(lvlStr); err == nil {
		lvl = val
	}
	return zerolog.Level(lvl)
}

func NewLogger() *Logger {
	log := NewZeroLogger()
	return &Logger{log}
}

func NewLoggerWithoutCaller() *Logger {
	log := NewZeroLoggerWithoutCaller()
	return &Logger{log}
}

func NewZeroLogger() *zerolog.Logger {
	log := zerolog.New(GetStderrWriter()).
		Level(GetLogLevel()).
		With().
		Timestamp().
		Caller().
		Logger()
	return &log
}

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


func (l *Logger) Printf(msg string, opts ...any) {
	l.Info().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Warnf(msg string, opts ...any) {
	l.Warn().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Errorf(msg string, opts ...any) {
	l.Error().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Debugf(msg string, opts ...any) {
	l.Debug().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) Tracef(msg string, opts ...any) {
	l.Trace().CallerSkipFrame(1).Msgf(msg, opts...)
}

func (l *Logger) PrintErr(err error) {
	l.Info().CallerSkipFrame(1).Err(err).Send()
}

func (l *Logger) WarnErr(err error) {
	l.Warn().CallerSkipFrame(1).Err(err).Send()
}

func (l *Logger) FatalErr(err error) {
	l.Fatal().CallerSkipFrame(1).Err(err).Send()
}

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

func GetLoggerFromContext(ctx context.Context) *Logger {
	if logger := ctx.Value(ctxLogger{}); logger != nil {
		ctxLogger, ok := logger.(*Logger)
		if ok {
			return ctxLogger
		}
	}
	return NewLogger()
}

func ContextWithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxLogger{}, l)
}

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

// LoggerOption is a functional option for logger creation
type LoggerOption func(*Config)

// Sink options

// WithConsoleSink configures console output
func WithConsoleSink() LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = ConsoleMode
	}
}

// WithFileSink configures file output with rotation
func WithFileSink(dir, filename string) LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = FileMode
		c.Sink.Dir = dir
		c.Sink.FileName = filename
	}
}

// WithRotation configures log rotation parameters
func WithRotation(maxSizeMB, maxFiles, maxAgeDays int) LoggerOption {
	return func(c *Config) {
		c.Sink.MaxSizeMB = maxSizeMB
		c.Sink.MaxFiles = maxFiles
		c.Sink.MaxAgeDays = maxAgeDays
	}
}

// Format options

// WithJSONFormat configures JSON output
func WithJSONFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatJSON
	}
}

// WithRawFormat configures colored console output
func WithRawFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatRaw
	}
}

// WithPlainFormat configures plain text output (no colors)
func WithPlainFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatPlain
	}
}

// WithLevel sets the log level
func WithLevel(level zerolog.Level) LoggerOption {
	return func(c *Config) {
		c.Format.Level = level
	}
}

// WithDebugLevel convenience for debug logging
func WithDebugLevel() LoggerOption {
	return WithLevel(zerolog.DebugLevel)
}

// WithInfoLevel convenience for info logging
func WithInfoLevel() LoggerOption {
	return WithLevel(zerolog.InfoLevel)
}

// WithTraceLevel convenience for trace logging
func WithTraceLevel() LoggerOption {
	return WithLevel(zerolog.TraceLevel)
}

// WithWarnLevel convenience for warn logging
func WithWarnLevel() LoggerOption {
	return WithLevel(zerolog.WarnLevel)
}

// WithErrorLevel convenience for error logging
func WithErrorLevel() LoggerOption {
	return WithLevel(zerolog.ErrorLevel)
}

// WithCaller enables caller information with specified directory depth
func WithCaller(dirLevel int) LoggerOption {
	return func(c *Config) {
		c.Format.CallerDirLvl = dirLevel
	}
}

// WithTimeOnly uses time-only format instead of full timestamp
func WithTimeOnly() LoggerOption {
	return func(c *Config) {
		c.Format.TimeOnly = true
	}
}

// CreateLogger creates a logr.Logger with functional options
func CreateLogger(opts ...LoggerOption) logr.Logger {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(&config)
	}
	return CreateLoggerFrom(config)
}
