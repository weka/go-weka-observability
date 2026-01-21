package logger

import (
	"log/slog"

	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog"
)

const (
	// logLevelInfo represents the info log level name for file separation
	logLevelInfo = "info"
	// logLevelError represents the error log level name for file separation
	logLevelError = "error"

	// defaultLogFileName is the fallback filename when LogFileName is not provided in FileMode
	defaultLogFileName = "app.log"

	// callerDisabled indicates that caller information should not be included in logs
	callerDisabled = -1

	// Default rotation configuration values
	defaultMaxSizeMB  = 100 // Maximum log file size in megabytes before rotation
	defaultMaxFiles   = 5   // Maximum number of backup files to keep
	defaultMaxAgeDays = 28  // Maximum number of days to retain old log files
)

type (
	// SinkConfig configures where logs are written and how they're rotated.
	// This is the "destination" or "sink" for log messages.
	// Follows industry conventions from Serilog (.NET) and Zap (Go).
	SinkConfig struct {
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
	FormatConfig struct {
		// Format is the output format (json/raw/plain)
		Format LogFormat `envconfig:"FORMAT"`

		// CallerDirLvl is the number of nested directories to display in caller field
		CallerDirLvl int `envconfig:"CALLER_DIR_LVL"`

		// Level is the minimum log level to output. -1=trace, 0=debug, 1=info, 2=warn, 3=error, 4=fatal
		Level zerolog.Level `envconfig:"LEVEL"`

		// TimeOnly uses time-only format instead of full timestamp
		TimeOnly bool `envconfig:"TIME_ONLY"`
	}

	// Config represents complete logger configuration.
	// Separates concerns: Sink (destination) vs Format (presentation).
	Config struct {
		Format FormatConfig
		Sink   SinkConfig
	}
)

// DefaultSinkConfig returns default sink configuration (console mode)
func DefaultSinkConfig() SinkConfig {
	return SinkConfig{
		Mode:       ConsoleMode,
		Dir:        "/var/log",
		FileName:   "",
		MaxSizeMB:  defaultMaxSizeMB,
		MaxFiles:   defaultMaxFiles,
		MaxAgeDays: defaultMaxAgeDays,
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

// DefaultConfig returns complete default configuration for the logger.
//
// Default configuration values:
//   - Sink.Mode:       ConsoleMode (logs to stderr)
//   - Sink.Dir:        "/var/log"
//   - Sink.FileName:   "" (empty, not used in console mode)
//   - Sink.MaxSizeMB:  100 MB
//   - Sink.MaxFiles:   5 backups
//   - Sink.MaxAgeDays: 28 days
//   - Format.Level:    zerolog.InfoLevel
//   - Format.Format:   LogFormatJSON
//   - Format.TimeOnly: false
//   - Format.CallerDirLvl: -1 (disabled)
//
// This configuration does NOT consider environment variables.
// For environment-aware configuration, use NewDefaultConfigWithEnvOverrides() instead.
//
// Example usage:
//
//	config := logger.DefaultConfig()
//	// Modify specific fields if needed
//	config.Format.Level = zerolog.DebugLevel
//	logr := logger.CreateLoggerFrom(config)
func DefaultConfig() Config {
	return Config{
		Sink:   DefaultSinkConfig(),
		Format: DefaultFormatConfig(),
	}
}

// NewConfigFromEnv creates Config with environment overrides.
// All fields with envconfig tags are overridable via LOG_* environment variables.
// Since envconfig doesn't automatically walk nested structs, we process Sink and Format separately.
// If environment variable processing fails, defaults are used as fallback.
//
//nolint:gocritic // hugeParam: intentional value semantics for clean API - called once at init, copy overhead negligible
func NewConfigFromEnv(defaultConfig Config) Config {
	// Process sink configuration
	if err := envconfig.Process("LOG", &defaultConfig.Sink); err != nil {
		slog.Warn("failed to process LOG_* environment variables for sink, using defaults",
			"error", err,
			"defaults", defaultConfig.Sink)
	}

	// Process format configuration
	if err := envconfig.Process("LOG", &defaultConfig.Format); err != nil {
		slog.Warn("failed to process LOG_* environment variables for format, using defaults",
			"error", err,
			"defaults", defaultConfig.Format)
	}

	return defaultConfig
}

// NewDefaultConfigWithEnvOverrides returns logger configuration with defaults that can be overridden by environment
// variables.
//
// Note: Since CreateLoggerFrom() now automatically applies environment overrides,
// using CreateLoggerFrom(DefaultConfig()) is equivalent to CreateLoggerFrom(NewDefaultConfigWithEnvOverrides()).
// This function is kept for backwards compatibility and explicit intent.
//
// Default configuration:
//   - Sink.Mode:       ConsoleMode (logs to stderr)
//   - Sink.Dir:        "/var/log"
//   - Sink.FileName:   "" (empty)
//   - Sink.MaxSizeMB:  100 MB
//   - Sink.MaxFiles:   5 backups
//   - Sink.MaxAgeDays: 28 days
//   - Format.Level:    zerolog.InfoLevel
//   - Format.Format:   LogFormatJSON
//   - Format.TimeOnly: false
//   - Format.CallerDirLvl: -1 (disabled)
//
// Environment variables (all optional):
//   - LOG_MODE:          "console" or "file"
//   - LOG_DIR:           Log directory path
//   - LOG_FILE_NAME:     Log file name
//   - LOG_MAX_SIZE_MB:   Max file size in MB
//   - LOG_MAX_FILES:     Max backup files
//   - LOG_MAX_AGE_DAYS:  Max retention days
//   - LOG_LEVEL:         Log level (-1=trace, 0=debug, 1=info, 2=warn, 3=error, 4=fatal)
//   - LOG_FORMAT:        "json", "raw", or "plain"
//   - LOG_TIME_ONLY:     "true" or "false"
//   - LOG_CALLER_DIR_LVL: Number of directory levels (-1=disabled)
//
// This function is equivalent to:
//
//	logger.NewConfigFromEnv(logger.DefaultConfig())
//
// Example usage (both are equivalent now):
//
//	// Explicit about env overrides
//	logr := logger.CreateLoggerFrom(logger.NewDefaultConfigWithEnvOverrides())
//
//	// Simpler (env overrides applied automatically)
//	logr := logger.CreateLoggerFrom(logger.DefaultConfig())
//
//	// Override via environment:
//	// export LOG_MODE=file
//	// export LOG_FILE_NAME=myapp.log
//	// export LOG_LEVEL=0  # debug level
func NewDefaultConfigWithEnvOverrides() Config {
	return NewConfigFromEnv(DefaultConfig())
}
