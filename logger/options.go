package logger

import "github.com/rs/zerolog"

// LoggerOption is a functional option for logger creation
//
//nolint:revive // LoggerOption maintains backwards compatibility - logger.LoggerOption is intentional
type LoggerOption func(*Config)

// Sink options

// WithConsoleSink configures console output (stderr).
// Perfect for containers, K8s, and cloud-native applications.
// Overrideable via LOG_MODE environment variable.
//
// Example:
//
//	logr := logger.CreateLogger(logger.WithConsoleSink())
func WithConsoleSink() LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = ConsoleMode
	}
}

// WithFileSink configures file output with automatic rotation.
// Use for traditional applications that need persistent logs.
// Overrideable via LOG_MODE, LOG_DIR, LOG_FILE_NAME environment variables.
//
// Example:
//
//	logr := logger.CreateLogger(
//	    logger.WithFileSink("/var/log", "app.log"),
//	    logger.WithRotation(100, 5, 28), // 100MB, 5 files, 28 days
//	)
func WithFileSink(dir, filename string) LoggerOption {
	return func(c *Config) {
		c.Sink.Mode = FileMode
		c.Sink.Dir = dir
		c.Sink.FileName = filename
	}
}

// WithRotation configures log rotation parameters.
// Only applies when using WithFileSink.
// Overrideable via LOG_MAX_SIZE_MB, LOG_MAX_FILES, LOG_MAX_AGE_DAYS environment variables.
//
// Parameters:
//   - maxSizeMB: Max file size in MB before rotation (e.g., 100)
//   - maxFiles: Number of backup files to keep (e.g., 5)
//   - maxAgeDays: Max age in days for old logs (e.g., 28)
//
// Example:
//
//	logger.WithRotation(100, 5, 28) // 100MB, keep 5 backups, delete after 28 days
func WithRotation(maxSizeMB, maxFiles, maxAgeDays int) LoggerOption {
	return func(c *Config) {
		c.Sink.MaxSizeMB = maxSizeMB
		c.Sink.MaxFiles = maxFiles
		c.Sink.MaxAgeDays = maxAgeDays
	}
}

// Format options

// WithJSONFormat configures JSON output (structured logging).
// Best for production, machine parsing, and log aggregation systems.
// Overrideable via LOG_FORMAT=json environment variable.
func WithJSONFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatJSON
	}
}

// WithRawFormat configures colored console output (human-readable).
// Best for local development and debugging with terminal colors.
// Overrideable via LOG_FORMAT=raw environment variable.
func WithRawFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatRaw
	}
}

// WithPlainFormat configures plain text output (no colors).
// Best for CI/CD systems or when color codes cause issues.
// Overrideable via LOG_FORMAT=plain environment variable.
func WithPlainFormat() LoggerOption {
	return func(c *Config) {
		c.Format.Format = LogFormatPlain
	}
}

// WithLevel sets the minimum log level.
// Only messages at or above this level will be logged.
// Overrideable via LOG_LEVEL environment variable (e.g., LOG_LEVEL=0 for debug).
func WithLevel(level zerolog.Level) LoggerOption {
	return func(c *Config) {
		c.Format.Level = level
	}
}

// WithDebugLevel sets debug logging level (shows debug, info, warn, error).
// Overrideable via LOG_LEVEL=0 environment variable.
func WithDebugLevel() LoggerOption {
	return WithLevel(zerolog.DebugLevel)
}

// WithInfoLevel sets info logging level (shows info, warn, error).
// This is the default level. Overrideable via LOG_LEVEL=1 environment variable.
func WithInfoLevel() LoggerOption {
	return WithLevel(zerolog.InfoLevel)
}

// WithTraceLevel sets trace logging level (shows everything including trace).
// Very verbose, use only for detailed debugging. Overrideable via LOG_LEVEL=-1 environment variable.
func WithTraceLevel() LoggerOption {
	return WithLevel(zerolog.TraceLevel)
}

// WithWarnLevel sets warn logging level (shows warn, error only).
// Overrideable via LOG_LEVEL=2 environment variable.
func WithWarnLevel() LoggerOption {
	return WithLevel(zerolog.WarnLevel)
}

// WithErrorLevel sets error logging level (shows error only).
// Overrideable via LOG_LEVEL=3 environment variable.
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
