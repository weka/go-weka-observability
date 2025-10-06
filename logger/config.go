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
		Mode:       ConsoleMode,
		Dir:        "/var/log",
		FileName:   "",
		MaxSizeMB:  100,
		MaxFiles:   5,
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
// If environment variable processing fails, defaults are used as fallback.
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

// NewDefaultConfigWithEnvOverrides convenience wrapper for NewConfigFromEnv with defaults
func NewDefaultConfigWithEnvOverrides() Config {
	return NewConfigFromEnv(DefaultConfig())
}
